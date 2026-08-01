package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

// MemoryStore 是线程安全的内存 Session Store。
//
// 主要用于：
//   - 单元测试
//   - 本地开发
//   - 验证 Runtime 与 Store 接口
type MemoryStore struct {
	mu sync.RWMutex

	sessions       map[string]Session
	messages       map[string][]StoredMessage
	runs           map[string]Run
	toolExecutions map[string]ToolExecution

	idCounter atomic.Uint64
}

// NewMemoryStore 创建空的内存 Store。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions:       make(map[string]Session),
		messages:       make(map[string][]StoredMessage),
		runs:           make(map[string]Run),
		toolExecutions: make(map[string]ToolExecution),
	}
}

func (store *MemoryStore) CreateSession(
	ctx context.Context,
	input Session,
) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}

	now := time.Now().UTC()

	if strings.TrimSpace(input.ID) == "" {
		input.ID = store.nextID("session")
	}

	if input.Status == "" {
		input.Status = StatusActive
	}

	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}

	input.UpdatedAt = now
	input.Metadata = cloneMetadata(input.Metadata)

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, exists := store.sessions[input.ID]; exists {
		return Session{}, fmt.Errorf(
			"session %q already exists",
			input.ID,
		)
	}

	store.sessions[input.ID] = input

	return cloneSession(input), nil
}

func (store *MemoryStore) GetSession(
	ctx context.Context,
	sessionID string,
) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}

	store.mu.RLock()
	defer store.mu.RUnlock()

	current, exists := store.sessions[sessionID]
	if !exists {
		return Session{}, ErrSessionNotFound
	}

	return cloneSession(current), nil
}

func (store *MemoryStore) UpdateSession(
	ctx context.Context,
	input Session,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	current, exists := store.sessions[input.ID]
	if !exists {
		return ErrSessionNotFound
	}

	if input.Status == "" {
		input.Status = current.Status
	}

	input.CreatedAt = current.CreatedAt
	input.UpdatedAt = time.Now().UTC()
	input.Metadata = cloneMetadata(input.Metadata)

	store.sessions[input.ID] = input
	return nil
}

func (store *MemoryStore) AppendMessage(
	ctx context.Context,
	sessionID string,
	message agentcore.Message,
) (StoredMessage, error) {
	if err := ctx.Err(); err != nil {
		return StoredMessage{}, err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, exists := store.sessions[sessionID]; !exists {
		return StoredMessage{}, ErrSessionNotFound
	}

	currentMessages := store.messages[sessionID]

	stored := StoredMessage{
		ID:        store.nextID("message"),
		SessionID: sessionID,
		Sequence:  int64(len(currentMessages) + 1),
		Message:   message,
		CreatedAt: time.Now().UTC(),
	}

	store.messages[sessionID] = append(
		currentMessages,
		stored,
	)

	sessionValue := store.sessions[sessionID]
	sessionValue.UpdatedAt = stored.CreatedAt
	store.sessions[sessionID] = sessionValue

	return stored, nil
}

func (store *MemoryStore) ListMessages(
	ctx context.Context,
	sessionID string,
	limit int,
) ([]StoredMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	store.mu.RLock()
	defer store.mu.RUnlock()

	if _, exists := store.sessions[sessionID]; !exists {
		return nil, ErrSessionNotFound
	}

	source := store.messages[sessionID]

	start := 0
	if limit > 0 && len(source) > limit {
		start = len(source) - limit
	}

	result := slices.Clone(source[start:])
	return result, nil
}

func (store *MemoryStore) CreateRun(
	ctx context.Context,
	input Run,
) (Run, error) {
	if err := ctx.Err(); err != nil {
		return Run{}, err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, exists := store.sessions[input.SessionID]; !exists {
		return Run{}, ErrSessionNotFound
	}

	if strings.TrimSpace(input.ID) == "" {
		input.ID = store.nextID("run")
	}

	if input.Status == "" {
		input.Status = RunStatusRunning
	}

	if input.StartedAt.IsZero() {
		input.StartedAt = time.Now().UTC()
	}

	if _, exists := store.runs[input.ID]; exists {
		return Run{}, fmt.Errorf(
			"run %q already exists",
			input.ID,
		)
	}

	store.runs[input.ID] = input
	return input, nil
}

func (store *MemoryStore) UpdateRun(
	ctx context.Context,
	input Run,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	current, exists := store.runs[input.ID]
	if !exists {
		return ErrRunNotFound
	}

	if input.SessionID == "" {
		input.SessionID = current.SessionID
	}

	if input.StartedAt.IsZero() {
		input.StartedAt = current.StartedAt
	}

	store.runs[input.ID] = input
	return nil
}

func (store *MemoryStore) SaveToolExecution(
	ctx context.Context,
	input ToolExecution,
) (ToolExecution, error) {
	if err := ctx.Err(); err != nil {
		return ToolExecution{}, err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, exists := store.sessions[input.SessionID]; !exists {
		return ToolExecution{}, ErrSessionNotFound
	}

	if _, exists := store.runs[input.RunID]; !exists {
		return ToolExecution{}, ErrRunNotFound
	}

	if strings.TrimSpace(input.ID) == "" {
		input.ID = store.nextID("tool")
	}

	if input.StartedAt.IsZero() {
		input.StartedAt = time.Now().UTC()
	}

	store.toolExecutions[input.ID] = input
	return input, nil
}

func (store *MemoryStore) nextID(prefix string) string {
	value := store.idCounter.Add(1)
	return fmt.Sprintf("%s_%06d", prefix, value)
}

func cloneSession(input Session) Session {
	input.Metadata = cloneMetadata(input.Metadata)
	return input
}

func cloneMetadata(
	input map[string]string,
) map[string]string {
	if input == nil {
		return nil
	}

	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}

	return result
}

// 编译期检查 MemoryStore 是否完整实现 Store。
var _ Store = (*MemoryStore)(nil)

// 防止 errors 包因未来调整出现未使用问题，同时显式保留错误语义。
var _ = errors.Is
