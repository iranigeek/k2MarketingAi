package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	maxRetries    = 3
	baseRetryWait = 2 * time.Second
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

	return c.doRequestWithRetry(ctx, body)
}

// RawRequest sends an arbitrary JSON payload to the Gemini generateContent
// endpoint and returns the first candidate text. This enables multimodal
// requests (e.g. inline PDF/image data) that ChatCompletion cannot express.
func (c *GeminiClient) RawRequest(ctx context.Context, payload map[string]any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal gemini payload: %w", err)
	}

	return c.doRequestWithRetry(ctx, body)
}

// ──────────────────────────────────────────────────────────────────────
// doRequestWithRetry sends a JSON body to Gemini and retries on
// transient errors (429 rate-limit, 500, 503 overloaded, network
// errors, empty candidates). Uses exponential backoff with jitter.
// ──────────────────────────────────────────────────────────────────────

func (c *GeminiClient) doRequestWithRetry(ctx context.Context, body []byte) (string, error) {
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

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter: 2s, 4s, 8s (±25%)
			wait := baseRetryWait * time.Duration(1<<(attempt-1))
			jitter := time.Duration(float64(wait) * (0.75 + rand.Float64()*0.5))
			log.Printf("gemini: retry %d/%d after %v (prev error: %v)", attempt, maxRetries, jitter, lastErr)
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("gemini: context cancelled during retry: %w", ctx.Err())
			case <-time.After(jitter):
			}
		}

		result, err, retryable := c.doSingleRequest(ctx, endpoint, body)
		if err == nil {
			if attempt > 0 {
				log.Printf("gemini: succeeded on retry %d", attempt)
			}
			return result, nil
		}

		lastErr = err
		if !retryable {
			return "", err
		}
	}

	return "", fmt.Errorf("gemini: all %d retries exhausted: %w", maxRetries, lastErr)
}

// doSingleRequest performs one HTTP call and returns (result, error, retryable).
func (c *GeminiClient) doSingleRequest(ctx context.Context, endpoint string, body []byte) (string, error, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("gemini request: %w", err), false
	}
	req.Header.Set("Content-Type", "application/json")

	if c.tokenSource != nil {
		token, err := c.tokenSource.Token()
		if err != nil {
			return "", fmt.Errorf("gemini: fetch oauth token: %w", err), true
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		// Network errors are always retryable
		return "", fmt.Errorf("gemini perform request: %w", err), true
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var failure struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(bodyBytes, &failure)

		errMsg := fmt.Errorf("gemini status %d: %s", resp.StatusCode, failure.Error.Message)

		// 429 (rate limit), 500, 502, 503, 504 are retryable
		retryable := resp.StatusCode == 429 || resp.StatusCode >= 500
		return "", errMsg, retryable
	}

	var completion struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return "", fmt.Errorf("gemini decode response: %w", err), true
	}

	if len(completion.Candidates) == 0 || len(completion.Candidates[0].Content.Parts) == 0 {
		// Empty candidates can happen due to safety filters — retryable
		reason := ""
		if len(completion.Candidates) > 0 {
			reason = completion.Candidates[0].FinishReason
		}
		return "", fmt.Errorf("gemini returned no candidates (finishReason=%s)", reason), true
	}

	var parts []string
	for _, part := range completion.Candidates[0].Content.Parts {
		if trimmed := strings.TrimSpace(part.Text); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("gemini candidate missing text"), true
	}
	return strings.Join(parts, "\n\n"), nil, false
}

func normalizeModel(model string) string {
	clean := strings.TrimSpace(model)
	clean = strings.TrimPrefix(clean, "models/")
	if clean == "" {
		return "gemini-1.5-pro-latest"
	}
	return clean
}