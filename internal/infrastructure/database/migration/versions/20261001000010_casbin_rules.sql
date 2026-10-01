-- 20261001000010_casbin_rules: durable Casbin policy storage (E09-T03, CR-006).
-- Standard Casbin six-column schema (ptype + v0..v5); tenant scope lives in
-- the dom columns (v1 for p-rules, v2 for g-rules) and is enforced by the
-- matcher at decision time, so this is intentionally a GLOBAL table with no
-- tenant RLS policy (explicit global-policy-table decision per review
-- adjudication; enforcement remains dom-scoped).

-- +goose Up
CREATE TABLE IF NOT EXISTS casbin_rules (
    id     BIGSERIAL NOT NULL PRIMARY KEY,
    ptype  TEXT      NOT NULL DEFAULT '',
    v0     TEXT      NOT NULL DEFAULT '',
    v1     TEXT      NOT NULL DEFAULT '',
    v2     TEXT      NOT NULL DEFAULT '',
    v3     TEXT      NOT NULL DEFAULT '',
    v4     TEXT      NOT NULL DEFAULT '',
    v5     TEXT      NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_casbin_rules_ptype ON casbin_rules (ptype);

-- +goose Down
DROP TABLE IF EXISTS casbin_rules;
