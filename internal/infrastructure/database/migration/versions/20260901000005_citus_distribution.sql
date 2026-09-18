-- 20260901000005_citus_distribution: Citus 14.0 tenant sharding (E07.1-T01, ADR-013, ADR-019).
-- Composite tenant-first primary keys let core tables shard on tenant_id via
-- create_distributed_table; the asset registry replicates everywhere via
-- create_reference_table. Tables blocked by triggers (postings, entries) or
-- non-tenant uniques (outbox_events) stay local under the resilient path.
-- Citus forbids these calls inside an ambient transaction, hence NO TRANSACTION.
-- On single-node PostgreSQL (no citus extension) this is a NOTICE no-op so the
-- same migration applies on plain PG and on the Citus coordinator.

-- +goose NO TRANSACTION

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    distributed_tables TEXT[] := ARRAY[
        'ledgers', 'accounts', 'holds', 'idempotency_records', 'outbox_events', 'audit_refs'
    ];
    t TEXT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'citus') THEN
        RAISE NOTICE 'citus extension absent; skipping tenant sharding (single-node posture)';
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

    BEGIN
        PERFORM create_reference_table('assets');
        RAISE NOTICE 'citus reference table: assets';
    EXCEPTION
        WHEN duplicate_object THEN
            RAISE NOTICE 'citus table already a reference table: assets';
        WHEN OTHERS THEN
            RAISE NOTICE 'citus reference deferred for assets: %', SQLERRM;
    END;
END
$$;
-- +goose StatementEnd

-- Loud completion summary: per-table deferrals above are NOTICE-level by
-- design (see deferral taxonomy in tasks/evidence/E07.1-T01.md), but the
-- aggregate outcome must be visible. RAISE WARNING lands in the PostgreSQL
-- server log (goose/pgx clients do not forward server messages), so
-- pg_dist_partition remains ground truth and T05 logs it per run.
-- +goose StatementBegin
DO $$
DECLARE
    distributed_count INTEGER;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'citus') THEN
        RAISE NOTICE 'citus extension absent; single-node posture, nothing distributed';
        RETURN;
    END IF;

    SELECT COUNT(*) INTO distributed_count FROM pg_dist_partition
    WHERE logicalrelid::regclass::text IN (
        'ledgers', 'accounts', 'holds', 'idempotency_records', 'outbox_events', 'audit_refs', 'assets'
    );

    IF distributed_count < 7 THEN
        RAISE WARNING 'citus distribution partial: % of 7 tables distributed; remainder deferred (see E07.1-T01 evidence taxonomy)', distributed_count;
    ELSE
        RAISE NOTICE 'citus distribution complete: all 7 tables distributed';
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    distributed_tables TEXT[] := ARRAY[
        'ledgers', 'accounts', 'holds', 'idempotency_records', 'outbox_events', 'audit_refs'
    ];
    t TEXT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'citus') THEN
        RAISE NOTICE 'citus extension absent; nothing to undistribute';
        RETURN;
    END IF;

    FOREACH t IN ARRAY distributed_tables LOOP
        BEGIN
            PERFORM undistribute_table(t);
            RAISE NOTICE 'citus undistributed table: %', t;
        EXCEPTION
            WHEN OTHERS THEN
                RAISE NOTICE 'citus undistribute deferred for %: %', t, SQLERRM;
        END;
    END LOOP;
END
$$;
-- +goose StatementEnd
