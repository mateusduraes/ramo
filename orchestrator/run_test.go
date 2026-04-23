package orchestrator

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mateusduraes/ramo/observability"
	"github.com/mateusduraes/ramo/sandbox"
	sandboxfake "github.com/mateusduraes/ramo/sandbox/fake"
)

// setupGitRepo initializes a minimal repo for worktree ops.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "t@t.com"},
		{"git", "config", "user.name", "t"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// writePromptFile creates a file that Step.PromptFile can reference.
func writePromptFile(t *testing.T, repoDir, rel, content string) {
	t.Helper()
	path := filepath.Join(repoDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// collectingConsumer records events for assertion.
type collectingConsumer struct {
	mu     sync.Mutex
	events []observability.Event
}

func (c *collectingConsumer) Handle(ev observability.Event) {
	c.mu.Lock()
	c.events = append(c.events, ev)
	c.mu.Unlock()
}

func (c *collectingConsumer) Close() error { return nil }

func (c *collectingConsumer) kinds() []observability.EventKind {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]observability.EventKind, len(c.events))
	for i, e := range c.events {
		out[i] = e.Kind
	}
	return out
}

func (c *collectingConsumer) countByKind(k observability.EventKind) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, e := range c.events {
		if e.Kind == k {
			n++
		}
	}
	return n
}

func TestRunSingleWorktreeSingleStepSuccess(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "prompts/implement.md", "implement the feature")

	fake := sandboxfake.New()
	fake.SetDefault("", sandboxfake.ExecScript{Stdout: "ok\n"})

	cc := &collectingConsumer{}

	err := Run(context.Background(), Config{
		RepoDir:          repoDir,
		Sandbox:          fake,
		SkipPreflightEnv: true,
		Consumers:        []observability.Consumer{cc},
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Steps: []Step{
				{Name: "implement", PromptFile: "prompts/implement.md"},
			},
		}},
	})

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if cc.countByKind(observability.KindWorktreeSucceeded) != 1 {
		t.Errorf("expected 1 WorktreeSucceeded; kinds=%v", cc.kinds())
	}
	if cc.countByKind(observability.KindStepFinished) != 1 {
		t.Errorf("expected 1 StepFinished")
	}
	if cc.countByKind(observability.KindIterationStarted) != 1 {
		t.Errorf("expected 1 iteration")
	}
}

func TestRunDoneSignalEarlyExit(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "prompt")

	fake := sandboxfake.New()
	// First iteration: no signal. Second: signal present. Third: should never run.
	fake.QueueExec("feat/x", sandboxfake.ExecScript{Stdout: "working...\n", ExitCode: 0})
	fake.QueueExec("feat/x", sandboxfake.ExecScript{Stdout: "all done <ramo>DONE</ramo>\n", ExitCode: 0})
	fake.QueueExec("feat/x", sandboxfake.ExecScript{Stdout: "should not run\n", ExitCode: 0})

	cc := &collectingConsumer{}
	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Consumers: []observability.Consumer{cc},
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Steps: []Step{{
				Name: "impl", PromptFile: "p.md",
				MaxIterations: 5,
				DoneSignal:    "<ramo>DONE</ramo>",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Only 2 claude execs happened (iterations 1 and 2). Sandbox records all execs.
	// Note: setup has 0 calls because Setup is empty; sandbox calls are exactly iterations.
	if n := len(fake.Calls()); n != 2 {
		t.Errorf("expected 2 exec calls, got %d", n)
	}
	if cc.countByKind(observability.KindDoneSignalFound) != 1 {
		t.Errorf("expected 1 done-signal event")
	}
}

func TestRunDoneSignalExhaustedFails(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")
	fake := sandboxfake.New()
	fake.SetDefault("feat/x", sandboxfake.ExecScript{Stdout: "no signal\n", ExitCode: 0})

	cc := &collectingConsumer{}
	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Consumers: []observability.Consumer{cc},
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Steps: []Step{{
				Name: "impl", PromptFile: "p.md",
				MaxIterations: 3,
				DoneSignal:    "<DONE>",
			}},
		}},
	})
	if err == nil {
		t.Fatalf("expected error when DoneSignal never matches")
	}
	if n := len(fake.Calls()); n != 3 {
		t.Errorf("expected exactly 3 iterations, got %d", n)
	}
	if cc.countByKind(observability.KindStepFailed) != 1 {
		t.Errorf("expected 1 step_failed event")
	}
}

