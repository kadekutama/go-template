// Package testcontainers provides one shared Docker lifecycle harness for
// integration suites (E07-T09). Every dependency gets a start/wait-ready/
// connection/terminate helper with pinned images and bounded contexts.
// Helpers skip (never fail) when no Docker daemon is reachable.
package testcontainers

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Profile selects resource sizing for started containers.
type Profile string

const (
	// ProfileLocal is the default developer profile.
	ProfileLocal Profile = "local"
	// ProfileCI reduces memory/CPU requests for CI runners.
	ProfileCI Profile = "ci"
)

// DefaultTimeout bounds container startup; CI uses the shorter bound.
const DefaultTimeout = 120 * time.Second

// CITimeout bounds container startup on CI runners.
const CITimeout = 60 * time.Second

// ActiveProfile resolves the profile from TESTCONTAINERS_PROFILE (ci|local).
func ActiveProfile() Profile {
	if os.Getenv("TESTCONTAINERS_PROFILE") == "ci" {
		return ProfileCI
	}
	return ProfileLocal
}

// StartupTimeout returns the bounded startup timeout for the active profile.
func StartupTimeout() time.Duration {
	if ActiveProfile() == ProfileCI {
		return CITimeout
	}
	return DefaultTimeout
}

// SkipIfNoDocker skips the calling test when no Docker daemon is reachable.
// Container tests must call this first so unit jobs stay green without Docker.
func SkipIfNoDocker(t *testing.T) {
	t.Helper()

	if os.Getenv("TESTCONTAINERS_SKIP") == "1" {
		t.Skip("skipping container test: TESTCONTAINERS_SKIP=1")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//nolint:gosec // test helper probes the local Docker daemon only.
	cmd := exec.CommandContext(ctx, "docker", "info")
	if err := cmd.Run(); err != nil {
		t.Skipf("skipping container test: no reachable Docker daemon: %v", err)
	}
}

// Background returns a bounded context for container startup.
func Background() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), StartupTimeout())
}
