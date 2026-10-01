package oauth2

import (
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
)

// ProviderConfig is one federated provider (Google, GitHub, generic OIDC).
type ProviderConfig struct {
	ClientID     string   `validate:"required,max=256"`
	ClientSecret string   `validate:"required,max=1024"`
	AuthURL      string   `validate:"required,url"`
	TokenURL     string   `validate:"required,url"`
	UserInfoURL  string   `validate:"required,url"`
	Scopes       []string `validate:"max=32,dive,max=128"`
}

// OAuthParams carries constructor dependencies (Parameter Object pattern).
type OAuthParams struct {
	Providers   map[string]ProviderConfig `validate:"required,min=1"`
	StateSecret []byte                    `validate:"required,min=32"`
	StateTTL    time.Duration             `validate:"required,gt=0"`
	HTTP        httpclient.Doer
	KV          KVStore
	Clock       appport.Clock
}

// Validate checks params deterministically before construction.
func (p OAuthParams) Validate() error {
	if len(p.Providers) == 0 {
		return ErrConfigRequired
	}

	if len(p.StateSecret) < 32 {
		return ErrConfigRequired
	}

	if p.StateTTL <= 0 {
		return ErrConfigRequired
	}

	if p.HTTP == nil || p.KV == nil {
		return ErrConfigRequired
	}

	if p.Clock == nil {
		return ErrClockRequired
	}

	return nil
}
