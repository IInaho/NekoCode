package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Anthropic struct {
	APIKey      string
	BaseURL     string
	Model       string
	maxTokens   int
	temperature float64
}

func NewAnthropic(apiKey, model string) *Anthropic {
	return &Anthropic{
		APIKey:      apiKey,
		BaseURL:     "https://api.anthropic.com/v1",
		Model:       model,
		maxTokens:   4096,
		temperature: 0.7,
	}
}

func (a *Anthropic) SetAPIKey(apiKey string) { a.APIKey = apiKey }
func (a *Anthropic) SetBaseURL(url string)   { a.BaseURL = url }

type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anthropicRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
	System      string          `json:"system,omitempty"`
	Messages    []anthropicMsg  `json:"messages"`
	Tools       []anthropicTool `json:"tools,omitempty"`
	Stream      bool            `json:"stream"`
}

type anthropicMsg struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicResponse struct {
	ID      string                  `json:"id"`
	Content []anthropicContentBlock `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func toAnthropicTools(tools []ToolDef) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]anthropicTool, len(tools))
	for i, t := range tools {
		result[i] = anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		}
	}
	return result
}

func toAnthropicMessages(messages []Message) ([]anthropicMsg, string) {
	var systemPrompt string
	var result []anthropicMsg

	for _, msg := range messages {
		if msg.Role == "system" {
			systemPrompt += msg.Content
			continue
		}

		role := msg.Role

		if msg.Role == "tool" {
			result = append(result, anthropicMsg{
				Role: "user",
				Content: []anthropicContentBlock{{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
				}},
			})
			continue
		}

		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			blocks := make([]anthropicContentBlock, 0, len(msg.ToolCalls)+1)
			if msg.Content != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: json.RawMessage(tc.Function.Arguments),
				})
			}
			result = append(result, anthropicMsg{Role: role, Content: blocks})
			continue
		}

		result = append(result, anthropicMsg{Role: role, Content: msg.Content})
	}

	return result, systemPrompt
}

func (a *Anthropic) Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
	anthropicMsgs, systemPrompt := toAnthropicMessages(messages)

	body := anthropicRequest{
		Model:       a.Model,
		MaxTokens:   a.maxTokens,
		Temperature: a.temperature,
		System:      systemPrompt,
		Messages:    anthropicMsgs,
		Tools:       toAnthropicTools(tools),
		Stream:      false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/messages", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	var anthResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthResp); err != nil {
		return nil, err
	}

	return anthropicToResponse(&anthResp), nil
}

func anthropicToResponse(anth *anthropicResponse) *Response {
	resp := &Response{ID: anth.ID}

	var textContent string
	var toolCalls []ToolCall

	for _, block := range anth.Content {
		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      block.Name,
					Arguments: string(block.Input),
				},
			})
		}
	}

	resp.Choices = []Choice{{
		Message: Message{
			Role:      "assistant",
			Content:   textContent,
			ToolCalls: toolCalls,
		},
	}}

	return resp
}

func (a *Anthropic) ChatStream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan StreamToken, <-chan error) {
	tokenChan := make(chan StreamToken)
	errChan := make(chan error)

	go func() {
		defer close(tokenChan)
		defer close(errChan)

		anthropicMsgs, systemPrompt := toAnthropicMessages(messages)

		body := anthropicRequest{
			Model:       a.Model,
			MaxTokens:   a.maxTokens,
			Temperature: a.temperature,
			System:      systemPrompt,
			Messages:    anthropicMsgs,
			Tools:       toAnthropicTools(tools),
			Stream:      true,
		}

		jsonBody, _ := json.Marshal(body)

		req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/messages", bytes.NewBuffer(jsonBody))
		if err != nil {
			errChan <- err
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", a.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := (&http.Client{Timeout: 0}).Do(req)
		if err != nil {
			errChan <- err
			return
		}
		defer resp.Body.Close()

		reader := NewEventReader(resp.Body)
		for {
			chunk, err := reader.Read()
			if err != nil {
				if err != io.EOF {
					errChan <- err
				}
				return
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				tokenChan <- StreamToken{Content: chunk.Choices[0].Delta.Content}
			}
		}
	}()

	return tokenChan, errChan
}
