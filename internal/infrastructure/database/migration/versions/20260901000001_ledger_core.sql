-- 000001_ledger_core: immutable financial facts + rebuildable checkpoints.
-- Identity (ADR-019): DB-generated native UUIDv7 keys with composite
-- tenant-first primary keys so Citus can co-locate tenants. Money rule:
-- entries are BIGINT minor units; checkpoint aggregates are NUMERIC(38,0).
-- No float/ambiguous decimal money columns anywhere.

-- +goose Up
CREATE TABLE IF NOT EXISTS ledgers (
    tenant_id     UUID        NOT NULL,
    id            UUID        NOT NULL DEFAULT uuidv7(),
    name          TEXT        NOT NULL,
    alias         TEXT        NOT NULL,
    base_asset    TEXT        NOT NULL,
    chart_version TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, alias)
);
CREATE INDEX IF NOT EXISTS idx_ledgers_alias ON ledgers (alias);

CREATE TABLE IF NOT EXISTS assets (
    code       TEXT        NOT NULL PRIMARY KEY,
    precision  INTEGER     NOT NULL DEFAULT 2 CHECK (precision >= 0),
    status     TEXT        NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'SUSPENDED', 'RETIRED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS accounts (
    tenant_id  UUID        NOT NULL,
    id         UUID        NOT NULL DEFAULT uuidv7(),
    ledger_id  UUID        NOT NULL,
    parent_id  UUID        NULL,
    number     TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    class      TEXT        NOT NULL,
    asset_code TEXT        NOT NULL REFERENCES assets (code),
    status     TEXT        NOT NULL DEFAULT 'ACTIVE',
    purpose    TEXT        NOT NULL DEFAULT '',
    metadata   JSONB       NOT NULL DEFAULT '{}',
    version    BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, ledger_id) REFERENCES ledgers (tenant_id, id),
    FOREIGN KEY (tenant_id, parent_id) REFERENCES accounts (tenant_id, id),
    UNIQUE (tenant_id, ledger_id, number)
);
CREATE INDEX IF NOT EXISTS idx_accounts_tenant_ledger ON accounts (tenant_id, ledger_id);

CREATE TABLE IF NOT EXISTS postings (
    tenant_id           UUID        NOT NULL,
    id                  UUID        NOT NULL DEFAULT uuidv7(),
    ledger_id           UUID        NOT NULL,
    operation           TEXT        NOT NULL,
    external_reference  TEXT        NOT NULL DEFAULT '',
    description         TEXT        NOT NULL DEFAULT '',
    effective_at        TIMESTAMPTZ NOT NULL,
    recorded_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    reversal_of         UUID        NULL,
    reason              TEXT        NOT NULL DEFAULT '',
    metadata            JSONB       NOT NULL DEFAULT '{}',
    ledger_seq          BIGSERIAL   NOT NULL,
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, ledger_id) REFERENCES ledgers (tenant_id, id),
    FOREIGN KEY (tenant_id, reversal_of) REFERENCES postings (tenant_id, id),
    UNIQUE (tenant_id, ledger_id, ledger_seq)
);
CREATE INDEX IF NOT EXISTS idx_postings_tenant_ledger_seq ON postings (tenant_id, ledger_id, ledger_seq);
CREATE INDEX IF NOT EXISTS idx_postings_external_ref ON postings (tenant_id, external_reference);

CREATE TABLE IF NOT EXISTS entries (
    tenant_id    UUID   NOT NULL,
    id           UUID   NOT NULL DEFAULT uuidv7(),
    posting_id   UUID   NOT NULL,
    ledger_id    UUID   NOT NULL,
    account_id   UUID   NOT NULL,
    side         TEXT   NOT NULL CHECK (side IN ('DEBIT', 'CREDIT')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    asset_code   TEXT   NOT NULL REFERENCES assets (code),
    account_seq  BIGINT NOT NULL CHECK (account_seq >= 1),
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, posting_id) REFERENCES postings (tenant_id, id),
    FOREIGN KEY (tenant_id, account_id) REFERENCES accounts (tenant_id, id)
);
CREATE INDEX IF NOT EXISTS idx_entries_posting ON entries (posting_id);
CREATE INDEX IF NOT EXISTS idx_entries_account_cursor ON entries (tenant_id, account_id, account_seq);

