package orchestrator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mateusduraes/ramo/worktree"
)

// ensureWorktree creates the worktree if it does not exist, or reuses it if it
// does. Returns (exists bool) indicating whether the worktree was already
// present (true = resumed).
func (r *runner) ensureWorktree(w Worktree) (resumed bool, err error) {
	if worktree.Exists(r.cfg.WorktreesDir, w.Branch) {
		return true, nil
	}
	if err := worktree.Add(r.cfg.RepoDir, r.cfg.WorktreesDir, w.Branch); err != nil {
		return false, fmt.Errorf("add worktree %q: %w", w.Branch, err)
	}
	return false, nil
}

// worktreePath returns the absolute path to the worktree for branch.
func (r *runner) worktreePath(branch string) string {
	return filepath.Join(r.cfg.WorktreesDir, branch)
}

// runCopies applies each FileCopy for a freshly created worktree.
// Skipped on resume.
func (r *runner) runCopies(w Worktree) error {
	wtRoot := r.worktreePath(w.Branch)
	for _, c := range w.Copy {
		from := resolvePath(r.cfg.RepoDir, c.From)
		to := c.To
		if !filepath.IsAbs(to) {
			to = filepath.Join(wtRoot, to)
		}
		if err := copyFile(from, to); err != nil {
			return fmt.Errorf("copy %s -> %s: %w", from, to, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
