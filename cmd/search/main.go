// search — CLI wrapper around internal/memory.Store.Search.
//
// The resolver uses the memory.Store interface directly (in-process); this
// CLI exists for human spot-checks and for cross-component scripts that
// don't link the Go module. See docs/components/search.md.
package main

import (
	"flag"
	"fmt"
)

func main() {
	var (
		query = flag.String("q", "", "free-text query (typically bug.Title + bug.Body)")
		k     = flag.Int("k", 5, "max results to return")
	)
	flag.Parse()

	if *query == "" {
		fmt.Println("search: --q is required")
		fmt.Println("see docs/components/search.md")
		return
	}

	// TODO(search-owner):
	//   1. open the configured memory.Store backend (claude-flow MCP, sqlite,
	//      bolt, or whatever the team chooses — see ARCHITECTURE.md)
	//   2. call Store.Search(*query, *k)
	//   3. print results in human-readable form (smash_id, score, pattern,
	//      pr_url, fix_shape) one per line
	// See docs/components/search.md.
	fmt.Printf("search: not implemented yet — query=%q k=%d\n", *query, *k)
	fmt.Println("see docs/components/search.md")
}
