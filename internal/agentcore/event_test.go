package agentcore

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewAgentEvent(t *testing.T) {
	before := time.Now().UTC()

	event := NewAgentEvent(EventAgentStart)

	after := time.Now().UTC()

	if event.Type != EventAgentStart {
		t.Fatalf(
			"expected event type %q, got %q",
			EventAgentStart,
			event.Type,
		)
	}

	if event.Timestamp.Before(before) {
		t.Fatalf(
			"expected timestamp after %v, got %v",
			before,
			event.Timestamp,
		)
	}

	if event.Timestamp.After(after) {
		t.Fatalf(
			"expected timestamp before %v, got %v",
			after,
			event.Timestamp,
		)
	}
}

func TestNewMessageDeltaEvent(t *testing.T) {
	event := NewMessageDeltaEvent(
		2,
		"Hello",
	)

	if event.Type != EventMessageDelta {
		t.Fatalf(
			"expected type %q, got %q",
			EventMessageDelta,
			event.Type,
		)
	}

	if event.Turn != 2 {
		t.Fatalf(
			"expected turn 2, got %d",
			event.Turn,
		)
	}

	if event.Delta != "Hello" {
		t.Fatalf(
			"expected delta %q, got %q",
			"Hello",
			event.Delta,
		)
	}
}

func TestNewToolEvent(t *testing.T) {
	call := ToolCall{
		ID:   "call_001",
		Name: "read_file",
		Arguments: []byte(
			`{"path":"README.md"}`,
		),
	}

	event := NewToolEvent(
		EventToolStart,
		1,
		call,
	)

	if event.Type != EventToolStart {
		t.Fatalf(
			"expected type %q, got %q",
			EventToolStart,
			event.Type,
		)
	}

	if event.ToolCall == nil {
		t.Fatal("expected tool call")
	}

	if event.ToolCall.Name != "read_file" {
		t.Fatalf(
			"expected tool name %q, got %q",
			"read_file",
			event.ToolCall.Name,
		)
	}
}

func TestNewErrorEvent(t *testing.T) {
	event := NewErrorEvent(
		3,
		errors.New("provider failed"),
	)

	if event.Type != EventError {
		t.Fatalf(
			"expected type %q, got %q",
			EventError,
			event.Type,
		)
	}

	if event.Turn != 3 {
		t.Fatalf(
			"expected turn 3, got %d",
			event.Turn,
		)
	}

	if event.Error != "provider failed" {
		t.Fatalf(
			"unexpected error message: %q",
			event.Error,
		)
	}
}

func TestEventStreamEmitAndClose(t *testing.T) {
	stream := NewEventStream(2)

	ctx := context.Background()

	first := NewAgentEvent(EventAgentStart)
	second := NewTurnEvent(EventTurnStart, 1)

	if err := stream.Emit(ctx, first); err != nil {
		t.Fatalf("emit first event: %v", err)
	}

	if err := stream.Emit(ctx, second); err != nil {
		t.Fatalf("emit second event: %v", err)
	}

	stream.Close()
	stream.Close()

	events := make([]AgentEvent, 0, 2)

	for event := range stream.Events() {
		events = append(events, event)
	}

	if len(events) != 2 {
		t.Fatalf(
			"expected 2 events, got %d",
			len(events),
		)
	}

	if events[0].Type != EventAgentStart {
		t.Fatalf(
			"unexpected first event: %q",
			events[0].Type,
		)
	}

	if events[1].Type != EventTurnStart {
		t.Fatalf(
			"unexpected second event: %q",
			events[1].Type,
		)
	}
}

func TestEventStreamReturnsContextError(t *testing.T) {
	stream := NewEventStream(0)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err := stream.Emit(
		ctx,
		NewAgentEvent(EventAgentStart),
	)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}
