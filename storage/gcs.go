package storage

import (
	"bytes"
	"context"
	"fmt"

	gcsstorage "cloud.google.com/go/storage"
)

// GCSBackend writes PDFs to a GCS bucket and returns the public object URL.
// The bucket must have uniform bucket-level access enabled and grant
// roles/storage.objectViewer to allUsers (or use signed URLs instead).
type GCSBackend struct {
	bucket string
	client *gcsstorage.Client
}

// NewGCSBackend creates a GCSBackend using Application Default Credentials.
func NewGCSBackend(ctx context.Context, bucket string) (*GCSBackend, error) {
	client, err := gcsstorage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: gcs: new client: %w", err)
	}
	return &GCSBackend{bucket: bucket, client: client}, nil
}

// Save implements StorageBackend.
func (b *GCSBackend) Save(ctx context.Context, filename string, data []byte) (string, error) {
	obj := b.client.Bucket(b.bucket).Object(filename)
	w := obj.NewWriter(ctx)
	w.ContentType = "application/pdf"

	if _, err := bytes.NewReader(data).WriteTo(w); err != nil {
		_ = w.Close()
		return "", fmt.Errorf("storage: gcs: write object: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("storage: gcs: close writer: %w", err)
	}

	downloadURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", b.bucket, filename)
	return downloadURL, nil
}
