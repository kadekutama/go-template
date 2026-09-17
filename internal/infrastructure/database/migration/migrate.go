// Package migration applies embedded versioned SQL pairs with a
// schema_migrations ledger. Up applies pending versions in order inside one
// transaction per version; Down rolls back the latest applied version.
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var versionName = regexp.MustCompile(`^(\d{6})_.+\.(up|down)\.sql$`)

// Migration is one numbered up/down pair.
type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

// RunnerParams carries Runner dependencies (Parameter Object pattern).
type RunnerParams struct {
	DB  *sql.DB
	FS  fs.FS
	Dir string
}

// Runner applies migrations from an embedded FS directory.
type Runner struct {
	db   *sql.DB
	fsys fs.FS
	dir  string
}

// NewRunner builds a Runner; DB must be non-nil.
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

	return &Runner{db: params.DB, fsys: fsys, dir: dir}, nil
}

// List parses and validates embedded pairs: zero gaps, up+down both present.
func (r *Runner) List() ([]Migration, error) {
	return ListPairs(r.fsys, r.dir)
}

// ListPairs parses pairs from any FS without a database connection.
func ListPairs(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("migration: read dir: %w", err)
	}

	byVersion, err := parsePairs(fsys, dir, entries)
	if err != nil {
		return nil, err
	}

	return orderPairs(byVersion)
}

// pairFile is one parsed migration file.
type pairFile struct {
	version   int
	direction string
	sql       string
	name      string
}

// parsePairs reads every pair file into a version map.
func parsePairs(fsys fs.FS, dir string, entries []fs.DirEntry) (map[int]*Migration, error) {
	byVersion := make(map[int]*Migration)

	for _, entry := range entries {
		file, err := parsePairFile(fsys, dir, entry.Name())
		if err != nil {
			return nil, err
		}

		m, ok := byVersion[file.version]
		if !ok {
			m = &Migration{Version: file.version, Name: file.name}
			byVersion[file.version] = m
		}

		if file.direction == "up" {
			m.UpSQL = file.sql
		} else {
			m.DownSQL = file.sql
		}
	}

	return byVersion, nil
}

// parsePairFile validates one file name and reads its SQL.
func parsePairFile(fsys fs.FS, dir string, filename string) (pairFile, error) {
	match := versionName.FindStringSubmatch(filename)
	if match == nil {
		return pairFile{}, fmt.Errorf("migration: bad file name %s", filename)
	}

	version, convErr := strconv.Atoi(match[1])
	if convErr != nil {
		return pairFile{}, fmt.Errorf("migration: bad version %s: %w", filename, convErr)
	}

	raw, readErr := fs.ReadFile(fsys, dir+"/"+filename)
	if readErr != nil {
		return pairFile{}, fmt.Errorf("migration: read %s: %w", filename, readErr)
	}

	return pairFile{version: version, direction: match[2], sql: string(raw), name: match[1]}, nil
}

// orderPairs sorts versions and enforces zero gaps with up+down present.
func orderPairs(byVersion map[int]*Migration) ([]Migration, error) {
	versions := make([]int, 0, len(byVersion))
	for version := range byVersion {
		versions = append(versions, version)
	}

	sort.Ints(versions)

	out := make([]Migration, 0, len(versions))

	for index, version := range versions {
		if version != index+1 {
			return nil, fmt.Errorf("migration: numbering gap: have %v, want 1..%d", versions, len(versions))
		}

		m := byVersion[version]
		if strings.TrimSpace(m.UpSQL) == "" || strings.TrimSpace(m.DownSQL) == "" {
			return nil, fmt.Errorf("migration: version %06d needs both up and down files", version)
		}

		out = append(out, *m)
	}

	return out, nil
}

// ensureLedger creates the bookkeeping table.
func (r *Runner) ensureLedger(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER NOT NULL PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`)
	if err != nil {
		return fmt.Errorf("migration: ensure ledger: %w", err)
	}

	return nil
}

// applied returns the set of applied versions.
func (r *Runner) applied(ctx context.Context) (map[int]bool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("migration: read ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()

	done := make(map[int]bool)

	for rows.Next() {
		var version int
		if scanErr := rows.Scan(&version); scanErr != nil {
			return nil, fmt.Errorf("migration: scan ledger: %w", scanErr)
		}

		done[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migration: iterate ledger: %w", err)
	}

	return done, nil
}

// Up applies every pending version in order.
func (r *Runner) Up(ctx context.Context) error {
	migrations, err := r.List()
	if err != nil {
		return err
	}

	if err := r.ensureLedger(ctx); err != nil {
		return err
	}

	done, err := r.applied(ctx)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if done[m.Version] {
			continue
		}

		tx, txErr := r.db.BeginTx(ctx, nil)
		if txErr != nil {
			return fmt.Errorf("migration: begin %06d: %w", m.Version, txErr)
		}

		if _, execErr := tx.ExecContext(ctx, m.UpSQL); execErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration: apply %06d: %w", m.Version, execErr)
		}

		if _, execErr := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.Version); execErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration: record %06d: %w", m.Version, execErr)
		}

		if commitErr := tx.Commit(); commitErr != nil {
			return fmt.Errorf("migration: commit %06d: %w", m.Version, commitErr)
		}
	}

	return nil
}

// Down rolls back the latest applied version.
func (r *Runner) Down(ctx context.Context) error {
	migrations, err := r.List()
	if err != nil {
		return err
	}

	if err := r.ensureLedger(ctx); err != nil {
		return err
	}

	done, err := r.applied(ctx)
	if err != nil {
		return err
	}

	for index := len(migrations) - 1; index >= 0; index-- {
		m := migrations[index]
		if !done[m.Version] {
			continue
		}

		tx, txErr := r.db.BeginTx(ctx, nil)
		if txErr != nil {
			return fmt.Errorf("migration: begin rollback %06d: %w", m.Version, txErr)
		}

		if _, execErr := tx.ExecContext(ctx, m.DownSQL); execErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration: rollback %06d: %w", m.Version, execErr)
		}

		if _, execErr := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, m.Version); execErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration: unrecord %06d: %w", m.Version, execErr)
		}

		if commitErr := tx.Commit(); commitErr != nil {
			return fmt.Errorf("migration: commit rollback %06d: %w", m.Version, commitErr)
		}

		return nil
	}

	return fmt.Errorf("migration: nothing applied")
}

// Version returns the highest applied version (0 when none).
func (r *Runner) Version(ctx context.Context) (int, error) {
	if err := r.ensureLedger(ctx); err != nil {
		return 0, err
	}

	var version sql.NullInt64

	err := r.db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("migration: current version: %w", err)
	}

	if !version.Valid {
		return 0, nil
	}

	return int(version.Int64), nil
}
