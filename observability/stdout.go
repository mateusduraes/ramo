package observability

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// StdoutConsumer emits one human-readable line per significant event,
// prefixed with the worktree tag. It never streams raw agent output.
type StdoutConsumer struct {
	mu sync.Mutex
	w  io.Writer
}

func NewStdoutConsumer() *StdoutConsumer {
	return &StdoutConsumer{w: os.Stdout}
}

func NewStdoutConsumerTo(w io.Writer) *StdoutConsumer {
	return &StdoutConsumer{w: w}
}

func (s *StdoutConsumer) Handle(ev Event) {
	line := formatStdoutLine(ev)
	if line == "" {
		return
	}
	s.mu.Lock()
	fmt.Fprintln(s.w, line)
	s.mu.Unlock()
}

func (s *StdoutConsumer) Close() error { return nil }

func formatStdoutLine(ev Event) string {
	tag := ""
	if ev.Worktree != "" {
		tag = fmt.Sprintf("[%s] ", ev.Worktree)
	}
	switch ev.Kind {
	case KindRunStarted:
		return fmt.Sprintf("run %s started", ev.RunID)
	case KindRunFinished:
		return fmt.Sprintf("run %s finished", ev.RunID)
	case KindCreateWorktree:
		return tag + "creating new worktree"
	case KindResumeWorktree:
		files := 0
		if v, ok := ev.Payload["files_changed"]; ok {
			if n, ok := toInt(v); ok {
				files = n
			}
		}
		return fmt.Sprintf("%sresuming existing worktree (%d files changed)", tag, files)
	case KindStepStarted:
		return fmt.Sprintf("%sstep %s started", tag, ev.Step)
	case KindStepFinished:
		return fmt.Sprintf("%sstep %s finished", tag, ev.Step)
	case KindStepFailed:
		reason := ""
		if v, ok := ev.Payload["reason"]; ok {
			reason = fmt.Sprintf(": %v", v)
		}
		return fmt.Sprintf("%sstep %s failed%s", tag, ev.Step, reason)
	case KindStepSkipped:
		return fmt.Sprintf("%sstep %s skipped", tag, ev.Step)
	case KindIterationStarted:
		iter := 0
		if v, ok := ev.Payload["iteration"]; ok {
			if n, ok := toInt(v); ok {
				iter = n
			}
		}
		return fmt.Sprintf("%sstep %s iteration %d", tag, ev.Step, iter)
	case KindDoneSignalFound:
		return fmt.Sprintf("%sstep %s done-signal matched", tag, ev.Step)
	case KindIdleTimeout:
		return fmt.Sprintf("%sstep %s idle timeout", tag, ev.Step)
	case KindCancelled:
		return tag + "cancelled"
	case KindPreflightWarning:
		msg := ""
		if v, ok := ev.Payload["message"]; ok {
			msg = fmt.Sprintf("%v", v)
		}
		return fmt.Sprintf("preflight warning: %s", msg)
	case KindWorktreeSucceeded:
		return tag + "worktree succeeded"
	case KindWorktreeFailed:
		return tag + "worktree failed"
	}
	return ""
}
