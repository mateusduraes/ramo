# Orchestrator Specification

### Requirement: Library entrypoint

The orchestrator SHALL expose a `Run(ctx context.Context, cfg Config) error` function as its public entrypoint. The `Config` value SHALL declare the sandbox, one or more worktrees with their pipelines, and optional run-level settings (base branch, idle timeout, additional env-var passthrough).

#### Scenario: Single-worktree run completes successfully
- **WHEN** a user calls `orchestrator.Run(ctx, Config{Sandbox: mockSandbox, Worktrees: []Worktree{{Branch: "feat/x", Steps: [...]}}})`
- **THEN** the orchestrator creates or resumes the worktree, executes the declared steps in order, and returns a nil error

#### Scenario: Multi-worktree run executes pipelines in parallel
- **WHEN** `Run` is called with three worktrees, each declaring identical steps
- **THEN** the orchestrator executes the three pipelines concurrently (e.g. each begins its first step without waiting for the others) and returns after all pipelines complete or fail

### Requirement: Step execution in declared order within a worktree

Within a single worktree, the orchestrator SHALL execute steps sequentially in the order they appear in `Worktree.Steps`. A step SHALL NOT start until the prior step in the same worktree has finished.

#### Scenario: Steps run in order
- **WHEN** a worktree declares steps `[implement, verify, commit-and-pr]`
- **THEN** the orchestrator runs `implement` to completion, then `verify` to completion, then `commit-and-pr` to completion

### Requirement: No cross-worktree barriers

The orchestrator SHALL NOT synchronize step boundaries across worktrees. One worktree's progression to its next step SHALL NOT depend on any other worktree's state.

#### Scenario: Fast worktree does not wait for slow worktree
- **WHEN** worktree A's `implement` step completes in 30 seconds and worktree B's `implement` step is still running after 5 minutes
- **THEN** worktree A proceeds to its `verify` step immediately, without waiting for worktree B

### Requirement: Fail-fast per worktree

When any step in a worktree fails, the orchestrator SHALL skip all remaining steps in that worktree and record the worktree as failed. Other worktrees SHALL continue undisturbed. A step is "failed" when (a) its claude subprocess exits non-zero, OR (b) `MaxIterations` is reached with `DoneSignal` configured and no substring match, OR (c) a `Setup` shell command exits non-zero, OR (d) a declared `Copy` source file is missing.

#### Scenario: Implement step fails, verify and commit-and-pr are skipped
- **GIVEN** a worktree with steps `[implement, verify, commit-and-pr]`
- **WHEN** the `implement` step's claude subprocess exits with code 1
- **THEN** `verify` and `commit-and-pr` are not executed, and the worktree's final status is recorded as failed at `implement`

#### Scenario: One worktree's failure does not affect others
- **GIVEN** worktrees A, B, C running concurrently
- **WHEN** worktree B's first step fails
- **THEN** worktrees A and C continue to run their pipelines to completion independently

### Requirement: Ralph Loop — fresh subprocess per iteration

For each step with `RunOn: Container`, the orchestrator SHALL spawn a fresh `claude -p <prompt>` subprocess via `docker exec` on each iteration. For each step with `RunOn: Host`, the orchestrator SHALL spawn a fresh `claude -p <prompt>` subprocess on the host with cwd set to the worktree path. Successive iterations within the same step SHALL NOT share conversation state; the filesystem and git state are the only memory between iterations.

#### Scenario: Two iterations produce two separate subprocesses
- **GIVEN** a step with `MaxIterations: 2` and a DoneSignal that the agent never emits
- **WHEN** the step executes
- **THEN** ramo launches two distinct `claude` subprocesses in sequence, each with a fresh session, and records two iterations in the event stream

### Requirement: DoneSignal substring-match early exit

When a step's `DoneSignal` is non-empty, the orchestrator SHALL capture stdout from each iteration's claude subprocess and check whether the captured output contains the `DoneSignal` substring. If the substring is present, the orchestrator SHALL treat the step as successfully converged and SHALL NOT execute further iterations for that step.

#### Scenario: DoneSignal found on second iteration — third iteration is skipped
- **GIVEN** a step with `MaxIterations: 5` and `DoneSignal: "<ramo>DONE</ramo>"`
- **WHEN** the first iteration's stdout does not contain the signal and the second iteration's stdout does
- **THEN** the step is recorded as successful, and iterations 3, 4, 5 are not executed

### Requirement: MaxIterations defaults to 1 when DoneSignal is absent

A step without a configured `DoneSignal` SHALL run exactly `MaxIterations` times (defaulting to 1 if unset), with all iterations treated as success by definition. The orchestrator SHALL NOT attempt substring-matching when `DoneSignal` is empty.

#### Scenario: Single-shot step runs once without signal checking
- **GIVEN** a step with no `DoneSignal` and no explicit `MaxIterations`
- **WHEN** the step executes
- **THEN** ramo runs exactly one `claude -p` subprocess and marks the step successful on that subprocess's clean exit

### Requirement: Setup primitive — container shell commands, every run

For each worktree, after the sandbox is started and before the first step runs, the orchestrator SHALL execute each command in `Worktree.Setup` inside the container, in order. Setup SHALL run on every invocation of `Run`, including resumes. Any non-zero exit SHALL be treated as a worktree failure per the fail-fast rule.

#### Scenario: Setup runs on resume
- **GIVEN** an existing worktree with `Setup: ["pnpm install"]`
- **WHEN** `Run` is called for the second time against the same worktree
- **THEN** `pnpm install` is executed inside the container before the first step runs

### Requirement: Copy primitive — host file copies, fresh only

