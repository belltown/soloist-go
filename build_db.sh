#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/.env"

: "${SQL_PATH:?SQL_PATH is not set in .env}"
: "${DB_PATH:?DB_PATH is not set in .env}"

if [[ ! -f "$SQL_PATH" ]]; then
    echo "SQL file not found: $SQL_PATH" >&2
    exit 1
fi

mkdir -p "$(dirname "$DB_PATH")"

sqlite3 "$DB_PATH" < "$SQL_PATH"

echo "Created database at $DB_PATH from $SQL_PATH"
