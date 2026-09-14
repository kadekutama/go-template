package valueobject_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	testUSD     = "USD"
	testEUR     = "EUR"
	kindAccount = "account"
	kindPosting = "posting"
	kindEntry   = "entry"
	kindHold    = "hold"
	kindTenant  = "tenant"
	kindLedger  = "ledger"
	kindUser    = "user"
	kindJournal = "journal"
	kindPeriod  = "period"
)

func TestRegistryRoundTrip(t *testing.T) {
	t.Parallel()
	reg, err := valueobject.NewRegistry(
		valueobject.AssetInfo{Code: testUSD, Exponent: 2, Kind: valueobject.AssetKindFiat},
		valueobject.AssetInfo{Code: testEUR, Exponent: 2, Kind: valueobject.AssetKindFiat},
	)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	info, err := reg.Lookup(testUSD)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if info.Exponent != 2 || info.Kind != valueobject.AssetKindFiat {
		t.Fatalf("Lookup = %+v", info)
	}
	if err := reg.Register(valueobject.AssetInfo{Code: testUSD, Exponent: 2}); err == nil {
		t.Error("duplicate Register must error")
	}
	if err := reg.Register(valueobject.AssetInfo{Code: "", Exponent: 2}); err == nil {
		t.Error("empty code must error")
	}
	if err := reg.Register(valueobject.AssetInfo{Code: "XXX", Exponent: -1}); err == nil {
		t.Error("negative exponent must error")
	}
	if _, err := reg.Lookup("ZZZ"); err == nil {
		t.Error("unknown code must error")
	}
}

func TestNoSettersOnValueObjects(t *testing.T) {
	t.Parallel()
	types := []reflect.Type{
		reflect.TypeOf(valueobject.Money{}),
		reflect.TypeOf(valueobject.AssetCode("")),
		reflect.TypeOf(valueobject.Registry{}),
		reflect.TypeOf(valueobject.AccountID("")),
		reflect.TypeOf(valueobject.PostingID("")),
		reflect.TypeOf(valueobject.EntryID("")),
		reflect.TypeOf(valueobject.HoldID("")),
		reflect.TypeOf(valueobject.TenantID("")),
		reflect.TypeOf(valueobject.LedgerID("")),
		reflect.TypeOf(valueobject.UserID("")),
		reflect.TypeOf(valueobject.JournalID("")),
		reflect.TypeOf(valueobject.PeriodID("")),
	}
	for _, typ := range types {
		for i := 0; i < typ.NumMethod(); i++ {
			if strings.HasPrefix(typ.Method(i).Name, "Set") {
				t.Errorf("%s has setter %s", typ.Name(), typ.Method(i).Name)
			}
		}
		ptr := reflect.PointerTo(typ)
		for i := 0; i < ptr.NumMethod(); i++ {
			if strings.HasPrefix(ptr.Method(i).Name, "Set") {
				t.Errorf("%s has setter %s", typ.Name(), ptr.Method(i).Name)
			}
		}
	}
}

func TestNoFloatsInValueObjects(t *testing.T) {
	t.Parallel()
	for _, typ := range []reflect.Type{
		reflect.TypeOf(valueobject.Money{}),
		reflect.TypeOf(valueobject.AssetInfo{}),
	} {
		for i := 0; i < typ.NumField(); i++ {
			k := typ.Field(i).Type.Kind()
			if k == reflect.Float32 || k == reflect.Float64 {
				t.Errorf("%s.%s uses float", typ.Name(), typ.Field(i).Name)
			}
		}
	}
}
