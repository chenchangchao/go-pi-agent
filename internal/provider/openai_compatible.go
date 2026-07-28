package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chenchangchao/go-pi-agent/internal/agentcore"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultTimeout = 60 * time.Second
)

// OpenAICompatibleConfig 是 Provider 配置。
type OpenAICompatibleConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration

	// HTTPClient 主要用于测试或自定义代理。
	// 为 nil 时自动创建。
	HTTPClient *http.Client
}

// OpenAICompatibleProvider 调用兼容 Chat Completions 协议的模型服务。
type OpenAICompatibleProvider struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAICompatibleProvider 创建 Provider。
func NewOpenAICompatibleProvider(
	config OpenAICompatibleConfig,
) (*OpenAICompatibleProvider, error) {
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	baseURL = strings.TrimRight(baseURL, "/")

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf(
			"base URL must use http or https: %q",
			baseURL,
		)
	}

	model := strings.TrimSpace(config.Model)
	if model == "" {
		return nil, errors.New("model cannot be empty")
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: timeout,
		}
	}

	return &OpenAICompatibleProvider{
		baseURL:    baseURL,
		apiKey:     strings.TrimSpace(config.APIKey),
		model:      model,
		httpClient: httpClient,
	}, nil
}

// Chat 调用 /chat/completions。
func (provider *OpenAICompatibleProvider) Chat(
	ctx context.Context,
	request ChatRequest,
) (ChatResponse, error) {
	apiRequest, err := provider.buildRequest(request)
	if err != nil {
		return ChatResponse{}, err
	}

	requestBody, err := json.Marshal(apiRequest)
	if err != nil {
		return ChatResponse{}, fmt.Errorf(
			"encode chat request: %w",
			err,
		)
	}

	endpoint := provider.baseURL + "/chat/completions"

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return ChatResponse{}, fmt.Errorf(
			"create HTTP request: %w",
			err,
		)
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	if provider.apiKey != "" {
		httpRequest.Header.Set(
			"Authorization",
			"Bearer "+provider.apiKey,
		)
	}

	httpResponse, err := provider.httpClient.Do(httpRequest)
	if err != nil {
		return ChatResponse{}, fmt.Errorf(
			"send chat request: %w",
			err,
		)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(
		io.LimitReader(httpResponse.Body, 10*1024*1024),
	)
	if err != nil {
		return ChatResponse{}, fmt.Errorf(
			"read chat response: %w",
			err,
		)
	}

	if httpResponse.StatusCode < 200 ||
		httpResponse.StatusCode >= 300 {
		return ChatResponse{}, decodeAPIError(
			httpResponse.StatusCode,
			responseBody,
		)
	}

	var apiResponse chatCompletionResponse

	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		return ChatResponse{}, fmt.Errorf(
			"decode chat response: %w",
			err,
		)
	}

	if len(apiResponse.Choices) == 0 {
		return ChatResponse{}, errors.New(
			"chat response contains no choices",
		)
	}

	choice := apiResponse.Choices[0]

	message, err := decodeAssistantMessage(choice.Message)
	if err != nil {
		return ChatResponse{}, err
	}

	return ChatResponse{
		Message:      message,
		FinishReason: choice.FinishReason,
		Usage: Usage{
			PromptTokens:     apiResponse.Usage.PromptTokens,
			CompletionTokens: apiResponse.Usage.CompletionTokens,
			TotalTokens:      apiResponse.Usage.TotalTokens,
		},
	}, nil
}

func (provider *OpenAICompatibleProvider) buildRequest(
	request ChatRequest,
) (chatCompletionRequest, error) {
	if len(request.Messages) == 0 {
		return chatCompletionRequest{}, errors.New(
			"messages cannot be empty",
		)
	}

	messages := make(
		[]chatCompletionMessage,
		0,
		len(request.Messages),
	)

	for _, message := range request.Messages {
		encodedMessage, err := encodeMessage(message)
		if err != nil {
			return chatCompletionRequest{}, err
		}

		messages = append(messages, encodedMessage)
	}

	tools := make(
		[]chatCompletionTool,
		0,
		len(request.Tools),
	)

	for _, definition := range request.Tools {
		if strings.TrimSpace(definition.Name) == "" {
			return chatCompletionRequest{}, errors.New(
				"tool definition name cannot be empty",
			)
		}

		tools = append(tools, chatCompletionTool{
			Type: "function",
			Function: chatCompletionFunction{
				Name:        definition.Name,
				Description: definition.Description,
				Parameters:  definition.Parameters,
			},
		})
	}

	apiRequest := chatCompletionRequest{
		Model:    provider.model,
		Messages: messages,
	}

	if len(tools) > 0 {
		apiRequest.Tools = tools
		apiRequest.ToolChoice = "auto"
	}

	return apiRequest, nil
}

