package audit

import (
	"context"
	"fmt"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Checkpoint signs the current per-tenant root and archives via WORM.
// It is a lock-free read followed by an exporter call.
func (l *Logger) Checkpoint(ctx context.Context, tenant string) (SignedRoot, error) {
	if l == nil || l.db == nil || len(l.signingKey) == 0 {
		return SignedRoot{}, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return SignedRoot{}, fmt.Errorf("audit: checkpoint: %w", err)
	}

	if _, err := valueobject.ParseTenantID(tenant); err != nil {
		return SignedRoot{}, fmt.Errorf("%w: tenant: %s", ErrConfigRequired, err.Error())
	}

	chain, err := l.Entries(ctx, tenant)
	if err != nil {
		return SignedRoot{}, err
	}

	signed := SignedRoot{
		Tenant:    tenant,
		Seq:       int64(len(chain)),
		RootHash:  rootHash(chain),
		Signature: signHash(l.signingKey, tenant+"\x00"+rootHash(chain)),
		At:        l.now(),
	}

	if l.worm != nil {
		if err := l.worm.ArchiveCheckpoint(ctx, signed); err != nil {
			return SignedRoot{}, fmt.Errorf("audit: worm archive: %w", err)
		}
	}

	return signed, nil
}

// VerifyCheckpoint recomputes the root and checks the signature against an
// independently stored checkpoint. The root attests the prefix of length
// root.Seq: rows appended after the checkpoint do not invalidate it, while
// truncation (chain shorter than the attested prefix) or content tampering
// still fail closed (CR-003).
func (l *Logger) VerifyCheckpoint(ctx context.Context, root SignedRoot) error {
	if l == nil || l.db == nil || len(l.signingKey) == 0 {
		return ErrNotInitialized
	}

	chain, err := l.Entries(ctx, root.Tenant)
	if err != nil {
		return err
	}

	if root.Seq < 0 || int64(len(chain)) < root.Seq {
		return fmt.Errorf("%w: chain length %d shorter than checkpoint seq %d", ErrCheckpointInvalid, len(chain), root.Seq)
	}

	prefix := chain[:root.Seq]

	if rootHash(prefix) != root.RootHash {
		return fmt.Errorf("%w: root mismatch", ErrCheckpointInvalid)
	}

	if !verifySignature(l.signingKey, root.Tenant+"\x00"+root.RootHash, root.Signature) {
		return fmt.Errorf("%w: bad signature", ErrCheckpointInvalid)
	}

	return nil
}
