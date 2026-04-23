package observability

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type stepState struct {
	name       string
	status     string // running, succeeded, failed, skipped, cancelled, not-run
	iterations int
	doneSignal bool
	reason     string
}

type worktreeState struct {
	name      string
	branch    string
	status    string // running, succeeded, failed, cancelled
	steps     []*stepState
	stepIndex map[string]*stepState
	resumed   bool
	filesChanged int
	startedAt time.Time
	finishedAt time.Time
}

// SummaryBuilder builds a single markdown report per run and writes it at Close.
type SummaryBuilder struct {
	root string // report lives at <root>/reports/run-<runID>.md

	mu        sync.Mutex
	runID     string
	startedAt time.Time
	endedAt   time.Time
	cancelled bool
	worktrees map[string]*worktreeState
	order     []string
}

func NewSummaryBuilder(root string) *SummaryBuilder {
	return &SummaryBuilder{
		root:      root,
		worktrees: map[string]*worktreeState{},
	}
}

func (s *SummaryBuilder) Handle(ev Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ev.RunID != "" {
		s.runID = ev.RunID
	}

	switch ev.Kind {
	case KindRunStarted:
		s.startedAt = ev.Time
	case KindRunFinished:
		s.endedAt = ev.Time
	case KindCancelled:
		s.cancelled = true
		if wt := ev.Worktree; wt != "" {
			w := s.ensureWorktree(wt)
			w.status = "cancelled"
			for _, st := range w.steps {
				if st.status == "running" {
					st.status = "cancelled"
				} else if st.status == "" {
					st.status = "not-run"
				}
			}
		}
	case KindResumeWorktree:
		w := s.ensureWorktree(ev.Worktree)
		w.resumed = true
		if v, ok := ev.Payload["files_changed"]; ok {
			if n, ok := toInt(v); ok {
				w.filesChanged = n
			}
		}
	case KindCreateWorktree:
		w := s.ensureWorktree(ev.Worktree)
		w.resumed = false
	case KindStepStarted:
		w := s.ensureWorktree(ev.Worktree)
		st := s.ensureStep(w, ev.Step)
		st.status = "running"
	case KindStepFinished:
		w := s.ensureWorktree(ev.Worktree)
		st := s.ensureStep(w, ev.Step)
		st.status = "succeeded"
	case KindStepFailed:
		w := s.ensureWorktree(ev.Worktree)
		st := s.ensureStep(w, ev.Step)
		st.status = "failed"
		if v, ok := ev.Payload["reason"]; ok {
			st.reason = fmt.Sprintf("%v", v)
		}
		// any later step starts as not-run; we mark upon pipeline skip events.
	case KindStepSkipped:
		w := s.ensureWorktree(ev.Worktree)
		st := s.ensureStep(w, ev.Step)
		st.status = "not-run"
	case KindIterationFinished:
		w := s.ensureWorktree(ev.Worktree)
		st := s.ensureStep(w, ev.Step)
		st.iterations++
	case KindDoneSignalFound:
		w := s.ensureWorktree(ev.Worktree)
		st := s.ensureStep(w, ev.Step)
		st.doneSignal = true
	case KindWorktreeSucceeded:
		w := s.ensureWorktree(ev.Worktree)
		w.status = "succeeded"
	case KindWorktreeFailed:
		w := s.ensureWorktree(ev.Worktree)
		w.status = "failed"
	}
}

func (s *SummaryBuilder) ensureWorktree(name string) *worktreeState {
	if w, ok := s.worktrees[name]; ok {
		return w
	}
	w := &worktreeState{
		name:      name,
		branch:    name,
		status:    "running",
		stepIndex: map[string]*stepState{},
	}
	s.worktrees[name] = w
	s.order = append(s.order, name)
	return w
}

func (s *SummaryBuilder) ensureStep(w *worktreeState, name string) *stepState {
	if st, ok := w.stepIndex[name]; ok {
		return st
	}
	st := &stepState{name: name}
	w.stepIndex[name] = st
	w.steps = append(w.steps, st)
	return st
}

func (s *SummaryBuilder) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runID == "" {
		return nil
	}

	path := filepath.Join(s.root, "reports", fmt.Sprintf("run-%s.md", s.runID))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# ramo run %s\n\n", s.runID)
	if !s.startedAt.IsZero() {
		fmt.Fprintf(&b, "- started: %s\n", s.startedAt.Format(time.RFC3339))
	}
	if !s.endedAt.IsZero() {
		fmt.Fprintf(&b, "- ended: %s\n", s.endedAt.Format(time.RFC3339))
	}
	if s.cancelled {
		fmt.Fprintf(&b, "- cancelled: true\n")
	}
	fmt.Fprintln(&b)

	names := append([]string(nil), s.order...)
	sort.Strings(names)
	for _, name := range names {
		w := s.worktrees[name]
		fmt.Fprintf(&b, "## %s\n\n", w.branch)
		fmt.Fprintf(&b, "- status: %s\n", w.status)
		if w.resumed {
			fmt.Fprintf(&b, "- resumed: true (files changed: %d)\n", w.filesChanged)
		} else {
			fmt.Fprintf(&b, "- resumed: false\n")
		}
		fmt.Fprintf(&b, "- logs: logs/%s/%s/\n\n", s.runID, slugify(w.branch))
		fmt.Fprintln(&b, "| step | status | iterations | done signal |")
		fmt.Fprintln(&b, "| --- | --- | --- | --- |")
		for _, st := range w.steps {
			status := st.status
			if status == "" {
				status = "not-run"
			}
			sig := "no"
			if st.doneSignal {
				sig = "yes"
			}
			fmt.Fprintf(&b, "| %s | %s | %d | %s |\n", st.name, status, st.iterations, sig)
			if st.reason != "" {
				fmt.Fprintf(&b, "\n> %s: %s\n", st.name, st.reason)
			}
		}
		fmt.Fprintln(&b)
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
