# The Ralph Loop

Ramo's core execution pattern. Named after [the pattern](https://github.com/mattpocock/sandcastle) that Sandcastle popularized: iterate on a task in fresh sessions until it's done.

## What it is

A Ralph Loop step runs up to `MaxIterations` fresh `claude -p` subprocesses. Each iteration:

1. Spawns a new subprocess — no session resume, no inter-iteration conversation state.
2. Reads its prompt from stdin (`claude -p -`) with `--dangerously-skip-permissions`.
3. Produces output on stdout. Ramo captures it and checks for the step's `DoneSignal` substring.
4. If `DoneSignal` is found, the step exits early and success. Otherwise ramo launches the next iteration.

The only memory between iterations is **the filesystem** — the bind-mounted worktree, its git state, any files the agent writes. This forces the agent to externalize progress.

## The canonical pattern: `tasks.md` + chunked Ralph

Structure the prompt so the agent reads a task file, works up to N items, and checks them off:

```markdown
You are working on this worktree. Read `tasks.md` first.

Pick the next 1–3 uncompleted tasks (lines starting with `- [ ]`). Complete them.
Use `- [x]` to mark completed tasks. Add a `<!-- blocked: reason -->` comment on
tasks you cannot complete.

When you are done with this iteration, verify that tasks.md still parses cleanly
and that all tests still pass.

When — and only when — `tasks.md` has no `- [ ]` items remaining, emit this
exact line as the last thing you do:

<ramo>DONE</ramo>

If there are still unchecked items, do NOT emit the done signal. Just stop
normally.
```

Paired with `DoneSignal: "<ramo>DONE</ramo>"` on the step, this gives you:

- **Free resumability.** Kill the run, re-run it; the next iteration reads `tasks.md` and continues where it left off.
- **Progress visibility.** `tasks.md` fills up as the agent works; you can watch from another terminal.
- **Fresh context.** Each iteration starts with no conversation baggage — a derailed iteration doesn't ruin the run.
- **Idempotent re-runs.** Worktrees are always resumed; running `go run ./ramo.go` twice does not redo completed work.

## When it works, when it doesn't

**Works well** when the task decomposes into mostly independent subtasks. "Add logging to 12 handlers" → 12 checklist items → 3–4 iterations each touching a chunk.

**Struggles** when subtasks are tightly coupled or require deep cross-file reasoning in a single shot. The fresh-session reset means the agent rebuilds context every iteration; if the task needs a long continuous chain of reasoning, you may want a single iteration with a larger, more guided prompt.

## Configuration knobs

| Field | Default | Purpose |
|-------|---------|---------|
| `MaxIterations` | 1 | Hard cap on fresh claude invocations per step |
| `DoneSignal` | `""` | Substring that triggers early exit. Empty = run exactly `MaxIterations` times, no early exit |
| `IdleTimeout` (Config-level) | 10m | If no stdout for this duration, SIGTERM the subprocess |

With `DoneSignal == ""` the step is a single-shot invocation after `MaxIterations` runs. With `DoneSignal != ""` the step is a loop that can exit early.
