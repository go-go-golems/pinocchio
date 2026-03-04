#!/usr/bin/env bash
set -euo pipefail

# Verify turn + timeline persistence for switch-profiles-tui smoke runs.
#
# Requirements:
# - sqlite3 and jq installed
#
# Inputs via env vars (defaults match tmux smoke script):
# - CONV_ID
# - TIMELINE_DB
# - TURNS_DB
#
# Notes:
# - We expect at least 2 turns and 2 distinct runtime keys when switching
#   between mento-haiku-4.5 and mento-sonnet-4.6.

CONV_ID="${CONV_ID:-spt-1-smoke}"
TIMELINE_DB="${TIMELINE_DB:-/tmp/spt-1-smoke.timeline.db}"
TURNS_DB="${TURNS_DB:-/tmp/spt-1-smoke.turns.db}"

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

# Assert assistant messages are persisted as streaming=false (final)
ASSISTANT_FINAL_COUNT="$(sqlite3 "${TIMELINE_DB}" \
	"SELECT COUNT(*) FROM timeline_entities WHERE conv_id='${CONV_ID}' AND kind='message' AND json_extract(entity_json,'$.props.role')='assistant' AND json_extract(entity_json,'$.props.streaming')=0")"
if [[ "${ASSISTANT_FINAL_COUNT}" -lt 2 ]]; then
	echo "FAIL: expected at least 2 assistant message entities with props.streaming=false persisted" >&2
	exit 1
fi

# Assert at least one assistant message has runtime attribution persisted
ASSISTANT_WITH_ATTRIB="$(sqlite3 -json "${TIMELINE_DB}" \
	"SELECT entity_json FROM timeline_entities WHERE conv_id='${CONV_ID}' AND kind='message' ORDER BY version ASC LIMIT 100" \
	| jq -r '[.[].entity_json | fromjson | .props | select(.runtime_key != null and .["profile.slug"] != null)] | length')"

if [[ "${ASSISTANT_WITH_ATTRIB}" -lt 1 ]]; then
	echo "FAIL: expected at least 1 message entity with props.runtime_key and props[\"profile.slug\"] persisted" >&2
	exit 1
fi

echo "OK: persistence checks passed"
