// Package classifier provides bug-vs-noise classification for Slack messages
// using Claude Haiku with caching by message timestamp.
package classifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Classifier uses Claude Haiku to determine if a Slack message is a bug or noise.
type Classifier struct {
	apiKey     string
	httpClient *http.Client
	cache      map[string]string // messageTS -> "bug" or "noise"
	mu         sync.RWMutex
}

// NewClassifier creates a new bug classifier with the given Anthropic API key.
func NewClassifier(apiKey string) *Classifier {
	return &Classifier{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: make(map[string]string),
	}
}

// Classification result.
type Classification string

const (
	Bug   Classification = "bug"
	Noise Classification = "noise"
)

// Classify determines if a Slack message text describes a bug or is noise.
// Results are cached by messageTimestamp to avoid redundant API calls.
func (c *Classifier) Classify(messageText, messageTimestamp string) (Classification, error) {
	// Check cache first
	c.mu.RLock()
	if cached, ok := c.cache[messageTimestamp]; ok {
		c.mu.RUnlock()
		return Classification(cached), nil
	}
	c.mu.RUnlock()

	// Call Claude Haiku
	result, err := c.callHaiku(messageText)
	if err != nil {
		return "", err
	}

	// Cache the result
	c.mu.Lock()
	c.cache[messageTimestamp] = string(result)
	c.mu.Unlock()

	return result, nil
}

// callHaiku makes an API call to Claude Haiku with the classification prompt.
func (c *Classifier) callHaiku(messageText string) (Classification, error) {
	prompt := fmt.Sprintf(
		"Decide if this Slack message reports a software defect that requires a code change. "+
			"Reply with one word: `bug` or `noise`.\n\nMessage:\n%s",
		messageText,
	)

	requestBody := map[string]interface{}{
		"model":      "claude-3-5-haiku-20241022",
		"max_tokens": 10,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("classifier: failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("classifier: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("classifier: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("classifier: API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("classifier: failed to read response: %w", err)
	}

	var response struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("classifier: failed to parse response: %w", err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("classifier: empty response from API")
	}

	text := strings.TrimSpace(strings.ToLower(response.Content[0].Text))
	if strings.Contains(text, "bug") {
		return Bug, nil
	}
	return Noise, nil
}
