#!/usr/bin/env bash
set -euo pipefail

# Verify switch-profiles-tui refuses to start when registries load zero profiles.

PINOCCHIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

TMP_REGISTRY="$(mktemp /tmp/empty-profile-registry.XXXXXX.yaml)"
cleanup() { rm -f "${TMP_REGISTRY}"; }
trap cleanup EXIT

cat >"${TMP_REGISTRY}" <<'YAML'
slug: empty
display_name: Empty Registry
profiles: {}
YAML

set +e
OUT="$(
	cd "${PINOCCHIO_DIR}" && go run ./cmd/switch-profiles-tui \
		--profile-registries "${TMP_REGISTRY}" \
		--conv-id "spt-1-empty-registry" \
		--timeline-db "" \
		--turns-db "" 2>&1
)"
CODE="$?"
set -e

if [[ "${CODE}" -eq 0 ]]; then
	echo "FAIL: expected non-zero exit code when zero profiles are loaded" >&2
	echo "${OUT}" >&2
	exit 1
fi

if ! echo "${OUT}" | grep -q "no profiles loaded"; then
	echo "FAIL: expected error output to mention \"no profiles loaded\"" >&2
	echo "${OUT}" >&2
	exit 1
fi

echo "OK: startup fails when zero profiles are loaded"

