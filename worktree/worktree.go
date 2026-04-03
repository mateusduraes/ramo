package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Entry struct {
	Path   string
	Branch string
}

func Exists(worktreesDir, branch string) bool {
	wtPath := filepath.Join(worktreesDir, branch)
	info, err := os.Stat(wtPath)
	return err == nil && info.IsDir()
}

func Fetch(repoDir string) error {
	cmd := exec.Command("git", "fetch")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func BranchExists(repoDir, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = repoDir
	return cmd.Run() == nil
}

func Add(repoDir, worktreesDir, branch string) error {
	wtPath := filepath.Join(worktreesDir, branch)

	var cmd *exec.Cmd
	if BranchExists(repoDir, branch) {
		cmd = exec.Command("git", "worktree", "add", wtPath, branch)
	} else {
		cmd = exec.Command("git", "worktree", "add", wtPath, "-b", branch)
	}

	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func Remove(repoDir, worktreesDir, branch string) error {
	wtPath := filepath.Join(worktreesDir, branch)
	cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func DeleteBranch(repoDir, branch string) error {
	cmd := exec.Command("git", "branch", "-D", branch)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func List(repoDir, worktreesDir string) ([]Entry, error) {
	// Resolve worktreesDir relative to repoDir if it's relative
	var absWorktreesDir string
	if filepath.IsAbs(worktreesDir) {
		absWorktreesDir = worktreesDir
	} else {
		absWorktreesDir = filepath.Join(repoDir, worktreesDir)
	}

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}

	var entries []Entry
	var currentPath string
	var currentBranch string

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			// If we have a pending entry, save it
			if currentPath != "" && currentBranch != "" {
				absCurrentPath := currentPath
				// Resolve symlinks to get the canonical path
				if resolved, err := filepath.EvalSymlinks(absCurrentPath); err == nil {
					absCurrentPath = resolved
				}
				absCheck := absWorktreesDir
				if resolved, err := filepath.EvalSymlinks(absCheck); err == nil {
					absCheck = resolved
				}
				if strings.HasPrefix(absCurrentPath, absCheck+string(filepath.Separator)) {
					entries = append(entries, Entry{Path: currentPath, Branch: currentBranch})
				}
			}
			currentPath = strings.TrimPrefix(line, "worktree ")
			currentBranch = ""
		} else if strings.HasPrefix(line, "branch ") {
			currentBranch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}

	// Don't forget the last entry
	if currentPath != "" && currentBranch != "" {
		absCurrentPath := currentPath
		// Resolve symlinks to get the canonical path
		if resolved, err := filepath.EvalSymlinks(absCurrentPath); err == nil {
			absCurrentPath = resolved
		}
		absCheck := absWorktreesDir
		if resolved, err := filepath.EvalSymlinks(absCheck); err == nil {
			absCheck = resolved
		}
		if strings.HasPrefix(absCurrentPath, absCheck+string(filepath.Separator)) {
			entries = append(entries, Entry{Path: currentPath, Branch: currentBranch})
		}
	}

	return entries, nil
}
