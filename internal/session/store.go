package session

import (
	"context"
	"errors"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

var (
	// ErrSessionNotFound 表示指定会话不存在。
	ErrSessionNotFound = errors.New("session not found")

	// ErrRunNotFound 表示指定运行记录不存在。
	ErrRunNotFound = errors.New("run not found")
)

// Store 定义 Agent 会话持久化能力。
//
// Runtime 只依赖该接口，不依赖 PostgreSQL、Redis 或其他实现。
type Store interface {
	CreateSession(
		ctx context.Context,
		session Session,
	) (Session, error)

	GetSession(
		ctx context.Context,
		sessionID string,
	) (Session, error)

	UpdateSession(
		ctx context.Context,
		session Session,
	) error

	AppendMessage(
		ctx context.Context,
		sessionID string,
		message agentcore.Message,
	) (StoredMessage, error)

	ListMessages(
		ctx context.Context,
		sessionID string,
		limit int,
	) ([]StoredMessage, error)

	CreateRun(
		ctx context.Context,
		run Run,
	) (Run, error)

	UpdateRun(
		ctx context.Context,
		run Run,
	) error

	SaveToolExecution(
		ctx context.Context,
		execution ToolExecution,
	) (ToolExecution, error)
}
