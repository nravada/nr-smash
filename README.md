# nr-smash

**SMASH — Semantic Multi-source Autonomous SRE Hub.**
An always-on agent that triages bugs from team-owned repos, the team's Jira board, and the team's Slack help channel, then drives each one to a draft PR end-to-end.

> Hackathon project. Built on top of the existing `/smash` Phase-1 Bug Smasher pattern, extended for autonomous operation.

## What this repo is

Five components, one shared contract, one shared memory store. Each component has a single job; they communicate through well-defined Go interfaces in `internal/`. Five teammates can land code in parallel without stepping on each other.

```
                        ┌──────────────────────────┐
                        │      Bug Collector       │ ← Slack #help-db-o11y
                        │  (cmd/collector)         │ ← Jira board (DI)
                        └────────────┬─────────────┘
                                     │ Bug records
                        ┌────────────▼─────────────┐
                        │      Triage Agent        │ ← repo scan (issues, PRs, lint)
                        │  (cmd/triage)            │
                        └────────────┬─────────────┘
                                     │ Tier-classified Bugs
                        ┌────────────▼─────────────┐
                        │  Scheduler / Orchestrator│
                        │  (cmd/scheduler)         │ ← runs collector+triage periodically
                        └────────────┬─────────────┘
                                     │ Priority queue
                        ┌────────────▼─────────────┐    ┌────────────────────┐
                        │       Resolver           │───▶│   Search Skill     │
                        │  (cmd/resolver)          │◀───│   (cmd/search)     │
                        └────────────┬─────────────┘    └────────────────────┘
                                     │                        ▲
                                     │ writes after smash     │ queries before smash
                                     ▼                        │
                          ┌────────────────────────────────────┴───┐
                          │     SMASHed memory store               │
                          │     (internal/memory)                  │
                          └────────────────────────────────────────┘
```

## The five components

| Component | Entrypoint | What it does | Owner |
|---|---|---|---|
| **Bug collector** | `cmd/collector` | Polls Slack hero channel + Jira board, normalizes into `bug.Bug` records | (TBD — fill in CODEOWNERS) |
| **Triage agent** | `cmd/triage` | Identifies bugs from team-owned repos (issue scan, PR labels, lint findings); enriches `bug.Bug` records with `repo` and `labels` | (TBD — fill in CODEOWNERS) |
| **Scheduler / Orchestrator** | `cmd/scheduler` | Runs collector + triage periodically; merges into a deduped, priority-ordered queue | (TBD — fill in CODEOWNERS) |
| **Resolver** | `cmd/resolver` | Pulls from queue → analyse → clone → fix → test → draft PR with change-doc → write to SMASHed memory | @rmalhan-thatsit (Rahul) |
| **Search skill** | `cmd/search` | Queries SMASHed memory before resolver runs; returns past similar fixes for fast-path application | (TBD — fill in CODEOWNERS) |

Each component has a docs page at `docs/components/<name>.md` covering inputs, outputs, dependencies, and a starter checklist.

## Shared contracts (the only files that need cross-team coordination)

- [`internal/bug/bug.go`](./internal/bug/bug.go) — the `Bug` record: every component reads or writes this.
- [`internal/memory/memory.go`](./internal/memory/memory.go) — the `Store` interface for SMASHed memory: resolver writes, search reads, both depend on the same shape.
- [`internal/priority/priority.go`](./internal/priority/priority.go) — the priority signal scheduler emits and resolver consumes.

If you need to change one of these, open a PR with `[contract]` in the title and ping the team — every component depends on them.

## Quick start

```bash
git clone https://github.com/nravada/nr-smash.git
cd nr-smash

# Pick your component and build its entrypoint:
go build ./cmd/resolver
./resolver --help    # each component documents its own flags
```

Common dev:
```bash
go test ./...        # all tests
go vet ./...         # static analysis
gofmt -l -s .        # formatting check
```

## Project board

Hackathon work items live as GitHub issues in this repo, labeled by component (`triage`, `collector`, `scheduler`, `resolver`, `search`, `contract`). The Resolver's design and progress are tracked in the `resolver` issue thread.

## Architecture detail

See [ARCHITECTURE.md](./ARCHITECTURE.md) for the full flow, the memory schema, the priority model, and what runs as a daemon vs. a one-shot.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md). Short version: branch off `main`, prefix PR title with the component name (`resolver:`, `triage:`, etc.), at least one approving review per PR, never `--no-verify`.

## License

Apache-2.0 — see [LICENSE](./LICENSE).
