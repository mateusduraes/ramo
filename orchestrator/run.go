package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mateusduraes/ramo/observability"
	"github.com/mateusduraes/ramo/sandbox"
)

// Run is the orchestrator's public entry point. It normalizes cfg, runs
// preflight, launches one goroutine per worktree, and blocks until all
// finish (or ctx is cancelled). It returns nil on a fully successful run,
// an error describing the first/aggregate failure otherwise.
func Run(ctx context.Context, cfg Config) error {
	cfg = normalizeConfig(cfg)
	if cfg.RepoDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		cfg.RepoDir = wd
	}
	if cfg.WorktreesDir == "" {
		cfg.WorktreesDir = filepath.Join(cfg.RepoDir, ".worktrees")
	}
	if cfg.RunID == "" {
		cfg.RunID = newRunID()
	}
	if cfg.HostExecutor == nil {
		cfg.HostExecutor = newHostExecutor()
	}

	if hasContainerSteps(cfg) && cfg.Sandbox == nil {
		return errors.New("orchestrator: Sandbox is required when any step uses RunOn=Container")
	}

	emitter := observability.NewEmitter()
	for _, c := range cfg.Consumers {
		emitter.Subscribe(c)
	}
	defer emitter.Close()

	r := &runner{
		cfg:     cfg,
		runID:   cfg.RunID,
		emitter: emitter,
	}

	r.emit(observability.Event{
		Time: time.Now(), RunID: r.runID, Kind: observability.KindRunStarted,
	})
	defer r.emit(observability.Event{
		Time: time.Now(), RunID: r.runID, Kind: observability.KindRunFinished,
	})

	if err := preflight(ctx, cfg, r.runID, emitter); err != nil {
		return err
	}

	var wg sync.WaitGroup
	failures := make([]error, len(cfg.Worktrees))

	for i, w := range cfg.Worktrees {
		wg.Add(1)
		go func(i int, w Worktree) {
			defer wg.Done()
			if err := r.runPipeline(ctx, w); err != nil {
				failures[i] = err
			}
		}(i, w)
	}

	wg.Wait()

	var firstErr error
	for _, e := range failures {
		if e != nil {
			if firstErr == nil {
				firstErr = e
			}
		}
	}
	if err := ctx.Err(); err != nil && firstErr == nil {
		return err
	}
	return firstErr
}

// runner holds per-Run shared state. Exactly one is allocated per Run call.
type runner struct {
	cfg     Config
	runID   string
	emitter *observability.Emitter
}

func (r *runner) emit(ev observability.Event) {
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	r.emitter.Emit(ev)
}

