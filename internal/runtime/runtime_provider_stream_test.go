package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
	"github.com/chenchangchao/go-pi-agent/internal/agenttool"
	"github.com/chenchangchao/go-pi-agent/internal/provider"
)

type fakeStreamingProvider struct {
	chatCalled   bool
	streamCalled bool
	events       []provider.StreamEvent
}

func (fake *fakeStreamingProvider) Chat(
	_ context.Context,
	_ provider.ChatRequest,
) (provider.ChatResponse, error) {
	fake.chatCalled = true

	return provider.ChatResponse{
		Message: agentcore.AssistantMessage(
			"同步回答",
		),
	}, nil
}

func (fake *fakeStreamingProvider) Stream(
	_ context.Context,
	_ provider.ChatRequest,
) (provider.StreamResult, error) {
	fake.streamCalled = true

	events := make(chan provider.StreamEvent, len(fake.events))

	for _, event := range fake.events {
		events <- event
	}

	close(events)

	return provider.StreamResult{
		Events: events,
	}, nil
}

func TestRuntimeStreamUsesStreamingProvider(t *testing.T) {
	registry := agenttool.NewRegistry()

	finalMessage := agentcore.AssistantMessage(
		"你好，Go Agent",
	)

	modelProvider := &fakeStreamingProvider{
		events: []provider.StreamEvent{
			{
				Type: provider.StreamEventMessageStart,
			},
			{
				Type:  provider.StreamEventTextDelta,
				Delta: "你好",
			},
			{
				Type:  provider.StreamEventTextDelta,
				Delta: "，Go Agent",
			},
			{
				Type:    provider.StreamEventMessageEnd,
				Message: &finalMessage,
				Usage: provider.Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
		},
	}

	stream := Stream(
		context.Background(),
		Config{
			Provider: modelProvider,
			Tools:    registry,
		},
		"你好",
	)

	var deltas strings.Builder
	var eventTypes []agentcore.EventType

	for event := range stream.Events() {
		eventTypes = append(eventTypes, event.Type)

		if event.Type == agentcore.EventMessageDelta {
			deltas.WriteString(event.Delta)
		}
	}

	if !modelProvider.streamCalled {
		t.Fatal("expected StreamingProvider.Stream to be called")
	}

	if modelProvider.chatCalled {
		t.Fatal("did not expect Provider.Chat to be called")
	}

	if deltas.String() != "你好，Go Agent" {
		t.Fatalf(
			"unexpected deltas: %q",
			deltas.String(),
		)
	}

	expectedOrder := []agentcore.EventType{
		agentcore.EventAgentStart,
		agentcore.EventTurnStart,
		agentcore.EventMessageStart,
		agentcore.EventMessageDelta,
		agentcore.EventMessageDelta,
		agentcore.EventMessageEnd,
		agentcore.EventTurnEnd,
		agentcore.EventAgentEnd,
	}

	if len(eventTypes) != len(expectedOrder) {
		t.Fatalf(
			"expected %d events, got %d: %#v",
			len(expectedOrder),
			len(eventTypes),
			eventTypes,
		)
	}

	for index, expected := range expectedOrder {
		if eventTypes[index] != expected {
			t.Fatalf(
				"event %d: expected %q, got %q",
				index,
				expected,
				eventTypes[index],
			)
		}
	}
}

func TestRuntimeRunUsesSynchronousChat(t *testing.T) {
	registry := agenttool.NewRegistry()

	modelProvider := &fakeStreamingProvider{
		events: nil,
	}

	result, err := Run(
		context.Background(),
		Config{
			Provider: modelProvider,
			Tools:    registry,
		},
		"同步问题",
	)
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	if !modelProvider.chatCalled {
		t.Fatal("expected synchronous Chat to be called")
	}

	if modelProvider.streamCalled {
		t.Fatal("did not expect Stream to be called")
	}

	if result.Message.Content != "同步回答" {
		t.Fatalf(
			"unexpected answer: %q",
			result.Message.Content,
		)
	}
}
