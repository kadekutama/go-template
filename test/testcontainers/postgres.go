package testcontainers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresImage is the pinned PostgreSQL image for integration suites.
const PostgresImage = "postgres:18.6"

// PostgresHandle is one isolated PostgreSQL container.
type PostgresHandle struct {
	container tc.Container
	host      string
	port      string
	user      string
	password  string
	dbname    string
}

// StartPostgres boots one isolated PostgreSQL container and waits until it
// accepts connections. Cleanup is registered on t; Terminate is idempotent.
func StartPostgres(t *testing.T, dbname string) (*PostgresHandle, error) {
	t.Helper()
	SkipIfNoDocker(t)

	ctx, cancel := Background()
	defer cancel()

	if dbname == "" {
		dbname = "ledger_test"
	}

	req := tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        PostgresImage,
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "ledger",
				"POSTGRES_PASSWORD": "ledger_test_pw",
				"POSTGRES_DB":       dbname,
			},
			// SQL-level readiness (not just TCP accept): Postgres opens the
			// port while still starting up, which flakes first queries.
			WaitingFor: wait.ForSQL("5432/tcp", "pgx", func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://ledger:ledger_test_pw@%s:%s/%s?sslmode=disable",
					host, port.Port(), dbname)
			}).WithStartupTimeout(StartupTimeout()).WithPollInterval(500 * time.Millisecond),
		},
		Started: true,
	}

	container, err := tc.GenericContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	h := &PostgresHandle{container: container, user: "ledger", password: "ledger_test_pw", dbname: dbname}
	t.Cleanup(func() {
		_ = h.Terminate(context.Background())
	})

	mapped, err := container.MappedPort(ctx, "5432")
	if err != nil {
		_ = h.Terminate(context.Background())
		return nil, fmt.Errorf("map postgres port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = h.Terminate(context.Background())
		return nil, fmt.Errorf("postgres host: %w", err)
	}

	h.host = host
	h.port = mapped.Port()

	return h, nil
}

// ConnectionString returns the pgx-compatible DSN for the container.
func (h *PostgresHandle) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", h.user, h.password, h.host, h.port, h.dbname)
}

// Terminate stops the container; double-terminate returns nil.
func (h *PostgresHandle) Terminate(ctx context.Context) error {
	if h == nil || h.container == nil {
		return nil
	}

	c := h.container
	h.container = nil

	if err := c.Terminate(ctx); err != nil {
		return fmt.Errorf("terminate postgres container: %w", err)
	}

	return nil
}
