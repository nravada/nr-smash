// collector — polls Slack help channels and Jira boards for bug-shaped messages.
//
// Inputs:  Slack channel IDs, Jira project keys
// Outputs: bug.Bug records with Source=slack|jira
//
// See docs/components/bug-collector.md for the full contract.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/nravada/nr-smash/internal/bug"
	"github.com/nravada/nr-smash/internal/classifier"
	"github.com/nravada/nr-smash/internal/jira"
	"github.com/nravada/nr-smash/internal/slack"
)

func main() {
	var (
		slackChannels = flag.String("slack-channels", "", "comma-separated Slack channel IDs (e.g. C0AB12CD)")
		jiraProjects  = flag.String("jira-projects", "", "comma-separated Jira project keys (e.g. DI)")
		slackLimit    = flag.Int("slack-limit", 100, "max messages to fetch per Slack channel")
		slackFile     = flag.String("slack-file", "", "read Slack messages from JSON file instead of API (for MCP workflow)")
		channelID     = flag.String("channel-id", "", "channel ID for file-based Slack messages (required with --slack-file)")
		jiraFile      = flag.String("jira-file", "", "read Jira issues from JSON file instead of API (for MCP workflow)")
		jiraBaseURL   = flag.String("jira-base-url", "https://new-relic.atlassian.net", "Jira base URL for generating issue links")
	)
	flag.Parse()

	// File-based mode for MCP workflow
	if *slackFile != "" && *jiraFile != "" {
		// Process both files
		processSlackFile(*slackFile, *channelID)
		processJiraFile(*jiraFile, *jiraBaseURL)
		return
	}

	if *slackFile != "" {
		if *channelID == "" {
			fmt.Fprintln(os.Stderr, "collector: --channel-id required when using --slack-file")
			os.Exit(2)
		}
		processSlackFile(*slackFile, *channelID)
		return
	}

	if *jiraFile != "" {
		processJiraFile(*jiraFile, *jiraBaseURL)
		return
	}

	if *slackChannels == "" && *jiraProjects == "" {
		fmt.Fprintln(os.Stderr, "collector: at least one of --slack-channels or --jira-projects is required")
		os.Exit(2)
	}

	// Read required environment variables
	slackToken := os.Getenv("SLACK_BOT_TOKEN")
	jiraBaseURLEnv := os.Getenv("JIRA_BASE_URL")
	jiraEmail := os.Getenv("JIRA_EMAIL")
	jiraAPIToken := os.Getenv("JIRA_API_TOKEN")
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")

	// Validate Slack env if Slack channels requested
	if *slackChannels != "" && (slackToken == "" || anthropicKey == "") {
		fmt.Fprintln(os.Stderr, "collector: SLACK_BOT_TOKEN and ANTHROPIC_API_KEY required for Slack collection")
		os.Exit(1)
	}

	// Validate Jira env if Jira projects requested
	if *jiraProjects != "" && (jiraBaseURLEnv == "" || jiraEmail == "" || jiraAPIToken == "") {
		fmt.Fprintln(os.Stderr, "collector: JIRA_BASE_URL, JIRA_EMAIL, and JIRA_API_TOKEN required for Jira collection")
		os.Exit(1)
	}

	// Collect bugs from all sources
	bugs := make(map[string]*bug.Bug) // dedupe by Bug.ID
	discoveredAt := time.Now()

	// Collect from Slack
	if *slackChannels != "" {
		slackClient := slack.NewClient(slackToken)
		bugClassifier := classifier.NewClassifier(anthropicKey)

		channelIDs := strings.Split(*slackChannels, ",")
		for _, channelID := range channelIDs {
			channelID = strings.TrimSpace(channelID)
			if channelID == "" {
				continue
			}

			log.Printf("collector: fetching Slack channel %s", channelID)
			messages, err := slackClient.GetChannelHistory(channelID, *slackLimit)
			if err != nil {
				log.Printf("collector: failed to fetch Slack channel %s: %v", channelID, err)
				continue
			}

			log.Printf("collector: classifying %d messages from %s", len(messages), channelID)
			for _, msg := range messages {
				// Skip non-user messages
				if msg.Type != "message" || msg.User == "" {
					continue
				}

				// Classify bug vs noise
				classification, err := bugClassifier.Classify(msg.Text, msg.Timestamp)
				if err != nil {
					log.Printf("collector: failed to classify message %s: %v", msg.Timestamp, err)
					continue
				}

				if classification != classifier.Bug {
					continue
				}

				// Normalize timestamp for ID (remove decimal point)
				tsID := strings.ReplaceAll(msg.Timestamp, ".", "")
				bugID := fmt.Sprintf("slack:%s/p%s", channelID, tsID)

				// Build Bug record
				b := &bug.Bug{
					ID:           bugID,
					Source:       bug.SourceSlack,
					DiscoveredAt: discoveredAt,
					Title:        truncate(msg.Text, 100), // First 100 chars as title
					Body:         msg.Text,
					Reporter:     msg.User,
					URL:          slack.MakePermalink(channelID, tsID),
					Status:       bug.StatusQueued,
				}

				bugs[bugID] = b
				log.Printf("collector: detected bug %s", bugID)
			}
		}
	}

	// Collect from Jira
	if *jiraProjects != "" {
		jiraClient := jira.NewClient(jiraBaseURLEnv, jiraEmail, jiraAPIToken)

		projectKeys := strings.Split(*jiraProjects, ",")
		for _, projectKey := range projectKeys {
			projectKey = strings.TrimSpace(projectKey)
			if projectKey == "" {
				continue
			}

			log.Printf("collector: fetching Jira project %s", projectKey)
			issues, err := jiraClient.GetBugIssues(projectKey)
			if err != nil {
				log.Printf("collector: failed to fetch Jira project %s: %v", projectKey, err)
				continue
			}

			log.Printf("collector: found %d bug issues in %s", len(issues), projectKey)
			for _, issue := range issues {
				bugID := fmt.Sprintf("jira:%s", issue.Key)

				// Build Bug record
				b := &bug.Bug{
					ID:           bugID,
					Source:       bug.SourceJira,
					DiscoveredAt: discoveredAt,
					Title:        issue.Fields.Summary,
					Body:         issue.Fields.Description,
					Reporter:     issue.Fields.Reporter.AccountID,
					URL:          jiraClient.MakeIssueURL(issue.Key),
					Labels:       issue.Fields.Labels,
					Status:       bug.StatusQueued,
				}

				bugs[bugID] = b
				log.Printf("collector: detected bug %s", bugID)
			}
		}
	}

	// Output JSONL to stdout
	for _, b := range bugs {
		data, err := json.Marshal(b)
		if err != nil {
			log.Printf("collector: failed to marshal bug %s: %v", b.ID, err)
			continue
		}
		fmt.Println(string(data))
	}

	log.Printf("collector: emitted %d bugs", len(bugs))
}

