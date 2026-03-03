#!/usr/bin/env bash
set -euo pipefail

# Smoke-test the profile-switching TUI in tmux with real inference.
#
# Requirements:
# - /tmp/profile-registry.yaml exists and contains working provider credentials.
# - tmux and sqlite3 are installed.
#
# This script:
# 1) runs the TUI in a tmux session
# 2) submits a prompt under one profile
# 3) switches profile with /profile
# 4) submits a second prompt
# 5) quits
#
# It intentionally uses TAB as "submit" because bobatea chat keymap binds submit to "tab".

PINOCCHIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

PROFILE_REGISTRIES="${PROFILE_REGISTRIES:-/tmp/profile-registry.yaml}"
PROFILE_A="${PROFILE_A:-mento-haiku-4.5}"
PROFILE_B="${PROFILE_B:-mento-sonnet-4.6}"

CONV_ID="${CONV_ID:-spt-1-smoke}"
TIMELINE_DB="${TIMELINE_DB:-/tmp/spt-1-smoke.timeline.db}"
TURNS_DB="${TURNS_DB:-/tmp/spt-1-smoke.turns.db}"

SESSION="${SESSION:-spt-1-tui-smoke}"
PANE="${SESSION}:0.0"

WAIT_TIMEOUT_S="${WAIT_TIMEOUT_S:-180}"

cleanup() {
	tmux kill-session -t "${SESSION}" 2>/dev/null || true
}
trap cleanup EXIT

rm -f "${TIMELINE_DB}" "${TURNS_DB}"

CMD="cd '${PINOCCHIO_DIR}' && exec go run ./cmd/switch-profiles-tui \
  --profile-registries '${PROFILE_REGISTRIES}' \
  --profile '${PROFILE_A}' \
  --conv-id '${CONV_ID}' \
  --timeline-db '${TIMELINE_DB}' \
  --turns-db '${TURNS_DB}' \
  --log-level info"

tmux kill-session -t "${SESSION}" 2>/dev/null || true
tmux new-session -d -s "${SESSION}" "sh -lc ${CMD@Q}"

sleep 2

wait_for_turn_count() {
	local expected="$1"
	local start_ts
	start_ts="$(date +%s)"
	while true; do
		if [[ -f "${TURNS_DB}" ]]; then
			local count
			count="$(sqlite3 "${TURNS_DB}" "SELECT COUNT(*) FROM turns WHERE conv_id='${CONV_ID}'" 2>/dev/null || echo 0)"
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

echo "--- prompt 1 (profile=${PROFILE_A}) ---"
tmux send-keys -t "${PANE}" "Say just one word: OK." C-i
if ! wait_for_turn_count 1; then
	echo "FAIL: timeout waiting for first turn to persist (conv_id=${CONV_ID}, turns_db=${TURNS_DB})" >&2
	exit 1
fi

echo "--- switch profile (to=${PROFILE_B}) ---"
tmux send-keys -t "${PANE}" "/profile ${PROFILE_B}" C-i
sleep 1

echo "--- prompt 2 (profile=${PROFILE_B}) ---"
tmux send-keys -t "${PANE}" "Say just one word: OK." C-i
if ! wait_for_turn_count 2; then
	echo "FAIL: timeout waiting for second turn to persist (conv_id=${CONV_ID}, turns_db=${TURNS_DB})" >&2
	exit 1
fi

echo "--- quit ---"
# Quit (alt+q)
tmux send-keys -t "${PANE}" M-q
sleep 1

echo "--- capture-pane (tail) ---"
tmux capture-pane -t "${PANE}" -p | tail -n 160

echo "--- artifacts ---"
echo "conv_id=${CONV_ID}"
echo "timeline_db=${TIMELINE_DB}"
echo "turns_db=${TURNS_DB}"

