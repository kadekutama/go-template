package repository

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// PostingRepository commits immutable postings. Commit is the single atomic
// write path for postings + entries; adapters extend the same atomic unit to
// checkpoints and outbox rows in E07 without changing this signature.
type PostingRepository interface {
	// Commit atomically stores one posting with all its entries.
	// Strong write; fails on duplicate ID.
	Commit(ctx context.Context, posting entity.PostingData) error
	// FindByID returns one posting with entries by tenant + ID. Strong read.
	FindByID(ctx context.Context, tenant valueobject.TenantID, id valueobject.PostingID) (entity.PostingData, error)
	// FindByExternalReference returns the posting filed under a provider or
	// caller reference. Strong read.
	FindByExternalReference(ctx context.Context, tenant valueobject.TenantID, reference string) (entity.PostingData, error)
	// FindByAccount returns one page of postings touching an account.
	// Point-in-time page with opaque cursor.
	FindByAccount(ctx context.Context, tenant valueobject.TenantID, account valueobject.AccountID, cursor string, limit int) ([]entity.PostingData, string, error)
}
