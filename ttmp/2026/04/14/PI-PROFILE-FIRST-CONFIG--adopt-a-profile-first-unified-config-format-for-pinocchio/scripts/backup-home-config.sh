#!/usr/bin/env bash
set -euo pipefail
src="${1:-$HOME/.pinocchio/config.yaml}"
stamp="${2:-$(date +%F-%H%M%S)}"
if [[ ! -f "$src" ]]; then
  echo "missing config: $src" >&2
  exit 1
fi
backup="$(dirname "$src")/$(basename "$src").bak-${stamp}"
cp -p "$src" "$backup"
echo "$backup"
