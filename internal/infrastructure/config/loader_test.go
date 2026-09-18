package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
)

// repoConfig anchors tests to the committed config/ tree regardless of where
// the suite is invoked from (go test always runs in the package directory).
func repoConfig(name string) string {
	return filepath.Join("..", "..", "..", "config", name)
}

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoadBaseConfig(t *testing.T) {
	t.Parallel()

	cfg, err := Load(repoConfig("config.yaml"))
	if err != nil {
		t.Fatalf("Load base: %v", err)
	}
	if cfg.App.Name != "go-template" || cfg.App.Env != "local" {
		t.Errorf("unexpected app section: %+v", cfg.App)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("unexpected server port: %d", cfg.Server.Port)
	}
}

func TestLoadFailures(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		filePath      func() string
		expectedError string
	}

	testCases := []testCase{
		{
			name: "non-existent config file fails",
			filePath: func() string {
				return "non-existent-config-file.yaml"
			},
			expectedError: "load config file",
		},
		{
			name: "malformed yaml fails",
			filePath: func() string {
				return writeTemp(t, "app:\n  name: [unclosed")
			},
			expectedError: "yaml:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(tc.filePath())
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestMissingRequiredListsAllViolations(t *testing.T) {
	t.Parallel()

	_, err := Load(writeTemp(t, "app:\n  name: x\n"))
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	for _, want := range []string{"Server", "Database", "Cache", "Auth", "NATS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name missing section %s, got: %v", want, err)
		}
	}
}

func TestEnvOverrideWins(t *testing.T) {
	t.Setenv("APP_SERVER__PORT", "9999")

	cfg, err := Load(repoConfig("config.yaml"))
	if err != nil {
		t.Fatalf("Load with env override: %v", err)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("env override lost: got port %d, want 9999", cfg.Server.Port)
	}
}

func TestOverlayWins(t *testing.T) {
	t.Parallel()

	base := writeTemp(t, "server:\n  host: 127.0.0.1\n  port: 1\napp:\n  name: x\n  env: local\ndatabase:\n  host: h\n  port: 1\n  user: u\n  password: p\n  name: n\n  sslmode: disable\ncache:\n  host: h\n  port: 1\nauth:\n  issuer: http://x\n  access_ttl_min: 1\n  refresh_ttl_days: 1\nnats:\n  url: nats://x:4222\n")
	overlay := writeTemp(t, "server:\n  port: 1234\n")

	cfg, err := Load(base, overlay)
	if err != nil {
		t.Fatalf("Load layered: %v", err)
	}
	if cfg.Server.Port != 1234 {
		t.Errorf("overlay lost: got port %d, want 1234", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("base value lost: got host %q", cfg.Server.Host)
	}
}

func TestSecretRefValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		yamlBody      string
		expectedError string
	}

	testCases := []testCase{
		{
			name:          "scalar secret ref rejected with E09-T05",
			yamlBody:      "database:\n  password: \"{{ secret:db/x }}\"\n",
			expectedError: "E09-T05",
		},
		{
			name:          "array element secret ref rejected with path",
			yamlBody:      "app:\n  name: test\ncustom_list:\n  - item1\n  - \"{{ secret:vault/token }}\"\n",
			expectedError: "custom_list[1]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(writeTemp(t, tc.yamlBody))
			var secretErr *SecretRefError
			assert.ErrorAs(t, err, &secretErr)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestStagingAndProductionFailOnSecretRefs(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name       string
		configFile string
	}

	testCases := []testCase{
		{
			name:       "staging config fails on secret refs",
			configFile: "config.staging.yaml",
		},
		{
			name:       "production config fails on secret refs",
			configFile: "config.production.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(repoConfig("config.yaml"), repoConfig(tc.configFile))
			var secretErr *SecretRefError
			assert.ErrorAs(t, err, &secretErr)
		})
	}
}

