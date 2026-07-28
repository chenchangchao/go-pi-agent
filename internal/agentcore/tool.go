package agentcore

import (
	"context"
	"encoding/json"
)

// ToolCall 表示模型发起的一次工具调用。
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolDefinition 是发送给模型的工具描述。
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Tool 是所有 Agent 工具必须实现的统一接口。
type Tool interface {
	// Definition 返回工具名称、描述和 JSON Schema。
	Definition() ToolDefinition

	// Execute 执行工具调用。
	Execute(ctx context.Context, arguments json.RawMessage) (string, error)
}
