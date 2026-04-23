package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSummaryBuilderSuccessRun(t *testing.T) {
	root := t.TempDir()
	s := NewSummaryBuilder(root)

	runID := "run-1"
	now := time.Now()
	s.Handle(Event{Time: now, RunID: runID, Kind: KindRunStarted})
	s.Handle(Event{Time: now, RunID: runID, Worktree: "feat/signup", Kind: KindCreateWorktree})
	s.Handle(Event{RunID: runID, Worktree: "feat/signup", Step: "implement", Kind: KindStepStarted})
	s.Handle(Event{RunID: runID, Worktree: "feat/signup", Step: "implement", Kind: KindIterationFinished, Payload: map[string]any{"iteration": 1}})
	s.Handle(Event{RunID: runID, Worktree: "feat/signup", Step: "implement", Kind: KindDoneSignalFound})
	s.Handle(Event{RunID: runID, Worktree: "feat/signup", Step: "implement", Kind: KindStepFinished})
	s.Handle(Event{RunID: runID, Worktree: "feat/signup", Kind: KindWorktreeSucceeded})
	s.Handle(Event{Time: now, RunID: runID, Kind: KindRunFinished})

	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	path := filepath.Join(root, "reports", "run-"+runID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("report missing: %v", err)
	}
	report := string(data)
	if !strings.Contains(report, "feat/signup") {
		t.Errorf("report missing worktree name:\n%s", report)
	}
	if !strings.Contains(report, "succeeded") {
		t.Errorf("report missing status:\n%s", report)
	}
	if !strings.Contains(report, "implement") {
		t.Errorf("report missing step:\n%s", report)
	}
	if !strings.Contains(report, "yes") {
		t.Errorf("report missing done signal marker:\n%s", report)
	}
}

func TestSummaryBuilderCancelledRun(t *testing.T) {
	root := t.TempDir()
	s := NewSummaryBuilder(root)
	runID := "run-cancel"

	s.Handle(Event{RunID: runID, Kind: KindRunStarted})
	s.Handle(Event{RunID: runID, Worktree: "feat/x", Kind: KindCreateWorktree})
	s.Handle(Event{RunID: runID, Worktree: "feat/x", Step: "implement", Kind: KindStepStarted})
	s.Handle(Event{RunID: runID, Worktree: "feat/x", Kind: KindCancelled})
	s.Handle(Event{RunID: runID, Kind: KindRunFinished})

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(root, "reports", "run-"+runID+".md"))
	report := string(data)
	if !strings.Contains(report, "cancelled") {
		t.Errorf("expected cancelled status:\n%s", report)
	}
}

func TestSummaryBuilderFailureRun(t *testing.T) {
	root := t.TempDir()
	s := NewSummaryBuilder(root)
	runID := "run-fail"

	s.Handle(Event{RunID: runID, Kind: KindRunStarted})
	s.Handle(Event{RunID: runID, Worktree: "feat/x", Kind: KindCreateWorktree})
	s.Handle(Event{RunID: runID, Worktree: "feat/x", Step: "implement", Kind: KindStepStarted})
	s.Handle(Event{RunID: runID, Worktree: "feat/x", Step: "implement", Kind: KindStepFailed, Payload: map[string]any{"reason": "exit 1"}})
	s.Handle(Event{RunID: runID, Worktree: "feat/x", Step: "verify", Kind: KindStepSkipped})
	s.Handle(Event{RunID: runID, Worktree: "feat/x", Kind: KindWorktreeFailed})
	s.Handle(Event{RunID: runID, Kind: KindRunFinished})

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(root, "reports", "run-"+runID+".md"))
	report := string(data)
	if !strings.Contains(report, "failed") {
		t.Errorf("expected failed status:\n%s", report)
	}
	if !strings.Contains(report, "exit 1") {
		t.Errorf("expected failure reason:\n%s", report)
	}
	if !strings.Contains(report, "verify") {
		t.Errorf("expected skipped step in report:\n%s", report)
	}
}
