package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
	"github.com/chenchangchao/go-pi-agent/internal/agenttool"
	"github.com/chenchangchao/go-pi-agent/internal/provider"
	"github.com/chenchangchao/go-pi-agent/internal/session"
)

const (
	defaultMaxTurns        = 8
	defaultEventBufferSize = 32
	defaultHistoryLimit    = 50
)

// Config 定义一次 Agent 运行所需的依赖和限制。
type Config struct {
	Provider     provider.Provider
	Tools        *agenttool.Registry
	SystemPrompt string
	MaxTurns     int

	// SessionStore 和 SessionID 同时设置时，
	// Runtime 会加载并持久化该会话的历史消息。
	SessionStore session.Store
	SessionID    string

	// HistoryLimit 限制加载的最近历史消息数量。
	// 小于等于 0 时使用默认值。
	HistoryLimit int
}

// Result 是一次 Agent 运行的最终结果。
type Result struct {
	Message   agentcore.Message
	Messages  []agentcore.Message
	Turns     int
	ToolCalls int
	Usage     provider.Usage
	SessionID string
}

// eventEmitter 是 Runtime 内部使用的事件发送函数。
//
// 当 emitter 为 nil 时，Runtime 按原来的同步模式运行，
// 不产生任何外部事件。
type eventEmitter func(agentcore.AgentEvent) error

// Run 同步执行 Agent，并在结束后返回完整结果。
//
// 该接口保持与 v0.1.0 兼容。
func Run(
	ctx context.Context,
	config Config,
	userPrompt string,
) (Result, error) {
	return run(ctx, config, userPrompt, nil)
}

// Stream 异步执行 Agent，并返回事件流。
//
// 当前阶段 Provider 仍然是非流式 Chat，因此暂时不会产生
// MessageDelta。下一阶段接入 SSE 后，会在同一事件流中逐步
// 发出 MessageDelta。
func Stream(
	ctx context.Context,
	config Config,
	userPrompt string,
) *agentcore.EventStream {
	stream := agentcore.NewEventStream(defaultEventBufferSize)

	go func() {
		defer stream.Close()

		emit := func(event agentcore.AgentEvent) error {
			return stream.Emit(ctx, event)
		}

		_, err := run(ctx, config, userPrompt, emit)
		if err == nil {
			return
		}

		// context 已取消时，事件流的消费者通常也已经停止，
		// 因此尽力发送错误事件，不再覆盖原始错误。
		_ = emit(agentcore.NewErrorEvent(0, err))
	}()

	return stream
}

