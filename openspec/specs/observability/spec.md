# Observability Specification

### Requirement: Structured event stream

The orchestrator SHALL emit structured `Event` values describing every significant state transition during a run: sandbox started/stopped, step started/finished, iteration started/finished, DoneSignal found, idle timeout fired, setup command ran, pipeline cancelled, preflight warning, resume-existing-worktree notice. Each event SHALL carry a timestamp, a worktree identifier, a step name (when applicable), and a typed payload. Events SHALL be delivered to subscribed consumers through a channel-based mechanism that does not block the orchestrator's execution.

#### Scenario: Step lifecycle events are emitted in order
- **GIVEN** a single-worktree run with one step
- **WHEN** the pipeline executes
- **THEN** subscribed consumers receive events in the order: `sandbox_started`, `setup_started` (if any), `setup_finished`, `step_started`, `iteration_started`, `iteration_finished`, `step_finished`, `sandbox_stopped`

### Requirement: Pluggable Consumer interface

The observability layer SHALL define a `Consumer` interface that subscribes to the event stream. The orchestrator SHALL accept zero or more consumers via `Config` and SHALL deliver every emitted event to every subscribed consumer. Consumers SHALL run independently of one another; a slow or failing consumer SHALL NOT block or corrupt other consumers or the orchestrator.

#### Scenario: Multiple consumers receive the same events
- **GIVEN** two consumers A and B registered with the orchestrator
- **WHEN** an event is emitted
- **THEN** both A and B receive the event independently

### Requirement: FileLogger writes per-worktree-step log files

The `FileLogger` consumer SHALL write one log file per (worktree, step) pair at `.ramo/logs/<runID>/<worktree-slug>/<step-name>.log`. The log file SHALL contain the full stdout and stderr of the step's claude subprocess(es), including iteration boundaries if the step looped. The `runID` SHALL be a unique identifier generated at the start of each `Run` invocation.

#### Scenario: Each step produces its own log file
- **GIVEN** a worktree `feat/signup` with steps `[implement, verify]`
- **WHEN** the pipeline executes
- **THEN** two files exist: `.ramo/logs/<runID>/feat-signup/implement.log` and `.ramo/logs/<runID>/feat-signup/verify.log`

#### Scenario: Iteration output is preserved in the log
- **GIVEN** a step that ran two Ralph Loop iterations
- **WHEN** the pipeline completes
- **THEN** the step's log file contains stdout from both iterations with clear iteration boundary markers

### Requirement: SummaryBuilder writes a markdown report per run

The `SummaryBuilder` consumer SHALL write a single markdown report at `.ramo/reports/run-<runID>.md` summarizing the run. The report SHALL contain, for each worktree: branch name, overall status (succeeded / failed / cancelled), per-step status with iteration count used and whether DoneSignal fired, number of files changed, and the path to the log directory. If the run is cancelled or ends with failures, the report SHALL still be written with partial information reflecting the actual final state.

#### Scenario: Report is written on successful run
- **GIVEN** a completed run with two worktrees
- **WHEN** `Run` returns
- **THEN** `.ramo/reports/run-<runID>.md` exists and contains a section for each worktree with branch, status, and step details

#### Scenario: Report is written on cancelled run
- **GIVEN** a run cancelled via SIGINT mid-pipeline
- **WHEN** `Run` returns
- **THEN** `.ramo/reports/run-<runID>.md` exists and contains, for the in-flight worktrees, a status of "cancelled" on the in-progress step and "not run" on subsequent steps

### Requirement: Stdout consumer emits one-line-per-event minimal output

A default `Stdout` consumer SHALL be registered when stdout is a TTY. It SHALL emit one human-readable line per significant event (step started/finished, iteration started, DoneSignal found, failure, resume notice). It SHALL NOT stream raw agent output.

#### Scenario: Minimal stdout includes step transitions
- **GIVEN** a run with a single worktree executing `[implement, verify]`
- **WHEN** the pipeline executes
- **THEN** stdout contains lines tagged with the worktree identifier for at least: step started, step finished, per iteration

### Requirement: Loud startup line when resuming a worktree

When the orchestrator starts a worktree that already exists on disk, it SHALL emit a visible event and the corresponding stdout/log line making the resume explicit (e.g. `[feat/signup] resuming existing worktree (12 files changed, last commit: 2 days ago)`). Fresh worktree creation SHALL also emit a corresponding line (e.g. `[feat/signup] creating new worktree`).

#### Scenario: Resume is announced
- **GIVEN** an existing worktree `feat/signup` with uncommitted changes
- **WHEN** `Run` is called against it
- **THEN** the event stream contains a resume event for that worktree and the stdout consumer prints a resume line referencing the branch
