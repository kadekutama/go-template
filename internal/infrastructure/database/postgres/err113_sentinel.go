package postgres

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrPostgresLagAlertMetricIsRequired        = errors.New("postgres: lag alert metric is required")
	ErrPostgresLagAlertThresholdMustBePositive = errors.New("postgres: lag alert threshold must be positive")
	ErrPostgresLagAlertSeverityIsRequired      = errors.New("postgres: lag alert severity is required")
	ErrPostgresDSNIsRequired                   = errors.New("postgres: DSN is required")
	ErrPostgresMaxOpenMustBePositive           = errors.New("postgres: MaxOpen must be positive")
	ErrPostgresMaxIdleMustBePositive           = errors.New("postgres: MaxIdle must be positive")
	ErrPostgresConnMaxLifetimeMustBePositive   = errors.New("postgres: ConnMaxLifetime must be positive")
	ErrPostgresDbIsRequired                    = errors.New("postgres: db is required")
	ErrPostgresTransactionCallbackIsRequired   = errors.New("postgres: transaction callback is required")
	ErrPostgresContextCanceledContextCanceled  = errors.New("postgres: context canceled: context canceled")
	ErrPostgresTenantIsRequired                = errors.New("postgres: tenant is required")
	ErrPostgresReplicaPoolsNeedAPrimary        = errors.New("postgres: replica pools need a primary")
	ErrUnreachable                             = errors.New("unreachable")
	ErrPostgresSelectorNeedsPools              = errors.New("postgres: selector needs pools")
	ErrPostgresLagBoundMustBePositive          = errors.New("postgres: lag bound must be positive")
	ErrPostgresUnknownRegionXx                 = errors.New("postgres: unknown region xx: postgres: unknown region")
	ErrPostgresResidencyRegionIsRequired       = errors.New("postgres: residency region is required")
	ErrPostgresRouterNeedsAtLeastOneRegionPool = errors.New("postgres: router needs at least one region pool")
	ErrPostgresRouterNeedsANamedPoolPerRegion  = errors.New("postgres: router needs a named pool per region")
	ErrPostgresUnknownRegion                   = errors.New("postgres: unknown region")
)
