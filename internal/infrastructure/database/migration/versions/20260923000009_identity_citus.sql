-- 20260923000009_identity_citus: distribute E09 tables on tenant_id (ADR-013, ADR-020).
-- Mirrors 20260901000005's resilient path: no-op NOTICE on single-node
-- PostgreSQL, duplicate_object-tolerant on the coordinator. Refresh, API-key,
-- and audit rows are all tenant-scoped facts, so tenant_id is the correct
-- distribution column; no reference tables here.

-- +goose NO TRANSACTION

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    distributed_tables TEXT[] := ARRAY[
        'refresh_families', 'refresh_tokens', 'api_keys', 'audit_entries', 'audit_heads'
    ];
    t TEXT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'citus') THEN
        RAISE NOTICE 'citus extension absent; skipping E09 sharding (single-node posture)';
        RETURN;
    END IF;

    FOREACH t IN ARRAY distributed_tables LOOP
        BEGIN
            PERFORM create_distributed_table(t, 'tenant_id');
            RAISE NOTICE 'citus distributed table: %', t;
        EXCEPTION
            WHEN duplicate_object THEN
                RAISE NOTICE 'citus table already distributed: %', t;
            WHEN OTHERS THEN
                RAISE NOTICE 'citus distribution deferred for %: %', t, SQLERRM;
        END;
    END LOOP;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- Single-node safe: undistribute is a coordinator-only concern; plain PG runs
-- the same guarded block and no-ops.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'citus') THEN
        RAISE NOTICE 'citus extension absent; nothing to undistribute';
        RETURN;
    END IF;
    BEGIN
        PERFORM undistribute_table('refresh_families');
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'undistribute deferred: %', SQLERRM;
    END;
    BEGIN
        PERFORM undistribute_table('refresh_tokens');
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'undistribute deferred: %', SQLERRM;
    END;
    BEGIN
        PERFORM undistribute_table('api_keys');
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'undistribute deferred: %', SQLERRM;
    END;
    BEGIN
        PERFORM undistribute_table('audit_entries');
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'undistribute deferred: %', SQLERRM;
    END;
    BEGIN
        PERFORM undistribute_table('audit_heads');
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'undistribute deferred: %', SQLERRM;
    END;
END
$$;
-- +goose StatementEnd
