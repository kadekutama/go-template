package featureflag

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	// ErrRefreshProbeFailed marks one failed refresh probe (status or transport).
	ErrRefreshProbeFailed = errors.New("featureflag: refresh probe failed")
	// ErrEvaluationFailed marks one failed provider evaluation.
	ErrEvaluationFailed = errors.New("featureflag: evaluation failed")
	// ErrInvalidEvalContext marks an evaluation attribute that cannot convert
	// to a string. The offending key travels in the wrapper; values never do.
	ErrInvalidEvalContext = errors.New("featureflag: evaluation attribute is not a string")
)
