package observability

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileLogger writes per-(worktree,step) log files at:
//
//	<root>/logs/<runID>/<worktree-slug>/<step-slug>.log
//
// Stdout and stderr lines from each iteration are appended with clear
// iteration-boundary markers.
type FileLogger struct {
	root string // directory where "logs/<runID>/..." will live (e.g. ".ramo")

	mu    sync.Mutex
	files map[string]*os.File // keyed by "<worktree>/<step>"
	// iteration tracks the last-known iteration per (worktree,step) for boundary markers.
	iteration map[string]int
}

func NewFileLogger(root string) *FileLogger {
	return &FileLogger{
		root:      root,
		files:     map[string]*os.File{},
		iteration: map[string]int{},
	}
}

func (f *FileLogger) Handle(ev Event) {
	switch ev.Kind {
	case KindIterationStarted:
		f.writeBoundary(ev)
	case KindStdoutLine, KindStderrLine:
		f.writeLine(ev)
	case KindSetupStarted:
		f.writeSetupBoundary(ev, "start")
	case KindSetupFinished:
		f.writeSetupBoundary(ev, "finish")
	}
}

func (f *FileLogger) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, fh := range f.files {
		_ = fh.Close()
	}
	f.files = nil
	return nil
}

func (f *FileLogger) logPath(ev Event) string {
	return filepath.Join(f.root, "logs", ev.RunID, slugify(ev.Worktree), slugify(ev.Step)+".log")
}

func (f *FileLogger) openFile(ev Event) (*os.File, error) {
	if ev.RunID == "" || ev.Worktree == "" || ev.Step == "" {
		return nil, fmt.Errorf("missing runID, worktree, or step")
	}
	key := ev.Worktree + "/" + ev.Step
	if fh, ok := f.files[key]; ok {
		return fh, nil
	}
	path := f.logPath(ev)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	f.files[key] = fh
	return fh, nil
}

func (f *FileLogger) writeBoundary(ev Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fh, err := f.openFile(ev)
	if err != nil {
		return
	}
	iter := 0
	if v, ok := ev.Payload["iteration"]; ok {
		switch n := v.(type) {
		case int:
			iter = n
		case float64:
			iter = int(n)
		}
	}
	key := ev.Worktree + "/" + ev.Step
	f.iteration[key] = iter
	_, _ = fmt.Fprintf(fh, "\n===== iteration %d =====\n", iter)
}

func (f *FileLogger) writeSetupBoundary(ev Event, phase string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Setup has no step name; synthesize one.
	setupEv := ev
	if setupEv.Step == "" {
		setupEv.Step = "setup"
	}
	fh, err := f.openFile(setupEv)
	if err != nil {
		return
	}
	cmd := ""
	if v, ok := ev.Payload["command"]; ok {
		cmd = fmt.Sprintf("%v", v)
	}
	_, _ = fmt.Fprintf(fh, "\n===== setup %s: %s =====\n", phase, cmd)
}

func (f *FileLogger) writeLine(ev Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// For Setup stdout/stderr lines without a step name, tag them under "setup".
	logEv := ev
	if logEv.Step == "" {
		logEv.Step = "setup"
	}
	fh, err := f.openFile(logEv)
	if err != nil {
		return
	}
	line := ""
	if v, ok := ev.Payload["line"]; ok {
		line = fmt.Sprintf("%v", v)
	}
	prefix := ""
	if ev.Kind == KindStderrLine {
		prefix = "[stderr] "
	}
	_, _ = fmt.Fprintf(fh, "%s%s\n", prefix, line)
}

func slugify(s string) string {
	s = strings.TrimSpace(s)
	out := strings.Builder{}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '-', r == '_', r == '.':
			out.WriteRune(r)
		default:
			out.WriteRune('-')
		}
	}
	return out.String()
}
