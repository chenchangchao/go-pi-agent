package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chenchangchao/go-pi-agent/internal/agenttool"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(
			os.Stderr,
			`用法: go run ./cmd/agent "文件路径"`,
		)
		os.Exit(1)
	}

	path := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	if path == "" {
		fmt.Fprintln(os.Stderr, "错误：文件路径不能为空")
		os.Exit(1)
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		exitWithError("获取当前目录失败", err)
	}

	readFileTool, err := agenttool.NewReadFileTool(workingDirectory)
	if err != nil {
		exitWithError("创建 read_file 工具失败", err)
	}

	registry := agenttool.NewRegistry()

	if err := registry.Register(readFileTool); err != nil {
		exitWithError("注册 read_file 工具失败", err)
	}

	result, err := registry.ExecuteRaw(
		context.Background(),
		"manual_call_001",
		"read_file",
		map[string]string{
			"path": path,
		},
	)
	if err != nil {
		exitWithError("执行工具失败", err)
	}

	fmt.Println("Go Pi Agent")
	fmt.Println("------------")
	fmt.Printf("Tool: %s\n", result.Name)
	fmt.Printf("Tool Call ID: %s\n\n", result.ToolCallID)
	fmt.Println(result.Content)
}

func exitWithError(message string, err error) {
	fmt.Fprintf(os.Stderr, "%s：%v\n", message, err)
	os.Exit(1)
}
