package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitFreshWritesAllFiles(t *testing.T) {
	dir := t.TempDir()
	if err := runInit(dir, "github"); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	for _, name := range []string{
		".ramo/Dockerfile",
		"ramo.go",
		".ramo/implement.prompt.md",
		".ramo/open-pr.prompt.md",
		".ramo/.gitignore",
	} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
		}
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".ramo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".ramo", "Dockerfile"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runInit(dir, "github")
	if err == nil {
		t.Fatalf("expected error when Dockerfile already exists")
	}
	if !strings.Contains(err.Error(), "Dockerfile") {
		t.Errorf("error should name existing file, got: %v", err)
	}

	// Verify no other files were written.
	if _, err := os.Stat(filepath.Join(dir, "ramo.go")); err == nil {
		t.Errorf("ramo.go should not have been written")
	}
}

func TestInitGithubProvider(t *testing.T) {
	dir := t.TempDir()
	if err := runInit(dir, "github"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".ramo", "open-pr.prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "gh pr create") {
		t.Errorf("expected gh pr create in prompt, got:\n%s", s)
	}
	if strings.Contains(s, "glab") {
		t.Errorf("github prompt should not mention glab")
	}
}

func TestInitGitlabProvider(t *testing.T) {
	dir := t.TempDir()
	if err := runInit(dir, "gitlab"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".ramo", "open-pr.prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "glab mr create") {
		t.Errorf("expected glab mr create in prompt, got:\n%s", s)
	}
	if strings.Contains(s, "gh pr create") {
		t.Errorf("gitlab prompt should not mention gh pr create")
	}
}

func TestInitDefaultProviderIsGithub(t *testing.T) {
	dir := t.TempDir()
	if err := runInit(dir, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".ramo", "open-pr.prompt.md"))
	if !strings.Contains(string(data), "gh pr create") {
		t.Errorf("expected default provider to be github")
	}
}

func TestInitUnknownProvider(t *testing.T) {
	dir := t.TempDir()
	err := runInit(dir, "bitbucket")
	if err == nil {
		t.Fatalf("expected error for unknown provider")
	}
}

func TestInitStripsBuildTag(t *testing.T) {
	dir := t.TempDir()
	if err := runInit(dir, "github"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "ramo.go"))
	if strings.Contains(string(data), "//go:build ramo") {
		t.Errorf("scaffolded ramo.go still contains the build tag; it should have been stripped")
	}
	if !strings.HasPrefix(string(data), "// Run this file") {
		t.Errorf("ramo.go should start with the doc comment, got:\n%s", string(data)[:100])
	}
}
