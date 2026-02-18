#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TICKET_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DB_PATH="${TICKET_DIR}/sources/analysis.db"

EVENT_YAML="${1:-/home/manuel/Downloads/event-log-6ac68635-37bc-4f4d-9657-dfc96df3c5c6-20260218-222305.yaml}"
GPT_LOG="${2:-/tmp/gpt-5.log}"

python3 "${SCRIPT_DIR}/10_import_logs.py" \
  --db "${DB_PATH}" \
  --schema "${SCRIPT_DIR}/00_schema.sql" \
  --event-yaml "${EVENT_YAML}" \
  --gpt-log "${GPT_LOG}"

echo "Built sqlite db: ${DB_PATH}"
