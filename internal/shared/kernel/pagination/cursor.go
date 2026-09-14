package pagination

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"math"
)

// Cursor carries the resume offset for keyset-style paging.
//
// Tokens are base64url(offset-be64 || crc32-castagnoli) — opaque to callers
// and tamper-evident. E09 may upgrade the checksum to HMAC without changing
// this contract.
type Cursor struct {
	Offset int64
}

// Encode serializes the offset into an opaque token. Negative offsets are
// rejected: callers normalize first (PageRequest.Normalize floors at zero).
func (c Cursor) Encode() (string, error) {
	if c.Offset < 0 {
		return "", errors.New("pagination: negative offset")
	}
	var payload [8]byte
	binary.BigEndian.PutUint64(payload[:], uint64(c.Offset)) //nolint:gosec // guarded non-negative above
	sum := crc32.Checksum(payload[:], crc32.MakeTable(crc32.Castagnoli))
	var raw [12]byte
	copy(raw[:8], payload[:])
	binary.BigEndian.PutUint32(raw[8:], sum)
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

// DecodeCursor parses a token from Encode; forged, truncated, or overflowing
// tokens error.
func DecodeCursor(token string) (Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 12 {
		return Cursor{}, errors.New("pagination: malformed cursor")
	}
	var payload [8]byte
	copy(payload[:], raw[:8])
	want := crc32.Checksum(payload[:], crc32.MakeTable(crc32.Castagnoli))
	if binary.BigEndian.Uint32(raw[8:]) != want {
		return Cursor{}, errors.New("pagination: cursor checksum mismatch")
	}
	offset := binary.BigEndian.Uint64(payload[:])
	if offset > math.MaxInt64 {
		return Cursor{}, errors.New("pagination: cursor offset overflow")
	}
	return Cursor{Offset: int64(offset)}, nil
}