func TestRunSingleIterationNoSignal(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")
	fake := sandboxfake.New()
	fake.SetDefault("", sandboxfake.ExecScript{Stdout: "done\n"})

	cc := &collectingConsumer{}
	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Consumers: []observability.Consumer{cc},
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Steps:  []Step{{Name: "impl", PromptFile: "p.md"}},
		}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := len(fake.Calls()); n != 1 {
		t.Errorf("expected 1 iteration, got %d", n)
	}
}

func TestRunFailFastWithinWorktree(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "a.md", "")
	writePromptFile(t, repoDir, "b.md", "")

	fake := sandboxfake.New()
	// First step fails (non-zero exit).
	fake.QueueExec("feat/x", sandboxfake.ExecScript{ExitCode: 1})

	cc := &collectingConsumer{}
	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Consumers: []observability.Consumer{cc},
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Steps: []Step{
				{Name: "implement", PromptFile: "a.md"},
				{Name: "verify", PromptFile: "b.md"},
			},
		}},
	})
	if err == nil {
		t.Fatalf("expected failure")
	}
	// Only 1 claude call — the second step must not have executed.
	if n := len(fake.Calls()); n != 1 {
		t.Errorf("expected 1 exec, got %d", n)
	}
	if cc.countByKind(observability.KindStepSkipped) != 1 {
		t.Errorf("expected verify step to be skipped")
	}
}

func TestRunMultipleWorktreesInParallel(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")

	fake := sandboxfake.New()
	// Per-worktree delay so we can observe parallelism.
	fake.SetDefault("feat/a", sandboxfake.ExecScript{Stdout: "a done\n", Delay: 120 * time.Millisecond})
	fake.SetDefault("feat/b", sandboxfake.ExecScript{Stdout: "b done\n", Delay: 120 * time.Millisecond})
	fake.SetDefault("feat/c", sandboxfake.ExecScript{Stdout: "c done\n", Delay: 120 * time.Millisecond})

	cc := &collectingConsumer{}
	start := time.Now()
	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Consumers: []observability.Consumer{cc},
		Worktrees: []Worktree{
			{Branch: "feat/a", Steps: []Step{{Name: "s", PromptFile: "p.md"}}},
			{Branch: "feat/b", Steps: []Step{{Name: "s", PromptFile: "p.md"}}},
			{Branch: "feat/c", Steps: []Step{{Name: "s", PromptFile: "p.md"}}},
		},
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if elapsed > 250*time.Millisecond {
		t.Errorf("expected parallel execution (< 250ms), got %v", elapsed)
	}
	if cc.countByKind(observability.KindWorktreeSucceeded) != 3 {
		t.Errorf("expected 3 worktree successes")
	}
}

func TestRunOneWorktreeFailureDoesNotAffectOthers(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")

	fake := sandboxfake.New()
	fake.SetDefault("feat/ok", sandboxfake.ExecScript{Stdout: "ok\n"})
	fake.SetDefault("feat/fail", sandboxfake.ExecScript{ExitCode: 1})

	cc := &collectingConsumer{}
	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Consumers: []observability.Consumer{cc},
		Worktrees: []Worktree{
			{Branch: "feat/ok", Steps: []Step{{Name: "s", PromptFile: "p.md"}}},
			{Branch: "feat/fail", Steps: []Step{{Name: "s", PromptFile: "p.md"}}},
		},
	})
	if err == nil {
		t.Fatalf("expected failure")
	}
	if cc.countByKind(observability.KindWorktreeSucceeded) != 1 {
		t.Errorf("expected 1 worktree success")
	}
	if cc.countByKind(observability.KindWorktreeFailed) != 1 {
		t.Errorf("expected 1 worktree failure")
	}
}

func TestPreflightMissingPromptFile(t *testing.T) {
	repoDir := setupGitRepo(t)
	fake := sandboxfake.New()

	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Steps:  []Step{{Name: "s", PromptFile: "nonexistent.md"}},
		}},
	})
	if err == nil {
		t.Fatalf("expected error for missing prompt file")
	}
	if !strings.Contains(err.Error(), "nonexistent.md") {
		t.Errorf("error should name the missing file, got: %v", err)
	}
	// No container should have been created.
	if n := len(fake.Starts()); n != 0 {
		t.Errorf("expected no sandbox starts; got %d", n)
	}
}

