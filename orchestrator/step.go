package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mateusduraes/ramo/observability"
	"github.com/mateusduraes/ramo/sandbox"
)

// stepExecutor is the minimal interface an executor must satisfy to run
// steps. Both sandbox.Instance and HostExecutor implement it.
type stepExecutor interface {
	Exec(ctx context.Context, opts sandbox.ExecOptions) (sandbox.ExecResult, error)
}

// runStep executes one step against the given executor.
// Returns nil on success, an error on any failure. On failure, the iteration
// events are still emitted so consumers can see the history.
func (r *runner) runStep(ctx context.Context, exec stepExecutor, w Worktree, step Step) error {
	r.emit(observability.Event{
		RunID: r.runID, Worktree: w.Branch, Step: step.Name,
		Kind: observability.KindStepStarted,
	})

	var lastErr error
	for iter := 1; iter <= step.MaxIterations; iter++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.emit(observability.Event{
			RunID: r.runID, Worktree: w.Branch, Step: step.Name,
			Kind:    observability.KindIterationStarted,
			Payload: map[string]any{"iteration": iter},
		})

		output, idleFired, err := r.runIteration(ctx, exec, w, step)

		r.emit(observability.Event{
			RunID: r.runID, Worktree: w.Branch, Step: step.Name,
			Kind:    observability.KindIterationFinished,
			Payload: map[string]any{"iteration": iter},
		})

		if idleFired {
			r.emit(observability.Event{
				RunID: r.runID, Worktree: w.Branch, Step: step.Name,
				Kind:    observability.KindIdleTimeout,
				Payload: map[string]any{"iteration": iter},
			})
			lastErr = fmt.Errorf("iteration %d: idle timeout", iter)
			// Fall through to next iteration if MaxIterations allows.
			continue
		}
		if err != nil {
			return r.stepFailed(w, step, err)
		}

		if step.DoneSignal != "" && strings.Contains(output, step.DoneSignal) {
			r.emit(observability.Event{
				RunID: r.runID, Worktree: w.Branch, Step: step.Name,
				Kind: observability.KindDoneSignalFound,
			})
			r.emit(observability.Event{
				RunID: r.runID, Worktree: w.Branch, Step: step.Name,
				Kind: observability.KindStepFinished,
			})
			return nil
		}

		// No DoneSignal configured: exit after max iterations without signal check.
		if step.DoneSignal == "" {
			// Single iteration model when DoneSignal is absent:
			// Step completes successfully after MaxIterations runs, regardless.
			// We still emit per-iteration events; the loop continues until iter == MaxIterations.
			if iter == step.MaxIterations {
				r.emit(observability.Event{
					RunID: r.runID, Worktree: w.Branch, Step: step.Name,
					Kind: observability.KindStepFinished,
				})
				return nil
			}
		}
	}

	// Reached MaxIterations with DoneSignal configured but not found.
	if step.DoneSignal != "" {
		reason := "max iterations reached without DoneSignal match"
		if lastErr != nil {
			reason = lastErr.Error()
		}
		return r.stepFailed(w, step, errors.New(reason))
	}

	// Shouldn't happen in practice (the DoneSignal=="" branch returns above), but be safe.
	return nil
}

func (r *runner) stepFailed(w Worktree, step Step, err error) error {
	r.emit(observability.Event{
		RunID: r.runID, Worktree: w.Branch, Step: step.Name,
		Kind:    observability.KindStepFailed,
		Payload: map[string]any{"reason": err.Error()},
	})
	return fmt.Errorf("step %q failed: %w", step.Name, err)
}

