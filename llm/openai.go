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

type OpenAI struct {
	APIKey      string
	BaseURL     string
	Model       string
	maxTokens   int
	temperature float64
}

func NewOpenAI(apiKey, baseURL, model string) *OpenAI {
	return &OpenAI{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       model,
		maxTokens:   4096,
		temperature: 0.7,
	}
}

func (o *OpenAI) SetAPIKey(apiKey string) { o.APIKey = apiKey }
func (o *OpenAI) SetBaseURL(url string)   { o.BaseURL = url }

func (o *OpenAI) Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
	body := map[string]interface{}{
		"model":       o.Model,
		"messages":    messages,
		"max_tokens":  o.maxTokens,
		"temperature": o.temperature,
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

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.APIKey))

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

func (o *OpenAI) ChatStream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan StreamToken, <-chan error) {
	body := map[string]interface{}{
		"model":       o.Model,
		"messages":    messages,
		"max_tokens":  o.maxTokens,
		"temperature": o.temperature,
		"stream":      true,
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		errChan := make(chan error, 1)
		errChan <- err
		return nil, errChan
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.APIKey))

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
			if chunk == nil {
				continue
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				tokenChan <- StreamToken{Content: chunk.Choices[0].Delta.Content}
			}
		}
	}()

	return tokenChan, errChan
}
