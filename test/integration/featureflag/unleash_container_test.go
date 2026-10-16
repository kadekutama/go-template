package featureflag_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/moby/moby/api/types/network"
	tc "github.com/testcontainers/testcontainers-go"
	tcnetwork "github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/kadekutama/go-template/pkg/jsonparser"
	"github.com/kadekutama/go-template/test/testcontainers"

	// Blank import registers the pgx stdlib driver for the Postgres
	// readiness probe, mirroring the shared harness.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Unleash container pins. The server matches SPEC §12.2 (6.5 line); Postgres
// matches the shared harness pin.

// Static test sentinels: Admin API failures wrap these so no dynamic error
// is ever defined inline (the repository lints tests with err113).
var (
	errUnleashLogin  = errors.New("featureflag test: unleash login failed")
	errUnleashToken  = errors.New("featureflag test: unleash token creation failed")
	errUnleashFlag   = errors.New("featureflag test: unleash flag operation failed")
	errUnleashRandom = errors.New("featureflag test: random generation failed")
)

const (
	unleashServerImage = "unleashorg/unleash-server:6.5.1"
	unleashPGImage     = "postgres:18.6"
	unleashPGUser      = "unleash"
	unleashPGPassword  = "unleash_test_pw"
	unleashPGDB        = "unleash"
	unleashPGalias     = "unleashpg"
	unleashAdminUser   = "admin"
)

// unleashHarness is one isolated Unleash server + Postgres pair on a private
// Docker network with an admin session and a backend client token. Everything
// runs in local containers: no vendor credentials, no external network.
type unleashHarness struct {
	network     *tc.DockerNetwork
	postgres    tc.Container
	server      tc.Container
	baseURL     string
	adminClient *http.Client
	clientToken string
}

// startUnleashHarness boots Postgres, waits for SQL readiness, boots Unleash
// against it, waits for HTTP readiness, logs in as the seeded admin, and
// mints one backend client token. The caller terminates the harness.
func startUnleashHarness(ctx context.Context) (*unleashHarness, error) {
	dockerNet, err := tcnetwork.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create unleash network: %w", err)
	}

	harness := &unleashHarness{network: dockerNet}

	postgres, err := startUnleashPostgres(ctx, dockerNet.Name)
	if err != nil {
		harness.terminate(ctx)

		return nil, err
	}

	harness.postgres = postgres

	adminPassword, err := randomHex(16)
	if err != nil {
		harness.terminate(ctx)

		return nil, err
	}

	server, err := startUnleashServer(ctx, dockerNet.Name, adminPassword)
	if err != nil {
		harness.terminate(ctx)

		return nil, err
	}

	harness.server = server

	host, err := server.Host(ctx)
	if err != nil {
		harness.terminate(ctx)

		return nil, fmt.Errorf("unleash host: %w", err)
	}

	mapped, err := server.MappedPort(ctx, "4242")
	if err != nil {
		harness.terminate(ctx)

		return nil, fmt.Errorf("map unleash port: %w", err)
	}

	harness.baseURL = "http://" + net.JoinHostPort(host, mapped.Port())

	jar, err := cookiejar.New(nil)
	if err != nil {
		harness.terminate(ctx)

		return nil, fmt.Errorf("cookie jar: %w", err)
	}

	harness.adminClient = &http.Client{Jar: jar, Timeout: 10 * time.Second}

	if err := harness.loginAdmin(ctx, unleashAdminUser, adminPassword); err != nil {
		harness.terminate(ctx)

		return nil, err
	}

	token, err := harness.createBackendToken(ctx)
	if err != nil {
		harness.terminate(ctx)

		return nil, err
	}

	harness.clientToken = token

	return harness, nil
}

// terminate stops both containers and removes the network; nil-safe.
// Termination failures are logged to stderr: cleanup runs without *testing.T.
func (h *unleashHarness) terminate(ctx context.Context) {
	if h == nil {
		return
	}

	if h.server != nil {
		if err := h.server.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "terminate unleash: %v\n", err)
		}

		h.server = nil
	}

	if h.postgres != nil {
		if err := h.postgres.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "terminate unleash postgres: %v\n", err)
		}

		h.postgres = nil
	}

	if h.network != nil {
		if err := h.network.Remove(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "remove unleash network: %v\n", err)
		}

		h.network = nil
	}
}

// startUnleashPostgres boots the Unleash database on the harness network and
// waits for SQL readiness (TCP accept alone flakes first queries).
func startUnleashPostgres(ctx context.Context, networkName string) (tc.Container, error) {
	req := tc.GenericContainerRequest{
		Image:    unleashPGImage,
		Hostname: unleashPGalias,
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {unleashPGalias},
		},
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     unleashPGUser,
			"POSTGRES_PASSWORD": unleashPGPassword,
			"POSTGRES_DB":       unleashPGDB,
		},
		WaitingFor: wait.ForSQL("5432/tcp", "pgx", func(host string, port network.Port) string {
			return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
				unleashPGUser, unleashPGPassword, net.JoinHostPort(host, port.Port()), unleashPGDB)
		}).WithStartupTimeout(testcontainers.StartupTimeout()).WithPollInterval(500 * time.Millisecond),
		Started: true,
	}

	container, err := tc.GenericContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("start unleash postgres: %w", err)
	}

	return container, nil
}

