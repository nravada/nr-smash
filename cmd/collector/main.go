// collector — polls Slack help channels and Jira boards for bug-shaped messages.
//
// Inputs:  Slack channel IDs, Jira project keys
// Outputs: bug.Bug records with Source=slack|jira
//
// See docs/components/bug-collector.md for the full contract.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		slackChannels = flag.String("slack-channels", "", "comma-separated Slack channel IDs (e.g. C0AB12CD)")
		jiraProjects  = flag.String("jira-projects", "", "comma-separated Jira project keys (e.g. DI)")
	)
	flag.Parse()

	if *slackChannels == "" && *jiraProjects == "" {
		fmt.Fprintln(os.Stderr, "collector: at least one of --slack-channels or --jira-projects is required")
		os.Exit(2)
	}

	// TODO(collector-owner): for each Slack channel, poll new messages and
	// classify bug-vs-noise via Haiku; for each Jira project, fetch tickets
	// labelled `bug` or in defined statuses. Emit bug.Bug records.
	// See ARCHITECTURE.md and docs/components/bug-collector.md.
	fmt.Println("collector: not implemented yet — see docs/components/bug-collector.md")
}
