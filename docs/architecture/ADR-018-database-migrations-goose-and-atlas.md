# ADR-018: Hybrid Database Migration Architecture (Goose v3 + Atlas CI Linter + UTC Timestamps)

**Status:** Proposed  
**Date:** 2026-09-17  
**Note:** Proposed for repository owner review and acceptance  

## Context

In high-throughput financial ledger, event-driven, and multi-tenant SaaS environments maintained by multiple autonomous engineering teams working in parallel, database migration tooling represents a critical operational reliability boundary. A migration failure or poor architectural choice in database change management directly threatens system uptime, data consistency, and regulatory audit compliance (PCI-DSS, SOX, SOC 2).

### Architectural Audit: The Existing Migration Runner

The repository previously relied on a dual-runner approach with significant limitations:
1. **In-Process Runtime (`internal/infrastructure/database/migration/migrate.go`)**:
   A custom ~310-line standard library runner maintaining its own `schema_migrations` ledger (`version INTEGER PRIMARY KEY, applied_at TIMESTAMPTZ`). It enforced strict contiguous 1..N sequential integer numbering (`version != index+1`), required split `.up.sql` and `.down.sql` file pairs, and rigidly executed every migration inside an ambient transaction (`tx, err := r.db.BeginTx(ctx, nil)`).
2. **External CLI (`scripts/db/migrate.sh`)**:
   An external script invoking `golang-migrate`, listed in `go.mod` for CLI tooling. This created an operational impedance mismatch: the in-process service runner and the external CLI maintained different conventions and behaviors on the same database.

### Core Architectural Bottlenecks

As the architecture evolves toward distributed multi-tenant persistence (Citus horizontal sharding in Epic E07.1), high-volume concurrent ledger entries, and enterprise envelope encryption (OpenBao in Epic E09), the existing custom runner introduces five critical operational failure modes:

1. **Rigid Ambient Transaction Wrapping (Distributed DDL & Non-Blocking Index Failure)**:
   The custom runner hardcodes `r.db.BeginTx(ctx, nil)` around every migration. Advanced PostgreSQL and Citus commands **strictly forbid** executing inside transaction blocks:
   - **Citus Table Distribution**: `SELECT create_distributed_table(...)` coordinates distributed catalogs across coordinator and worker nodes; Citus aborts if executed inside an ambient multi-statement transaction.
   - **Zero-Downtime Indexing**: In an active ledger with millions of entries, `CREATE INDEX` acquires an exclusive lock (`SHARE`) that blocks all incoming payment writes. Creating an index without blocking requires `CREATE INDEX CONCURRENTLY`, which PostgreSQL strictly forbids inside transaction blocks (`ERROR: CREATE INDEX CONCURRENTLY cannot run inside a transaction block`).
   - Because the custom runner cannot disable transaction wrapping per file or statement, it cannot support Citus sharding or zero-downtime indexing.

2. **Strict Sequential Gap Enforcement (`version != index+1`)**:
   The custom runner requires consecutive integer versions 1..N with zero gaps. When multiple engineering squads develop parallel features simultaneously (e.g., Squad A on webhooks and Squad B on FX), both squads claim version `000005`. If Squad B merges first, Squad A's PR cannot deploy without manually renumbering files, modifying PR history, and resolving merge conflicts.

3. **Monolithic String Execution & Lack of Statement Delimiters**:
   The custom runner executes `m.UpSQL` as a single unparsed query string. Multi-statement PL/pgSQL functions, complex triggers (`enforce_balanced_posting`), and anonymous `DO $$ ... $$` blocks require explicit statement delimiter parsing (`-- +goose StatementBegin` / `-- +goose StatementEnd`) to prevent parser syntax errors.

4. **Absence of Pre-Deployment Safety Analysis (Destructive DDL in CI)**:
   The custom runner is an execution engine, not a safety linter. A pull request that includes an accidental table rewrite (e.g., `ALTER TABLE accounts ALTER COLUMN currency TYPE VARCHAR(10)` or adding a non-null column without a default) will pass local tests, only to acquire an exclusive table lock in production, queuing requests and causing cascading service outages.

5. **Inability to Support Pure-Compute Programmatic Go Migrations**:
   Certain schema evolution tasks require deterministic, in-process computational logic (such as converting serialized legacy data structures or calculating algorithmic checksums) that cannot be expressed cleanly in SQL. The custom runner cannot execute Go migration code.

