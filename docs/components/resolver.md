# Component — Resolver

**Path:** `cmd/resolver`
**Owner:** Rahul Malhan (@rmalhan-thatsit)

## Job

Pull bugs from the scheduler's priority queue and drive each one through the full smash flow end-to-end: search SMASHed memory → analyse → clone affected repo → fix → test → render change-doc → open draft PR → write SMASHed memory entry.

The resolver is the part of the system that *actually does the work*. Everything upstream — collector, triage, scheduler — exists to feed the resolver high-quality, prioritised bug records. Everything downstream — the SMASHed memory and search skill — exists to make the resolver faster on the second occurrence of any pattern.

The resolver never auto-merges. It always stops at "draft PR ready for review."

## Inputs

- A scheduler queue address via `--queue-addr` (default `localhost:7777`)
- A staging directory via `--output-dir` (default `./.smash`) where each smash gets its own subdirectory: clones, change-docs, test artefacts
- A similarity-search budget via `--similarity-search-budget` (default `2s`) — max time spent in `memory.Store.Search` before falling back to a cold smash
- `--dry-run` — do everything except open the draft PR

## Outputs

For each bug processed, the resolver produces:
1. A new branch on the affected repo: `smash/<smash-id>`
2. A change-doc committed to that branch at `.smash/<smash-id>.md`
3. A draft PR opened against `main` of the affected repo
4. A `memory.Entry` written to the SMASHed memory store
5. A `bug.Status` update on the original `bug.Bug`: `resolved`, `aborted`, or `needs-human-review`

## End-to-end flow

```
dequeue Bug from scheduler
    │
    ├─ memory.Store.Search(Bug.Title + Bug.Body, k=5) within --similarity-search-budget
    │       │
    │       └─ if top hit > threshold → use FixShape as starting hypothesis
    │
    ├─ generate smash-id (jira:DI-1234 → DI-1234, slack:C0AB12CD/p... → freetext-<slug>)
    │
    ├─ clone Bug.Repo into output-dir/<smash-id>/repo/
    ├─ branch smash/<smash-id> from origin/main
    │
    ├─ analyse:
    │     ├─ memory cascade (already searched above)
    │     ├─ entity graph (BLOCKS / RELATED_TO past smashes in the same area)
    │     └─ focused code reading (not whole-repo)
    │
    ├─ produce a reproducer artefact (one of):
    │     ├─ failing test (preferred for well-tested codebases)
    │     ├─ NRQL query chain (for prod-observable bugs)
    │     └─ docker repro (for env-specific bugs)
    │
    ├─ implement the fix on the smash branch
    │     └─ if reproducer was a test, the test goes in the same commit as the fix
    │
    ├─ run tests (up to 3 fix iterations on failure; abort if still failing)
    │
    ├─ render change-doc with all sections (root cause, fix, repro, testing, risk)
    │
    ├─ AIDefence scan on the diff (existing NR tooling)
    │     └─ if risk > LOW → set Status=needs-human-review, halt
    │
    ├─ open draft PR via gh CLI (skip if --dry-run)
    │
    └─ write memory.Entry for this smash
```

## Tier handling

- **Trivial / Standard** — full flow, no human gate; PR opens as draft for review
- **Architectural** — write an OpenSpec proposal at `<output-dir>/<smash-id>/proposal.md`, set Status=`needs-human-review`, do NOT implement
- **Customer-facing** — full flow PLUS: invoke a customer-comms agent (Haiku, Rahul's voice) that produces 3 candidate response drafts (1-line, 1-paragraph, full-detail). Drafts are written to `<output-dir>/<smash-id>/customer-response-{1line,paragraph,full}.txt` and referenced from the change-doc — the human picks one at PR-review time

## Hackathon checklist

- [ ] Wire scheduler queue client (`internal/queue` or direct HTTP)
- [ ] Wire `memory.Store` (claude-flow MCP backend recommended — already populated by the existing `/smash` skill)
- [ ] Wire GitHub clone + branch (`internal/github`)
- [ ] Implement reproducer-builder for the three artefact types (start with test-only; NRQL and docker can be stubs)
- [ ] Wire Anthropic SDK for analyse + fix (`internal/llm`)
- [ ] Wire `gh pr create --draft` for PR open
- [ ] Wire AIDefence scan via existing NR tooling
- [ ] Implement Customer-facing customer-comms agent — reuse the prompt verbatim from `~/tau/.claude/commands/smash.md` "Customer-facing sub-team protocol"
- [ ] Write `memory.Entry` after every successful smash
- [ ] Test: end-to-end on one known bug from `nri-mssql` (open PR with `--dry-run` flag set, verify all artefacts land)

## Pattern label discipline

When writing the `memory.Entry`, set `Pattern` from this enum:

```
missing-context-timeout
swallowed-error
unbounded-retry
stale-cache
nil-deref-on-empty
goroutine-leak
sql-no-timeout
yaml-parse-error
field-required-missing
auth-failed
```

If no existing label fits, coin a new one in `kebab-case` and document it here in the same PR. The label space is what makes search effective on the second occurrence; coining sloppy labels degrades the speedup.

## Out of scope (hackathon)

- Auto-merge or auto-deploy
- Multi-repo smashes (one smash per bug, per repo)
- Continuous re-base of the smash branch on long-lived bugs (one shot per smash)
- Real-time gate UIs (the dashboard at localhost:9347 is sufficient)

## Related

- Shared types: `internal/bug`, `internal/memory`, `internal/priority`
- Existing /smash skill: `~/tau/.claude/commands/smash.md` — the resolver is a non-interactive, autonomous evolution of this
- Architecture: `ARCHITECTURE.md`
