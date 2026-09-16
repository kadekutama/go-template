package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

// OpenDisputeCommand opens a funds-holding dispute against a payment inside
// the network window with a versioned policy.
type OpenDisputeCommand struct {
	PaymentID         string
	OriginalPostingID string
	Network           string
	AmountMinor       int64
	PaymentAt         time.Time
	Actor             string
	IdempotencyKey    string
}

// EvidenceCommand submits dispute evidence before the evidence deadline.
type EvidenceCommand struct {
	DisputeID      string
	Actor          string
	IdempotencyKey string
}

// RepresentCommand files one representment stage under the versioned network
// allowance.
type RepresentCommand struct {
	DisputeID      string
	Actor          string
	IdempotencyKey string
}

// CloseDisputeCommand decides a dispute as won or lost. Approver carries the
// second pair of eyes for above-threshold SoD (E04-T02 rule surfaced here).
type CloseDisputeCommand struct {
	DisputeID      string
	Outcome        string
	Approver       string
	Actor          string
	IdempotencyKey string
}

// DisputeQuery reads one dispute by ID. Strong read.
type DisputeQuery struct {
	DisputeID string
}

// DisputeListFilter pages disputes with optional status/date bounds.
type DisputeListFilter struct {
	Status string
	From   time.Time
	To     time.Time
	Cursor string
	Limit  int
}

// DisputePage is one point-in-time page of disputes.
type DisputePage struct {
	Disputes   []DisputeResult
	NextCursor string
}

// DisputeResult carries the stored dispute with its read cursor.
type DisputeResult struct {
	Dispute entity.Dispute
	Cursor  string
}

// DisputeCommandUseCases defines the mutating operations on disputes. Strong writes.
type DisputeCommandUseCases interface {
	// OpenDispute validates the window and holds funds. Strong write.
	OpenDispute(ctx context.Context, cmd OpenDisputeCommand) (DisputeResult, error)
	// SubmitEvidence files evidence before the deadline. Strong write.
	SubmitEvidence(ctx context.Context, cmd EvidenceCommand) (DisputeResult, error)
	// RepresentDispute files one representment stage. Strong write.
	RepresentDispute(ctx context.Context, cmd RepresentCommand) (DisputeResult, error)
	// CloseDispute decides won/lost with SoD passthrough. Strong write.
	CloseDispute(ctx context.Context, cmd CloseDisputeCommand) (DisputeResult, error)
}

// DisputeQueryUseCases defines the read operations on disputes. Strong reads.
type DisputeQueryUseCases interface {
	// GetDispute returns one dispute with deadline + fee. Strong read.
	GetDispute(ctx context.Context, query DisputeQuery) (DisputeResult, error)
	// ListDisputes pages disputes by filter. Point-in-time page.
	ListDisputes(ctx context.Context, filter DisputeListFilter) (DisputePage, error)
}

// DisputeUseCases is the composite inbound dispute face for api-contracts §7.11
// (application side of E03-T07; added in E06-T11).
type DisputeUseCases interface {
	DisputeCommandUseCases
	DisputeQueryUseCases
}