// run 是同步运行和事件流运行共用的 Agent 核心循环。
func run(
	ctx context.Context,
	config Config,
	userPrompt string,
	emit eventEmitter,
) (Result, error) {
	if err := validateConfig(config, userPrompt); err != nil {
		return Result{}, err
	}

	maxTurns := config.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}

	historyLimit := config.HistoryLimit
	if historyLimit <= 0 {
		historyLimit = defaultHistoryLimit
	}

	messages := make(
		[]agentcore.Message,
		0,
		historyLimit+maxTurns*2+2,
	)

	if strings.TrimSpace(config.SystemPrompt) != "" {
		messages = append(
			messages,
			agentcore.SystemMessage(config.SystemPrompt),
		)
	}

	if config.SessionStore != nil {
		history, err := loadSessionHistory(
			ctx,
			config.SessionStore,
			config.SessionID,
			historyLimit,
		)
		if err != nil {
			return Result{}, fmt.Errorf(
				"load session history: %w",
				err,
			)
		}

		messages = append(messages, history...)
	}

	userMessage := agentcore.UserMessage(userPrompt)
	messages = append(messages, userMessage)

	if config.SessionStore != nil {
		if _, err := config.SessionStore.AppendMessage(
			ctx,
			config.SessionID,
			userMessage,
		); err != nil {
			return Result{}, fmt.Errorf(
				"persist user message: %w",
				err,
			)
		}
	}

	if err := emitEvent(
		emit,
		agentcore.NewAgentEvent(agentcore.EventAgentStart),
	); err != nil {
		return Result{}, err
	}

	var totalUsage provider.Usage
	toolCallCount := 0

	for turn := 1; turn <= maxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}

		if err := emitEvent(
			emit,
			agentcore.NewTurnEvent(
				agentcore.EventTurnStart,
				turn,
			),
		); err != nil {
			return Result{}, err
		}

		// 必须在请求 Provider 之前发送 MessageStart。
		// 否则流式 MessageDelta 会先于 MessageStart 到达消费者。
		messageStart := agentcore.NewTurnEvent(
			agentcore.EventMessageStart,
			turn,
		)

		if err := emitEvent(emit, messageStart); err != nil {
			return Result{}, err
		}

		assistantMessage, responseUsage, err := requestAssistantMessage(
			ctx,
			config.Provider,
			provider.ChatRequest{
				Messages: messages,
				Tools:    config.Tools.Definitions(),
			},
			turn,
			emit,
		)
		if err != nil {
			return Result{}, fmt.Errorf(
				"turn %d provider call failed: %w",
				turn,
				err,
			)
		}

		accumulateUsage(&totalUsage, responseUsage)

		messages = append(messages, assistantMessage)

		if config.SessionStore != nil {
			if _, err := config.SessionStore.AppendMessage(
				ctx,
				config.SessionID,
				assistantMessage,
			); err != nil {
				return Result{}, fmt.Errorf(
					"persist assistant message: %w",
					err,
				)
			}
		}

		messageEnd := agentcore.NewTurnEvent(
			agentcore.EventMessageEnd,
			turn,
		)
		messageEnd.Message = cloneMessagePointer(assistantMessage)

		if err := emitEvent(emit, messageEnd); err != nil {
			return Result{}, err
		}

		if len(assistantMessage.ToolCalls) == 0 {
			if err := emitEvent(
				emit,
				agentcore.NewTurnEvent(
					agentcore.EventTurnEnd,
					turn,
				),
			); err != nil {
				return Result{}, err
			}

			result := Result{
				Message:   assistantMessage,
				Messages:  messages,
				Turns:     turn,
				ToolCalls: toolCallCount,
				Usage:     totalUsage,
				SessionID: config.SessionID,
			}

			// agentEnd := agentcore.NewAgentEvent(
			// 	agentcore.EventAgentEnd,
			// )
			// agentEnd.Turn = turn
			// agentEnd.Message = cloneMessagePointer(
			// 	assistantMessage,
			// )
			agentEnd := agentcore.NewAgentEvent(
				agentcore.EventAgentEnd,
			)
			agentEnd.Turn = turn
			agentEnd.Message = cloneMessagePointer(
				assistantMessage,
			)

			usage := totalUsage
			agentEnd.Usage = &usage

			if err := emitEvent(emit, agentEnd); err != nil {
				return Result{}, err
			}

			return result, nil
		}

		for _, toolCall := range assistantMessage.ToolCalls {
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}

			if err := emitEvent(
				emit,
				agentcore.NewToolEvent(
					agentcore.EventToolStart,
					turn,
					toolCall,
				),
			); err != nil {
				return Result{}, err
			}

			toolResult, toolErr := config.Tools.Execute(
				ctx,
				toolCall,
			)

			toolEnd := agentcore.NewToolEvent(
				agentcore.EventToolEnd,
				turn,
				toolCall,
			)

			if toolErr != nil {
				toolEnd.Error = toolErr.Error()

				// 工具错误作为 ToolResult 回填，让模型有机会
				// 修正参数或向用户解释，而不是终止整个 Agent。
				toolResult = agentcore.ToolResultMessage(
					toolCall.ID,
					toolCall.Name,
					fmt.Sprintf(
						"工具执行失败：%v",
						toolErr,
					),
				)
			}

			toolEnd.Message = cloneMessagePointer(
				toolResult,
			)

			if err := emitEvent(emit, toolEnd); err != nil {
				return Result{}, err
			}

			messages = append(messages, toolResult)

			if config.SessionStore != nil {
				if _, err := config.SessionStore.AppendMessage(
					ctx,
					config.SessionID,
					toolResult,
				); err != nil {
					return Result{}, fmt.Errorf(
						"persist tool result: %w",
						err,
					)
				}
			}

			toolCallCount++
		}

		if err := emitEvent(
			emit,
			agentcore.NewTurnEvent(
				agentcore.EventTurnEnd,
				turn,
			),
		); err != nil {
			return Result{}, err
		}
	}

	return Result{}, fmt.Errorf(
		"agent exceeded maximum turns: %d",
		maxTurns,
	)
}

