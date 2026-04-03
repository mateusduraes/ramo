package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/mateusduraes/ramo/config"
	"github.com/mateusduraes/ramo/worktree"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open [branch-name]",
	Short: "Open an existing worktree in a cmux workspace",
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

		wtPath := filepath.Join(worktreesDir, branch)

		if !worktree.Exists(worktreesDir, branch) {
			return fmt.Errorf("worktree %q does not exist. Run 'ramo new %s' to create it", branch, branch)
		}

		fmt.Printf("Opening worktree %q...\n", branch)
		openWorktreeWorkspace(branch, wtPath, cfg)

		fmt.Println("Done!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(openCmd)
}
