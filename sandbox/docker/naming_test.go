package docker

import "testing"

func TestContainerName(t *testing.T) {
	cases := []struct {
		branch string
		runID  string
		want   string
	}{
		{"feat/signup", "r1", "ramo-feat-signup-r1"},
		{"main", "abc", "ramo-main-abc"},
		{"bugfix/USER/PANIC", "42", "ramo-bugfix-user-panic-42"},
		{"  weird//slashes  ", "x", "ramo-weird-slashes-x"},
	}
	for _, c := range cases {
		if got := ContainerName(c.branch, c.runID); got != c.want {
			t.Errorf("ContainerName(%q, %q): got %q want %q", c.branch, c.runID, got, c.want)
		}
	}
}
