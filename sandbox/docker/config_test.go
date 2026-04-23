package docker

import "testing"

func TestNewRequiresDockerfileOrImage(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Errorf("expected error when neither Dockerfile nor Image is set")
	}
}

func TestNewRejectsBothDockerfileAndImage(t *testing.T) {
	if _, err := New(Config{Dockerfile: "x", Image: "y"}); err == nil {
		t.Errorf("expected error when both Dockerfile and Image are set")
	}
}

func TestNewAcceptsDockerfileOnly(t *testing.T) {
	if _, err := New(Config{Dockerfile: "x"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewAcceptsImageOnly(t *testing.T) {
	if _, err := New(Config{Image: "y"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
