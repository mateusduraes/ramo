package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`{"setup": ["pnpm install"]}`)
	err := os.WriteFile(filepath.Join(dir, "ramo.json"), content, 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.WorktreesDir != ".worktrees" {
		t.Errorf("expected default worktreesDir '.worktrees', got %q", cfg.WorktreesDir)
	}
	if len(cfg.Setup) != 1 || cfg.Setup[0] != "pnpm install" {
		t.Errorf("expected setup [pnpm install], got %v", cfg.Setup)
	}
	if cfg.Copy == nil {
		t.Error("expected copy to be initialized, got nil")
	}
}

func TestLoadConfigMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing ramo.json")
	}
}

func TestCreateDefault(t *testing.T) {
	dir := t.TempDir()
	err := CreateDefault(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.WorktreesDir != ".worktrees" {
		t.Errorf("expected '.worktrees', got %q", cfg.WorktreesDir)
	}
	if len(cfg.Setup) != 0 {
		t.Errorf("expected empty setup, got %v", cfg.Setup)
	}
	if len(cfg.Copy) != 0 {
		t.Errorf("expected empty copy, got %v", cfg.Copy)
	}
}

func TestCreateDefaultAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "ramo.json"), []byte(`{}`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = CreateDefault(dir)
	if err == nil {
		t.Fatal("expected error when ramo.json already exists")
	}
}

func TestLoadConfigWithWorkspace(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`{
		"workspace": {
			"panes": [
				{},
				{"command": "claude"}
			]
		}
	}`)
	err := os.WriteFile(filepath.Join(dir, "ramo.json"), content, 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Workspace == nil {
		t.Fatal("expected workspace to be set")
	}
	if len(cfg.Workspace.Panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(cfg.Workspace.Panes))
	}
	if cfg.Workspace.Panes[0].Command != "" {
		t.Errorf("expected first pane no command, got %q", cfg.Workspace.Panes[0].Command)
	}
	if cfg.Workspace.Panes[1].Command != "claude" {
		t.Errorf("expected second pane command 'claude', got %q", cfg.Workspace.Panes[1].Command)
	}
}

func TestLoadConfigWithoutWorkspace(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`{"setup": ["echo hi"]}`)
	err := os.WriteFile(filepath.Join(dir, "ramo.json"), content, 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Workspace != nil {
		t.Error("expected workspace to be nil when not set")
	}
}
