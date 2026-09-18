package entity

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Ledger is the boundary containing one chart of accounts and one base
// reporting context. Every posting belongs to exactly one ledger.
type Ledger struct {
	ID           valueobject.LedgerID
	TenantID     valueobject.TenantID
	Name         string
	Alias        string
	BaseAsset    valueobject.AssetCode
	ChartVersion string
}

// NewLedger validates ledger identity and scope. An empty ID is permitted
// prior to persistence; if set, it must be a valid canonical UUID.
func NewLedger(id valueobject.LedgerID, tenant valueobject.TenantID, name string, base valueobject.AssetCode, chartVersion string) (Ledger, error) {
	if id.String() != "" {
		if _, err := valueobject.ParseLedgerID(id.String()); err != nil {
			return Ledger{}, NewError("LEDGER_ID_INVALID", "ledger id is invalid")
		}
	}
	if tenant.String() == "" {
		return Ledger{}, NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if name == "" {
		return Ledger{}, NewError("LEDGER_NAME_REQUIRED", "ledger name is required")
	}
	if base == "" {
		return Ledger{}, NewError("LEDGER_ASSET_REQUIRED", "base asset code is required")
	}
	if chartVersion == "" {
		return Ledger{}, NewError("LEDGER_CHART_REQUIRED", "chart version is required")
	}
	return Ledger{ID: id, TenantID: tenant, Name: name, BaseAsset: base, ChartVersion: chartVersion}, nil
}

// AccountData is the persistence record for an account: classification
// metadata with tenant/ledger/asset scope. It carries no balance — balances
// are projections owned by postings, checkpoints, and holds. Structural
// validation lives here; lifecycle invariants live in the aggregate.
type AccountData struct {
	ID        valueobject.AccountID
	TenantID  valueobject.TenantID
	LedgerID  valueobject.LedgerID
	ParentID  *valueobject.AccountID
	Number    string
	Name      string
	Class     valueobject.AccountClass
	AssetCode valueobject.AssetCode
	Status    valueobject.AccountStatus
	Purpose   string
	Metadata  map[string]string
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks structural scope and classification. Number uniqueness is a
// repository concern, not a domain check. An empty ID is permitted prior to
// persistence; if set, it must be a valid canonical UUID.
func (a AccountData) Validate() error {
	if a.ID.String() != "" {
		if _, err := valueobject.ParseAccountID(a.ID.String()); err != nil {
			return NewError("ACCOUNT_ID_INVALID", "account id is invalid")
		}
	}
	if a.TenantID.String() == "" {
		return NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if a.LedgerID.String() == "" {
		return NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if a.Number == "" {
		return NewError("ACCOUNT_NUMBER_REQUIRED", "account number is required")
	}
	if a.Name == "" {
		return NewError("ACCOUNT_NAME_REQUIRED", "account name is required")
	}
	if _, err := valueobject.ParseAccountClass(string(a.Class)); err != nil {
		return NewError("ACCOUNT_CLASS_INVALID", "account class is invalid")
	}
	if a.AssetCode == "" {
		return NewError("ACCOUNT_ASSET_REQUIRED", "asset code is explicit on every postable account")
	}
	if _, err := valueobject.ParseAccountStatus(string(a.Status)); err != nil {
		return NewError("ACCOUNT_STATUS_INVALID", "account status is invalid")
	}
	if a.Version < 1 {
		return NewError("ACCOUNT_VERSION_INVALID", "version starts at 1")
	}
	return nil
}
