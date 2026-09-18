-- 000002_idempotency_outbox: durable idempotency results + transactional outbox.
-- Written in the same atomic unit as the posting they describe (E07-T01);
-- relayed by the poller (E07-T03). Tenant scoping uses UUID per ADR-019.

-- +goose Up
CREATE TABLE IF NOT EXISTS idempotency_records (
    tenant_id    UUID        NOT NULL,
    key          TEXT        NOT NULL,
    fingerprint  TEXT        NOT NULL,
    response     BYTEA       NULL,
    completed_at TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, key)
);

CREATE TABLE IF NOT EXISTS outbox_events (
    tenant_id         UUID        NOT NULL,
    id                BIGSERIAL   NOT NULL,
    ledger_id         UUID        NOT NULL,
    event_type        TEXT        NOT NULL,
    aggregate_id      TEXT        NOT NULL,
    aggregate_version BIGINT      NOT NULL,
    payload           BYTEA       NOT NULL,
    occurred_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    claimed_at        TIMESTAMPTZ NULL,
    delivered_at      TIMESTAMPTZ NULL,
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, aggregate_id, aggregate_version)
);
CREATE INDEX IF NOT EXISTS idx_outbox_undelivered ON outbox_events (tenant_id, delivered_at, id) WHERE delivered_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS idempotency_records;
