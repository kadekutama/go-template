package command

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrRefundWindowDaysMustBePositive                  = errors.New("refund: window days must be positive")
	ErrShouldNotExecuteOnReplay                        = errors.New("should not execute on replay")
	ErrShouldNotExecuteOnConflict                      = errors.New("should not execute on conflict")
	ErrTenantUowIsRequired                             = errors.New("tenant: uow is required")
	ErrTenantTenantStoreIsRequired                     = errors.New("tenant: tenant store is required")
	ErrTenantClockIsRequired                           = errors.New("tenant: clock is required")
	ErrTenantIDGeneratorIsRequired                     = errors.New("tenant: id generator is required")
	ErrTenantAuthorizerIsRequired                      = errors.New("tenant: authorizer is required")
	ErrTenantRegionsAreRequired                        = errors.New("tenant: regions are required")
	ErrTenantDefaultAssetsAreRequired                  = errors.New("tenant: default assets are required")
	ErrTenantMaxAssetsMustBePositive                   = errors.New("tenant: max assets must be positive")
	ErrTenantPlatformTenantIDIsRequired                = errors.New("tenant: platform tenant id is required")
	ErrTenantInvalidPlatformTenantIDIDsInvalidTenantID = errors.New("tenant: invalid platform tenant id: ids: invalid tenant id: ids: invalid id")
	ErrPaymentAuthExpiryDaysMustBePositive             = errors.New("payment: auth expiry days must be positive")
	ErrProcessorDown                                   = errors.New("processor down")
)
