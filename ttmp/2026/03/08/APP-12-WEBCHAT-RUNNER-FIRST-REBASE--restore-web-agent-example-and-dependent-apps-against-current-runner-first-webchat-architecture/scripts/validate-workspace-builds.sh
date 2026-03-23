#!/usr/bin/env bash
set -euo pipefail

ROOT="/home/manuel/workspaces/2026-03-02/os-openai-app-server"

echo "== workspace modules =="
(
  cd "$ROOT/web-agent-example"
  go list -m -json github.com/go-go-golems/go-go-os-chat
)

echo
echo "== web-agent-example =="
(
  cd "$ROOT/web-agent-example"
  go build ./cmd/web-agent-example
)

echo
echo "== go-go-os-chat =="
(
  cd "$ROOT/go-go-os-chat"
  go build ./pkg/...
)

echo
echo "== wesen-os assistant backend =="
(
  cd "$ROOT/wesen-os"
  go build ./pkg/assistantbackendmodule
)

echo
echo "== linked inventory app =="
(
  cd "$ROOT/wesen-os/workspace-links/go-go-app-inventory"
  go build ./...
)

echo
echo "== linked arc-agi app =="
(
  cd "$ROOT/wesen-os/workspace-links/go-go-app-arc-agi-3"
  go build ./...
)

echo
echo "Validation complete."
