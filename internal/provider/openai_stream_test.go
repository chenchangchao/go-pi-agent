package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

func TestOpenAICompatibleProviderStreamsText(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.Header().Set(
				"Content-Type",
				"text/event-stream",
			)

			flusher, ok := writer.(http.Flusher)
			if !ok {
				t.Fatal("response writer does not support flushing")
			}

			chunks := []string{
				`{"choices":[{"delta":{"role":"assistant","content":""},"finish_reason":""}]}`,
				`{"choices":[{"delta":{"content":"你好"},"finish_reason":""}]}`,
				`{"choices":[{"delta":{"content":"，Go Agent"},"finish_reason":""}]}`,
				`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
				`{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
			}

			for _, chunk := range chunks {
				fmt.Fprintf(writer, "data: %s\n\n", chunk)
				flusher.Flush()
			}

			fmt.Fprint(writer, "data: [DONE]\n\n")
			flusher.Flush()
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

	result, err := client.Stream(
		context.Background(),
		ChatRequest{
			Messages: []agentcore.Message{
				agentcore.UserMessage("你好"),
			},
		},
	)
	if err != nil {
		t.Fatalf("start stream: %v", err)
	}

	var deltas strings.Builder
	var finalEvent StreamEvent

	for event := range result.Events {
		switch event.Type {
		case StreamEventTextDelta:
			deltas.WriteString(event.Delta)
		case StreamEventMessageEnd:
			finalEvent = event
		case StreamEventError:
			t.Fatalf("unexpected stream error: %v", event.Err)
		}
	}

	if deltas.String() != "你好，Go Agent" {
		t.Fatalf(
			"unexpected deltas: %q",
			deltas.String(),
		)
	}

	if finalEvent.Message == nil {
		t.Fatal("expected final message")
	}

	if finalEvent.Message.Content != "你好，Go Agent" {
		t.Fatalf(
			"unexpected final message: %q",
			finalEvent.Message.Content,
		)
	}

	if finalEvent.FinishReason != "stop" {
		t.Fatalf(
			"unexpected finish reason: %q",
			finalEvent.FinishReason,
		)
	}

	if finalEvent.Usage.TotalTokens != 15 {
		t.Fatalf(
			"expected 15 tokens, got %d",
			finalEvent.Usage.TotalTokens,
		)
	}
}

func TestOpenAICompatibleProviderStreamsToolCall(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.Header().Set(
				"Content-Type",
				"text/event-stream",
			)

			chunks := []string{
				`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"read_file","arguments":""}}]},"finish_reason":""}]}`,
				`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":"}}]},"finish_reason":""}]}`,
				`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"README.md\"}"}}]},"finish_reason":""}]}`,
				`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
			}

			for _, chunk := range chunks {
				fmt.Fprintf(writer, "data: %s\n\n", chunk)
			}

			fmt.Fprint(writer, "data: [DONE]\n\n")
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

	result, err := client.Stream(
		context.Background(),
		ChatRequest{
			Messages: []agentcore.Message{
				agentcore.UserMessage("读取 README"),
			},
		},
	)
	if err != nil {
		t.Fatalf("start stream: %v", err)
	}

	var finalEvent StreamEvent

	for event := range result.Events {
		if event.Type == StreamEventError {
			t.Fatalf("unexpected stream error: %v", event.Err)
		}

		if event.Type == StreamEventMessageEnd {
			finalEvent = event
		}
	}

	if finalEvent.Message == nil {
		t.Fatal("expected final message")
	}

	if len(finalEvent.Message.ToolCalls) != 1 {
		t.Fatalf(
			"expected one tool call, got %d",
			len(finalEvent.Message.ToolCalls),
		)
	}

	call := finalEvent.Message.ToolCalls[0]

	if call.ID != "call_123" {
		t.Fatalf(
			"unexpected call ID: %q",
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
			"unexpected arguments: %s",
			call.Arguments,
		)
	}

	if finalEvent.FinishReason != "tool_calls" {
		t.Fatalf(
			"unexpected finish reason: %q",
			finalEvent.FinishReason,
		)
	}
}
