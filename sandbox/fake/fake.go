// Package fake provides an in-memory Sandbox implementation used by orchestrator
// unit tests. It records Start and Exec invocations and lets tests script the
// behavior of Exec calls (exit code, stdout/stderr output, timing).
package fake

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/mateusduraes/ramo/sandbox"
)

// ExecCall captures a single Exec invocation for later inspection by tests.
type ExecCall struct {
	Worktree string
	Cmd      []string
	Dir      string
	Env      map[string]string
	Stdin    string
}

// ExecScript describes how the fake should respond to a single Exec call.
// Tests enqueue one ExecScript per expected call (or pass a single default via SetDefault).
type ExecScript struct {
	// Stdout is delivered line-by-line to opts.Stdout. Empty string = no output.
	Stdout string
	// Stderr is delivered line-by-line to opts.Stderr.
	Stderr string
	// ExitCode is the returned process exit code.
	ExitCode int
	// Delay, if non-zero, is slept before the call returns (after streaming output).
	Delay time.Duration
	// PerLineDelay, if non-zero, is slept between each stdout/stderr line.
	PerLineDelay time.Duration
	// Err, if non-nil, is returned from Exec in place of a normal result.
	Err error
}

// Sandbox is a fake sandbox for tests. Safe for concurrent use.
type Sandbox struct {
	mu       sync.Mutex
	started  []sandbox.StartConfig
	scripts  map[string][]ExecScript // keyed by worktree; consumed in order
	defaults map[string]ExecScript   // fallback keyed by worktree ("" = global default)
	execs    []ExecCall
}

// New returns a ready-to-use fake Sandbox.
func New() *Sandbox {
	return &Sandbox{
		scripts:  map[string][]ExecScript{},
		defaults: map[string]ExecScript{},
	}
}

// QueueExec appends a scripted response for the named worktree. The next Exec
// against that worktree will return it.
func (s *Sandbox) QueueExec(worktree string, script ExecScript) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scripts[worktree] = append(s.scripts[worktree], script)
}

// SetDefault configures the default scripted response used when the per-worktree
// queue is empty. An empty worktree string makes the default apply to all worktrees.
func (s *Sandbox) SetDefault(worktree string, script ExecScript) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaults[worktree] = script
}

// Calls returns a snapshot of all recorded Exec invocations.
func (s *Sandbox) Calls() []ExecCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ExecCall(nil), s.execs...)
}

// Starts returns a snapshot of every StartConfig the sandbox has seen.
func (s *Sandbox) Starts() []sandbox.StartConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sandbox.StartConfig(nil), s.started...)
}

func (s *Sandbox) Start(_ context.Context, cfg sandbox.StartConfig) (sandbox.Instance, error) {
	s.mu.Lock()
	s.started = append(s.started, cfg)
	s.mu.Unlock()
	return &instance{sandbox: s, worktree: cfg.Worktree}, nil
}

type instance struct {
	sandbox  *Sandbox
	worktree string

	mu      sync.Mutex
	stopped bool
}

func (i *instance) Stop(_ context.Context) error {
	i.mu.Lock()
	i.stopped = true
	i.mu.Unlock()
	return nil
}

func (i *instance) Exec(ctx context.Context, opts sandbox.ExecOptions) (sandbox.ExecResult, error) {
	stdin := ""
	if opts.Stdin != nil {
		data, err := io.ReadAll(opts.Stdin)
		if err != nil {
			return sandbox.ExecResult{}, err
		}
		stdin = string(data)
	}

	call := ExecCall{
		Worktree: i.worktree,
		Cmd:      append([]string(nil), opts.Cmd...),
		Dir:      opts.Dir,
		Env:      copyMap(opts.Env),
		Stdin:    stdin,
	}

	script := i.sandbox.nextScript(i.worktree)
	i.sandbox.recordCall(call)

	if script.Err != nil {
		return sandbox.ExecResult{}, script.Err
	}

	if err := streamLines(ctx, script.Stdout, opts.Stdout, script.PerLineDelay); err != nil {
		return sandbox.ExecResult{}, err
	}
	if err := streamLines(ctx, script.Stderr, opts.Stderr, script.PerLineDelay); err != nil {
		return sandbox.ExecResult{}, err
	}

	if script.Delay > 0 {
		select {
		case <-time.After(script.Delay):
		case <-ctx.Done():
			return sandbox.ExecResult{}, ctx.Err()
		}
	}

	return sandbox.ExecResult{ExitCode: script.ExitCode}, nil
}

func (s *Sandbox) nextScript(worktree string) ExecScript {
	s.mu.Lock()
	defer s.mu.Unlock()
	if queue := s.scripts[worktree]; len(queue) > 0 {
		script := queue[0]
		s.scripts[worktree] = queue[1:]
		return script
	}
	if script, ok := s.defaults[worktree]; ok {
		return script
	}
	if script, ok := s.defaults[""]; ok {
		return script
	}
	return ExecScript{ExitCode: 0}
}

func (s *Sandbox) recordCall(call ExecCall) {
	s.mu.Lock()
	s.execs = append(s.execs, call)
	s.mu.Unlock()
}

func streamLines(ctx context.Context, data string, w io.Writer, perLine time.Duration) error {
	if data == "" || w == nil {
		return nil
	}
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if _, err := fmt.Fprintln(w, scanner.Text()); err != nil {
			return err
		}
		if perLine > 0 {
			select {
			case <-time.After(perLine):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return scanner.Err()
}

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
