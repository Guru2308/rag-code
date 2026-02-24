#!/bin/bash
# Run example queries against a local RAG server.
# Prerequisites: ./rag-server running, codebase indexed via POST /api/index

BASE_URL="${RAG_URL:-http://localhost:8080}"
QUERIES_FILE="$(dirname "$0")/queries.json"

if ! command -v jq &>/dev/null; then
  echo "jq is required. Install with: brew install jq"
  exit 1
fi

echo "Querying RAG server at $BASE_URL"
echo ""

jq -r '.queries[] | "\(.query)"' "$QUERIES_FILE" | while read -r query; do
  echo "--- Query: $query ---"
  curl -s -X POST "$BASE_URL/api/query" \
    -H "Content-Type: application/json" \
    -d "{\"query\": $(echo "$query" | jq -Rs .), \"max_results\": 5}" | jq -r '.response // .error // .'
  echo ""
done
