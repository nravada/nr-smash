# Component — Bug Collector

**Path:** `cmd/collector`
**Owner:** _(set in `.github/CODEOWNERS`)_

## Job

Poll the team's Slack help channel(s) and Jira board(s); detect bug-shaped messages; emit normalised `bug.Bug` records.

## Inputs

- Slack channel IDs via `--slack-channels` (comma-separated)
- Jira project keys via `--jira-projects` (comma-separated)
- Auth: Slack bot token in `SLACK_BOT_TOKEN` env var; Jira via the existing Atlassian MCP

## Outputs

`bug.Bug` records with:
- `Source` set to `slack` or `jira`
- `ID` formatted per ARCHITECTURE.md (`slack:CHANNEL/p<ts>` or `jira:PROJECT-NUMBER`)
- `Title`, `Body`, `Reporter`, `URL`, `Labels` populated
- `DiscoveredAt` set to the collector's `time.Now()`
- `Tier` and `Repo` left empty (the triage agent fills them)

## Slack bug-vs-noise classification

Most messages in `#help-db-o11y` are not bugs — they're questions, reactions, status updates. The collector calls Haiku once per message with a prompt like:

> Decide if this Slack message reports a software defect that requires a code change. Reply with one word: `bug` or `noise`.

Cache decisions by Slack message timestamp so re-runs are cheap.

## Jira filter

For the hackathon, default JQL: `project = DI AND status in ("To Do", "Open", "In Progress") AND labels = bug`.
Pull tickets with the existing Atlassian MCP, requesting only the fields the `bug.Bug` schema needs.

## Hackathon checklist

- [ ] Wire Slack client (`internal/slack`) — channel history, thread replies
- [ ] Wire Jira client (`internal/jira`) — JQL search, ticket fetch
- [ ] Implement Haiku-based bug-vs-noise for Slack messages
- [ ] Implement deduplication by `Bug.ID` so re-polls don't spam the queue
- [ ] Output: JSONL of `bug.Bug` on stdout
- [ ] Test: fixture inputs for 3 Slack messages + 3 Jira tickets → expected output

## Required env

```
SLACK_BOT_TOKEN=xoxb-...
JIRA_BASE_URL=https://new-relic.atlassian.net
ANTHROPIC_API_KEY=sk-ant-...
```

A `.env.example` must accompany any new env var.

## Out of scope (hackathon)

- Real-time Slack Events API (polling is fine)
- Cross-channel deduplication
- Reading Slack threads recursively (top-level message only)

## Related

- Shared types: `internal/bug`
- Architecture: `ARCHITECTURE.md`