For each worktree, on first creation (worktree did not previously exist), the orchestrator SHALL copy each declared `Copy` entry from its source (relative to the main repo root) to its destination (relative to the worktree root) on the host, before starting the sandbox. On resume (worktree already exists), Copy SHALL be skipped to preserve any user edits. A missing source path SHALL cause the worktree to fail with a clear error before any container starts.

#### Scenario: Copy runs on fresh worktree creation
- **GIVEN** a worktree declaring `Copy: [{From: "../.env.local", To: ".env.local"}]` that does not yet exist on disk
- **WHEN** `Run` is called
- **THEN** `../.env.local` is copied into the new worktree at `.env.local` before the container starts

#### Scenario: Copy is skipped on resume
- **GIVEN** an existing worktree with a `.env.local` that the user has edited, and `Copy` still declared
- **WHEN** `Run` is called again
- **THEN** the user's `.env.local` is left unchanged and the declared source file is not re-copied

### Requirement: Always-resume lifecycle

When `Run` is called and a target worktree already exists on disk, the orchestrator SHALL reuse it as-is (no wipe, no error) and proceed to run the declared pipeline. When only the branch exists (no worktree), the orchestrator SHALL check out the branch into a new worktree. When neither exists, the orchestrator SHALL create both. There SHALL be no configuration knob to alter this behavior.

#### Scenario: Resume reuses existing worktree
- **GIVEN** a worktree `feat/signup` that already exists on disk with uncommitted changes
- **WHEN** `Run` is called with the same branch name
- **THEN** the existing worktree is used as-is, uncommitted changes are preserved, and the pipeline runs on top

### Requirement: Unconditional `--dangerously-skip-permissions`

Every `claude -p` subprocess the orchestrator spawns SHALL include the `--dangerously-skip-permissions` flag. There SHALL NOT be a configuration knob to disable this flag.

#### Scenario: Container step invocation includes the flag
- **WHEN** the orchestrator executes any step with `RunOn: Container`
- **THEN** the constructed command line contains `--dangerously-skip-permissions`

#### Scenario: Host step invocation includes the flag
- **WHEN** the orchestrator executes any step with `RunOn: Host`
- **THEN** the constructed command line contains `--dangerously-skip-permissions`

### Requirement: Idle timeout as only hang-detection mechanism

The orchestrator SHALL enforce a per-iteration idle timeout (default 10 minutes, configurable via `Config.IdleTimeout`). If no stdout line is received from a claude subprocess for the configured duration, the orchestrator SHALL send SIGTERM to that subprocess (SIGKILL after 5 seconds if still running), record the iteration as failed with reason "idle timeout," and proceed to the next iteration if `MaxIterations` allows. The orchestrator SHALL NOT enforce any wall-clock timeout at the step, worktree, or run level.

#### Scenario: Idle claude subprocess is terminated and iteration fails
- **GIVEN** a step whose claude subprocess produces no stdout for longer than the configured idle timeout
- **WHEN** the idle timer fires
- **THEN** the subprocess is terminated and the iteration is recorded as failed

### Requirement: Preflight checks before launching any worktree

Before starting any sandbox or spawning any claude subprocess, the orchestrator SHALL verify: (a) the Docker daemon is reachable, (b) the `claude` CLI is available on the host, (c) the Dockerfile builds successfully or the override image is pullable, (d) every step's `PromptFile` exists, and (e) every `Copy.From` source path exists for worktrees that will be created fresh. If any of (a)–(e) fails, the orchestrator SHALL abort with a clear error message before any container is started. The orchestrator SHALL also check for known auth env vars (`ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN`, AWS credentials, etc.) and emit a warning if none are set, but SHALL NOT abort the run for this reason.

#### Scenario: Missing prompt file aborts run before any container starts
- **GIVEN** a Step declaring `PromptFile: "./missing.md"` that does not exist
- **WHEN** `Run` is called
- **THEN** the orchestrator returns an error naming the missing file and no container is created

#### Scenario: Missing auth vars produces warning but run continues
- **GIVEN** no `ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN`, or AWS credentials in the environment
- **WHEN** `Run` is called
- **THEN** a warning is emitted to the event stream and the run proceeds to execute the pipeline

### Requirement: Graceful cancellation on SIGINT

When the process receives SIGINT (Ctrl-C), the orchestrator SHALL cancel the run context, SIGTERM all in-flight claude subprocesses (with 5-second grace then SIGKILL), tear down every sandbox with `docker rm -f`, write a partial summary report reflecting "cancelled" status for in-progress steps and "not run" for subsequent steps, and return a non-zero error. A second SIGINT received during shutdown SHALL cause immediate process exit, skipping further cleanup.

#### Scenario: First Ctrl-C performs graceful teardown
- **GIVEN** a run in progress with running containers and active claude subprocesses
- **WHEN** the user sends SIGINT
- **THEN** all claude subprocesses are terminated, all containers are removed, a partial report is written, and `Run` returns a non-nil error

### Requirement: Auth env var passthrough to containers

The orchestrator SHALL pass through to every container the values of these host env vars (when set): `ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `AWS_REGION`, `AWS_PROFILE`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`. Additional env var names declared in `Config.Env` SHALL also be passed through.

#### Scenario: OAuth token passthrough
- **GIVEN** `CLAUDE_CODE_OAUTH_TOKEN=abc123` set in the host environment and `ANTHROPIC_API_KEY` unset
- **WHEN** a container step executes
- **THEN** the claude subprocess inside the container sees `CLAUDE_CODE_OAUTH_TOKEN=abc123`

#### Scenario: User-declared env passthrough
- **GIVEN** `Config.Env: ["COMPANY_PROXY"]` and `COMPANY_PROXY=http://proxy` in the host env
- **WHEN** a container step executes
- **THEN** the claude subprocess inside the container sees `COMPANY_PROXY=http://proxy`
