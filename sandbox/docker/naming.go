package docker

import (
	"strings"
)

// ContainerName returns a deterministic container name for the given branch
// and run ID: "ramo-<branch-slug>-<runID>". Branch slugs are formed by
// replacing any non-alphanumeric/underscore character with '-'.
func ContainerName(branch, runID string) string {
	return "ramo-" + slugifyBranch(branch) + "-" + runID
}

func slugifyBranch(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			prevDash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
