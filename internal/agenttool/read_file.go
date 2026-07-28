package agenttool

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

const (
	defaultMaxReadLines = 500
	defaultMaxLineBytes = 8 * 1024
)

// ReadFileTool 读取工作目录内的文本文件。
//
// RootDir 是工具允许访问的根目录。
// 模型传入的相对路径最终必须位于 RootDir 内。
type ReadFileTool struct {
	RootDir  string
	MaxLines int
}

// readFileArguments 是模型调用 read_file 时传入的参数。
type readFileArguments struct {
	Path string `json:"path"`
}

// NewReadFileTool 创建一个文件读取工具。
func NewReadFileTool(rootDir string) (*ReadFileTool, error) {
	if strings.TrimSpace(rootDir) == "" {
		return nil, errors.New("root directory cannot be empty")
	}

	absoluteRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root directory: %w", err)
	}

	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("stat root directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", absoluteRoot)
	}

	return &ReadFileTool{
		RootDir:  absoluteRoot,
		MaxLines: defaultMaxReadLines,
	}, nil
}

// Definition 返回发送给大模型的工具定义。
func (tool *ReadFileTool) Definition() agentcore.ToolDefinition {
	return agentcore.ToolDefinition{
		Name:        "read_file",
		Description: "读取当前项目目录内的文本文件，并返回带行号的内容。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "相对于项目根目录的文件路径，例如 README.md 或 internal/runtime/runtime.go",
				},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		},
	}
}

// Execute 执行文件读取。
func (tool *ReadFileTool) Execute(
	ctx context.Context,
	rawArguments json.RawMessage,
) (string, error) {
	var arguments readFileArguments

	if err := json.Unmarshal(rawArguments, &arguments); err != nil {
		return "", fmt.Errorf("decode read_file arguments: %w", err)
	}

	arguments.Path = strings.TrimSpace(arguments.Path)
	if arguments.Path == "" {
		return "", errors.New("path cannot be empty")
	}

	resolvedPath, err := tool.resolveWithinRoot(arguments.Path)
	if err != nil {
		return "", err
	}

	file, err := os.Open(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("open file %q: %w", arguments.Path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file %q: %w", arguments.Path, err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path %q is a directory, not a file", arguments.Path)
	}

	maxLines := tool.MaxLines
	if maxLines <= 0 {
		maxLines = defaultMaxReadLines
	}

	content, truncated, err := readLines(ctx, file, maxLines)
	if err != nil {
		return "", fmt.Errorf("read file %q: %w", arguments.Path, err)
	}

	var result strings.Builder

	fmt.Fprintf(&result, "File: %s\n", arguments.Path)
	fmt.Fprintf(&result, "Size: %d bytes\n\n", info.Size())
	result.WriteString(content)

	if truncated {
		fmt.Fprintf(
			&result,
			"\n[内容已截断：最多返回 %d 行]\n",
			maxLines,
		)
	}

	return result.String(), nil
}

// resolveWithinRoot 将用户路径解析为 RootDir 内的绝对路径。
func (tool *ReadFileTool) resolveWithinRoot(path string) (string, error) {
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(tool.RootDir, candidate)
	}

	absolutePath, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve file path: %w", err)
	}

	relativePath, err := filepath.Rel(tool.RootDir, absolutePath)
	if err != nil {
		return "", fmt.Errorf("check file boundary: %w", err)
	}

	if relativePath == ".." ||
		strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf(
			"access denied: path %q is outside project root",
			path,
		)
	}

	return absolutePath, nil
}

func readLines(
	ctx context.Context,
	reader io.Reader,
	maxLines int,
) (content string, truncated bool, err error) {
	scanner := bufio.NewScanner(reader)

	// Scanner 默认单行限制约为 64 KB。
	// MVP 中主动设定上限，防止异常超长行占用过多内存。
	scanner.Buffer(make([]byte, 1024), defaultMaxLineBytes)

	var builder strings.Builder
	lineNumber := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		default:
		}

		lineNumber++

		if lineNumber > maxLines {
			return builder.String(), true, nil
		}

		fmt.Fprintf(
			&builder,
			"%4d | %s\n",
			lineNumber,
			scanner.Text(),
		)
	}

	if err := scanner.Err(); err != nil {
		return "", false, err
	}

	return builder.String(), false, nil
}