CREATE TABLE IF NOT EXISTS holds (
    tenant_id    UUID        NOT NULL,
    id           UUID        NOT NULL DEFAULT uuidv7(),
    ledger_id    UUID        NOT NULL,
    account_id   UUID        NOT NULL,
    asset_code   TEXT        NOT NULL REFERENCES assets (code),
    amount_minor BIGINT      NOT NULL CHECK (amount_minor > 0),
    kind         TEXT        NOT NULL,
    state        TEXT        NOT NULL DEFAULT 'ACTIVE'
        CHECK (state IN ('ACTIVE', 'CAPTURED', 'RELEASED', 'EXPIRED')),
    expires_at   TIMESTAMPTZ NOT NULL,
    version      BIGINT      NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, ledger_id) REFERENCES ledgers (tenant_id, id),
    FOREIGN KEY (tenant_id, account_id) REFERENCES accounts (tenant_id, id)
);
CREATE INDEX IF NOT EXISTS idx_holds_account_state ON holds (tenant_id, account_id, state);

-- Rebuildable checkpoint reference (ADR-011 interim posture): derived from
-- entries, never the spend authority. No mutable balance column exists.
CREATE TABLE IF NOT EXISTS checkpoints (
    tenant_id      UUID          NOT NULL,
    ledger_id      UUID          NOT NULL,
    account_id     UUID          NOT NULL,
    asset_code     TEXT          NOT NULL REFERENCES assets (code),
    cursor_seq     BIGINT        NOT NULL,
    balance_minor  NUMERIC(38,0) NOT NULL,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, ledger_id, account_id, asset_code, cursor_seq),
    FOREIGN KEY (tenant_id, account_id) REFERENCES accounts (tenant_id, id)
);
CREATE INDEX IF NOT EXISTS idx_checkpoints_tenant ON checkpoints (tenant_id, ledger_id);

-- Immutability: posted facts reject UPDATE/DELETE; corrections are new rows.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_posted_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'posted facts are immutable: %', TG_TABLE_NAME;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_postings_immutable ON postings;
CREATE TRIGGER trg_postings_immutable
    BEFORE UPDATE OR DELETE ON postings
    FOR EACH ROW EXECUTE FUNCTION prevent_posted_mutation();

DROP TRIGGER IF EXISTS trg_entries_immutable ON entries;
CREATE TRIGGER trg_entries_immutable
    BEFORE UPDATE OR DELETE ON entries
    FOR EACH ROW EXECUTE FUNCTION prevent_posted_mutation();

-- Per-asset balance: debits must equal credits within one posting per asset.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION check_posting_balanced()
RETURNS trigger AS $$
DECLARE
    v_unbalanced INTEGER;
BEGIN
    SELECT COUNT(*) INTO v_unbalanced FROM (
        SELECT asset_code
        FROM entries
        WHERE posting_id = NEW.posting_id
        GROUP BY asset_code
        HAVING SUM(CASE WHEN side = 'DEBIT' THEN amount_minor ELSE -amount_minor END) <> 0
    ) AS bad;
    IF v_unbalanced > 0 THEN
        RAISE EXCEPTION 'unbalanced posting %: debits must equal credits per asset', NEW.posting_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_entries_balanced ON entries;
CREATE CONSTRAINT TRIGGER trg_entries_balanced
    AFTER INSERT ON entries
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION check_posting_balanced();

-- +goose Down
DROP TRIGGER IF EXISTS trg_entries_balanced ON entries;
DROP FUNCTION IF EXISTS check_posting_balanced();
DROP TRIGGER IF EXISTS trg_entries_immutable ON entries;
DROP TRIGGER IF EXISTS trg_postings_immutable ON postings;
DROP FUNCTION IF EXISTS prevent_posted_mutation();
DROP TABLE IF EXISTS checkpoints;
DROP TABLE IF EXISTS holds;
DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS postings;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS ledgers;
