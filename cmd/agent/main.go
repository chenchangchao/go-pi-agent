package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
	"github.com/chenchangchao/go-pi-agent/internal/provider"
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

	model := strings.TrimSpace(
		os.Getenv("OPENAI_MODEL"),
	)
	baseURL := strings.TrimSpace(
		os.Getenv("OPENAI_BASE_URL"),
	)
	apiKey := strings.TrimSpace(
		os.Getenv("OPENAI_API_KEY"),
	)

	if model == "" {
		fmt.Fprintln(
			os.Stderr,
			"错误：未设置 OPENAI_MODEL",
		)
		os.Exit(1)
	}

	client, err := provider.NewOpenAICompatibleProvider(
		provider.OpenAICompatibleConfig{
			BaseURL: baseURL,
			APIKey:  apiKey,
			Model:   model,
			Timeout: 60 * time.Second,
		},
	)
	if err != nil {
		exitWithError("创建模型 Provider 失败", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		60*time.Second,
	)
	defer cancel()

	response, err := client.Chat(
		ctx,
		provider.ChatRequest{
			Messages: []agentcore.Message{
				agentcore.SystemMessage(
					"你是一个使用 Go 开发的简洁、可靠的 AI Agent。",
				),
				agentcore.UserMessage(prompt),
			},
		},
	)
	if err != nil {
		exitWithError("调用模型失败", err)
	}

	fmt.Println("Go Pi Agent")
	fmt.Println("------------")
	fmt.Println(response.Message.Content)
	fmt.Println()
	fmt.Printf(
		"Finish reason: %s\n",
		response.FinishReason,
	)
	fmt.Printf(
		"Token usage: prompt=%d completion=%d total=%d\n",
		response.Usage.PromptTokens,
		response.Usage.CompletionTokens,
		response.Usage.TotalTokens,
	)
}

func exitWithError(message string, err error) {
	fmt.Fprintf(os.Stderr, "%s：%v\n", message, err)
	os.Exit(1)
}
