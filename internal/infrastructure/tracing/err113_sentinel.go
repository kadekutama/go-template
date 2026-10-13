package tracing

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrTracingServiceNameIsRequired                            = errors.New("tracing: ServiceName is required")
	ErrTracingServiceVersionIsRequired                         = errors.New("tracing: ServiceVersion is required")
	ErrTracingExcludedRoutesIsRequired                         = errors.New("tracing: ExcludedRoutes is required")
	ErrTracingExcludedRouteMustNotBeBlank                      = errors.New("tracing: excluded route must not be blank")
	ErrTracingProductionRequiresOTLPEndpointRefusingSilentNoOp = errors.New("tracing: production requires OTLPEndpoint (refusing silent no-op)")
	ErrBoomTest                                                = errors.New("boom-test")
	ErrTracingSampleRatioOutOf01                               = errors.New("tracing: SampleRatio out of (0,1]")
	ErrTracingInvalidOTLPEndpointWantHTTPSHostPort             = errors.New("tracing: invalid OTLPEndpoint (want https?://host:port)")
)
