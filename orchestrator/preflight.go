package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mateusduraes/ramo/observability"
	"github.com/mateusduraes/ramo/worktree"
)

// preflight validates config and environment before any sandbox is started.
// Returns the first error encountered. Emits a warning event (but does not
// error) when no recognized auth env vars are set.
func preflight(ctx context.Context, cfg Config, runID string, emitter *observability.Emitter) error {
	for _, w := range cfg.Worktrees {
		if w.Branch == "" {
			return errors.New("worktree has empty Branch")
		}
		for _, s := range w.Steps {
			if s.Name == "" {
				return fmt.Errorf("worktree %q: step has empty Name", w.Branch)
			}
			if s.PromptFile == "" {
				return fmt.Errorf("worktree %q step %q: PromptFile is required", w.Branch, s.Name)
			}
			path := resolvePath(cfg.RepoDir, s.PromptFile)
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("worktree %q step %q: prompt file not found: %s", w.Branch, s.Name, path)
			}
		}

		if !worktree.Exists(cfg.WorktreesDir, w.Branch) {
			for _, c := range w.Copy {
				from := resolvePath(cfg.RepoDir, c.From)
				if _, err := os.Stat(from); err != nil {
					return fmt.Errorf("worktree %q: copy source not found: %s", w.Branch, from)
				}
			}
		}
	}

	if !cfg.SkipPreflightEnv {
		binary := cfg.ClaudeBinary
		if binary == "" {
			binary = "claude"
		}
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("claude CLI not found on host PATH: %w", err)
		}

		if hasContainerSteps(cfg) {
			if err := checkDockerDaemon(ctx); err != nil {
				return fmt.Errorf("docker daemon unreachable: %w", err)
			}
		}
	}

	if !hasAnyAuthEnv() {
		emitter.Emit(observability.Event{
			RunID:   runID,
			Kind:    observability.KindPreflightWarning,
			Payload: map[string]any{"message": "no recognized auth env vars are set; claude may fail"},
		})
	}

	return nil
}

func hasContainerSteps(cfg Config) bool {
	for _, w := range cfg.Worktrees {
		for _, s := range w.Steps {
			if s.RunOn == Container {
				return true
			}
		}
	}
	return false
}

func checkDockerDaemon(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// authEnvNames lists the host env vars that ramo forwards to containers.
// Duplicated from sandbox/docker to avoid import cycles / coupling.
var authEnvNames = []string{
	"ANTHROPIC_API_KEY",
	"CLAUDE_CODE_OAUTH_TOKEN",
	"ANTHROPIC_BASE_URL",
	"AWS_REGION",
	"AWS_PROFILE",
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"AWS_SESSION_TOKEN",
}

func hasAnyAuthEnv() bool {
	for _, name := range authEnvNames {
		if v := os.Getenv(name); v != "" {
			return true
		}
	}
	return false
}

func resolvePath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if baseDir == "" {
		baseDir = "."
	}
	return filepath.Join(baseDir, path)
}
