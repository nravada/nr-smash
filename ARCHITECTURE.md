# nr-smash — Architecture

This document is the canonical description of how the five components interact, what each owns, and what's intentionally out of scope.

## Goals

1. **Autonomy.** The system runs continuously without human invocation. Humans approve at PR-review time only.
2. **Cross-team source coverage.** Bugs are picked up from Jira *and* the team's Slack help channel, not one or the other.
3. **Memory pays back.** The second occurrence of any pattern is materially faster than the first; the speedup is measurable.
4. **Safe by default.** Resolver always stops at "draft PR ready for review." No auto-merge. Every push is scanned by AIDefence (existing NR tooling).

## Components and contracts

```
┌────────────────────┐   bug.Bug    ┌────────────────────┐
│  Bug Collector     │─────────────▶│   Triage Agent     │
│  cmd/collector     │              │   cmd/triage       │
└────────────────────┘              └─────────┬──────────┘
        ▲                                     │
        │ poll (Slack/Jira)                   │ enriched bug.Bug
        │                                     ▼
        │                          ┌────────────────────┐
        │                          │ Scheduler          │
        │  trigger every N min     │ cmd/scheduler      │
        └──────────────────────────┤ priority.Priority  │
                                   └─────────┬──────────┘
                                             │ priority-ordered queue
                                             ▼
                          ┌──────────────────────────────────────┐
                          │              Resolver                 │
                          │              cmd/resolver             │
                          │                                       │
                          │  1. dequeue Bug                       │
                          │  2. search SMASHed memory ──────────┐ │
                          │  3. clone repo, branch              │ │
                          │  4. analyse + fix                   │ │
                          │  5. test                            │ │
                          │  6. draft PR via gh                 │ │
                          │  7. write SMASHed memory entry ─────┤ │
                          └─────────────────────────────────────┼─┘
                                                                │
                                                                ▼
                          ┌──────────────────────────────────────┐
                          │       SMASHed memory store           │
                          │       internal/memory                │
                          │                                      │
                          │   ┌─────────────────────────────┐    │
                          │   │  Search Skill               │◀───┘ resolver writes
                          │   │  cmd/search                 │
                          │   │                             │
                          │   │  used by resolver step 2    │
                          │   └─────────────────────────────┘
                          └──────────────────────────────────────┘
```

## Process model (hackathon scope)

| Component | Lifetime | Trigger |
|---|---|---|
| Bug collector | Long-running daemon (or cron) | Scheduler invokes `collector.Run()` every N minutes (default: 5m) |
| Triage agent | Long-running daemon (or cron) | Scheduler invokes `triage.Run()` every N minutes (default: 5m) |
| Scheduler | Single long-running daemon | Wakes itself on a fixed interval |
| Resolver | Long-running worker | Polls the priority queue; resolves one bug at a time |
| Search skill | Library + CLI | Resolver imports; CLI for human spot-checks |

For the hackathon, `cmd/scheduler` runs as the parent process and starts the others as goroutines or sub-processes — operator's choice. Production-grade orchestration is out of scope.

## Data model

### `bug.Bug` — the cross-component record

See [internal/bug/bug.go](./internal/bug/bug.go) for the canonical type. Field summary:

| Field | Set by | Purpose |
|---|---|---|
| `ID` | collector or triage | Globally unique, e.g. `jira:DI-1234`, `slack:C0AB12CD/p1700000000123456`, `repo:newrelic/nri-mssql/issues/42` |
| `Source` | collector or triage | Origin: `jira`, `slack`, `repo-issue`, `repo-pr-comment`, `repo-lint` |
| `DiscoveredAt` | collector or triage | RFC 3339 |
| `Title`, `Body` | collector or triage | Human text |
| `Reporter` | collector or triage | Slack user ID, Jira account ID, or GitHub login |
| `URL` | collector or triage | Link back to source |
| `Labels` | collector or triage | Free-form strings, e.g. `customer-asked`, `sev1`, `backend`, `mssql` |
| `Repo` | triage (enrichment) | Affected repo if known, e.g. `newrelic/nri-mssql` |
| `Tier` | triage | One of `trivial`, `standard`, `architectural`, `customer-facing` |
| `Priority` | scheduler | A `priority.Priority` value (see below) |
| `Status` | scheduler / resolver | `queued`, `resolving`, `resolved`, `aborted` |
| `SmashID` | resolver | Set when resolver claims the bug; identifier used in branch + memory entry |

