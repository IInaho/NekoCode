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
}

func (c *OpenAICompatible) SetMaxTokens(n int)              { c.maxTokens = n }
func (c *OpenAICompatible) MaxTokens() int                   { return c.maxTokens }
func (c *OpenAICompatible) SetDisableThinking(disable bool) { c.disableThinking = disable }

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
		if c.disableThinking {
			body["thinking"] = map[string]string{"type": "disabled"}
		}
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
		if c.disableThinking {
			body["thinking"] = map[string]string{"type": "disabled"}
		}
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		errChan := make(chan error, 1)
		errChan <- err
		return nil, errChan
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := SharedHTTPClient.Do(req)
	if err != nil {
		errChan := make(chan error, 1)
		errChan <- err
		return nil, errChan
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
