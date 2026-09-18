package migration

import "embed"

// Versions embeds unified Goose SQL migrations (E07.1-T01, ADR-018):
// 000001–000004 baseline (numeric prefixes preserved) plus 14-digit UTC
// timestamped additions (e.g. 20260901000005 Citus distribution).
//
//go:embed versions/*.sql
var Versions embed.FS
