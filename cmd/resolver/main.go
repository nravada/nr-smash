// resolver — pulls bugs from the scheduler's priority queue and drives each
// one through the full smash flow: search SMASHed memory → analyse → clone
// → fix → test → render change-doc → open draft PR → write SMASHed memory.
//
// Owner: Rahul Malhan (@rmalhan-thatsit)
//
// See docs/components/resolver.md for the full flow and gating contract.
package main

import (
	"flag"
	"fmt"
	"time"
)

func main() {
	var (
		queueAddr = flag.String("queue-addr", "localhost:7777", "scheduler's priority-queue address")
		outputDir = flag.String("output-dir", "./.smash", "where to stage clones, change-docs, and per-smash artefacts")
		dryRun    = flag.Bool("dry-run", false, "do everything except open the draft PR")
		simBudget = flag.Duration("similarity-search-budget", 2*time.Second, "max time spent searching SMASHed memory before falling back to a cold smash")
	)
	flag.Parse()

	// TODO(rahul):
	//   1. dial the scheduler's queue at *queueAddr
	//   2. for each bug:
	//      a. memory.Store.Search(bug.Title + bug.Body, k=5) within *simBudget
	//      b. if a hit exceeds threshold → use that FixShape as the starting hypothesis
	//      c. clone the affected repo into *outputDir/<smash-id>/
	//      d. analyse + fix + test (re-uses ~/tau/scripts/smash-{branch,test,doc}.sh
	//         where helpful; the autonomous version inlines the gate-less path)
	//      e. open draft PR via gh CLI (skip if *dryRun)
	//      f. write a memory.Entry for this smash
	//   3. mark bug.Status accordingly (resolved / aborted / needs-human-review)
	// See ARCHITECTURE.md and docs/components/resolver.md.

	fmt.Printf("resolver: not implemented yet — queueAddr=%s outputDir=%s dryRun=%v simBudget=%s\n",
		*queueAddr, *outputDir, *dryRun, *simBudget)
	fmt.Println("see docs/components/resolver.md")
}
