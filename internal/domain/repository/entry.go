package repository

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// EntryReader serves entry projections. There is intentionally no entry write
// port: entries persist only through PostingRepository.Commit.
type EntryReader interface {
	// FindByPosting returns all entries of one posting. Strong read.
	FindByPosting(ctx context.Context, tenant valueobject.TenantID, posting valueobject.PostingID) ([]entity.Entry, error)
	// FindByAccount returns one page of entries posted to an account.
	// Point-in-time page with opaque cursor; cacheable with the cursor.
	FindByAccount(ctx context.Context, tenant valueobject.TenantID, account valueobject.AccountID, cursor string, limit int) ([]entity.Entry, string, error)
}
