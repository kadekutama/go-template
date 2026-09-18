// Command seed applies the deterministic development seed (E07-T04)
// through the shared di.RunApp lifecycle like every other binary: kernel
// logging with binary+version on every lifecycle line, explicit fx timeouts,
// pool drain on stop, and exit codes without os.Exit past defers.
package main

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
	"github.com/kadekutama/go-template/internal/infrastructure/database/seed"
	"github.com/kadekutama/go-template/internal/shared/di"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// version identifies the running build. Release pipelines override it with
// -ldflags "-X main.version=$(git describe --tags --always --dirty)" (E17);
// "dev" in output means an unstamped local build seeded the database.
var version = "dev"

func main() {
	logger := di.ProvideLogger()

	var runner *seedRunner

	work := func(ctx context.Context) error {
		return runner.run(ctx)
	}

	os.Exit(di.RunApp(logger, "seed", version, work, seedGraphOptions(&runner)...))
}

// seedGraphOptions builds the seed fx graph; main and the graph-validation
// test share it so wiring drift fails the test before any binary is built.
// RunApp appends the framework logger and lifecycle timeouts itself.
func seedGraphOptions(runner **seedRunner) []fx.Option {
	return []fx.Option{
		di.InfrastructureModule(),
		fx.Provide(
			providePool,
			seed.DevPlan,
			newSeedRunner,
		),
		fx.Populate(runner),
	}
}

// seedRunner applies the deterministic plan with structured logging.
type seedRunner struct {
	db     *gorm.DB
	plan   seed.Plan
	logger log.Logger
}

// newSeedRunner constructs the runner from fx-provided dependencies.
func newSeedRunner(db *gorm.DB, plan seed.Plan, logger log.Logger) *seedRunner {
	return &seedRunner{db: db, plan: plan, logger: logger}
}

// run applies the plan once.
func (r *seedRunner) run(ctx context.Context) error {
	return seed.ApplyPostgres(ctx, r.db, r.plan)
}

// providePool opens the ledger pool from the explicit dev-tool contract
// (DATABASE_URL) and drains it on container stop. Pool sizing stays on
// dev defaults; E11 generalizes file-driven pool configuration.
func providePool(lc fx.Lifecycle) (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("seed: DATABASE_URL is required")
	}

	db, err := postgres.Open(postgres.DefaultConfig(dsn))
	if err != nil {
		return nil, fmt.Errorf("seed: open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("seed: pool handle: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return sqlDB.Close()
		},
	})

	return db, nil
}