// startUnleashServer boots the Unleash server against the harness Postgres
// and waits for HTTP readiness (migrations run on first boot). Auth stays
// enabled; the admin user is seeded from a random per-run password and the
// backend token is minted through the Admin API after login (no token format
// is guessed, no static credential is baked in).
func startUnleashServer(ctx context.Context, networkName, adminPassword string) (tc.Container, error) {
	req := tc.GenericContainerRequest{
		Image:    unleashServerImage,
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"unleash"},
		},
		ExposedPorts: []string{"4242/tcp"},
		Env: map[string]string{
			"DATABASE_HOST":                  unleashPGalias,
			"DATABASE_PORT":                  "5432",
			"DATABASE_NAME":                  unleashPGDB,
			"DATABASE_USERNAME":              unleashPGUser,
			"DATABASE_PASSWORD":              unleashPGPassword,
			"DATABASE_SSL":                   "false",
			"UNLEASH_DEFAULT_ADMIN_USERNAME": unleashAdminUser,
			"UNLEASH_DEFAULT_ADMIN_PASSWORD": adminPassword,
			"CHECK_VERSION":                  "false",
		},
		WaitingFor: wait.ForHTTP("/health").WithPort("4242/tcp").WithStartupTimeout(testcontainers.StartupTimeout()),
		Started:    true,
	}

	container, err := tc.GenericContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("start unleash server: %w", err)
	}

	return container, nil
}

// loginAdmin establishes the admin session cookie used for flag and token
// administration.
func (h *unleashHarness) loginAdmin(ctx context.Context, username, password string) error {
	body, err := jsonparser.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		return fmt.Errorf("encode login: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+"/auth/simple/login", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := h.adminClient.Do(req)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	defer drainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", errUnleashLogin, resp.StatusCode)
	}

	return nil
}

// createBackendToken mints one development backend token for SDK polling.
func (h *unleashHarness) createBackendToken(ctx context.Context) (string, error) {
	body, err := jsonparser.Marshal(map[string]any{
		"username":    "featureflag-test",
		"type":        "client",
		"project":     "default",
		"environment": "development",
	})
	if err != nil {
		return "", fmt.Errorf("encode token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+"/api/admin/api-tokens", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := h.adminClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create token: %w", err)
	}

	defer drainBody(resp)

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: status %d: %s", errUnleashToken, resp.StatusCode, string(raw))
	}

	var payload struct {
		Secret string `json:"secret"`
	}

	if err := jsonparser.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if payload.Secret == "" {
		return "", fmt.Errorf("%w: empty secret", errUnleashToken)
	}

	return payload.Secret, nil
}

// createFlag registers one toggle; 2xx (201 created, 200 exists) succeeds so
// -count repeats stay green. A 4xx fails loudly with the bounded body so a
// naming-constraint mismatch surfaces instead of hiding.
func (h *unleashHarness) createFlag(ctx context.Context, name string) error {
	body, err := jsonparser.Marshal(map[string]string{"name": name})
	if err != nil {
		return fmt.Errorf("encode flag: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+"/api/admin/projects/default/features", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("flag request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := h.adminClient.Do(req)
	if err != nil {
		return fmt.Errorf("create flag: %w", err)
	}

	defer drainBody(resp)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

		return fmt.Errorf("%w: %q status: %d: %s", errUnleashFlag, name, resp.StatusCode, string(raw))
	}

	return nil
}

// setFlagEnabled flips one toggle on or off in the development environment
// through the Admin API.
func (h *unleashHarness) setFlagEnabled(ctx context.Context, name string, enabled bool) error {
	action := "off"
	if enabled {
		action = "on"
	}

	endpoint := h.baseURL + "/api/admin/projects/default/features/" + url.PathEscape(name) + "/environments/development/" + action

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("toggle request: %w", err)
	}

	resp, err := h.adminClient.Do(req)
	if err != nil {
		return fmt.Errorf("toggle flag: %w", err)
	}

	defer drainBody(resp)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("%w: %q status: %d", errUnleashFlag, name, resp.StatusCode)
	}

	return nil
}

// drainBody discards and closes a response body, logging failures to stderr:
// cleanup and test paths never silently ignore them.
func drainBody(resp *http.Response) {
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, 4096)); err != nil {
		fmt.Fprintf(os.Stderr, "drain body: %v\n", err)
	}

	if err := resp.Body.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close body: %v\n", err)
	}
}

// randomHex returns n random bytes as hex for per-run credentials. No static
// secret is baked into the test.
func randomHex(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("%w: %w", errUnleashRandom, err)
	}

	return hex.EncodeToString(raw), nil
}

// dockerReachable reports whether test containers can run here (mirrors
// testcontainers.SkipIfNoDocker for TestMain, which has no *testing.T).
func dockerReachable() bool {
	if os.Getenv("TESTCONTAINERS_SKIP") == "1" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return exec.CommandContext(ctx, "docker", "info").Run() == nil
}
