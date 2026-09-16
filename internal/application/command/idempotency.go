package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/kadekutama/go-template/internal/application/port"
)

// Fingerprint hashes request parts into a canonical idempotency fingerprint.
// Length-prefixing keeps concatenated parts unambiguous ("ab"+"c" hashes
// differently from "a"+"bc"). Callers pass stable, canonical encodings.
func Fingerprint(parts ...string) string {
	sum := sha256.New()
	for _, part := range parts {
		_, _ = fmt.Fprintf(sum, "%d:%s;", len(part), part)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// MapParts renders a string map into canonical fingerprint parts: the prefix
// followed by sorted key/value pairs, so equal maps hash equally regardless
// of Go map iteration order.
func MapParts(prefix string, m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, 2*len(keys)+1)
	parts = append(parts, prefix)
	for _, k := range keys {
		parts = append(parts, k, m[k])
	}
	return parts
}

// RunIdempotent reserves rec, replays the stored response on identical
// resubmission, and runs execute at most once per key+fingerprint. A
// fingerprint conflict returns the store error without executing. When
// execute fails, the lease stays open so a later identical submission may
// still run; only completed fingerprints replay.
func RunIdempotent(ctx context.Context, store port.IdempotencyStore, rec port.IdempotencyRecord, execute func(ctx context.Context) ([]byte, error)) (response []byte, replayed bool, err error) {
	outcome, err := store.Reserve(ctx, rec)
	if err != nil {
		return nil, false, err
	}
	if outcome.Replay {
		return outcome.Response, true, nil
	}
	response, err = execute(ctx)
	if err != nil {
		return nil, false, err
	}
	if err := store.Complete(ctx, rec.Key, response); err != nil {
		return nil, false, err
	}
	return response, false, nil
}
