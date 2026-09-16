package port

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// OpenAccountRequest provisions one ledger account. Number uniqueness is an
// adapter concern; structural validity is enforced by the domain.
type OpenAccountRequest struct {
	TenantID       valueobject.TenantID
	LedgerID       valueobject.LedgerID
	Number         string
	Name           string
	Class          valueobject.AccountClass
	AssetCode      valueobject.AssetCode
	Purpose        string
	Metadata       map[string]string
	IdempotencyKey string
	Actor          string
}

// UpdateAccountRequest mutates name/purpose/metadata under optimistic locking.
type UpdateAccountRequest struct {
	TenantID        valueobject.TenantID
	AccountID       valueobject.AccountID
	Name            string
	Purpose         string
	Metadata        map[string]string
	ExpectedVersion int64
	Actor           string
}

// AccountLifecycleRequest freezes, unfreezes, or closes one account under
// optimistic locking. Reason is recorded for audit.
type AccountLifecycleRequest struct {
	TenantID        valueobject.TenantID
	AccountID       valueobject.AccountID
	ExpectedVersion int64
	Reason          string
	Actor           string
}

// AccountQuery reads one account by tenant + ID. Strong read.
type AccountQuery struct {
	TenantID  valueobject.TenantID
	AccountID valueobject.AccountID
}

// AccountListQuery pages the accounts of one tenant. Point-in-time page with
// opaque cursor.
type AccountListQuery struct {
	TenantID valueobject.TenantID
	Cursor   string
	Limit    int
}

// AccountResult carries the stored account with the cursor it was read at.
type AccountResult struct {
	Account entity.AccountData
	Cursor  string
}

// AccountPage is one page of accounts with an opaque cursor owned by the
// adapter. An empty NextCursor ends pagination.
type AccountPage struct {
	Accounts   []entity.AccountData
	NextCursor string
}

// AccountCommandUseCases defines the mutating operations on accounts. Strong writes.
type AccountCommandUseCases interface {
	// OpenAccount creates one ACTIVE account. Strong write.
	OpenAccount(ctx context.Context, req OpenAccountRequest) (AccountResult, error)
	// UpdateAccount mutates a non-closed account. Strong write.
	UpdateAccount(ctx context.Context, req UpdateAccountRequest) (AccountResult, error)
	// FreezeAccount blocks spending while preserving history. Strong write.
	FreezeAccount(ctx context.Context, req AccountLifecycleRequest) (AccountResult, error)
	// UnfreezeAccount restores a frozen account to ACTIVE. Strong write.
	UnfreezeAccount(ctx context.Context, req AccountLifecycleRequest) (AccountResult, error)
	// CloseAccount terminally closes an account with zero available. Strong write.
	CloseAccount(ctx context.Context, req AccountLifecycleRequest) (AccountResult, error)
}

// AccountQueryUseCases defines the read operations on accounts.
type AccountQueryUseCases interface {
	// GetAccount returns one account. Strong read.
	GetAccount(ctx context.Context, query AccountQuery) (AccountResult, error)
	// ListAccounts pages tenant accounts. Point-in-time page.
	ListAccounts(ctx context.Context, query AccountListQuery) (AccountPage, error)
}

// AccountUseCases is the composite account + tenant-provisioning read/write
// surface (implemented in E06-T02). Mutations run inside one UnitOfWork with
// durable idempotency; reads name their consistency per method.
type AccountUseCases interface {
	AccountCommandUseCases
	AccountQueryUseCases
}
