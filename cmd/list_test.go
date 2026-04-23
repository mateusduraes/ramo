package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateusduraes/ramo/worktree"
)

func gitInit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "t@t.com"},
		{"git", "config", "user.name", "t"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestListEmptyExitsZero(t *testing.T) {
	dir := gitInit(t)
	t.Chdir(dir)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestListPrintsWorktreeLines(t *testing.T) {
	dir := gitInit(t)
	t.Chdir(dir)
	wtDir := filepath.Join(dir, ".worktrees")
	if err := worktree.Add(dir, wtDir, "feat/a"); err != nil {
		t.Fatal(err)
	}
	if err := worktree.Add(dir, wtDir, "feat/b"); err != nil {
		t.Fatal(err)
	}

	// Capture stdout via pipe.
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	rootCmd.SetArgs([]string{"list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	w.Close()
	var got bytes.Buffer
	got.ReadFrom(r)

	out := got.String()
	if !strings.Contains(out, "feat/a") || !strings.Contains(out, "feat/b") {
		t.Errorf("expected list to include both branches, got:\n%s", out)
	}
}

func TestRemoveDeletesWorktreeAndBranch(t *testing.T) {
	dir := gitInit(t)
	t.Chdir(dir)
	wtDir := filepath.Join(dir, ".worktrees")
	if err := worktree.Add(dir, wtDir, "feat/delme"); err != nil {
		t.Fatal(err)
	}

	rootCmd.SetArgs([]string{"remove", "feat/delme"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wtDir, "feat/delme")); !os.IsNotExist(err) {
		t.Errorf("worktree still exists: %v", err)
	}
	out, _ := exec.Command("git", "-C", dir, "branch", "--list", "feat/delme").Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("branch still exists: %q", out)
	}
}

func TestUnknownCommands(t *testing.T) {
	for _, sub := range []string{"new", "open"} {
		t.Run(sub, func(t *testing.T) {
			cmd, _, err := rootCmd.Find([]string{sub})
			if err == nil && cmd != rootCmd {
				t.Errorf("expected %q to be unknown, got %v", sub, cmd.Name())
			}
		})
	}
}
