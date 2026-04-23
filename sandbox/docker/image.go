package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// ImageTag returns "ramo-sandbox:<sha256-hex>" where the hex is computed over
// the bytes at dockerfilePath. Editing the Dockerfile flips the tag so Docker
// builds a fresh image (its layer cache still makes the rebuild fast).
func ImageTag(dockerfilePath string) (string, error) {
	data, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return "", fmt.Errorf("read dockerfile: %w", err)
	}
	return ImageTagFromBytes(data), nil
}

// ImageTagFromBytes is the pure hashing helper used by ImageTag; exposed so
// tests don't have to touch the filesystem.
func ImageTagFromBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return "ramo-sandbox:" + hex.EncodeToString(sum[:])
}
