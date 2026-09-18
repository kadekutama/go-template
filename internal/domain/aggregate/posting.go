package aggregate

import (
	"fmt"
	"maps"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/event"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Posting is the immutable accounting-fact aggregate. It exposes no mutator:
// corrections are new linked postings via ReversePosting.
type Posting struct {
	data   entity.PostingData
	events []event.DomainEvent
}

// PostingParams carries caller-supplied identity, scope, lines, and the
// account set the lines are checked against.
type PostingParams struct {
	ID                valueobject.PostingID
	TenantID          valueobject.TenantID
	LedgerID          valueobject.LedgerID
	Operation         string
	ExternalReference string
	Description       string
	Entries           []entity.Entry
	Accounts          map[valueobject.AccountID]entity.AccountData
	EffectiveAt       time.Time
	RecordedAt        time.Time
	ReversalOf        *valueobject.PostingID
	Reason            string
	Metadata          map[string]string
	EventID           string
}

// balanceTotals carries per-asset debit/credit sums for error details.
type balanceTotals struct {
	debit  int64
	credit int64
}

func (a *Posting) append(ev event.DomainEvent) { a.events = append(a.events, ev) }

func validatePostingAccount(e entity.Entry, postingID valueobject.PostingID, tenantID valueobject.TenantID, ledgerID valueobject.LedgerID, accounts map[valueobject.AccountID]entity.AccountData) error {
	if postingID != "" && e.PostingID != "" && e.PostingID != postingID {
		return entity.NewError("ENTRY_POSTING_MISMATCH", "entry posting id must match the posting")
	}
	acct, ok := accounts[e.AccountID]
	if !ok {
		return entity.Errorf("ACCOUNT_NOT_FOUND", "account %s is unknown", e.AccountID)
	}
	if acct.TenantID != tenantID || acct.LedgerID != ledgerID {
		return entity.Errorf("ACCOUNT_SCOPE_MISMATCH", "account %s is outside the posting scope", e.AccountID)
	}
	if acct.AssetCode != e.AssetCode {
		return entity.Errorf("ACCOUNT_ASSET_MISMATCH", "entry asset %s does not match account %s asset", e.AssetCode, e.AccountID)
	}
	switch acct.Status {
	case valueobject.StatusActive:
		return nil
	case valueobject.StatusFrozen:
		return entity.Errorf("ACCOUNT_FROZEN", "account %s is frozen", e.AccountID)
	case valueobject.StatusClosed:
		return entity.Errorf("ACCOUNT_CLOSED", "account %s is closed", e.AccountID)
	default:
		return entity.Errorf("ACCOUNT_STATUS_INVALID", "account %s has invalid status", e.AccountID)
	}
}

func validatePostingEntries(entries []entity.Entry, postingID valueobject.PostingID, tenantID valueobject.TenantID, ledgerID valueobject.LedgerID, accounts map[valueobject.AccountID]entity.AccountData) error {
	for i := range entries {
		if err := validatePostingAccount(entries[i], postingID, tenantID, ledgerID, accounts); err != nil {
			return err
		}
	}
	return nil
}

func validatePostingBalance(entries []entity.Entry) error {
	totals := map[valueobject.AssetCode]*balanceTotals{}
	for _, e := range entries {
		t, ok := totals[e.AssetCode]
		if !ok {
			t = &balanceTotals{}
			totals[e.AssetCode] = t
		}
		if e.Side == valueobject.DirectionDebit {
			t.debit += e.AmountMinor
		} else {
			t.credit += e.AmountMinor
		}
	}
	for asset, t := range totals {
		if t.debit != t.credit {
			return &entity.Error{
				Code: "UNBALANCED_TRANSACTION",
				Message: fmt.Sprintf("asset %s unbalanced: debits=%d credits=%d",
					asset, t.debit, t.credit),
			}
		}
	}
	return nil
}

func buildPostingPostedEvent(p PostingParams, entries []entity.Entry) (event.DomainEvent, error) {
	entryPayloads := make([]event.EntryPayload, len(entries))
	for i, e := range entries {
		entryPayloads[i] = event.EntryPayload{
			EntryID: e.ID.String(), AccountID: e.AccountID.String(),
			Direction: string(e.Side), AmountMinor: e.AmountMinor,
			AssetCode: string(e.AssetCode), AccountSeq: e.AccountSeq,
		}
	}
	return event.NewTransactionPosted(p.EventID, p.ID.String(), p.RecordedAt, 1, 0,
		event.TransactionPostedPayload{
			PostingID: p.ID.String(), TenantID: p.TenantID.String(), LedgerID: p.LedgerID.String(),
			Operation: p.Operation, Description: p.Description, Reference: p.ExternalReference,
			Entries: entryPayloads, RecordedAt: p.RecordedAt.UTC(),
		},
		event.EventMetadata{TenantID: p.TenantID.String(), LedgerID: p.LedgerID.String(),
			CausationID: p.EventID, CorrelationID: p.EventID})
}

// ConstructPosting validates and freezes an immutable posting: structural
// validity, account existence/scope/asset/status, per-asset balance, and
// reversal linkage. It records transaction.posted.v1.
func ConstructPosting(p PostingParams) (Posting, error) {
	entries := make([]entity.Entry, len(p.Entries))
	copy(entries, p.Entries)
	metadata := maps.Clone(p.Metadata)
	data := entity.PostingData{
		ID: p.ID, TenantID: p.TenantID, LedgerID: p.LedgerID, Operation: p.Operation,
		ExternalReference: p.ExternalReference, Description: p.Description,
		Entries: entries, EffectiveAt: p.EffectiveAt.UTC(), RecordedAt: p.RecordedAt.UTC(),
		ReversalOf: p.ReversalOf, Reason: p.Reason, Metadata: metadata,
	}
	if err := data.Validate(); err != nil {
		return Posting{}, err
	}
	if err := validatePostingEntries(entries, p.ID, p.TenantID, p.LedgerID, p.Accounts); err != nil {
		return Posting{}, err
	}
	if err := validatePostingBalance(entries); err != nil {
		return Posting{}, err
	}
	posting := Posting{data: data}
	if p.ID != "" {
		evt, err := buildPostingPostedEvent(p, entries)
		if err != nil {
			return Posting{}, err
		}
		posting.append(evt)
	}
	return posting, nil
}

// AssignID attaches the database-generated ID and entries to an unpersisted Posting and emits transaction.posted.v1.
func (a *Posting) AssignID(id valueobject.PostingID, entries []entity.Entry, eventID string) error {
	if a.data.ID != "" {
		return entity.NewError("POSTING_ID_IMMUTABLE", "posting id is already assigned")
	}
	if _, err := valueobject.ParsePostingID(id.String()); err != nil {
		return entity.NewError("POSTING_ID_INVALID", "posting id is invalid")
	}
	a.data.ID = id
	if len(entries) > 0 {
		a.data.Entries = make([]entity.Entry, len(entries))
		copy(a.data.Entries, entries)
	}
	evt, err := buildPostingPostedEvent(PostingParams{
		ID:                id,
		TenantID:          a.data.TenantID,
		LedgerID:          a.data.LedgerID,
		Operation:         a.data.Operation,
		ExternalReference: a.data.ExternalReference,
		Description:       a.data.Description,
		RecordedAt:        a.data.RecordedAt,
		EventID:           eventID,
	}, a.data.Entries)
	if err != nil {
		return err
	}
	a.append(evt)
	return nil
}

// ReverseParams carries the new posting identity, the reversal reason/actor,
// and the generator minting the mirrored entry IDs.
type ReverseParams struct {
	NewID    valueobject.PostingID
	Reason   string
	Actor    valueobject.UserID
	EventID  string
	At       time.Time
	Metadata map[string]string
	// IDGen mints the mirrored entry identities. Required: nil fails closed
	// instead of hiding generation inside the domain.
	IDGen valueobject.IDGenerator
}

func validateReverseParams(p ReverseParams) error {
	if p.Reason == "" {
		return entity.NewError("REVERSAL_REASON_REQUIRED", "reversal requires a reason")
	}
	if p.NewID != "" {
		if _, err := valueobject.ParsePostingID(p.NewID.String()); err != nil {
			return entity.NewError("POSTING_ID_INVALID", "posting id is invalid")
		}
		if p.IDGen == nil {
			return entity.NewError("ID_GENERATOR_REQUIRED", "reversal requires an ID generator for mirrored entries")
		}
	}
	return nil
}

func mirrorEntries(srcEntries []entity.Entry, newID valueobject.PostingID, idGen valueobject.IDGenerator) ([]entity.Entry, int64, error) {
	mirrored := make([]entity.Entry, len(srcEntries))
	var total int64
	for i, e := range srcEntries {
		side := valueobject.DirectionDebit
		if e.Side == valueobject.DirectionDebit {
			side = valueobject.DirectionCredit
		}
		var entryID valueobject.EntryID
		if idGen != nil {
			var err error
			entryID, err = valueobject.ParseEntryID(idGen.NewID())
			if err != nil {
				return nil, 0, entity.Errorf("ENTRY_ID_INVALID", "generated entry id rejected: %v", err)
			}
		}
		mirrored[i] = entity.Entry{
			ID: entryID, PostingID: newID, AccountID: e.AccountID,
			Side: side, AmountMinor: e.AmountMinor, AssetCode: e.AssetCode, AccountSeq: e.AccountSeq,
		}
		if e.Side == valueobject.DirectionDebit {
			total += e.AmountMinor
		}
	}
	return mirrored, total, nil
}

// ReversePosting builds the opposite-side linked correction for a committed
// posting and records transaction.reversed.v1 on the new posting. The original
// is returned unchanged.
func ReversePosting(original Posting, accounts map[valueobject.AccountID]entity.AccountData, p ReverseParams) (Posting, error) {
	if err := validateReverseParams(p); err != nil {
		return Posting{}, err
	}
	src := original.data
	mirrored, total, err := mirrorEntries(src.Entries, p.NewID, p.IDGen)
	if err != nil {
		return Posting{}, err
	}
	reversed, err := ConstructPosting(PostingParams{
		ID: p.NewID, TenantID: src.TenantID, LedgerID: src.LedgerID,
		Operation: src.Operation, Description: "Reversal: " + src.Description,
		Entries: mirrored, Accounts: accounts,
		EffectiveAt: p.At, RecordedAt: p.At,
		ReversalOf: &src.ID, Reason: p.Reason, Metadata: maps.Clone(p.Metadata),
		EventID: p.EventID + "-posted",
	})
	if err != nil {
		return Posting{}, err
	}
	if p.NewID != "" {
		revEntries := make([]event.EntryPayload, len(mirrored))
		for i, e := range mirrored {
			revEntries[i] = event.EntryPayload{
				EntryID: e.ID.String(), AccountID: e.AccountID.String(),
				Direction: string(e.Side), AmountMinor: e.AmountMinor,
				AssetCode: string(e.AssetCode), AccountSeq: e.AccountSeq,
			}
		}
		evt, err := event.NewTransactionReversed(p.EventID, p.NewID.String(), p.At, 1, 0,
			event.TransactionReversedPayload{
				PostingID: p.NewID.String(), TenantID: src.TenantID.String(),
				OriginalPostingID: src.ID.String(), ReversalType: "FULL",
				ReversedAmountMinor: total, ReversedEntries: revEntries,
				Reason: p.Reason, ReversedAt: p.At.UTC(),
			},
			event.EventMetadata{TenantID: src.TenantID.String(), LedgerID: src.LedgerID.String(),
				CausationID: p.EventID, CorrelationID: p.EventID, UserID: p.Actor.String()})
		if err != nil {
			return Posting{}, err
		}
		reversed.append(evt)
	}
	return reversed, nil
}

// ID returns the posting identifier.
func (a *Posting) ID() valueobject.PostingID { return a.data.ID }

// Record returns a defensive copy of the posting record.
func (a *Posting) Record() entity.PostingData {
	out := a.data
	out.Entries = make([]entity.Entry, len(a.data.Entries))
	copy(out.Entries, a.data.Entries)
	out.Metadata = maps.Clone(a.data.Metadata)
	return out
}

// UncommittedEvents returns a copy of events not yet drained to the outbox.
func (a *Posting) UncommittedEvents() []event.DomainEvent {
	out := make([]event.DomainEvent, len(a.events))
	copy(out, a.events)
	return out
}

// LoadPosting rehydrates an aggregate from its stored record for reversal
// handling (added in E06-T03: no rehydration path existed for stored
// postings). Events start empty: only the new reversal is uncommitted.
func LoadPosting(data entity.PostingData) Posting {
	return Posting{data: data}
}

// ClearEvents drains the uncommitted buffer after persistence.
func (a *Posting) ClearEvents() { a.events = nil }
