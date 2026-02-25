#!/usr/bin/env bash
set -euo pipefail

IFS=$'\n\t'

usage() {
  cat <<'EOF'
Usage:
  scripts/profile_registry_cutover_smoke.sh [options]

End-to-end migration/smoke script:
1) Backup existing profiles YAML
2) Convert legacy profiles YAML -> canonical registry YAML
3) Import canonical registry YAML -> SQLite profile registry DB
4) Start web-chat using that DB
5) Smoke test web-chat profile selection/runtime metadata
6) Smoke test pinocchio profile loading with --print-parsed-fields

Options:
  --profile-file PATH   Source profiles YAML (default: $XDG_CONFIG_HOME/pinocchio/profiles.yaml or ~/.config/pinocchio/profiles.yaml)
  --registry SLUG       Preferred registry slug for tests/default legacy fallback (default: default)
  --db PATH             Target SQLite DB path (default: <work-dir>/profiles.db)
  --work-dir PATH       Working directory for generated files/logs
  --port PORT           web-chat port (default: 18080)
  --keep-server         Keep web-chat running after script exits
  -h, --help            Show this help

Examples:
  scripts/profile_registry_cutover_smoke.sh
  scripts/profile_registry_cutover_smoke.sh --profile-file /tmp/profiles.yaml --registry team --port 18081
EOF
}

