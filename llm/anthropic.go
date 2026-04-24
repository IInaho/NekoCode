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

func (a *Anthropic) SetAPIKey(apiKey string) {
	a.APIKey = apiKey
}

func (a *Anthropic) SetBaseURL(url string) {
	a.BaseURL = url
}

type anthropicRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (a *Anthropic) Chat(ctx context.Context, messages []Message) (*Response, error) {
	url := fmt.Sprintf("%s/messages", a.BaseURL)

	anthropicMessages := make([]Message, len(messages))
	for i, msg := range messages {
		role := msg.Role
		if role == "system" {
			role = "user"
			msg.Content = "System: " + msg.Content
		}
		anthropicMessages[i] = Message{
			Role:    role,
			Content: msg.Content,
		}
	}

	body := anthropicRequest{
		Model:       a.Model,
		MaxTokens:   a.maxTokens,
		Temperature: a.temperature,
		Messages:    anthropicMessages,
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
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, err
	}

	response := &Response{
		ID: anthropicResp.ID,
		Choices: []Choice{
			{
				Message: Message{
					Role:    "assistant",
					Content: anthropicResp.Content[0].Text,
				},
			},
		},
		Usage: Usage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
		},
	}

	return response, nil
}

func (a *Anthropic) ChatStream(ctx context.Context, messages []Message) (<-chan StreamToken, <-chan error) {
	tokenChan := make(chan StreamToken)
	errChan := make(chan error)

	go func() {
		defer close(tokenChan)
		defer close(errChan)

		url := fmt.Sprintf("%s/messages", a.BaseURL)

		anthropicMessages := make([]Message, len(messages))
		for i, msg := range messages {
			role := msg.Role
			if role == "system" {
				role = "user"
				msg.Content = "System: " + msg.Content
			}
			anthropicMessages[i] = Message{
				Role:    role,
				Content: msg.Content,
			}
		}

		body := anthropicRequest{
			Model:       a.Model,
			MaxTokens:   a.maxTokens,
			Temperature: a.temperature,
			Messages:    anthropicMessages,
			Stream:      true,
		}

		jsonBody, _ := json.Marshal(body)

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			errChan <- err
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", a.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		client := &http.Client{Timeout: 0}
		resp, err := client.Do(req)
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
