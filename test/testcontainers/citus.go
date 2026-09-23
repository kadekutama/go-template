package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	mobyNetwork "github.com/moby/moby/api/types/network"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// CitusImage pins the Citus coordinator/worker image for the multi-node
// integration suite (E07.1-T05). It matches the compose profile pin.
const CitusImage = "citusdata/citus:14.0"

// citusStartupTimeout bounds Citus image pull + two-node cluster formation.
const citusStartupTimeout = 10 * time.Minute

// CitusHandle is one isolated Citus coordinator + worker cluster sharing a
// Docker network. The coordinator DSN is the application-facing endpoint.
type CitusHandle struct {
	network     *tc.DockerNetwork
	coordinator tc.Container
	worker      tc.Container
	host        string
	port        string
	workerHost  string
	workerPort  string
	user        string
	password    string
	dbname      string
}

// StartCitus boots a coordinator + one worker, enables the citus extension on
// both, and registers the worker on the coordinator. Cleanup is registered
// on t; Terminate is idempotent. It skips (never fails) without Docker.
func StartCitus(t *testing.T, dbname string) (*CitusHandle, error) {
	t.Helper()
	SkipIfNoDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), citusStartupTimeout)
	defer cancel()

	if dbname == "" {
		dbname = "ledger_citus_test"
	}

	//nolint:gosec // fixture uses ephemeral testcontainer credentials only, never production secrets.
	h := &CitusHandle{
		user:     "postgres",
		password: "citus_test_pw",
		dbname:   dbname,
	}
	t.Cleanup(func() { _ = h.Terminate(context.Background()) })

	clusterNet, err := network.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create citus network: %w", err)
	}

	h.network = clusterNet

	worker, workerHost, workerPort, err := startCitusNode(ctx, clusterNet.Name, "citus-worker", dbname, h.user, h.password)
	if err != nil {
		return nil, err
	}

	h.worker = worker
	h.workerHost = workerHost
	h.workerPort = workerPort

	coordinator, host, port, err := startCitusNode(ctx, clusterNet.Name, "citus-coordinator", dbname, h.user, h.password)
	if err != nil {
		return nil, err
	}

	h.coordinator = coordinator
	h.host = host
	h.port = port

	if err := h.enableCluster(ctx); err != nil {
		return nil, err
	}

	return h, nil
}

// startCitusNode boots one Citus-capable PostgreSQL node on the cluster
// network and waits until it accepts SQL connections.
func startCitusNode(ctx context.Context, networkName, alias, dbname, user, password string) (tc.Container, string, string, error) {
	req := tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        CitusImage,
			Hostname:     alias,
			ExposedPorts: []string{"5432/tcp"},
			Networks:     []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {alias},
			},
			Env: map[string]string{
				"POSTGRES_USER":     user,
				"POSTGRES_PASSWORD": password,
				"POSTGRES_DB":       dbname,
			},
			WaitingFor: wait.ForSQL("5432/tcp", "pgx", func(host string, port mobyNetwork.Port) string {
				return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
					user, password, host, port.Port(), dbname)
			}).WithStartupTimeout(citusStartupTimeout).WithPollInterval(time.Second),
		},
		Started: true,
	}

	container, err := tc.GenericContainer(ctx, req)
	if err != nil {
		return nil, "", "", fmt.Errorf("start citus node %s: %w", alias, err)
	}

	mapped, err := container.MappedPort(ctx, "5432")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", "", fmt.Errorf("map citus node %s port: %w", alias, err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", "", fmt.Errorf("citus node %s host: %w", alias, err)
	}

	return container, host, mapped.Port(), nil
}

// execCitusSQL runs one statement against dsn.
func execCitusSQL(ctx context.Context, dsn, stmt string, args ...any) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open citus connection: %w", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(ctx, stmt, args...); err != nil {
		return fmt.Errorf("exec citus statement: %w", err)
	}

	return nil
}

// enableCluster creates the citus extension on both nodes and registers the
// worker on the coordinator.
func (h *CitusHandle) enableCluster(ctx context.Context) error {
	if err := execCitusSQL(ctx, h.WorkerDSN(), "CREATE EXTENSION IF NOT EXISTS citus"); err != nil {
		return fmt.Errorf("enable worker citus extension: %w", err)
	}

	if err := execCitusSQL(ctx, h.ConnectionString(), "CREATE EXTENSION IF NOT EXISTS citus"); err != nil {
		return fmt.Errorf("enable coordinator citus extension: %w", err)
	}

	if err := execCitusSQL(ctx, h.ConnectionString(), "SELECT citus_add_node('citus-worker', 5432)"); err != nil {
		return fmt.Errorf("register citus worker: %w", err)
	}

	return nil
}

// ConnectionString returns the coordinator DSN (application endpoint).
func (h *CitusHandle) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", h.user, h.password, h.host, h.port, h.dbname)
}

// WorkerDSN returns the direct worker DSN (fixture introspection only;
// application traffic must use ConnectionString).
func (h *CitusHandle) WorkerDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", h.user, h.password, h.workerHost, h.workerPort, h.dbname)
}

// Coordinator exposes the coordinator container for crash-recovery drills.
func (h *CitusHandle) Coordinator() tc.Container {
	if h == nil {
		return nil
	}

	return h.coordinator
}

// Refresh re-resolves mapped host ports after a container restart: runtimes
// may reassign auto-allocated host ports on start, so a boot-time DSN can go
// stale while the container itself is healthy.
func (h *CitusHandle) Refresh(ctx context.Context) error {
	if h == nil || h.coordinator == nil {
		return fmt.Errorf("citus handle is not initialized")
	}

	mapped, err := h.coordinator.MappedPort(ctx, "5432")
	if err != nil {
		return fmt.Errorf("refresh citus coordinator port: %w", err)
	}

	host, err := h.coordinator.Host(ctx)
	if err != nil {
		return fmt.Errorf("refresh citus coordinator host: %w", err)
	}

	h.host = host
	h.port = mapped.Port()

	return nil
}

// Terminate stops the worker, coordinator, and network; nil-safe.
func (h *CitusHandle) Terminate(ctx context.Context) error {
	if h == nil {
		return nil
	}

	if h.coordinator != nil {
		c := h.coordinator
		h.coordinator = nil

		if err := c.Terminate(ctx); err != nil {
			return fmt.Errorf("terminate citus coordinator: %w", err)
		}
	}

	if h.worker != nil {
		w := h.worker
		h.worker = nil

		if err := w.Terminate(ctx); err != nil {
			return fmt.Errorf("terminate citus worker: %w", err)
		}
	}

	if h.network != nil {
		n := h.network
		h.network = nil

		if err := n.Remove(ctx); err != nil {
			return fmt.Errorf("remove citus network: %w", err)
		}
	}

	return nil
}
