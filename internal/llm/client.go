package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("LLM 未配置，请设置 LLM_BASE_URL、LLM_API_KEY 和 LLM_MODEL")

// Client is a minimal OpenAI-compatible chat completions client.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type Message struct {
	Role       string        `json:"role"`
	Content    string        `json:"content"`
	Parts      []ContentPart `json:"-"`
	ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

// MarshalJSON 保持文本消息的字符串格式，仅图片消息使用多模态内容数组。
func (m Message) MarshalJSON() ([]byte, error) {
	var content any = m.Content
	if len(m.Parts) > 0 {
		content = m.Parts
	}
	return json.Marshal(struct {
		Role       string     `json:"role"`
		Content    any        `json:"content"`
		ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
		ToolCallID string     `json:"tool_call_id,omitempty"`
	}{m.Role, content, m.ToolCalls, m.ToolCallID})
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}
type FunctionDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type chatRequest struct {
	Tools []Tool `json:"tools,omitempty"`

	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		model:   strings.TrimSpace(model),
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && c.apiKey != "" && c.model != ""
}

func (c *Client) CompleteJSON(ctx context.Context, messages []Message) (string, error) {
	message, err := c.complete(ctx, chatRequest{Model: c.model, Messages: messages, Temperature: 0, ResponseFormat: &responseFormat{Type: "json_object"}})
	if err == nil && strings.TrimSpace(message.Content) == "" {
		return "", errors.New("LLM 响应为空")
	}
	return strings.TrimSpace(message.Content), err
}

func (c *Client) CompleteTools(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	return c.complete(ctx, chatRequest{Model: c.model, Messages: messages, Temperature: 0, Tools: tools})
}

func (c *Client) complete(ctx context.Context, request chatRequest) (Message, error) {
	if !c.Configured() {
		return Message{}, ErrNotConfigured
	}
	body, err := json.Marshal(request)
	if err != nil {
		return Message{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Message{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Message{}, err
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Message{}, fmt.Errorf("解析 LLM 响应失败: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return Message{}, fmt.Errorf("LLM 请求失败: %s", parsed.Error.Message)
		}
		return Message{}, fmt.Errorf("LLM 请求失败，HTTP 状态码: %d", resp.StatusCode)
	}

	if len(parsed.Choices) == 0 || (strings.TrimSpace(parsed.Choices[0].Message.Content) == "" && len(parsed.Choices[0].Message.ToolCalls) == 0) {
		return Message{}, errors.New("LLM 响应为空")
	}

	return parsed.Choices[0].Message, nil
}
