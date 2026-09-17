# Epic E17: Delivery (CI/CD, Images, Compose, K8s, GitOps)

**Status:** pending
**Story Points:** 16
**Phase:** 9
**Dependencies:** E16 (gates to enforce)
**SDD Gate:** G7
**Design refs:** `SPEC.md §11–§12`, `SPEC.md §14` (supply chain)

> Note: E00 created the minimal lint+build pipeline. This epic completes it —
> it does not start a second one.

## Tasks

### E17-T01: Full CI pipeline (extend E00-T05, don't duplicate)
**Status:** pending
**Background:** `SPEC.md §11.1` stage list end-to-end.
**Files:**
- Modify: `.github/workflows/ci.yml`
- Create: `.github/workflows/{cd-staging.yml,cd-production.yml,dependency-update.yml,release.yml,nightly.yml}`
**Steps:**
1. Extend `ci.yml`: lint → unit → integration → contract → build → security
   (govulncheck, gosec, trivy) → sbom (syft→spdx.json) → license (go-licenses).
2. `cd-staging`: merge-to-main → GHCR push → ArgoCD sync staging.
3. `cd-production`: manual dispatch + approval → tag → push → ArgoCD sync prod.
4. `nightly`: k6 load + chaos dry-run + restore drill (E07-T05 scripts).
5. Minimal `permissions: read-all`, explicit writes; secrets inventory documented.
**Acceptance Criteria:**
- [ ] A test PR exercises every ci.yml stage (status checks all green).
- [ ] No second/duplicate pipeline file (`ls .github/workflows/` reviewed).
**Story Points:** 4
**Depends On:** E00-T05, E16-T05
**Related Docs:** `SPEC.md §11.1–§11.2`, `SPEC.md §14`, `tasks/epics/E00-foundation.md#E00-T05`
**SDD Gate:** G7

---

### E17-T02: Multi-arch Dockerfiles (5 binaries)
**Status:** pending
**Background:** Secure minimal images per SPEC §12.
**Files:**
- Create: `deployments/docker/Dockerfile.{rest-api,grpc-api,graphql-api,cron,consumer}`
**Steps:**
1. Stages: build (Go 1.27.1 base image) → test → runtime (Alpine 3.20 plus
   ca-certificates and tzdata). Resolve both by immutable digests in the
   release configuration; tags here are illustrative only.
2. Non-root `appuser:10001`; multi-arch `linux/amd64,arm64` via buildx.
3. Health checks: wget `/health` (REST), `grpc_health_probe` (gRPC).
4. SBOM stage emits `spdx.json`; trivy fs scan fails on HIGH/CRITICAL.
**Acceptance Criteria:**
- [ ] `docker buildx build --platform linux/amd64,linux/arm64` succeeds per binary.
- [ ] Running image as non-root verified (`whoami` ≠ root in smoke test).
- [ ] Production image references are digest-pinned; no floating `latest` tag is deployed.
**Story Points:** 4
**Depends On:** E11-T01, E12-T02, E13-T04, E14-T01, E14-T03
**Related Docs:** `SPEC.md §12`, `SPEC.md §2` (Go 1.27.1, Alpine 3.20)
**SDD Gate:** G7

---

### E17-T03: Compose variants verified (shared Postgres, all deps)
**Status:** pending
**Background:** `SPEC.md §12.1–§12.7`: core, featureflags, gateway, observability,
local, CI-minimal, prod — single shared Postgres (app+unleash DBs).
**Files:**
- Create/modify: `deployments/docker/docker-compose*.yml`, `deployments/docker/traefik/*`,
  `deployments/docker/{prometheus,grafana,loki,tempo}/*`, `deployments/docker/minio/*`
**Steps:**
1. Implement all 7 variants exactly per SPEC §12 (versions: Citus/Postgres 18, Valkey 9.1.2,
   Redpanda 26.2, NATS 2.14.6, OpenBao 2.6.2, Unleash 6.5, Traefik v3.2, Prom 3.14.0, Grafana 13.0, Loki 3.7.7, Tempo 2.9.4,
   maildev 3.0.0-rc.3, MinIO).
2. Prove single-Postgres: unleash connects to shared instance (no second DB container).
3. `make dev-up` (local), CI job (minimal), prod file with secrets/limits/replicas.
**Acceptance Criteria:**
- [ ] `docker compose -f <each variant> config` validates.
- [ ] `make dev-up` → all healthchecks green; `make test-integration` passes against it.
- [ ] Only one postgres container in every variant (`docker ps` assertion in smoke test).
- [ ] Production compose references immutable image digests and external secrets;
  local/CI tags are never promoted implicitly.
**Story Points:** 4
**Depends On:** E00-T04, E07-T09, E07.1-T02
**Related Docs:** `SPEC.md §12.1–§12.7`, `SPEC.md §10.3`
**SDD Gate:** G7

---

### E17-T04: K8s manifests + ArgoCD GitOps
**Status:** pending
**Background:** `SPEC.md §11.2` deployment model.
**Files:**
- Create: `deployments/k8s/{base/,overlays/{dev,staging,prod}/,argocd/}` (+ optional `helm/`)
**Steps:**
1. Base: Deployment/Service/ConfigMap/Secret/HPA/PDB/NetworkPolicy per binary + infra.
2. Overlays per env (replicas/resources/env/ingress); SealedSecrets (Bitwarden→controller).
3. ArgoCD Applications: auto-sync staging, manual-sync prod.
4. `kustomize build` validated in CI; `kube-linter` clean.
**Acceptance Criteria:**
- [ ] `kustomize build overlays/staging | kubeval` passes; kube-linter clean.
- [ ] Staging syncs automatically on merge (smoke-verified once).
**Story Points:** 4
**Depends On:** E17-T02
**Related Docs:** `SPEC.md §11.2`, `SPEC.md §14`
**SDD Gate:** G7

## Acceptance Criteria

- [ ] E17-T01 … E17-T04 all `completed` (count 16 SP in `tasks/tracking/PROGRESS.md`)
- [ ] PR→staging→prod path demonstrated; nightly perf/chaos/restore scheduled
- [ ] One-Postgres proven in every compose variant
- [ ] SDD gate G7 checks pass — `tasks/tracking/GATES.md#G7`
