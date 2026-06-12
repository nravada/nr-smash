// Package bug defines the Bug record — the cross-component contract
// every nr-smash component reads or writes.
//
// Stability: changes to this package require a [contract] PR with all
// component owners tagged. See CONTRIBUTING.md.
package bug

import "time"

// Bug is the canonical record of one issue moving through the nr-smash
// pipeline. It is created by the collector (Slack/Jira) or the triage
// agent (repo scan), enriched by the triage agent, prioritised by the
// scheduler, and consumed by the resolver.
type Bug struct {
	// ID is a globally unique identifier across all sources.
	// Format examples:
	//   jira:DI-1234
	//   slack:C0AB12CD/p1700000000123456
	//   repo:newrelic/nri-mssql/issues/42
	ID string `json:"id"`

	// Source is the channel the bug was discovered on.
	Source Source `json:"source"`

	// DiscoveredAt is when the collector or triage agent first saw the bug.
	DiscoveredAt time.Time `json:"discovered_at"`

	// Title and Body are the human description.
	Title string `json:"title"`
	Body  string `json:"body"`

	// Reporter is the originating user. Format depends on Source:
	//   jira:    Atlassian account ID
	//   slack:   Slack user ID
	//   repo-*:  GitHub login
	Reporter string `json:"reporter"`

	// URL is a link back to the source for human follow-up.
	URL string `json:"url"`

	// Labels are free-form strings carried from the source. Common values:
	// customer-asked, sev1, sev2, sev3, backend, mssql, mysql, postgres.
	Labels []string `json:"labels,omitempty"`

	// Repo is the affected repo if known, e.g. "newrelic/nri-mssql".
	// Set by the triage agent when it can map the bug to a repo.
	Repo string `json:"repo,omitempty"`

	// Tier is the smash tier classification. Set by the triage agent.
	Tier Tier `json:"tier,omitempty"`

	// Priority is the scheduling score. Set by the scheduler only.
	// Higher is more urgent. See internal/priority.
	Priority int `json:"priority,omitempty"`

	// Status tracks the bug through the pipeline.
	Status Status `json:"status"`

	// SmashID is set when the resolver claims the bug. It identifies the
	// resolver branch, the change-doc, and the SMASHed memory entry.
	SmashID string `json:"smash_id,omitempty"`
}

// Source enumerates the channels nr-smash listens on.
type Source string

const (
	SourceJira          Source = "jira"
	SourceSlack         Source = "slack"
	SourceRepoIssue     Source = "repo-issue"
	SourceRepoPRComment Source = "repo-pr-comment"
	SourceRepoLint      Source = "repo-lint"
)

// Tier mirrors the existing /smash skill tiers.
type Tier string

const (
	TierTrivial        Tier = "trivial"
	TierStandard       Tier = "standard"
	TierArchitectural  Tier = "architectural"
	TierCustomerFacing Tier = "customer-facing"
)

// Status walks the bug through the pipeline.
type Status string

const (
	StatusQueued     Status = "queued"
	StatusResolving  Status = "resolving"
	StatusResolved   Status = "resolved"
	StatusAborted    Status = "aborted"
	StatusNeedsHuman Status = "needs-human-review"
)

// HasLabel reports whether b has any of the given labels.
func (b *Bug) HasLabel(labels ...string) bool {
	for _, want := range labels {
		for _, have := range b.Labels {
			if have == want {
				return true
			}
		}
	}
	return false
}