log() {
  printf '[%s] %s\n' "$(date +%H:%M:%S)" "$*"
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
PROFILE_FILE="${DEFAULT_CONFIG_HOME}/pinocchio/profiles.yaml"
REGISTRY_SLUG="default"
PORT="18080"
KEEP_SERVER=0
STAMP="$(date +%Y%m%d-%H%M%S)"
WORK_DIR="${ROOT_DIR}/.tmp/profile-registry-cutover-${STAMP}"
DB_PATH=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile-file)
      [[ $# -ge 2 ]] || die "missing value for --profile-file"
      PROFILE_FILE="$2"
      shift 2
      ;;
    --registry)
      [[ $# -ge 2 ]] || die "missing value for --registry"
      REGISTRY_SLUG="$2"
      shift 2
      ;;
    --db)
      [[ $# -ge 2 ]] || die "missing value for --db"
      DB_PATH="$2"
      shift 2
      ;;
    --work-dir)
      [[ $# -ge 2 ]] || die "missing value for --work-dir"
      WORK_DIR="$2"
      shift 2
      ;;
    --port)
      [[ $# -ge 2 ]] || die "missing value for --port"
      PORT="$2"
      shift 2
      ;;
    --keep-server)
      KEEP_SERVER=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

need_cmd go
need_cmd curl
need_cmd jq
need_cmd rg

[[ -f "${PROFILE_FILE}" ]] || die "profile file does not exist: ${PROFILE_FILE}"
[[ "${PORT}" =~ ^[0-9]+$ ]] || die "port must be numeric"

mkdir -p "${WORK_DIR}"
if [[ -z "${DB_PATH}" ]]; then
  DB_PATH="${WORK_DIR}/profiles.db"
fi
CANONICAL_YAML="${WORK_DIR}/profiles.registry.yaml"
SUMMARY_JSON="${WORK_DIR}/import-summary.json"
WEBCHAT_LOG="${WORK_DIR}/web-chat.log"
PIN_PARSED_FIELDS="${WORK_DIR}/pinocchio-print-parsed-fields.yaml"
EMPTY_CONFIG="${WORK_DIR}/empty-config.yaml"
IMPORT_HELPER="${WORK_DIR}/import_profiles_to_sqlite.go"

WEBCHAT_PID=""
cleanup() {
  if [[ -n "${WEBCHAT_PID}" ]] && kill -0 "${WEBCHAT_PID}" >/dev/null 2>&1; then
    if [[ "${KEEP_SERVER}" -eq 1 ]]; then
      log "Keeping web-chat running (pid=${WEBCHAT_PID})"
    else
      log "Stopping web-chat (pid=${WEBCHAT_PID})"
      kill "${WEBCHAT_PID}" >/dev/null 2>&1 || true
      wait "${WEBCHAT_PID}" >/dev/null 2>&1 || true
    fi
  fi
}
trap cleanup EXIT

log "Working directory: ${WORK_DIR}"
log "Source profile file: ${PROFILE_FILE}"

backup_dir="$(dirname "${PROFILE_FILE}")/backups"
mkdir -p "${backup_dir}"
backup_file="${backup_dir}/profiles.yaml.${STAMP}.bak"
cp "${PROFILE_FILE}" "${backup_file}"
log "Backup created: ${backup_file}"

log "Converting source profiles to canonical registry YAML"
(
  cd "${ROOT_DIR}"
  go run ./cmd/pinocchio profiles migrate-legacy \
    --input "${PROFILE_FILE}" \
    --output "${CANONICAL_YAML}" \
    --registry "${REGISTRY_SLUG}" \
    --force >/dev/null
)
log "Canonical registry YAML: ${CANONICAL_YAML}"

cat > "${IMPORT_HELPER}" <<'EOF'
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
)

type registrySummary struct {
	Slug           string   `json:"slug"`
	DefaultProfile string   `json:"default_profile,omitempty"`
	Profiles       []string `json:"profiles"`
}

type importSummary struct {
	ImportedCount int               `json:"imported_count"`
	Registries    []registrySummary `json:"registries"`
}

func main() {
	var inputPath string
	var dbPath string
	var defaultRegistryRaw string
	flag.StringVar(&inputPath, "input", "", "canonical registry YAML path")
	flag.StringVar(&dbPath, "db", "", "sqlite db path")
	flag.StringVar(&defaultRegistryRaw, "default-registry", "default", "default registry slug for decoding legacy input")
	flag.Parse()

	if inputPath == "" {
		fatalf("missing --input")
	}
	if dbPath == "" {
		fatalf("missing --db")
	}
	defaultRegistry, err := gepprofiles.ParseRegistrySlug(defaultRegistryRaw)
	if err != nil {
		fatalf("parse default registry slug: %v", err)
	}

	raw, err := os.ReadFile(inputPath)
	if err != nil {
		fatalf("read %s: %v", inputPath, err)
	}
	registries, err := gepprofiles.DecodeYAMLRegistries(raw, defaultRegistry)
	if err != nil {
		fatalf("decode registries: %v", err)
	}
	if len(registries) == 0 {
		fatalf("no registries decoded from %s", inputPath)
	}

	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		fatalf("remove existing db %s: %v", dbPath, err)
	}
	dsn, err := gepprofiles.SQLiteProfileDSNForFile(dbPath)
	if err != nil {
		fatalf("build sqlite dsn: %v", err)
	}
	store, err := gepprofiles.NewSQLiteProfileStore(dsn, defaultRegistry)
	if err != nil {
		fatalf("open sqlite store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	for _, registry := range registries {
		if registry == nil {
			continue
		}
		if err := store.UpsertRegistry(ctx, registry, gepprofiles.SaveOptions{
			Actor:  "profile-registry-cutover-smoke",
			Source: "migration-script",
		}); err != nil {
			fatalf("upsert registry %q: %v", registry.Slug, err)
		}
	}

	summary := importSummary{
		ImportedCount: len(registries),
		Registries:    make([]registrySummary, 0, len(registries)),
	}
	for _, registry := range registries {
		if registry == nil {
			continue
		}
		profiles := make([]string, 0, len(registry.Profiles))
		for slug := range registry.Profiles {
			profiles = append(profiles, slug.String())
		}
		sort.Strings(profiles)
		defaultProfile := registry.DefaultProfileSlug.String()
		if defaultProfile == "" && len(profiles) > 0 {
			defaultProfile = profiles[0]
		}
		summary.Registries = append(summary.Registries, registrySummary{
			Slug:           registry.Slug.String(),
			DefaultProfile: defaultProfile,
			Profiles:       profiles,
		})
	}
	sort.Slice(summary.Registries, func(i, j int) bool {
		return summary.Registries[i].Slug < summary.Registries[j].Slug
	})

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		fatalf("encode summary: %v", err)
	}
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
EOF

log "Importing canonical registry YAML into SQLite profile DB"
(
  cd "${ROOT_DIR}"
  go run "${IMPORT_HELPER}" --input "${CANONICAL_YAML}" --db "${DB_PATH}" --default-registry "${REGISTRY_SLUG}" > "${SUMMARY_JSON}"
)
log "SQLite profile DB: ${DB_PATH}"
log "Import summary: ${SUMMARY_JSON}"

SELECTED_REGISTRY="$(jq -r --arg preferred "${REGISTRY_SLUG}" '
  if any(.registries[]; .slug == $preferred) then
    $preferred
  else
    .registries[0].slug
  end
' "${SUMMARY_JSON}")"

SELECTED_PROFILE="$(jq -r --arg reg "${SELECTED_REGISTRY}" '
  (.registries[] | select(.slug == $reg) | (.default_profile // .profiles[0] // ""))
' "${SUMMARY_JSON}")"

[[ -n "${SELECTED_REGISTRY}" ]] || die "could not determine registry to test"
[[ -n "${SELECTED_PROFILE}" ]] || die "could not determine profile to test in registry ${SELECTED_REGISTRY}"

log "Selected registry/profile for smoke checks: ${SELECTED_REGISTRY}/${SELECTED_PROFILE}"

log "Starting web-chat on :${PORT}"
(
  cd "${ROOT_DIR}"
  go run ./cmd/web-chat web-chat \
    --addr ":${PORT}" \
    --profile-registry-db "${DB_PATH}" \
    > "${WEBCHAT_LOG}" 2>&1
) &
WEBCHAT_PID="$!"

BASE_URL="http://127.0.0.1:${PORT}"
ready=0
for _ in $(seq 1 60); do
  if curl -fsS "${BASE_URL}/api/chat/profiles" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
[[ "${ready}" -eq 1 ]] || {
  tail -n 120 "${WEBCHAT_LOG}" >&2 || true
  die "web-chat did not become ready on ${BASE_URL}"
}
log "web-chat is ready"

webchat_profiles_json="${WORK_DIR}/webchat-profiles.json"
curl -fsS "${BASE_URL}/api/chat/profiles?registry=${SELECTED_REGISTRY}" > "${webchat_profiles_json}"
jq -e --arg slug "${SELECTED_PROFILE}" 'any(.[]; .slug == $slug)' "${webchat_profiles_json}" >/dev/null \
  || die "profile ${SELECTED_PROFILE} not present in /api/chat/profiles?registry=${SELECTED_REGISTRY}"
log "web-chat profiles endpoint includes selected profile"

chat_default_resp="${WORK_DIR}/chat-default.json"
chat_default_status="$(curl -sS -o "${chat_default_resp}" -w '%{http_code}' \
  -X POST "${BASE_URL}/chat" \
  -H 'content-type: application/json' \
  -d '{"prompt":"smoke default","conv_id":"smoke-default-script"}')"
[[ "${chat_default_status}" == "200" ]] || die "default chat failed (${chat_default_status}): $(cat "${chat_default_resp}")"
jq -e '.runtime_fingerprint | startswith("sha256:")' "${chat_default_resp}" >/dev/null \
  || die "default chat response missing runtime_fingerprint"
jq -e '.profile_metadata["profile.slug"] | type == "string"' "${chat_default_resp}" >/dev/null \
  || die "default chat response missing profile metadata slug"
log "default /chat smoke passed"

chat_explicit_req="${WORK_DIR}/chat-explicit-request.json"
chat_explicit_resp="${WORK_DIR}/chat-explicit.json"
jq -n \
  --arg prompt "smoke explicit" \
  --arg conv "smoke-explicit-script" \
  --arg runtime "${SELECTED_PROFILE}" \
  --arg registry "${SELECTED_REGISTRY}" \
  '{prompt:$prompt, conv_id:$conv, runtime_key:$runtime, registry_slug:$registry}' \
  > "${chat_explicit_req}"

chat_explicit_status="$(curl -sS -o "${chat_explicit_resp}" -w '%{http_code}' \
  -X POST "${BASE_URL}/chat" \
  -H 'content-type: application/json' \
  --data-binary "@${chat_explicit_req}")"
[[ "${chat_explicit_status}" == "200" ]] || die "explicit chat failed (${chat_explicit_status}): $(cat "${chat_explicit_resp}")"
jq -e \
  --arg runtime "${SELECTED_PROFILE}" \
  --arg registry "${SELECTED_REGISTRY}" \
  '.profile_metadata["profile.slug"] == $runtime and .profile_metadata["profile.registry"] == $registry' \
  "${chat_explicit_resp}" >/dev/null || die "explicit /chat response does not match selected profile metadata"
log "explicit runtime_key/registry_slug /chat smoke passed"

invalid_runtime_body="${WORK_DIR}/chat-invalid-runtime.txt"
invalid_runtime_status="$(curl -sS -o "${invalid_runtime_body}" -w '%{http_code}' \
  -X POST "${BASE_URL}/chat" \
  -H 'content-type: application/json' \
  -d '{"prompt":"x","conv_id":"smoke-bad-runtime","runtime_key":"bad slug!"}')"
[[ "${invalid_runtime_status}" == "400" ]] || die "invalid runtime should return 400, got ${invalid_runtime_status}"
rg -q "invalid runtime_key" "${invalid_runtime_body}" || die "invalid runtime error body did not include expected message"
log "invalid runtime_key validation smoke passed"

printf '{}\n' > "${EMPTY_CONFIG}"
log "Running pinocchio --print-parsed-fields with migrated profile source"
(
  cd "${ROOT_DIR}"
  go run ./cmd/pinocchio generate-prompt \
    --goal "profile registry smoke" \
    --profile-file "${CANONICAL_YAML}" \
    --profile "${SELECTED_PROFILE}" \
    --config-file "${EMPTY_CONFIG}" \
    --print-parsed-fields > "${PIN_PARSED_FIELDS}"
)

rg -q '^profile-settings:' "${PIN_PARSED_FIELDS}" || die "print-parsed-fields output missing profile-settings section"
rg -q 'mode: profile-registry' "${PIN_PARSED_FIELDS}" || die "print-parsed-fields output missing profile-registry middleware marker"
rg -F -q "profileFile: ${CANONICAL_YAML}" "${PIN_PARSED_FIELDS}" || die "print-parsed-fields output missing expected profile file path"
rg -q "profile:\s*${SELECTED_PROFILE}" "${PIN_PARSED_FIELDS}" || die "print-parsed-fields output missing expected profile value"
log "pinocchio profile middleware smoke passed"

cat <<EOF

Smoke run completed successfully.

Artifacts:
  backup profile file: ${backup_file}
  canonical registry yaml: ${CANONICAL_YAML}
  sqlite registry db: ${DB_PATH}
  import summary: ${SUMMARY_JSON}
  web-chat log: ${WEBCHAT_LOG}
  web-chat profiles response: ${webchat_profiles_json}
  web-chat chat responses:
    - ${chat_default_resp}
    - ${chat_explicit_resp}
  pinocchio --print-parsed-fields output: ${PIN_PARSED_FIELDS}

Selected registry/profile:
  registry: ${SELECTED_REGISTRY}
  profile:  ${SELECTED_PROFILE}
EOF
