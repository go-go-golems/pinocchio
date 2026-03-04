#!/usr/bin/env bash
set -euo pipefail

# Smoke-test the main `pinocchio ... --chat` TUI profile switching in tmux.
#
# Requirements:
# - /tmp/profile-registry.yaml exists and contains working provider credentials.
# - tmux and sqlite3 are installed.
#
# This script:
# 1) runs `pinocchio code professional ... --chat` in a tmux session
# 2) opens the profile picker with `/profile` (then aborts it)
# 3) switches profile with `/profile <slug>`
# 4) submits a prompt under the new profile
# 5) quits
#
# Notes:
# - bobatea chat binds "submit" to TAB, so we use C-i for submission.
# - pinocchio CLI chat conv_id is generated; we detect it via turns DB.

PINOCCHIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

PROFILE_REGISTRIES="${PROFILE_REGISTRIES:-/tmp/profile-registry.yaml}"
PROFILE_A="${PROFILE_A:-mento-haiku-4.5}"
PROFILE_B="${PROFILE_B:-mento-sonnet-4.6}"

TIMELINE_DB="${TIMELINE_DB:-/tmp/pin-chat-smoke.timeline.db}"
TURNS_DB="${TURNS_DB:-/tmp/pin-chat-smoke.turns.db}"

SESSION="${SESSION:-pin-chat-prof-smoke}"
PANE="${SESSION}:0.0"

WAIT_TIMEOUT_S="${WAIT_TIMEOUT_S:-240}"
BIN="${BIN:-/tmp/pinocchio-chat-smoke}"
LOG_LEVEL="${LOG_LEVEL:-error}"

cleanup() {
	tmux kill-session -t "${SESSION}" 2>/dev/null || true
}
trap cleanup EXIT

rm -f "${TIMELINE_DB}" "${TURNS_DB}"

dump_debug() {
	echo "--- capture-pane (tail) ---"
	tmux capture-pane -t "${PANE}" -p | tail -n 240 || true
	if [[ -f "${TURNS_DB}" ]]; then
		echo "--- turns.db counts ---"
		sqlite3 "${TURNS_DB}" "SELECT conv_id, COUNT(*) AS n FROM turns GROUP BY conv_id ORDER BY n DESC" 2>/dev/null || true
	fi
	if [[ -f "${TIMELINE_DB}" ]]; then
		echo "--- timeline.db kinds ---"
		sqlite3 "${TIMELINE_DB}" "SELECT conv_id, kind, COUNT(*) AS n FROM timeline_entities GROUP BY conv_id, kind ORDER BY n DESC" 2>/dev/null || true
	fi
}

echo "--- build ---"
(
	cd "${PINOCCHIO_DIR}"
	go build -o "${BIN}" ./cmd/pinocchio
)

CMD="cd '${PINOCCHIO_DIR}' && exec '${BIN}' \
  --log-level '${LOG_LEVEL}' \
  --profile-registries '${PROFILE_REGISTRIES}' \
  code professional \
  --profile '${PROFILE_A}' \
  --timeline-db '${TIMELINE_DB}' \
  --turns-db '${TURNS_DB}' \
  test \
  --chat"

tmux kill-session -t "${SESSION}" 2>/dev/null || true
tmux new-session -d -s "${SESSION}" "sh -lc ${CMD@Q}"

sleep 2

wait_for_any_turn_count() {
	local expected="$1"
	local start_ts
	start_ts="$(date +%s)"
	while true; do
		if [[ -f "${TURNS_DB}" ]]; then
			local count
			count="$(sqlite3 "${TURNS_DB}" "SELECT COUNT(*) FROM turns" 2>/dev/null || echo 0)"
			count="$(echo "${count}" | tr -d '[:space:]' || true)"
			if [[ "${count}" =~ ^[0-9]+$ ]] && [[ "${count}" -ge "${expected}" ]]; then
				return 0
			fi
		fi
		if (( $(date +%s) - start_ts > WAIT_TIMEOUT_S )); then
			return 1
		fi
		sleep 1
	done
}

latest_conv_id() {
	sqlite3 "${TURNS_DB}" "SELECT conv_id FROM turns ORDER BY turn_created_at_ms DESC LIMIT 1" 2>/dev/null | tr -d '[:space:]'
}

wait_for_assistant_final_messages() {
	local conv_id="$1"
	local expected="$2"
	local start_ts
	start_ts="$(date +%s)"
	while true; do
		if [[ -f "${TIMELINE_DB}" ]]; then
			local count
			count="$(sqlite3 "${TIMELINE_DB}" "SELECT COUNT(*) FROM timeline_entities WHERE conv_id='${conv_id}' AND kind='message' AND json_extract(entity_json,'$.props.role')='assistant' AND json_extract(entity_json,'$.props.streaming')=0" 2>/dev/null || echo 0)"
			count="$(echo "${count}" | tr -d '[:space:]' || true)"
			if [[ "${count}" =~ ^[0-9]+$ ]] && [[ "${count}" -ge "${expected}" ]]; then
				return 0
			fi
		fi
		if (( $(date +%s) - start_ts > WAIT_TIMEOUT_S )); then
			return 1
		fi
		sleep 1
	done
}