---

## Detailed Comparative Analysis: Why Goose + Atlas Over Alternatives

We conducted an exhaustive technical evaluation of database migration approaches:
- **`pressly/goose/v3`** (v3.28.0)
- **`ariga/atlas`** (CLI v1.3.0 community edition)
- **Bespoke stdlib runner + `golang-migrate` CLI** (previous baseline)
- **`dbmate`**

### Architectural Comparison Matrix

| Architectural Dimension | Bespoke stdlib runner | `golang-migrate` CLI | `dbmate` | `ariga/atlas` (Standalone) | **Hybrid: Goose v3 + Atlas (Selected)** |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Versioning Scheme** | Strict gapless integer (1..N) | Sequential integers | UTC timestamps | Declarative or timestamps | **Preserved 1..4 integers + 14-digit UTC timestamps (`YYYYMMDDHHMMSS`)** |
| **Concurrent Branch Immunity** | ❌ Fails; hard check `version != index+1` | ❌ Fails; dirty state on out-of-order | ⚠️ Executes, but no integrity manifest | ✅ Supported | **✅ Out-of-order execution (`-allow-missing`) + `atlas.sum` Merkle tree** |
| **Transaction Control / Pragmas** | ❌ Rigid; hardcoded `BeginTx` | ❌ Rigid; forces entire file into tx | ⚠️ Global per-file flag only | ✅ Supported in HCL/DDL | **✅ Granular statement annotations (`-- +goose NO TRANSACTION`, `StatementBegin/End`)** |
| **Citus Distributed DDL Support** | ❌ Fails on `create_distributed_table` | ❌ Fails on `create_distributed_table` | ⚠️ Shell-only workaround | ✅ Supported | **✅ Native support via non-transactional annotations** |
| **Non-Blocking Indexing** | ❌ Cannot run `CREATE INDEX CONCURRENTLY` | ❌ Cannot run `CREATE INDEX CONCURRENTLY` | ⚠️ Shell-only workaround | ✅ Supported | **✅ Seamless non-transactional execution** |
| **Go Library Embed (`embed.FS`)** | ✅ Supported (`io/fs`) | ✅ Supported (`io/fs`) | ❌ None (external binary only) | ⚠️ Go SDK complex/heavy | **✅ Native lightweight Go library (`goose.NewProvider`)** |
| **Programmatic `.go` Migrations** | ❌ Pure SQL only | ❌ Pure SQL only | ❌ Pure SQL only | ❌ HCL/SQL only | **✅ Native Go migration functions (`goose.AddMigration`)** |
| **CI Safety Analysis & Linting** | ❌ None (blind executor) | ❌ None (blind executor) | ❌ None (blind executor) | ✅ Industry standard | **✅ Ariga Atlas CLI dev-container pre-deployment analysis** |
| **File Tamper Detection** | ❌ None | ❌ None | ❌ None | ✅ `atlas.sum` checksum file | **✅ Cryptographic Merkle tree verification (`atlas.sum`)** |
| **Regulatory & Financial Audit** | ✅ Explicit SQL files | ✅ Explicit SQL files | ✅ Explicit SQL files | ⚠️ Pure declarative diff abstracts change record | **✅ Explicit, immutable SQL scripts versioned in Git** |

### Why the Custom Runner and `golang-migrate` Were Superseded
- The custom runner's hardcoded transaction wrapping (`BeginTx`) completely blocks Citus sharding (Epic E07.1) and non-blocking indexing (`CREATE INDEX CONCURRENTLY`).
- The custom runner's strict sequential gap check (`version != index+1`) halts deployment whenever concurrent feature branches merge out of order.
- `golang-migrate` CLI created an operational duality where the script-level tooling did not match the in-process execution engine.
- Neither provided pre-deployment safety linting or Merkle-tree file integrity.

### Why `dbmate` Was Not Selected
- `dbmate` is strictly an external binary CLI tool. It does not provide an idiomatic, lightweight Go library API that can be cleanly embedded inside the application binary or invoked in-process within Testcontainers integration test suites.
- It lacks programmatic Go migration capabilities and provides no pre-deployment safety linting.

