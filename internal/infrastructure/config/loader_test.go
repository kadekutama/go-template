package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
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

func TestLoadNonExistentFile(t *testing.T) {
	t.Parallel()

	_, err := Load("non-existent-config-file.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
	if !strings.Contains(err.Error(), "load config file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	t.Parallel()

	malformed := writeTemp(t, "app:\n  name: [unclosed")
	_, err := Load(malformed)
	if err == nil {
		t.Fatal("expected error for malformed yaml, got nil")
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

func TestSecretRefFailsClosed(t *testing.T) {
	t.Parallel()

	_, err := Load(writeTemp(t, "database:\n  password: \"{{ secret:db/x }}\"\n"))
	if err == nil {
		t.Fatal("expected secret error, got nil")
	}
	var secretErr *SecretRefError
	if !errors.As(err, &secretErr) {
		t.Fatalf("expected *SecretRefError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "E09-T05") {
		t.Errorf("error should name E09-T05, got: %v", err)
	}
}

func TestSecretRefInArrayFailsClosed(t *testing.T) {
	t.Parallel()

	body := "app:\n  name: test\ncustom_list:\n  - item1\n  - \"{{ secret:vault/token }}\"\n"
	_, err := Load(writeTemp(t, body))
	if err == nil {
		t.Fatal("expected secret error for array element, got nil")
	}
	var secretErr *SecretRefError
	if !errors.As(err, &secretErr) {
		t.Fatalf("expected *SecretRefError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "custom_list[1]") {
		t.Errorf("expected error to name custom_list[1], got: %v", err)
	}
}

func TestStagingAndProductionFailOnSecretRefs(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"config.staging.yaml", "config.production.yaml"} {
		_, err := Load(repoConfig("config.yaml"), repoConfig(name))
		var secretErr *SecretRefError
		if !errors.As(err, &secretErr) {
			t.Errorf("%s: expected *SecretRefError (no secret values may load pre-E09), got %v", name, err)
		}
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
