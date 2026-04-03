package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mateusduraes/ramo/cmux"
	"github.com/mateusduraes/ramo/config"
	"github.com/mateusduraes/ramo/worktree"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [branch-name]",
	Short: "Remove a worktree and its cmux workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := getWorkingDir()
		if err != nil {
			return err
		}

		cfg, err := config.Load(dir)
		if err != nil {
			return err
		}

		worktreesDir := filepath.Join(dir, cfg.WorktreesDir)

		var branch string
		if len(args) == 1 {
			branch = args[0]
		} else {
			branch, err = selectWorktree(dir, worktreesDir)
			if err != nil {
				return err
			}
		}

		// 1. Close cmux workspace
		if cmux.IsAvailable() {
			fmt.Printf("Closing cmux workspace %q...\n", branch)
			if err := cmux.CloseWorkspace(branch); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			} else {
				fmt.Println("cmux workspace closed")
			}
		}

		// 2. Remove worktree
		fmt.Printf("Removing worktree %q...\n", branch)
		if err := worktree.Remove(dir, worktreesDir, branch); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}

		// 3. Delete branch
		fmt.Printf("Deleting branch %q...\n", branch)
		if err := worktree.DeleteBranch(dir, branch); err != nil {
			return fmt.Errorf("failed to delete branch: %w", err)
		}

		fmt.Println("Done!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
