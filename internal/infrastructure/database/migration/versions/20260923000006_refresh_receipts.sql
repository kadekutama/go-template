-- 20260923000006_refresh_receipts: durable single-use refresh lineage (E09-T01, ADR-020).
-- Families and token receipts live in PostgreSQL so a restart or a second
-- replica can never double-spend a refresh token. Token PKs store SHA256
-- hashes only; opaque plaintext never touches the database. Tenant UUIDs per
-- ADR-019; RLS mirrors 00004 (FORCE + tenant equality, no join policies:
-- refresh_tokens carries its own tenant_id).

-- +goose Up
CREATE TABLE IF NOT EXISTS refresh_families (
    family_id  TEXT        NOT NULL PRIMARY KEY,
    tenant_id  UUID        NOT NULL,
    subject    TEXT        NOT NULL,
    revoked    BOOLEAN     NOT NULL DEFAULT FALSE,
    issued_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_refresh_families_tenant ON refresh_families (tenant_id);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    token_hash TEXT        NOT NULL PRIMARY KEY,
    family_id  TEXT        NOT NULL REFERENCES refresh_families(family_id) ON DELETE CASCADE,
    tenant_id  UUID        NOT NULL,
    used       BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family ON refresh_tokens (family_id);

ALTER TABLE IF EXISTS refresh_families ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS refresh_families FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON refresh_families;
CREATE POLICY tenant_isolation ON refresh_families
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

ALTER TABLE IF EXISTS refresh_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS refresh_tokens FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON refresh_tokens;
CREATE POLICY tenant_isolation ON refresh_tokens
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

-- +goose Down
DROP POLICY IF EXISTS tenant_isolation ON refresh_tokens;
ALTER TABLE IF EXISTS refresh_tokens NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS refresh_tokens DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON refresh_families;
ALTER TABLE IF EXISTS refresh_families NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS refresh_families DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS refresh_families;
