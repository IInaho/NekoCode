package llm

import (
	"context"
	"net/http"
	"time"
)

var sharedTransport = &http.Transport{
	MaxIdleConns:        20,
	IdleConnTimeout:     90 * time.Second,
	DisableCompression:  false,
}

var SharedHTTPClient = &http.Client{Transport: sharedTransport}

var SharedHTTPClientTimeout = &http.Client{
	Transport: sharedTransport,
	Timeout:   120 * time.Second,
}

type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	Name             string     `json:"name,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	Index    int          `json:"index"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Choice struct {
	Message      Message `json:"message"`
	Delta        Delta   `json:"delta"`
	FinishReason string  `json:"finish_reason"`
}

type Delta struct {
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Response struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type StreamChunk struct {
	Choices []struct {
		Delta        Delta  `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *StreamUsage `json:"usage"`
}

type StreamToken struct {
	Content          string
	ReasoningContent string
	ToolCallDelta    *ToolCallDelta // non-nil when streaming a tool call fragment
	Usage            *StreamUsage   // final chunk carries usage
}

type ToolCallDelta struct {
	Index     int    // which tool call (0-based)
	ID        string // set on first fragment
	Name      string // function name, set on first fragment
	Arguments string // JSON fragment, accumulated across chunks
}

type StreamUsage struct {
	PromptTokens     int
	CompletionTokens int
}

type ToolDef struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

type Parameters struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

type LLM interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error)
	ChatStream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan StreamToken, <-chan error)
	SetAPIKey(apiKey string)
	SetBaseURL(url string)
}

