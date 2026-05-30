package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"time"

	gcsstorage "cloud.google.com/go/storage"
	iamcredentials "google.golang.org/api/iamcredentials/v1"
)

// GCSBackend writes PDFs to a GCS bucket.
// The bucket must have uniform bucket-level access enabled (no public access);
// use SignURL to generate time-limited download links via IAM signing.
type GCSBackend struct {
	bucket      string
	client      *gcsstorage.Client
	serviceAcct string // SA email; empty = signing disabled
	iamSvc      *iamcredentials.Service
}

// NewGCSBackend creates a GCSBackend using Application Default Credentials.
// serviceAcct is the Cloud Run service account email used for signed URL generation;
// pass "" to disable signing (Save still works, but SignURL will return an error).
func NewGCSBackend(ctx context.Context, bucket, serviceAcct string) (*GCSBackend, error) {
	client, err := gcsstorage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: gcs: new client: %w", err)
	}
	b := &GCSBackend{bucket: bucket, client: client, serviceAcct: serviceAcct}
	if serviceAcct != "" {
		svc, err := iamcredentials.NewService(ctx)
		if err != nil {
			return nil, fmt.Errorf("storage: gcs: iam credentials client: %w", err)
		}
		b.iamSvc = svc
	}
	return b, nil
}

// Bucket returns the GCS bucket name.
func (b *GCSBackend) Bucket() string { return b.bucket }

// Save implements StorageBackend. It writes data under filename and returns the
// plain (private) GCS object URL. Use SignURL to get a downloadable link.
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

	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", b.bucket, filename), nil
}

// SignURL returns a V4 signed URL for the given filename valid for expiry duration.
// Requires the backend to be configured with a service account email and the SA
// must have roles/iam.serviceAccountTokenCreator granted on itself.
func (b *GCSBackend) SignURL(ctx context.Context, filename string, expiry time.Duration) (string, error) {
	if b.serviceAcct == "" || b.iamSvc == nil {
		return "", fmt.Errorf("storage: gcs: signed URLs require a service account email")
	}

	signBytes := func(payload []byte) ([]byte, error) {
		name := "projects/-/serviceAccounts/" + b.serviceAcct
		resp, err := b.iamSvc.Projects.ServiceAccounts.SignBlob(name, &iamcredentials.SignBlobRequest{
			Payload: base64.StdEncoding.EncodeToString(payload),
		}).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("sign blob: %w", err)
		}
		return base64.StdEncoding.DecodeString(resp.SignedBlob)
	}

	url, err := gcsstorage.SignedURL(b.bucket, filename, &gcsstorage.SignedURLOptions{
		GoogleAccessID: b.serviceAcct,
		Method:         "GET",
		Expires:        time.Now().Add(expiry),
		Scheme:         gcsstorage.SigningSchemeV4,
		SignBytes:      signBytes,
	})
	if err != nil {
		return "", fmt.Errorf("storage: gcs: sign url: %w", err)
	}
	return url, nil
}
