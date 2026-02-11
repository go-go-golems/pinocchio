#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(dirname -- "$SCRIPT_DIR")
WEB_DIR="$ROOT_DIR/web"

if ! command -v npm >/dev/null 2>&1; then
	echo "npm is required to build cmd/web-chat frontend" >&2
	exit 1
fi

attempt=1
max_attempts=2

while [ "$attempt" -le "$max_attempts" ]; do
	echo "frontend install attempt $attempt/$max_attempts"
	rm -rf "$WEB_DIR/node_modules"

	if npm --prefix "$WEB_DIR" ci --include=dev --no-audit --no-fund; then
		npm --prefix "$WEB_DIR" run build
		exit 0
	fi

	if [ "$attempt" -ge "$max_attempts" ]; then
		echo "frontend install failed after $max_attempts attempts" >&2
		exit 1
	fi

	echo "npm ci failed; retrying after clean node_modules" >&2
	attempt=$((attempt + 1))
	sleep 1
done
