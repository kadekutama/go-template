-- 000003_workflow_tenancy_recon: operational state referencing ledger facts.
-- Workflow tables NEVER mutate postings/entries (separate tables, no triggers
-- into financial facts). Every row carries tenant scope.

CREATE TABLE IF NOT EXISTS tenants (
    id          TEXT        NOT NULL PRIMARY KEY,
    name        TEXT        NOT NULL,
    region      TEXT        NOT NULL DEFAULT 'local',
    settings    JSONB       NOT NULL DEFAULT '{}',
    status      TEXT        NOT NULL DEFAULT 'ACTIVE',
    version     BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workflows (
    id           TEXT        NOT NULL PRIMARY KEY,
    tenant_id    TEXT        NOT NULL REFERENCES tenants (id),
    ledger_id    TEXT        NULL,
    kind         TEXT        NOT NULL
        CHECK (kind IN ('TRANSFER', 'SCHEDULED_TRANSFER', 'BATCH_TRANSFER', 'PAYMENT', 'REFUND', 'PAYOUT', 'TOPUP', 'DISPUTE', 'PERIOD_CLOSE', 'RECONCILIATION')),
    status       TEXT        NOT NULL DEFAULT 'PENDING',
    posting_id   TEXT        NULL,
    payload_hash TEXT        NOT NULL DEFAULT '',
    version      BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_workflows_tenant_kind ON workflows (tenant_id, kind, status);

CREATE TABLE IF NOT EXISTS recon_sources (
    id             TEXT        NOT NULL PRIMARY KEY,
    tenant_id      TEXT        NOT NULL REFERENCES tenants (id),
    format         TEXT        NOT NULL,
    payload_hash   TEXT        NOT NULL,
    parser_version TEXT        NOT NULL,
    raw_ref        TEXT        NOT NULL DEFAULT '',
    received_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, payload_hash)
);
CREATE INDEX IF NOT EXISTS idx_recon_sources_tenant ON recon_sources (tenant_id, received_at);

CREATE TABLE IF NOT EXISTS recon_match_groups (
    id          TEXT        NOT NULL PRIMARY KEY,
    tenant_id   TEXT        NOT NULL REFERENCES tenants (id),
    period      TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL DEFAULT 'OPEN',
    version     BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS recon_breaks (
    id          TEXT        NOT NULL PRIMARY KEY,
    tenant_id   TEXT        NOT NULL REFERENCES tenants (id),
    group_id    TEXT        NOT NULL REFERENCES recon_match_groups (id),
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
    id          TEXT        NOT NULL PRIMARY KEY,
    tenant_id   TEXT        NOT NULL REFERENCES tenants (id),
    ledger_id   TEXT        NULL,
    status      TEXT        NOT NULL DEFAULT 'OPEN'
        CHECK (status IN ('OPEN', 'CLOSING', 'CLOSED', 'REOPENED')),
    version     BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    opened_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at   TIMESTAMPTZ NULL
);
CREATE INDEX IF NOT EXISTS idx_periods_tenant ON periods (tenant_id, status);

CREATE TABLE IF NOT EXISTS reports (
    id          TEXT        NOT NULL PRIMARY KEY,
    tenant_id   TEXT        NOT NULL REFERENCES tenants (id),
    template    TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'PENDING',
    signed_url  TEXT        NOT NULL DEFAULT '',
    version     BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS approvals (
    id          TEXT        NOT NULL PRIMARY KEY,
    tenant_id   TEXT        NOT NULL REFERENCES tenants (id),
    subject_id  TEXT        NOT NULL,
    approver    TEXT        NOT NULL,
    decision    TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, subject_id, approver)
);

CREATE TABLE IF NOT EXISTS inbox_receipts (
    tenant_id    TEXT        NOT NULL,
    key          TEXT        NOT NULL,
    event_type   TEXT        NOT NULL DEFAULT '',
    received_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, key)
);

CREATE TABLE IF NOT EXISTS audit_refs (
    id          BIGSERIAL   NOT NULL PRIMARY KEY,
    tenant_id   TEXT        NOT NULL,
    actor       TEXT        NOT NULL,
    action      TEXT        NOT NULL,
    subject_id  TEXT        NOT NULL DEFAULT '',
    residency   TEXT        NOT NULL DEFAULT 'local',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_tenant_time ON audit_refs (tenant_id, created_at);
