package observability

import "time"

type EventKind string

const (
	KindSandboxStarted    EventKind = "sandbox_started"
	KindSandboxStopped    EventKind = "sandbox_stopped"
	KindSetupStarted      EventKind = "setup_started"
	KindSetupFinished     EventKind = "setup_finished"
	KindStepStarted       EventKind = "step_started"
	KindStepFinished      EventKind = "step_finished"
	KindStepFailed        EventKind = "step_failed"
	KindStepSkipped       EventKind = "step_skipped"
	KindIterationStarted  EventKind = "iteration_started"
	KindIterationFinished EventKind = "iteration_finished"
	KindDoneSignalFound   EventKind = "done_signal_found"
	KindIdleTimeout       EventKind = "idle_timeout"
	KindCancelled         EventKind = "cancelled"
	KindPreflightWarning  EventKind = "preflight_warning"
	KindResumeWorktree    EventKind = "resuming_existing_worktree"
	KindCreateWorktree    EventKind = "creating_new_worktree"
	KindWorktreeFailed    EventKind = "worktree_failed"
	KindWorktreeSucceeded EventKind = "worktree_succeeded"
	KindStdoutLine        EventKind = "stdout_line"
	KindStderrLine        EventKind = "stderr_line"
	KindRunStarted        EventKind = "run_started"
	KindRunFinished       EventKind = "run_finished"
)

type Event struct {
	Time     time.Time      `json:"time"`
	RunID    string         `json:"run_id,omitempty"`
	Worktree string         `json:"worktree,omitempty"`
	Step     string         `json:"step,omitempty"`
	Kind     EventKind      `json:"kind"`
	Payload  map[string]any `json:"payload,omitempty"`
}
