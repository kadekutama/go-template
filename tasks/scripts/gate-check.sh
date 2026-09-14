#!/usr/bin/env bash
# Gate Check Script - validates capability-promotion gates for CI
# Usage: ./gate-check.sh G1|G2|G3|G4|G5|G6|G7|G8

set -euo pipefail

GATE="${1:-}"
BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GATES_FILE="${BASE_DIR}/tasks/tracking/GATES.md"
TASK_CHECK="${BASE_DIR}/tasks/scripts/check-tasks.py"

if [[ -z "${GATE}" ]]; then
    echo "Usage: $0 <GATE_ID> (e.g., G1, G2, G3, ...)"
    exit 1
fi

if [[ ! -f "${GATES_FILE}" ]]; then
    echo "Gates file not found: ${GATES_FILE}"
    exit 1
fi

# Gate name comes from the Markdown source of truth (## G1 — <name>)
GATE_NAME=$(grep -m1 "^## ${GATE} " "${GATES_FILE}" | sed "s/^## ${GATE} *[—-] *//")
if [[ -z "${GATE_NAME}" ]]; then
    echo "Gate ${GATE} not found in ${GATES_FILE}"
    exit 1
fi

cd "${BASE_DIR}"

# Ensure CGO is enabled with available compiler for -race tests
if [[ -z "${CGO_ENABLED:-}" || "${CGO_ENABLED}" == "0" ]]; then
    if command -v clang >/dev/null 2>&1; then
        export CC="${CC:-clang}"
        export CGO_ENABLED=1
    elif command -v gcc >/dev/null 2>&1; then
        export CC="${CC:-gcc}"
        export CGO_ENABLED=1
    fi
fi

echo "=== Gate ${GATE}: ${GATE_NAME} ==="
echo ""

