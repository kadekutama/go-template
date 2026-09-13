# Epic E19: Hardening + Release

**Status:** pending
**Story Points:** 13
**Phase:** 11
**Dependencies:** E16, E17, E18
**SDD Gate:** G8
**Design refs:** `SPEC.md §14`, `SPEC.md §16`, `docs/fintech-ledger-features.md §11.3, §14`

> Why the last epic: hardening and release sign-off only mean something against
> a finished system — vuln-zero, SBOM, benchmarks, and the DR drill certify
> everything built in E00–E18.

## Tasks

### E19-T01: Security hardening sign-off (headers, TLS, mTLS, vuln-zero)
**Status:** pending
**Background:** Final OWASP pass (`SPEC.md §14`): headers, TLS 1.3 only, mTLS
service-to-service, dependency scans at zero.
**Files:**
- Modify: Traefik + Echo security middleware, service TLS configs
**Steps:**
1. Headers: CSP/HSTS/X-Frame/X-Content-Type/Referrer/Permissions-Policy verified via scanner.
2. TLS 1.3 only; cert rotation via Let's Encrypt + cert-manager; mTLS for gRPC/NATS/DB.
3. `govulncheck`, `gosec`, `trivy` at zero HIGH/CRITICAL; exceptions require ADR + expiry.
4. Pen-test findings (if any) tracked to closure.
**Acceptance Criteria:**
- [ ] Header scan + TLS scan clean (reports in artifacts).
- [ ] Vuln scans zero HIGH/CRITICAL on release commit.
**Story Points:** 4
**Depends On:** E11-T01, E17-T01
**Related Docs:** `SPEC.md §14`, `SPEC.md §2` (gosec, govulncheck, Trivy)
**SDD Gate:** G8

---

### E19-T02: SBOM, licenses, supply chain
**Status:** pending
**Background:** `syft` SBOM + `go-licenses` compliance on every release.
**Files:**
- Modify: `.github/workflows/release.yml` (attach SBOM), root `LICENSE`, `THIRD_PARTY_LICENSES`
**Steps:**
1. Release attaches `spdx.json` per image; provenance attestation enabled.
2. License scan allows MIT/Apache-2.0/BSD only; violations block release.
**Acceptance Criteria:**
- [ ] Test release contains SBOM + license files; violating dep blocks (verified once).
**Story Points:** 2
**Depends On:** E17-T01
**Related Docs:** `SPEC.md §11`, `SPEC.md §14`
**SDD Gate:** G8

---

### E19-T03: Performance tuning + benchmarks baseline
**Status:** pending
**Background:** Close the NFR gap: E16 measured, this epic tunes (pool sizes,
pipelining, prepared statements, PGO).
**Files:**
- Create: `test/bench/...`, `profile.pgo` (if beneficial); Modify: pool/batch configs
**Steps:**
1. Benchmarks for hot paths (Money ops, spec eval, cache get/set, DB query, NATS publish).
2. Tune from profiles; enable PGO build for release if it wins.
3. Commit baselines; CI fails on >10% regression vs baseline.
**Acceptance Criteria:**
- [ ] Baselines committed; regression gate active in CI.
**Story Points:** 3
**Depends On:** E16-T03
**Related Docs:** `docs/fintech-ledger-features.md §14`, `SPEC.md §10.4`
**SDD Gate:** G8

---

### E19-T04: Backup/DR drills + NFR sign-off + release checklist
**Status:** pending
**Background:** Prove RPO<5min/RTO<30min and NFRs, then ship.
**Files:**
- Create: `docs/development/runbooks/dr-drill-report.md` (per drill), `RELEASE_CHECKLIST.md`
**Steps:**
1. Run E07-T05 backup→restore→verify against staging; measure RPO/RTO; record.
2. NFR sign-off table (features §14 rows × measured values × suite links):
   10K TPS throughput, p99 <50ms transaction post, 99.99% availability (multi-AZ),
   strong (ACID) ledger consistency, jurisdictional data retention, and
   indefinitely retained signed audit roots.
3. Release checklist: gates G1–G8 green, ADRs indexed, SBOM attached, runbooks reviewed,
   rollback plan (binary + migration down) tested once.
**Acceptance Criteria:**
- [ ] Drill report with measured RPO/RTO inside targets.
- [ ] Every NFR row signed with evidence link.
- [ ] Checklist fully ticked for the release commit.
**Story Points:** 4
**Depends On:** E07-T05, E16-T03, E18-T04, E19-T01, E19-T02, E19-T03
**Related Docs:** `docs/fintech-ledger-features.md §11.3, §14`, `SPEC.md §16`
**SDD Gate:** G8

## Acceptance Criteria

- [ ] E19-T01 … E19-T04 all `completed` (count 13 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Release checklist green; NFRs evidenced; drill measured
- [ ] SDD gate G8 checks pass — `tasks/tracking/GATES.md#G8`
