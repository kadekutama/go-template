package cronui

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrCronuiSessionKeyMustBeAtLeast32Bytes = errors.New("cronui: session key must be at least 32 bytes")
	ErrCronuiSessionIssuerIsNotInitialized  = errors.New("cronui: session issuer is not initialized")
	ErrCronuiSubjectIsRequired              = errors.New("cronui: subject is required")
	ErrCronuiBadSessionToken                = errors.New("cronui: bad session token")
	ErrCronuiSchedulerIsRequired            = errors.New("cronui: scheduler is required")
	ErrCronuiTokenIssuerIsRequired          = errors.New("cronui: token issuer is required")
	ErrCronuiBadBearerToken                 = errors.New("cronui: bad bearer token")
)