// runIteration spawns a single claude subprocess with idle-timer supervision.
// Returns the captured stdout, whether the idle timer fired, and any other error.
func (r *runner) runIteration(ctx context.Context, exec stepExecutor, w Worktree, step Step) (string, bool, error) {
	promptPath := resolvePath(r.cfg.RepoDir, step.PromptFile)
	promptFile, err := os.Open(promptPath)
	if err != nil {
		return "", false, fmt.Errorf("open prompt file: %w", err)
	}
	defer promptFile.Close()

	iterCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	idle := newIdleWatcher(iterCtx, cancel, r.cfg.IdleTimeout)
	defer idle.stop()

	var captured strings.Builder
	stdout := &lineRelay{
		worktree: w.Branch,
		step:     step.Name,
		runID:    r.runID,
		kind:     observability.KindStdoutLine,
		emit:     r.emit,
		onLine:   idle.reset,
		buf:      &captured,
	}
	stderr := &lineRelay{
		worktree: w.Branch,
		step:     step.Name,
		runID:    r.runID,
		kind:     observability.KindStderrLine,
		emit:     r.emit,
		onLine:   idle.reset,
	}

	cmd := []string{"claude", "-p", "-", "--dangerously-skip-permissions"}
	opts := sandbox.ExecOptions{
		Cmd:    cmd,
		Dir:    r.stepDir(w, step),
		Stdin:  promptFile,
		Stdout: stdout,
		Stderr: stderr,
	}

	// Prime the idle timer so pre-output hangs are also caught.
	idle.reset()

	result, execErr := exec.Exec(iterCtx, opts)
	idle.stop()

	output := captured.String()

	if idle.fired() {
		return output, true, nil
	}
	if execErr != nil {
		return output, false, execErr
	}
	if result.ExitCode != 0 {
		return output, false, fmt.Errorf("exit code %d", result.ExitCode)
	}
	return output, false, nil
}

// stepDir returns the working directory to set for the step's exec.
// Container steps use /workspace (the bind-mount). Host steps use the
// worktree's host path.
func (r *runner) stepDir(w Worktree, step Step) string {
	if step.RunOn == Host {
		return r.worktreePath(w.Branch)
	}
	return "/workspace"
}

// idleWatcher cancels the given context if reset() is not called within d.
type idleWatcher struct {
	ctx      context.Context
	cancel   context.CancelFunc
	duration time.Duration

	mu        sync.Mutex
	timer     *time.Timer
	firedOnce bool
	stopped   bool
}

func newIdleWatcher(ctx context.Context, cancel context.CancelFunc, d time.Duration) *idleWatcher {
	if d <= 0 {
		d = DefaultIdleTimeout
	}
	iw := &idleWatcher{
		ctx:      ctx,
		cancel:   cancel,
		duration: d,
	}
	return iw
}

func (iw *idleWatcher) reset() {
	iw.mu.Lock()
	defer iw.mu.Unlock()
	if iw.stopped {
		return
	}
	if iw.timer == nil {
		iw.timer = time.AfterFunc(iw.duration, iw.onFire)
		return
	}
	iw.timer.Reset(iw.duration)
}

func (iw *idleWatcher) onFire() {
	iw.mu.Lock()
	if iw.stopped {
		iw.mu.Unlock()
		return
	}
	iw.firedOnce = true
	iw.mu.Unlock()
	iw.cancel()
}

func (iw *idleWatcher) fired() bool {
	iw.mu.Lock()
	defer iw.mu.Unlock()
	return iw.firedOnce
}

func (iw *idleWatcher) stop() {
	iw.mu.Lock()
	iw.stopped = true
	if iw.timer != nil {
		iw.timer.Stop()
	}
	iw.mu.Unlock()
}

// lineRelay writes complete lines to an event stream (and optional buffer)
// and invokes onLine for each line. It implements io.Writer.
type lineRelay struct {
	runID    string
	worktree string
	step     string
	kind     observability.EventKind
	emit     func(observability.Event)
	onLine   func()
	buf      *strings.Builder

	mu      sync.Mutex
	partial []byte
}

func (l *lineRelay) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data := append(l.partial, p...)
	l.partial = nil

	start := 0
	for i, b := range data {
		if b == '\n' {
			line := string(data[start:i])
			l.handleLine(line)
			start = i + 1
		}
	}
	if start < len(data) {
		l.partial = append(l.partial[:0], data[start:]...)
	}
	return len(p), nil
}

func (l *lineRelay) handleLine(line string) {
	if l.buf != nil {
		l.buf.WriteString(line)
		l.buf.WriteByte('\n')
	}
	if l.onLine != nil {
		l.onLine()
	}
	if l.emit != nil {
		l.emit(observability.Event{
			RunID:    l.runID,
			Worktree: l.worktree,
			Step:     l.step,
			Kind:     l.kind,
			Payload:  map[string]any{"line": line},
		})
	}
}

// flushWriter is an io.Closer wrapper: callers that want to flush trailing
// partial lines before the iteration ends can call Flush. Currently unused but
// kept for future consumers.
var _ io.Writer = (*lineRelay)(nil)
