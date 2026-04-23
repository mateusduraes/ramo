package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/mateusduraes/ramo/worktree"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <branch>",
	Short: "Remove a worktree and its branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := getWorkingDir()
		if err != nil {
			return err
		}

		worktreesDir := filepath.Join(dir, defaultWorktreesDir)
		branch := args[0]

		fmt.Printf("Removing worktree %q...\n", branch)
		if err := worktree.Remove(dir, worktreesDir, branch); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}

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
