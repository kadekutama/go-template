-- 20260923000008_audit_entries: append-only tamper-evident audit chain (E09-T07, ADR-020).
-- PRIMARY KEY (tenant_id, seq) makes the chain fork-proof across replicas.
-- Per-tenant append serialization uses PostgreSQL row-level locks on audit_heads
-- (SELECT ... FOR UPDATE), guaranteeing Citus distributed compatibility without
-- advisory lock hash collisions. RLS mirrors 00004.

-- +goose Up
CREATE TABLE IF NOT EXISTS audit_entries (
    tenant_id   UUID        NOT NULL,
    seq         BIGINT      NOT NULL,
    actor       TEXT        NOT NULL,
    action      TEXT        NOT NULL,
    resource    TEXT        NOT NULL,
    before_hash TEXT        NOT NULL DEFAULT '',
    after_hash  TEXT        NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    prev_hash   TEXT        NOT NULL,
    entry_hash  TEXT        NOT NULL,
    signature   TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, seq)
);
CREATE INDEX IF NOT EXISTS idx_audit_entries_tenant_time ON audit_entries (tenant_id, occurred_at);

ALTER TABLE IF EXISTS audit_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS audit_entries FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON audit_entries;
CREATE POLICY tenant_isolation ON audit_entries
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE TABLE IF NOT EXISTS audit_heads (
    tenant_id   UUID        NOT NULL,
    last_seq    BIGINT      NOT NULL DEFAULT 0,
    last_hash   TEXT        NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id)
);

ALTER TABLE IF EXISTS audit_heads ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS audit_heads FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON audit_heads;
CREATE POLICY tenant_isolation ON audit_heads
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

-- +goose Down
DROP POLICY IF EXISTS tenant_isolation ON audit_heads;
ALTER TABLE IF EXISTS audit_heads NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS audit_heads DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS audit_heads;

DROP POLICY IF EXISTS tenant_isolation ON audit_entries;
ALTER TABLE IF EXISTS audit_entries NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS audit_entries DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS audit_entries;
