package httpclient

import "errors"

// ErrTransport reports a transport shape the factory cannot tune.
var ErrTransport = errors.New("httpclient: transport unavailable")
