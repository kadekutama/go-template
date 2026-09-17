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

// DefaultConfig returns pool defaults honoring E01-T03 sizing knobs.
func DefaultConfig(dsn string) Config {
	return Config{
		DSN:             dsn,
		MaxOpen:         25,
		MaxIdle:         5,
		ConnMaxLifetime: 30 * time.Minute,
	}
}

// Open connects with the postgres dialect and applies pool settings.
func Open(cfg Config) (*gorm.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("postgres: DSN is required")
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
