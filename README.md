# Ramo

<p align="center">
  <img src=".github/assets/logo.png" alt="Ramo" width="200" />
</p>

**Ramo** is a Go library for orchestrating parallel AI coding agents across isolated git worktrees. You declare N worktrees, each with a pipeline of steps (implement → verify → open PR), and `go run ./ramo.go` spawns one Docker container per worktree, runs the pipelines in parallel, and drops a draft PR from every one that succeeds.

Design inspired by [Sandcastle](https://github.com/mattpocock/sandcastle). Ramo's core loop mirrors Sandcastle's Ralph Loop pattern — fresh `claude -p` subprocess per iteration, early-exit on a done-signal substring match — adapted to Go and specialized for git-worktree orchestration.

## Prerequisites

- [Go](https://go.dev/) 1.26+
- [Git](https://git-scm.com/)
- [Docker](https://www.docker.com/) (for container steps)
- [Claude Code](https://github.com/anthropics/claude-code) CLI on host
- A code host CLI on the host if you want auto-PR: [gh](https://cli.github.com/) or [glab](https://gitlab.com/gitlab-org/cli)

## Installation

```
go install github.com/mateusduraes/ramo@latest
```

This installs the `ramo` binary, which is used only for scaffolding and worktree maintenance:

| Command | Description |
|---------|-------------|
| `ramo init [--provider github\|gitlab]` | Scaffold `.ramo/Dockerfile`, `ramo.go`, `.ramo/open-pr.prompt.md`, `.ramo/.gitignore` |
| `ramo list` | List ramo-managed worktrees |
| `ramo remove <branch>` | Remove a worktree and its branch |

The actual orchestration is a Go library you invoke from your own `ramo.go`.

## Quick Start

From your repo root:

```
ramo init
```

This writes:

| File | Purpose |
|------|---------|
| `.ramo/Dockerfile` | Sandbox image. Edit to add your stack's runtimes |
| `.ramo/implement.prompt.md` | Starter "implement tasks from tasks.md" prompt for container steps |
| `.ramo/open-pr.prompt.md` | Host-step prompt that commits + pushes + opens a draft PR |
| `.ramo/.gitignore` | Keeps `logs/` and `reports/` out of git |
| `ramo.go` | Starter orchestrator script |

Edit `ramo.go` to declare your worktrees and steps, then:

```
go run ./ramo.go
```

Each worktree runs in parallel. Each step spawns a fresh `claude -p` subprocess per iteration with `--dangerously-skip-permissions`, checks for the step's `DoneSignal` substring, and stops as soon as it appears (or when `MaxIterations` is reached).

## First run walkthrough

End-to-end smoke test against a real repo. Assumes Docker is running, `claude` is on your PATH, and at least one auth env var (`ANTHROPIC_API_KEY` or `CLAUDE_CODE_OAUTH_TOKEN`) is set.

1. **Scaffold.**
   ```
   cd ~/code/myproject
   ramo init
   git add .ramo ramo.go && git commit -m "chore: add ramo"
   ```

2. **Pick one worktree and one branch name you want the agent to work on.** Open `ramo.go` and replace the `feat/example` worktree with something real for your project. Example:
   ```go
   Worktrees: []orchestrator.Worktree{
       {
           Branch: "feat/add-healthcheck",
           Setup:  []string{"pnpm install"},  // or your stack's equivalent
           Steps: []orchestrator.Step{
               {
                   Name:          "implement",
                   PromptFile:    ".ramo/implement.prompt.md",
                   MaxIterations: 10,
                   DoneSignal:    "<ramo>DONE</ramo>",
               },
               {
                   Name:       "open-pr",
                   PromptFile: ".ramo/open-pr.prompt.md",
                   RunOn:      orchestrator.Host,
               },
           },
       },
   },
   ```

3. **Write a `tasks.md` on the target branch.** Ramo creates the worktree on first run and `implement.prompt.md` expects `tasks.md` to exist inside it. Easiest flow:
   ```
   git worktree add .worktrees/feat/add-healthcheck -b feat/add-healthcheck
   cd .worktrees/feat/add-healthcheck
   cat > tasks.md <<'EOF'
   - [ ] Add a GET /healthz handler that returns 200 with body "ok"
   - [ ] Add a unit test for the new handler
   - [ ] Wire the handler into the main router
   EOF
   git add tasks.md && git commit -m "chore: seed tasks.md"
   cd ../..
   ```
   (Ramo will reuse this worktree on its first run instead of creating it fresh. That's the "always resume" behavior.)

4. **Run it.**
   ```
   go run ./ramo.go
   ```
   Stdout shows one line per significant event: step started, iteration N, done-signal matched, step finished, sandbox stopped. Raw agent output does not stream to stdout — look in the log files for that.

5. **Inspect the artifacts.**
   - `.ramo/logs/<runID>/feat-add-healthcheck/implement.log` — full stdout/stderr of every iteration, with iteration-boundary markers.
   - `.ramo/logs/<runID>/feat-add-healthcheck/open-pr.log` — the host step's log.
   - `.ramo/reports/run-<runID>.md` — markdown summary: per-step status, iteration count, whether `DoneSignal` fired, number of files changed.
   - The worktree itself (`.worktrees/feat/add-healthcheck/`) — browse what the agent did; `tasks.md` should have `[x]` items.

6. **Clean up when you're done reviewing.**
   ```
   ramo remove feat/add-healthcheck
   ```

## Testing

This repo ships unit tests for every package:

```
go test ./...
```

Runs in ~5 seconds on a fresh checkout and requires no Docker daemon. Covers:

- `observability/` — event emitter fan-out, file logger, summary builder, stdout consumer.
- `sandbox/fake/` — the in-memory sandbox used by orchestrator tests.
- `sandbox/docker/` — pure helpers (image-tag hashing, container naming, config validation). Shell-out paths are deferred to the integration suite.
- `orchestrator/` — Ralph Loop, DoneSignal early exit, MaxIterations cap, preflight, Setup, Copy, parallel worktrees, fail-fast, idle timeout, cancellation, auth env passthrough, host-step dispatch. Uses real git worktrees against temp dirs and the fake sandbox.
- `cmd/` — `init`, `list`, `remove` end-to-end against temp dirs.
- `worktree/` — the existing git worktree helpers plus a static check that keeps the package cmux-free.

The integration suite (`//go:build integration`) is not yet written. When it exists, `go test -tags=integration ./...` will cover the real Docker lifecycle (build → start → exec → stop) against your local daemon.

## The Ralph Loop

Every step is a loop: up to `MaxIterations` fresh `claude -p` subprocesses. There is no session state between iterations — the only memory is the filesystem (the bind-mounted worktree + its git state).

The recommended shape of a step prompt is: "read `tasks.md`, work up to N related tasks, check them off, emit `DoneSignal` only if tasks.md has no unchecked items left." That gives you free resumability (kill and re-run picks up where it left off), keeps context fresh per iteration, and makes progress visibly auditable. See `docs/ralph-loop.md` for details.

## Authentication

Ramo is auth-method-agnostic. It passes through these host env vars (if set) to every container:

- `ANTHROPIC_API_KEY`
- `CLAUDE_CODE_OAUTH_TOKEN` (Claude subscription; generate with `claude setup-token`)
- `ANTHROPIC_BASE_URL`
- `AWS_REGION`, `AWS_PROFILE`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` (Bedrock)

Add more with `Config.Env: []string{"MY_VAR"}`. Host steps inherit the full host environment. See `docs/auth.md`.

## Safety

Every `claude -p` invocation includes `--dangerously-skip-permissions`. There is no opt-out.

Permission prompts block forever in an unattended orchestrator (no human at the terminal to approve). Opt-out is theoretical; in practice, any pipeline that needs to run unattended needs this flag. That also means:

- Container steps have full access to the bind-mounted worktree and whatever runtimes are in the Dockerfile.
- Host steps have full access to the host — your shell, your credentials, everything your user can do.
- Keep host steps narrow (commit, push, open PR) and host-step prompts auditable in git.

## License

MIT
