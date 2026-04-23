package observability

import (
	"bytes"
	"strings"
	"testing"
)

func TestStdoutConsumerPrintsStepTransitions(t *testing.T) {
	var buf bytes.Buffer
	c := NewStdoutConsumerTo(&buf)

	c.Handle(Event{Worktree: "feat/signup", Step: "implement", Kind: KindStepStarted})
	c.Handle(Event{Worktree: "feat/signup", Step: "implement", Kind: KindIterationStarted, Payload: map[string]any{"iteration": 1}})
	c.Handle(Event{Worktree: "feat/signup", Step: "implement", Kind: KindStepFinished})

	out := buf.String()
	if !strings.Contains(out, "[feat/signup]") {
		t.Errorf("expected worktree tag, got:\n%s", out)
	}
	if !strings.Contains(out, "implement started") {
		t.Errorf("expected step started line, got:\n%s", out)
	}
	if !strings.Contains(out, "iteration 1") {
		t.Errorf("expected iteration line, got:\n%s", out)
	}
	if !strings.Contains(out, "implement finished") {
		t.Errorf("expected step finished line, got:\n%s", out)
	}
}

func TestStdoutConsumerIgnoresRawAgentOutput(t *testing.T) {
	var buf bytes.Buffer
	c := NewStdoutConsumerTo(&buf)
	c.Handle(Event{Kind: KindStdoutLine, Worktree: "feat/x", Step: "implement", Payload: map[string]any{"line": "agent says hi"}})
	if buf.Len() != 0 {
		t.Errorf("expected stdout consumer to skip raw lines, got: %q", buf.String())
	}
}

func TestStdoutConsumerResumeLine(t *testing.T) {
	var buf bytes.Buffer
	c := NewStdoutConsumerTo(&buf)
	c.Handle(Event{Worktree: "feat/x", Kind: KindResumeWorktree, Payload: map[string]any{"files_changed": 12}})
	out := buf.String()
	if !strings.Contains(out, "resuming existing worktree") || !strings.Contains(out, "12") {
		t.Errorf("expected resume line with file count, got: %q", out)
	}
}
