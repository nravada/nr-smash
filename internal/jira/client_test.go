package jira

import (
	"testing"
)

func TestMakeIssueURL(t *testing.T) {
	client := NewClient("https://new-relic.atlassian.net", "test@example.com", "fake-token")

	tests := []struct {
		name     string
		issueKey string
		want     string
	}{
		{
			name:     "DI project issue",
			issueKey: "DI-1234",
			want:     "https://new-relic.atlassian.net/browse/DI-1234",
		},
		{
			name:     "different project",
			issueKey: "PLAT-567",
			want:     "https://new-relic.atlassian.net/browse/PLAT-567",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.MakeIssueURL(tt.issueKey)
			if got != tt.want {
				t.Errorf("MakeIssueURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	baseURL := "https://new-relic.atlassian.net/"
	client := NewClient(baseURL, "test@example.com", "fake-token")

	// Should strip trailing slash
	if client.baseURL != "https://new-relic.atlassian.net" {
		t.Errorf("NewClient() baseURL = %v, want %v", client.baseURL, "https://new-relic.atlassian.net")
	}
}
