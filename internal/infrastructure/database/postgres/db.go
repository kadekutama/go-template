// Package postgres implements the PostgreSQL persistence adapters for the
// ledger-core boundary (E07-T01): GORM models, repository adapters, and the
// explicit atomic posting transaction behind port.UnitOfWork.
package postgres

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Config carries connection settings (Parameter Object pattern).
type Config struct {
	DSN             string
	MaxOpen         int
	MaxIdle         int
	ConnMaxLifetime time.Duration
}

// Validate verifies all required PostgreSQL pool configuration fields are set.
func (c Config) Validate() error {
	if c.DSN == "" {
		return fmt.Errorf("postgres: DSN is required")
	}
	if c.MaxOpen <= 0 {
		return fmt.Errorf("postgres: MaxOpen must be positive")
	}
	if c.MaxIdle <= 0 {
		return fmt.Errorf("postgres: MaxIdle must be positive")
	}
	if c.ConnMaxLifetime <= 0 {
		return fmt.Errorf("postgres: ConnMaxLifetime must be positive")
	}
	return nil
}

// Open connects with the postgres dialect and applies pool settings.
func Open(cfg Config) (*gorm.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("postgres: pool: %w", err)
	}

	if cfg.MaxOpen > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	}

	if cfg.MaxIdle > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	}

	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	return db, nil
}