### Why Standalone Declarative `ariga/atlas` Was Not Chosen as the Pure Engine
- Relying *exclusively* on Atlas's declarative mode (where engineers edit desired schema state and Atlas calculates dynamic diffs on the fly) abstracts the explicit audit trail required by financial regulators (PCI-DSS Requirement 6.4, SOX 404 IT General Controls).
- **The Ideal Synthesis**: Pairing **Goose v3** (for explicit, auditable, embeddable SQL execution) with **Atlas CLI** (for pre-deployment safety linting and Merkle tree validation) delivers automated pre-deployment safety while retaining an immutable, human-reviewed SQL audit log.

---

## Decision

We adopt a **Hybrid Migration Architecture**:

1. **Runtime Execution Engine: Pressly Goose v3 (`github.com/pressly/goose/v3` v3.28.0)**:
   - Integrated as the embedded migration runner in `internal/infrastructure/database/migration/migrate.go` via `goose.NewProvider`.
   - Embeds migration scripts using Go standard library `embed.FS` (`//go:embed versions/*.sql`).
   - **Preserves Applied History Without Renumbering**: Historical baseline migrations `000001` through `000004` retain their existing integer numbering (`000001_ledger_core.sql` … `000004_rls_policies.sql`), merging the `.up.sql` and `.down.sql` pairs into single cohesive files using `-- +goose Up` and `-- +goose Down`.
   - **14-Digit UTC Timestamps for Future Migrations**: All new migrations starting after `000004` use 14-digit UTC timestamps (`YYYYMMDDHHMMSS_<name>.sql`). Goose transparently parses both numeric prefixes and timestamps (`1 < 2 < 3 < 4 < 20260901000005`).
   - **Automatic Bootstrap / Cutover**: On startup, if the legacy `schema_migrations` table exists and `goose_db_version` is uninitialized, the runner executes a one-time bootstrap copying applied versions from `schema_migrations` into `goose_db_version`. This guarantees zero migration replay on existing databases.
   - **Transaction Control**: Uses `-- +goose NO TRANSACTION` placed at the top of migration files for Citus sharding commands (`create_distributed_table`, `create_reference_table`) and zero-downtime index creation (`CREATE INDEX CONCURRENTLY`).
   - **Statement Delimiters**: Uses `-- +goose StatementBegin` and `-- +goose StatementEnd` for multi-line stored procedures, triggers, and anonymous `DO $$ ... $$` blocks.
   - **Out-of-Order Execution**: Configured with `goose.WithAllowOutofOrder(true)` / `-allow-missing` to guarantee concurrent feature branches merge and deploy without version conflicts.

2. **CI/CD Quality & Safety Gate: Ariga Atlas CLI (v1.3.0)**:
   - Integrated into developer workflows and CI/CD pipelines via `make migrate-lint` and `scripts/db/migrate.sh lint`.
   - Enforces cryptographic migration immutability via `internal/infrastructure/database/migration/versions/atlas.sum` (`atlas migrate validate`). Any modification to historical migrations fails CI immediately.
   - Executes dynamic safety linting (`atlas migrate lint`) against an ephemeral dev container (PostgreSQL 18) to catch destructive DDL, table lock risks, and data loss hazards prior to merge.

3. **Go Migration Usage Policy**:
   - **Pure-Compute Only**: Programmatic Go migrations (`.go`) are permitted **exclusively for deterministic in-process compute and data transformations** (e.g. data re-encoding, complex mathematical recalculations, parsing legacy binary blobs, or schema format normalization).
   - **External Service Calls Strictly Prohibited**: Any data backfill requiring communication with external network services (such as OpenBao Transit encryption, third-party APIs, or message brokers) is **strictly forbidden in migration DDL**. Such backfills MUST be implemented as asynchronous, chunked, idempotent background worker jobs in **Epic E14** (`cmd/worker` or `cmd/cron`) with progress tracking, checkpointing, and rate limiting.

---

## Technical Implementation Specification

### 1. Migration File Structure & Annotations

All migration files live in `internal/infrastructure/database/migration/versions/`:
- Baseline: `000001_ledger_core.sql` … `000004_rls_policies.sql`
- Future additions: `YYYYMMDDHHMMSS_<descriptive_name>.sql`

#### Standard DDL with Constraints & Triggers
```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_audit_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE NOTICE 'Audit log entry created: %', NEW.id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS log_audit_mutation();
DROP TABLE IF EXISTS audit_logs CASCADE;
-- +goose StatementEnd
```

