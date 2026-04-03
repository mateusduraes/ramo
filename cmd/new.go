package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mateusduraes/ramo/cmux"
	"github.com/mateusduraes/ramo/config"
	"github.com/mateusduraes/ramo/worktree"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new <branch-name>",
	Short: "Create a new worktree with setup and cmux workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]

		dir, err := getWorkingDir()
		if err != nil {
			return err
		}

		cfg, err := config.Load(dir)
		if err != nil {
			return err
		}

		worktreesDir := filepath.Join(dir, cfg.WorktreesDir)
		wtPath := filepath.Join(worktreesDir, branch)

		// Fetch to know about remote branches
		fmt.Println("Fetching latest branches...")
		if err := worktree.Fetch(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: git fetch failed: %v\n", err)
		}

		// If worktree already exists, just open it
		if worktree.Exists(worktreesDir, branch) {
			fmt.Printf("Worktree %q already exists, opening it...\n", branch)
			openWorktreeWorkspace(branch, wtPath, cfg)
			fmt.Println("Done!")
			return nil
		}

		// 1. Create worktree
		fmt.Printf("Creating worktree for branch %q...\n", branch)
		if err := worktree.Add(dir, worktreesDir, branch); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}
		fmt.Printf("Worktree created at %s\n", wtPath)

		// 2. Copy files
		for _, file := range cfg.Copy {
			src := filepath.Join(dir, file)
			dst := filepath.Join(wtPath, file)

			if err := copyFile(src, dst); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not copy %s: %v\n", file, err)
				continue
			}
			fmt.Printf("Copied %s\n", file)
		}

		// 3. Run setup commands
		for _, command := range cfg.Setup {
			fmt.Printf("Running: %s\n", command)
			c := exec.Command("sh", "-c", command)
			c.Dir = wtPath
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("setup command failed: %s: %w", command, err)
			}
		}

		// 4. Create cmux workspace
		openWorktreeWorkspace(branch, wtPath, cfg)

		fmt.Println("Done!")
		return nil
	},
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func openWorktreeWorkspace(name, wtPath string, cfg *config.Config) {
	if cmux.IsAvailable() {
		fmt.Printf("Creating cmux workspace %q...\n", name)
		wsRef, err := cmux.NewWorkspace(name, wtPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		} else {
			fmt.Println("cmux workspace created")
			setupWorkspacePanes(wsRef, cfg)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Warning: cmux not found, skipping workspace creation")
	}
}

func setupWorkspacePanes(wsRef string, cfg *config.Config) {
	if cfg.Workspace == nil || len(cfg.Workspace.Panes) == 0 {
		return
	}

	panes := cfg.Workspace.Panes

	// Get the first surface (created with the workspace)
	surfaces, err := cmux.ListSurfaces(wsRef)
	if err != nil || len(surfaces) == 0 {
		fmt.Fprintf(os.Stderr, "Warning: could not list workspace surfaces: %v\n", err)
		return
	}
	firstSurface := surfaces[0]

	// Configure the first pane
	if panes[0].Command != "" {
		if err := cmux.Send(wsRef, firstSurface, panes[0].Command); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not send command to pane: %v\n", err)
		}
	}

	// Create and configure additional panes (split right)
	for i := 1; i < len(panes); i++ {
		pane := panes[i]
		surfaceRef, err := cmux.NewSplit(wsRef, "right")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not create pane: %v\n", err)
			continue
		}
		fmt.Printf("Created pane %d\n", i+1)

		if pane.Command != "" {
			if err := cmux.Send(wsRef, surfaceRef, pane.Command); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not send command to pane: %v\n", err)
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(newCmd)
}
