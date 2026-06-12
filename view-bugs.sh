#!/bin/bash
# Helper script to view collected bugs in a readable format

if [ ! -f "collector-output.jsonl" ]; then
    echo "No collector output found. Run ./test-collector.sh first."
    exit 1
fi

# Count total bugs
total=$(grep -c '^{' collector-output.jsonl 2>/dev/null || echo "0")
echo "==================================="
echo "Total bugs collected: $total"
echo "==================================="
echo ""

# Parse and display each bug
jq -r '. | "ID: \(.id)\nSource: \(.source)\nTitle: \(.title)\nReporter: \(.reporter)\nURL: \(.url)\nLabels: \(.labels // [])\n---"' collector-output.jsonl 2>/dev/null

if [ $? -ne 0 ]; then
    echo "Note: Install 'jq' for pretty output (brew install jq)"
    echo ""
    echo "Raw output:"
    cat collector-output.jsonl
fi
