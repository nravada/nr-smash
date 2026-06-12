package priority

import (
	"testing"
	"time"

	"github.com/nravada/nr-smash/internal/bug"
)

func TestScore_TierOrder(t *testing.T) {
	now := time.Now()
	tiers := []bug.Tier{
		bug.TierCustomerFacing,
		bug.TierArchitectural,
		bug.TierStandard,
		bug.TierTrivial,
	}
	var prev int
	for i, tier := range tiers {
		b := &bug.Bug{Tier: tier, DiscoveredAt: now}
		s := Score(b, now)
		if i > 0 && s >= prev {
			t.Fatalf("tier %s scored %d, expected < previous %d", tier, s, prev)
		}
		prev = s
	}
}

func TestScore_LabelsBoost(t *testing.T) {
	now := time.Now()
	base := &bug.Bug{Tier: bug.TierStandard, DiscoveredAt: now}
	withSev1 := &bug.Bug{Tier: bug.TierStandard, DiscoveredAt: now, Labels: []string{"sev1"}}
	withCustomer := &bug.Bug{Tier: bug.TierStandard, DiscoveredAt: now, Labels: []string{"customer-asked"}}

	if Score(withSev1, now) <= Score(base, now) {
		t.Fatalf("sev1 should boost score")
	}
	if Score(withCustomer, now) <= Score(base, now) {
		t.Fatalf("customer-asked should boost score")
	}
}

func TestScore_AgeAccumulates(t *testing.T) {
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Minute)
	now := time.Now()

	bOlder := &bug.Bug{Tier: bug.TierStandard, DiscoveredAt: older}
	bNewer := &bug.Bug{Tier: bug.TierStandard, DiscoveredAt: newer}

	if Score(bOlder, now) <= Score(bNewer, now) {
		t.Fatalf("older bug should score higher than newer bug at same tier")
	}
}
