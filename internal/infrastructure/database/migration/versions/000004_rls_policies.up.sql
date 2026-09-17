-- 000004_rls_policies: hard tenant boundaries (E05-T03 matrix).
-- Application and table-owner roles see only rows whose tenant_id matches
-- current_setting('app.current_tenant') via FORCE ROW LEVEL SECURITY.
-- Superusers and roles explicitly granted BYPASSRLS bypass RLS.
-- Enable RLS table by table.

ALTER TABLE IF EXISTS ledgers ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS postings ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS holds ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS checkpoints ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS idempotency_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS workflows ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS recon_sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS recon_match_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS recon_breaks ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS periods ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS approvals ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS inbox_receipts ENABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS audit_refs ENABLE ROW LEVEL SECURITY;

-- One policy shape per table: tenant equality against the request setting.
-- Tables without tenant_id (none after 000001–000003) would need a join
-- policy; the schema review in E07-T06 asserts every RLS table exposes it.

DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'ledgers', 'accounts', 'postings', 'entries', 'holds',
        'checkpoints', 'idempotency_records', 'outbox_events', 'tenants',
        'workflows', 'recon_sources', 'recon_match_groups', 'recon_breaks',
        'periods', 'reports', 'approvals', 'inbox_receipts', 'audit_refs'
    ] LOOP
        EXECUTE format('ALTER TABLE IF EXISTS %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
        IF t = 'tenants' THEN
            EXECUTE format(
                'CREATE POLICY tenant_isolation ON %I USING (id = current_setting(%L, true))',
                t, 'app.current_tenant');
        ELSE
            EXECUTE format(
                'CREATE POLICY tenant_isolation ON %I USING (tenant_id = current_setting(%L, true))',
                t, 'app.current_tenant');
        END IF;
    END LOOP;
END
$$;
