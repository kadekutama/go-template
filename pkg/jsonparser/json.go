// Package jsonparser is the single JSON choke point of the repository
// (E01-T07, SPEC §2 codec policy, backend per ADR-012).
//
// All code imports this package — never a concrete codec directly (enforced by
// seam_test.go). The backend is Sonic's stdlib-compatible profile
// (sonic.ConfigStd, v1.12.0): same observable escaping/number behavior as
// encoding/json for covered shapes (proven by the round-trip tests), with the
// benchmark recorded in E01-T07 evidence. Sonic falls back gracefully on
// unsupported architectures.
package jsonparser

import (
	"fmt"
	"strconv"

	"github.com/bytedance/sonic"
)

// std is the stdlib-compatible Sonic profile (drops in for encoding/json).
var std = sonic.ConfigStd

// Marshal encodes v exactly as encoding/json does (via the compatible profile).
func Marshal(v any) ([]byte, error) {
	data, err := std.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("jsonparser: marshal: %w", err)
	}
	return data, nil
}

// Unmarshal decodes data into v exactly as encoding/json does.
func Unmarshal(data []byte, v any) error {
	if err := std.Unmarshal(data, v); err != nil {
		return fmt.Errorf("jsonparser: unmarshal: %w", err)
	}
	return nil
}

// Get traverses decoded objects/arrays by path and re-encodes the leaf.
// Map keys are literal segments; array indices are decimal segments
// (e.g. Get(doc, "entries", "0", "amount_minor")). Missing paths and type
// mismatches return errors; hostile input never panics.
func Get(data []byte, path ...string) ([]byte, error) {
	var current any
	if err := std.Unmarshal(data, &current); err != nil {
		return nil, fmt.Errorf("jsonparser: get: invalid document: %w", err)
	}
	for i, segment := range path {
		switch node := current.(type) {
		case map[string]any:
			next, ok := node[segment]
			if !ok {
				return nil, fmt.Errorf("jsonparser: get: missing key %q at segment %d", segment, i)
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(node) {
				return nil, fmt.Errorf("jsonparser: get: bad index %q at segment %d", segment, i)
			}
			current = node[index]
		default:
			return nil, fmt.Errorf("jsonparser: get: cannot descend into %T at segment %d", current, i)
		}
	}
	leaf, err := std.Marshal(current)
	if err != nil {
		return nil, fmt.Errorf("jsonparser: get: re-encode leaf: %w", err)
	}
	return leaf, nil
}
