#!/usr/bin/env bash
# Smoke test of `pinocchio agent serve` with the scripted engine: create a
# session with page instructions, register a browser tool, send a message that
# calls it, answer the call, read the conversation. No model credentials needed.
#
# Usage (from the pinocchio repository root):
#   ttmp/.../scripts/02-agent-serve-smoke.sh [workdir] [preset-dir]
# The preset dir defaults to a one-preset directory written into workdir.
set -euo pipefail
WORK="${1:-$(mktemp -d)}"
PRESETS="${2:-}"
PORT="${PORT:-8799}"
B="http://127.0.0.1:$PORT/v1"
mkdir -p "$WORK/state"
if [ -z "$PRESETS" ]; then
  PRESETS="$WORK/presets"; mkdir -p "$PRESETS"
  printf 'You drive the page.\n' > "$PRESETS/bench.md"
  printf 'name: bench\nsystemPrompt: bench.md\npageInstructions: true\nmaxRounds: 4\n' > "$PRESETS/bench.yaml"
fi
go build -o "$WORK/pinocchio" ./cmd/pinocchio
"$WORK/pinocchio" agent serve --presets "$PRESETS" --engine fake --address "127.0.0.1:$PORT" --state-dir "$WORK/state" > "$WORK/serve.log" 2>&1 &
SERVER=$!
trap 'kill $SERVER 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do curl -sf "$B/health" >/dev/null && break; sleep 0.1; done

curl -s "$B/health"; echo
curl -s "$B/presets"; echo
SID=$(curl -s -X POST "$B/sessions" -H 'Content-Type: application/json' \
  -d '{"data":{"instructions":"API: bench.help()"}}' | python3 -c 'import sys,json; print(json.load(sys.stdin)["sessionId"])')
echo "session $SID"
EX=$(curl -s -X POST "$B/sessions/$SID/tools/manifest" -H 'Content-Type: application/json' \
  -d '{"clientInstanceId":"tab","connectionId":"c1","revision":1,"tools":[{"name":"eval","mode":"frontend","inputSchema":{"type":"object","properties":{"code":{"type":"string"}}},"available":true}]}' \
  | python3 -c 'import sys,json; print(json.dumps(json.load(sys.stdin)["executor"]))')
(timeout 5 curl -sN "$B/sessions/$SID/events" > "$WORK/events.txt" &)
sleep 0.3
curl -s -X POST "$B/sessions/$SID/messages" -H 'Content-Type: application/json' -d '{"prompt":"/tool eval {\"code\":\"6*7\"}"}'; echo
sleep 0.5
CALL=$(grep -o '"toolCallId":"[^"]*"' "$WORK/events.txt" | head -1 | cut -d'"' -f4)
echo "tool call $CALL"
curl -s -X POST "$B/sessions/$SID/tools/results" -H 'Content-Type: application/json' \
  -d "{\"toolCallId\":\"$CALL\",\"toolName\":\"eval\",\"status\":\"success\",\"result\":42,\"executor\":$EX}"; echo
sleep 1
curl -s "$B/sessions/$SID" | python3 -c 'import sys,json; d=json.load(sys.stdin); print([(e["payload"].get("role"), e["payload"].get("text")) for e in d["snapshot"]["entities"] if e["kind"]=="ChatMessage"])'
echo "SSE event names:"; grep -o '"name":"[A-Za-z]*"' "$WORK/events.txt" | sort | uniq -c
