// Package httpclient provides the single shared *http.Client for outbound
// provider calls (E09-T02 OAuth, E09-T05 OpenBao, E09-T06 Transit).
//
// One instance is shared by all modules: connection reuse (keep-alive,
// idle pool) is a global resource, and a single choke point is where proxy,
// tracing, and User-Agent policy get enforced once. Construct another client
// only for genuinely different requirements (client certificates, streaming
// without timeouts, pinned test doubles) — never one per module by default.
//
// Consumers take the narrow Doer seam (Do only); *http.Client satisfies
// it, so wiring the shared instance is a one-line fx binding with zero
// business-logic edits. Retry/breaker policy stays where it belongs:
// E01-T08 resilience primitives and E08 coordination, not the transport.
package httpclient

import (
	"net"
	"net/http"
	"time"

	"go.uber.org/fx"

	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// Doer is the single injectable HTTP seam (DIP) for all outbound provider
// calls: OAuth, OpenBao, Transit. It lives here — not in each consumer — so
// unrelated packages never import each other's interfaces to name the same
// shape. *http.Client satisfies it, so the shared client wires in directly;
// tests supply httpmock-backed or stub transports.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Params tunes the shared transport (Parameter Object pattern).
type Params struct {
	Timeout             time.Duration `validate:"required,gt=0"`
	MaxIdleConns        int           `validate:"gte=0,lte=10000"`
	MaxIdleConnsPerHost int           `validate:"gte=0,lte=1000"`
	IdleConnTimeout     time.Duration `validate:"gte=0"`
}

// NewShared builds the shared client. The returned client is safe for
// concurrent use and must be shared, not cloned per call site.
// It constructs an isolated, dedicated *http.Transport without borrowing or cloning
// http.DefaultTransport, avoiding shared connection pools, mutation vulnerabilities,
// and uncontrolled global transport state.
func NewShared(params Params) (*http.Client, error) {
	if err := validate.Struct("httpclient", "params", params); err != nil {
		return nil, err
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          params.MaxIdleConns,
		MaxIdleConnsPerHost:   params.MaxIdleConnsPerHost,
		IdleConnTimeout:       params.IdleConnTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: params.Timeout,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   params.Timeout,
	}, nil
}

// ClientIn supplies parameters via fx.
type ClientIn struct {
	fx.In
	Params Params
}

func provideSharedClient(in ClientIn) (*http.Client, error) {
	return NewShared(in.Params)
}

// Module provides the single shared client in an fx container: one instance
// for every consumer holding a Doer seam.
func Module() fx.Option {
	return fx.Module("httpclient",
		fx.Provide(provideSharedClient),
		fx.Provide(func(client *http.Client) Doer {
			return client
		}),
	)
}
