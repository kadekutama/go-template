package specification_test

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/kadekutama/go-template/internal/domain/specification"
)

const (
	testUSD         = "USD"
	testEUR         = "EUR"
	testTenant1     = "t-1"
	testLedger1     = "l-1"
	testAccount1    = "a-1"
	testAccount2    = "a-2"
	testAccount3    = "a-3"
	testPosting1    = "p-1"
	testPosting2    = "p-2"
	testPosting3    = "p-3"
	testPosting4    = "p-4"
	testCodeEA      = "E_A"
	testCodeEB      = "E_B"
	testFieldAmount = "amount"
)

func passSpec() specification.Specification[string] {
	return specification.NewFuncSpec[string]("PASS", "passes", func(_ context.Context, _ string) bool { return true })
}

func failSpec(code string) specification.Specification[string] {
	return specification.NewFuncSpec[string](code, "fails "+code, func(_ context.Context, _ string) bool { return false })
}

func violationCodes(res specification.SpecResult) []string {
	out := make([]string, 0, len(res.Violations))
	for _, v := range res.Violations {
		out = append(out, v.Code)
	}
	return out
}

func equalCodes(a, b []string) bool {
	return slices.Equal(a, b)
}

func TestAllAggregatesEveryViolationInOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	res := specification.All[string](passSpec(), failSpec(testCodeEA), failSpec(testCodeEB)).Evaluate(ctx, "c")
	if res.Passed() {
		t.Fatal("All with failures must not pass")
	}
	if got, want := violationCodes(res), []string{testCodeEA, testCodeEB}; !equalCodes(got, want) {
		t.Fatalf("All violations = %v, want %v (every child, in order)", got, want)
	}
}

func TestAllPassesWhenEveryChildPasses(t *testing.T) {
	t.Parallel()
	res := specification.All[string](passSpec(), passSpec()).Evaluate(context.Background(), "c")
	if !res.Passed() || len(res.Violations) != 0 {
		t.Fatalf("All of passes = %+v, want pass", res)
	}
}

func TestAnyShortCircuitsOnFirstPass(t *testing.T) {
	t.Parallel()
	res := specification.Any[string](failSpec("E_A"), failSpec("E_B"), passSpec()).Evaluate(context.Background(), "c")
	if !res.Passed() || len(res.Violations) != 0 {
		t.Fatalf("Any with a passing child = %+v, want pass with zero violations", res)
	}
}

func TestAnyAggregatesWhenAllFail(t *testing.T) {
	t.Parallel()
	res := specification.Any[string](failSpec("E_A"), failSpec("E_B")).Evaluate(context.Background(), "c")
	if res.Passed() {
		t.Fatal("Any with all failures must not pass")
	}
	if got, want := violationCodes(res), []string{"E_A", "E_B"}; !equalCodes(got, want) {
		t.Fatalf("Any violations = %v, want %v", got, want)
	}
}

func TestNotInvertsChild(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	explicit := specification.Violation{Code: "NOT_OK", Message: "must not satisfy child"}
	if res := specification.Not[string](failSpec("E_X"), explicit).Evaluate(ctx, "c"); !res.Passed() {
		t.Fatalf("Not(failing) = %+v, want pass", res)
	}
	res := specification.Not[string](passSpec(), explicit).Evaluate(ctx, "c")
	if res.Passed() {
		t.Fatal("Not(passing) must fail")
	}
	if got := violationCodes(res); !equalCodes(got, []string{"NOT_OK"}) {
		t.Fatalf("Not(passing) violations = %v, want [NOT_OK]", got)
	}
}

func TestEmptySemantics(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if res := specification.All[string]().Evaluate(ctx, "c"); !res.Passed() {
		t.Fatalf("All() empty = %+v, want pass", res)
	}
	res := specification.Any[string]().Evaluate(ctx, "c")
	if res.Passed() || len(res.Violations) != 1 {
		t.Fatalf("Any() empty = %+v, want exactly one explicit violation", res)
	}
}

func TestNilChildIsViolationNotPanic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil child panicked: %v", r)
		}
	}()
	if res := specification.All[string](nil).Evaluate(ctx, "c"); res.Passed() {
		t.Error("All(nil) must fail")
	}
	if res := specification.Any[string](nil).Evaluate(ctx, "c"); res.Passed() {
		t.Error("Any(nil) must fail")
	}
	if res := specification.Not[string](nil, specification.Violation{Code: "N"}).Evaluate(ctx, "c"); res.Passed() {
		t.Error("Not(nil) must fail")
	}
}

func TestNilCandidateDoesNotPanic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil candidate panicked: %v", r)
		}
	}()
	nilSafe := specification.NewFuncSpec[*string]("NIL_SAFE", "nil rejected", func(_ context.Context, s *string) bool {
		return s != nil && *s != ""
	})
	if res := nilSafe.Evaluate(ctx, nil); res.Passed() {
		t.Error("nil candidate must fail the predicate, not pass")
	}
	if res := specification.All[*string](nilSafe).Evaluate(ctx, nil); res.Passed() {
		t.Error("All with nil candidate must fail, not panic")
	}
}

func TestConcurrentReuseIsRaceSafe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	shared := specification.All[string](passSpec(), failSpec("E_A"), failSpec("E_B"))
	const workers = 8
	const iters = 50
	var wg sync.WaitGroup
	errs := make(chan string, workers*iters)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				res := shared.Evaluate(ctx, "c")
				if res.Passed() || !equalCodes(violationCodes(res), []string{testCodeEA, testCodeEB}) {
					errs <- "inconsistent concurrent evaluation"
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}

func TestViolationDetailsAreCopied(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	details := map[string]string{"field": testFieldAmount}
	withDetails := specification.Not[string](
		passSpec(),
		specification.Violation{Code: "D", Message: "m", Details: details},
	)
	res := withDetails.Evaluate(ctx, "c")
	if res.Passed() || res.Violations[0].Details["field"] != testFieldAmount {
		t.Fatalf("expected copied details, got %+v", res)
	}
	details["field"] = "mutated"
	res.Violations[0].Details["field"] = "mutated-again"
	res2 := withDetails.Evaluate(ctx, "c")
	if res2.Violations[0].Details["field"] != testFieldAmount {
		t.Fatalf("violation details aliased across evaluations: %+v", res2)
	}
}
