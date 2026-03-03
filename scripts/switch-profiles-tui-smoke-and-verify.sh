#!/usr/bin/env bash
set -euo pipefail

PINOCCHIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

"${PINOCCHIO_DIR}/scripts/switch-profiles-tui-tmux-smoke.sh"
"${PINOCCHIO_DIR}/scripts/switch-profiles-tui-verify-persistence.sh"

