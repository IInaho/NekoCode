package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OpenAICompatible handles any OpenAI-compatible API (OpenAI, GLM, DeepSeek, etc.).
type OpenAICompatible struct {
	APIKey          string
	BaseURL         string
	Model           string
	maxTokens       int
	temperature     float64
	disableThinking bool
	reasoningEffort string // "" = default, "high"/"max" for DeepSeek
}

func (c *OpenAICompatible) SetMaxTokens(n int)              { c.maxTokens = n }
func (c *OpenAICompatible) MaxTokens() int                   { return c.maxTokens }
func (c *OpenAICompatible) SetDisableThinking(disable bool) { c.disableThinking = disable }
func (c *OpenAICompatible) SetThinkingBudget(tokens int) {
	// Map Anthropic-style token budgets to OpenAI-compat reasoning_effort levels.
	// This way callers can use SetThinkingBudget generically without provider awareness.
	if tokens < 0 {
		c.disableThinking = true
		c.reasoningEffort = ""
	} else if tokens == 0 {
		c.disableThinking = false
		c.reasoningEffort = "" // let the API use its default
	} else {
		c.disableThinking = false
		switch {
		case tokens <= 4000:
			c.reasoningEffort = "low"
		case tokens <= 8000:
			c.reasoningEffort = "high"
		default:
			c.reasoningEffort = "max"
		}
	}
}
func (c *OpenAICompatible) SetReasoningEffort(effort string) { c.reasoningEffort = effort }

func newOpenAICompat(apiKey, baseURL, model string) *OpenAICompatible {
	return &OpenAICompatible{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       model,
		maxTokens:   32000,
		temperature: 0.7,
	}
}

func NewOpenAI(apiKey, baseURL, model string) *OpenAICompatible {
	return newOpenAICompat(apiKey, baseURL, model)
}

func NewGLM(apiKey, baseURL, model string) *OpenAICompatible {
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/paas/v4"
	}
	return newOpenAICompat(apiKey, baseURL, model)
}

func (c *OpenAICompatible) SetAPIKey(apiKey string) { c.APIKey = apiKey }
func (c *OpenAICompatible) SetBaseURL(url string)   { c.BaseURL = url }

func (c *OpenAICompatible) Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
	body := map[string]interface{}{
		"model":       c.Model,
		"messages":    messages,
		"max_tokens":  c.maxTokens,
		"temperature": c.temperature,
		"stream":      false,
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	if c.disableThinking {
		body["thinking"] = map[string]string{"type": "disabled"}
	} else if c.reasoningEffort != "" {
		body["reasoning_effort"] = c.reasoningEffort
		body["thinking"] = map[string]string{"type": "enabled"}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := SharedHTTPClientTimeout.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var response Response
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *OpenAICompatible) ChatStream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan StreamToken, <-chan error) {
	body := map[string]interface{}{
		"model":       c.Model,
		"messages":    messages,
		"max_tokens":  c.maxTokens,
		"temperature": c.temperature,
		"stream":      true,
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	if c.disableThinking {
		body["thinking"] = map[string]string{"type": "disabled"}
	} else if c.reasoningEffort != "" {
		body["reasoning_effort"] = c.reasoningEffort
		body["thinking"] = map[string]string{"type": "enabled"}
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		tokenChan := make(chan StreamToken)
		errChan := make(chan error, 1)
		errChan <- err
		close(tokenChan)
		return tokenChan, errChan
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := SharedHTTPStreamClient.Do(req)
	if err != nil {
		tokenChan := make(chan StreamToken)
		errChan := make(chan error, 1)
		errChan <- err
		close(tokenChan)
		return tokenChan, errChan
	}

	tokenChan := make(chan StreamToken)
	errChan := make(chan error, 1)

	go func() {
		defer close(tokenChan)
		defer close(errChan)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
			return
		}

		reader := NewEventReader(resp.Body)
		for {
			chunk, err := reader.Read()
			if err != nil {
				if err != io.EOF {
					errChan <- err
				}
				return
			}
			if chunk == nil || len(chunk.Choices) == 0 {
				continue
			}
				delta := chunk.Choices[0].Delta
				token := StreamToken{
					Content:          delta.Content,
					ReasoningContent: delta.ReasoningContent,
					Usage:            chunk.Usage,
					FinishReason:     chunk.Choices[0].FinishReason,
				}
				if token.Content != "" || token.ReasoningContent != "" || token.Usage != nil || token.FinishReason != "" {
					tokenChan <- token
				}
				for _, tc := range delta.ToolCalls {
					tokenChan <- StreamToken{
						ToolCallDelta: &ToolCallDelta{
							Index: tc.Index, ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments,
						},
					}
				}
		}
	}()

	return tokenChan, errChan
}
