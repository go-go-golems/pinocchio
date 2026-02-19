#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TICKET_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DB_PATH="${TICKET_DIR}/sources/analysis.db"
OUT_DIR="${TICKET_DIR}/sources/query-results"

mkdir -p "${OUT_DIR}"

if [[ ! -f "${DB_PATH}" ]]; then
  echo "Missing db: ${DB_PATH}" >&2
  echo "Run scripts/11_build_db.sh first." >&2
  exit 1
fi

for q in \
  q01_event_type_counts.sql \
  q02_thinking_event_timeline.sql \
  q03_post_final_thinking_delta.sql \
  q04_timeline_thinking_streaming_state.sql \
  q05_gpt_thinking_markers.sql \
  q06_events_after_last_thinking_final.sql
do
  sqlite3 "${DB_PATH}" < "${SCRIPT_DIR}/${q}" > "${OUT_DIR}/${q%.sql}.txt"
  echo "Wrote ${OUT_DIR}/${q%.sql}.txt"
done
