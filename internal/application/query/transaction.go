package query

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// statementScanPages caps the pages scanned per statement: statements stay
// bounded and fail loudly instead of scanning forever.
const statementScanPages = 20

// Statement is one account statement: the account with its committed entries
// in the inclusive window.
type Statement struct {
	Account entity.AccountData
	Entries []entity.Entry
	From    time.Time
	To      time.Time
}

// TransactionQueryServiceParams encapsulates dependencies for TransactionQueryService.
type TransactionQueryServiceParams struct {
	Posting  port.GetPosting
	Query    port.PostingQuery
	Accounts repository.AccountRepository
}

// TransactionQueryService serves generic transaction reads for the edge by
// delegating to the core posting query. Strong reads; the DTO layer shapes
// the edge representation.
type TransactionQueryService struct {
	posting  port.GetPosting
	query    port.PostingQuery
	accounts repository.AccountRepository
}

// NewTransactionQueryService creates an encapsulated TransactionQueryService with validated dependencies.
func NewTransactionQueryService(params TransactionQueryServiceParams) *TransactionQueryService {
	return &TransactionQueryService{
		posting:  params.Posting,
		query:    params.Query,
		accounts: params.Accounts,
	}
}

// GetTransaction returns one committed transaction with entries. Strong read.
func (s *TransactionQueryService) GetTransaction(ctx context.Context, tenant valueobject.TenantID, id valueobject.PostingID) (port.PostingView, error) {
	return s.posting.Execute(ctx, port.GetPostingQuery{TenantID: tenant, PostingID: id})
}

// ListTransactions pages committed transactions by filter. Point-in-time page.
func (s *TransactionQueryService) ListTransactions(ctx context.Context, filter port.PostingFilter) (port.PostingSearchPage, error) {
	return s.query.Search(ctx, filter)
}

// GetStatement returns one account with its committed entries in the
// inclusive window, newest last. Strong read, bounded scan.
func (s *TransactionQueryService) GetStatement(ctx context.Context, tenant valueobject.TenantID, ledger valueobject.LedgerID, accountID valueobject.AccountID, from, to time.Time) (Statement, error) {
	account, err := s.accounts.FindByID(ctx, tenant, accountID)
	if err != nil {
		return Statement{}, err
	}
	if account.LedgerID != ledger {
		return Statement{}, entity.NewError("LEDGER_MISMATCH", "account belongs to a different ledger")
	}
	entries, err := s.scanStatementEntries(ctx, tenant, accountID, from, to)
	if err != nil {
		return Statement{}, err
	}
	return Statement{Account: account, Entries: entries, From: from, To: to}, nil
}

// scanStatementEntries pages committed postings in the window and gathers
// the account's lines.
func (s *TransactionQueryService) scanStatementEntries(ctx context.Context, tenant valueobject.TenantID, accountID valueobject.AccountID, from, to time.Time) ([]entity.Entry, error) {
	var out []entity.Entry
	cursor := ""
	for pages := 0; ; pages++ {
		if pages >= statementScanPages {
			return nil, entity.NewError("STATEMENT_TOO_LARGE", "statement exceeds the bounded scan")
		}
		page, err := s.query.Search(ctx, port.PostingFilter{TenantID: tenant, Cursor: cursor, Limit: 500})
		if err != nil {
			return nil, err
		}
		for _, posting := range page.Postings {
			if posting.RecordedAt.Before(from) || posting.RecordedAt.After(to) {
				continue
			}
			for _, entry := range posting.Entries {
				if entry.AccountID == accountID {
					out = append(out, entry)
				}
			}
		}
		if page.NextCursor == "" || page.NextCursor == cursor {
			return out, nil
		}
		cursor = page.NextCursor
	}
}
