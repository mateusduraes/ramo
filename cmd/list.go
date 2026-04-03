package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/mateusduraes/ramo/config"
	"github.com/mateusduraes/ramo/worktree"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List worktrees managed by ramo",
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

		entries, err := worktree.List(dir, worktreesDir)
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("No worktrees found")
			return nil
		}

		for _, e := range entries {
			fmt.Printf("  %s\t%s\n", e.Branch, e.Path)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
