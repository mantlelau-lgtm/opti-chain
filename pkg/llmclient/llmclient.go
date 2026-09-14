package llmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ContentPart struct {
	Type     string    `json:"type"` // "text" | "image_url"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low" | "high" | "auto"
}

type Message struct {
	Role        string        `json:"role"`
	Content     string        `json:"content,omitempty"`
	ContentList []ContentPart `json:"-"`
	ToolCalls   []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID  string        `json:"tool_call_id,omitempty"`
	Name        string        `json:"name,omitempty"`
}

func (m Message) MarshalJSON() ([]byte, error) {
	type alias Message
	if len(m.ContentList) > 0 {
		return json.Marshal(&struct {
			Role       string        `json:"role"`
			Content    []ContentPart `json:"content"`
			ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
			ToolCallID string        `json:"tool_call_id,omitempty"`
			Name       string        `json:"name,omitempty"`
		}{
			Role:       m.Role,
			Content:    m.ContentList,
			ToolCalls:  m.ToolCalls,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		})
	}
	return json.Marshal((*alias)(&m))
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Client struct {
	baseURL string
	model   string
	key     string
	http    *http.Client
}

func New(baseURL, model, key string) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		key:     key,
		http:    &http.Client{Timeout: 300 * time.Second},
	}
}

// Model returns the model id this client was constructed with (fallback when
// the gateway omits the "model" field in a streamed/partial response).
func (c *Client) Model() string { return c.model }

func (c *Client) Chat(ctx context.Context, msgs []Message, tools []Tool) (*ChatResponse, error) {
	payload := map[string]any{"model": c.model, "messages": msgs}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("llm http %d: %s", resp.StatusCode, string(raw))
	}
	var out ChatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("llm decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return nil, errors.New("llm returned no choices")
	}
	return &out, nil
}
