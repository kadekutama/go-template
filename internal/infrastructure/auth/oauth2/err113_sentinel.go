// Package oauth2 validates OIDC bearer tokens and maps them to operator identity for the admin UI.
package oauth2

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrFakeKvMiss = errors.New("fake kv: miss")
)
