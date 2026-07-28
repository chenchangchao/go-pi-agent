package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
	"github.com/chenchangchao/go-pi-agent/internal/agenttool"
	"github.com/chenchangchao/go-pi-agent/internal/provider"
)

const defaultMaxTurns = 8

// Config 定义一次 Agent 运行所需的依赖和限制。
type Config struct {
	Provider     provider.Provider
	Tools        *agenttool.Registry
	SystemPrompt string
	MaxTurns     int
}

// Result 是一次 Agent 运行的最终结果。
type Result struct {
	Message   agentcore.Message
	Messages  []agentcore.Message
	Turns     int
	ToolCalls int
	Usage     provider.Usage
}

// Run 执行最小 Agent 循环。
func Run(
	ctx context.Context,
	config Config,
	userPrompt string,
) (Result, error) {
	if config.Provider == nil {
		return Result{}, errors.New("provider cannot be nil")
	}

	if config.Tools == nil {
		return Result{}, errors.New("tool registry cannot be nil")
	}

	maxTurns := config.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}

	messages := make([]agentcore.Message, 0, maxTurns*2+2)

	if config.SystemPrompt != "" {
		messages = append(
			messages,
			agentcore.SystemMessage(config.SystemPrompt),
		)
	}

	messages = append(
		messages,
		agentcore.UserMessage(userPrompt),
	)

	var totalUsage provider.Usage
	toolCallCount := 0

	for turn := 1; turn <= maxTurns; turn++ {
		response, err := config.Provider.Chat(
			ctx,
			provider.ChatRequest{
				Messages: messages,
				Tools:    config.Tools.Definitions(),
			},
		)
		if err != nil {
			return Result{}, fmt.Errorf(
				"turn %d provider call failed: %w",
				turn,
				err,
			)
		}

		totalUsage.PromptTokens += response.Usage.PromptTokens
		totalUsage.CompletionTokens += response.Usage.CompletionTokens
		totalUsage.TotalTokens += response.Usage.TotalTokens

		assistantMessage := response.Message
		messages = append(messages, assistantMessage)

		if len(assistantMessage.ToolCalls) == 0 {
			return Result{
				Message:   assistantMessage,
				Messages:  messages,
				Turns:     turn,
				ToolCalls: toolCallCount,
				Usage:     totalUsage,
			}, nil
		}

		for _, toolCall := range assistantMessage.ToolCalls {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			default:
			}

			toolResult, err := config.Tools.Execute(ctx, toolCall)
			if err != nil {
				// 工具错误作为消息回填，让模型有机会修正参数，
				// 而不是直接终止整个 Agent。
				toolResult = agentcore.ToolResultMessage(
					toolCall.ID,
					toolCall.Name,
					fmt.Sprintf("工具执行失败：%v", err),
				)
			}

			messages = append(messages, toolResult)
			toolCallCount++
		}
	}

	return Result{}, fmt.Errorf(
		"agent exceeded maximum turns: %d",
		maxTurns,
	)
}
