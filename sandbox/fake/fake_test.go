package fake

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mateusduraes/ramo/sandbox"
)

func TestFakeRecordsExecCalls(t *testing.T) {
	s := New()
	inst, err := s.Start(context.Background(), sandbox.StartConfig{Worktree: "feat/x"})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Stop(context.Background())

	_, err = inst.Exec(context.Background(), sandbox.ExecOptions{
		Cmd: []string{"echo", "hello"},
		Dir: "/workspace",
		Env: map[string]string{"FOO": "bar"},
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}

	calls := s.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	got := calls[0]
	if got.Worktree != "feat/x" {
		t.Errorf("worktree: got %q", got.Worktree)
	}
	if strings.Join(got.Cmd, " ") != "echo hello" {
		t.Errorf("cmd: got %v", got.Cmd)
	}
	if got.Env["FOO"] != "bar" {
		t.Errorf("env: got %v", got.Env)
	}
}

func TestFakeStdoutStreaming(t *testing.T) {
	s := New()
	s.QueueExec("feat/x", ExecScript{Stdout: "line one\nline two\n", ExitCode: 0})
	inst, _ := s.Start(context.Background(), sandbox.StartConfig{Worktree: "feat/x"})
	defer inst.Stop(context.Background())

	var out bytes.Buffer
	res, err := inst.Exec(context.Background(), sandbox.ExecOptions{
		Cmd:    []string{"claude"},
		Stdout: &out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit: %d", res.ExitCode)
	}
	if !strings.Contains(out.String(), "line one") || !strings.Contains(out.String(), "line two") {
		t.Errorf("stdout: got %q", out.String())
	}
}

func TestFakeExitCodeNonZero(t *testing.T) {
	s := New()
	s.QueueExec("w", ExecScript{ExitCode: 7})
	inst, _ := s.Start(context.Background(), sandbox.StartConfig{Worktree: "w"})
	defer inst.Stop(context.Background())

	res, err := inst.Exec(context.Background(), sandbox.ExecOptions{Cmd: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 7 {
		t.Errorf("exit: %d", res.ExitCode)
	}
}

func TestFakeExecErr(t *testing.T) {
	s := New()
	s.QueueExec("w", ExecScript{Err: errors.New("boom")})
	inst, _ := s.Start(context.Background(), sandbox.StartConfig{Worktree: "w"})
	defer inst.Stop(context.Background())
	_, err := inst.Exec(context.Background(), sandbox.ExecOptions{Cmd: []string{"x"}})
	if err == nil || err.Error() != "boom" {
		t.Errorf("expected boom, got %v", err)
	}
}

func TestFakeContextCancellation(t *testing.T) {
	s := New()
	s.QueueExec("w", ExecScript{Delay: 1 * time.Second})
	inst, _ := s.Start(context.Background(), sandbox.StartConfig{Worktree: "w"})
	defer inst.Stop(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := inst.Exec(ctx, sandbox.ExecOptions{Cmd: []string{"x"}})
	elapsed := time.Since(start)
	if err == nil {
		t.Errorf("expected cancellation error")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("exec should have returned promptly after cancel, took %v", elapsed)
	}
}
