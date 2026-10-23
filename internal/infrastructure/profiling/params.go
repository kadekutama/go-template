package profiling

import (
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// ProfilerParams carries constructor dependencies (Parameter Object pattern).
// Logger is required; a nil logger is rejected with ErrNilLogger rather than
// silently dropping SDK diagnostics.
type ProfilerParams struct {
	Config Config
	Logger log.Logger
}
