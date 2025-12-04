package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ChatMessage mirrors OpenAI chat message format.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIClient wraps minimal functionality needed for chat completions.
type OpenAIClient struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenAIClient constructs a client using the provided API key and default model.
func NewOpenAIClient(apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

// ChatCompletion sends chat messages to OpenAI and returns the first response content.
func (c *OpenAIClient) ChatCompletion(ctx context.Context, messages []ChatMessage, temperature float64) (string, error) {
	payload := map[string]any{
		"model":       c.model,
		"temperature": temperature,
		"messages":    messages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal openai payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var failure struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&failure)
		return "", fmt.Errorf("openai status %d: %s", resp.StatusCode, failure.Error.Message)
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}
	return completion.Choices[0].Message.Content, nil
}
