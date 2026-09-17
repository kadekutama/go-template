package migration

import "embed"

// Versions embeds reversible migration pairs (000001+). E07-T01 owns
// 000001–000002; E07-T10 owns 000003; E07-T02 owns 000004.
//
//go:embed versions/*.sql
var Versions embed.FS
