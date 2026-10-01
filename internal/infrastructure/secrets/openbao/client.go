package openbao

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// OpenBaoParams carries constructor dependencies (Parameter Object pattern).
// DefaultTTL bounds the ExpiresAt hint returned with each secret; values are
// never cached in the adapter (every Get is a strong read against OpenBao),
// so rotation lands on the next read without restart.
type OpenBaoParams struct {
	Address    string        `validate:"required,url"`
	Token      string        `validate:"required,min=8"`
	KVMount    string        `validate:"required,max=64"`
	DBMount    string        `validate:"required,max=64"`
	DefaultTTL time.Duration `validate:"required,gt=0"`
	HTTP       httpclient.Doer
	Clock      appport.Clock
}

// Store implements port.SecretStore over the OpenBao/Vault HTTP API. It is
// stateless: no maps, no locks, no cached values.
type Store struct {
	address    string
	token      string
	kvMount    string
	dbMount    string
	defaultTTL time.Duration
	http       httpclient.Doer
	clock      appport.Clock
}

// Compile-time port conformance.
var _ appport.SecretStore = (*Store)(nil)

// NewStore builds the OpenBao adapter; HTTP and Clock must be non-nil and
// DBMount must be explicit (no silent "database" default per ADR-021).
func NewStore(params OpenBaoParams) (*Store, error) {
	if params.Address == "" || params.Token == "" || params.KVMount == "" || params.DBMount == "" {
		return nil, ErrConfigRequired
	}

	if params.HTTP == nil {
		return nil, ErrConfigRequired
	}

	if params.Clock == nil {
		return nil, ErrClockRequired
	}

	if params.DefaultTTL <= 0 {
		return nil, ErrConfigRequired
	}

	dbMount := params.DBMount

	return &Store{
		address:    trimSuffix(params.Address, "/"),
		token:      params.Token,
		kvMount:    params.KVMount,
		dbMount:    dbMount,
		defaultTTL: params.DefaultTTL,
		http:       params.HTTP,
		clock:      params.Clock,
	}, nil
}

// now returns the current UTC time through the injected clock.
func (s *Store) now() time.Time {
	return s.clock.Now().UTC()
}

// kvResponse is the minimal KVv2 read payload we consume.
type kvResponse struct {
	Data struct {
		Data map[string]any `json:"data"`
	} `json:"data"`
}

// Get returns one secret by tenant + name. Every call is a strong read
// against OpenBao: nothing is cached, so rotation is visible on the next
// read and a restart loses nothing.
func (s *Store) Get(ctx context.Context, tenant valueobject.TenantID, name string) (appport.Secret, error) {
	if s == nil || s.http == nil || s.clock == nil {
		return appport.Secret{}, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return appport.Secret{}, fmt.Errorf("openbao: get: %w", err)
	}

	if tenant == "" || name == "" {
		return appport.Secret{}, fmt.Errorf("%w: tenant and name required", ErrSecretNotFound)
	}

	value, err := s.fetchKV(ctx, string(tenant), name)
	if err != nil {
		return appport.Secret{}, err
	}

	return appport.Secret{Name: name, Value: value, ExpiresAt: s.now().Add(s.defaultTTL)}, nil
}

// fetchKV reads one KVv2 secret (tenant-scoped path). A missing secret maps
// to ErrSecretNotFound; transport errors, unexpected statuses, and undecodable
// bodies map to ErrSecretUnavailable so callers can distinguish an outage
// from a misconfiguration. Both fail closed.
func (s *Store) fetchKV(ctx context.Context, tenant, name string) ([]byte, error) {
	path := fmt.Sprintf("%s/v1/%s/data/%s/%s", s.address, s.kvMount, tenant, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSecretUnavailable, name)
	}

	req.Header.Set("X-Vault-Token", s.token)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSecretUnavailable, name)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s", ErrSecretNotFound, name)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s status %d", ErrSecretUnavailable, name, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSecretUnavailable, name)
	}

	var payload kvResponse
	if err := jsonparser.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSecretUnavailable, name)
	}

	raw, ok := payload.Data.Data["value"]
	if !ok {
		return nil, fmt.Errorf("%w: %s missing value", ErrSecretNotFound, name)
	}

	str, ok := raw.(string)
	if !ok || str == "" {
		return nil, fmt.Errorf("%w: %s missing value", ErrSecretNotFound, name)
	}

	return []byte(str), nil
}

// trimSuffix removes one trailing slash with the standard library.
func trimSuffix(s, suffix string) string {
	return strings.TrimSuffix(s, suffix)
}
