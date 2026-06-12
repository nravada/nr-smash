#!/bin/bash
# Test collector with both Slack and Jira sample data

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║      Testing Bug Collector - Combined Slack + Jira          ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check if Anthropic API key is set (needed for Slack classification)
if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "⚠️  Warning: ANTHROPIC_API_KEY not set"
    echo "   Slack messages will not be classified."
    echo "   Set it with: export ANTHROPIC_API_KEY=sk-ant-..."
    echo ""
    echo "Testing Jira only..."
    echo ""

    ./collector --jira-file test-fixtures/sample-jira-issues.json 2>&1 | tee collector-output.jsonl
else
    echo "✓ ANTHROPIC_API_KEY found"
    echo ""
    echo "Testing both Slack and Jira..."
    echo ""

    ./collector \
        --slack-file test-fixtures/sample-slack-messages.json \
        --channel-id C0AB12CD3EF \
        --jira-file test-fixtures/sample-jira-issues.json \
        2>&1 | tee collector-output.jsonl
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "Results"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# Count bugs
TOTAL=$(grep -c '^{' collector-output.jsonl 2>/dev/null || echo "0")
SLACK=$(grep -c '"source":"slack"' collector-output.jsonl 2>/dev/null || echo "0")
JIRA=$(grep -c '"source":"jira"' collector-output.jsonl 2>/dev/null || echo "0")

echo "Total bugs collected: $TOTAL"
echo "  - From Slack: $SLACK"
echo "  - From Jira:  $JIRA"
echo ""
echo "Output saved to: collector-output.jsonl"
echo ""
echo "View with: ./view-bugs.sh"
echo "Or: cat collector-output.jsonl | jq ."