### `priority.Priority` — the scheduling signal

See [internal/priority/priority.go](./internal/priority/priority.go). A single integer score, higher is more urgent. Computed by the scheduler from:

- Tier (`customer-facing > architectural > standard > trivial`)
- Labels (`sev1`, `customer-asked` boost)
- Age (older bugs accumulate priority over time, capped)
- Repo (some repos may be weighted by ops oncall load — optional)

The scheduler is the only component that writes `Priority`. Resolver only reads it.

### `memory.Entry` — SMASHed memory

See [internal/memory/memory.go](./internal/memory/memory.go). Written by resolver after a successful smash, queried by search before resolver starts work. Fields:

| Field | Purpose |
|---|---|
| `SmashID` | Stable ID matching the resolver branch + change-doc |
| `BugID` | The originating `Bug.ID` |
| `Repo` | The repo the fix landed in |
| `Pattern` | Categorical label (e.g. `missing-context-timeout`, `swallowed-error`) — drives search hits |
| `FixShape` | One-paragraph description of the fix |
| `LOCDelta` | `[added, removed]` |
| `Tier` | Tier the smash was classified under |
| `PRURL` | Draft PR URL |
| `PassedTests` | Whether the in-smash tests passed |
| `ShippedAt` | When the PR was opened |
| `Embedding` | `[]float32` — semantic vector for similarity search |

## Search skill — query model

See [internal/memory/memory.go](./internal/memory/memory.go) `Store.Search`. Resolver calls this with the new bug's title + body before doing real work. Returns top-K past entries with similarity scores. If a hit's score exceeds the configured threshold, the resolver uses the past `FixShape` as a starting hypothesis.

## Memory store — implementation choices (open)

The memory `Store` interface is fixed (see `internal/memory`). The implementation is not. Reasonable hackathon options:

1. **SQLite + a Go vector ext** — local file, easy demo, no service dep
2. **Bolt + in-memory HNSW** — Go-native, no SQL
3. **Existing `claude-flow` MCP** — `memory_store` / `memory_search` over HNSW; reuses the SONA learning loop the existing `/smash` already feeds

**Recommendation for hackathon:** option 3, since `/smash` already writes to it and the memory will already have warm patterns from past human-driven smashes — that's the speedup story.

## What's intentionally out of scope (hackathon)

- Auto-merge or auto-deploy
- Multi-tenant operation (one team per deploy)
- Dashboard polish beyond what's needed for gate approval
- HA / failover / multi-region
- Customer-comms voice training on real customer history (we use the existing `/smash` baseline prompt)

## Failure modes and recovery

| Failure | Recovery |
|---|---|
| LLM call fails | Retry once. If it fails twice, the resolver aborts that bug, marks `Status=aborted`, moves on. |
| Repo clone fails | Resolver aborts, marks `aborted`, logs to memory. |
| Tests fail after fix | Resolver attempts up to 3 fix iterations. After 3 failures, abort with "needs human." |
| AIDefence flags push | Branch stays local; Status=`needs-human-review`; gate dashboard is notified. |
| Memory write fails | Resolver continues — memory write is best-effort. Logged for replay. |

## Build + run

```bash
# Build all components
go build ./cmd/...

# Run the orchestrator (which starts everything)
./scheduler --config ./deploy/scheduler.yaml

# Or run a single component
./collector --slack-channel C0AB12CD --jira-project DI
./resolver --queue-addr localhost:7777 --output-dir ./.smash
```

Each component documents its own flags in `docs/components/<name>.md`.
