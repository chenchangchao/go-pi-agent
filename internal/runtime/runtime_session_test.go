package runtime

import (
	"context"
	"testing"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
	"github.com/chenchangchao/go-pi-agent/internal/agenttool"
	"github.com/chenchangchao/go-pi-agent/internal/provider"
	"github.com/chenchangchao/go-pi-agent/internal/session"
)

type sessionAwareProvider struct {
	requests  []provider.ChatRequest
	responses []provider.ChatResponse
}

func (fake *sessionAwareProvider) Chat(
	_ context.Context,
	request provider.ChatRequest,
) (provider.ChatResponse, error) {
	fake.requests = append(fake.requests, request)

	response := fake.responses[0]
	fake.responses = fake.responses[1:]

	return response, nil
}

func TestRuntimeLoadsAndPersistsSessionHistory(t *testing.T) {
	ctx := context.Background()
	store := session.NewMemoryStore()

	currentSession, err := store.CreateSession(
		ctx,
		session.Session{
			Title: "Runtime session test",
		},
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if _, err := store.AppendMessage(
		ctx,
		currentSession.ID,
		agentcore.UserMessage("我叫 Dust Chen"),
	); err != nil {
		t.Fatalf("append historical user message: %v", err)
	}

	if _, err := store.AppendMessage(
		ctx,
		currentSession.ID,
		agentcore.AssistantMessage("你好，Dust Chen"),
	); err != nil {
		t.Fatalf("append historical assistant message: %v", err)
	}

	modelProvider := &sessionAwareProvider{
		responses: []provider.ChatResponse{
			{
				Message: agentcore.AssistantMessage(
					"你之前告诉我，你叫 Dust Chen。",
				),
				Usage: provider.Usage{
					PromptTokens:     20,
					CompletionTokens: 10,
					TotalTokens:      30,
				},
			},
		},
	}

	result, err := Run(
		ctx,
		Config{
			Provider:     modelProvider,
			Tools:        agenttool.NewRegistry(),
			SystemPrompt: "你是一个会话助手。",
			SessionStore: store,
			SessionID:    currentSession.ID,
		},
		"我叫什么名字？",
	)
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	if result.SessionID != currentSession.ID {
		t.Fatalf(
			"expected session ID %q, got %q",
			currentSession.ID,
			result.SessionID,
		)
	}

	if len(modelProvider.requests) != 1 {
		t.Fatalf(
			"expected one provider request, got %d",
			len(modelProvider.requests),
		)
	}

	requestMessages := modelProvider.requests[0].Messages

	expectedContents := []string{
		"你是一个会话助手。",
		"我叫 Dust Chen",
		"你好，Dust Chen",
		"我叫什么名字？",
	}

	if len(requestMessages) != len(expectedContents) {
		t.Fatalf(
			"expected %d request messages, got %d",
			len(expectedContents),
			len(requestMessages),
		)
	}

	for index, expected := range expectedContents {
		if requestMessages[index].Content != expected {
			t.Fatalf(
				"message %d: expected %q, got %q",
				index,
				expected,
				requestMessages[index].Content,
			)
		}
	}

	storedMessages, err := store.ListMessages(
		ctx,
		currentSession.ID,
		0,
	)
	if err != nil {
		t.Fatalf("list stored messages: %v", err)
	}

	if len(storedMessages) != 4 {
		t.Fatalf(
			"expected 4 stored messages, got %d",
			len(storedMessages),
		)
	}

	if storedMessages[2].Message.Content != "我叫什么名字？" {
		t.Fatalf(
			"unexpected current user message: %q",
			storedMessages[2].Message.Content,
		)
	}

	if storedMessages[3].Message.Content !=
		"你之前告诉我，你叫 Dust Chen。" {
		t.Fatalf(
			"unexpected assistant message: %q",
			storedMessages[3].Message.Content,
		)
	}
}

func TestRuntimeRejectsIncompleteSessionConfig(t *testing.T) {
	_, err := Run(
		context.Background(),
		Config{
			Provider:     &fakeProvider{},
			Tools:        agenttool.NewRegistry(),
			SessionStore: session.NewMemoryStore(),
		},
		"hello",
	)

	if err == nil {
		t.Fatal("expected incomplete session configuration error")
	}
}
