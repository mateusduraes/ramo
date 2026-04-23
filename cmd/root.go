package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const defaultWorktreesDir = ".worktrees"

var rootCmd = &cobra.Command{
	Use:   "ramo",
	Short: "Orchestrate parallel AI coding agents across git worktrees",
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
