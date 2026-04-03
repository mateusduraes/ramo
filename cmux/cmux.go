package cmux

import (
	"fmt"
	"os/exec"
	"strings"
)

func IsAvailable() bool {
	_, err := exec.LookPath("cmux")
	return err == nil
}

// NewWorkspace creates a workspace and returns its ref (e.g., "workspace:8").
func NewWorkspace(name, cwd string) (string, error) {
	cmd := exec.Command("cmux", "new-workspace", "--name", name, "--cwd", cwd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cmux new-workspace failed: %s", strings.TrimSpace(string(out)))
	}
	// Output format: "OK workspace:8"
	return parseRef(string(out), "workspace:"), nil
}

// ListSurfaces returns the surface refs for a workspace.
func ListSurfaces(workspaceRef string) ([]string, error) {
	cmd := exec.Command("cmux", "list-pane-surfaces", "--workspace", workspaceRef)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("cmux list-pane-surfaces failed: %s", strings.TrimSpace(string(out)))
	}

	var refs []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Lines like: "* surface:12  title  [selected]" or "  surface:13  title"
		fields := strings.Fields(line)
		for _, f := range fields {
			if strings.HasPrefix(f, "surface:") {
				refs = append(refs, f)
				break
			}
		}
	}
	return refs, nil
}

// NewSplit creates a new pane (side-by-side split) in the workspace and returns its surface ref.
func NewSplit(workspaceRef, direction string) (string, error) {
	cmd := exec.Command("cmux", "new-split", direction, "--workspace", workspaceRef)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cmux new-split failed: %s", strings.TrimSpace(string(out)))
	}
	// Output format: "OK surface:21 workspace:12"
	return parseRef(string(out), "surface:"), nil
}

// RenameTab renames a tab/surface in a workspace.
func RenameTab(workspaceRef, surfaceRef, name string) error {
	cmd := exec.Command("cmux", "rename-tab", "--workspace", workspaceRef, "--surface", surfaceRef, name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cmux rename-tab failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// Send sends a command (with newline) to a surface.
func Send(workspaceRef, surfaceRef, text string) error {
	cmd := exec.Command("cmux", "send", "--workspace", workspaceRef, "--surface", surfaceRef, text+`\n`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cmux send failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func CloseWorkspace(name string) error {
	listCmd := exec.Command("cmux", "list-workspaces")
	out, err := listCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cmux list-workspaces failed: %s", strings.TrimSpace(string(out)))
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, name) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				ref := fields[0]
				if ref == "*" && len(fields) > 1 {
					ref = fields[1]
				}
				closeCmd := exec.Command("cmux", "close-workspace", "--workspace", ref)
				closeOut, err := closeCmd.CombinedOutput()
				if err != nil {
					return fmt.Errorf("cmux close-workspace failed: %s", strings.TrimSpace(string(closeOut)))
				}
				return nil
			}
		}
	}

	return fmt.Errorf("cmux workspace %q not found", name)
}

// parseRef finds a ref like "workspace:8" or "surface:13" in output text.
func parseRef(output, prefix string) string {
	for _, field := range strings.Fields(output) {
		if strings.HasPrefix(field, prefix) {
			return field
		}
	}
	return ""
}
