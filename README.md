# Go Clean Architecture Template

[![Go Version](https://img.shields.io/badge/Go-1.27.1-00ADD8?logo=go)](https://go.dev/)
[![Architecture](https://img.shields.io/badge/Architecture-Clean%20%2B%20DDD%20%2B%20SDD-blue)](SPEC.md)

A planning-stage, AI-friendly specification for a Go template implementing
**Clean Architecture + Domain-Driven Design + Spec-Driven Development** with a
Stripe-style payment ledger as its reference domain. No runtime has been
implemented yet; “production-ready” is the target, not the current status.

> **Audit status:** The 2026-09-11 repository audit found release-blocking
> accounting and reliability issues in the original design. Read the
> [audit](docs/repository-audit.md) and the normative
> [ledger correctness contract](docs/ledger-core.md) before implementation.

> **Repository decision (2026-09-13):** This is a standalone repository and its
> owner-approved default branch is `main`. The initial baseline commit, shared
> remote, and claim-serialization policy remain the E00-T00 bootstrap work.

## 🎯 Purpose

Designed as a foundation for complex systems:
- **Fintech Ledger** (Stripe/Plaid core) - Primary reference
- Multi-tenant SaaS Platforms (Notion/Linear)
- B2B E-commerce (Shopify Plus)
- Logistics Orchestration (Flexport)
- IoT Device Management
- Healthcare Interoperability (FHIR)
- Real-time Collaboration (Figma/Miro)

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  Interface Layer (cmd/)     │  REST │ gRPC │ GraphQL │ Cron │ Consumer │
├─────────────────────────────────────────────────────────────────┤
│  Application Layer          │  CQRS Commands/Queries, Workflows, DTOs  │
├─────────────────────────────────────────────────────────────────┤
│  Domain Layer               │  Entities, VOs, Aggregates, Events, Specs │
├─────────────────────────────────────────────────────────────────┤
│  Infrastructure Layer       │  Postgres, Valkey, NATS, Auth, Cache, etc │
└─────────────────────────────────────────────────────────────────┘
```

**Dependency Rule**: `Interface → Application → Domain ← Infrastructure`

**Note**: This template uses **Valkey** (OSS fork of Redis) instead of Redis, and **maildev** instead of MailHog.

## 🚀 Quick Start

For a detailed step-by-step walkthrough covering every phase from machine setup to local testing, refer to the **[Developer Workflow & Makefile Guide](docs/development/developer-workflow.md)**.

```bash
# Clone and enter
cd go-template

# Bootstrap environment, toolchains, and dependencies
make setup   # or ./scripts/dev/setup.sh

# Start all dependencies (Postgres, Valkey, NATS, Unleash, Traefik, LGTM stack)
make dev-up

# Run database migrations
make migrate-up

# Build all 5 binaries
make build-all

# Run tests
make test-all

# Start REST API (example)
./bin/rest-api
```

## 📦 Tech Stack (Target pins; verified at bootstrap)

These are reproducible target pins, not a blanket “latest” claim. E00/E01
resolve and verify them in `go.mod` and the container manifests; automated
dependency updates must keep the files and evidence synchronized. Go 1.27.1 is
the current stable patch baseline for this audit date.

| Category | Technology |
|----------|------------|
| **Language** | Go 1.27.1 |
| **DI** | fx (Uber) v1.24.0 |
| **HTTP** | Echo v5.3.1 |
| **gRPC** | grpc-go v1.66.0 |
| **GraphQL** | gqlgen v0.17.94 |
| **Database & Sharding** | PostgreSQL 18.x + Citus 14.0 + GORM v1.31.2 |
| **Database Migrations & Safety** | Pressly Goose v3.28.0 (runtime) + Ariga Atlas v1.3.0 (CI linter) (ADR-018) |
| **High Availability** | Patroni v4.1.5 (Bare-metal/VM) / CloudNativePG v1.30.0 (K8s) |
| **Distributed Coordination**| etcd v3.7.0 (3-node Raft DCS & config streaming) |
| **Cache** | Otter v2.3.0 (L1 W-TinyLFU) + Valkey 9.1.2 Cluster (L2) |
| **Messaging & Streaming** | Redpanda v26.2 (Kafka API Log) + NATS Core 2.14.6 (Edge Fanout) |
| **Auth** | JWT RS256 (v5.3.1), OAuth2/OIDC, Casbin RBAC (v2.8.0) |
| **Feature Flags** | OpenFeature v1.17.2 + Unleash v6.5.1 |
| **Secrets & Tokenization** | OpenBao v2.6.2 (Dynamic DB creds & Transit PCI-DSS encryption) |
| **Observability** | OpenTelemetry v1.46.0 + zerolog v1.35.1 + Prometheus 3.14.0 + Grafana 13.0 + Loki 3.7.7 + Tempo 2.9.4 |
| **API Gateway** | Traefik v3.2 |
| **Testing** | Testcontainers, k6, Pact, Litmus |

## 📁 Project Structure

```
go-template/
├── cmd/                    # 5 Entry points (binaries)
│   ├── rest-api/           # Echo REST server
│   ├── grpc-api/           # gRPC server
│   ├── graphql-api/        # gqlgen GraphQL
│   ├── cron/               # Distributed scheduler (etcd leader election)
│   └── consumer/           # Redpanda & NATS consumers (separate cluster)
├── internal/               # Private application code
│   ├── domain/             # DDD Core (no external deps)
│   ├── application/        # CQRS Use Cases
│   ├── interface/          # Private REST/gRPC/job adapters
│   ├── infrastructure/     # External adapters
│   └── shared/             # Kernel, DI, i18n
├── pkg/                    # Public reusable packages
├── api/                    # API contracts (Protobuf, OpenAPI, GraphQL)
├── config/                 # Configuration files
├── deployments/            # Docker, K8s, Helm
├── docs/                   # Documentation (MD per layer)
├── scripts/                # Automation scripts
├── test/                   # Test organization
└── SPEC.md                 # System specification; delivery SDD lives in tasks/
```

## 📋 Specification

See **[SPEC.md](SPEC.md)** for complete technical specification including:
- Architecture decisions (ADRs)
- Layer specifications with code examples
- Domain models (Fintech Ledger reference)
- Infrastructure adapters
- Testing strategy
- CI/CD pipeline
- Docker Compose configurations
- Security & compliance (OWASP Top 10)
- Implementation phases with acceptance criteria

## 🤖 AI-Agent Ready (planning/control plane)

This template includes:
- **AGENTS.md** - Harness-neutral mandatory agent instructions
- **tasks/SDD.md** - Task packets, claims, evidence, and takeover protocol
- **tasks/DELIVERY-SLICES.md** - Vertical financial delivery order
- **.opencode/agents/** - Thin OpenCode role adapters (Domain, API, DB, DevOps, Test)
- **.opencode/skills/** - Harness adapter skills (Go, DDD, Clean Architecture, domain specifications)
- **SPEC.md** - System specification governed by the normative precedence rules

## 📚 Documentation

- [`docs/repository-audit.md`](docs/repository-audit.md) - Full readiness audit, risks, and dispositions
- [`docs/ledger-core.md`](docs/ledger-core.md) - Normative ledger accounting and money-safety contract
- [`docs/fintech-ledger-features.md`](docs/fintech-ledger-features.md) - Product feature scope
- [`docs/api-contracts.md`](docs/api-contracts.md) - REST/gRPC/GraphQL/webhook design
- [`docs/money-flow.md`](docs/money-flow.md) - Money movement examples
- [`docs/data-flow.md`](docs/data-flow.md) - Persistence and processing flows
- [`docs/domain-events.md`](docs/domain-events.md) - Event catalog and delivery model
- [`docs/user-journeys.md`](docs/user-journeys.md) - Actor journeys
- [`docs/development/go-conventions.md`](docs/development/go-conventions.md) - Go, SOLID, CQRS, and ledger implementation rules
- [`tasks/SDD.md`](tasks/SDD.md) - Harness-neutral development and takeover protocol
- [`tasks/SDD-INTEROP.md`](tasks/SDD-INTEROP.md) - OpenSpec/Spec Kit mapping without a second source of truth
- [`tasks/DELIVERY-SLICES.md`](tasks/DELIVERY-SLICES.md) - Incremental build slices
- `docs/architecture/` - Architecture Decision Records (ADRs)
- `docs/domain/` - Domain layer documentation + executable domain specifications
- `docs/application/` - Application layer (CQRS, Workflows)
- `docs/infrastructure/` - Infrastructure adapters
- `docs/api/` - API documentation
- `docs/development/` - Getting started, testing, deployment guides

## 🧪 Testing

```bash
# Unit tests (fast, isolated)
make test-unit

# Integration tests (Testcontainers - real Postgres, Valkey, NATS)
make test-integration

# Contract tests (Pact)
make test-contract

# Performance tests (k6)
make test-performance

# All tests
make test-all
```

## 🐳 Docker Compose Variants

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Core: Citus (Coordinator + Workers), etcd, Valkey 9, Redpanda, NATS Core, OpenBao, Jaeger, maildev |
| `docker-compose.featureflags.yml` | + Unleash 6.5 (shares PostgreSQL/Citus) |
| `docker-compose.gateway.yml` | + Traefik 3.2 |
| `docker-compose.observability.yml` | + Prometheus 3.14.0 (HA), Grafana 13.0 (HA), Loki 3.7.7 (HA), Tempo 2.9.4 |
| `docker-compose.ha-patroni.yml` | + Patroni v4.1.5 + etcd DCS multi-node failover simulation |
| `docker-compose.local.yml` | All merged (local dev) |
| `docker-compose.ci.yml` | Minimal (CI only) |

## 🔐 Security

These are planned controls, not an implementation or compliance claim:

- OWASP Top 10 control mapping
- JWT RS256 with rotating refresh tokens
- Argon2id password hashing
- Envelope encryption (DEK + KEK in OpenBao Transit Engine / HSM)
- PCI-DSS cardholder data tokenization via OpenBao Transit API
- Ephemeral dynamic PostgreSQL credentials with 1h lease revocation
- TLS 1.3 and mTLS for service-to-service and etcd peer traffic
- Rate limiting, CSP, and secure headers
- SBOM generation and vulnerability scanning (govulncheck, gosec, Trivy)
- Append-only, signed audit logging

## 📄 License

Not yet selected. Add an explicit `LICENSE` file before distribution; do not
infer licensing from this planning document.

## 🤝 Contributing

1. Read [SPEC.md](SPEC.md) and [AGENTS.md](AGENTS.md)
2. Follow the implementation phases
3. Run `make test-all` before PR
4. Update documentation with changes
5. Record ADR for architectural decisions

---

**Status**: Planning audited; bootstrap and implementation are pending.  
**Next**: Establish the shared Git baseline (E00-T00), then implement the
narrow vertical slice in `tasks/DELIVERY-SLICES.md`; resolve ADR-006/007/008/011
before their dependent features.
