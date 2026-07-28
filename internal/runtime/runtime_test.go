package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
	"github.com/chenchangchao/go-pi-agent/internal/agenttool"
	"github.com/chenchangchao/go-pi-agent/internal/provider"
)

type fakeProvider struct {
	responses []provider.ChatResponse
	requests  []provider.ChatRequest
	callIndex int
}

func (fake *fakeProvider) Chat(
	_ context.Context,
	request provider.ChatRequest,
) (provider.ChatResponse, error) {
	fake.requests = append(fake.requests, request)

	if fake.callIndex >= len(fake.responses) {
		return provider.ChatResponse{}, errors.New(
			"fake provider has no more responses",
		)
	}

	response := fake.responses[fake.callIndex]
	fake.callIndex++

	return response, nil
}

func TestRunExecutesToolAndReturnsFinalAnswer(t *testing.T) {
	rootDir := t.TempDir()

	filePath := filepath.Join(rootDir, "README.md")
	if err := os.WriteFile(
		filePath,
		[]byte("# Go Pi Agent\n"),
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
							ID:   "call_001",
							Name: "read_file",
							Arguments: []byte(
								`{"path":"README.md"}`,
							),
						},
					},
				},
				FinishReason: "tool_calls",
				Usage: provider.Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			{
				Message: agentcore.AssistantMessage(
					"这是一个使用 Go 开发的 Agent 项目。",
				),
				FinishReason: "stop",
				Usage: provider.Usage{
					PromptTokens:     20,
					CompletionTokens: 10,
					TotalTokens:      30,
				},
			},
		},
	}

	result, err := Run(
		context.Background(),
		Config{
			Provider:     modelProvider,
			Tools:        registry,
			SystemPrompt: "你是一个测试 Agent。",
			MaxTurns:     4,
		},
		"读取 README.md 并总结",
	)
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	if result.Turns != 2 {
		t.Fatalf(
			"expected 2 turns, got %d",
			result.Turns,
		)
	}

	if result.ToolCalls != 1 {
		t.Fatalf(
			"expected 1 tool call, got %d",
			result.ToolCalls,
		)
	}

	if result.Message.Content !=
		"这是一个使用 Go 开发的 Agent 项目。" {
		t.Fatalf(
			"unexpected final answer: %q",
			result.Message.Content,
		)
	}

	if result.Usage.TotalTokens != 45 {
		t.Fatalf(
			"expected total token usage 45, got %d",
			result.Usage.TotalTokens,
		)
	}

	if len(modelProvider.requests) != 2 {
		t.Fatalf(
			"expected 2 provider requests, got %d",
			len(modelProvider.requests),
		)
	}

	secondRequest := modelProvider.requests[1]

	var foundToolResult bool

	for _, message := range secondRequest.Messages {
		if message.Role == agentcore.RoleTool &&
			message.ToolCallID == "call_001" &&
			strings.Contains(
				message.Content,
				"# Go Pi Agent",
			) {
			foundToolResult = true
			break
		}
	}

	if !foundToolResult {
		t.Fatal(
			"expected second provider request to contain tool result",
		)
	}
}

func TestRunReturnsDirectAnswerWithoutToolCall(t *testing.T) {
	registry := agenttool.NewRegistry()

	modelProvider := &fakeProvider{
		responses: []provider.ChatResponse{
			{
				Message: agentcore.AssistantMessage(
					"Go 是一种编译型语言。",
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
		"介绍 Go",
	)
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	if result.Turns != 1 {
		t.Fatalf(
			"expected 1 turn, got %d",
			result.Turns,
		)
	}

	if result.ToolCalls != 0 {
		t.Fatalf(
			"expected 0 tool calls, got %d",
			result.ToolCalls,
		)
	}
}

func TestRunStopsAfterMaximumTurns(t *testing.T) {
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
			{
				Message: agentcore.Message{
					Role: agentcore.RoleAssistant,
					ToolCalls: []agentcore.ToolCall{
						{
							ID:        "call_002",
							Name:      "unknown_tool",
							Arguments: []byte(`{}`),
						},
					},
				},
			},
		},
	}

	_, err := Run(
		context.Background(),
		Config{
			Provider: modelProvider,
			Tools:    registry,
			MaxTurns: 2,
		},
		"不断调用工具",
	)
	if err == nil {
		t.Fatal("expected maximum turns error")
	}

	if !strings.Contains(
		err.Error(),
		"exceeded maximum turns",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}
