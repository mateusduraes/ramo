package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileLoggerWritesPerStepFile(t *testing.T) {
	root := t.TempDir()
	f := NewFileLogger(root)

	ev := func(kind EventKind, wt, step string, payload map[string]any) Event {
		return Event{RunID: "run-42", Worktree: wt, Step: step, Kind: kind, Payload: payload}
	}

	f.Handle(ev(KindIterationStarted, "feat/signup", "implement", map[string]any{"iteration": 1}))
	f.Handle(ev(KindStdoutLine, "feat/signup", "implement", map[string]any{"line": "hello"}))
	f.Handle(ev(KindIterationFinished, "feat/signup", "implement", map[string]any{"iteration": 1}))

	f.Handle(ev(KindIterationStarted, "feat/signup", "verify", map[string]any{"iteration": 1}))
	f.Handle(ev(KindStdoutLine, "feat/signup", "verify", map[string]any{"line": "all good"}))

	if err := f.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	implement := filepath.Join(root, "logs", "run-42", "feat-signup", "implement.log")
	verify := filepath.Join(root, "logs", "run-42", "feat-signup", "verify.log")

	implementData := mustRead(t, implement)
	if !strings.Contains(implementData, "hello") {
		t.Errorf("expected 'hello' in implement log, got:\n%s", implementData)
	}
	if !strings.Contains(implementData, "iteration 1") {
		t.Errorf("expected iteration marker, got:\n%s", implementData)
	}

	verifyData := mustRead(t, verify)
	if !strings.Contains(verifyData, "all good") {
		t.Errorf("expected 'all good' in verify log, got:\n%s", verifyData)
	}
}

func TestFileLoggerPreservesMultipleIterations(t *testing.T) {
	root := t.TempDir()
	f := NewFileLogger(root)

	push := func(kind EventKind, payload map[string]any) {
		f.Handle(Event{RunID: "r", Worktree: "feat/x", Step: "impl", Kind: kind, Payload: payload})
	}

	push(KindIterationStarted, map[string]any{"iteration": 1})
	push(KindStdoutLine, map[string]any{"line": "first try"})
	push(KindIterationFinished, map[string]any{"iteration": 1})

	push(KindIterationStarted, map[string]any{"iteration": 2})
	push(KindStdoutLine, map[string]any{"line": "second try"})

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	data := mustRead(t, filepath.Join(root, "logs", "r", "feat-x", "impl.log"))
	if !strings.Contains(data, "iteration 1") || !strings.Contains(data, "iteration 2") {
		t.Errorf("expected both iteration markers, got:\n%s", data)
	}
	if !strings.Contains(data, "first try") || !strings.Contains(data, "second try") {
		t.Errorf("expected both iterations' output, got:\n%s", data)
	}
	// iteration 2 marker must appear after "first try"
	i1 := strings.Index(data, "first try")
	i2 := strings.Index(data, "iteration 2")
	if i1 < 0 || i2 < 0 || i2 < i1 {
		t.Errorf("iteration ordering wrong:\n%s", data)
	}
}

func TestFileLoggerStderrTagged(t *testing.T) {
	root := t.TempDir()
	f := NewFileLogger(root)

	f.Handle(Event{RunID: "r", Worktree: "w", Step: "s", Kind: KindIterationStarted, Payload: map[string]any{"iteration": 1}})
	f.Handle(Event{RunID: "r", Worktree: "w", Step: "s", Kind: KindStderrLine, Payload: map[string]any{"line": "boom"}})
	_ = f.Close()

	data := mustRead(t, filepath.Join(root, "logs", "r", "w", "s.log"))
	if !strings.Contains(data, "[stderr] boom") {
		t.Errorf("expected [stderr] tag, got:\n%s", data)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
