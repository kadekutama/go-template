# Epic E19: Hardening + Release

**Status:** pending
**Story Points:** 16
**Phase:** 11
**Dependencies:** E16, E17, E18
**SDD Gate:** G8
**Design refs:** `SPEC.md §2`, `SPEC.md §14`, `SPEC.md §16`, `docs/fintech-ledger-features.md §11.3, §14`, `docs/architecture/ADR-027-continuous-profiling-pyroscope-pprof.md`

> Why the last epic: hardening and release sign-off only mean something against
> a finished system — vuln-zero, SBOM, benchmarks, `google/pprof` + Go 1.27 PGO optimization, and the DR drill certify
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
pipelining, prepared statements) before E19-T05 generates `google/pprof` flamegraphs and Go 1.27 PGO profiles.
**Files:**
- Create: `test/bench/...`; Modify: pool/batch configs
**Steps:**
1. Benchmarks for hot paths (Money ops, spec eval, cache get/set, DB query, NATS publish).
2. Tune connection pools, batching, and allocations from initial benchmark runs.
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
**Depends On:** E07-T05, E16-T03, E18-T04, E19-T01, E19-T02, E19-T03, E19-T05
**Related Docs:** `docs/fintech-ledger-features.md §11.3, §14`, `SPEC.md §16`
**SDD Gate:** G8

---

### E19-T05: Offline `google/pprof` CLI (`v0.0.0-20261014150452-c3b4e57cab2c`) + `graphviz` toolchain & Go 1.27 PGO (`default.pgo`) pipeline
**Status:** pending
**Background:** While Grafana Pyroscope (`E09.2-T04`, `E15-T07`) provides always-on continuous sampling in deployed environments, offline microbenchmark optimization (`go test -bench`) and Go 1.27 Profile-Guided Optimization (`-pgo=auto`) require standalone `github.com/google/pprof` (`v0.0.0-20261014150452-c3b4e57cab2c`) and `graphviz` (`dot`) to inspect CPU/heap/mutex flamegraphs, run differential profile comparisons (`pprof -diff_base`), and merge representative production/benchmark CPU profiles into `cmd/*/default.pgo` (`ADR-027`).
**Files:**
- Modify: `scripts/dev/setup.sh`, `Makefile`, `scripts/build/build.sh`
- Create: `scripts/test/bench-profile.sh`, `cmd/rest-api/default.pgo`, `cmd/grpc-api/default.pgo`, `cmd/graphql-api/default.pgo`, `cmd/cron/default.pgo`, `cmd/consumer/default.pgo`
**Steps:**
1. In `scripts/dev/setup.sh`, add `graphviz` to `PIXI_PACKAGES` and `"pprof:github.com/google/pprof@v0.0.0-20261014150452-c3b4e57cab2c"` to `GO_TOOLS` so `pprof` and `dot` are installed into `~/.pixi/bin`.
2. Create `scripts/test/bench-profile.sh` and `Makefile` targets `bench-profile` and `pgo-profile`:
   - `make bench-profile`: runs `test/bench/...` and domain/infrastructure hot-path benchmarks with `-cpuprofile`, `-memprofile`, `-mutexprofile`, and `-blockprofile`, then invokes `pprof -top -cum` and `pprof -svg` to emit flamegraph/callgraph artifacts under `artifacts/profiles/`.
   - `make pgo-profile`: merges representative CPU profiles (`pprof -proto`) into `cmd/{rest-api,grpc-api,graphql-api,cron,consumer}/default.pgo` and verifies each file is a valid protobuf pprof profile (`pprof -top cmd/rest-api/default.pgo`).
3. Update `scripts/build/build.sh` and `Makefile` (`build-all`) to compile with `-pgo=auto` and record before/after benchmark throughput and `p99` latency improvements in the `E19-T05` evidence record.
**Acceptance Criteria:**
- [ ] `scripts/dev/setup.sh` installs `pprof` (`v0.0.0-20261014150452-c3b4e57cab2c`) and `graphviz` (`dot`).
- [ ] `make bench-profile` generates CPU, heap, mutex, and block `.pprof` profiles and SVG callgraphs/flamegraphs without error.
- [ ] `make pgo-profile` produces valid `default.pgo` files verified by `pprof -top`, and `make build-all` compiles all 5 binaries with `-pgo=auto`.
**Story Points:** 3
**Depends On:** E15-T07, E19-T03
**Related Docs:** `SPEC.md §2`, `SPEC.md §10.4`, `docs/fintech-ledger-features.md §14`, `docs/architecture/ADR-027-continuous-profiling-pyroscope-pprof.md`
**SDD Gate:** G8

## Acceptance Criteria

- [ ] E19-T01 … E19-T05 all `completed` (count 16 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Release checklist green; NFRs evidenced; `google/pprof` + `default.pgo` benchmarks recorded; drill measured
- [ ] SDD gate G8 checks pass — `tasks/tracking/GATES.md#G8`
