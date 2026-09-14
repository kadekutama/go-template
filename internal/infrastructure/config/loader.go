package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// envPrefix is the environment-variable namespace. APP_DATABASE__HOST sets
// database.host; keys are lowercased and __ becomes the nesting delimiter.
const envPrefix = "APP_"

// secretMarker opens a secret reference resolved only by E09-T05.
const secretMarker = "{{ secret:"

// SecretRefError reports a {{ secret:… }} reference encountered before the
// E09-T05 SecretManager exists. It always fails closed: callers must supply a
// concrete value or wait for E09.
type SecretRefError struct {
	Key string
}

func (e *SecretRefError) Error() string {
	return fmt.Sprintf("config key %q references a secret (%s…); resolution belongs to E09-T05 SecretManager", e.Key, secretMarker)
}

// Load reads base, then each overlay in order, then APP_ env vars, validates
// the merged result, and rejects secret references. Later sources win.
// The base and every overlay path must exist; there is no implicit local-file
// lookup (pass config.local.yaml explicitly when desired).
func Load(base string, overlays ...string) (*Config, error) {
	k := koanf.New(".")

	files := append([]string{base}, overlays...)
	for _, path := range files {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("load config file %s: %w", path, err)
		}
	}

	if err := k.Load(env.Provider(envPrefix, ".", func(s string) string {
		return strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(s, envPrefix), "__", "."))
	}), nil); err != nil {
		return nil, fmt.Errorf("load env overrides: %w", err)
	}

	var cfg Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if err := checkSecretRefs(k.All()); err != nil {
		return nil, err
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// validate runs every rule and joins all violations into one error.
func validate(cfg *Config) error {
	v := validator.New()
	if err := v.Struct(cfg); err != nil {
		var verrs validator.ValidationErrors
		if !errors.As(err, &verrs) {
			return fmt.Errorf("validate config: %w", err)
		}
		parts := make([]string, 0, len(verrs))
		for _, fe := range verrs {
			parts = append(parts, fmt.Sprintf("%s: rule %q on value %v", fe.Namespace(), fe.Tag(), fe.Value()))
		}
		return fmt.Errorf("invalid config (%d violation(s)): %s", len(parts), strings.Join(parts, "; "))
	}
	return nil
}

// checkSecretRefs walks the merged raw map and fails on the first secret
// reference. Raw values are inspected (not the decoded struct) so no secret
// shape can slip through an `any`-typed field later.
func checkSecretRefs(raw map[string]any) error {
	var walk func(prefix string, node map[string]any) error
	walk = func(prefix string, node map[string]any) error {
		for key, val := range node {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			switch typed := val.(type) {
			case string:
				if strings.Contains(typed, secretMarker) {
					return &SecretRefError{Key: path}
				}
			case []any:
				if err := checkSecretSlice(path, typed, walk); err != nil {
					return err
				}
			case map[string]any:
				if err := walk(path, typed); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk("", raw)
}

func checkSecretSlice(path string, items []any, walk func(string, map[string]any) error) error {
	for i, elem := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if s, ok := elem.(string); ok && strings.Contains(s, secretMarker) {
			return &SecretRefError{Key: itemPath}
		}
		if m, ok := elem.(map[string]any); ok {
			if err := walk(itemPath, m); err != nil {
				return err
			}
		}
	}
	return nil
}
