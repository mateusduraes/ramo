package cmd

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed templates/Dockerfile templates/ramo.go.tmpl templates/open-pr.github.prompt.md templates/open-pr.gitlab.prompt.md templates/gitignore templates/implement.prompt.md
var templateFS embed.FS

var initProvider string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold ramo files in the current repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := getWorkingDir()
		if err != nil {
			return err
		}
		return runInit(dir, initProvider)
	},
}

func runInit(dir, provider string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "github"
	}
	if provider != "github" && provider != "gitlab" {
		return fmt.Errorf("unknown provider %q (expected github or gitlab)", provider)
	}

	targets := []struct {
		path     string
		template string
	}{
		{filepath.Join(dir, ".ramo", "Dockerfile"), "templates/Dockerfile"},
		{filepath.Join(dir, "ramo.go"), "templates/ramo.go.tmpl"},
		{filepath.Join(dir, ".ramo", "implement.prompt.md"), "templates/implement.prompt.md"},
		{filepath.Join(dir, ".ramo", "open-pr.prompt.md"), "templates/open-pr." + provider + ".prompt.md"},
		{filepath.Join(dir, ".ramo", ".gitignore"), "templates/gitignore"},
	}

	// Refuse if any file already exists. Fail before writing anything so init is atomic.
	var existing []string
	for _, t := range targets {
		if _, err := os.Stat(t.path); err == nil {
			existing = append(existing, t.path)
		}
	}
	if len(existing) > 0 {
		return fmt.Errorf("refusing to overwrite existing files: %s", strings.Join(existing, ", "))
	}

	for _, t := range targets {
		content, err := templateFS.ReadFile(t.template)
		if err != nil {
			return fmt.Errorf("read embedded template %s: %w", t.template, err)
		}
		if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(t.path), err)
		}
		// Strip the `//go:build ramo` build tag from the starter ramo.go so users
		// can `go run ./ramo.go` without an explicit build tag.
		if strings.HasSuffix(t.template, ".go.tmpl") {
			content = stripBuildTag(content)
		}
		if err := os.WriteFile(t.path, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", t.path, err)
		}
		fmt.Printf("wrote %s\n", t.path)
	}
	return nil
}

// stripBuildTag removes the leading `//go:build ramo` line and surrounding blank
// lines from the starter ramo.go. The tag exists only so `go build ./...` from
// the ramo repo skips the template.
func stripBuildTag(content []byte) []byte {
	const tag = "//go:build ramo\n"
	s := string(content)
	s = strings.Replace(s, tag, "", 1)
	s = strings.TrimLeft(s, "\n")
	return []byte(s)
}

func init() {
	initCmd.Flags().StringVar(&initProvider, "provider", "github", "code host provider for generated open-pr prompt (github or gitlab)")
	rootCmd.AddCommand(initCmd)
}
