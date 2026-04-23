// Package sandbox defines the execution-environment abstraction used by the
// orchestrator. Concrete implementations (e.g. sandbox/docker) provide the
// actual container or process isolation.
package sandbox

import (
	"context"
	"io"
)

// StartConfig configures a single Instance start.
type StartConfig struct {
	// Worktree is the human-readable worktree identifier (usually the branch name).
	Worktree string

	// HostPath is the absolute path to the worktree on the host. Docker implementations
	// bind-mount this path into the container at /workspace.
	HostPath string

	// RunID uniquely identifies the current Run invocation. Used to name containers.
	RunID string

	// Env is the set of environment variables to provide to the instance.
	// The sandbox SHALL make these available to every Exec call.
	Env map[string]string
}

// ExecOptions configures a single command execution inside an Instance.
type ExecOptions struct {
	// Cmd is the command and its arguments. Cmd[0] is the executable.
	Cmd []string

	// Dir is the working directory inside the instance. If empty, the instance
	// default is used (/workspace for docker).
	Dir string

	// Env is the set of additional env vars to set for this exec. These are merged
	// with (and override) the instance's Start-time env.
	Env map[string]string

	// Stdin, if non-nil, is piped to the command's stdin.
	Stdin io.Reader

	// Stdout and Stderr receive streamed output. Each is written to line-by-line
	// as output is produced. If nil, output is discarded.
	Stdout io.Writer
	Stderr io.Writer
}

// ExecResult describes a completed command execution.
type ExecResult struct {
	ExitCode int
}

// Sandbox is the factory for Instances.
type Sandbox interface {
	// Start provisions an instance for the given worktree.
	// The returned Instance is alive until Stop is called.
	Start(ctx context.Context, cfg StartConfig) (Instance, error)
}

// Instance is a live execution environment.
type Instance interface {
	// Exec runs the command inside the instance. It blocks until the command
	// exits (successfully or otherwise) or the context is cancelled.
	// Non-zero ExitCode is returned in ExecResult; only I/O or lifecycle
	// errors return a non-nil error.
	Exec(ctx context.Context, opts ExecOptions) (ExecResult, error)

	// Stop tears the instance down. Repeated calls are safe; subsequent calls
	// after the first SHALL be no-ops.
	Stop(ctx context.Context) error
}
