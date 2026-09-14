package pagination

import (
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
