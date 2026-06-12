# Component — Triage Agent

**Path:** `cmd/triage`
**Owner:** _(set in `.github/CODEOWNERS`)_

## Job

Identify bugs from team-owned repos that aren't already in Jira or Slack — open issues, comments on closed PRs that look like new defects, recurring lint/CI failures. Enrich any inbound `bug.Bug` (from collector or another caller) with `Repo` and `Tier` fields.

## Inputs

- A list of GitHub repos (passed via `--repos`)
- Optional: stream of `bug.Bug` records lacking `Repo`/`Tier` (for enrichment-only runs)

## Outputs

`bug.Bug` records with:
- `Source` set to one of `repo-issue`, `repo-pr-comment`, `repo-lint`
- `Repo` always set
- `Tier` set when classification is confident (Trivial / Standard / Architectural / Customer-facing)
- `Labels` carried forward from the source

## Tier classification heuristic

Use the same heuristic table from the existing `/smash` skill:

| Tier | Triggers |
|---|---|
| **Architectural** | Touches public API surface, requires schema migration, breaks backward compat, spans more than 3 files in different packages, body contains "redesign"/"migration"/"breaking" |
| **Customer-facing** | Issue label includes `customer-asked`, severity `sev1`, body mentions a specific customer name/account |
| **Trivial** | Single-file lint finding, typo, dep bump, deprecated API swap, LOC estimate < 20 |
| **Standard** | Everything else |

If classification is ambiguous, leave `Tier` empty and let the scheduler decide.

## Hackathon checklist

- [ ] Wire GitHub client (`internal/github`) — list issues, list PR comments, list lint findings
- [ ] Implement Source mapping (issue → `repo-issue`, comment → `repo-pr-comment`, lint → `repo-lint`)
- [ ] Implement Tier classification (call Haiku for the close calls)
- [ ] Output: write `bug.Bug` records as JSONL on stdout
- [ ] Test: golden file with 5 known repo issues + expected tier output

## Out of scope (hackathon)

- Watching repos via GitHub webhooks (poll-only is fine for the demo)
- Cross-repo issue linking
- Auto-closing duplicates

## Related

- Shared types: `internal/bug`
- Architecture: `ARCHITECTURE.md`
