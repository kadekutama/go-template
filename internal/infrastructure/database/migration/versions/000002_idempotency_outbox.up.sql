-- 000002_idempotency_outbox: durable idempotency results + transactional outbox.
-- Written in the same atomic unit as the posting they describe (E07-T01);
-- relayed by the poller (E07-T03).

CREATE TABLE IF NOT EXISTS idempotency_records (
    tenant_id    TEXT        NOT NULL,
    key          TEXT        NOT NULL,
    fingerprint  TEXT        NOT NULL,
    response     BYTEA       NULL,
    completed_at TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, key)
);

CREATE TABLE IF NOT EXISTS outbox_events (
    id                BIGSERIAL   NOT NULL PRIMARY KEY,
    tenant_id         TEXT        NOT NULL,
    ledger_id         TEXT        NOT NULL,
    event_type        TEXT        NOT NULL,
    aggregate_id      TEXT        NOT NULL,
    aggregate_version BIGINT      NOT NULL,
    payload           BYTEA       NOT NULL,
    occurred_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    claimed_at        TIMESTAMPTZ NULL,
    delivered_at      TIMESTAMPTZ NULL,
    UNIQUE (aggregate_id, aggregate_version)
);
CREATE INDEX IF NOT EXISTS idx_outbox_undelivered ON outbox_events (delivered_at, id) WHERE delivered_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_outbox_tenant ON outbox_events (tenant_id, id);
