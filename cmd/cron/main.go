// Command cron is the distributed scheduler entrypoint (E01-T02: fx composition; jobs land in E14).
package main

import (
	"example.com/go-template/internal/shared/di"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		di.DomainModule(),
		di.ApplicationModule(),
		di.InfrastructureModule(),
		di.CronModule(),
		fx.NopLogger,
	).Run()
}
