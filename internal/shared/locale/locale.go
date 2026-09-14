// Package locale owns the embedded error-message catalogs (E01-T06).
//
// en.yaml/id.yaml are the sources of truth; go:embed confines patterns to
// this directory, so the FS lives here and the apperror Translator consumes it.
package locale

import "embed"

// FS carries en.yaml and id.yaml into every binary.
//
//go:embed en.yaml id.yaml
var FS embed.FS
