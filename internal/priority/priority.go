// Package priority defines the scheduling signal the scheduler emits and
// the resolver consumes.
//
// The scheduler is the only writer; resolver reads bug.Priority directly
// off the Bug record.
package priority

import (
	"time"

	"github.com/nravada/nr-smash/internal/bug"
)

// Score returns a priority score for a Bug. Higher is more urgent.
//
// The exact formula is owned by the scheduler component and may change.
// This function is the suggested starting point — callers should treat
// it as a default that can be replaced wholesale by the scheduler's own
// implementation.
//
// Inputs:
//   - Tier (customer-facing > architectural > standard > trivial)
//   - Labels (sev1, customer-asked boost)
//   - Age (older bugs accumulate priority over time, capped at 7 days)
func Score(b *bug.Bug, now time.Time) int {
	score := 0

	switch b.Tier {
	case bug.TierCustomerFacing:
		score += 1000
	case bug.TierArchitectural:
		score += 500
	case bug.TierStandard:
		score += 100
	case bug.TierTrivial:
		score += 50
	}

	if b.HasLabel("sev1") {
		score += 800
	}
	if b.HasLabel("sev2") {
		score += 400
	}
	if b.HasLabel("customer-asked") {
		score += 600
	}

	// Age boost: 1 point per minute, capped at 7 days = 10080.
	age := now.Sub(b.DiscoveredAt)
	if age < 0 {
		age = 0
	}
	mins := int(age.Minutes())
	if mins > 10080 {
		mins = 10080
	}
	score += mins

	return score
}
