package di

import "go.uber.org/fx"

// CronModule holds scheduler-binary providers (jobs land in E14).
func CronModule() fx.Option {
	return fx.Module("cron")
}
