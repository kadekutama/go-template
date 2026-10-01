package audit

import (
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// Module registers the audit logger in an fx container.
func Module() fx.Option {
	return fx.Module("audit",
		fx.Provide(NewLogger),
		fx.Provide(func(logger *Logger) appport.AuditLogger {
			return logger
		}),
	)
}
