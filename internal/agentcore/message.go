package agentcore

// Role 表示一条消息在对话中的角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 是 Agent 内部统一使用的消息结构。
//
// 第一版 MVP 先采用一个简单结构，后续再扩展为：
//   - 多模态 ContentBlock
//   - 流式事件
//   - Token 用量
//   - 停止原因
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// SystemMessage 创建一条系统消息。
func SystemMessage(content string) Message {
	return Message{
		Role:    RoleSystem,
		Content: content,
	}
}

// UserMessage 创建一条用户消息。
func UserMessage(content string) Message {
	return Message{
		Role:    RoleUser,
		Content: content,
	}
}

// AssistantMessage 创建一条普通助手消息。
func AssistantMessage(content string) Message {
	return Message{
		Role:    RoleAssistant,
		Content: content,
	}
}

// ToolResultMessage 创建一条工具执行结果消息。
func ToolResultMessage(toolCallID, name, content string) Message {
	return Message{
		Role:       RoleTool,
		ToolCallID: toolCallID,
		Name:       name,
		Content:    content,
	}
}
