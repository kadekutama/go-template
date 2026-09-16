package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// StoredObject carries object bytes with integrity metadata for reports,
// statements, and dispute evidence.
type StoredObject struct {
	// Content carries the object bytes.
	Content []byte
	// ContentType is the MIME type set at Put time.
	ContentType string
	// Checksum is the hex SHA-256 recorded at Put time.
	Checksum string
}

// ObjectStorage is the blob boundary (MinIO/S3-style in E10) for generated
// reports, ingested statements, and evidence files. Keys are tenant-scoped;
// financial facts themselves live in the ledger, never here alone. Signed
// URLs expire and are issued per delivery, never stored.
type ObjectStorage interface {
	// Put stores one object under key with integrity checksum. Strong write.
	Put(ctx context.Context, tenant valueobject.TenantID, key string, object StoredObject) error
	// Get returns one object by key. Strong read.
	Get(ctx context.Context, tenant valueobject.TenantID, key string) (StoredObject, error)
	// Delete removes one object by key. Idempotent: missing keys succeed.
	Delete(ctx context.Context, tenant valueobject.TenantID, key string) error
	// SignedURL issues a short-lived download URL for one delivery. The URL
	// itself is never persisted.
	SignedURL(ctx context.Context, tenant valueobject.TenantID, key string, ttl time.Duration) (string, error)
}
