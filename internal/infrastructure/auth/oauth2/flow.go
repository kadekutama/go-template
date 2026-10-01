package oauth2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// KVStore is the durable session/code seam (DIP): Valkey in production,
// `*_test.go` fakes in unit tests. Sessions are TTL-bound handshake state;
// the store surviving an app restart only resumes logins, it never grants
// access by itself — identity still requires the provider exchange.
type KVStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
}

// session is one pending login handshake, serialized as JSON in the store.
type session struct {
	Tenant      string    `json:"tenant"`
	Provider    string    `json:"provider"`
	RedirectURL string    `json:"redirect_url"`
	Verifier    string    `json:"verifier"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// sessionKey namespaces handshake blobs; codeKey namespaces single-use claims.
func sessionKey(state string) string { return "oauth:sess:" + state }
func codeKey(provider, code string) string {
	sum := sha256.Sum256([]byte(provider + "\x00" + code))

	return "oauth:code:" + hex.EncodeToString(sum[:])
}

// Provider implements port.OAuthProvider with PKCE + signed state. It holds
// no maps and no locks: all mutable state lives in KVStore.
type Provider struct {
	providers map[string]ProviderConfig
	secret    []byte
	stateTTL  time.Duration
	http      httpclient.Doer
	kv        KVStore
	clock     appport.Clock
}

// Compile-time port conformance.
var _ appport.OAuthProvider = (*Provider)(nil)

// NewProvider builds the OAuth adapter; HTTP, KV, and Clock must be non-nil.
func NewProvider(params OAuthParams) (*Provider, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	providers := make(map[string]ProviderConfig, len(params.Providers))
	maps.Copy(providers, params.Providers)

	return &Provider{
		providers: providers,
		secret:    append([]byte(nil), params.StateSecret...),
		stateTTL:  params.StateTTL,
		http:      params.HTTP,
		kv:        params.KV,
		clock:     params.Clock,
	}, nil
}

// now returns the current UTC time through the injected clock.
func (p *Provider) now() time.Time {
	return p.clock.Now().UTC()
}

// AuthURL starts one login handshake for provider.
func (p *Provider) AuthURL(ctx context.Context, tenant valueobject.TenantID, provider, redirectURL string) (appport.OAuthSession, error) {
	if p == nil || p.http == nil || p.kv == nil || p.clock == nil {
		return appport.OAuthSession{}, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return appport.OAuthSession{}, fmt.Errorf("oauth2: auth url: %w", err)
	}

	cfg, ok := p.providers[provider]
	if !ok {
		return appport.OAuthSession{}, ErrUnknownProvider
	}

	if tenant == "" || redirectURL == "" {
		return appport.OAuthSession{}, fmt.Errorf("%w: tenant and redirect are required", ErrStateInvalid)
	}

	verifier, err := newVerifier()
	if err != nil {
		return appport.OAuthSession{}, fmt.Errorf("oauth2: verifier: %w", err)
	}

	nonce := newNonce()
	exp := p.now().Add(p.stateTTL)
	state := signState(p.secret, string(tenant), provider, nonce, exp)

	blob, err := jsonparser.Marshal(session{
		Tenant:      string(tenant),
		Provider:    provider,
		RedirectURL: redirectURL,
		Verifier:    verifier,
		ExpiresAt:   exp,
	})
	if err != nil {
		return appport.OAuthSession{}, fmt.Errorf("oauth2: session: %w", err)
	}

	if err := p.kv.Set(ctx, sessionKey(state), blob, p.stateTTL); err != nil {
		return appport.OAuthSession{}, fmt.Errorf("oauth2: session store: %w", err)
	}

	authURL := cfg.AuthURL + "?" + url.Values{
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {redirectURL},
		"response_type":         {"code"},
		"scope":                 {strings.Join(cfg.Scopes, " ")},
		"state":                 {state},
		"code_challenge":        {challengeS256(verifier)},
		"code_challenge_method": {"S256"},
	}.Encode()

	return appport.OAuthSession{
		Provider:    provider,
		State:       state,
		RedirectURL: authURL,
	}, nil
}

// tokenResponse is the minimal OAuth token payload we consume.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// userInfoResponse is the minimal userinfo payload we consume.
type userInfoResponse struct {
	ID    string `json:"id"`
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Login string `json:"login"`
}

// Exchange trades one authorization code + state for an identity.
func (p *Provider) Exchange(ctx context.Context, tenant valueobject.TenantID, provider, code, state string) (appport.OAuthIdentity, error) {
	if p == nil || p.http == nil || p.kv == nil || p.clock == nil {
		return appport.OAuthIdentity{}, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return appport.OAuthIdentity{}, fmt.Errorf("oauth2: exchange: %w", err)
	}

	cfg, ok := p.providers[provider]
	if !ok {
		return appport.OAuthIdentity{}, ErrUnknownProvider
	}

	if err := verifyState(p.secret, state, string(tenant), provider, p.now()); err != nil {
		return appport.OAuthIdentity{}, err
	}

	sess, err := p.consumeSession(ctx, tenant, provider, state)
	if err != nil {
		return appport.OAuthIdentity{}, err
	}

	if err := p.claimCode(ctx, provider, code); err != nil {
		return appport.OAuthIdentity{}, err
	}

	accessToken, err := p.exchangeCode(ctx, cfg, code, sess.RedirectURL, sess.Verifier)
	if err != nil {
		return appport.OAuthIdentity{}, err
	}

	return p.fetchIdentity(ctx, provider, tenant, cfg.UserInfoURL, accessToken)
}

// consumeSession loads and deletes one handshake atomically-from-the-caller's
// view: the delete makes a replayed state fail closed on the next use.
func (p *Provider) consumeSession(ctx context.Context, tenant valueobject.TenantID, provider, state string) (session, error) {
	blob, err := p.kv.Get(ctx, sessionKey(state))
	if err != nil {
		return session{}, ErrStateInvalid
	}

	var sess session
	if err := jsonparser.Unmarshal(blob, &sess); err != nil {
		return session{}, ErrStateInvalid
	}

	if sess.Tenant != string(tenant) || sess.Provider != provider {
		return session{}, ErrStateInvalid
	}

	if p.now().After(sess.ExpiresAt) {
		_ = p.kv.Delete(ctx, sessionKey(state))

		return session{}, ErrStateExpired
	}

	if err := p.kv.Delete(ctx, sessionKey(state)); err != nil {
		return session{}, ErrStateInvalid
	}

	return sess, nil
}

// claimCode atomically claims one authorization code via SET NX. A second
// claim of the same code fails: the code is single-use across all replicas.
func (p *Provider) claimCode(ctx context.Context, provider, code string) error {
	if code == "" {
		return ErrCodeReuse
	}

	claimed, err := p.kv.SetNX(ctx, codeKey(provider, code), []byte("1"), p.stateTTL)
	if err != nil {
		return fmt.Errorf("%w: claim store", ErrProviderFailure)
	}

	if !claimed {
		return ErrCodeReuse
	}

	return nil
}

// exchangeCode trades the code for an access token via the token endpoint.
func (p *Provider) exchangeCode(ctx context.Context, cfg ProviderConfig, code, redirectURL, verifier string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURL},
		"code_verifier": {verifier},
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("%w: token request", ErrProviderFailure)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: token exchange", ErrProviderFailure)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("%w: token body", ErrProviderFailure)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: status %d", ErrProviderFailure, resp.StatusCode)
	}

	var payload tokenResponse
	if err := jsonparser.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("%w: token decode", ErrProviderFailure)
	}

	if payload.AccessToken == "" {
		return "", fmt.Errorf("%w: empty access token", ErrProviderFailure)
	}

	return payload.AccessToken, nil
}

// fetchIdentity resolves the access token onto a linked internal identity.
func (p *Provider) fetchIdentity(ctx context.Context, provider string, tenant valueobject.TenantID, userInfoURL, accessToken string) (appport.OAuthIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return appport.OAuthIdentity{}, fmt.Errorf("%w: userinfo request", ErrProviderFailure)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.http.Do(req)
	if err != nil {
		return appport.OAuthIdentity{}, fmt.Errorf("%w: userinfo", ErrProviderFailure)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return appport.OAuthIdentity{}, fmt.Errorf("%w: status %d", ErrProviderFailure, resp.StatusCode)
	}

	var payload userInfoResponse
	if body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)); err != nil {
		return appport.OAuthIdentity{}, fmt.Errorf("%w: userinfo body", ErrProviderFailure)
	} else if err := jsonparser.Unmarshal(body, &payload); err != nil {
		return appport.OAuthIdentity{}, fmt.Errorf("%w: userinfo decode", ErrProviderFailure)
	}

	user := payload.ID
	if user == "" {
		user = payload.Sub
	}

	if user == "" {
		user = payload.Login
	}

	if user == "" {
		return appport.OAuthIdentity{}, fmt.Errorf("%w: missing user id", ErrProviderFailure)
	}

	return appport.OAuthIdentity{
		TenantID:     tenant,
		Provider:     provider,
		ProviderUser: user,
		Email:        payload.Email,
		VerifiedAt:   p.now(),
	}, nil
}
