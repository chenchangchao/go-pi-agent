package agentcore

import "time"

// EventType 表示 Agent Runtime 对外发送的事件类型。
type EventType string

const (
	EventAgentStart   EventType = "agent_start"
	EventAgentEnd     EventType = "agent_end"
	EventTurnStart    EventType = "turn_start"
	EventTurnEnd      EventType = "turn_end"
	EventMessageStart EventType = "message_start"
	EventMessageDelta EventType = "message_delta"
	EventMessageEnd   EventType = "message_end"
	EventToolStart    EventType = "tool_start"
	EventToolEnd      EventType = "tool_end"
	EventError        EventType = "error"
)

// AgentEvent 是 Runtime 对外暴露的统一事件。
//
// MVP 阶段使用一个简单结构，后续可以继续扩展：
//   - session_id
//   - trace_id
//   - run_id
//   - duration
//   - usage
//   - error code
type AgentEvent struct {
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Turn      int       `json:"turn,omitempty"`
	Delta     string    `json:"delta,omitempty"`
	Message   *Message  `json:"message,omitempty"`
	ToolCall  *ToolCall `json:"tool_call,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// NewAgentEvent 创建一个带时间戳的事件。
func NewAgentEvent(eventType EventType) AgentEvent {
	return AgentEvent{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
	}
}

// NewTurnEvent 创建与某一轮执行相关的事件。
func NewTurnEvent(eventType EventType, turn int) AgentEvent {
	event := NewAgentEvent(eventType)
	event.Turn = turn
	return event
}

// NewMessageDeltaEvent 创建模型增量文本事件。
func NewMessageDeltaEvent(turn int, delta string) AgentEvent {
	event := NewTurnEvent(EventMessageDelta, turn)
	event.Delta = delta
	return event
}

// NewToolEvent 创建工具执行相关事件。
func NewToolEvent(
	eventType EventType,
	turn int,
	toolCall ToolCall,
) AgentEvent {
	event := NewTurnEvent(eventType, turn)
	event.ToolCall = &toolCall
	return event
}

// NewErrorEvent 创建错误事件。
func NewErrorEvent(turn int, err error) AgentEvent {
	event := NewTurnEvent(EventError, turn)

	if err != nil {
		event.Error = err.Error()
	}

	return event
}
