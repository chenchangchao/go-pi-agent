package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

const defaultStreamBufferSize = 32

// Stream 调用 OpenAI-compatible Chat Completions SSE 接口。
func (provider *OpenAICompatibleProvider) Stream(
	ctx context.Context,
	request ChatRequest,
) (StreamResult, error) {
	apiRequest, err := provider.buildRequest(request)
	if err != nil {
		return StreamResult{}, err
	}

	apiRequest.Stream = true

	streamRequest := chatCompletionStreamRequest{
		chatCompletionRequest: apiRequest,
		StreamOptions: &chatCompletionStreamOptions{
			IncludeUsage: true,
		},
	}

	requestBody, err := json.Marshal(streamRequest)
	if err != nil {
		return StreamResult{}, fmt.Errorf(
			"encode streaming chat request: %w",
			err,
		)
	}

	endpoint := provider.baseURL + "/chat/completions"

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return StreamResult{}, fmt.Errorf(
			"create streaming HTTP request: %w",
			err,
		)
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")

	if provider.apiKey != "" {
		httpRequest.Header.Set(
			"Authorization",
			"Bearer "+provider.apiKey,
		)
	}

	httpResponse, err := provider.httpClient.Do(httpRequest)
	if err != nil {
		return StreamResult{}, fmt.Errorf(
			"send streaming chat request: %w",
			err,
		)
	}

	if httpResponse.StatusCode < 200 ||
		httpResponse.StatusCode >= 300 {
		defer httpResponse.Body.Close()

		responseBody, readErr := io.ReadAll(
			io.LimitReader(httpResponse.Body, 10*1024*1024),
		)
		if readErr != nil {
			return StreamResult{}, fmt.Errorf(
				"read streaming API error: %w",
				readErr,
			)
		}

		return StreamResult{}, decodeAPIError(
			httpResponse.StatusCode,
			responseBody,
		)
	}

	events := make(chan StreamEvent, defaultStreamBufferSize)

	go provider.consumeSSE(
		ctx,
		httpResponse.Body,
		events,
	)

	return StreamResult{
		Events: events,
	}, nil
}

func (provider *OpenAICompatibleProvider) consumeSSE(
	ctx context.Context,
	body io.ReadCloser,
	events chan<- StreamEvent,
) {
	defer close(events)
	defer body.Close()

	send := func(event StreamEvent) bool {
		select {
		case <-ctx.Done():
			return false
		case events <- event:
			return true
		}
	}

	if !send(StreamEvent{
		Type: StreamEventMessageStart,
	}) {
		return
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(
		make([]byte, 64*1024),
		2*1024*1024,
	)

	accumulator := newStreamAccumulator()

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			send(StreamEvent{
				Type: StreamEventError,
				Err:  err,
			})
			return
		}

		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(
			strings.TrimPrefix(line, "data:"),
		)

		if data == "[DONE]" {
			message, err := accumulator.message()
			if err != nil {
				send(StreamEvent{
					Type: StreamEventError,
					Err:  err,
				})
				return
			}

			send(StreamEvent{
				Type:         StreamEventMessageEnd,
				Message:      &message,
				FinishReason: accumulator.finishReason,
				Usage:        accumulator.usage,
			})
			return
		}

		var chunk chatCompletionStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			send(StreamEvent{
				Type: StreamEventError,
				Err: fmt.Errorf(
					"decode SSE chunk: %w",
					err,
				),
			})
			return
		}

		deltaEvents, err := accumulator.apply(chunk)
		if err != nil {
			send(StreamEvent{
				Type: StreamEventError,
				Err:  err,
			})
			return
		}

		for _, event := range deltaEvents {
			if !send(event) {
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		send(StreamEvent{
			Type: StreamEventError,
			Err: fmt.Errorf(
				"read SSE stream: %w",
				err,
			),
		})
		return
	}

	// 少数兼容 Provider 可能不发送 [DONE]，但正常关闭连接。
	message, err := accumulator.message()
	if err != nil {
		send(StreamEvent{
			Type: StreamEventError,
			Err:  err,
		})
		return
	}

	send(StreamEvent{
		Type:         StreamEventMessageEnd,
		Message:      &message,
		FinishReason: accumulator.finishReason,
		Usage:        accumulator.usage,
	})
}

type streamAccumulator struct {
	content      strings.Builder
	toolCalls    map[int]*toolCallAccumulator
	finishReason string
	usage        Usage
}

type toolCallAccumulator struct {
	id        string
	name      string
	arguments strings.Builder
}

func newStreamAccumulator() *streamAccumulator {
	return &streamAccumulator{
		toolCalls: make(map[int]*toolCallAccumulator),
	}
}

