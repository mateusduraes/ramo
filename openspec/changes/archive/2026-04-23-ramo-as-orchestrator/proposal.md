## Why

Ramo today is a thin CLI that couples git worktree creation with the external `cmux` terminal multiplexer. Its value is limited to a narrow terminal-layout use case. We want to reposition ramo as a **parallel orchestrator for AI coding agents**: a Go library that spins up multiple Claude Code instances across isolated git worktrees, runs user-declared pipelines (implement → verify → open PR) with Ralph Loop semantics, and produces draft PRs automatically. The existing `worktree/` package gives us a head start; cmux is dead weight.

## What Changes

- **BREAKING** — Remove the cmux integration entirely: delete `cmux/`, `cmd/open.go`, `cmd/new.go`, the `Workspace`/`Pane` fields in config, and all cmux teardown in `cmd/remove.go`.
- **BREAKING** — Remove the `ramo.json` config file and the `config/` package. Orchestrator configuration now lives in user-authored Go code (a `ramo.go` that imports the ramo library).
- **BREAKING** — Reduce the `ramo` binary to three commands: `ramo init`, `ramo list`, `ramo remove`. `ramo new` and `ramo open` are dropped.
- **NEW** — Add a Go library for orchestrating parallel agent pipelines. Users import it from their own `main.go` (Sandcastle-style) and invoke `orchestrator.Run(Config{...})` with a list of `Worktree`s, each containing `Setup`, `Copy`, and a pipeline of `Step`s.
- **NEW** — Each `Step` runs either inside a Docker container (`RunOn: Container`, default) or on the host (`RunOn: Host`). Steps spawn `claude -p` subprocesses with `--dangerously-skip-permissions`. Ralph Loop is implemented as an outer Go loop: `MaxIterations` fresh claude invocations per step with early exit on a user-configurable `DoneSignal` substring match.
- **NEW** — Sandbox abstraction with a `Docker` implementation. Ramo ships a vendored Dockerfile template (not a prebuilt image) via `ramo init`. Container lifecycle is long-lived per worktree, worktree bind-mounted at `/workspace`, host UID mapped into the container, auth env vars passed through agnostically (API key, OAuth token, Bedrock, Vertex).
- **NEW** — Event-emitter observability architecture: the orchestrator emits a structured event stream consumed by independent subscribers. v1 ships with a file logger (per-worktree-step log files), a summary builder, and minimal stdout output. A TUI consumer is planned for v1.1 but out of scope now.
- **NEW** — Fail-fast pipeline semantics per worktree (one step failing halts that worktree; other worktrees continue). Single idle-timeout knob (default 10 min) is the only hang detection. Preflight checks before launching (docker daemon, claude CLI, Dockerfile buildability, prompt/copy-source existence).
- **NEW** — Worktree lifecycle: re-runs always resume existing worktrees (no configurability), ramo never auto-deletes, user cleans up via `ramo remove`.
- **NEW** — Commit/PR creation is **not a ramo primitive** — it's a user-authored host `Step` whose prompt uses `gh` / `glab` / whatever the host has installed. Ramo exposes "run a step on the host" and the user owns provider-specific logic.

## Capabilities

### New Capabilities

- `orchestrator`: The library's public surface and execution engine. Defines `Config`, `Worktree`, `Step`, `FileCopy` types; `Run()` entrypoint; per-worktree pipeline executor with fail-fast semantics; Ralph Loop mechanics (fresh subprocess per iteration, DoneSignal substring match); resume-on-every-re-run lifecycle; idle timeout; preflight checks; cancellation via context and SIGINT.
- `sandbox`: The `Sandbox` interface and the `docker.New(...)` implementation. Handles Dockerfile build with content-hash tagging, long-lived container per worktree (bind-mounted worktree, UID mapping, `CMD ["sleep", "infinity"]`), `Exec` for setup shell and agent invocations, auth env passthrough, teardown.
- `worktree`: Cmux-free git worktree CRUD (`Add`, `Remove`, `List`, `Exists`, `BranchExists`, `Fetch`, `DeleteBranch`). Mostly a simplification of today's `worktree/` package — drop cmux references, preserve the test suite, integrate with orchestrator's resume-on-existing behavior.
- `cli`: The shrunk `ramo` binary surface — `ramo init --provider github|gitlab` (scaffolds `.ramo/Dockerfile`, `ramo.go`, `.ramo/open-pr.prompt.md`, `.ramo/.gitignore`), `ramo list`, `ramo remove <branch>`. Drop `ramo new`, `ramo open`, `ramo.json` parsing.
- `observability`: The structured event stream and its pluggable consumers. Defines `Event` type, `Consumer` interface, ships `FileLogger` (writes `.ramo/logs/<runID>/<worktree>/<step>.log`) and `SummaryBuilder` (writes `.ramo/reports/run-<runID>.md`) + a minimal stdout consumer. Orchestrator depends on the stream; consumers are independent subscribers.

### Modified Capabilities

(None — the project has no pre-existing specs in `openspec/specs/`.)

## Impact

- **Code removed**: `cmux/` package, `config/` package, `cmd/new.go`, `cmd/open.go`, cmux references in `cmd/remove.go`, `ramo.json` parsing.
- **Code added**: `orchestrator/` package (public API + execution engine), `sandbox/` package with `sandbox/docker/` subpackage, `observability/` consumers, updated `cmd/init.go` for the new scaffolding output.
- **Dependencies added**: direct dependency on the Docker Engine API via `github.com/docker/docker/client` (or equivalent) for building images, creating and exec'ing into containers. No TUI library in v1.
- **Dependencies preserved**: `github.com/spf13/cobra` (CLI), `github.com/manifoldco/promptui` (interactive prompts — may be reviewed but kept for `ramo init`).
- **External tooling required on user's machine**: Docker daemon, `claude` CLI, whichever provider CLI the user's host step calls (`gh` / `glab` / etc).
- **Breaking migration for existing users**: any user of the cmux-backed ramo must delete `ramo.json` and re-run `ramo init` to get the new scaffolding. There is no automatic migration path; the tool's identity changes substantially.
- **Testing**: existing `worktree/` tests preserved (they predate cmux coupling). New packages get unit tests via Go stdlib `testing`. Docker-dependent tests gated behind a `//go:build integration` tag so `go test ./...` works without Docker; CI runs integration separately.
- **Authentication**: no code-level preference between API key, OAuth subscription token, Bedrock, Vertex — ramo passes through any of a known auth-env-var list plus user-declared additions. Preflight only *warns* on auth absence; claude produces the real error.
