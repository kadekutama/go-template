# ADR-016: Enterprise Secrets Management & Cryptographic Tokenization via OpenBao

**Status:** Proposed  
**Date:** 2026-09-17  
**Note:** Proposed for repository owner review and acceptance (supersedes Bitwarden Secrets Manager in SPEC §7.7)  

## Context

In regulated financial technology (PCI-DSS Level 1, SOC 2 Type II), static credentials stored in configuration files or container environment variables represent a major security vulnerability. 

We audited secrets management requirements across two options:
1. **Bitwarden Secrets Manager (BWS)**:
   - Primarily a static key-value injection tool for developer environments and CI/CD.
   - *Limitations*: Lacks dynamic database credential generation, lacks an Encryption-as-a-Service (Transit) engine for card/PII data, and lacks an internal PKI engine for automated mTLS certificate rotation.
2. **HashiCorp Vault**:
   - The enterprise banking gold standard for dynamic credentials, transit encryption, and PKI.
   - *Limitation*: In August 2023, HashiCorp shifted Vault from open-source MPL-2.0 to the proprietary Business Source License (BSL 1.1), introducing commercial licensing encumbrances and vendor lock-in for redistributable templates and SaaS platforms.

In response, the **Linux Foundation** (along with IBM, Red Hat, Docker, and SUSE) established **OpenBao** as the official, 100% open-source (MPL-2.0) community fork of Vault. OpenBao maintains complete wire and API compatibility with HashiCorp Vault while eliminating proprietary licensing constraints.

## Decision

1. **Adopt OpenBao v2.6.2 (Linux Foundation)**:
   - Deploy a 3-node OpenBao cluster with integrated Raft storage as the primary secrets and cryptographic engine.
   - Supersede Bitwarden Secrets Manager in `SPEC.md §7.7` and `tasks/epics/E09-identity-security.md`.
2. **Dynamic Database Credential Leasing**:
   - OpenBao dynamically provisions temporary, short-lived PostgreSQL credentials (e.g. 1-hour leased roles for `app_user`).
   - If an application container is compromised, the attacker acquires only a temporary lease that can be revoked immediately via a single OpenBao API call.
3. **Transit Secrets Engine (Encryption-as-a-Service / Tokenization)**:
   - Application servers **never store or hold master encryption keys in RAM**.
   - For credit card Primary Account Numbers (PANs), bank account routing numbers, and sensitive PII, the application sends plaintext to OpenBao's Transit engine; OpenBao encrypts the data using HSM/KMS-backed keys (AES-256-GCM) and returns ciphertext.
   - Enables cryptographic key rotation and instant **GDPR crypto-shredding** (destroying the encryption key renders stored ciphertext permanently unrecoverable while preserving double-entry ledger balance integrity).
4. **Wire Compatibility Guarantee**:
   - Go application services utilize the standard Vault Go client SDK (`github.com/hashicorp/vault/api`). If an enterprise client already utilizes HashiCorp Vault Enterprise, they can switch endpoints with zero code modifications.

## Real-World Scenarios Covered

- **Pod Intrusion / Lateral Movement**: An attacker gains shell access to a Kubernetes worker pod. Instead of finding a permanent PostgreSQL password, they discover an ephemeral credential expiring in 12 minutes. The security team revokes all active leases instantly.
- **GDPR "Right to be Forgotten"**: A user requests complete account erasure. The ledger requires preserving immutable double-entry postings for financial audit laws. By crypto-shredding the user's specific PII encryption key in OpenBao Transit, their personal data is rendered permanently unreadable while accounting entries remain mathematically sound.

## Consequences

- `SPEC.md §7.7`, `§14.2`, and `tasks/epics/E09-identity-security.md` are updated to specify OpenBao v2.6.2.
- `deployments/docker/` includes a 3-node OpenBao cluster definition with Raft storage.
