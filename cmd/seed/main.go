// Command seed applies the deterministic development seed (E07-T04).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
	"github.com/kadekutama/go-template/internal/infrastructure/database/seed"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "seed: DATABASE_URL is required")
		os.Exit(1)
	}

	db, err := postgres.Open(postgres.DefaultConfig(dsn))
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed: open database: %v\n", err)
		os.Exit(1)
	}

	plan := seed.DevPlan()
	if err := seed.ApplyPostgres(context.Background(), db, plan); err != nil {
		fmt.Fprintf(os.Stderr, "seed: apply error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("seed: dev data successfully applied")
}
