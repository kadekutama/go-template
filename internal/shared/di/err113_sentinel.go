package di

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrTestWorkFailed                                            = errors.New("test work failed")
	ErrLoggingLogLevelIsRequiredSetAPPLOGLEVELOrAPPOBSERVABILITY = errors.New("logging: log level is required (set APP_LOG_LEVEL or APP_OBSERVABILITY__LOG_LEVEL)")
	ErrDiIsRequired                                              = errors.New("di: is required")
	ErrDiInvalidMustBePositive                                   = errors.New("di: invalid: must be positive")
)
