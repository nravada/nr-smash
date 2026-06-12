# Bug Collector Quick Start Guide

## Prerequisites

You'll need these credentials:

1. **Slack Bot Token** (`xoxb-...`)
2. **Anthropic API Key** (`sk-ant-...`)
3. **Jira Email & API Token** (optional, if using Jira)

## Setup (5 minutes)

### 1. Create `.env` file

```bash
cp .env.example .env
```

Then edit `.env` with your actual credentials:

```bash
SLACK_BOT_TOKEN=xoxb-your-actual-token
JIRA_BASE_URL=https://new-relic.atlassian.net
JIRA_EMAIL=your.email@newrelic.com
JIRA_API_TOKEN=your-jira-token
ANTHROPIC_API_KEY=sk-ant-your-key
```

### 2. Get Your Slack Channel ID

**Option A:** From browser
- Open your Slack channel in a browser
- Look at the URL: `https://app.slack.com/client/T0XXXXX/C0AB12CD3EF`
- The last part (`C0AB12CD3EF`) is your channel ID

**Option B:** Right-click method
- Right-click on the channel name in Slack
- Click "Copy link"
- The link contains the channel ID

### 3. Get Credentials

#### Slack Bot Token:
1. Go to https://api.slack.com/apps
2. Create a new app or select existing
3. Go to **OAuth & Permissions**
4. Add these scopes:
   - `channels:history`
   - `channels:read`
5. Install app to workspace
6. Copy the **Bot User OAuth Token**

#### Anthropic API Key:
1. Go to https://console.anthropic.com/
2. Create an API key
3. Copy it

#### Jira API Token:
1. Go to https://id.atlassian.com/manage-profile/security/api-tokens
2. Create a new token
3. Copy it

## Running the Collector

### Easy way (recommended):

```bash
# Test with Slack only
./test-collector.sh --slack C0AB12CD3EF

# Test with Jira only
./test-collector.sh --jira DI

# Test with both
./test-collector.sh --both C0AB12CD3EF DI
```

### Manual way:

```bash
# Build
go build ./cmd/collector

# Run
export $(cat .env | grep -v '^#' | xargs)
./collector --slack-channels C0AB12CD3EF --jira-projects DI
```

## Viewing Results

```bash
# Pretty view (requires jq)
./view-bugs.sh

# Raw JSONL
cat collector-output.jsonl

# Count bugs
grep -c '^{' collector-output.jsonl
```

## Example Output

```
2026/06/12 13:00:00 collector: fetching Slack channel C0AB12CD3EF
2026/06/12 13:00:01 collector: classifying 45 messages from C0AB12CD3EF
2026/06/12 13:00:05 collector: detected bug slack:C0AB12CD3EF/p1700000000123456
2026/06/12 13:00:08 collector: fetching Jira project DI
2026/06/12 13:00:09 collector: found 12 bug issues in DI
2026/06/12 13:00:09 collector: detected bug jira:DI-1234
2026/06/12 13:00:09 collector: emitted 13 bugs
```

## Troubleshooting

### "Failed to fetch Slack channel"
- Check your `SLACK_BOT_TOKEN` is valid
- Make sure the bot is added to the channel
- Verify channel ID is correct

### "Failed to classify message"
- Check your `ANTHROPIC_API_KEY` is valid
- Check your Anthropic account has credits

### "Failed to fetch Jira project"
- Check `JIRA_EMAIL` and `JIRA_API_TOKEN` are correct
- Verify you have access to the project
- Check the project key is correct (e.g., "DI", not "di")

## What Happens Next?

The collector outputs JSONL (one JSON object per line) with `bug.Bug` records. These feed into the **triage agent**, which will:
1. Enrich bugs with repo information
2. Classify them by tier
3. Pass them to the scheduler

For the hackathon, you can pipe the output or save it:

```bash
# Save to file
./collector --slack-channels C123 > bugs.jsonl 2> collector.log

# Pipe to triage (once implemented)
./collector --slack-channels C123 | ./triage
```
