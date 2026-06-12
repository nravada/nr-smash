#!/bin/bash
# Fetch your team's Jira bugs - Working version

set -e

JIRA_BASE_URL="${JIRA_BASE_URL:-https://new-relic.atlassian.net}"
OUTPUT_FILE="${1:-team-jira-bugs.json}"

if [ -z "$JIRA_EMAIL" ] || [ -z "$JIRA_API_TOKEN" ]; then
    echo "Error: JIRA_EMAIL and JIRA_API_TOKEN required"
    exit 1
fi

echo "Fetching your team's bugs from Jira..."
echo ""

# Step 1: Search with your team filter to get issue IDs
JQL='project in ("New Relic Private") AND type != Risk AND "Team[Team]" in (ea229518-a006-4d09-b8c0-223a885aeff7-44, 6f6d0e4c-fac8-4023-87c7-5efaeed38632) ORDER BY Rank ASC'

SEARCH_RESPONSE=$(curl -s -X POST \
  -u "$JIRA_EMAIL:$JIRA_API_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  --data "$(jq -n --arg jql "$JQL" '{jql: $jql, maxResults: 50}')" \
  "$JIRA_BASE_URL/rest/api/3/search/jql")

# Extract issue IDs
ISSUE_IDS=$(echo "$SEARCH_RESPONSE" | jq -r '.issues[].id')
TOTAL=$(echo "$ISSUE_IDS" | wc -l | tr -d ' ')

echo "Found $TOTAL issues from your team"
echo "Fetching full details..."
echo ""

# Step 2: Fetch full details for each issue
echo "[" > "$OUTPUT_FILE"
FIRST=true

for ISSUE_ID in $ISSUE_IDS; do
    echo -n "."

    ISSUE_DATA=$(curl -s -X GET \
      -u "$JIRA_EMAIL:$JIRA_API_TOKEN" \
      -H "Accept: application/json" \
      "$JIRA_BASE_URL/rest/api/3/issue/$ISSUE_ID?fields=key,summary,description,reporter,labels,status,issuetype")

    # Extract plain text from description (ADF format) - just get first paragraph
    DESC_TEXT=$(echo "$ISSUE_DATA" | jq -r '
      if .fields.description then
        if .fields.description.type == "doc" then
          [.fields.description.content[]? |
           select(.type == "paragraph") |
           .content[]? |
           select(.type == "text") |
           .text] | join(" ") | .[0:500]
        else
          .fields.description[0:500]
        end
      else
        ""
      end
    ')

    # Build bug record
    if [ "$FIRST" = false ]; then
        echo "," >> "$OUTPUT_FILE"
    fi
    FIRST=false

    echo "$ISSUE_DATA" | jq --arg desc "$DESC_TEXT" '{
      key: .key,
      fields: {
        summary: .fields.summary,
        description: $desc,
        reporter: {
          accountId: .fields.reporter.accountId,
          displayName: .fields.reporter.displayName
        },
        labels: (.fields.labels // []),
        status: {
          name: .fields.status.name
        }
      }
    }' >> "$OUTPUT_FILE"
done

echo "" >> "$OUTPUT_FILE"
echo "]" >> "$OUTPUT_FILE"

echo ""
echo ""
echo "✓ Fetched $TOTAL issues"
echo "✓ Saved to: $OUTPUT_FILE"
echo ""

# Show summary
echo "Summary:"
jq -r 'group_by(.fields.status.name) | .[] | "\(.length) - \(.[0].fields.status.name)"' "$OUTPUT_FILE"

echo ""
echo "Test with:"
echo "  ./collector --jira-file $OUTPUT_FILE"
