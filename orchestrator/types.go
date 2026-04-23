package orchestrator

import (
	"time"

	"github.com/mateusduraes/ramo/observability"
	"github.com/mateusduraes/ramo/sandbox"
)

// Environment selects where a step's claude subprocess runs.
type Environment string

const (
	// Container runs the step inside the sandbox's container. This is the default.
	Container Environment = "container"
	// Host runs the step directly on the host, cwd set to the worktree path,
	// inheriting the full host environment. Use for git/PR operations.
	Host Environment = "host"
)

// DefaultIdleTimeout is the default per-iteration idle timeout: if no stdout
// is produced within this duration, the iteration is SIGTERMed.
const DefaultIdleTimeout = 10 * time.Minute

// Config is the top-level orchestrator configuration.
type Config struct {
	// RepoDir is the main repository root. Defaults to the current working directory.
	RepoDir string

	// WorktreesDir is where git worktrees live. Defaults to RepoDir/.worktrees.
	WorktreesDir string

	// Sandbox is used for container-run steps. Required if any step RunOn == Container.
	Sandbox sandbox.Sandbox

	// Worktrees is the set of per-worktree pipelines to execute in parallel.
	Worktrees []Worktree

	// Env is an additional list of env var names to pass from host to container.
	// These are merged with the package's built-in auth env list.
	Env []string

	// IdleTimeout is the per-iteration idle timeout. Defaults to DefaultIdleTimeout.
	IdleTimeout time.Duration

	// Consumers are observability sinks. A nil/empty list is valid: no sinks.
	Consumers []observability.Consumer

	// RunID identifies this run. Auto-generated if empty.
	RunID string

	// ClaudeBinary overrides the name/path of the claude CLI invoked on the host.
	// Defaults to "claude". Container steps always invoke "claude" inside the image.
	ClaudeBinary string

	// HostExecutor executes Host-run steps. Injectable for tests; defaults to a
	// real os/exec-backed runner.
	HostExecutor HostExecutor

	// SkipPreflightEnv disables preflight checks for external binaries (claude,
	// docker daemon). File-existence preflight (prompt files, copy sources) still
	// runs. Intended for tests; production callers should leave this false.
	SkipPreflightEnv bool
}

// Worktree declares a single worktree's pipeline.
type Worktree struct {
	// Branch is the git branch this worktree tracks; it is also the worktree's identifier.
	Branch string

	// Setup is a list of shell commands executed in the container (in order) on every run.
	Setup []string

	// Copy is a list of host file copies applied only on fresh-worktree creation.
	Copy []FileCopy

	// Steps is the pipeline of steps executed in order. Fail-fast on first failure.
	Steps []Step
}

// FileCopy describes a single host-side file copy applied to a fresh worktree.
type FileCopy struct {
	// From is the source path, relative to the main repo root (or absolute).
	From string
	// To is the destination path, relative to the worktree root (or absolute).
	To string
}

// Step is one stage in a worktree's pipeline.
type Step struct {
	// Name identifies the step. Used for logs and events.
	Name string

	// PromptFile is the path to the prompt file passed to `claude -p` via stdin.
	// May be relative to RepoDir or absolute.
	PromptFile string

	// RunOn selects Container (default) or Host execution.
	RunOn Environment

	// MaxIterations caps the number of claude invocations. Defaults to 1.
	MaxIterations int

	// DoneSignal, if non-empty, causes the step to exit early when this
	// substring appears in an iteration's stdout.
	DoneSignal string
}

// normalizeConfig fills in defaults on a Config value. Returns a copy.
func normalizeConfig(cfg Config) Config {
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}
	for i := range cfg.Worktrees {
		for j := range cfg.Worktrees[i].Steps {
			s := &cfg.Worktrees[i].Steps[j]
			if s.MaxIterations <= 0 {
				s.MaxIterations = 1
			}
			if s.RunOn == "" {
				s.RunOn = Container
			}
		}
	}
	return cfg
}
