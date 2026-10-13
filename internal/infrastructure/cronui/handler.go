// Package cronui constructs the pinned gocron-ui upstream dashboard handler
// (E09.1-T02). It owns the gocron-ui import so the interface wrapper package
// stays free of upstream types. The handler is mounted only behind the
// authenticated read-only admin wrapper; it is never exposed directly.
package cronui

import (
	"net/http"

	ui "github.com/go-co-op/gocron-ui/server"
	gocron "github.com/go-co-op/gocron/v2"
)

// NewUpstreamHandler builds the upstream dashboard handler for the single
// enabled process-lifetime UI instance. It starts no listener. Constructing
// the upstream server starts its broadcaster goroutine, so callers must
// invoke this at most once per process and only when the UI is enabled.
func NewUpstreamHandler(scheduler gocron.Scheduler) (http.Handler, error) {
	if scheduler == nil {
		return nil, ErrCronuiSchedulerIsRequired
	}

	return ui.NewServer(scheduler, 0).Router, nil
}
