package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// ChatMessage represents a generic chat turn in the prompt history.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client defines the behaviour required by the generation package.
type Client interface {
	ChatCompletion(ctx context.Context, messages []ChatMessage, temperature float64) (string, error)
}

// GeminiClient wraps the Google Generative Language API.
type GeminiClient struct {
	apiKey      string
	model       string
	client      *http.Client
	tokenSource oauth2.TokenSource
}

// NewGeminiClient constructs a Gemini client for the desired model.
func NewGeminiClient(apiKey, model string, timeout time.Duration, tokenSource oauth2.TokenSource) *GeminiClient {
	if model == "" {
		model = "gemini-3-pro-preview"
	}
	if timeout <= 0 {
		timeout = 1000 * time.Second
	}
	return &GeminiClient{
		apiKey:      apiKey,
		model:       normalizeModel(model),
		client:      &http.Client{Timeout: timeout},
		tokenSource: tokenSource,
	}
}

// ChatCompletion sends conversational content to Gemini and returns the first candidate text.
func (c *GeminiClient) ChatCompletion(ctx context.Context, messages []ChatMessage, temperature float64) (string, error) {
	var systemPrompts []string
	var contents []map[string]any

	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		switch role {
		case "system":
			systemPrompts = append(systemPrompts, msg.Content)
			continue
		case "assistant":
			role = "model"
		default:
			role = "user"
		}

		contents = append(contents, map[string]any{
			"role": role,
			"parts": []map[string]string{
				{"text": msg.Content},
			},
		})
	}

	if len(contents) == 0 {
		return "", fmt.Errorf("gemini: missing user or assistant messages")
	}

	payload := map[string]any{
		"contents": contents,
		"generationConfig": map[string]any{
			"temperature": temperature,
		},
	}

	if len(systemPrompts) > 0 {
		payload["systemInstruction"] = map[string]any{
			"parts": []map[string]string{
				{"text": strings.Join(systemPrompts, "\n\n")},
			},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal gemini payload: %w", err)
	}

	model := c.model
	if override := modelFromContext(ctx); override != "" {
		model = override
	}

	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
		url.PathEscape(model),
	)
	if c.tokenSource == nil {
		if strings.TrimSpace(c.apiKey) == "" {
			return "", fmt.Errorf("gemini: missing API key or service account credentials")
		}
		endpoint = fmt.Sprintf("%s?key=%s", endpoint, url.QueryEscape(c.apiKey))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if c.tokenSource != nil {
		token, err := c.tokenSource.Token()
		if err != nil {
			return "", fmt.Errorf("gemini: fetch oauth token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var failure struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&failure)
		return "", fmt.Errorf("gemini status %d: %s", resp.StatusCode, failure.Error.Message)
	}

	var completion struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return "", fmt.Errorf("gemini decode response: %w", err)
	}

	if len(completion.Candidates) == 0 || len(completion.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned no candidates")
	}

	var parts []string
	for _, part := range completion.Candidates[0].Content.Parts {
		if trimmed := strings.TrimSpace(part.Text); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("gemini candidate missing text")
	}
	return strings.Join(parts, "\n\n"), nil
}

func normalizeModel(model string) string {
	clean := strings.TrimSpace(model)
	clean = strings.TrimPrefix(clean, "models/")
	if clean == "" {
		return "gemini-1.5-pro-latest"
	}
	return clean
}
