package agentcore

import (
	"context"
	"sync"
)

// EventStream 是 Agent 事件的异步传输通道。
//
// Producer 负责 Emit 和 Close。
// Consumer 通过 Events 只读消费事件。
type EventStream struct {
	events    chan AgentEvent
	closeOnce sync.Once
}

// NewEventStream 创建事件流。
func NewEventStream(bufferSize int) *EventStream {
	if bufferSize < 0 {
		bufferSize = 0
	}

	return &EventStream{
		events: make(chan AgentEvent, bufferSize),
	}
}

// Events 返回只读事件通道。
func (stream *EventStream) Events() <-chan AgentEvent {
	return stream.events
}

// Emit 向事件流发送一个事件。
//
// 如果 context 被取消，返回 context 错误。
func (stream *EventStream) Emit(
	ctx context.Context,
	event AgentEvent,
) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case stream.events <- event:
		return nil
	}
}

// Close 关闭事件流。
//
// 重复调用是安全的。
func (stream *EventStream) Close() {
	stream.closeOnce.Do(func() {
		close(stream.events)
	})
}
