package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoCmuxReferences(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read package dir: %v", err)
	}
	forbidden := []byte{'c', 'm', 'u', 'x'}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		if strings.Contains(string(data), string(forbidden)) {
			t.Errorf("%s contains a forbidden reference; worktree package must stay free of that dependency", name)
		}
	}
}

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	return dir
}

func TestAdd(t *testing.T) {
	repoDir := setupGitRepo(t)
	wtDir := filepath.Join(repoDir, ".worktrees")

	err := Add(repoDir, wtDir, "test-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(wtDir, "test-branch")); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}
}

func TestAddDuplicateBranch(t *testing.T) {
	repoDir := setupGitRepo(t)
	wtDir := filepath.Join(repoDir, ".worktrees")

	err := Add(repoDir, wtDir, "test-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = Add(repoDir, wtDir, "test-branch")
	if err == nil {
		t.Fatal("expected error for duplicate branch")
	}
}

func TestRemove(t *testing.T) {
	repoDir := setupGitRepo(t)
	wtDir := filepath.Join(repoDir, ".worktrees")

	err := Add(repoDir, wtDir, "test-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = Remove(repoDir, wtDir, "test-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(wtDir, "test-branch")); !os.IsNotExist(err) {
		t.Error("worktree directory still exists")
	}
}

func TestList(t *testing.T) {
	repoDir := setupGitRepo(t)
	wtDir := filepath.Join(repoDir, ".worktrees")

	err := Add(repoDir, wtDir, "branch-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = Add(repoDir, wtDir, "branch-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := List(repoDir, wtDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Branch
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "branch-a") || !strings.Contains(joined, "branch-b") {
		t.Errorf("expected branch-a and branch-b in list, got %v", names)
	}
}

func TestDeleteBranch(t *testing.T) {
	repoDir := setupGitRepo(t)
	wtDir := filepath.Join(repoDir, ".worktrees")

	err := Add(repoDir, wtDir, "test-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = Remove(repoDir, wtDir, "test-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = DeleteBranch(repoDir, "test-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmd := exec.Command("git", "branch", "--list", "test-branch")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Error("branch still exists after delete")
	}
}
