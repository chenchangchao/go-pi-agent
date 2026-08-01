package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
	"github.com/chenchangchao/go-pi-agent/internal/agenttool"
	"github.com/chenchangchao/go-pi-agent/internal/provider"
)

func TestStreamEmitsAgentLifecycleEvents(t *testing.T) {
	rootDir := t.TempDir()

	filePath := filepath.Join(rootDir, "README.md")
	if err := os.WriteFile(
		filePath,
		[]byte("# Streaming Agent\n"),
		0o600,
	); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	readFileTool, err := agenttool.NewReadFileTool(rootDir)
	if err != nil {
		t.Fatalf("create read_file tool: %v", err)
	}

	registry := agenttool.NewRegistry()
	if err := registry.Register(readFileTool); err != nil {
		t.Fatalf("register read_file tool: %v", err)
	}

	modelProvider := &fakeProvider{
		responses: []provider.ChatResponse{
			{
				Message: agentcore.Message{
					Role: agentcore.RoleAssistant,
					ToolCalls: []agentcore.ToolCall{
						{
							ID:   "call_stream_001",
							Name: "read_file",
							Arguments: []byte(
								`{"path":"README.md"}`,
							),
						},
					},
				},
				FinishReason: "tool_calls",
			},
			{
				Message: agentcore.AssistantMessage(
					"这是一个支持事件流的 Go Agent。",
				),
				FinishReason: "stop",
			},
		},
	}

	stream := Stream(
		context.Background(),
		Config{
			Provider: modelProvider,
			Tools:    registry,
			MaxTurns: 4,
		},
		"读取 README.md 并总结",
	)

	var events []agentcore.AgentEvent

	for event := range stream.Events() {
		events = append(events, event)
	}

	expectedTypes := []agentcore.EventType{
		agentcore.EventAgentStart,

		agentcore.EventTurnStart,
		agentcore.EventMessageStart,
		agentcore.EventMessageEnd,
		agentcore.EventToolStart,
		agentcore.EventToolEnd,
		agentcore.EventTurnEnd,

		agentcore.EventTurnStart,
		agentcore.EventMessageStart,
		agentcore.EventMessageEnd,
		agentcore.EventTurnEnd,

		agentcore.EventAgentEnd,
	}

	if len(events) != len(expectedTypes) {
		t.Fatalf(
			"expected %d events, got %d: %#v",
			len(expectedTypes),
			len(events),
			eventTypes(events),
		)
	}

	for index, expectedType := range expectedTypes {
		if events[index].Type != expectedType {
			t.Fatalf(
				"event %d: expected %q, got %q",
				index,
				expectedType,
				events[index].Type,
			)
		}
	}

	toolStart := events[4]
	if toolStart.ToolCall == nil {
		t.Fatal("expected tool_start to contain tool call")
	}

	if toolStart.ToolCall.Name != "read_file" {
		t.Fatalf(
			"expected read_file tool, got %q",
			toolStart.ToolCall.Name,
		)
	}

	toolEnd := events[5]
	if toolEnd.Message == nil {
		t.Fatal("expected tool_end to contain result message")
	}

	agentEnd := events[len(events)-1]
	if agentEnd.Message == nil {
		t.Fatal("expected agent_end to contain final message")
	}

	if agentEnd.Message.Content !=
		"这是一个支持事件流的 Go Agent。" {
		t.Fatalf(
			"unexpected final content: %q",
			agentEnd.Message.Content,
		)
	}
}

func TestStreamEmitsErrorEvent(t *testing.T) {
	registry := agenttool.NewRegistry()

	modelProvider := &fakeProvider{
		responses: []provider.ChatResponse{
			{
				Message: agentcore.Message{
					Role: agentcore.RoleAssistant,
					ToolCalls: []agentcore.ToolCall{
						{
							ID:        "call_001",
							Name:      "unknown_tool",
							Arguments: []byte(`{}`),
						},
					},
				},
			},
		},
	}

	stream := Stream(
		context.Background(),
		Config{
			Provider: modelProvider,
			Tools:    registry,
			MaxTurns: 1,
		},
		"一直调用不存在的工具",
	)

	var lastEvent agentcore.AgentEvent

	for event := range stream.Events() {
		lastEvent = event
	}

	if lastEvent.Type != agentcore.EventError {
		t.Fatalf(
			"expected last event %q, got %q",
			agentcore.EventError,
			lastEvent.Type,
		)
	}

	if lastEvent.Error == "" {
		t.Fatal("expected error event to contain error message")
	}
}

func TestRunStillWorksWithoutEvents(t *testing.T) {
	registry := agenttool.NewRegistry()

	modelProvider := &fakeProvider{
		responses: []provider.ChatResponse{
			{
				Message: agentcore.AssistantMessage(
					"同步接口仍然可用。",
				),
				FinishReason: "stop",
			},
		},
	}

	result, err := Run(
		context.Background(),
		Config{
			Provider: modelProvider,
			Tools:    registry,
		},
		"测试同步接口",
	)
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	if result.Message.Content != "同步接口仍然可用。" {
		t.Fatalf(
			"unexpected result: %q",
			result.Message.Content,
		)
	}
}

func eventTypes(
	events []agentcore.AgentEvent,
) []agentcore.EventType {
	types := make(
		[]agentcore.EventType,
		0,
		len(events),
	)

	for _, event := range events {
		types = append(types, event.Type)
	}

	return types
}