func TestPreflightMissingCopySourceForFreshWorktree(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")
	fake := sandboxfake.New()

	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Copy:   []FileCopy{{From: "missing.txt", To: "missing.txt"}},
			Steps:  []Step{{Name: "s", PromptFile: "p.md"}},
		}},
	})
	if err == nil {
		t.Fatalf("expected error for missing copy source")
	}
	if len(fake.Starts()) != 0 {
		t.Errorf("expected no sandbox start")
	}
}

func TestSetupRunsInOrder(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")

	fake := sandboxfake.New()
	fake.SetDefault("", sandboxfake.ExecScript{Stdout: "ok\n"})

	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Setup:  []string{"echo one", "echo two"},
			Steps:  []Step{{Name: "s", PromptFile: "p.md"}},
		}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	calls := fake.Calls()
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls (2 setup + 1 step), got %d", len(calls))
	}
	if !strings.Contains(strings.Join(calls[0].Cmd, " "), "echo one") {
		t.Errorf("first call should be 'echo one', got %v", calls[0].Cmd)
	}
	if !strings.Contains(strings.Join(calls[1].Cmd, " "), "echo two") {
		t.Errorf("second call should be 'echo two', got %v", calls[1].Cmd)
	}
}

func TestSetupFailureAbortsWorktree(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")

	fake := sandboxfake.New()
	fake.QueueExec("feat/x", sandboxfake.ExecScript{ExitCode: 2})

	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Setup:  []string{"false-cmd"},
			Steps:  []Step{{Name: "s", PromptFile: "p.md"}},
		}},
	})
	if err == nil {
		t.Fatalf("expected setup failure")
	}
	if n := len(fake.Calls()); n != 1 {
		t.Errorf("expected only 1 call (setup), got %d", n)
	}
}

func TestCopyRunsOnFreshOnly(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")
	src := filepath.Join(repoDir, "env.txt")
	if err := os.WriteFile(src, []byte("HELLO=world"), 0o644); err != nil {
		t.Fatal(err)
	}

	fake := sandboxfake.New()
	fake.SetDefault("", sandboxfake.ExecScript{Stdout: "ok\n"})

	cfg := Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Copy:   []FileCopy{{From: "env.txt", To: ".env"}},
			Steps:  []Step{{Name: "s", PromptFile: "p.md"}},
		}},
	}
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("first run: %v", err)
	}
	destPath := filepath.Join(repoDir, ".worktrees", "feat/x", ".env")
	if data, err := os.ReadFile(destPath); err != nil || !strings.Contains(string(data), "HELLO=world") {
		t.Errorf("expected copy on fresh; file=%q err=%v", data, err)
	}

	// Overwrite user file to detect preservation.
	if err := os.WriteFile(destPath, []byte("USER_EDIT=1"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("second run: %v", err)
	}
	data, _ := os.ReadFile(destPath)
	if !strings.Contains(string(data), "USER_EDIT=1") {
		t.Errorf("expected user edit to be preserved on resume; got %q", string(data))
	}
}

func TestHostStepUsesHostExecutor(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")
	fake := sandboxfake.New()

	hostCalls := 0
	var cmd []string
	var dir string
	hostExec := hostExecutorFunc(func(ctx context.Context, opts sandbox.ExecOptions) (sandbox.ExecResult, error) {
		hostCalls++
		cmd = opts.Cmd
		dir = opts.Dir
		return sandbox.ExecResult{ExitCode: 0}, nil
	})

	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		HostExecutor: hostExec,
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Steps:  []Step{{Name: "open-pr", PromptFile: "p.md", RunOn: Host}},
		}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hostCalls != 1 {
		t.Errorf("expected 1 host call, got %d", hostCalls)
	}
	if len(fake.Starts()) != 0 {
		t.Errorf("host-only pipeline should not start sandbox; got %d", len(fake.Starts()))
	}
	if !containsString(strings.Join(cmd, " "), "--dangerously-skip-permissions") {
		t.Errorf("host step missing permission flag: %v", cmd)
	}
	expectedDir := filepath.Join(repoDir, ".worktrees", "feat/x")
	if dir != expectedDir {
		t.Errorf("host step dir: got %q want %q", dir, expectedDir)
	}
}

