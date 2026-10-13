package repositories

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrPostgresLedgerRepositoryNeedsDB                           = errors.New("postgres: ledger repository needs DB")
	ErrPostgresAccountRepositoryNeedsDB                          = errors.New("postgres: account repository needs DB")
	ErrPostgresPostingRepositoryNeedsDB                          = errors.New("postgres: posting repository needs DB")
	ErrPostgresHoldRepositoryNeedsDB                             = errors.New("postgres: hold repository needs DB")
	ErrPostgresTenantIsRequired                                  = errors.New("postgres: tenant is required")
	ErrPostgresUnbalancedPostingForAssetUSD                      = errors.New("postgres: unbalanced posting for asset USD: postgres: unbalanced posting for asset")
	ErrPostgresUnbalancedPostingForAssetEUR                      = errors.New("postgres: unbalanced posting for asset EUR: postgres: unbalanced posting for asset")
	ErrPostgresBalanceOverflowForAssetUSD                        = errors.New("postgres: balance overflow for asset USD: postgres: balance overflow for asset")
	ErrPostgresWorkflowStoreNeedsDB                              = errors.New("postgres: workflow store needs DB")
	ErrPostgresPayloadHashIsRequired                             = errors.New("postgres: payload hash is required")
	ErrPostgresInboxKeyIsRequired                                = errors.New("postgres: inbox key is required")
	ErrPostgresInvalidCursor                                     = errors.New("postgres: invalid cursor")
	ErrPostgresPostingSearchNeedsDB                              = errors.New("postgres: posting search needs DB")
	ErrPostgresAmountBoundsNeedTheReportingReadModelE06T09       = errors.New("postgres: amount bounds need the reporting read model (E06-T09)")
	ErrPostgresPostingsCarryNoStatusCorrectionsAreNewLinkedPosti = errors.New("postgres: postings carry no status; corrections are new linked postings")
	ErrPostgresEntryStoreNeedsDB                                 = errors.New("postgres: entry store needs DB")
	ErrPostgresUnknownAccount                                    = errors.New("postgres: unknown account")
	ErrPostgresAccountOutsidePostingScope                        = errors.New("postgres: account outside posting scope")
	ErrPostgresAccountIsNotACTIVE                                = errors.New("postgres: account is not ACTIVE")
	ErrPostgresPostingVanishedMidPage                            = errors.New("postgres: posting vanished mid-page")
	ErrPostgresBalanceOverflowForAsset                           = errors.New("postgres: balance overflow for asset")
	ErrPostgresUnbalancedPostingForAsset                         = errors.New("postgres: unbalanced posting for asset")
)
