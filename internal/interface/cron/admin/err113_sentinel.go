package admin

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrAdminAllowedOriginMustBeAnExactHTTPSURLWithAHost          = errors.New("admin: allowed origin must be an exact https URL with a host")
	ErrAdminTLSConfigIsRequiredWhenEnabled                       = errors.New("admin: TLS config is required when enabled")
	ErrAdminBindAddressIsRequiredWhenEnabled                     = errors.New("admin: bind address is required when enabled")
	ErrAdminNonLoopbackBindRequiresPrivateListenerMode           = errors.New("admin: non-loopback bind requires private-listener mode")
	ErrAdminPrivateListenerModeRequiresVerifiedMTLSClientCertifi = errors.New("admin: private-listener mode requires verified mTLS client certificates")
	ErrAdminSessionScopeIsRequiredWhenEnabled                    = errors.New("admin: session scope is required when enabled")
	ErrAdminOnlyTheCronReadScopeIsSupported                      = errors.New("admin: only the cron:read scope is supported")
	ErrAdminSessionCookieNameIsRequiredWhenEnabled               = errors.New("admin: session cookie name is required when enabled")
	ErrAdminSessionCookieMustKeepTheHostPrefix                   = errors.New("admin: session cookie must keep the __Host- prefix")
	ErrAdminLimiterNamespaceIsRequiredWhenEnabled                = errors.New("admin: limiter namespace is required when enabled")
	ErrAdminLimiterNamespaceMustBeACanonicalKeySegment           = errors.New("admin: limiter namespace must be a canonical key segment")
	ErrAdminAnonymousSubjectIsRequiredWhenEnabled                = errors.New("admin: anonymous subject is required when enabled")
	ErrAdminAnonymousSubjectMustBeACanonicalKeySegment           = errors.New("admin: anonymous subject must be a canonical key segment")
	ErrAdminRateLimitBudgetMustBePositive                        = errors.New("admin: rate limit budget must be positive")
	ErrAdminRateLimitWindowMustBePositive                        = errors.New("admin: rate limit window must be positive")
	ErrAdminSessionMaxTTLMustBeWithin05m                         = errors.New("admin: session max ttl must be within (0, 5m]")
	ErrBadBearer                                                 = errors.New("bad bearer")
	ErrStoreDown                                                 = errors.New("store down")
	ErrAdminUpstreamFactoryReturnedNoHandler                     = errors.New("admin: upstream factory returned no handler")
	ErrAdminUpstreamFactoryIsRequired                            = errors.New("admin: upstream factory is required")
	ErrAdminSessionSeamsAreRequired                              = errors.New("admin: session seams are required")
	ErrAdminAuthorizerIsRequired                                 = errors.New("admin: authorizer is required")
	ErrAdminAuditLoggerIsRequired                                = errors.New("admin: audit logger is required")
	ErrAdminRateLimiterIsRequired                                = errors.New("admin: rate limiter is required")
	ErrAdminDisabled                                             = errors.New("admin: disabled")
	ErrAdminHijackNotSupported                                   = errors.New("admin: hijack not supported")
)
