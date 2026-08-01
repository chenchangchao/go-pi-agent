package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

func TestMemoryStoreSessionLifecycle(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	created, err := store.CreateSession(
		ctx,
		Session{
			Title: "Go Agent Session",
			Metadata: map[string]string{
				"source": "test",
			},
		},
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if created.ID == "" {
		t.Fatal("expected generated session ID")
	}

	if created.Status != StatusActive {
		t.Fatalf(
			"expected active status, got %q",
			created.Status,
		)
	}

	loaded, err := store.GetSession(ctx, created.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}

	if loaded.Title != "Go Agent Session" {
		t.Fatalf(
			"unexpected title: %q",
			loaded.Title,
		)
	}

	loaded.Title = "Updated Session"

	if err := store.UpdateSession(ctx, loaded); err != nil {
		t.Fatalf("update session: %v", err)
	}

	updated, err := store.GetSession(ctx, created.ID)
	if err != nil {
		t.Fatalf("get updated session: %v", err)
	}

	if updated.Title != "Updated Session" {
		t.Fatalf(
			"unexpected updated title: %q",
			updated.Title,
		)
	}
}

func TestMemoryStoreAppendsMessagesInOrder(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	currentSession, err := store.CreateSession(
		ctx,
		Session{Title: "Message Test"},
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	first, err := store.AppendMessage(
		ctx,
		currentSession.ID,
		agentcore.UserMessage("你好"),
	)
	if err != nil {
		t.Fatalf("append first message: %v", err)
	}

	second, err := store.AppendMessage(
		ctx,
		currentSession.ID,
		agentcore.AssistantMessage("你好，有什么可以帮你？"),
	)
	if err != nil {
		t.Fatalf("append second message: %v", err)
	}

	if first.Sequence != 1 {
		t.Fatalf(
			"expected sequence 1, got %d",
			first.Sequence,
		)
	}

	if second.Sequence != 2 {
		t.Fatalf(
			"expected sequence 2, got %d",
			second.Sequence,
		)
	}

	messages, err := store.ListMessages(
		ctx,
		currentSession.ID,
		0,
	)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf(
			"expected 2 messages, got %d",
			len(messages),
		)
	}

	if messages[0].Message.Content != "你好" {
		t.Fatalf(
			"unexpected first message: %q",
			messages[0].Message.Content,
		)
	}

	if messages[1].Message.Role != agentcore.RoleAssistant {
		t.Fatalf(
			"unexpected second role: %q",
			messages[1].Message.Role,
		)
	}
}

func TestMemoryStoreListMessagesLimit(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	currentSession, err := store.CreateSession(
		ctx,
		Session{Title: "Limit Test"},
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	for _, content := range []string{"one", "two", "three"} {
		if _, err := store.AppendMessage(
			ctx,
			currentSession.ID,
			agentcore.UserMessage(content),
		); err != nil {
			t.Fatalf("append message: %v", err)
		}
	}

	messages, err := store.ListMessages(
		ctx,
		currentSession.ID,
		2,
	)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf(
			"expected 2 messages, got %d",
			len(messages),
		)
	}

	if messages[0].Message.Content != "two" {
		t.Fatalf(
			"expected first limited message %q, got %q",
			"two",
			messages[0].Message.Content,
		)
	}
}

func TestMemoryStoreRunLifecycle(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	currentSession, err := store.CreateSession(
		ctx,
		Session{Title: "Run Test"},
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	run, err := store.CreateRun(
		ctx,
		Run{
			SessionID:  currentSession.ID,
			UserPrompt: "读取 README",
		},
	)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	if run.Status != RunStatusRunning {
		t.Fatalf(
			"expected running status, got %q",
			run.Status,
		)
	}

	completedAt := time.Now().UTC()
	run.Status = RunStatusCompleted
	run.Turns = 2
	run.ToolCalls = 1
	run.CompletedAt = &completedAt
	run.Usage = agentcore.Usage{
		PromptTokens:     100,
		CompletionTokens: 30,
		TotalTokens:      130,
	}

	if err := store.UpdateRun(ctx, run); err != nil {
		t.Fatalf("update run: %v", err)
	}
}

func TestMemoryStoreRejectsUnknownSession(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.AppendMessage(
		context.Background(),
		"missing",
		agentcore.UserMessage("hello"),
	)

	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf(
			"expected ErrSessionNotFound, got %v",
			err,
		)
	}
}

func TestMemoryStoreHonorsContextCancellation(t *testing.T) {
	store := NewMemoryStore()

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := store.CreateSession(
		ctx,
		Session{Title: "Cancelled"},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}
