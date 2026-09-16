package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// AuditEntry is one privileged-change record. Entries are append-only facts:
// actors, actions, and before/after digests — never secrets or full PII.
// Signed checkpoints and WORM export (E09) make tampering detectable.
type AuditEntry struct {
	TenantID   valueobject.TenantID
	Actor      string
	Action     string
	Resource   string
	BeforeHash string
	AfterHash  string
	OccurredAt time.Time
}

// AuditLogger is the immutable audit boundary (E09 implements). Recording is
// fire-and-append inside the command's UnitOfWork where the change commits;
// standalone calls are for read-side privileged actions. Reads are for
// auditors, never for command decisions.
type AuditLogger interface {
	// Record appends one audit fact. Strong write.
	Record(ctx context.Context, entry AuditEntry) error
}