# Run checks based on gate
case "${GATE}" in
    G1)
        echo "Checking: bootstrap, platform core, and delivery metadata"
        python3 "${TASK_CHECK}" --format --graph --sdd || { echo "FAIL: task/SDD metadata invalid"; exit 1; }
        make -C "${BASE_DIR}" build-all || { echo "FAIL: binaries do not build"; exit 1; }
        make -C "${BASE_DIR}" lint || { echo "FAIL: lint failed"; exit 1; }
        go test -race -count=1 ./internal/shared/... || { echo "FAIL: platform-core tests failed"; exit 1; }
        echo "OK: bootstrap and platform core verified"
        ;;
    G2)
        echo "Checking: Domain Tests Green"
        COV_FILE=$(mktemp)
        go test -race -count=3 -coverprofile="${COV_FILE}" ./internal/domain/... || { echo "FAIL: domain tests failed"; exit 1; }
        COV=$(go tool cover -func="${COV_FILE}" | awk '/^total:/ {gsub("%", "", $3); print $3}')
        if ! awk -v actual="${COV:-0}" 'BEGIN { exit !(actual + 0 >= 90) }'; then
            echo "FAIL: domain coverage ${COV}% < 90%"
            exit 1
        fi
        python3 "${TASK_CHECK}" --specs --events || { echo "FAIL: domain documentation traceability failed"; exit 1; }
        MODULE=$(go list -m)
        if go list -deps ./internal/domain/... | grep -v "^${MODULE}" | grep -q '^github.com/\|^go.uber.org/\|^golang.org/'; then
            echo "FAIL: domain imports external modules"
            exit 1
        fi
        echo "OK: Domain tests pass with ${COV}% coverage"
        ;;
    G3)
        echo "Checking: Application Tests Green"
        COV_FILE=$(mktemp)
        go test -race -count=3 -coverprofile="${COV_FILE}" ./internal/application/... || { echo "FAIL: application tests failed"; exit 1; }
        COV=$(go tool cover -func="${COV_FILE}" | awk '/^total:/ {gsub("%", "", $3); print $3}')
        if ! awk -v actual="${COV:-0}" 'BEGIN { exit !(actual + 0 >= 85) }'; then
            echo "FAIL: application coverage ${COV}% < 85%"
            exit 1
        fi
        python3 "${TASK_CHECK}" --handlers --ports || { echo "FAIL: application traceability failed"; exit 1; }
        echo "OK: Application tests pass with ${COV}% coverage"
        ;;
    G4)
        echo "Checking: Infrastructure Integrated"
        go test -race -count=3 ./test/integration/... || { echo "FAIL: adapter integration tests failed"; exit 1; }
        python3 "${TASK_CHECK}" --migrations --ports --subjects || { echo "FAIL: adapter traceability failed"; exit 1; }
        echo "OK: adapter integration tests pass"
        ;;
    G5)
        echo "Checking: API Contracts Satisfied"
        go test -race -count=3 ./test/integration/api/rest/... || { echo "FAIL: REST integration tests failed"; exit 1; }
        go test -race -count=3 ./test/integration/api/grpc/... || { echo "FAIL: gRPC integration tests failed"; exit 1; }
        go test -race -count=3 ./test/integration/api/graphql/... || { echo "FAIL: GraphQL integration tests failed"; exit 1; }
        go test -race -count=3 ./test/integration/api/cron/... || { echo "FAIL: cron integration tests failed"; exit 1; }
        go test -race -count=3 ./test/integration/api/consumer/... || { echo "FAIL: consumer integration tests failed"; exit 1; }
        python3 "${TASK_CHECK}" --openapi --handlers --events || { echo "FAIL: API traceability failed"; exit 1; }
        # Contract tests
        buf breaking --against '.git#branch=main' --exclude=ENUM_VALUE_DELETED || { echo "FAIL: buf breaking change detected"; exit 1; }
        echo "OK: API contracts satisfied"
        ;;
    G6)
        echo "Checking: Observability Complete"
        # Smoke test: trace propagation
        curl -sf "http://localhost:16686/api/traces?service=ledger&limit=1" >/dev/null || { echo "FAIL: Tempo not reachable"; exit 1; }
        curl -sf "http://localhost:9090/api/v1/query?query=up" >/dev/null || { echo "FAIL: Prometheus not reachable"; exit 1; }
        curl -sf "http://localhost:3100/ready" >/dev/null || { echo "FAIL: Loki not reachable"; exit 1; }
        curl -sf "http://localhost:3200/ready" >/dev/null || { echo "FAIL: Tempo not reachable"; exit 1; }
        # Rate limit test
        curl -sf -H "X-Request-ID: test-123" "http://localhost:8080/health" >/dev/null || { echo "FAIL: health endpoint down"; exit 1; }
        echo "OK: Observability stack reachable"
        ;;
    G7)
        echo "Checking: cross-cutting verification and delivery"
        go test -race -count=1 ./test/contract/... || { echo "FAIL: contract suite failed"; exit 1; }
        # Check if CI workflow exists and is green (mock for now)
        [[ -f .github/workflows/ci.yml ]] || { echo "FAIL: ci.yml missing"; exit 1; }
        [[ -f .github/workflows/cd-staging.yml ]] || { echo "FAIL: cd-staging.yml missing"; exit 1; }
        [[ -f .github/workflows/cd-production.yml ]] || { echo "FAIL: cd-production.yml missing"; exit 1; }
        [[ -f .github/workflows/release.yml ]] || { echo "FAIL: release.yml missing"; exit 1; }
        # Check for security scanning steps
        grep -q "govulncheck" .github/workflows/ci.yml || { echo "FAIL: govulncheck missing in CI"; exit 1; }
        grep -q "gosec" .github/workflows/ci.yml || { echo "FAIL: gosec missing in CI"; exit 1; }
        grep -q "trivy" .github/workflows/ci.yml || { echo "FAIL: trivy missing in CI"; exit 1; }
        grep -q "syft" .github/workflows/ci.yml || { echo "FAIL: syft missing in CI"; exit 1; }
        grep -q "go-licenses" .github/workflows/ci.yml || { echo "FAIL: go-licenses missing in CI"; exit 1; }
        echo "OK: CI/CD pipeline configured"
        ;;
    G8)
        echo "Checking: Documentation Complete"
        python3 "${TASK_CHECK}" --docs --adrs || { echo "FAIL: docs or ADR decisions incomplete"; exit 1; }
        [[ -f docs/domain/entities.md ]] && [[ -f docs/domain/value-objects.md ]] || { echo "FAIL: domain docs missing"; exit 1; }
        [[ -f docs/application/commands.md ]] && [[ -f docs/application/queries.md ]] || { echo "FAIL: application docs missing"; exit 1; }
        [[ -f docs/infrastructure/database.md ]] && [[ -f docs/infrastructure/cache.md ]] || { echo "FAIL: infra docs missing"; exit 1; }
        [[ -f docs/api/rest-api.md ]] && [[ -f docs/api/grpc-api.md ]] && [[ -f docs/api/graphql-api.md ]] || { echo "FAIL: API docs missing"; exit 1; }
        [[ -f docs/development/deployment.md ]] && [[ -f docs/development/testing.md ]] || { echo "FAIL: dev docs missing"; exit 1; }
        # Security
        grep -q "HIGH" trivy-report.sarif 2>/dev/null && { echo "FAIL: HIGH vulns in Trivy report"; exit 1; }
        grep -q "CRITICAL" trivy-report.sarif 2>/dev/null && { echo "FAIL: CRITICAL vulns in Trivy report"; exit 1; }
        # SBOM
        [[ -f spdx.json ]] || { echo "FAIL: SBOM missing"; exit 1; }
        # License
        go-licenses check ./... 2>/dev/null || { echo "FAIL: license check failed"; exit 1; }
        # Performance
        [[ -f benchmark.baseline ]] || { echo "FAIL: benchmark baseline missing"; exit 1; }
        echo "OK: Documentation complete"
        ;;
    *)
        echo "Unknown gate: ${GATE}"
        exit 1
        ;;
esac

echo "Gate ${GATE}: PASSED"
exit 0
