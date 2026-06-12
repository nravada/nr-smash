# Component — Scheduler / Orchestrator

**Path:** `cmd/scheduler`
**Owner:** _(set in `.github/CODEOWNERS`)_

## Job

Run the collector and triage agent on an interval. Merge their outputs into a deduped, priority-ordered queue. Expose a `Pop()` API for the resolver.

## Inputs

- Configuration: collector args, triage args, interval (default 5m)
- Stream of `bug.Bug` records from the collector and triage agent (in-process channels or stdout pipes — orchestrator's choice)

## Outputs

- Priority queue exposed at `--listen` (default `:7777`)
- API surface: `POST /pop` returns the next bug; `GET /peek?n=10` returns the top-N

## Priority computation

Use `internal/priority.Score(bug, now)` as the default. Score is recomputed on every `Pop()` so that age-related boost reflects current time rather than time-at-enqueue.

The scheduler is the only writer of `bug.Priority`. The resolver only reads it.

## Deduplication

Bugs are deduped by `bug.ID`. If the collector emits the same Jira ticket twice (e.g. across two polls), the second emission wins (overwrites previous). If the same bug exists from two sources (e.g. a Slack message that links a Jira ticket), the scheduler may merge them — implementation choice. For the hackathon, "first one wins" is acceptable.

## Hackathon checklist

- [ ] Spawn collector + triage as goroutines or sub-processes
- [ ] Drain their `bug.Bug` outputs into a deduped pool
- [ ] HTTP API at `--listen`: `/pop`, `/peek`, `/health`
- [ ] Recompute priority via `priority.Score` on every `Pop`
- [ ] Test: with a known bug stream, assert priority ordering matches expectation

## Out of scope (hackathon)

- Persistent queue (in-memory is fine; restart loses state)
- Multi-tenant queues
- Prioritising sticky bugs (always-skip lists)

## Related

- Shared types: `internal/bug`, `internal/priority`
- Architecture: `ARCHITECTURE.md`
