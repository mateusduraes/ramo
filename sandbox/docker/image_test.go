package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageTagChangesWithContent(t *testing.T) {
	tag1 := ImageTagFromBytes([]byte("FROM debian:slim\nRUN apt-get install -y git"))
	tag2 := ImageTagFromBytes([]byte("FROM debian:slim\nRUN apt-get install -y git chromium"))
	if tag1 == tag2 {
		t.Fatalf("expected different tags for different content")
	}
	if !strings.HasPrefix(tag1, "ramo-sandbox:") || !strings.HasPrefix(tag2, "ramo-sandbox:") {
		t.Errorf("tags must start with ramo-sandbox:")
	}
}

func TestImageTagFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(path, []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tag, err := ImageTag(path)
	if err != nil {
		t.Fatal(err)
	}
	if tag != ImageTagFromBytes([]byte("FROM alpine\n")) {
		t.Errorf("file-based tag should equal byte-based tag for same content")
	}
}

func TestImageTagDeterministic(t *testing.T) {
	a := ImageTagFromBytes([]byte("same"))
	b := ImageTagFromBytes([]byte("same"))
	if a != b {
		t.Errorf("expected deterministic tag; got %q and %q", a, b)
	}
}
