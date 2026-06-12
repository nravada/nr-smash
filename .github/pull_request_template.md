<!--
Title format: <component>: <short imperative description>
Examples:
  resolver: wire search skill before clone step
  scheduler: dedupe Slack vs Jira sources by URL canonicalisation
  collector [contract]: extend Bug.Source with repo-pr-comment

If your PR touches internal/bug, internal/memory, or internal/priority,
include [contract] in the title and tag every component owner.
-->

## What this PR does

<!-- One paragraph. The reviewer should not have to read the diff to know. -->

## Why

<!-- Link to the issue or describe the trigger. Avoid restating the title. -->

## Component(s) affected

- [ ] triage
- [ ] collector
- [ ] scheduler
- [ ] resolver
- [ ] search
- [ ] contract (`internal/bug`, `internal/memory`, `internal/priority`)
- [ ] docs / CI / build only

## Testing

<!-- How did you verify this works? Unit tests, integration runs, manual repro, etc. -->

## Checklist

- [ ] `go test ./...` passes
- [ ] `go vet ./...` clean
- [ ] `gofmt -s -l .` is empty
- [ ] If this is a `[contract]` change, all component owners are tagged
- [ ] Docs page (`docs/components/<name>.md`) updated if behaviour visible to other components changed
