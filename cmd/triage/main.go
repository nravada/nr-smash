// triage — identifies bugs from team-owned repos.
//
// Inputs:  configured list of GitHub repos to watch
// Outputs: bug.Bug records with Source=repo-issue|repo-pr-comment|repo-lint
//
//	and (where possible) Repo and Tier set
//
// See docs/components/triage-agent.md for the full contract.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		reposFlag = flag.String("repos", "", "comma-separated list of repos, e.g. newrelic/nri-mssql,newrelic/nri-mysql")
	)
	flag.Parse()

	if *reposFlag == "" {
		fmt.Fprintln(os.Stderr, "triage: --repos is required")
		os.Exit(2)
	}

	// TODO(triage-owner): scan each repo's issues, PRs, lint findings;
	// emit bug.Bug records on stdout (or post to the scheduler's queue).
	// See ARCHITECTURE.md and docs/components/triage-agent.md.
	fmt.Println("triage: not implemented yet — see docs/components/triage-agent.md")
}