func encodeMessage(
	message agentcore.Message,
) (chatCompletionMessage, error) {
	switch message.Role {
	case agentcore.RoleSystem,
		agentcore.RoleUser:
		return chatCompletionMessage{
			Role:    string(message.Role),
			Content: message.Content,
		}, nil

	case agentcore.RoleAssistant:
		encoded := chatCompletionMessage{
			Role:    string(message.Role),
			Content: message.Content,
		}

		for _, call := range message.ToolCalls {
			encoded.ToolCalls = append(
				encoded.ToolCalls,
				chatCompletionToolCall{
					ID:   call.ID,
					Type: "function",
					Function: chatCompletionToolCallFunction{
						Name:      call.Name,
						Arguments: string(call.Arguments),
					},
				},
			)
		}

		return encoded, nil

	case agentcore.RoleTool:
		if strings.TrimSpace(message.ToolCallID) == "" {
			return chatCompletionMessage{}, errors.New(
				"tool message requires tool_call_id",
			)
		}

		return chatCompletionMessage{
			Role:       string(message.Role),
			Content:    message.Content,
			ToolCallID: message.ToolCallID,
		}, nil

	default:
		return chatCompletionMessage{}, fmt.Errorf(
			"unsupported message role %q",
			message.Role,
		)
	}
}

func decodeAssistantMessage(
	message chatCompletionResponseMessage,
) (agentcore.Message, error) {
	result := agentcore.Message{
		Role:    agentcore.RoleAssistant,
		Content: message.Content,
	}

	for _, call := range message.ToolCalls {
		if call.Type != "" && call.Type != "function" {
			return agentcore.Message{}, fmt.Errorf(
				"unsupported tool call type %q",
				call.Type,
			)
		}

		if strings.TrimSpace(call.ID) == "" {
			return agentcore.Message{}, errors.New(
				"tool call ID cannot be empty",
			)
		}

		if strings.TrimSpace(call.Function.Name) == "" {
			return agentcore.Message{}, errors.New(
				"tool call function name cannot be empty",
			)
		}

		arguments := strings.TrimSpace(
			call.Function.Arguments,
		)
		if arguments == "" {
			arguments = "{}"
		}

		if !json.Valid([]byte(arguments)) {
			return agentcore.Message{}, fmt.Errorf(
				"tool %q returned invalid JSON arguments",
				call.Function.Name,
			)
		}

		result.ToolCalls = append(
			result.ToolCalls,
			agentcore.ToolCall{
				ID:        call.ID,
				Name:      call.Function.Name,
				Arguments: json.RawMessage(arguments),
			},
		)
	}

	return result, nil
}

func decodeAPIError(
	statusCode int,
	body []byte,
) error {
	var response apiErrorResponse

	if err := json.Unmarshal(body, &response); err == nil &&
		strings.TrimSpace(response.Error.Message) != "" {
		return fmt.Errorf(
			"model API returned HTTP %d: %s",
			statusCode,
			response.Error.Message,
		)
	}

	message := strings.TrimSpace(string(body))
	if len(message) > 500 {
		message = message[:500] + "..."
	}

	if message == "" {
		message = http.StatusText(statusCode)
	}

	return fmt.Errorf(
		"model API returned HTTP %d: %s",
		statusCode,
		message,
	)
}

type chatCompletionRequest struct {
	Model      string                  `json:"model"`
	Messages   []chatCompletionMessage `json:"messages"`
	Tools      []chatCompletionTool    `json:"tools,omitempty"`
	ToolChoice string                  `json:"tool_choice,omitempty"`
	Stream     bool                    `json:"stream"`
}

type chatCompletionMessage struct {
	Role       string                   `json:"role"`
	Content    string                   `json:"content,omitempty"`
	ToolCalls  []chatCompletionToolCall `json:"tool_calls,omitempty"`
	ToolCallID string                   `json:"tool_call_id,omitempty"`
}

type chatCompletionTool struct {
	Type     string                 `json:"type"`
	Function chatCompletionFunction `json:"function"`
}

type chatCompletionFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type chatCompletionToolCall struct {
	ID       string                         `json:"id"`
	Type     string                         `json:"type"`
	Function chatCompletionToolCallFunction `json:"function"`
}

type chatCompletionToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
	Usage   chatCompletionUsage    `json:"usage"`
}

type chatCompletionChoice struct {
	Message      chatCompletionResponseMessage `json:"message"`
	FinishReason string                        `json:"finish_reason"`
}

type chatCompletionResponseMessage struct {
	Role      string                   `json:"role"`
	Content   string                   `json:"content"`
	ToolCalls []chatCompletionToolCall `json:"tool_calls"`
}

type chatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type apiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}
