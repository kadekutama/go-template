package pagination

import (
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"math"
	"strings"
	"testing"
)

func TestCursorRoundTrip(t *testing.T) {
	t.Parallel()

	token, err := Cursor{Offset: 120}.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.Contains(token, "120") {
		t.Errorf("token should be opaque, got %q", token)
	}
	got, err := DecodeCursor(token)
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if got.Offset != 120 {
		t.Errorf("got offset %d, want 120", got.Offset)
	}
}

func TestCursorRejectsForgery(t *testing.T) {
	t.Parallel()

	token, err := Cursor{Offset: 7}.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	tampered := token[:len(token)-2] + "AA"
	if _, err := DecodeCursor(tampered); err == nil {
		t.Error("expected error for tampered token, got nil")
	}
	if _, err := DecodeCursor("!!!"); err == nil {
		t.Error("expected error for garbage token, got nil")
	}
	_, encodeErr := Cursor{Offset: -1}.Encode()
	if encodeErr == nil {
		t.Error("expected error for negative offset, got nil")
	}
}

func TestNormalizeBounds(t *testing.T) {
	t.Parallel()

	got := PageRequest{Limit: 9999, Offset: -5}.Normalize()
	if got.Limit != MaxLimit || got.Offset != 0 {
		t.Errorf("unexpected normalization: %+v", got)
	}
	if got := (PageRequest{}).Normalize(); got.Limit != DefaultLimit {
		t.Errorf("expected default limit %d, got %+v", DefaultLimit, got)
	}
}

func TestParseLimitOffsetFallsBack(t *testing.T) {
	t.Parallel()

	got := ParseLimitOffset("abc", "-3")
	if got.Limit != DefaultLimit || got.Offset != 0 {
		t.Errorf("expected defaults, got %+v", got)
	}
	got = ParseLimitOffset("25", "50")
	if got.Limit != 25 || got.Offset != 50 {
		t.Errorf("expected 25/50, got %+v", got)
	}
}

func TestCursorOffsetOverflow(t *testing.T) {
	t.Parallel()

	var payload [8]byte
	binary.BigEndian.PutUint64(payload[:], math.MaxUint64)
	sum := crc32.Checksum(payload[:], crc32.MakeTable(crc32.Castagnoli))
	var raw [12]byte
	copy(raw[:8], payload[:])
	binary.BigEndian.PutUint32(raw[8:], sum)
	token := base64.RawURLEncoding.EncodeToString(raw[:])

	if _, err := DecodeCursor(token); err == nil {
		t.Error("expected error for overflow cursor offset, got nil")
	}
}

func TestPageResultShape(t *testing.T) {
	t.Parallel()

	result := PageResult[string]{
		Items:  []string{"item1", "item2"},
		Total:  100,
		Limit:  50,
		Offset: 0,
	}

	if len(result.Items) != 2 || result.Total != 100 || result.Limit != 50 || result.Offset != 0 {
		t.Errorf("unexpected PageResult: %+v", result)
	}
}
