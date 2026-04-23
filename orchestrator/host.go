package orchestrator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/mateusduraes/ramo/sandbox"
)

// HostExecutor runs a command on the host. Used for Host-run steps.
// The orchestrator exposes this as an injectable seam so tests can
// stub out claude invocations.
type HostExecutor interface {
	Exec(ctx context.Context, opts sandbox.ExecOptions) (sandbox.ExecResult, error)
}

// defaultHostExecutor runs commands via os/exec, inheriting the host
// environment (plus any opts.Env overrides).
type defaultHostExecutor struct{}

func newHostExecutor() HostExecutor {
	return &defaultHostExecutor{}
}

func (h *defaultHostExecutor) Exec(ctx context.Context, opts sandbox.ExecOptions) (sandbox.ExecResult, error) {
	if len(opts.Cmd) == 0 {
		return sandbox.ExecResult{}, fmt.Errorf("host exec: empty command")
	}
	cmd := exec.CommandContext(ctx, opts.Cmd[0], opts.Cmd[1:]...)
	cmd.Dir = opts.Dir
	cmd.Env = append(os.Environ(), mapToEnvSlice(opts.Env)...)

	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}

	stdout := opts.Stdout
	stderr := opts.Stderr
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return sandbox.ExecResult{}, err
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		return sandbox.ExecResult{}, err
	}

	if err := cmd.Start(); err != nil {
		return sandbox.ExecResult{}, err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); forwardLines(outPipe, stdout) }()
	go func() { defer wg.Done(); forwardLines(errPipe, stderr) }()
	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return sandbox.ExecResult{ExitCode: exitErr.ExitCode()}, nil
		}
		return sandbox.ExecResult{}, err
	}
	return sandbox.ExecResult{ExitCode: 0}, nil
}

func mapToEnvSlice(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

func forwardLines(src io.Reader, dst io.Writer) {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		fmt.Fprintln(dst, scanner.Text())
	}
}