func TestStepsIncludeDangerouslySkipPermissions(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")
	fake := sandboxfake.New()
	fake.SetDefault("", sandboxfake.ExecScript{ExitCode: 0})

	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Steps:  []Step{{Name: "s", PromptFile: "p.md"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	joined := strings.Join(calls[0].Cmd, " ")
	if !strings.Contains(joined, "--dangerously-skip-permissions") {
		t.Errorf("container step missing permission flag: %v", calls[0].Cmd)
	}
	if !strings.Contains(joined, "claude -p -") {
		t.Errorf("expected 'claude -p -' in command, got %v", calls[0].Cmd)
	}
}

func TestAuthEnvPassedToSandbox(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")

	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-test")
	t.Setenv("MY_CUSTOM_VAR", "custom-val")

	fake := sandboxfake.New()
	fake.SetDefault("", sandboxfake.ExecScript{ExitCode: 0})

	err := Run(context.Background(), Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Env: []string{"MY_CUSTOM_VAR"},
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Steps:  []Step{{Name: "s", PromptFile: "p.md"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	starts := fake.Starts()
	if len(starts) != 1 {
		t.Fatalf("expected 1 start, got %d", len(starts))
	}
	got := starts[0].Env
	if got["ANTHROPIC_API_KEY"] != "sk-test" {
		t.Errorf("ANTHROPIC_API_KEY missing: %v", got)
	}
	if got["CLAUDE_CODE_OAUTH_TOKEN"] != "oauth-test" {
		t.Errorf("CLAUDE_CODE_OAUTH_TOKEN missing: %v", got)
	}
	if got["MY_CUSTOM_VAR"] != "custom-val" {
		t.Errorf("MY_CUSTOM_VAR passthrough failed: %v", got)
	}
}

func TestIdleTimeoutFires(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")

	fake := sandboxfake.New()
	// Script: no stdout, long delay — idle timer will fire.
	fake.SetDefault("", sandboxfake.ExecScript{Delay: 2 * time.Second})

	cc := &collectingConsumer{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := Run(ctx, Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		IdleTimeout: 100 * time.Millisecond,
		Consumers:   []observability.Consumer{cc},
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Steps: []Step{{
				Name: "s", PromptFile: "p.md",
				MaxIterations: 1,
				// no DoneSignal: step succeeds after 1 iteration. But idle fires,
				// then next iter is tried if available. With MaxIterations=1 and
				// no DoneSignal, step still ends after the single iteration.
			}},
		}},
	})
	// Idle timeout with MaxIterations=1 and no DoneSignal: the single iteration
	// is idle-terminated; step-level success depends on whether we treat idle as failure.
	// Our implementation: idle iteration doesn't fail the step unless DoneSignal requires more.
	// Without DoneSignal + MaxIterations=1, step completes successfully after the idle-terminated iter.
	// But we still want to see the idle_timeout event.
	_ = err
	if cc.countByKind(observability.KindIdleTimeout) != 1 {
		t.Errorf("expected idle_timeout event, kinds=%v", cc.kinds())
	}
}

func TestCancellationTearsDownSandbox(t *testing.T) {
	repoDir := setupGitRepo(t)
	writePromptFile(t, repoDir, "p.md", "")

	fake := sandboxfake.New()
	fake.SetDefault("", sandboxfake.ExecScript{Delay: 5 * time.Second})

	cc := &collectingConsumer{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := Run(ctx, Config{
		RepoDir: repoDir, Sandbox: fake, SkipPreflightEnv: true,
		Consumers: []observability.Consumer{cc},
		Worktrees: []Worktree{{
			Branch: "feat/x",
			Steps:  []Step{{Name: "s", PromptFile: "p.md", MaxIterations: 1}},
		}},
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected cancellation error")
	}
	if elapsed > 2*time.Second {
		t.Errorf("expected Run to return promptly after cancel; took %v", elapsed)
	}
	if cc.countByKind(observability.KindSandboxStopped) != 1 {
		t.Errorf("expected sandbox to be stopped on cancel; kinds=%v", cc.kinds())
	}
}

// hostExecutorFunc adapts a function into the HostExecutor interface.
type hostExecutorFunc func(ctx context.Context, opts sandbox.ExecOptions) (sandbox.ExecResult, error)

func (f hostExecutorFunc) Exec(ctx context.Context, opts sandbox.ExecOptions) (sandbox.ExecResult, error) {
	return f(ctx, opts)
}

// containsString is a tiny helper to avoid importing strings just for this.
func containsString(s, sub string) bool { return strings.Contains(s, sub) }

// ensure errors package import is used.
var _ = errors.New
