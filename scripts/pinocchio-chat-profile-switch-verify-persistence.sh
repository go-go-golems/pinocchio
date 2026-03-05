#!/usr/bin/env bash
set -euo pipefail

# Verify turn + timeline persistence for main `pinocchio ... --chat` profile switching smoke runs.
#
# Requirements:
# - sqlite3 and jq installed
#
# Inputs via env vars:
# - TIMELINE_DB (required)
# - TURNS_DB (required)
# - CONV_ID (optional; if empty, we auto-detect newest conv_id in turns DB)

TIMELINE_DB="${TIMELINE_DB:-}"
TURNS_DB="${TURNS_DB:-}"
CONV_ID="${CONV_ID:-}"

if [[ -z "${TIMELINE_DB}" || -z "${TURNS_DB}" ]]; then
	echo "ERROR: set TIMELINE_DB and TURNS_DB" >&2
	exit 2
fi
if [[ ! -f "${TIMELINE_DB}" || ! -f "${TURNS_DB}" ]]; then
	echo "ERROR: db file(s) missing: timeline_db=${TIMELINE_DB} turns_db=${TURNS_DB}" >&2
	exit 2
fi

if [[ -z "${CONV_ID}" ]]; then
	CONV_ID="$(sqlite3 "${TURNS_DB}" "SELECT conv_id FROM turns ORDER BY turn_created_at_ms DESC LIMIT 1" 2>/dev/null | tr -d '[:space:]')"
fi
if [[ -z "${CONV_ID}" ]]; then
	echo "ERROR: could not determine CONV_ID" >&2
	exit 2
fi

echo "conv_id=${CONV_ID}"

echo "--- turns (runtime_key) ---"
sqlite3 -json "${TURNS_DB}" \
	"SELECT conv_id, session_id, turn_id, turn_created_at_ms, runtime_key, inference_id FROM turns WHERE conv_id='${CONV_ID}' ORDER BY turn_created_at_ms ASC" \
	| jq .

TURN_COUNT="$(sqlite3 "${TURNS_DB}" "SELECT COUNT(*) FROM turns WHERE conv_id='${CONV_ID}'")"
if [[ "${TURN_COUNT}" -lt 2 ]]; then
	echo "FAIL: expected at least 2 turns for conv_id=${CONV_ID}, got ${TURN_COUNT}" >&2
	exit 1
fi

INFERENCE_WITH_ID="$(sqlite3 -json "${TURNS_DB}" \
	"SELECT inference_id FROM turns WHERE conv_id='${CONV_ID}' ORDER BY turn_created_at_ms ASC" \
	| jq -r '[.[].inference_id | select(. != null and . != "")] | length')"
if [[ "${INFERENCE_WITH_ID}" -lt 1 ]]; then
	echo "FAIL: expected at least 1 turn with non-empty inference_id" >&2
	exit 1
fi

DISTINCT_TURN_KEYS="$(sqlite3 -json "${TURNS_DB}" \
	"SELECT DISTINCT runtime_key FROM turns WHERE conv_id='${CONV_ID}' AND runtime_key != '' ORDER BY runtime_key ASC" \
	| jq -r 'map(.runtime_key)')"
DISTINCT_TURN_KEY_COUNT="$(echo "${DISTINCT_TURN_KEYS}" | jq -r 'length')"
if [[ "${DISTINCT_TURN_KEY_COUNT}" -lt 2 ]]; then
	echo "FAIL: expected at least 2 distinct runtime_key values after profile switch, got ${DISTINCT_TURN_KEY_COUNT}" >&2
	echo "distinct_runtime_keys=${DISTINCT_TURN_KEYS}" >&2
	exit 1
fi

echo "distinct_turn_runtime_keys=${DISTINCT_TURN_KEYS}"

echo "--- timeline entities (props excerpt) ---"
sqlite3 -json "${TIMELINE_DB}" \
	"SELECT conv_id, entity_id, kind, version, entity_json FROM timeline_entities WHERE conv_id='${CONV_ID}' ORDER BY version ASC" \
	| jq 'map({conv_id, entity_id, kind, version, props: (.entity_json | fromjson | .props)})'

PROFILE_SWITCH_COUNT="$(sqlite3 "${TIMELINE_DB}" "SELECT COUNT(*) FROM timeline_entities WHERE conv_id='${CONV_ID}' AND kind='profile_switch'")"
if [[ "${PROFILE_SWITCH_COUNT}" -lt 1 ]]; then
	echo "FAIL: expected at least 1 profile_switch entity in timeline DB" >&2
	exit 1
fi

echo "OK: persistence checks passed"

