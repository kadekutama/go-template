// Package port owns the application layer's inbound and outbound contracts.
// Inner layers own their ports: adapters in infrastructure implement these
// interfaces and depend inward. Application code imports domain packages and
// its own ports only — never infrastructure or protocol packages.
package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// NewEntry is one requested journal line: a positive minor-unit quantity on
// exactly one debit or credit side, in the posting's charge asset or a
// caller-declared settlement asset resolved by domain rules.
type NewEntry struct {
	AccountID   valueobject.AccountID
	Side        valueobject.Direction
	AmountMinor int64
	AssetCode   valueobject.AssetCode
}

// PostPostingCommand is the restricted core posting request. It carries one
// approved-template posting (construction rules live in the domain); unknown
// commit outcomes are surfaced, never retried blindly, and resolved through
// idempotent replay of IdempotencyKey.
type PostPostingCommand struct {
	TenantID          valueobject.TenantID
	LedgerID          valueobject.LedgerID
	Operation         string
	ExternalReference string
	Description       string
	Entries           []NewEntry
	IdempotencyKey    string
	Actor             string
}

// PostingResult is the durable outcome of one committed posting, including
// the ledger cursor for subsequent strong reads.
type PostingResult struct {
	PostingID valueobject.PostingID
	TenantID  valueobject.TenantID
	LedgerID  valueobject.LedgerID
	Cursor    string
}

// PostLedgerPosting is the inbound restricted posting use case. Implementations
// authorize the command, reserve idempotency, validate the template, commit
// exactly one UnitOfWork, and return the original response on identical replay.
type PostLedgerPosting interface {
	// Execute posts one template-approved posting atomically. Strong write.
	Execute(ctx context.Context, cmd PostPostingCommand) (PostingResult, error)
}

// GetPostingQuery reads one committed posting with its entries.
type GetPostingQuery struct {
	TenantID  valueobject.TenantID
	PostingID valueobject.PostingID
}

// PostingView is one committed posting with the cursor it was read at.
type PostingView struct {
	Posting entity.PostingData
	Cursor  string
}

// GetPosting is the inbound single-posting strong read.
type GetPosting interface {
	// Execute returns one posting with entries. Strong read.
	Execute(ctx context.Context, query GetPostingQuery) (PostingView, error)
}

// BalanceQuery reads the spendable balance of one account in one asset.
// Decision-making callers MUST pass the strong projection; cached figures
// are never acceptable here.
type BalanceQuery struct {
	TenantID  valueobject.TenantID
	LedgerID  valueobject.LedgerID
	AccountID valueobject.AccountID
	AssetCode valueobject.AssetCode
}

// BalanceView is the strongly consistent balance with its as-of time and the
// ledger cursor backing it. Extended balance dimensions live in E06-T02
// queries; this core view stays minimal and stable.
type BalanceView struct {
	AccountID      valueobject.AccountID
	AssetCode      valueobject.AssetCode
	AvailableMinor int64
	AsOf           time.Time
	Cursor         string
}

// GetBalance is the inbound strongly consistent balance read.
type GetBalance interface {
	// Execute returns the strong-projection balance. Strong read.
	Execute(ctx context.Context, query BalanceQuery) (BalanceView, error)
}

// EntriesQuery pages the journal lines of one account in cursor order.
type EntriesQuery struct {
	TenantID  valueobject.TenantID
	LedgerID  valueobject.LedgerID
	AccountID valueobject.AccountID
	Cursor    string
	Limit     int
}

// EntriesPage is one point-in-time page with an opaque cursor owned by the
// adapter. An empty NextCursor ends pagination.
type EntriesPage struct {
	Entries    []entity.Entry
	NextCursor string
}

// ListEntries is the inbound cursor-paged entry read.
type ListEntries interface {
	// Execute returns one page of entries. Point-in-time page; strong reads
	// use GetPosting or GetBalance instead.
	Execute(ctx context.Context, query EntriesQuery) (EntriesPage, error)
}
