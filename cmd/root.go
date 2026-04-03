package cmd

import (
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/mateusduraes/ramo/worktree"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ramo",
	Short: "A git worktree manager with cmux integration",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getWorkingDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return dir, nil
}

func selectWorktree(repoDir, worktreesDir string) (string, error) {
	entries, err := worktree.List(repoDir, worktreesDir)
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %w", err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no worktrees found")
	}

	branches := make([]string, len(entries))
	for i, e := range entries {
		branches[i] = e.Branch
	}

	prompt := promptui.Select{
		Label: "Select a worktree",
		Items: branches,
	}

	_, selected, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("selection cancelled")
	}

	return selected, nil
}
