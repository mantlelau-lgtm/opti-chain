// Package llmclient is a minimal OpenAI-compatible chat client (function
// calling) targeting the local LLM gateway. It carries just enough surface to
// run an agent loop: messages with tool calls and a `tools` schema.
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

// Message is a chat message. Tool-call and tool-result round-trips reuse the
// ToolCalls / ToolCallID fields.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall is one function invocation requested by the model.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Tool is a function definition sent to the model (OpenAI tools format).
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function describes a callable function and its JSON schema.
type Function struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Choice is a single completion alternative.
type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// ChatResponse is the OpenAI chat.completion envelope.
type ChatResponse struct {
	Choices []Choice `json:"choices"`
}

// Client talks to one model on the gateway.
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
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

// Chat performs a single completion round with optional tool definitions.
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
