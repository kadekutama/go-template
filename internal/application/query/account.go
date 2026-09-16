package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/repository"
)

// AccountQueryServiceParams encapsulates dependencies for AccountQueryService.
type AccountQueryServiceParams struct {
	Accounts repository.AccountRepository
}

// AccountQueryService serves account reads for the edge directly from the account repository.
// Reads are strong (Get) or point-in-time pages (List);
// spend decisions use the E06-T13 strong balance read, never these views.
type AccountQueryService struct {
	accounts repository.AccountRepository
}

// NewAccountQueryService creates an encapsulated AccountQueryService with validated dependencies.
func NewAccountQueryService(params AccountQueryServiceParams) *AccountQueryService {
	return &AccountQueryService{
		accounts: params.Accounts,
	}
}

var _ port.AccountQueryUseCases = (*AccountQueryService)(nil)

// GetAccount returns one account. Strong read.
func (s *AccountQueryService) GetAccount(ctx context.Context, query port.AccountQuery) (port.AccountResult, error) {
	account, err := s.accounts.FindByID(ctx, query.TenantID, query.AccountID)
	if err != nil {
		return port.AccountResult{}, err
	}
	return port.AccountResult{Account: account}, nil
}

// ListAccounts pages tenant accounts. Point-in-time page.
func (s *AccountQueryService) ListAccounts(ctx context.Context, query port.AccountListQuery) (port.AccountPage, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}
	accounts, next, err := s.accounts.FindByTenant(ctx, query.TenantID, query.Cursor, limit)
	if err != nil {
		return port.AccountPage{}, err
	}
	return port.AccountPage{Accounts: accounts, NextCursor: next}, nil
}
