-- 000003_workflow_tenancy_recon: operational state referencing ledger facts.
-- Workflow tables NEVER mutate postings/entries (separate tables, no triggers
-- into financial facts). Identity per ADR-019: UUID keys, tenant UUIDs.

-- +goose Up
CREATE TABLE IF NOT EXISTS tenants (
    id          UUID        NOT NULL DEFAULT uuidv7() PRIMARY KEY,
    name        TEXT        NOT NULL,
    alias       TEXT        NOT NULL UNIQUE,
    region      TEXT        NOT NULL DEFAULT 'local',
    settings    JSONB       NOT NULL DEFAULT '{}',
    status      TEXT        NOT NULL DEFAULT 'ACTIVE',
    version     BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workflows (
    id           UUID        NOT NULL DEFAULT uuidv7() PRIMARY KEY,
    tenant_id    UUID        NOT NULL REFERENCES tenants (id),
    ledger_id    UUID        NULL,
    kind         TEXT        NOT NULL
        CHECK (kind IN ('TRANSFER', 'SCHEDULED_TRANSFER', 'BATCH_TRANSFER', 'PAYMENT', 'REFUND', 'PAYOUT', 'TOPUP', 'DISPUTE', 'PERIOD_CLOSE', 'RECONCILIATION')),
    status       TEXT        NOT NULL DEFAULT 'PENDING',
    posting_id   UUID        NULL,
    payload_hash TEXT        NOT NULL DEFAULT '',
    version      BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_workflows_tenant_kind ON workflows (tenant_id, kind, status);

CREATE TABLE IF NOT EXISTS recon_sources (
    id             UUID        NOT NULL DEFAULT uuidv7() PRIMARY KEY,
    tenant_id      UUID        NOT NULL REFERENCES tenants (id),
    format         TEXT        NOT NULL,
    payload_hash   TEXT        NOT NULL,
    parser_version TEXT        NOT NULL,
    raw_ref        TEXT        NOT NULL DEFAULT '',
    received_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, payload_hash)
);
CREATE INDEX IF NOT EXISTS idx_recon_sources_tenant ON recon_sources (tenant_id, received_at);

CREATE TABLE IF NOT EXISTS recon_match_groups (
    id          UUID        NOT NULL DEFAULT uuidv7() PRIMARY KEY,
    tenant_id   UUID        NOT NULL REFERENCES tenants (id),
    period      TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL DEFAULT 'OPEN',
    version     BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS recon_breaks (
    id          UUID        NOT NULL DEFAULT uuidv7() PRIMARY KEY,
    tenant_id   UUID        NOT NULL REFERENCES tenants (id),
    group_id    UUID        NOT NULL REFERENCES recon_match_groups (id),
    break_type  TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'OPEN',
    resolver    TEXT        NOT NULL DEFAULT '',
    approver    TEXT        NOT NULL DEFAULT '',
    version     BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_recon_breaks_group ON recon_breaks (group_id, status);

CREATE TABLE IF NOT EXISTS periods (
    id          UUID        NOT NULL DEFAULT uuidv7() PRIMARY KEY,
    tenant_id   UUID        NOT NULL REFERENCES tenants (id),
    ledger_id   UUID        NULL,
    status      TEXT        NOT NULL DEFAULT 'OPEN'
        CHECK (status IN ('OPEN', 'CLOSING', 'CLOSED', 'REOPENED')),
    version     BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    opened_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at   TIMESTAMPTZ NULL
);
CREATE INDEX IF NOT EXISTS idx_periods_tenant ON periods (tenant_id, status);

CREATE TABLE IF NOT EXISTS reports (
    id          UUID        NOT NULL DEFAULT uuidv7() PRIMARY KEY,
    tenant_id   UUID        NOT NULL REFERENCES tenants (id),
    template    TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'PENDING',
    signed_url  TEXT        NOT NULL DEFAULT '',
    version     BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS approvals (
    id          UUID        NOT NULL DEFAULT uuidv7() PRIMARY KEY,
    tenant_id   UUID        NOT NULL REFERENCES tenants (id),
    subject_id  TEXT        NOT NULL,
    approver    TEXT        NOT NULL,
    decision    TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, subject_id, approver)
);

CREATE TABLE IF NOT EXISTS inbox_receipts (
    tenant_id    UUID        NOT NULL,
    key          TEXT        NOT NULL,
    event_type   TEXT        NOT NULL DEFAULT '',
    received_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, key)
);

CREATE TABLE IF NOT EXISTS audit_refs (
    tenant_id   UUID        NOT NULL,
    id          UUID        NOT NULL DEFAULT uuidv7(),
    actor       TEXT        NOT NULL,
    action      TEXT        NOT NULL,
    subject_id  TEXT        NOT NULL DEFAULT '',
    residency   TEXT        NOT NULL DEFAULT 'local',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);
CREATE INDEX IF NOT EXISTS idx_audit_tenant_time ON audit_refs (tenant_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS audit_refs;
DROP TABLE IF EXISTS inbox_receipts;
DROP TABLE IF EXISTS approvals;
DROP TABLE IF EXISTS reports;
DROP TABLE IF EXISTS periods;
DROP TABLE IF EXISTS recon_breaks;
DROP TABLE IF EXISTS recon_match_groups;
DROP TABLE IF EXISTS recon_sources;
DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS tenants;
