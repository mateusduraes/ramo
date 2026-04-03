package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const FileName = "ramo.json"

type Pane struct {
	Command string `json:"command,omitempty"`
}

type Workspace struct {
	Panes []Pane `json:"panes"`
}

type Config struct {
	WorktreesDir string     `json:"worktreesDir"`
	Setup        []string   `json:"setup"`
	Copy         []string   `json:"copy"`
	Workspace    *Workspace `json:"workspace,omitempty"`
}

func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ramo.json not found. Run 'ramo init' to create one")
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse ramo.json: %w", err)
	}

	if cfg.WorktreesDir == "" {
		cfg.WorktreesDir = ".worktrees"
	}
	if cfg.Setup == nil {
		cfg.Setup = []string{}
	}
	if cfg.Copy == nil {
		cfg.Copy = []string{}
	}

	return cfg, nil
}

func CreateDefault(dir string) error {
	path := filepath.Join(dir, FileName)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("ramo.json already exists")
	}

	cfg := Config{
		WorktreesDir: ".worktrees",
		Setup:        []string{},
		Copy:         []string{},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, append(data, '\n'), 0644)
}
