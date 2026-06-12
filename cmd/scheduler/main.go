// scheduler — runs the collector and triage on an interval, merges + dedupes
// the resulting bug stream, and exposes a priority-ordered queue for the
// resolver to consume.
//
// Inputs:  collector + triage outputs (in-process or via stdout)
// Outputs: priority-ordered queue of bug.Bug records (HTTP, gRPC, or
//
//	shared-memory — implementation choice).
//
// See docs/components/scheduler.md for the full contract.
package main

import (
	"flag"
	"fmt"
	"time"
)

func main() {
	var (
		interval = flag.Duration("interval", 5*time.Minute, "how often to run collector + triage")
		listen   = flag.String("listen", ":7777", "address to expose the priority queue API on")
	)
	flag.Parse()

	// TODO(scheduler-owner):
	//   1. start the collector and triage as goroutines or sub-processes
	//   2. on each tick, drain their outputs into a deduped pool keyed by Bug.ID
	//   3. recompute priority via internal/priority.Score on every dequeue
	//   4. expose Pop() to the resolver via HTTP/gRPC at *listen
	// See ARCHITECTURE.md.
	fmt.Printf("scheduler: not implemented yet — interval=%s, listen=%s\n", *interval, *listen)
	fmt.Println("see docs/components/scheduler.md")
}
