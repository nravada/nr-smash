// Package memory defines the SMASHed memory store interface — the
// shared contract between the resolver (which writes) and the search
// skill (which reads).
//
// Implementations are pluggable. See ARCHITECTURE.md for the discussion
// of SQLite-vs-Bolt-vs-claude-flow as the hackathon backend.
package memory

import (
	"context"
	"time"
)

// Entry is a record of one completed smash. Resolver writes one of these
// after a successful smash; search returns these in similarity-ordered
// slices to callers.
type Entry struct {
	// SmashID — stable identifier matching the resolver's branch and change-doc.
	SmashID string `json:"smash_id"`

	// BugID — the originating bug.Bug.ID this smash resolved.
	BugID string `json:"bug_id"`

	// Repo — the repo the fix landed in, e.g. "newrelic/nri-mssql".
	Repo string `json:"repo"`

	// Pattern — categorical label for the kind of bug. Drives search hits.
	// Pull from the running enum (missing-context-timeout, swallowed-error,
	// unbounded-retry, stale-cache, nil-deref-on-empty, goroutine-leak,
	// sql-no-timeout, …); coin a new one if no existing label fits and
	// document it in docs/components/resolver.md.
	Pattern string `json:"pattern"`

	// FixShape — one-paragraph description of the fix the resolver applied.
	// Used by future resolver runs as a starting hypothesis when search
	// returns this entry above the similarity threshold.
	FixShape string `json:"fix_shape"`

	// LOCDelta — [added, removed].
	LOCDelta [2]int `json:"loc_delta"`

	// Tier — the tier this smash was classified under.
	Tier string `json:"tier"`

	// PRURL — link to the draft PR opened by the resolver.
	PRURL string `json:"pr_url"`

	// PassedTests — whether the in-smash tests passed before opening the PR.
	PassedTests bool `json:"passed_tests"`

	// ShippedAt — when the PR was opened.
	ShippedAt time.Time `json:"shipped_at"`

	// Embedding — semantic vector for similarity search. Length must match
	// the model used by the implementation (e.g. 384 for all-MiniLM-L6-v2).
	// Resolver fills this; Store implementations may overwrite if they
	// embed natively.
	Embedding []float32 `json:"embedding,omitempty"`
}

// SearchResult pairs a stored Entry with its similarity score (0..1).
type SearchResult struct {
	Entry Entry   `json:"entry"`
	Score float32 `json:"score"`
}

// Store is the SMASHed memory interface every implementation must satisfy.
type Store interface {
	// Put writes one Entry. Idempotent on SmashID — implementations should
	// upsert. Returns an error if persistence fails; resolver treats memory
	// writes as best-effort and continues on error.
	Put(ctx context.Context, e Entry) error

	// Get retrieves one Entry by SmashID. Returns (zero, false, nil) if not
	// found. Returns an error only on transport/storage failure.
	Get(ctx context.Context, smashID string) (Entry, bool, error)

	// Search returns up to k entries most similar to the query text, with
	// scores in [0, 1] (higher is more similar). Implementations may apply
	// their own minimum-score threshold; callers should filter further.
	//
	// The query is the bug's title + body concatenated, or any free text.
	Search(ctx context.Context, query string, k int) ([]SearchResult, error)
}
