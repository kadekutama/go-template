-- 20260923000007_api_keys: durable API-key registry (E09-T04, ADR-020).
-- Hashes (Argon2id hash/salt BYTEA with a version tag; no PHC strings are
-- ever parsed) and revocation live in PostgreSQL; no replica
-- can honor a key another replica revoked. Plaintext exists only in the
-- Create response and never reaches this table. Scopes ride a JSON array in
-- TEXT (stdlib encoding/json; no array-type driver dependency). Tenant UUIDs
-- per ADR-019; RLS mirrors 00004.

-- +goose Up
CREATE TABLE IF NOT EXISTS api_keys (
    id          UUID        NOT NULL DEFAULT uuidv7() PRIMARY KEY,
    tenant_id   UUID        NOT NULL,
    name        TEXT        NOT NULL,
    prefix      TEXT        NOT NULL UNIQUE,
    hash        BYTEA       NOT NULL,
    salt        BYTEA       NOT NULL,
    version     INT         NOT NULL DEFAULT 1,
    scopes      TEXT        NOT NULL DEFAULT '[]',
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked     BOOLEAN     NOT NULL DEFAULT FALSE,
    rotates_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant ON api_keys (tenant_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys (prefix);

ALTER TABLE IF EXISTS api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS api_keys FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON api_keys;
CREATE POLICY tenant_isolation ON api_keys
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

-- +goose Down
DROP POLICY IF EXISTS tenant_isolation ON api_keys;
ALTER TABLE IF EXISTS api_keys NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS api_keys DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS api_keys;