// processSlackFile reads Slack messages from a JSON file (for MCP workflow)
// and processes them through the classifier, outputting bug.Bug records.
func processSlackFile(filePath, channelID string) {
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		fmt.Fprintln(os.Stderr, "collector: ANTHROPIC_API_KEY required for classification")
		os.Exit(1)
	}

	// Read the JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("collector: failed to read file %s: %v", filePath, err)
	}

	// Parse messages
	var messages []slack.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		log.Fatalf("collector: failed to parse JSON: %v", err)
	}

	log.Printf("collector: processing %d messages from %s", len(messages), filePath)

	// Classify and build bugs
	bugs := make(map[string]*bug.Bug)
	discoveredAt := time.Now()
	bugClassifier := classifier.NewClassifier(anthropicKey)

	for _, msg := range messages {
		// Skip non-user messages
		if msg.Type != "message" || msg.User == "" {
			continue
		}

		// Classify bug vs noise
		classification, err := bugClassifier.Classify(msg.Text, msg.Timestamp)
		if err != nil {
			log.Printf("collector: failed to classify message %s: %v", msg.Timestamp, err)
			continue
		}

		if classification != classifier.Bug {
			continue
		}

		// Normalize timestamp for ID (remove decimal point)
		tsID := strings.ReplaceAll(msg.Timestamp, ".", "")
		bugID := fmt.Sprintf("slack:%s/p%s", channelID, tsID)

		// Build Bug record
		b := &bug.Bug{
			ID:           bugID,
			Source:       bug.SourceSlack,
			DiscoveredAt: discoveredAt,
			Title:        truncate(msg.Text, 100),
			Body:         msg.Text,
			Reporter:     msg.User,
			URL:          slack.MakePermalink(channelID, tsID),
			Status:       bug.StatusQueued,
		}

		bugs[bugID] = b
		log.Printf("collector: detected bug %s", bugID)
	}

	// Output JSONL to stdout
	for _, b := range bugs {
		data, err := json.Marshal(b)
		if err != nil {
			log.Printf("collector: failed to marshal bug %s: %v", b.ID, err)
			continue
		}
		fmt.Println(string(data))
	}

	log.Printf("collector: emitted %d bugs", len(bugs))
}

// processJiraFile reads Jira issues from a JSON file (for MCP workflow)
// and processes them, outputting bug.Bug records.
func processJiraFile(filePath, baseURL string) {
	// Read the JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("collector: failed to read file %s: %v", filePath, err)
	}

	// Parse issues
	var issues []jira.Issue
	if err := json.Unmarshal(data, &issues); err != nil {
		log.Fatalf("collector: failed to parse JSON: %v", err)
	}

	log.Printf("collector: processing %d Jira issues from %s", len(issues), filePath)

	// Build bugs
	bugs := make(map[string]*bug.Bug)
	discoveredAt := time.Now()
	client := jira.NewClient(baseURL, "", "") // Just for URL generation

	for _, issue := range issues {
		bugID := fmt.Sprintf("jira:%s", issue.Key)

		// Build Bug record
		b := &bug.Bug{
			ID:           bugID,
			Source:       bug.SourceJira,
			DiscoveredAt: discoveredAt,
			Title:        issue.Fields.Summary,
			Body:         issue.Fields.Description,
			Reporter:     issue.Fields.Reporter.AccountID,
			URL:          client.MakeIssueURL(issue.Key),
			Labels:       issue.Fields.Labels,
			Status:       bug.StatusQueued,
		}

		bugs[bugID] = b
		log.Printf("collector: detected bug %s", bugID)
	}

	// Output JSONL to stdout
	for _, b := range bugs {
		data, err := json.Marshal(b)
		if err != nil {
			log.Printf("collector: failed to marshal bug %s: %v", b.ID, err)
			continue
		}
		fmt.Println(string(data))
	}

	log.Printf("collector: emitted %d bugs", len(bugs))
}

// truncate shortens a string to maxLen characters, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
