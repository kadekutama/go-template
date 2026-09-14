package valueobject

import (
	"errors"
	"uuid"
)

// Typed domain identifiers. All IDs share the canonical UUID string shape;
// Parse* accepts any canonical UUID. TransactionID is a public compatibility
// alias for the immutable capture PostingID.
//
// The domain mints nothing: new identities come from the IDGenerator port.
// The shared kernel UUIDGenerator implements this interface (implicitly —
// both spell NewID() string), as do deterministic test stubs, so generation
// stays injectable and swappable per SPEC §7.10.
type (
	AccountID string
	PostingID string
	EntryID   string
	HoldID    string
	TenantID  string
	LedgerID  string
	UserID    string
	JournalID string
	PeriodID  string
)

// TransactionID aliases PostingID for payment/provider-facing compatibility.
type TransactionID = PostingID

// IDGenerator mints unique identifier strings. It is owned by the domain and
// implemented outside it (kernel UUIDGenerator in production, stubs in
// tests). Accept it as an explicit parameter wherever new identities are
// needed; never hide generation behind package state.
type IDGenerator interface {
	// NewID returns a new unique identifier string.
	NewID() string
}

func validUUID(s string) bool {
	// Validation itself is delegated to the standard library (uuid.Parse);
	// the length gate keeps the alternate urn:/braced/bare-hex forms Parse
	// also accepts out of the domain contract.
	if len(s) != 36 {
		return false
	}
	_, err := uuid.Parse(s)
	return err == nil
}

// parseID validates the canonical shape once for every typed ID.
func parseID[T ~string](s, kind string) (T, error) {
	var zero T
	if !validUUID(s) {
		return zero, errors.New("ids: invalid " + kind + " id")
	}
	return T(s), nil
}

// String returns the canonical string form.
func (id AccountID) String() string { return string(id) }

// Equals reports whether two IDs are identical.
func (id AccountID) Equals(o AccountID) bool { return id == o }

// ParseAccountID validates the canonical UUID shape.
func ParseAccountID(s string) (AccountID, error) {
	return parseID[AccountID](s, "account")
}

// String returns the canonical string form.
func (id PostingID) String() string { return string(id) }

// Equals reports whether two IDs are identical.
func (id PostingID) Equals(o PostingID) bool { return id == o }

// ParsePostingID validates the canonical UUID shape.
func ParsePostingID(s string) (PostingID, error) {
	return parseID[PostingID](s, "posting")
}

// String returns the canonical string form.
func (id EntryID) String() string { return string(id) }

// Equals reports whether two IDs are identical.
func (id EntryID) Equals(o EntryID) bool { return id == o }

// ParseEntryID validates the canonical UUID shape.
func ParseEntryID(s string) (EntryID, error) {
	return parseID[EntryID](s, "entry")
}

// String returns the canonical string form.
func (id HoldID) String() string { return string(id) }

// Equals reports whether two IDs are identical.
func (id HoldID) Equals(o HoldID) bool { return id == o }

// ParseHoldID validates the canonical UUID shape.
func ParseHoldID(s string) (HoldID, error) {
	return parseID[HoldID](s, "hold")
}

// String returns the canonical string form.
func (id TenantID) String() string { return string(id) }

// Equals reports whether two IDs are identical.
func (id TenantID) Equals(o TenantID) bool { return id == o }

// ParseTenantID validates the canonical UUID shape.
func ParseTenantID(s string) (TenantID, error) {
	return parseID[TenantID](s, "tenant")
}

// String returns the canonical string form.
func (id LedgerID) String() string { return string(id) }

// Equals reports whether two IDs are identical.
func (id LedgerID) Equals(o LedgerID) bool { return id == o }

// ParseLedgerID validates the canonical UUID shape.
func ParseLedgerID(s string) (LedgerID, error) {
	return parseID[LedgerID](s, "ledger")
}

// String returns the canonical string form.
func (id UserID) String() string { return string(id) }

// Equals reports whether two IDs are identical.
func (id UserID) Equals(o UserID) bool { return id == o }

// ParseUserID validates the canonical UUID shape.
func ParseUserID(s string) (UserID, error) {
	return parseID[UserID](s, "user")
}

// String returns the canonical string form.
func (id JournalID) String() string { return string(id) }

// Equals reports whether two IDs are identical.
func (id JournalID) Equals(o JournalID) bool { return id == o }

// ParseJournalID validates the canonical UUID shape.
func ParseJournalID(s string) (JournalID, error) {
	return parseID[JournalID](s, "journal")
}

// String returns the canonical string form.
func (id PeriodID) String() string { return string(id) }

// Equals reports whether two IDs are identical.
func (id PeriodID) Equals(o PeriodID) bool { return id == o }

// ParsePeriodID validates the canonical UUID shape.
func ParsePeriodID(s string) (PeriodID, error) {
	return parseID[PeriodID](s, "period")
}
