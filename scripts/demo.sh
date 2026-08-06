#!/usr/bin/env bash
set -euo pipefail
BASE_URL="${BASE_URL:-http://localhost:3000}"

echo '1) List seeded tasks'
curl -sS -i "$BASE_URL/tasks"

echo -e '\n2) Create a persistent task'
created="$(curl -fsS -X POST "$BASE_URL/tasks" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Buy milk"}')"
echo "$created"
id="$(printf '%s' "$created" | sed -n 's/.*"id":\([0-9][0-9]*\).*/\1/p')"

echo -e '\n3) Update it'
curl -sS -i -X PUT "$BASE_URL/tasks/$id" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Buy oat milk","done":true}'

echo -e '\n4) Query it through SQLite-backed search'
curl -sS -i "$BASE_URL/tasks?done=true&search=oat"

echo -e '\n5) Show SQL-computed statistics'
curl -sS -i "$BASE_URL/stats"

echo -e '\n6) Delete it'
curl -sS -i -X DELETE "$BASE_URL/tasks/$id"
