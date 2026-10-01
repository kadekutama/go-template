package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// StoredEntry is one persisted audit fact with chain metadata.
type StoredEntry struct {
	Seq        int64
	Tenant     string
	Actor      string
	Action     string
	Resource   string
	BeforeHash string
	AfterHash  string
	OccurredAt time.Time
	PrevHash   string
	EntryHash  string
	Signature  string
}

// SignedRoot is one signed per-tenant checkpoint.
type SignedRoot struct {
	Tenant    string
	Seq       int64
	RootHash  string
	Signature string
	At        time.Time
}

// hashEntry computes the chain hash for one entry. Fields use netstring-ish
// framing (len:bytes + NUL separator) built with strconv only, so field
// contents — including delimiters — can never be confused, and no numeric
// conversion beyond widening int to int64 occurs.
func hashEntry(tenant, actor, action, resource, before, after string, occurredAt time.Time, prevHash string, seq int64) string {
	var framed []byte

	framed = append(framed, "AUDIT-V1\x00"...)
	framed = strconv.AppendInt(framed, seq, 10)
	framed = append(framed, 0)
	framed = strconv.AppendInt(framed, occurredAt.UnixNano(), 10)
	framed = append(framed, 0)

	for _, field := range []string{tenant, actor, action, resource, before, after, prevHash} {
		framed = strconv.AppendInt(framed, int64(len(field)), 10)
		framed = append(framed, ':')
		framed = append(framed, field...)
		framed = append(framed, 0)
	}

	sum := sha256.Sum256(framed)

	return hex.EncodeToString(sum[:])
}

// signHash HMACs one hash with the signing key.
func signHash(key []byte, entryHash string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(entryHash))

	return hex.EncodeToString(mac.Sum(nil))
}

// verifySignature compares signatures in constant time.
func verifySignature(key []byte, entryHash, signature string) bool {
	expected := signHash(key, entryHash)

	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

// rootHash computes the cryptographic Merkle tree root over entry hashes.
// Pairs adjacent leaf hashes; odd leaves pair with themselves (RFC 6962 standard).
func rootHash(entries []StoredEntry) string {
	if len(entries) == 0 {
		sum := sha256.Sum256(nil)
		return hex.EncodeToString(sum[:])
	}

	leaves := make([]string, len(entries))
	for i, entry := range entries {
		leaves[i] = entry.EntryHash
	}

	return merkleTreeRoot(leaves)
}

// merkleTreeRoot performs pairwise iterative hashing up to the tree root.
func merkleTreeRoot(nodes []string) string {
	if len(nodes) == 0 {
		sum := sha256.Sum256(nil)
		return hex.EncodeToString(sum[:])
	}

	current := make([]string, len(nodes))
	copy(current, nodes)

	for len(current) > 1 {
		var next []string
		for i := 0; i < len(current); i += 2 {
			if i+1 < len(current) {
				sum := sha256.Sum256([]byte(current[i] + current[i+1]))
				next = append(next, hex.EncodeToString(sum[:]))
			} else {
				sum := sha256.Sum256([]byte(current[i] + current[i]))
				next = append(next, hex.EncodeToString(sum[:]))
			}
		}
		current = next
	}

	return current[0]
}

// secretMarkers are substrings that must never enter audit entries.
var secretMarkers = []string{"password=", "secret=", "bearer ", "private_key"}

// fromPort maps one port entry onto chain inputs with secret guard.
func fromPort(entry appport.AuditEntry) error {
	if entry.TenantID == "" || entry.Actor == "" || entry.Action == "" || entry.Resource == "" {
		return fmt.Errorf("%w: tenant, actor, action and resource required", ErrConfigRequired)
	}

	for _, field := range []string{entry.Actor, entry.Action, entry.Resource, entry.BeforeHash, entry.AfterHash} {
		lowered := strings.ToLower(field)
		for _, marker := range secretMarkers {
			if strings.Contains(lowered, marker) {
				return fmt.Errorf("%w: field carries secret material", ErrSecretInEntry)
			}
		}
	}

	return nil
}
