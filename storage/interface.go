// Package storage defines the StorageBackend interface used by the PDF generator.
// Use LocalBackend for local development and GCSBackend for Cloud Run deployments.
package storage

import "context"

// StorageBackend abstracts where generated PDF files are written.
type StorageBackend interface {
	// Save persists data under the given filename and returns the public download URL.
	Save(ctx context.Context, filename string, data []byte) (downloadURL string, err error)
}
