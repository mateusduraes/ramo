// Package docker implements sandbox.Sandbox by shelling out to the local
// `docker` CLI. Images are tagged by a content hash of the Dockerfile so edits
// trigger a rebuild (Docker's layer cache makes subsequent rebuilds fast).
// Each worktree runs in a single long-lived container (CMD ["sleep",
// "infinity"]) with the host worktree bind-mounted at /workspace.
package docker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mateusduraes/ramo/sandbox"
)

// AuthEnvNames lists the host env var names the sandbox SHALL forward to
// containers if set. Orchestrator clients may extend this list via Config.Env.
var AuthEnvNames = []string{
	"ANTHROPIC_API_KEY",
	"CLAUDE_CODE_OAUTH_TOKEN",
	"ANTHROPIC_BASE_URL",
	"AWS_REGION",
	"AWS_PROFILE",
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"AWS_SESSION_TOKEN",
}

// Config configures docker.New.
type Config struct {
	// Dockerfile is the path to the Dockerfile. Exactly one of Dockerfile or
	// Image must be set. When Dockerfile is used, New builds the image on
	// first start and tags it by content hash.
	Dockerfile string

	// Image overrides the Dockerfile path: when set, the sandbox pulls/uses
	// this image directly and does not build anything.
	Image string

	// ContextDir is the Docker build context directory. Defaults to the
	// directory containing the Dockerfile.
	ContextDir string
}

// New returns a docker-backed Sandbox. It returns an error if the config is invalid.
func New(cfg Config) (sandbox.Sandbox, error) {
	if cfg.Dockerfile == "" && cfg.Image == "" {
		return nil, errors.New("docker.Config: Dockerfile or Image must be set")
	}
	if cfg.Dockerfile != "" && cfg.Image != "" {
		return nil, errors.New("docker.Config: Dockerfile and Image are mutually exclusive")
	}
	return &dockerSandbox{cfg: cfg}, nil
}

type dockerSandbox struct {
	cfg Config

	mu    sync.Mutex
	built bool
	tag   string
}

func (d *dockerSandbox) Start(ctx context.Context, cfg sandbox.StartConfig) (sandbox.Instance, error) {
	tag, err := d.ensureImage(ctx)
	if err != nil {
		return nil, err
	}

	name := ContainerName(cfg.Worktree, cfg.RunID)
	absWorktree, err := filepath.Abs(cfg.HostPath)
	if err != nil {
		return nil, fmt.Errorf("abs worktree path: %w", err)
	}

	args := []string{
		"run", "-d",
		"--name", name,
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"--workdir", "/workspace",
		"-v", absWorktree + ":/workspace",
	}

	env := mergedEnv(cfg.Env)
	for _, k := range sortedKeys(env) {
		args = append(args, "--env", k+"="+env[k])
	}

	args = append(args, tag, "sleep", "infinity")

	out, err := runCommand(ctx, "docker", args, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("docker run: %s", out)
	}

	return &dockerInstance{name: name}, nil
}

func (d *dockerSandbox) ensureImage(ctx context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cfg.Image != "" {
		return d.cfg.Image, nil
	}
	if d.built {
		return d.tag, nil
	}
	tag, err := ImageTag(d.cfg.Dockerfile)
	if err != nil {
		return "", err
	}
	contextDir := d.cfg.ContextDir
	if contextDir == "" {
		contextDir = filepath.Dir(d.cfg.Dockerfile)
	}
	absDockerfile, err := filepath.Abs(d.cfg.Dockerfile)
	if err != nil {
		return "", fmt.Errorf("abs dockerfile: %w", err)
	}
	absContext, err := filepath.Abs(contextDir)
	if err != nil {
		return "", fmt.Errorf("abs context: %w", err)
	}

	args := []string{"build", "-f", absDockerfile, "-t", tag, absContext}
	out, err := runCommand(ctx, "docker", args, nil, os.Stderr, os.Stderr)
	if err != nil {
		return "", fmt.Errorf("docker build: %s", out)
	}
	d.built = true
	d.tag = tag
	return tag, nil
}

type dockerInstance struct {
	name string

	mu      sync.Mutex
	stopped bool
}

func (i *dockerInstance) Exec(ctx context.Context, opts sandbox.ExecOptions) (sandbox.ExecResult, error) {
	args := []string{"exec", "-i"}
	if opts.Dir != "" {
		args = append(args, "--workdir", opts.Dir)
	}
	env := opts.Env
	for _, k := range sortedKeys(env) {
		args = append(args, "--env", k+"="+env[k])
	}
	args = append(args, i.name)
	args = append(args, opts.Cmd...)

	stdout := opts.Stdout
	stderr := opts.Stderr
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
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

func (i *dockerInstance) Stop(ctx context.Context) error {
	i.mu.Lock()
	if i.stopped {
		i.mu.Unlock()
		return nil
	}
	i.stopped = true
	i.mu.Unlock()

	out, err := runCommand(ctx, "docker", []string{"rm", "-f", i.name}, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("docker rm: %s", out)
	}
	return nil
}

func mergedEnv(explicit map[string]string) map[string]string {
	out := map[string]string{}
	for _, name := range AuthEnvNames {
		if v, ok := os.LookupEnv(name); ok {
			out[name] = v
		}
	}
	for k, v := range explicit {
		out[k] = v
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple insertion sort, small slices.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

func forwardLines(src io.Reader, dst io.Writer) {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		fmt.Fprintln(dst, scanner.Text())
	}
}

func runCommand(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var combined strings.Builder
	if stdout == nil {
		cmd.Stdout = &combined
	} else {
		cmd.Stdout = io.MultiWriter(stdout, &combined)
	}
	if stderr == nil {
		cmd.Stderr = &combined
	} else {
		cmd.Stderr = io.MultiWriter(stderr, &combined)
	}
	cmd.Stdin = stdin
	err := cmd.Run()
	return strings.TrimSpace(combined.String()), err
}
