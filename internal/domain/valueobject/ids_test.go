package valueobject_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// seqIDs is a deterministic IDGenerator stub: the domain owns the port, tests
// (and the kernel UUIDGenerator in production) implement it.
type seqIDs struct {
	mu sync.Mutex
	n  int
}

var _ valueobject.IDGenerator = (*seqIDs)(nil)

func (s *seqIDs) NewID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	return fmt.Sprintf("00000000-0000-7000-8000-%012x", s.n)
}

func TestIDRoundTripAndParse(t *testing.T) {
	t.Parallel()
	gen := &seqIDs{}
	id, err := valueobject.ParseAccountID(gen.NewID())
	if err != nil {
		t.Fatalf("ParseAccountID: %v", err)
	}
	if id.String() == "" {
		t.Fatal("parsed id is empty")
	}
	back, err := valueobject.ParseAccountID(id.String())
	if err != nil {
		t.Fatalf("ParseAccountID: %v", err)
	}
	if !id.Equals(back) {
		t.Fatal("round-trip mismatch")
	}
	for _, bad := range []string{"", "not-a-uuid", "0193e5f0-1c2c-7a4e-b3f2-9c1e5a7b9c1", id.String() + "x"} {
		if _, err := valueobject.ParseAccountID(bad); err == nil {
			t.Errorf("ParseAccountID(%q) must error", bad)
		}
	}
}

func TestGeneratorPortYieldsUniqueIDs(t *testing.T) {
	t.Parallel()
	gen := &seqIDs{}
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		s := gen.NewID()
		if seen[s] {
			t.Fatalf("duplicate id %s", s)
		}
		seen[s] = true
		if _, err := valueobject.ParsePostingID(s); err != nil {
			t.Fatalf("generated id %s must parse: %v", s, err)
		}
	}
}

func TestTransactionIDAlias(t *testing.T) {
	t.Parallel()
	gen := &seqIDs{}
	post, err := valueobject.ParsePostingID(gen.NewID())
	if err != nil {
		t.Fatalf("ParsePostingID: %v", err)
	}
	txn := post
	if !post.Equals(txn) {
		t.Fatal("TransactionID must alias PostingID")
	}
}

func TestOtherIDTypes(t *testing.T) {
	t.Parallel()
	gen := &seqIDs{}
	types := []struct {
		name  string
		parse func(string) error
	}{
		{kindEntry, func(s string) error { _, err := valueobject.ParseEntryID(s); return err }},
		{kindHold, func(s string) error { _, err := valueobject.ParseHoldID(s); return err }},
		{kindTenant, func(s string) error { _, err := valueobject.ParseTenantID(s); return err }},
		{kindLedger, func(s string) error { _, err := valueobject.ParseLedgerID(s); return err }},
		{kindUser, func(s string) error { _, err := valueobject.ParseUserID(s); return err }},
		{kindJournal, func(s string) error { _, err := valueobject.ParseJournalID(s); return err }},
		{kindPeriod, func(s string) error { _, err := valueobject.ParsePeriodID(s); return err }},
		{kindPosting, func(s string) error { _, err := valueobject.ParsePostingID(s); return err }},
	}
	for _, tc := range types {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.parse(gen.NewID()); err != nil {
				t.Errorf("valid %s must parse: %v", tc.name, err)
			}
			if err := tc.parse("bad"); err == nil {
				t.Errorf("bad %s id must error", tc.name)
			}
		})
	}
}
