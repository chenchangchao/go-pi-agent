package session

import (
	"encoding/json"
	"time"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

// Status 表示会话当前状态。
type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

// Session 表示一个可跨多次 CLI 调用恢复的对话会话。
type Session struct {
	ID        string            `json:"id"`
	Title     string            `json:"title,omitempty"`
	Status    Status            `json:"status"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// StoredMessage 是持久化后的 Agent 消息。
//
// Sequence 用于确保消息顺序，不依赖数据库时间戳精度。
type StoredMessage struct {
	ID        string            `json:"id"`
	SessionID string            `json:"session_id"`
	Sequence  int64             `json:"sequence"`
	Message   agentcore.Message `json:"message"`
	CreatedAt time.Time         `json:"created_at"`
}

// RunStatus 表示一次 Agent Run 的状态。
type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// Run 表示一次完整的 Agent 执行。
type Run struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	Status      RunStatus       `json:"status"`
	UserPrompt  string          `json:"user_prompt"`
	Turns       int             `json:"turns"`
	ToolCalls   int             `json:"tool_calls"`
	Usage       agentcore.Usage `json:"usage"`
	Error       string          `json:"error,omitempty"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// ToolExecution 表示一次工具调用及其执行结果。
type ToolExecution struct {
	ID         string          `json:"id"`
	RunID      string          `json:"run_id"`
	SessionID  string          `json:"session_id"`
	Turn       int             `json:"turn"`
	ToolCallID string          `json:"tool_call_id"`
	ToolName   string          `json:"tool_name"`
	Arguments  json.RawMessage `json:"arguments"`
	Result     string          `json:"result,omitempty"`
	Error      string          `json:"error,omitempty"`
	StartedAt  time.Time       `json:"started_at"`
	EndedAt    *time.Time      `json:"ended_at,omitempty"`
}
