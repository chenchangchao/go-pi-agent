package provider

import (
	"context"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

// StreamEventType 表示模型 Provider 产生的增量事件类型。
type StreamEventType string

const (
	StreamEventMessageStart StreamEventType = "message_start"
	StreamEventTextDelta    StreamEventType = "text_delta"
	StreamEventToolDelta    StreamEventType = "tool_delta"
	StreamEventMessageEnd   StreamEventType = "message_end"
	StreamEventError        StreamEventType = "error"
)

// StreamEvent 是 Provider 流式响应的统一事件。
type StreamEvent struct {
	Type StreamEventType

	// Delta 仅用于文本增量。
	Delta string

	// ToolCall 用于已经完成或正在累积的工具调用。
	ToolCall *agentcore.ToolCall

	// Message 在 message_end 时包含完整助手消息。
	Message *agentcore.Message

	FinishReason string
	Usage        Usage
	Err          error
}

// StreamResult 包含模型流式响应事件通道。
type StreamResult struct {
	Events <-chan StreamEvent
}

// StreamingProvider 是支持增量流式输出的模型 Provider。
//
// 保留 Provider 接口是为了兼容已有同步调用与测试。
type StreamingProvider interface {
	Provider

	Stream(
		ctx context.Context,
		request ChatRequest,
	) (StreamResult, error)
}
