#!/usr/bin/env bash
set -euo pipefail
BIN="${1:-/tmp/pinocchio-live}"
PROFILE="${2:-gpt-5-mini}"
WORKDIR="${PINOCCHIO_TEST_WORKDIR:-$(mktemp -d /tmp/pinocchio-live-print-XXXXXX)}"
mkdir -p "$WORKDIR"
(cd "$WORKDIR" && "$BIN" --log-level debug --with-caller --profile "$PROFILE" code professional --print-inference-settings --non-interactive hello)
