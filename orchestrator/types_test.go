package orchestrator

import (
	"testing"
	"time"
)

func TestNormalizeDefaults(t *testing.T) {
	cfg := Config{
		Worktrees: []Worktree{
			{
				Branch: "feat/x",
				Steps: []Step{
					{Name: "implement"},
					{Name: "verify", MaxIterations: 3, RunOn: Host},
				},
			},
		},
	}

	got := normalizeConfig(cfg)

	if got.IdleTimeout != DefaultIdleTimeout {
		t.Errorf("IdleTimeout default: got %v want %v", got.IdleTimeout, DefaultIdleTimeout)
	}
	step1 := got.Worktrees[0].Steps[0]
	if step1.MaxIterations != 1 {
		t.Errorf("MaxIterations default: got %d", step1.MaxIterations)
	}
	if step1.RunOn != Container {
		t.Errorf("RunOn default: got %q", step1.RunOn)
	}
	step2 := got.Worktrees[0].Steps[1]
	if step2.MaxIterations != 3 {
		t.Errorf("explicit MaxIterations overridden: got %d", step2.MaxIterations)
	}
	if step2.RunOn != Host {
		t.Errorf("explicit RunOn overridden: got %q", step2.RunOn)
	}
}

func TestNormalizePreservesExplicitIdleTimeout(t *testing.T) {
	cfg := Config{IdleTimeout: 30 * time.Second}
	got := normalizeConfig(cfg)
	if got.IdleTimeout != 30*time.Second {
		t.Errorf("got %v", got.IdleTimeout)
	}
}
