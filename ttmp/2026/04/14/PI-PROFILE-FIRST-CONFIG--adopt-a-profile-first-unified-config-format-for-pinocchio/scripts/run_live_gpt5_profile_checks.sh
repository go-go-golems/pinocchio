#!/usr/bin/env bash
set -euo pipefail
BIN="${1:-/tmp/pinocchio-live}"
shift || true
WORKDIR="${PINOCCHIO_TEST_WORKDIR:-$(mktemp -d /tmp/pinocchio-live-run-XXXXXX)}"
LOGDIR="${PINOCCHIO_TEST_LOGDIR:-$(mktemp -d /tmp/pinocchio-live-logs-XXXXXX)}"
profiles=("$@")
if [[ ${#profiles[@]} -eq 0 ]]; then
  profiles=(gpt-5-mini gpt-5 gpt-5-low gpt-5-nano gpt-5-nano-low)
fi
mkdir -p "$WORKDIR" "$LOGDIR"
for profile in "${profiles[@]}"; do
  log="$LOGDIR/${profile}.log"
  echo "=== PROFILE: $profile ==="
  if (cd "$WORKDIR" && "$BIN" --log-level debug --with-caller --profile "$profile" code professional --non-interactive hello >"$log" 2>&1); then
    echo "status: success"
  else
    echo "status: failure"
  fi
  model_line=$(grep -m1 'Responses: built request' "$log" || true)
  http_line=$(grep -m1 'HTTP response received' "$log" || true)
  echo "model: ${model_line:-<missing>}"
  echo "http: ${http_line:-<missing>}"
  echo "assistant:"
  awk '
    /--- Output started ---/ {capture=1; next}
    /--- Output ended ---/ {capture=0}
    capture {print}
  ' "$log" || true
  echo "log: $log"
  echo
 done