func (accumulator *streamAccumulator) apply(
	chunk chatCompletionStreamChunk,
) ([]StreamEvent, error) {
	accumulator.usage = Usage{
		PromptTokens:     chunk.Usage.PromptTokens,
		CompletionTokens: chunk.Usage.CompletionTokens,
		TotalTokens:      chunk.Usage.TotalTokens,
	}

	if len(chunk.Choices) == 0 {
		return nil, nil
	}

	choice := chunk.Choices[0]

	if choice.FinishReason != "" {
		accumulator.finishReason = choice.FinishReason
	}

	var events []StreamEvent

	if choice.Delta.Content != "" {
		accumulator.content.WriteString(
			choice.Delta.Content,
		)

		events = append(events, StreamEvent{
			Type:  StreamEventTextDelta,
			Delta: choice.Delta.Content,
		})
	}

	for _, delta := range choice.Delta.ToolCalls {
		call, exists := accumulator.toolCalls[delta.Index]
		if !exists {
			call = &toolCallAccumulator{}
			accumulator.toolCalls[delta.Index] = call
		}

		if delta.ID != "" {
			call.id = delta.ID
		}

		if delta.Function.Name != "" {
			call.name += delta.Function.Name
		}

		if delta.Function.Arguments != "" {
			call.arguments.WriteString(
				delta.Function.Arguments,
			)
		}

		current, err := call.snapshot()
		if err != nil {
			// 参数仍可能是半截 JSON；此时不视为错误。
			continue
		}

		events = append(events, StreamEvent{
			Type:     StreamEventToolDelta,
			ToolCall: &current,
		})
	}

	return events, nil
}

func (accumulator *streamAccumulator) message() (
	agentcore.Message,
	error,
) {
	message := agentcore.Message{
		Role:    agentcore.RoleAssistant,
		Content: accumulator.content.String(),
	}

	for index := 0; index < len(accumulator.toolCalls); index++ {
		call, exists := accumulator.toolCalls[index]
		if !exists {
			continue
		}

		toolCall, err := call.finalize()
		if err != nil {
			return agentcore.Message{}, fmt.Errorf(
				"finalize tool call at index %d: %w",
				index,
				err,
			)
		}

		message.ToolCalls = append(
			message.ToolCalls,
			toolCall,
		)
	}

	return message, nil
}

func (call *toolCallAccumulator) snapshot() (
	agentcore.ToolCall,
	error,
) {
	arguments := strings.TrimSpace(
		call.arguments.String(),
	)

	if arguments == "" {
		arguments = "{}"
	}

	if !json.Valid([]byte(arguments)) {
		return agentcore.ToolCall{}, errors.New(
			"tool arguments are incomplete",
		)
	}

	return agentcore.ToolCall{
		ID:        call.id,
		Name:      call.name,
		Arguments: json.RawMessage(arguments),
	}, nil
}

func (call *toolCallAccumulator) finalize() (
	agentcore.ToolCall,
	error,
) {
	if strings.TrimSpace(call.id) == "" {
		return agentcore.ToolCall{}, errors.New(
			"tool call ID cannot be empty",
		)
	}

	if strings.TrimSpace(call.name) == "" {
		return agentcore.ToolCall{}, errors.New(
			"tool call name cannot be empty",
		)
	}

	arguments := strings.TrimSpace(
		call.arguments.String(),
	)
	if arguments == "" {
		arguments = "{}"
	}

	if !json.Valid([]byte(arguments)) {
		return agentcore.ToolCall{}, fmt.Errorf(
			"tool %q returned invalid JSON arguments: %s",
			call.name,
			arguments,
		)
	}

	return agentcore.ToolCall{
		ID:        call.id,
		Name:      call.name,
		Arguments: json.RawMessage(arguments),
	}, nil
}

type chatCompletionStreamRequest struct {
	chatCompletionRequest
	StreamOptions *chatCompletionStreamOptions `json:"stream_options,omitempty"`
}

type chatCompletionStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatCompletionStreamChunk struct {
	Choices []chatCompletionStreamChoice `json:"choices"`
	Usage   chatCompletionUsage          `json:"usage"`
}

type chatCompletionStreamChoice struct {
	Delta        chatCompletionStreamDelta `json:"delta"`
	FinishReason string                    `json:"finish_reason"`
}

type chatCompletionStreamDelta struct {
	Role      string                         `json:"role"`
	Content   string                         `json:"content"`
	ToolCalls []chatCompletionStreamToolCall `json:"tool_calls"`
}

type chatCompletionStreamToolCall struct {
	Index    int                            `json:"index"`
	ID       string                         `json:"id"`
	Type     string                         `json:"type"`
	Function chatCompletionToolCallFunction `json:"function"`
}