#### Non-Transactional Citus Distribution & Concurrent Indexing
`-- +goose NO TRANSACTION` must be placed at the very top of the file:
```sql
-- +goose NO TRANSACTION
-- +goose Up
SELECT create_distributed_table('ledger_entries', 'account_id');
CREATE INDEX CONCURRENTLY idx_entries_created_at ON ledger_entries (created_at);

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_entries_created_at;
```

### 2. Runtime Runner Architecture & Ledger Bootstrap

`internal/infrastructure/database/migration/migrate.go` encapsulates Goose v3 and handles one-time bootstrap from the legacy `schema_migrations` table:

```go
package migration

import (
    "context"
    "database/sql"
    "fmt"
    "io/fs"

    "github.com/pressly/goose/v3"
)

type Migration struct {
    Version int64
    Name    string
    Source  string
}

type RunnerParams struct {
    DB  *sql.DB
    FS  fs.FS
    Dir string
}

type Runner struct {
    db       *sql.DB
    fsys     fs.FS
    dir      string
    provider *goose.Provider
}

func NewRunner(params RunnerParams) (*Runner, error) {
    if params.DB == nil {
        return nil, fmt.Errorf("migration: DB is required")
    }

    fsys := params.FS
    if fsys == nil {
        fsys = Versions
    }

    dir := params.Dir
    if dir == "" {
        dir = "versions"
    }

    var subFS fs.FS
    if dir == "." || dir == "" {
        subFS = fsys
    } else {
        var err error
        subFS, err = fs.Sub(fsys, dir)
        if err != nil {
            return nil, fmt.Errorf("migration: sub fs %s: %w", dir, err)
        }
    }

    // Bootstrap legacy schema_migrations into goose_db_version if present
    if err := bootstrapLegacyLedger(params.DB); err != nil {
        return nil, fmt.Errorf("migration: bootstrap legacy ledger: %w", err)
    }

    provider, err := goose.NewProvider(
        goose.DialectPostgres,
        params.DB,
        subFS,
        goose.WithAllowOutofOrder(true),
        goose.WithLogger(goose.NopLogger()),
    )
    if err != nil {
        return nil, fmt.Errorf("migration: init goose provider: %w", err)
    }

    return &Runner{
        db:       params.DB,
        fsys:     fsys,
        dir:      dir,
        provider: provider,
    }, nil
}

func bootstrapLegacyLedger(db *sql.DB) error {
    ctx := context.Background()
    // If schema_migrations exists, copy applied versions into goose_db_version
    query := `
    DO $$
    BEGIN
        IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'schema_migrations') THEN
            CREATE TABLE IF NOT EXISTS goose_db_version (
                id serial PRIMARY KEY,
                version_id bigint NOT NULL,
                is_applied boolean NOT NULL,
                tstamp timestamp DEFAULT now()
            );
            CREATE UNIQUE INDEX IF NOT EXISTS goose_db_version_version_id_idx ON goose_db_version (version_id);
            INSERT INTO goose_db_version (version_id, is_applied)
            SELECT sm.version, true
            FROM schema_migrations sm
            WHERE NOT EXISTS (
                SELECT 1 FROM goose_db_version gv WHERE gv.version_id = sm.version
            );
        END IF;
    END $$;`
    _, err := db.ExecContext(ctx, query)
    return err
}

func (r *Runner) Up(ctx context.Context) error {
    _, err := r.provider.Up(ctx)
    return err
}

func (r *Runner) Down(ctx context.Context) error {
    _, err := r.provider.Down(ctx)
    return err
}

func (r *Runner) Version(ctx context.Context) (int64, error) {
    return r.provider.GetDBVersion(ctx)
}
```

### 3. Ariga Atlas CI Linter Integration

```bash
# Validate cryptographic Merkle tree of migration directory
atlas migrate validate \
  --dir "file://internal/infrastructure/database/migration/versions?format=goose"

# Lint new migrations against ephemeral PostgreSQL container
atlas migrate lint \
  --dir "file://internal/infrastructure/database/migration/versions?format=goose" \
  --dev-url "docker://postgres/18/dev" \
  --latest 1
```

### 4. Gate Verification Integration (`tasks/scripts/check-tasks.py`)

The `--migrations` verification flag in `tasks/scripts/check-tasks.py` is updated to validate the hybrid structure:
1. Globs `*.sql` in `internal/infrastructure/database/migration/versions/` (ignoring `atlas.sum`).
2. Asserts baseline versions 1..4 exist without gaps.
3. Asserts subsequent versions match 14-digit timestamps (`^\d{14}_[a-z0-9_]+\.sql$`).
4. Verifies `atlas.sum` exists and contains hash records.

