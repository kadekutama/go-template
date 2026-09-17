// Package persistence holds the G4 integration evidence for persistence
// adapters (E07-T06). Docker-gated suites run against PostgreSQL 18 via the
// shared harness; model-based and fault-matrix subsets always run (seeded,
// DB-free) so unit jobs stay green without a daemon.
package persistence
