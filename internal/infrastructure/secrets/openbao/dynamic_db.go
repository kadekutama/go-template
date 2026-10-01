package openbao

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// DBCredentials is one leased ephemeral database credential.
type DBCredentials struct {
	Username      string
	Password      string
	LeaseID       string
	Renewable     bool
	LeaseDuration time.Duration
	IssuedAt      time.Time
}

// dbCredsResponse is the minimal database-creds payload we consume.
type dbCredsResponse struct {
	LeaseID       string `json:"lease_id"`
	Renewable     bool   `json:"renewable"`
	LeaseDuration int    `json:"lease_duration"`
	Data          struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"data"`
}

// LeaseDB leases one ephemeral credential for role (1h policy enforced server-side).
func (s *Store) LeaseDB(ctx context.Context, role string) (DBCredentials, error) {
	if s == nil || s.http == nil || s.clock == nil {
		return DBCredentials{}, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return DBCredentials{}, fmt.Errorf("openbao: lease db: %w", err)
	}

	if role == "" {
		return DBCredentials{}, fmt.Errorf("%w: role required", ErrLeaseFailed)
	}

	payload, err := s.requestCreds(ctx, role)
	if err != nil {
		return DBCredentials{}, err
	}

	if payload.Data.Username == "" || payload.Data.Password == "" {
		return DBCredentials{}, fmt.Errorf("%w: %s empty credentials", ErrLeaseFailed, role)
	}

	return DBCredentials{
		Username:      payload.Data.Username,
		Password:      payload.Data.Password,
		LeaseID:       payload.LeaseID,
		Renewable:     payload.Renewable,
		LeaseDuration: time.Duration(payload.LeaseDuration) * time.Second,
		IssuedAt:      s.now(),
	}, nil
}

// requestCreds performs the database-creds wire call for role.
func (s *Store) requestCreds(ctx context.Context, role string) (dbCredsResponse, error) {
	path := fmt.Sprintf("%s/v1/%s/creds/%s", s.address, s.dbMount, role)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return dbCredsResponse{}, fmt.Errorf("%w: %s", ErrLeaseFailed, role)
	}

	req.Header.Set("X-Vault-Token", s.token)

	resp, err := s.http.Do(req)
	if err != nil {
		return dbCredsResponse{}, fmt.Errorf("%w: %s", ErrLeaseFailed, role)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return dbCredsResponse{}, fmt.Errorf("%w: %s status %d", ErrLeaseFailed, role, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return dbCredsResponse{}, fmt.Errorf("%w: %s", ErrLeaseFailed, role)
	}

	var payload dbCredsResponse
	if err := jsonparser.Unmarshal(body, &payload); err != nil {
		return dbCredsResponse{}, fmt.Errorf("%w: %s", ErrLeaseFailed, role)
	}

	return payload, nil
}

// RenewLease renews one lease without restart (sys/leases/renew wire call).
func (s *Store) RenewLease(ctx context.Context, leaseID string) (time.Duration, error) {
	if s == nil || s.http == nil || s.clock == nil {
		return 0, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("openbao: renew: %w", err)
	}

	if leaseID == "" {
		return 0, fmt.Errorf("%w: lease id required", ErrLeaseFailed)
	}

	body, _ := jsonparser.Marshal(map[string]string{"lease_id": leaseID})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.address+"/v1/sys/leases/renew", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrLeaseFailed, leaseID)
	}

	req.Header.Set("X-Vault-Token", s.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrLeaseFailed, leaseID)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%w: %s status %d", ErrLeaseFailed, leaseID, resp.StatusCode)
	}

	var payload struct {
		LeaseDuration int `json:"lease_duration"`
	}

	if body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)); err != nil {
		return 0, fmt.Errorf("%w: %s", ErrLeaseFailed, leaseID)
	} else if err := jsonparser.Unmarshal(body, &payload); err != nil {
		return 0, fmt.Errorf("%w: %s", ErrLeaseFailed, leaseID)
	}

	return time.Duration(payload.LeaseDuration) * time.Second, nil
}
