# Contributing to nr-smash

This is a hackathon project with five teammates landing code in parallel. Keeping that flowing requires light-touch discipline. This page is the discipline.

## Branching and PRs

- Branch off `main`. Branch naming: `<component>/<short-slug>` — e.g. `resolver/wire-search-skill`, `collector/jira-poll`.
- One PR per logical change. Don't bundle two components in a single PR.
- **Prefix PR titles with the component name**: `resolver: wire search skill before clone step` (matches the per-component CODEOWNERS routing).
- At least one approving review before merge. Self-merge of trivial typos is fine if you say so explicitly.
- Never push with `--no-verify`. If a hook fails, fix the underlying issue.

## Cross-component contracts

The contracts in `internal/bug`, `internal/memory`, and `internal/priority` are shared. Changes there affect every other component.

If your PR touches any of those three packages:
- Add `[contract]` to the PR title (e.g. `resolver [contract]: extend Bug.Repo to support multi-repo`).
- Tag every component owner.
- Plan for a follow-up sweep PR per affected component.

## Code style

- `gofmt -s` on every file (the CI workflow checks).
- `go vet` clean.
- Tests for non-trivial logic. We aren't enforcing coverage; we are enforcing "no test = won't merge for non-trivial PRs."
- Comments only where the *why* is non-obvious. Don't narrate what the code does — readable code already does that.

## Issues and labels

- Open an issue before starting non-trivial work so the team sees it.
- Apply the right component label: `triage`, `collector`, `scheduler`, `resolver`, `search`, `contract`.
- For hackathon-tracking, also apply: `phase-0` (planning), `phase-1` (build), `phase-2` (demo prep).

## Local dev quickstart

```bash
git clone https://github.com/nravada/nr-smash.git
cd nr-smash
go mod download
go test ./...
go build ./cmd/...
```

Per-component setup (DB drivers, Slack tokens, Jira auth, etc.) is documented in each component's `docs/components/<name>.md` page.

## Secrets

- Never commit secrets. Use environment variables loaded from `.env` (which is git-ignored).
- Provide a `.env.example` file in each component's docs page if you add new env vars.
- The repo is private but treat secrets as if it were public.

## Release / demo discipline (hackathon-specific)

- The demo branch is `main`. Anything we'll demo lives there.
- One-day-before-demo cutoff: no contract changes, only bugfixes.
- Demo-day rollback plan: tag a `pre-demo` tag the night before so we can `git reset --hard pre-demo` if something blows up during the live close.

## Communication

- Day-to-day: Slack thread on `#help-db-o11y` or your hackathon channel of choice.
- Async architecture decisions: PR comments on `[contract]` PRs.
- Stuck? Open an issue with the `help-wanted` label.