---

## Real-World Scenarios Covered

### Scenario 1: Conflict-Free Concurrent Feature Branch Merges
- **Context**: Engineer Alice develops `20260918100000_add_tenant_webhooks.sql` on branch `feat/webhooks`. Engineer Bob develops `20260918110000_add_fx_quote_ttl.sql` on branch `feat/fx`.
- **Event**: Bob's branch merges first to `main` and is deployed to staging. When Alice's branch merges 3 hours later, Goose detects Alice's earlier timestamp (`20260918100000`), applies it cleanly via `goose.WithAllowOutofOrder(true)` (`-allow-missing`), and records it in `goose_db_version`.
- **Outcome**: Zero merge conflicts, zero sequence collision, zero dirty database locks.

### Scenario 2: Zero-Downtime Indexing on Hundred-Million-Row Ledger
- **Context**: Performance analysis indicates a high-frequency query bottleneck on `ledger_entries (created_at)`.
- **Action**: Engineer creates a migration using `-- +goose NO TRANSACTION` and `CREATE INDEX CONCURRENTLY idx_entries_created_at ON ledger_entries (created_at)`.
- **Outcome**: PostgreSQL builds the index across multiple passes without holding an exclusive lock. Payment authorizations continue writing to `ledger_entries` without interruption.

### Scenario 3: Citus Multi-Tenant Sharded Table Distribution (Epic E07.1)
- **Context**: Distributing core ledger tables across Citus worker nodes requires executing `SELECT create_distributed_table('entries', 'account_id')`.
- **Action**: Running this inside an ambient multi-statement transaction (`BEGIN ... COMMIT`) causes Citus to abort with catalog lock errors.
- **Outcome**: Goose executes the DDL with `-- +goose NO TRANSACTION`, successfully sharding the financial ledger across worker nodes.

### Scenario 4: Automated CI Catch of Dangerous Table Rewrite
- **Context**: A developer submits a pull request containing `ALTER TABLE accounts ALTER COLUMN currency TYPE VARCHAR(10);`.
- **Action**: In GitHub Actions CI, `make migrate-lint` triggers Atlas against an ephemeral dev container.
- **Outcome**: Atlas identifies that modifying the column type requires rewriting the entire multi-gigabyte table while holding an `ACCESS EXCLUSIVE` lock. Atlas flags check `PG001 (Table Rewrite)` and fails the pull request, preventing a catastrophic production outage.

### Scenario 5: In-Process Data Re-encoding vs. Decoupled Service Backfills
- **Context**: An aggregate field requires in-process numerical transformation from a legacy representation to standard format upon migration, while a separate requirement in Epic E09 involves encrypting existing bank account records via OpenBao Transit.
- **Action**: 
  - **In-Process Compute**: Implemented as a programmatic Go migration (`goose.AddMigrationContext`) executing pure CPU mathematics and local updates without network I/O.
  - **Service-Coupled Backfill**: The OpenBao Transit encryption backfill is explicitly decoupled into an **Epic E14** background worker job (`cmd/worker`), preventing deploy-time DDL hangs and network-timeout lockouts.
- **Outcome**: Deploy-time migrations remain fast and bounded; external service interactions are safely managed by asynchronous worker pools with resumption checkpoints.

---

## Consequences

- **Dependency Replacement**: `github.com/golang-migrate/migrate/v4` is replaced by `github.com/pressly/goose/v3` (`v3.28.0`).
- **Toolchain Updates**: `scripts/dev/setup.sh` installs `atlasgo` (`v1.3.0`) via Pixi and `goose` (`v3.28.0`) via `go install`.
- **History Preservation**: Migrations `000001`–`000004` keep their integer numbering; future migrations follow `YYYYMMDDHHMMSS_<name>.sql`; all are tracked by `atlas.sum`.
- **Superseded Evidence**: The recorded evidence in `tasks/evidence/E07-T06.md` regarding the legacy split `.up.sql`/`.down.sql` format and the custom runner is superseded by task `E07.1-T01`.
- **Task Scheduling**: Transition is scheduled and specified under task **E07.1-T01** (`tasks/specs/E07.1-T01.md`).