func validateConfig(
	config Config,
	userPrompt string,
) error {

	if config.Provider == nil {
		return errors.New("provider cannot be nil")
	}

	if config.Tools == nil {
		return errors.New("tool registry cannot be nil")
	}

	if strings.TrimSpace(userPrompt) == "" {
		return errors.New("user prompt cannot be empty")
	}
	hasStore := config.SessionStore != nil
	hasSessionID := strings.TrimSpace(config.SessionID) != ""

	if hasStore != hasSessionID {
		return errors.New(
			"session store and session ID must be configured together",
		)
	}

	return nil
}

func emitEvent(
	emit eventEmitter,
	event agentcore.AgentEvent,
) error {
	if emit == nil {
		return nil
	}

	return emit(event)
}

func accumulateUsage(
	total *provider.Usage,
	current provider.Usage,
) {
	total.PromptTokens += current.PromptTokens
	total.CompletionTokens += current.CompletionTokens
	total.TotalTokens += current.TotalTokens
}

func cloneMessagePointer(
	message agentcore.Message,
) *agentcore.Message {
	cloned := message
	return &cloned
}

func requestAssistantMessage(
	ctx context.Context,
	modelProvider provider.Provider,
	request provider.ChatRequest,
	turn int,
	emit eventEmitter,
) (agentcore.Message, provider.Usage, error) {
	streamingProvider, supportsStreaming :=
		modelProvider.(provider.StreamingProvider)

	// 同步 Run() 没有事件消费者时，继续使用 Chat()。
	// 这样可以避免没有消费者时仍启动流式通道。
	if !supportsStreaming || emit == nil {
		response, err := modelProvider.Chat(ctx, request)
		if err != nil {
			return agentcore.Message{}, provider.Usage{}, err
		}

		return response.Message, response.Usage, nil
	}

	streamResult, err := streamingProvider.Stream(
		ctx,
		request,
	)
	if err != nil {
		return agentcore.Message{}, provider.Usage{}, err
	}

	var finalMessage *agentcore.Message
	var usage provider.Usage

	for streamEvent := range streamResult.Events {
		switch streamEvent.Type {
		case provider.StreamEventMessageStart:
			// Runtime 已经在 Provider 调用前发出了 MessageStart，
			// Provider 自身的 message_start 不再重复映射。

		case provider.StreamEventTextDelta:
			if streamEvent.Delta == "" {
				continue
			}

			if err := emitEvent(
				emit,
				agentcore.NewMessageDeltaEvent(
					turn,
					streamEvent.Delta,
				),
			); err != nil {
				return agentcore.Message{}, provider.Usage{}, err
			}

		case provider.StreamEventToolDelta:
			// 当前 MVP 不向 CLI 逐片展示半成品工具参数。
			// 完整 ToolCall 会包含在 MessageEnd.Message 中。

		case provider.StreamEventMessageEnd:
			if streamEvent.Message == nil {
				return agentcore.Message{},
					provider.Usage{},
					errors.New(
						"stream ended without final message",
					)
			}

			cloned := *streamEvent.Message
			finalMessage = &cloned
			usage = streamEvent.Usage

		case provider.StreamEventError:
			if streamEvent.Err == nil {
				return agentcore.Message{},
					provider.Usage{},
					errors.New(
						"provider stream returned empty error",
					)
			}

			return agentcore.Message{},
				provider.Usage{},
				streamEvent.Err

		default:
			return agentcore.Message{},
				provider.Usage{},
				fmt.Errorf(
					"unsupported provider stream event %q",
					streamEvent.Type,
				)
		}
	}

	if finalMessage == nil {
		return agentcore.Message{},
			provider.Usage{},
			errors.New(
				"provider stream closed without message_end",
			)
	}

	return *finalMessage, usage, nil
}
func loadSessionHistory(
	ctx context.Context,
	store session.Store,
	sessionID string,
	limit int,
) ([]agentcore.Message, error) {
	if _, err := store.GetSession(ctx, sessionID); err != nil {
		return nil, err
	}

	storedMessages, err := store.ListMessages(
		ctx,
		sessionID,
		limit,
	)
	if err != nil {
		return nil, err
	}

	history := make(
		[]agentcore.Message,
		0,
		len(storedMessages),
	)

	for _, stored := range storedMessages {
		history = append(history, stored.Message)
	}

	return history, nil
}
