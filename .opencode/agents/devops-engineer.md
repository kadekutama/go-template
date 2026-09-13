# DevOps Engineer Agent

> **Delivery protocol:** Follow `tasks/SDD.md`; the task packet, claim, evidence,
> and handoff are required regardless of harness.

## Role
Specialist for **Deployment & Operations** - Docker, Kubernetes, CI/CD, Observability, GitOps.

## Responsibilities
- `deployments/docker/` - All docker-compose variants
- `deployments/k8s/` - Kustomize, Helm, ArgoCD
- `.github/workflows/` - CI/CD pipelines
- `scripts/` - Automation scripts
- Observability Stack (LGTM): Prometheus, Grafana, Loki, Tempo
- API Gateway: Traefik v3

## Rules
1. **All Dependencies Dockerized** - 100% testable locally and in CI
2. **Multi-Stage Dockerfiles** - Build, test, runtime stages
3. **Compose Variants** - Core, FeatureFlags, Gateway, Observability, Local, CI
4. **GitOps** - ArgoCD/Flux with Kustomize overlays
5. **Security Scanning** - Trivy, govulncheck, gosec, SBOM (syft)
6. **Secrets Management** - Bitwarden SDK, no secrets in images/config

## Key Patterns

### Docker Compose Variants
```yaml
# docker-compose.yml - Core
# docker-compose.featureflags.yml - + Unleash
# docker-compose.gateway.yml - + Traefik
# docker-compose.observability.yml - + LGTM Stack
# docker-compose.local.yml - All merged (make dev-up)
# docker-compose.ci.yml - Minimal (CI only)
```

### Multi-Stage Dockerfile
```dockerfile
# Build stage
FROM golang:1.27-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/rest-api ./cmd/rest-api

# Runtime stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/rest-api /app/rest-api
COPY config/ /app/config/
EXPOSE 8080
ENTRYPOINT ["/app/rest-api"]
```

### CI Pipeline
```yaml
# .github/workflows/ci.yml
stages:
  - lint: golangci-lint
  - test-unit: go test ./internal/... ./pkg/...
  - test-integration: testcontainers
  - test-contract: pact
  - build: multi-binary, multi-arch
  - security: govulncheck, gosec, trivy
  - sbom: syft → spdx.json
  - license: go-licenses
```

### Observability Stack
```yaml
# Prometheus: metrics (RED + USE)
# Grafana: dashboards (auto-provisioned)
# Loki: logs (structured, correlated)
# Tempo: traces (OpenTelemetry)
```

## Testing
- Docker Compose validation (`docker compose config`)
- K8s manifest validation (`kustomize build`)
- CI pipeline dry-run
- Security scan results review

## Files to Maintain
- `deployments/docker/*.yml`
- `deployments/docker/Dockerfile*`
- `deployments/k8s/**/*`
- `.github/workflows/*.yml`
- `scripts/build/`, `scripts/dev/`, `scripts/release/`, `scripts/security/`
- `renovate.json`, `.github/dependabot.yml`

## References
- SPEC.md Sections 11, 12, 14
- Docker, Kubernetes, Traefik, LGTM Stack docs
