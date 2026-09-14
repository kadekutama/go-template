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

func TestAll(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                   string
		ctx                    context.Context
		spec                   specification.Specification[string]
		candidate              string
		expectedPassed         bool
		expectedViolationCodes []string
	}

	testCases := []testCase{
		{
			name:                   "every child passes",
			ctx:                    context.Background(),
			spec:                   specification.All[string](passSpec(), passSpec()),
			candidate:              "c",
			expectedPassed:         true,
			expectedViolationCodes: nil,
		},
		{
			name:                   "aggregates every violation in order",
			ctx:                    context.Background(),
			spec:                   specification.All[string](passSpec(), failSpec(testCodeEA), failSpec(testCodeEB)),
			candidate:              "c",
			expectedPassed:         false,
			expectedViolationCodes: []string{testCodeEA, testCodeEB},
		},
		{
			name:                   "empty all passes",
			ctx:                    context.Background(),
			spec:                   specification.All[string](),
			candidate:              "c",
			expectedPassed:         true,
			expectedViolationCodes: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := tc.spec.Evaluate(tc.ctx, tc.candidate)
			if tc.expectedPassed != res.Passed() {
				t.Fatalf("expected passed %v, got %v", tc.expectedPassed, res.Passed())
			}
			if !equalCodes(violationCodes(res), tc.expectedViolationCodes) {
				t.Fatalf("expected violation codes %v, got %v", tc.expectedViolationCodes, violationCodes(res))
			}
		})
	}
}

func TestAny(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                   string
		ctx                    context.Context
		spec                   specification.Specification[string]
		candidate              string
		expectedPassed         bool
		expectedViolationCodes []string
	}

	testCases := []testCase{
		{
			name:                   "short-circuits on first pass",
			ctx:                    context.Background(),
			spec:                   specification.Any[string](failSpec("E_A"), failSpec("E_B"), passSpec()),
			candidate:              "c",
			expectedPassed:         true,
			expectedViolationCodes: nil,
		},
		{
			name:                   "aggregates when all fail",
			ctx:                    context.Background(),
			spec:                   specification.Any[string](failSpec("E_A"), failSpec("E_B")),
			candidate:              "c",
			expectedPassed:         false,
			expectedViolationCodes: []string{"E_A", "E_B"},
		},
		{
			name:                   "empty any fails with explicit violation",
			ctx:                    context.Background(),
			spec:                   specification.Any[string](),
			candidate:              "c",
			expectedPassed:         false,
			expectedViolationCodes: []string{"SPEC_EMPTY_ANY"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := tc.spec.Evaluate(tc.ctx, tc.candidate)
			if tc.expectedPassed != res.Passed() {
				t.Fatalf("expected passed %v, got %v", tc.expectedPassed, res.Passed())
			}
			if !equalCodes(violationCodes(res), tc.expectedViolationCodes) {
				t.Fatalf("expected violation codes %v, got %v", tc.expectedViolationCodes, violationCodes(res))
			}
		})
	}
}

func TestNot(t *testing.T) {
	t.Parallel()

	explicit := specification.Violation{Code: "NOT_OK", Message: "must not satisfy child"}

	type testCase struct {
		name                   string
		ctx                    context.Context
		spec                   specification.Specification[string]
		candidate              string
		expectedPassed         bool
		expectedViolationCodes []string
	}

	testCases := []testCase{
		{
			name:                   "inverts failing child to pass",
			ctx:                    context.Background(),
			spec:                   specification.Not[string](failSpec("E_X"), explicit),
			candidate:              "c",
			expectedPassed:         true,
			expectedViolationCodes: nil,
		},
		{
			name:                   "inverts passing child to fail",
			ctx:                    context.Background(),
			spec:                   specification.Not[string](passSpec(), explicit),
			candidate:              "c",
			expectedPassed:         false,
			expectedViolationCodes: []string{"NOT_OK"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := tc.spec.Evaluate(tc.ctx, tc.candidate)
			if tc.expectedPassed != res.Passed() {
				t.Fatalf("expected passed %v, got %v", tc.expectedPassed, res.Passed())
			}
			if !equalCodes(violationCodes(res), tc.expectedViolationCodes) {
				t.Fatalf("expected violation codes %v, got %v", tc.expectedViolationCodes, violationCodes(res))
			}
		})
	}
}

func TestNilSafety(t *testing.T) {
	t.Parallel()

	nilSafe := specification.NewFuncSpec[*string]("NIL_SAFE", "nil rejected", func(_ context.Context, s *string) bool {
		return s != nil && *s != ""
	})

	type testCase struct {
		name                   string
		ctx                    context.Context
		eval                   func(ctx context.Context) specification.SpecResult
		expectedPassed         bool
		expectedViolationCodes []string
	}

	testCases := []testCase{
		{
			name: "All with nil child",
			ctx:  context.Background(),
			eval: func(ctx context.Context) specification.SpecResult {
				return specification.All[string](nil).Evaluate(ctx, "c")
			},
			expectedPassed:         false,
			expectedViolationCodes: []string{"SPEC_NIL_CHILD"},
		},
		{
			name: "Any with nil child",
			ctx:  context.Background(),
			eval: func(ctx context.Context) specification.SpecResult {
				return specification.Any[string](nil).Evaluate(ctx, "c")
			},
			expectedPassed:         false,
			expectedViolationCodes: []string{"SPEC_NIL_CHILD"},
		},
		{
			name: "Not with nil child",
			ctx:  context.Background(),
			eval: func(ctx context.Context) specification.SpecResult {
				return specification.Not[string](nil, specification.Violation{Code: "N"}).Evaluate(ctx, "c")
			},
			expectedPassed:         false,
			expectedViolationCodes: []string{"SPEC_NIL_CHILD"},
		},
		{
			name: "nil candidate on predicate fails without panic",
			ctx:  context.Background(),
			eval: func(ctx context.Context) specification.SpecResult {
				return nilSafe.Evaluate(ctx, nil)
			},
			expectedPassed:         false,
			expectedViolationCodes: []string{"NIL_SAFE"},
		},
		{
			name: "All with nil candidate fails without panic",
			ctx:  context.Background(),
			eval: func(ctx context.Context) specification.SpecResult {
				return specification.All[*string](nilSafe).Evaluate(ctx, nil)
			},
			expectedPassed:         false,
			expectedViolationCodes: []string{"NIL_SAFE"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("unexpected panic: %v", r)
				}
			}()
			res := tc.eval(tc.ctx)
			if tc.expectedPassed != res.Passed() {
				t.Fatalf("expected passed %v, got %v", tc.expectedPassed, res.Passed())
			}
			if !equalCodes(violationCodes(res), tc.expectedViolationCodes) {
				t.Fatalf("expected violation codes %v, got %v", tc.expectedViolationCodes, violationCodes(res))
			}
		})
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
