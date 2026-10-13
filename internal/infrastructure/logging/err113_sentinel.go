package logging

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrLoggingLevelIsRequired                     = errors.New("logging: level is required")
	ErrLoggingUnknownLogLevelTrace                = errors.New("logging: unknown log level \"trace\": logging: unknown log level")
	ErrDialRefused                                = errors.New("dial refused")
	ErrMissingType                                = errors.New("missing type")
	ErrStartFailed                                = errors.New("start failed")
	ErrNetworkFailure                             = errors.New("network failure")
	ErrLoggingFxLoggerRequiresANonNilKernelLogger = errors.New("logging: fx logger requires a non-nil kernel logger")
	ErrLoggingUnknownLogLevel                     = errors.New("logging: unknown log level")
)
