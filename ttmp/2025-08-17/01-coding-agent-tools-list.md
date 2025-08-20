### Useful tools for a coding LLM agent

This is a concise, high-signal catalog of tools that enable a code-focused LLM agent to understand codebases, make edits safely, run and validate changes, and operate within secure and observable boundaries. Each category lists battle-tested options with a short rationale so the agent (and humans supervising it) can pick the right tool quickly.

### Repository navigation and search
- **ripgrep (rg)**: Ultra-fast exact-text and regex search that honors `.gitignore`; the default for code spelunking.
- **fd**: Faster, user-friendly `find` replacement for listing files/directories.
- **fzf**: Fuzzy finder to interactively filter files, symbols, and command history.
- **ctags/Universal Ctags**: Generates symbol indexes for jumping to definitions across languages.
- **ast-grep / tree-sitter CLI**: Structural (AST) search and replace; safer than regex for code-aware queries.

### Semantic code understanding
- **Language Servers (LSP)**: `gopls`, `clangd`, `rust-analyzer`, `pyright/pylance`, `typescript-language-server` for xref, diagnostics, and refactors.
- **SourceGraph / OpenGrok (optional infra)**: Cross-repo semantic code navigation at scale.
- **Embeddings + vector search**: Build repo indexes with tools like FAISS, Qdrant, Weaviate to enable semantic retrieval for prompts.

### Editing, refactoring, and formatting
- **Formatters**: `gofmt/goimports`, `black`, `prettier`, `rustfmt`, `clang-format` to enforce consistent style.
- **Codemods**: `jscodeshift` (JS/TS), `ts-morph`, `rope` (Python), `rustfix` (Rust), `gofix` (Go) for scripted refactors.
- **Multi-file editors**: `sed`, `perl -pi`, `ripgrep | xargs` combos; prefer AST/codemods for non-trivial transforms.
- **Pre-commit hooks**: `pre-commit` to run formatters, linters, secret scans before commits.

### Build, test, and coverage
- **Build tools**: `go build`, `cargo`, `pip/uv + build`, `npm/pnpm/bun`, `bazel` for larger monorepos.
- **Test runners**: `go test`, `pytest`, `jest/vitest`, `cargo test`, `pytest-xdist` for parallelism.
- **Coverage**: `go tool cover`, `coverage.py`, `nyc`, `grcov` to enforce quality gates.
- **Mutation testing**: `stryker` (JS/TS), `mutmut` (Python), `pitest` (JVM) to harden test suites.
- **Fuzzing / property testing**: `go test -fuzz`, `hypothesis` (Python), `quickcheck` (Rust), `AFL/LibFuzzer`.

### Static analysis and security
- **Linters**: `golangci-lint`, `ruff`/`flake8`, `eslint`, `clippy`, `pylint` for early defect detection.
- **Type checkers**: `pyright`, `mypy`, `tsc`, `mvn verify` (JVM) to prevent entire classes of bugs.
- **SAST/Dependency scanning**: `semgrep`, `bandit`, `npm audit`, `pip-audit`, `cargo audit`, `safety`.
- **Secrets and policy**: `gitleaks`, `truffleHog`, `opa/conftest` for guardrails and compliance.

### Runtime debugging and profiling
- **Debuggers**: `dlv` (Go), `gdb/lldb`, Node/Chrome DevTools for step-through debugging.
- **Profilers**: `pprof` (Go), `perf`, `py-spy`, `rbspy`, `cargo flamegraph` for CPU/mem hotspots.
- **Tracing/logs**: `strace/ltrace`, `open-telemetry` SDKs and collectors; `jq`-driven log pipelines.

### HTTP, RPC, and data tooling
- **HTTP clients**: `curl`, `httpie`, `xh` for scripting API checks.
- **gRPC**: `grpcurl`, `evans`, `protoc` toolchain for schema-first workflows.
- **WebSocket/MQ**: `wscat`, `mosquitto-clients`, `kafka-console-*` for realtime backends.
- **Data helpers**: `jq`, `yq`, `csvkit`, `sqlite3`, `duckdb` for transforming fixtures and results.

### Environment, packaging, and isolation
- **Package managers**: `uv/pip`, `pipenv/poetry`, `npm/pnpm/bun`, `cargo`, `go mod`.
- **Virtual env**: `uv venv`, `virtualenv`, `pyenv`, `asdf` for language/runtime pinning.
- **Containers**: `docker/podman`, `docker compose` to sandbox runs and tests.
- **Reproducibility**: `nix`, `direnv`, `.tool-versions` (asdf) to stabilize dev environments.

### Orchestration and task runners
- **Make/Task**: `make`, `just`, `taskfile` to codify repeatable steps for the agent.
- **Job control**: `GNU parallel`, background processes, and timeouts for non-interactive automation.

### Git and review workflows
- **Core Git**: `git status/add/commit/rebase/restore/clean` with `--patch` to keep edits minimal.
- **PR tooling**: `gh` (GitHub CLI) or `glab` (GitLab) for non-interactive PRs, checks, artifacts.
- **Diff UX**: `delta`, `difftastic` for semantic diffs that are easier to review.

### Observability in dev and CI
- **Log tailing**: `tail -F`, `stern` (K8s), `k9s` for interactive cluster views.
- **Metrics and traces**: local `prometheus` + `grafana`, `tempo/jaeger` for distributed systems.
- **Coverage/reporting in CI**: codecov or built-in CI artifacts to gate merges.

### Web and frontend specifics
- **Build/test**: `vite`, `webpack`, `jest/vitest`, `playwright` or `cypress` for E2E.
- **Accessibility**: `axe-core` CLI for automated checks.
- **Bundle analysis**: `source-map-explorer`, `webpack-bundle-analyzer`.

### LLM- and agent-specific enablers
- **Local model runtimes**: `Ollama`, `vLLM`, `text-generation-inference` for on-box inference.
- **Prompt/devtools**: `OpenAI/Anthropic/Bedrock` CLIs/SDKs, `liteLLM` for provider abstraction.
- **RAG components**: `llamaindex`, `langchain`, `chromadb/qdrant/weaviate/faiss` for retrieval.
- **Evaluation**: `helm`, `lm-eval-harness`, `promptfoo` to score changes and regressions.
- **Safety**: sandboxing (containers, `firejail`), resource limits (cgroups/ulimits), timeouts, network egress controls.

### Practical conventions for an autonomous coding agent
- **Non-interactive defaults**: Prefer commands with `--yes/--force/--non-interactive` flags.
- **Idempotence**: Re-run safe; encode preconditions and checks in `make`/`just` targets.
- **Fast feedback**: Run file-scoped tests/linters first; gate broader suites behind quick checks.
- **Change containment**: Create topic branches, make minimal diffs, and attach artifacts (logs, coverage) to PRs.

### Minimal starter toolchain (quick pick)
- **Search**: ripgrep + fzf
- **Refactor**: formatter + language-specific codemod
- **Test**: project’s native test runner + coverage
- **Lint**: project’s linter + type checker
- **Git**: delta for diffs, pre-commit for guards
- **Sandbox**: docker compose for services


