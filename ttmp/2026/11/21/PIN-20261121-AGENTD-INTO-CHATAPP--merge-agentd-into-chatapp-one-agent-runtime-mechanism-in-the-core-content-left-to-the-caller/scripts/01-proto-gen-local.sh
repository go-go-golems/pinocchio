#!/usr/bin/env bash
# Regenerate pinocchio's chatapp protobuf code (Go and web-chat TypeScript)
# with locally installed plugins, for machines without a valid Buf Schema
# Registry token (`make proto-gen-core` uses buf.build remote plugins).
#
# The plugin versions match the headers of the committed generated files:
#   protoc-gen-go v1.36.12  (pkg/chatapp/pb/proto/.../*.pb.go)
#   protoc-gen-es v2.14.0   (cmd/web-chat/web/src/generated/.../*_pb.ts)
#
# Usage: run from the pinocchio repository root.
#   ttmp/2026/11/21/PIN-20261121-AGENTD-INTO-CHATAPP--*/scripts/01-proto-gen-local.sh [workdir]
set -euo pipefail
WORK="${1:-$(mktemp -d)}"
mkdir -p "$WORK/bin"
GOBIN="$WORK/bin" go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
npm install --silent --prefix "$WORK/es" @bufbuild/protoc-gen-es@2.14.0
ln -sf "$WORK/es/node_modules/.bin/protoc-gen-es" "$WORK/bin/protoc-gen-es"
cat > "$WORK/buf.gen.go.yaml" <<'YAML'
version: v1
plugins:
  - plugin: go
    out: pkg/chatapp/pb/proto
    opt:
      - paths=source_relative
YAML
cat > "$WORK/buf.gen.es.yaml" <<'YAML'
version: v1
plugins:
  - plugin: es
    out: cmd/web-chat/web/src/generated/chatapp/proto
    opt:
      - target=ts
      - import_extension=none
YAML
export PATH="$WORK/bin:$PATH"
buf generate --template "$WORK/buf.gen.go.yaml" --path proto/pinocchio
buf generate --template "$WORK/buf.gen.es.yaml" --path proto/pinocchio
(cd cmd/web-chat/web && npx --yes @biomejs/biome@2.3.8 check --write src/generated/chatapp/proto/pinocchio/chatapp)
git status --short -- pkg/chatapp/pb cmd/web-chat/web/src/generated
