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

type GLM struct {
	APIKey      string
	BaseURL     string
	Model       string
	maxTokens   int
	temperature float64
}

func NewGLM(apiKey, baseURL, model string) *GLM {
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/paas/v4"
	}
	return &GLM{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       model,
		maxTokens:   4096,
		temperature: 0.7,
	}
}

func (g *GLM) SetAPIKey(apiKey string) { g.APIKey = apiKey }
func (g *GLM) SetBaseURL(url string)   { g.BaseURL = url }

func (g *GLM) Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
	body := map[string]interface{}{
		"model":       g.Model,
		"messages":    messages,
		"max_tokens":  g.maxTokens,
		"temperature": g.temperature,
		"stream":      false,
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", g.BaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.APIKey))

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
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var response Response
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (g *GLM) ChatStream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan StreamToken, <-chan error) {
	body := map[string]interface{}{
		"model":       g.Model,
		"messages":    messages,
		"max_tokens":  g.maxTokens,
		"temperature": g.temperature,
		"stream":      true,
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", g.BaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		errChan := make(chan error, 1)
		errChan <- err
		return nil, errChan
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.APIKey))

	resp, err := (&http.Client{Timeout: 0}).Do(req)
	if err != nil {
		errChan := make(chan error, 1)
		errChan <- err
		return nil, errChan
	}

	tokenChan := make(chan StreamToken)
	errChan := make(chan error)

	go func() {
		defer close(tokenChan)
		defer close(errChan)
		defer resp.Body.Close()

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
			if delta.Content != "" || delta.ReasoningContent != "" {
				tokenChan <- StreamToken{
					Content:          delta.Content,
					ReasoningContent: delta.ReasoningContent,
				}
			}
		}
	}()

	return tokenChan, errChan
}
