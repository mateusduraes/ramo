## Context

Ramo today is a ~500-line Go CLI that wraps `git worktree add` + an external `cmux` terminal multiplexer to give users isolated repo copies with pre-configured terminal layouts. The worktree layer (`worktree/`) is clean and well-tested; the cmux layer (`cmux/`) is tightly coupled to the commands in `cmd/` and is the feature we're discarding.

The pivot is to turn ramo into an orchestrator for **parallel AI coding agents**. A user authors a Go script (`ramo.go`) that imports ramo as a library, declares N worktrees each with a pipeline of steps, and runs it with `go run ./ramo.go`. Each worktree runs its pipeline inside a long-lived Docker container in parallel with the others. Each step invokes `claude -p` (the Claude Code CLI) with Ralph Loop semantics — fresh subprocess per iteration, early-exit on a done-signal substring match. The commit-and-open-PR tail is a user-authored *host step* that runs `claude` directly on the laptop with access to `gh`/`glab`/`git` credentials.

Design reference: [Sandcastle](https://github.com/mattpocock/sandcastle). Ramo's core loop mirrors Sandcastle's `Orchestrator.ts` pattern (for-loop over iterations, fresh CLI subprocess each, DoneSignal string match), adapted to Go idioms and specialized for git worktree orchestration.

## Goals / Non-Goals

**Goals:**
- Turn ramo into a library. User authors their own `main.go` that imports ramo; no yaegi, no Go plugins, no CLI ingesting scripts.
- Let one `go run ./ramo.go` kick off N parallel pipelines, each in its own worktree + Docker container.
- Preserve the existing `worktree/` package and test suite (minus cmux coupling).
- Keep the sandbox layer pluggable via a `Sandbox` interface; ship `docker.New(...)` as the v1 implementation.
- Ship a thin `ramo` binary for worktree maintenance (`init`, `list`, `remove`) — that's it.
- Observability via a structured event stream with pluggable consumers; v1 ships file logs + markdown report + minimal stdout output.
- Graceful failure modes: fail-fast per worktree (others continue), one Ctrl-C = graceful cleanup, single idle-timeout knob as the only hang detection.

**Non-Goals:**
- TUI dashboard. Deferred to v1.1. Event-emitter architecture keeps the door open.
- Auto-cleanup of worktrees, branches, or orphaned containers. User owns cleanup via `ramo remove`.
- Provider-aware code in the orchestrator (github vs gitlab). Provider is a `ramo init --provider` scaffolding preference only.
- Prebuilt Docker image maintenance. Ramo ships a Dockerfile template; users build locally.
- Cross-worktree barriers / phase synchronization. Each worktree is an independent pipeline.
- OAuth session-state reuse from `~/.claude/`. v1 supports auth via env-var passthrough only (`ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN`, Bedrock/Vertex vars).
- Per-step or per-run wall-clock timeouts. Only the idle timeout exists.
- Backwards compatibility with `ramo.json` or cmux-era workflows. Breaking change, no migration shim.

## Decisions

### 1. Library + user-authored `main.go` (not yaegi, not Go plugin, not CLI script ingest)

Ramo is a Go library. Users write their own `main.go`, import `github.com/<you>/ramo/orchestrator` and `github.com/<you>/ramo/sandbox/docker`, and run it with `go run`.

**Alternatives considered:**
- **yaegi interpreter**: user hands ramo a `.go` file, ramo interprets it. Rejected because yaegi has meaningful stdlib and third-party-import gaps.
- **Go `plugin` package (`.so`)**: brittle — exact Go version + dep graph lock-in between plugin and host. Notoriously painful in practice.
- **CLI binary that ingests a script**: requires inventing our own DSL or embedding a language runtime. Defeats the "it's Go, so the script is Go" pitch.

**Rationale:** Mirrors the [Sandcastle](https://github.com/mattpocock/sandcastle) DX exactly (user writes a TS file, runs with `bun run`). Gives Go users their full toolchain (types, autocomplete, arbitrary imports). Avoids known pain of plugin/interpreter approaches. Cost is tiny: "user runs `go run` instead of `ramo run`."

### 2. Worktree-first, no cross-worktree barriers

Public API is worktree-centric:
```go
Config{
    Worktrees: []Worktree{
        {Branch: "feat/signup", Steps: [...]},
        {Branch: "feat/checkout", Steps: [...]},
    },
}
```
Parallelism = one goroutine per worktree. Each runs its pipeline independently. No `Phases` concept, no `ramo.Barrier()` primitive.

**Alternative considered:** phase-first config (`Phases: []Phase{}`) with implicit barriers between phases.

**Rationale:** User confirmed during design grill that signup's verify step does not depend on checkout's implement step completing. The phase mental model sounds clean but adds cross-worktree coordination complexity (context propagation, failure semantics, timeout interactions) for no real benefit in the target use case.

### 3. Bind-mount the worktree, don't copy in/out

Container mount: `-v /path/to/.worktrees/feat-signup:/workspace`. Agent edits land on the host instantly. Git ops (commit/push/PR) run on the host afterward, reusing the existing worktree directory as source of truth.

**Alternative considered:** copy worktree into container, extract changes afterward via `docker cp` or patch.

**Rationale:** Host-side worktree as source of truth is already the project's central abstraction. Copy-in/copy-out adds round-trip cost per run and complicates mid-run inspection. Bind-mount gives live observability (user can `cd` into the worktree during a run), zero extraction step, and matches git worktree semantics directly.

### 4. Vendored Dockerfile (not a prebuilt image)

`ramo init` writes `.ramo/Dockerfile` into the user's repo with a minimal base (debian-slim + git + curl + node + pnpm + `@anthropic-ai/claude-code`). The user edits it to add their stack-specific runtimes. Ramo builds it on demand, tagged by content hash.

**Alternatives considered:**
- Prebuilt `ramo-sandbox:latest` image on a registry.
- Ship both (prebuilt default + Dockerfile override).

**Rationale:** Sandbox contents are inherently project-specific (stack, runtimes, private tools). A prebuilt "kitchen sink" is either bloated-for-everyone or still needs override. Vendored Dockerfile is transparent (user opens the file and sees exactly what the agent has access to), version-controlled alongside their orchestrator script, and frees ramo from maintaining a registry. The `Image: "..."` escape hatch remains for teams that want prebuilt images from their own registry.

### 5. Ralph Loop = outer Go loop spawning fresh `claude -p` per iteration

Each iteration of a Step is a fresh `docker exec <container> claude -p <prompt>` subprocess. No session resume, no inter-iteration conversation state. The filesystem (the bind-mounted worktree + git state) is the agent's only memory across iterations. Early exit: ramo substring-matches the user-configured `DoneSignal` against the subprocess's captured stdout; if present, break the loop.

**Alternatives considered:**
- Single `claude -p` call with `--max-turns N` (no outer loop).
- Hybrid (outer loop + inner `--max-turns` cap).
- External shell verification (`ExitWhen: "pnpm test"`).

**Rationale:** Matches Sandcastle's verified implementation (`src/Orchestrator.ts`). Fresh sessions give recovery from off-trajectory iterations; inner turn caps don't (one derailed session ruins the run). DoneSignal substring is simpler than external verification, works for any step (no test infrastructure assumed), and puts convergence responsibility in the prompt where the agent has the richest context. External shell verification is a nice-to-have we can add later if needed.

### 6. Recommended user pattern: `tasks.md` + chunked Ralph

The canonical prompt shape instructs the agent to: read `tasks.md`, work up to N related tasks, check them off, emit `DoneSignal` **only if tasks.md has no unchecked items left**. Otherwise stop normally (no signal) — the next iteration reads tasks.md and continues.

**Rationale:** This pattern gives free resumability (kill and re-run picks up where it left off), keeps context fresh per iteration, makes progress visibly auditable (user watches `tasks.md` fill in), and is naturally idempotent when re-running on a partially-done worktree. It's the UX around which Ralph Loop earns its keep; the library should lead with it in documentation and the default scaffolded prompts.

### 7. Host steps as a generalized primitive (no bespoke PR concept)

Every Step has a `RunOn Environment` field — `Container` (default) or `Host`. Host-run steps execute `claude -p` as a subprocess on the host with cwd = worktree path, inheriting host env (`~/.ssh`, `~/.config/gh`, `~/.gitconfig`, `$PATH`). Commit + push + PR creation is a user-authored host step whose prompt uses `gh`/`glab`/whatever. Ramo has zero provider-specific runtime code.

**Alternatives considered:**
- Special `ProducesPR: true` flag on Step.
- Dedicated `OpenPRStep` primitive with Provider enum (`github` | `gitlab`).
- Ramo generates commit message + PR body itself via an internal `claude` call.

**Rationale:** Adding `RunOn` is one field; adding a PR-specific primitive would bolt on a provider abstraction, auth handling, message-generation policies, and N×M coupling between provider and template. "Run a step on the host" is strictly more general — users can use it for any host-side post-processing (changelog updates, release notes, Slack notifications) without ramo growing integrations. Provider survives only as a scaffolding preference on `ramo init --provider`.

### 8. Event-emitter architecture; TUI deferred to v1.1

Orchestrator emits structured `Event` values into a channel. Consumers (`FileLogger`, `SummaryBuilder`, in v1.1 a `TUI`) subscribe independently. Orchestrator doesn't know about consumers; consumers don't know about each other.

**Alternative considered:** Couple the TUI directly to the orchestration loop; ship A+B together in v1.

**Rationale:** A TUI is a 1–2 week effort easy to under-estimate (terminal sizing, colorless fallbacks, cancellation semantics, testability). The event-emitter shape means TUI is additive — deferring loses nothing except launch polish. v1 ships with working orchestration + useful logs + useful report; v1.1 adds the dashboard when the event schema has stabilized.

### 9. Always-resume lifecycle, no configurability

Re-running the same `ramo.go` on an existing worktree always resumes — it uses the worktree as-is, runs the declared pipeline on top. No `OnExisting` enum, no `--wipe` flag. Users who want a clean slate run `ramo remove <branch>` first.

**Alternatives considered:**
- Configurable `OnExisting: Resume | Wipe | Error`.
- Default to Wipe (pristine reruns).

**Rationale:** The chunked-tasks.md pattern is load-bearing for Ralph Loop's value, and that pattern *depends* on filesystem-as-memory being preserved across runs. Wipe as a default would silently destroy `tasks.md` progress. Configurability duplicates what `ramo remove` already does explicitly. Simpler rule + loud startup line ("resuming existing worktree (X files changed)") beats a configurable default.

### 10. Single idle-timeout knob; no wall-clock timeouts

The only hang-detection mechanism is `Config.IdleTimeout` (default 10 min). A timer resets on each stdout line from the claude subprocess; if it fires, ramo SIGTERMs the process and counts the iteration as failed. MaxIterations proceeds if available.

**Alternatives considered:** per-step wall-clock cap, per-iteration cap, per-run cap.

**Rationale:** Wall-clock caps kill legitimately long work on slow machines. Idle timeout targets the only pathology that matters (truly hung subprocess) without penalizing active-but-slow runs. Sandcastle's orchestrator uses the same pattern (idle-reset timer via `Effect.raceFirst`).

### 11. Auth is agnostic: env passthrough, no API-key-specific logic

Ramo passes through a known list of auth env vars to the container (`ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `AWS_*`, etc.), plus whatever the user adds via `Config.Env`. Preflight only *warns* on missing auth; the first `claude` call surfaces the real error. Host steps inherit the full host environment.

**Alternative considered:** Preflight aborts if `ANTHROPIC_API_KEY` is unset.

**Rationale:** User prefers Claude Max subscription auth via `claude setup-token` → `CLAUDE_CODE_OAUTH_TOKEN`. Hardcoding API-key expectations would block that path. Passthrough keeps ramo method-agnostic; users on API key, subscription, Bedrock, Vertex, or corporate proxies all work with zero ramo code changes.

### 12. `--dangerously-skip-permissions` unconditionally, no knob

Every claude invocation (container and host) includes `--dangerously-skip-permissions`. No config to disable.

**Rationale:** Permission prompts block forever in an unattended orchestrator (no human at the terminal to approve). Opt-out is theoretical; in practice any pipeline that needs to work unattended needs this flag. Documenting the tradeoff in the README and not offering a disable knob matches Sandcastle and keeps the API honest.

### 13. Two distinct non-agentic primitives: `Setup` and `Copy`

`Setup []string` — shell commands run in the container before the first Step (every run). `Copy []FileCopy` — host-side file copies run when the worktree is first created (skipped on resume to preserve user edits).

**Alternative considered:** Unify under `Step` with a "shell" vs "agent" flag.

**Rationale:** Forcing `pnpm install` through an agent invocation wastes tokens and time per worktree. `Setup` and `Copy` are mechanical; they shouldn't pay agent overhead. They also have different execution sites (container vs host) and different re-run semantics (always run vs fresh-only), so unifying would need per-item modal flags anyway — cleaner to keep them separate and named.

### 14. Package layout

```
orchestrator/   public API (Config, Worktree, Step, Run, events)
sandbox/        Sandbox interface
sandbox/docker/ docker.New(...) implementation
worktree/       existing git worktree package (cmux stripped)
observability/  file logger + summary consumers
cmd/            cobra commands for init/list/remove
```

**Alternatives considered:** flat single package exporting everything; subpackages under `orchestrator/` (e.g., `orchestrator/sandbox/docker/`).

**Rationale:** Top-level `sandbox/` mirrors the pluggability intent (a future `sandbox/local` or `sandbox/remote` would be peers of `sandbox/docker`, not children of `orchestrator`). Separating `observability/` from `orchestrator/` keeps the consumer pattern visible in imports; users who want a custom consumer (e.g., Slack notifier) import `observability.Consumer` cleanly.

## Risks / Trade-offs

- **[First-run Dockerfile build is slow (~1–3 min)]** → Document the expectation in README and `ramo init` output. Docker's layer cache makes subsequent runs near-instant. This is the cost of vendored-Dockerfile over prebuilt; we've accepted it.
- **[Bind-mount IO on macOS is slower than native FS]** → Fine for text editing (our primary workload). If test suites running inside the container are bottlenecked on IO, users can work around by running tests in `Setup`/`Step` output-mount paths rather than deep in node_modules. Not a v1 optimization target.
- **[`--dangerously-skip-permissions` expands blast radius, especially for host steps]** → Host steps have full host access (by design — that's why they're host steps). Mitigate via: (a) clear README warning, (b) users only write host steps for commit/PR-type tasks, (c) fail-fast pipeline means a broken earlier step stops the host step from running. Accept as inherent.
- **[DoneSignal is prompt-discipline-dependent]** → Agent may forget to emit the signal, or emit it prematurely. Mitigation: (a) default scaffolded prompts include the instruction and are robust, (b) MaxIterations caps wasted work, (c) future knob: "no progress" guard (if two iterations produce zero new `[x]` marks, break early). Out of v1 scope but tracked.
- **[Auth env var list can go stale]** → New auth methods (e.g., future Anthropic-managed credentials) won't be in the default list. Mitigation: users can extend via `Config.Env`; we can add defaults as providers evolve.
- **[Agent may silently skip blocked tasks in `tasks.md`]** → `MaxIterations` would spin without progress. Mitigation: recommend the `<!-- blocked: reason -->` marker convention in scaffolded prompts; consider the "no progress" guard in v1.1.
- **[Container per worktree memory/CPU footprint]** → N worktrees = N containers running dev servers, caches, etc. On low-RAM machines this bites. Mitigation: document; expose `--memory` / `--cpus` knobs on the Docker config in v1.1 if users ask. Not a v1 concern.
- **[Dropping `ramo.json` and cmux is a hard break]** → Anyone using ramo today for its cmux value loses that workflow entirely. Mitigation: README prominently states the pivot; legacy behavior is gone and not restorable.
- **[Testing Docker-dependent code in CI]** → Docker tests need Docker daemon in CI, but `go test ./...` should still work on a fresh laptop. Mitigation: `//go:build integration` tag on docker-touching tests; unit tests use a fake `Sandbox` implementation.

## Migration Plan

This is a net-new library, so "migration" is really: what happens to existing ramo usage?

1. **Delete cmux artifacts.** Remove `cmux/`, `config/`, `cmd/new.go`, `cmd/open.go`, cmux references in `cmd/remove.go`. Remove unused imports.
2. **Refactor `cmd/init.go`** to scaffold the new file tree (`.ramo/Dockerfile`, `ramo.go`, `.ramo/open-pr.prompt.md`, `.ramo/.gitignore`) rather than writing `ramo.json`.
3. **Refactor `cmd/list.go`** and **`cmd/remove.go`** to drop cmux teardown paths. Keep the worktree-side logic.
4. **Preserve `worktree/worktree.go` and `worktree/worktree_test.go` as-is.** They're cmux-free already.
5. **Add `orchestrator/`, `sandbox/`, `sandbox/docker/`, `observability/` packages** with TDD.
6. **Update `go.mod`** to add Docker client dependency.
7. **Update `README.md`** — new identity, new quickstart, clear notice that pre-pivot behavior is gone.
8. **Existing `ramo` users on an older version are not backwards-compatible.** They delete `ramo.json` and re-run `ramo init` for the new world. There is no compatibility shim.

## Open Questions

- **Docker Engine API vs shelling out to `docker` CLI.** The Go SDK is more robust for complex operations (attach, exec streaming) but adds a large dep; shelling out is simpler but stringly-typed. Defer to implementation phase; start with shelling out via `os/exec` and upgrade only if it becomes painful (Sandcastle uses shell-outs and works fine).
- **How much of the existing `promptui` usage survives?** Currently used in `selectWorktree()` for interactive worktree selection in `ramo open` and `ramo remove`. Since `open` is gone and `remove` can accept a branch arg, we may be able to drop `promptui` entirely. Decide during cleanup.
- **Log file rotation / size limits.** Per-worktree-step log files could grow large on long Ralph Loops. v1: accept as-is. v1.1: consider rotating or streaming compaction.
- **Exact format of the markdown summary report.** Draft during implementation; iterate based on what information is actually useful when reviewing a run.
