package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocalBackend writes PDFs to the local filesystem.
// Download URLs are constructed from BaseURL + "/download/" + filename.
type LocalBackend struct {
	Dir     string // directory to write PDFs; created if absent
	BaseURL string // e.g. "http://localhost:8081"
}

// Save implements StorageBackend.
func (b *LocalBackend) Save(_ context.Context, filename string, data []byte) (string, error) {
	if err := os.MkdirAll(b.Dir, 0o755); err != nil {
		return "", fmt.Errorf("storage: local: create dir: %w", err)
	}
	path := filepath.Join(b.Dir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("storage: local: write file: %w", err)
	}
	downloadURL := strings.TrimRight(b.BaseURL, "/") + "/download/" + filename
	return downloadURL, nil
}
