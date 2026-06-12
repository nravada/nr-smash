// Package slack provides a minimal Slack client for fetching channel history.
package slack

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client wraps Slack Web API calls.
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient returns a Slack client authenticated with the given bot token.
func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Message represents a Slack message from channel history.
type Message struct {
	Type      string `json:"type"`
	Timestamp string `json:"ts"`
	User      string `json:"user"`
	Text      string `json:"text"`
	ThreadTS  string `json:"thread_ts,omitempty"`
}

// ChannelHistoryResponse is the Slack API response for conversations.history.
type ChannelHistoryResponse struct {
	OK       bool      `json:"ok"`
	Messages []Message `json:"messages"`
	Error    string    `json:"error,omitempty"`
}

// GetChannelHistory fetches recent messages from a Slack channel.
// channelID is the channel ID (e.g., "C0AB12CD").
// limit is the maximum number of messages to fetch (max 1000 per Slack API).
func (c *Client) GetChannelHistory(channelID string, limit int) ([]Message, error) {
	apiURL := "https://slack.com/api/conversations.history"

	params := url.Values{}
	params.Set("channel", channelID)
	params.Set("limit", fmt.Sprintf("%d", limit))

	req, err := http.NewRequest("GET", apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("slack: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slack: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("slack: failed to read response: %w", err)
	}

	var historyResp ChannelHistoryResponse
	if err := json.Unmarshal(body, &historyResp); err != nil {
		return nil, fmt.Errorf("slack: failed to parse response: %w", err)
	}

	if !historyResp.OK {
		return nil, fmt.Errorf("slack: API error: %s", historyResp.Error)
	}

	return historyResp.Messages, nil
}

// MakePermalink generates a Slack permalink for a message.
func MakePermalink(channelID, timestamp string) string {
	return fmt.Sprintf("https://newrelic.slack.com/archives/%s/p%s", channelID, timestamp)
}