wait_for_profile_switch_entity() {
	local conv_id="$1"
	local expected="$2"
	local start_ts
	start_ts="$(date +%s)"
	while true; do
		if [[ -f "${TIMELINE_DB}" ]]; then
			local count
			count="$(sqlite3 "${TIMELINE_DB}" "SELECT COUNT(*) FROM timeline_entities WHERE conv_id='${conv_id}' AND kind='profile_switch'" 2>/dev/null || echo 0)"
			count="$(echo "${count}" | tr -d '[:space:]' || true)"
			if [[ "${count}" =~ ^[0-9]+$ ]] && [[ "${count}" -ge "${expected}" ]]; then
				return 0
			fi
		fi
		if (( $(date +%s) - start_ts > WAIT_TIMEOUT_S )); then
			return 1
		fi
		sleep 1
	done
}

wait_for_turn_count() {
	local conv_id="$1"
	local expected="$2"
	local start_ts
	start_ts="$(date +%s)"
	while true; do
		if [[ -f "${TURNS_DB}" ]]; then
			local count
			count="$(sqlite3 "${TURNS_DB}" "SELECT COUNT(*) FROM turns WHERE conv_id='${conv_id}'" 2>/dev/null || echo 0)"
			count="$(echo "${count}" | tr -d '[:space:]' || true)"
			if [[ "${count}" =~ ^[0-9]+$ ]] && [[ "${count}" -ge "${expected}" ]]; then
				return 0
			fi
		fi
		if (( $(date +%s) - start_ts > WAIT_TIMEOUT_S )); then
			return 1
		fi
		sleep 1
	done
}

wait_for_pane_contains() {
	local needle="$1"
	local start_ts
	start_ts="$(date +%s)"
	while true; do
		if tmux capture-pane -t "${PANE}" -p 2>/dev/null | rg -q --fixed-strings "${needle}"; then
			return 0
		fi
		if (( $(date +%s) - start_ts > WAIT_TIMEOUT_S )); then
			return 1
		fi
		sleep 0.5
	done
}

echo "--- wait for first persisted turn (auto-start) ---"
if ! wait_for_any_turn_count 1; then
	echo "FAIL: timeout waiting for first turn to persist (turns_db=${TURNS_DB})" >&2
	dump_debug
	exit 1
fi

CONV_ID="$(latest_conv_id)"
if [[ -z "${CONV_ID}" ]]; then
	echo "FAIL: could not detect conv_id from turns DB" >&2
	dump_debug
	exit 1
fi
echo "detected_conv_id=${CONV_ID}"

echo "--- wait for initial assistant final message ---"
if ! wait_for_assistant_final_messages "${CONV_ID}" 1; then
	echo "FAIL: timeout waiting for initial assistant final message to persist (conv_id=${CONV_ID}, timeline_db=${TIMELINE_DB})" >&2
	dump_debug
	exit 1
fi

echo "--- open picker via /profile (then abort with Esc) ---"
tmux send-keys -t "${PANE}" Enter
sleep 0.1
tmux send-keys -t "${PANE}" "/profile" C-i
if ! wait_for_pane_contains "/ filter"; then
	echo "FAIL: did not observe profile picker UI after /profile" >&2
	dump_debug
	exit 1
fi
tmux send-keys -t "${PANE}" Escape
if ! wait_for_pane_contains "ctrl+p profile"; then
	echo "FAIL: profile picker did not close after Esc" >&2
	dump_debug
	exit 1
fi

echo "--- switch profile (to=${PROFILE_B}) ---"
tmux send-keys -t "${PANE}" Enter
sleep 0.1
tmux send-keys -t "${PANE}" "/profile ${PROFILE_B}" C-i
if ! wait_for_pane_contains "profile=${PROFILE_B}"; then
	echo "FAIL: did not observe header update after /profile ${PROFILE_B}" >&2
	dump_debug
	exit 1
fi
if ! wait_for_profile_switch_entity "${CONV_ID}" 1; then
	echo "FAIL: timeout waiting for profile_switch entity to persist (conv_id=${CONV_ID}, timeline_db=${TIMELINE_DB})" >&2
	dump_debug
	exit 1
fi

echo "--- prompt under new profile ---"
tmux send-keys -t "${PANE}" Enter
sleep 0.1
tmux send-keys -t "${PANE}" "Say just one word: OK." C-i
if ! wait_for_turn_count "${CONV_ID}" 2; then
	echo "FAIL: timeout waiting for second turn to persist (conv_id=${CONV_ID}, turns_db=${TURNS_DB})" >&2
	dump_debug
	exit 1
fi

echo "--- quit ---"
tmux send-keys -t "${PANE}" M-q
sleep 1

echo "--- artifacts ---"
echo "conv_id=${CONV_ID}"
echo "timeline_db=${TIMELINE_DB}"
echo "turns_db=${TURNS_DB}"
