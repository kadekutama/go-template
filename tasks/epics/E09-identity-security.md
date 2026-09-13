# Epic E09: Identity + Security Adapters

**Status:** pending
**Story Points:** 22
**Phase:** 5 (parallel with E07, E08, E10)
**Dependencies:** E06 (ports)
**SDD Gate:** G4
**Design refs:** `SPEC.md §7.4`, `SPEC.md §7.7`, `SPEC.md §14`, `SPEC.md §7.10`,
`docs/fintech-ledger-features.md §10`, `docs/user-journeys.md §5`

> Why a separate epic: auth, secrets, encryption, and audit form the trust
> boundary — one epic means one coherent threat model and a single OWASP review.

## Tasks

### E09-T01: JWT (RS256) issuance + validation + rotation
**Status:** pending
**Background:** 15m access / 7d rotating refresh, JWKS, tenant/user/role claims.
**Files:**
- Create: `internal/infrastructure/auth/jwt/{issuer.go,validator.go,jwks.go,keys.go}`
**Steps:**
1. RS256 sign/verify; key IDs (`kid`) with rotation (overlap window, JWKS publish).
2. Claims: tenant_id, user_id, roles, permissions, token type; short expiry enforced.
3. Refresh rotation: single-use refresh tokens, reuse detection → revoke family (OWASP A07).
4. Keys from Bitwarden (E09-T05); never from disk in prod.
5. Implements: `TokenIssuer` + `TokenValidator` ports (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Expired/tampered/wrong-audience tokens rejected with distinct codes (tests).
- [ ] Rotation overlap: tokens signed by old key validate until grace expiry (test).
- [ ] Refresh reuse triggers family revocation (test).
**Story Points:** 4
**Depends On:** E06-T12, E01-T08
**Related Docs:** `SPEC.md §7.4`, `SPEC.md §2` (golang-jwt v5.3.1), `docs/fintech-ledger-features.md §10`, `SPEC.md §14`
**SDD Gate:** G4

---

### E09-T02: OAuth2/OIDC providers (Google, GitHub, generic)
**Status:** pending
**Background:** Federated login with PKCE per SPEC §7.4.
**Files:**
- Create: `internal/infrastructure/auth/oauth2/{providers.go,flow.go,state.go}`
**Steps:**
1. Providers: Google, GitHub, generic OIDC discovery; PKCE + `state` (signed, 10m TTL).
2. Map external identity → internal user (link table); new users get default tenant role.
3. Token exchange errors mapped to AppError codes (no provider details leaked).
4. Implements: `OAuthProvider` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Full code flow works against a mock OIDC server (test).
- [ ] `state` mismatch/tamper rejected (test).
**Story Points:** 3
**Depends On:** E06-T12, E01-T08
**Related Docs:** `SPEC.md §7.4`, `SPEC.md §2` (oauth2 v0.23.0)
**SDD Gate:** G4

---

### E09-T03: Casbin RBAC/ABAC enforcement
**Status:** pending
**Background:** Tenant/role/resource/action decisions (features §10, journeys §5).
**Files:**
- Create: `internal/infrastructure/auth/rbac/{enforcer.go,model.conf,policies.csv,loader.go}`
**Steps:**
1. Casbin v2.8.0 model: RBAC + ABAC (tenant match, resource owner, amount thresholds for approvals).
2. Policy loader from DB with hot reload; decision logging to audit (E09-T07).
3. Middleware helper for Echo/gRPC/GraphQL (wired in E11–E13).
4. Implements: `Authorizer` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Cross-tenant access denied even with valid JWT (test).
- [ ] Above-threshold adjustment requires approver ≠ resolver (test with E04-T02 rules).
- [ ] Policy reload without restart (test).
**Story Points:** 4
**Depends On:** E06-T12
**Related Docs:** `SPEC.md §7.4`, `SPEC.md §2` (Casbin v2.8.0), `docs/fintech-ledger-features.md §10`, `docs/user-journeys.md §5`
**SDD Gate:** G4

---

### E09-T04: API keys (scoped, rotatable, rate-limited)
**Status:** pending
**Background:** Server-to-server auth per features §10.
**Files:**
- Create: `internal/infrastructure/auth/apikey/{manager.go,hasher.go}`
**Steps:**
1. Key format with prefix + checksum; scopes per endpoint group; per-key rate-limit tier.
2. Argon2id-hashed storage; secret shown exactly once by a privileged create
   endpoint and redacted from logs/traces; rotation (dual-active window);
   durable revocation with Valkey cache.
3. Usage metrics per key (feeds fee-revenue reporting).
4. Implements: `ApiKeyManager` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Out-of-scope endpoint rejected with FORBIDDEN (test).
- [ ] Rotation keeps old key valid during window, invalid after (test).
**Story Points:** 3
**Depends On:** E06-T12
**Related Docs:** `SPEC.md §7.4`, `docs/fintech-ledger-features.md §10`, `docs/api-contracts.md §2`
**SDD Gate:** G4

---

### E09-T05: Bitwarden secrets provider
**Status:** pending
**Background:** Runtime secret injection per SPEC §7.7 (DB passwords, JWT keys,
API keys, KEKs, OAuth secrets).
**Files:**
- Create: `internal/infrastructure/secrets/bitwarden/{client.go,loader.go,cache.go}`
**Steps:**
1. Bitwarden SDK v2.1.0 client; `{{ secret:path }}` resolution in E01-T03 loader.
2. Local cache with TTL + background refresh; rotation-safe (versioned reads).
3. `bws` CLI documented for local dev; SDK for runtime; env-var fallback for tests only.
4. Implements: `SecretManager` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Missing secret fails boot with the secret path in the error (test).
- [ ] Rotation picked up within TTL without restart (test with fake).
**Story Points:** 2
**Depends On:** E01-T03, E01-T08, E06-T12
**Related Docs:** `SPEC.md §7.7`, `SPEC.md §2` (Bitwarden SDK v2.1.0)
**SDD Gate:** G4

---

### E09-T06: Envelope encryption + PII field handling
**Status:** pending
**Background:** AES-256-GCM DEK/KEK per SPEC §14, PII tokenization (features §10).
**Files:**
- Create: `internal/infrastructure/crypto/{envelope.go,keys.go,pii.go}`
**Steps:**
1. Per-record DEK wrapped by KEK (Bitwarden/HSM); encrypt/decrypt with AAD (tenant+record IDs).
2. PII markers: struct-tag driven (`pii:"email"`) encryption on write, decryption on authorized read.
3. PAN tokenization adapter interface (vault-backed in prod, fake in dev/test).
4. Key rotation: re-wrap DEKs; crypto-shredding path for E04-T04 erasure.
5. Implements: `Encrypter` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Ciphertext differs per record for identical plaintext (nonce test).
- [ ] Tampered AAD fails decryption (test).
- [ ] Erasure (crypto-shred) makes PII unrecoverable while ledger sums hold (test with E04-T04).
**Story Points:** 3
**Depends On:** E06-T12, E09-T05
**Related Docs:** `SPEC.md §14`, `docs/fintech-ledger-features.md §10`, `tasks/epics/E04-compliance.md#E04-T04`
**SDD Gate:** G4

---

### E09-T07: Audit logger (append-only, hash-chained, signed)
**Status:** pending
**Background:** Tamper-evident trail feeding E04 break/approval evidence and §14.
**Files:**
- Create: `internal/infrastructure/audit/{logger.go,chain.go,exporter.go}`
**Steps:**
1. Append-only table + per-tenant hash chain; HMAC-SHA256 per entry; periodic
   signed Merkle root/checkpoint exported to retention-locked object storage.
2. Capture: actor/action/resource/before-after/correlation/trace (from context).
3. Exporter to Loki + immutable cold store (7y hot configurable, indefinite signed roots).
4. Implements: `AuditLogger` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Tampered/deleted/reordered row detected against an independently stored signed checkpoint (test).
- [ ] Every E04-T02 approval and E04-T04 erasure produces an entry (integration with fakes).
**Story Points:** 3
**Depends On:** E01-T08, E06-T12
**Related Docs:** `SPEC.md §14`, `docs/fintech-ledger-features.md §4.1, §10`, `docs/data-flow.md §7`
**SDD Gate:** G4

## Acceptance Criteria

- [ ] E09-T01 … E09-T07 all `completed` (count 22 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Auth matrix (journeys §5 touchpoints) green
- [ ] No secret in code/images (grep CI check); PII round-trip + shred proven
- [ ] SDD gate G4 checks pass — `tasks/tracking/GATES.md#G4`
