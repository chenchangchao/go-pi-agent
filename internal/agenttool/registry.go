package agenttool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

// Registry 保存 Agent 可用的全部工具。
//
// Registry 在运行期间支持并发读取。
// MVP 阶段工具通常只在启动时注册，运行时只读。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]agentcore.Tool
}

// NewRegistry 创建空的工具注册表。
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]agentcore.Tool),
	}
}

// Register 注册一个工具。
//
// 工具名称必须非空，并且不能重复。
func (registry *Registry) Register(tool agentcore.Tool) error {
	if tool == nil {
		return errors.New("tool cannot be nil")
	}

	definition := tool.Definition()
	name := strings.TrimSpace(definition.Name)

	if name == "" {
		return errors.New("tool name cannot be empty")
	}

	if definition.Parameters == nil {
		return fmt.Errorf("tool %q parameters schema cannot be nil", name)
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if _, exists := registry.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}

	registry.tools[name] = tool
	return nil
}

// Get 根据名称查找工具。
func (registry *Registry) Get(name string) (agentcore.Tool, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	tool, exists := registry.tools[name]
	return tool, exists
}

// Definitions 返回全部工具定义。
//
// 返回结果按照工具名称排序，保证测试和请求内容稳定。
func (registry *Registry) Definitions() []agentcore.ToolDefinition {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	definitions := make([]agentcore.ToolDefinition, 0, len(registry.tools))

	for _, tool := range registry.tools {
		definitions = append(definitions, tool.Definition())
	}

	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].Name < definitions[j].Name
	})

	return definitions
}

// Execute 根据 ToolCall 查找并执行工具。
func (registry *Registry) Execute(
	ctx context.Context,
	call agentcore.ToolCall,
) (agentcore.Message, error) {
	name := strings.TrimSpace(call.Name)
	if name == "" {
		return agentcore.Message{}, errors.New("tool call name cannot be empty")
	}

	tool, exists := registry.Get(name)
	if !exists {
		return agentcore.Message{}, fmt.Errorf("tool %q is not registered", name)
	}

	result, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		return agentcore.Message{}, fmt.Errorf(
			"execute tool %q: %w",
			name,
			err,
		)
	}

	return agentcore.ToolResultMessage(
		call.ID,
		name,
		result,
	), nil
}

// ExecuteRaw 是一个便于测试和手动调用的辅助方法。
func (registry *Registry) ExecuteRaw(
	ctx context.Context,
	callID string,
	name string,
	arguments any,
) (agentcore.Message, error) {
	rawArguments, err := json.Marshal(arguments)
	if err != nil {
		return agentcore.Message{}, fmt.Errorf(
			"encode tool arguments: %w",
			err,
		)
	}

	return registry.Execute(
		ctx,
		agentcore.ToolCall{
			ID:        callID,
			Name:      name,
			Arguments: rawArguments,
		},
	)
}