func (r *runner) runPipeline(ctx context.Context, w Worktree) error {
	resumed, err := r.ensureWorktree(w)
	if err != nil {
		r.emit(observability.Event{
			RunID: r.runID, Worktree: w.Branch,
			Kind:    observability.KindWorktreeFailed,
			Payload: map[string]any{"reason": err.Error()},
		})
		return err
	}

	if resumed {
		r.emit(observability.Event{
			RunID: r.runID, Worktree: w.Branch,
			Kind:    observability.KindResumeWorktree,
			Payload: map[string]any{"files_changed": countChanges(r.worktreePath(w.Branch))},
		})
	} else {
		r.emit(observability.Event{
			RunID: r.runID, Worktree: w.Branch,
			Kind: observability.KindCreateWorktree,
		})
		if err := r.runCopies(w); err != nil {
			r.emit(observability.Event{
				RunID: r.runID, Worktree: w.Branch,
				Kind:    observability.KindWorktreeFailed,
				Payload: map[string]any{"reason": err.Error()},
			})
			return err
		}
	}

	var inst sandbox.Instance
	needsContainer := false
	for _, s := range w.Steps {
		if s.RunOn == Container {
			needsContainer = true
			break
		}
	}
	if needsContainer {
		startCfg := sandbox.StartConfig{
			Worktree: w.Branch,
			HostPath: r.worktreePath(w.Branch),
			RunID:    r.runID,
			Env:      r.resolveEnv(),
		}
		inst, err = r.cfg.Sandbox.Start(ctx, startCfg)
		if err != nil {
			r.emit(observability.Event{
				RunID: r.runID, Worktree: w.Branch,
				Kind:    observability.KindWorktreeFailed,
				Payload: map[string]any{"reason": err.Error()},
			})
			return err
		}
		r.emit(observability.Event{
			RunID: r.runID, Worktree: w.Branch, Kind: observability.KindSandboxStarted,
		})
		defer func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = inst.Stop(stopCtx)
			r.emit(observability.Event{
				RunID: r.runID, Worktree: w.Branch, Kind: observability.KindSandboxStopped,
			})
		}()

		if err := r.runSetup(ctx, inst, w); err != nil {
			r.emit(observability.Event{
				RunID: r.runID, Worktree: w.Branch,
				Kind:    observability.KindWorktreeFailed,
				Payload: map[string]any{"reason": err.Error()},
			})
			return err
		}
	}

	for i, step := range w.Steps {
		if err := ctx.Err(); err != nil {
			r.emit(observability.Event{
				RunID: r.runID, Worktree: w.Branch, Step: step.Name,
				Kind: observability.KindCancelled,
			})
			// remaining skipped
			for _, s := range w.Steps[i+1:] {
				r.emit(observability.Event{
					RunID: r.runID, Worktree: w.Branch, Step: s.Name,
					Kind: observability.KindStepSkipped,
				})
			}
			r.emit(observability.Event{
				RunID: r.runID, Worktree: w.Branch,
				Kind: observability.KindWorktreeFailed,
			})
			return err
		}

		var execer stepExecutor
		if step.RunOn == Container {
			execer = inst
		} else {
			execer = r.cfg.HostExecutor
		}

		if err := r.runStep(ctx, execer, w, step); err != nil {
			for _, s := range w.Steps[i+1:] {
				r.emit(observability.Event{
					RunID: r.runID, Worktree: w.Branch, Step: s.Name,
					Kind: observability.KindStepSkipped,
				})
			}
			r.emit(observability.Event{
				RunID: r.runID, Worktree: w.Branch,
				Kind:    observability.KindWorktreeFailed,
				Payload: map[string]any{"reason": err.Error()},
			})
			return err
		}
	}

	r.emit(observability.Event{
		RunID: r.runID, Worktree: w.Branch, Kind: observability.KindWorktreeSucceeded,
	})
	return nil
}

func (r *runner) runSetup(ctx context.Context, inst sandbox.Instance, w Worktree) error {
	for _, cmd := range w.Setup {
		r.emit(observability.Event{
			RunID: r.runID, Worktree: w.Branch,
			Kind:    observability.KindSetupStarted,
			Payload: map[string]any{"command": cmd},
		})
		stdout := &lineRelay{
			runID: r.runID, worktree: w.Branch,
			kind: observability.KindStdoutLine, emit: r.emit,
		}
		stderr := &lineRelay{
			runID: r.runID, worktree: w.Branch,
			kind: observability.KindStderrLine, emit: r.emit,
		}
		res, err := inst.Exec(ctx, sandbox.ExecOptions{
			Cmd:    []string{"/bin/sh", "-lc", cmd},
			Dir:    "/workspace",
			Stdout: stdout,
			Stderr: stderr,
		})
		r.emit(observability.Event{
			RunID: r.runID, Worktree: w.Branch,
			Kind:    observability.KindSetupFinished,
			Payload: map[string]any{"command": cmd, "exit_code": res.ExitCode},
		})
		if err != nil {
			return fmt.Errorf("setup command %q: %w", cmd, err)
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("setup command %q exited %d", cmd, res.ExitCode)
		}
	}
	return nil
}

// resolveEnv collects the host values of known auth env vars plus any user-declared
// Config.Env entries, returning them as a map for the sandbox to set.
func (r *runner) resolveEnv() map[string]string {
	out := map[string]string{}
	for _, name := range authEnvNames {
		if v, ok := os.LookupEnv(name); ok {
			out[name] = v
		}
	}
	for _, name := range r.cfg.Env {
		if v, ok := os.LookupEnv(name); ok {
			out[name] = v
		}
	}
	return out
}

func newRunID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}

// countChanges returns the number of files with pending changes in the
// given worktree. Used for the resume event payload. Best-effort: returns 0
// on any git error.
func countChanges(worktreePath string) int {
	// Use git status --porcelain to count lines.
	// Avoids introducing a dependency; best-effort.
	// Keep this function dependency-free by not importing worktree package.
	data, err := readFileOrEmpty(filepath.Join(worktreePath, ".git"))
	_ = data
	_ = err
	// Run git status --porcelain
	return gitPorcelainCount(worktreePath)
}

func readFileOrEmpty(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}
