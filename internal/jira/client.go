// Package jira provides a minimal Jira client for fetching bug tickets.
package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client wraps Jira REST API calls.
type Client struct {
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
}

// NewClient returns a Jira client authenticated with email and API token.
// baseURL should be the Jira instance URL (e.g., "https://new-relic.atlassian.net").
func NewClient(baseURL, email, apiToken string) *Client {
	return &Client{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		email:    email,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Issue represents a Jira issue with the fields we care about.
type Issue struct {
	Key    string `json:"key"`
	Fields Fields `json:"fields"`
}

// Fields contains the issue fields we need.
type Fields struct {
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Reporter    Reporter `json:"reporter"`
	Labels      []string `json:"labels"`
	Status      Status   `json:"status"`
}

// Reporter contains the issue reporter info.
type Reporter struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
}

// Status contains the issue status.
type Status struct {
	Name string `json:"name"`
}

// SearchResponse is the Jira API response for JQL search.
type SearchResponse struct {
	Issues []Issue `json:"issues"`
	Total  int     `json:"total"`
}

// SearchIssues performs a JQL search and returns matching issues.
func (c *Client) SearchIssues(jql string, maxResults int) ([]Issue, error) {
	apiURL := fmt.Sprintf("%s/rest/api/3/search", c.baseURL)

	params := url.Values{}
	params.Set("jql", jql)
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	params.Set("fields", "summary,description,reporter,labels,status")

	req, err := http.NewRequest("GET", apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("jira: failed to create request: %w", err)
	}

	req.SetBasicAuth(c.email, c.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("jira: failed to read response: %w", err)
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("jira: failed to parse response: %w", err)
	}

	return searchResp.Issues, nil
}

// GetBugIssues fetches bug issues for the given project key using the default JQL.
func (c *Client) GetBugIssues(projectKey string) ([]Issue, error) {
	jql := fmt.Sprintf(
		`project = %s AND status in ("To Do", "Open", "In Progress") AND labels = bug`,
		projectKey,
	)
	return c.SearchIssues(jql, 100)
}

// MakeIssueURL generates a web URL for a Jira issue.
func (c *Client) MakeIssueURL(issueKey string) string {
	return fmt.Sprintf("%s/browse/%s", c.baseURL, issueKey)
}
