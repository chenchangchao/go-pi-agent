package provider

import (
	"context"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

// ChatRequest 是 Runtime 发给模型 Provider 的统一请求。
type ChatRequest struct {
	Messages []agentcore.Message
	Tools    []agentcore.ToolDefinition
}

// ChatResponse 是 Provider 返回给 Runtime 的统一结果。
type ChatResponse struct {
	Message      agentcore.Message
	FinishReason string
	Usage        Usage
}

// Usage 记录模型本次请求的 Token 用量。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Provider 是所有模型供应商必须实现的统一接口。
type Provider interface {
	Chat(
		ctx context.Context,
		request ChatRequest,
	) (ChatResponse, error)
}
