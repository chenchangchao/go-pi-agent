package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chenchangchao/go-pi-agent/internal/agenttool"
	"github.com/chenchangchao/go-pi-agent/internal/provider"
	"github.com/chenchangchao/go-pi-agent/internal/runtime"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Fprintln(
			os.Stderr,
			"提示：未读取到 .env，将使用系统环境变量",
		)
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(
			os.Stderr,
			`用法: go run ./cmd/agent "你的问题"`,
		)
		os.Exit(1)
	}

	prompt := strings.TrimSpace(
		strings.Join(os.Args[1:], " "),
	)
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "错误：问题不能为空")
		os.Exit(1)
	}

	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	baseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))

	if model == "" {
		fmt.Fprintln(
			os.Stderr,
			"错误：未设置 OPENAI_MODEL",
		)
		os.Exit(1)
	}

	modelProvider, err := provider.NewOpenAICompatibleProvider(
		provider.OpenAICompatibleConfig{
			BaseURL: baseURL,
			APIKey:  apiKey,
			Model:   model,
			Timeout: 90 * time.Second,
		},
	)
	if err != nil {
		exitWithError("创建模型 Provider 失败", err)
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		exitWithError("获取当前目录失败", err)
	}

	readFileTool, err := agenttool.NewReadFileTool(
		workingDirectory,
	)
	if err != nil {
		exitWithError("创建 read_file 工具失败", err)
	}

	toolRegistry := agenttool.NewRegistry()

	if err := toolRegistry.Register(readFileTool); err != nil {
		exitWithError("注册 read_file 工具失败", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Minute,
	)
	defer cancel()

	result, err := runtime.Run(
		ctx,
		runtime.Config{
			Provider: modelProvider,
			Tools:    toolRegistry,
			SystemPrompt: `
你是一个使用 Go 开发的 AI Agent。
你可以调用工具读取当前项目中的文件。
当用户要求分析或总结文件时，必须先调用 read_file 获取真实内容，
不能凭空猜测文件内容。
工具返回结果后，请基于结果回答用户。
`,
			MaxTurns: 8,
		},
		prompt,
	)
	if err != nil {
		exitWithError("Agent 运行失败", err)
	}

	fmt.Println("Go Pi Agent")
	fmt.Println("------------")
	fmt.Println(result.Message.Content)
	fmt.Println()
	fmt.Printf("Turns: %d\n", result.Turns)
	fmt.Printf("Tool calls: %d\n", result.ToolCalls)
	fmt.Printf(
		"Token usage: prompt=%d completion=%d total=%d\n",
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens,
		result.Usage.TotalTokens,
	)
}

func exitWithError(message string, err error) {
	fmt.Fprintf(os.Stderr, "%s：%v\n", message, err)
	os.Exit(1)
}
