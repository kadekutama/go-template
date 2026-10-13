package crypto

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrDialTCPConnectionRefused  = errors.New("dial tcp: connection refused")
	ErrTLSHandshakeFailure       = errors.New("tls: handshake failure")
	ErrCryptoTransitRewrapDecode = errors.New("crypto: transit rewrap decode")
)
