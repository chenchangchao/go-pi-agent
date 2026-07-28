package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

func TestOpenAICompatibleProviderChatTextResponse(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			if request.URL.Path != "/chat/completions" {
				t.Fatalf(
					"unexpected request path: %s",
					request.URL.Path,
				)
			}

			if request.Method != http.MethodPost {
				t.Fatalf(
					"unexpected HTTP method: %s",
					request.Method,
				)
			}

			if request.Header.Get("Authorization") !=
				"Bearer test-key" {
				t.Fatalf(
					"unexpected authorization header: %q",
					request.Header.Get("Authorization"),
				)
			}

			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}

			var payload chatCompletionRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}

			if payload.Model != "test-model" {
				t.Fatalf(
					"expected model %q, got %q",
					"test-model",
					payload.Model,
				)
			}

			if len(payload.Messages) != 1 {
				t.Fatalf(
					"expected one message, got %d",
					len(payload.Messages),
				)
			}

			writer.Header().Set(
				"Content-Type",
				"application/json",
			)

			_, _ = writer.Write([]byte(`{
				"choices": [
					{
						"message": {
							"role": "assistant",
							"content": "你好，我是 Go Agent。"
						},
						"finish_reason": "stop"
					}
				],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 8,
					"total_tokens": 18
				}
			}`))
		}),
	)
	defer server.Close()

	client, err := NewOpenAICompatibleProvider(
		OpenAICompatibleConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "test-model",
		},
	)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	response, err := client.Chat(
		context.Background(),
		ChatRequest{
			Messages: []agentcore.Message{
				agentcore.UserMessage("你好"),
			},
		},
	)
	if err != nil {
		t.Fatalf("call provider: %v", err)
	}

	if response.Message.Content !=
		"你好，我是 Go Agent。" {
		t.Fatalf(
			"unexpected assistant content: %q",
			response.Message.Content,
		)
	}

	if response.FinishReason != "stop" {
		t.Fatalf(
			"unexpected finish reason: %q",
			response.FinishReason,
		)
	}

	if response.Usage.TotalTokens != 18 {
		t.Fatalf(
			"expected 18 tokens, got %d",
			response.Usage.TotalTokens,
		)
	}
}

func TestOpenAICompatibleProviderDecodesToolCall(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.Header().Set(
				"Content-Type",
				"application/json",
			)

			_, _ = writer.Write([]byte(`{
				"choices": [
					{
						"message": {
							"role": "assistant",
							"content": "",
							"tool_calls": [
								{
									"id": "call_123",
									"type": "function",
									"function": {
										"name": "read_file",
										"arguments": "{\"path\":\"README.md\"}"
									}
								}
							]
						},
						"finish_reason": "tool_calls"
					}
				],
				"usage": {
					"prompt_tokens": 20,
					"completion_tokens": 10,
					"total_tokens": 30
				}
			}`))
		}),
	)
	defer server.Close()

	client, err := NewOpenAICompatibleProvider(
		OpenAICompatibleConfig{
			BaseURL: server.URL,
			Model:   "test-model",
		},
	)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	response, err := client.Chat(
		context.Background(),
		ChatRequest{
			Messages: []agentcore.Message{
				agentcore.UserMessage(
					"读取 README.md",
				),
			},
			Tools: []agentcore.ToolDefinition{
				{
					Name:        "read_file",
					Description: "读取文件",
					Parameters: map[string]any{
						"type": "object",
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("call provider: %v", err)
	}

	if len(response.Message.ToolCalls) != 1 {
		t.Fatalf(
			"expected one tool call, got %d",
			len(response.Message.ToolCalls),
		)
	}

	call := response.Message.ToolCalls[0]

	if call.ID != "call_123" {
		t.Fatalf(
			"unexpected tool call ID: %q",
			call.ID,
		)
	}

	if call.Name != "read_file" {
		t.Fatalf(
			"unexpected tool name: %q",
			call.Name,
		)
	}

	if string(call.Arguments) !=
		`{"path":"README.md"}` {
		t.Fatalf(
			"unexpected tool arguments: %s",
			call.Arguments,
		)
	}
}

func TestOpenAICompatibleProviderReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.Header().Set(
				"Content-Type",
				"application/json",
			)

			writer.WriteHeader(http.StatusUnauthorized)

			_, _ = writer.Write([]byte(`{
				"error": {
					"message": "invalid API key",
					"type": "authentication_error"
				}
			}`))
		}),
	)
	defer server.Close()

	client, err := NewOpenAICompatibleProvider(
		OpenAICompatibleConfig{
			BaseURL: server.URL,
			Model:   "test-model",
		},
	)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	_, err = client.Chat(
		context.Background(),
		ChatRequest{
			Messages: []agentcore.Message{
				agentcore.UserMessage("hello"),
			},
		},
	)
	if err == nil {
		t.Fatal("expected API error")
	}

	if !strings.Contains(
		err.Error(),
		"invalid API key",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}