// koanfLeafPaths reflects over koanf tags to list every configuration leaf,
// e.g. server.port. New Config fields are covered automatically.
func koanfLeafPaths(t *testing.T, typ reflect.Type, prefix string, out *[]string) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("koanf")
		if tag == "" || tag == "-" {
			continue
		}
		path := tag
		if prefix != "" {
			path = prefix + "." + tag
		}
		fieldType := field.Type
		if fieldType.Kind() == reflect.Struct {
			koanfLeafPaths(t, fieldType, path, out)
			continue
		}
		*out = append(*out, path)
	}
}

func TestSchemaAlignsWithStruct(t *testing.T) {
	t.Parallel()

	var leaves []string
	koanfLeafPaths(t, reflect.TypeOf(Config{}), "", &leaves)
	if len(leaves) == 0 {
		t.Fatal("no koanf leaf paths reflected; check Config tags")
	}

	schema := koanf.New(".")
	if err := schema.Load(file.Provider(repoConfig("schemas/config.json")), json.Parser()); err != nil {
		t.Fatalf("load schema: %v", err)
	}

	toSchemaPath := func(configPath string) string {
		return "properties." + strings.ReplaceAll(configPath, ".", ".properties.")
	}
	for _, leaf := range leaves {
		if !schema.Exists(toSchemaPath(leaf)) {
			t.Errorf("schema missing Config leaf %q (want key %q)", leaf, toSchemaPath(leaf))
		}
	}

	required, ok := schema.Get("required").([]any)
	if !ok {
		t.Fatal("schema root has no required array")
	}
	have := map[string]bool{}
	for _, entry := range required {
		name, _ := entry.(string)
		have[name] = true
	}
	for _, section := range []string{"app", "server", "database", "cache", "auth", "nats"} {
		if !have[section] {
			t.Errorf("schema root required[] missing section %q", section)
		}
	}
}

func TestDatabasePoolSettings(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		config           DatabaseConfig
		expectedMaxOpen  int
		expectedMaxIdle  int
		expectedLifetime time.Duration
	}

	testCases := []testCase{
		{
			name:             "explicit knobs map through",
			config:           DatabaseConfig{MaxOpenConns: 25, MaxIdleConns: 5, ConnMaxLifetimeSec: 1800},
			expectedMaxOpen:  25,
			expectedMaxIdle:  5,
			expectedLifetime: 30 * time.Minute,
		},
		{
			name:             "unset knobs stay zero for driver defaults",
			config:           DatabaseConfig{},
			expectedMaxOpen:  0,
			expectedMaxIdle:  0,
			expectedLifetime: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			maxOpen, maxIdle, lifetime := tc.config.PoolSettings()
			assert.Equal(t, tc.expectedMaxOpen, maxOpen)
			assert.Equal(t, tc.expectedMaxIdle, maxIdle)
			assert.Equal(t, tc.expectedLifetime, lifetime)
		})
	}
}

func TestCoordinationEnvOverrides(t *testing.T) {
	t.Setenv("APP_COORDINATION__ETCD_ENDPOINTS", "http://etcd-a:2379,http://etcd-b:2379")
	t.Setenv("APP_COORDINATION__ETCD_DIAL_TIMEOUT_SEC", "7")
	t.Setenv("APP_COORDINATION__ETCD_ELECTION_TTL_SEC", "9")
	t.Setenv("APP_COORDINATION__LEADER_KEY_PREFIX", "/custom/leader")
	t.Setenv("APP_OBSERVABILITY__LOG_LEVEL", "debug")

	cfg, err := Load(repoConfig("config.yaml"))
	if err != nil {
		t.Fatalf("Load with coordination env overrides: %v", err)
	}

	assert.Equal(t, []string{"http://etcd-a:2379", "http://etcd-b:2379"}, cfg.Coordination.EtcdEndpoints)
	assert.Equal(t, 7, cfg.Coordination.EtcdDialTimeoutSec)
	assert.Equal(t, 9, cfg.Coordination.EtcdElectionTTLSeconds)
	assert.Equal(t, "/custom/leader", cfg.Coordination.LeaderKeyPrefix)
	assert.Equal(t, "debug", cfg.Observability.LogLevel)
}
