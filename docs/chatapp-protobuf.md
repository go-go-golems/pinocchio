# Chatapp protobuf schema publishing

## Overview

Pinocchio owns the protobuf definitions for the chatapp runtime. These schemas describe the chat WebSocket frames, runtime events, durable snapshot entities, frontend tool payloads, and widget payloads used by the Go backend and React frontends.

The source of truth is the `.proto` tree in this repository:

```text
proto/pinocchio/chatapp/v1/chat.proto
proto/pinocchio/chatapp/rpc/v1/rpc.proto
proto/pinocchio/chatapp/frontendtools/v1/frontend_tool.proto
proto/pinocchio/chatapp/widgets/v1/widget.proto
```

The Buf Schema Registry module for these schemas is:

```text
buf.build/go-go-golems/pinocchio-chatapp
```

Use this module when another repository needs authoritative chatapp schemas without depending on the whole Pinocchio repository.

## Local validation

Run these commands before committing proto or Buf configuration changes:

```bash
buf dep update
buf format -w
buf format --diff --exit-code
buf lint
buf build
```

After the module has been published at least once, also check breaking changes against the registry:

```bash
buf breaking --against-registry
```

Before the first registry publish, compare against a Git reference instead:

```bash
buf breaking . --against 'https://github.com/go-go-golems/pinocchio.git#format=git,branch=main'
```

## Code generation

Generate Go bindings:

```bash
buf generate --template buf.chatapp.gen.yaml
```

Generate TypeScript bindings for the web chat app:

```bash
buf generate --template buf.chatapp.web.gen.yaml
```

A downstream TypeScript package can generate directly from the published BSR module:

```bash
buf generate buf.build/go-go-golems/pinocchio-chatapp --template buf.gen.yaml
```

For reusable frontend packages, prefer generating a small npm package such as `@go-go-golems/chatapp-proto` from the BSR module instead of importing from `cmd/web-chat/web/src/generated`.

## One-time BSR setup

An organization owner or maintainer with Buf permissions must create the module once:

```bash
buf registry module create buf.build/go-go-golems/pinocchio-chatapp \
  --visibility public \
  --default-label-name main
```

If the organization wants schemas private initially, use `--visibility private` and update consumers/auth accordingly.

The GitHub Actions workflow reads the Buf token from Vault through GitHub Actions OIDC. Store the token at:

```text
kv/ci/buf/pinocchio-chatapp
  token = <Buf API token>
```

The corresponding Vault JWT role is:

```text
bsr-pinocchio-chatapp-publisher
```

That role is bound to `go-go-golems/pinocchio`, `refs/heads/main`, push events, and this workflow:

```text
go-go-golems/pinocchio/.github/workflows/buf-ci.yaml@refs/heads/main
```

Do not store the Buf token as a GitHub repository secret unless Vault OIDC is unavailable.

## Manual publishing

Login locally if needed:

```bash
buf registry login
```

Push the current schema to the default label:

```bash
buf push --label main --git-metadata
```

For a release tag, apply a release label as well:

```bash
buf push --label main --label v0.11.0 --git-metadata
```

A successful push prints a BSR commit ID similar to:

```text
buf.build/go-go-golems/pinocchio-chatapp:<commit-id>
```

Record that commit ID in release notes when a frontend package depends on it.

## CI behavior

`.github/workflows/buf-ci.yaml` uses `hashicorp/vault-action@v3` and `bufbuild/buf-action@v1`.

Expected behavior:

- Pull requests run Buf build, lint, format, and breaking-change checks without reading the Buf token.
- Pushes to `refs/heads/main` authenticate to Vault with GitHub Actions OIDC, read `kv/data/ci/buf/pinocchio-chatapp token`, and publish the named module to the BSR.
- Archive is disabled for now because delete events do not need the Buf token and do not have a matching Vault role.

The workflow is path-filtered so it only runs when protobuf, Buf, documentation/license, or workflow files relevant to schema publishing change.

## Compatibility rules

Follow protobuf compatibility rules carefully because React clients may decode these messages from WebSocket JSON payloads.

Safe or usually safe changes:

- Add a new field with a new field number.
- Add a new message type.
- Add a new enum value at the end of an enum.
- Add optional metadata that old clients can ignore.

Dangerous or breaking changes:

- Reusing a deleted field number.
- Changing a field type.
- Renaming a field if JSON/protojson consumers rely on the JSON name.
- Removing a message or enum value used by published clients.
- Changing semantic meaning while keeping the same field name and number.

When removing a field, reserve both its number and name:

```proto
message Example {
  reserved 4;
  reserved "old_field";
}
```

Prefer additive migrations. Keep old fields long enough for released clients to update, and document any intentionally breaking change in the pull request.
