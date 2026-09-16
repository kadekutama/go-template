package query

import (
	"context"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// balancePageSize bounds one entry-scan page; materialized projections
// (later adapter work) replace scanning for hot accounts.
const balancePageSize = 500

// BalanceService derives available balances from committed entries minus
// ACTIVE holds. It reads the strong projection only; cached or replica
// figures MUST NOT back spend decisions. Signing follows the account's
// normal side (ledger-core §5): normal-debit accounts read debits minus
// credits, normal-credit accounts the reverse.
// BalanceServiceParams encapsulates dependencies for BalanceService.
type BalanceServiceParams struct {
	Accounts repository.AccountRepository
	Entries  repository.EntryReader
	Holds    repository.HoldRepository
	Clock    port.Clock
}

// BalanceService derives available balances from committed entries minus
// ACTIVE holds. It reads the strong projection only; cached or replica
// figures MUST NOT back spend decisions. Signing follows the account's
// normal side (ledger-core §5): normal-debit accounts read debits minus
// credits, normal-credit accounts the reverse.
type BalanceService struct {
	accounts repository.AccountRepository
	entries  repository.EntryReader
	holds    repository.HoldRepository
	clock    port.Clock
}

// NewBalanceService creates an encapsulated BalanceService with validated dependencies.
func NewBalanceService(params BalanceServiceParams) *BalanceService {
	return &BalanceService{
		accounts: params.Accounts,
		entries:  params.Entries,
		holds:    params.Holds,
		clock:    params.Clock,
	}
}

var _ port.GetBalance = (*BalanceService)(nil)

// Execute returns the available balance for one account + asset at the read
// time. Strong read. The returned Cursor is empty: point reads carry no page
// position (ledger cursors arrive with the E07 read models).
func (s *BalanceService) Execute(ctx context.Context, query port.BalanceQuery) (port.BalanceView, error) {
	if err := validateBalanceQuery(query); err != nil {
		return port.BalanceView{}, err
	}
	account, err := s.accounts.FindByID(ctx, query.TenantID, query.AccountID)
	if err != nil {
		return port.BalanceView{}, err
	}
	if account.LedgerID != query.LedgerID {
		return port.BalanceView{}, entity.NewError("LEDGER_MISMATCH", "account belongs to a different ledger")
	}
	posted, err := s.scanPosted(ctx, query, account.Class.NormalSide())
	if err != nil {
		return port.BalanceView{}, err
	}
	now := s.clock.Now().UTC()
	held, err := s.sumActiveHolds(ctx, query, now)
	if err != nil {
		return port.BalanceView{}, err
	}
	available, ok := service.CheckedSub(posted, held)
	if !ok {
		return port.BalanceView{}, entity.NewError("BALANCE_OVERFLOW", "balance computation overflowed")
	}
	return port.BalanceView{
		AccountID:      query.AccountID,
		AssetCode:      query.AssetCode,
		AvailableMinor: available,
		AsOf:           now,
	}, nil
}

// validateBalanceQuery checks the query envelope.
func validateBalanceQuery(query port.BalanceQuery) error {
	if strings.TrimSpace(query.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(query.LedgerID.String()) == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if strings.TrimSpace(query.AccountID.String()) == "" {
		return entity.NewError("BALANCE_ACCOUNT_REQUIRED", "balance requires an account id")
	}
	if strings.TrimSpace(string(query.AssetCode)) == "" {
		return entity.NewError("BALANCE_ASSET_REQUIRED", "balance requires an asset code")
	}
	return nil
}

// scanPosted sums the normal-side net over committed entries for the
// account + asset, following opaque cursors until exhausted. An unchanged
// cursor ends the scan defensively so a faulty adapter cannot loop it.
func (s *BalanceService) scanPosted(ctx context.Context, query port.BalanceQuery, normal valueobject.Direction) (int64, error) {
	var total int64
	cursor := ""
	for {
		entries, next, err := s.entries.FindByAccount(ctx, query.TenantID, query.AccountID, cursor, balancePageSize)
		if err != nil {
			return 0, err
		}
		for _, entry := range entries {
			signed, include, signErr := signedEntryAmount(entry, query.AssetCode, normal)
			if signErr != nil {
				return 0, signErr
			}
			if !include {
				continue
			}
			summed, ok := service.CheckedAdd(total, signed)
			if !ok {
				return 0, entity.NewError("BALANCE_OVERFLOW", "balance computation overflowed")
			}
			total = summed
		}
		if next == "" || next == cursor {
			return total, nil
		}
		cursor = next
	}
}

// signedEntryAmount maps one entry to its signed contribution for asset on
// the account's normal side. The second return skips other-asset entries;
// negation is checked so MinInt64 can never silently wrap.
func signedEntryAmount(entry entity.Entry, asset valueobject.AssetCode, normal valueobject.Direction) (int64, bool, error) {
	if entry.AssetCode != asset {
		return 0, false, nil
	}
	if entry.Side == normal {
		return entry.AmountMinor, true, nil
	}
	negated, ok := service.CheckedSub(0, entry.AmountMinor)
	if !ok {
		return 0, false, entity.NewError("BALANCE_OVERFLOW", "balance computation overflowed")
	}
	return negated, true, nil
}

// sumActiveHolds totals ACTIVE holds blocking the account + asset.
// Expired holds no longer block: expiry authoritatively releases them even
// before the sweep worker records it.
func (s *BalanceService) sumActiveHolds(ctx context.Context, query port.BalanceQuery, now time.Time) (int64, error) {
	holds, err := s.holds.FindActiveByAccount(ctx, query.TenantID, query.AccountID)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, hold := range holds {
		if hold.AssetCode != query.AssetCode || hold.State != entity.HoldActive {
			continue
		}
		if !hold.ExpiresAt.IsZero() && !hold.ExpiresAt.After(now) {
			continue
		}
		next, ok := service.CheckedAdd(total, hold.AmountMinor)
		if !ok {
			return 0, entity.NewError("BALANCE_OVERFLOW", "balance computation overflowed")
		}
		total = next
	}
	return total, nil
}
