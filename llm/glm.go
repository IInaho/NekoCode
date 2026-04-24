package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

func (g *GLM) SetAPIKey(apiKey string) {
	g.APIKey = apiKey
}

func (g *GLM) SetBaseURL(url string) {
	g.BaseURL = url
}

type glmRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

func (g *GLM) Chat(ctx context.Context, messages []Message) (*Response, error) {
	url := fmt.Sprintf("%s/chat/completions", g.BaseURL)

	body := glmRequest{
		Model:       g.Model,
		Messages:    messages,
		MaxTokens:   g.maxTokens,
		Temperature: g.temperature,
		Stream:      false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.APIKey))

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
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

func (g *GLM) ChatStream(ctx context.Context, messages []Message) (<-chan StreamToken, <-chan error) {
	url := fmt.Sprintf("%s/chat/completions", g.BaseURL)

	body := glmRequest{
		Model:       g.Model,
		Messages:    messages,
		MaxTokens:   g.maxTokens,
		Temperature: g.temperature,
		Stream:      true,
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		errChan := make(chan error, 1)
		errChan <- err
		return nil, errChan
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.APIKey))

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
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
			log.Printf("GLM delta: content=%q reasoning=%q", delta.Content, delta.ReasoningContent)
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
