package service

import (
	"cmp"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// LedgerFact is one ledger-side comparable fact.
type LedgerFact struct {
	PostingID   string
	Reference   string
	AmountMinor int64
	AssetCode   string
	EffectiveAt time.Time
}

// Validate checks ledger-fact identity and shape.
func (f LedgerFact) Validate() error {
	if strings.TrimSpace(f.PostingID) == "" {
		return entity.NewError("POSTING_ID_REQUIRED", "ledger fact posting id is required")
	}
	if strings.TrimSpace(f.Reference) == "" {
		return entity.NewError("LEDGER_REFERENCE_REQUIRED", "ledger fact reference is required")
	}
	if f.AmountMinor <= 0 {
		return entity.NewError("INVALID_ENTRY_AMOUNT", "ledger fact amount must be positive")
	}
	if strings.TrimSpace(f.AssetCode) == "" {
		return entity.NewError("LEDGER_ASSET_REQUIRED", "ledger fact asset is required")
	}
	if f.EffectiveAt.IsZero() {
		return entity.NewError("LEDGER_EFFECTIVE_REQUIRED", "ledger fact effective time is required")
	}
	return nil
}

// TolerancePolicy is the rail/currency amount tolerance. There is no global
// epsilon: every rail+asset compared must have an entry.
type TolerancePolicy struct {
	Rail           string
	AssetCode      string
	ToleranceMinor int64
}

// MatchConfig carries the versioned rule inputs.
type MatchConfig struct {
	RuleVersion  string
	TimingWindow time.Duration
	Tolerances   []TolerancePolicy
	Actor        string
	Rail         string
	RunID        string
	TenantID     string
}

// MatchGroup records one matched comparison with provenance.
type MatchGroup struct {
	GroupID       string
	RuleVersion   string
	Confidence    string
	Decision      valueobject.MatchDecision
	LedgerRefs    []string
	ExternalRefs  []string
	LedgerSum     int64
	ExternalSum   int64
	DecisionActor string
}

// AutoResolvePolicy gates which breaks may auto-close.
type AutoResolvePolicy struct {
	RuleVersion   string
	MaxSkew       time.Duration
	AllowedTypes  []valueobject.BreakType
	DecisionActor string
}

const evidenceReferenceKey = "reference"

// MatchStatements compares ledger facts with external snapshot lines.
// It is pure and deterministic: identical inputs yield identical outputs.
func MatchStatements(ledger []LedgerFact, external []entity.ExternalStatementLine, cfg MatchConfig) ([]MatchGroup, []entity.ReconciliationBreak, error) {
	if err := validateMatchConfig(cfg); err != nil {
		return nil, nil, err
	}
	if err := validateLedgerFacts(ledger); err != nil {
		return nil, nil, err
	}
	if err := validateExternalLines(external); err != nil {
		return nil, nil, err
	}
	sortedLedger := sortedLedgerFacts(ledger)
	sortedExternal := sortedExternalLines(external)
	duplicates, deduped := splitDuplicates(sortedExternal)
	breaks := duplicateBreaks(duplicates, cfg)
	matched, refBreaks, err := matchByReference(sortedLedger, deduped, cfg)
	if err != nil {
		return nil, nil, err
	}
	return matched, append(breaks, refBreaks...), nil
}

// AutoResolveBreak reports whether a break may auto-close for the given skew.
// AMOUNT_MISMATCH always fails with AUTO_RESOLVE_FORBIDDEN; DUPLICATE never
// auto-resolves; timing kinds resolve only inside policy and allow-list.
func AutoResolveBreak(b entity.ReconciliationBreak, skew time.Duration, policy AutoResolvePolicy) (bool, error) {
	if b.Type == valueobject.BreakAmountMismatch {
		return false, entity.NewError("AUTO_RESOLVE_FORBIDDEN", "amount mismatches require human review")
	}
	if b.Type == valueobject.BreakDuplicate {
		return false, nil
	}
	if policy.RuleVersion == "" {
		return false, entity.NewError("RULE_VERSION_REQUIRED", "auto-resolve requires a rule version")
	}
	if strings.TrimSpace(policy.DecisionActor) == "" {
		return false, entity.NewError("DECISION_ACTOR_REQUIRED", "auto-resolve requires a decision actor")
	}
	if !typeAllowed(b.Type, policy.AllowedTypes) {
		return false, nil
	}
	if skew < 0 {
		skew = -skew
	}
	if skew > policy.MaxSkew {
		return false, nil
	}
	return true, nil
}

func validateMatchConfig(cfg MatchConfig) error {
	if strings.TrimSpace(cfg.RuleVersion) == "" {
		return entity.NewError("RULE_VERSION_REQUIRED", "match rule version is required")
	}
	if strings.TrimSpace(cfg.Actor) == "" {
		return entity.NewError("DECISION_ACTOR_REQUIRED", "match decision actor is required")
	}
	if strings.TrimSpace(cfg.Rail) == "" {
		return entity.NewError("TOLERANCE_POLICY_REQUIRED", "match rail is required for tolerance lookup")
	}
	if strings.TrimSpace(cfg.RunID) == "" {
		return entity.NewError("RUN_ID_REQUIRED", "run id is required")
	}
	if strings.TrimSpace(cfg.TenantID) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if cfg.TimingWindow < 0 {
		return entity.NewError("TIMING_WINDOW_INVALID", "timing window must be non-negative")
	}
	return checkDuplicateTolerances(cfg.Tolerances)
}

func checkDuplicateTolerances(policies []TolerancePolicy) error {
	seen := make(map[string]int64)
	for _, t := range policies {
		if strings.TrimSpace(t.Rail) == "" || strings.TrimSpace(t.AssetCode) == "" {
			return entity.NewError("TOLERANCE_POLICY_REQUIRED", "tolerance requires rail and asset")
		}
		if t.ToleranceMinor < 0 {
			return entity.NewError("TOLERANCE_POLICY_REQUIRED", "tolerance must be non-negative")
		}
		key := t.Rail + "\x00" + t.AssetCode
		if prev, ok := seen[key]; ok && prev != t.ToleranceMinor {
			return entity.NewError("TOLERANCE_POLICY_CONFLICT", "duplicate tolerance policy for rail and asset")
		}
		seen[key] = t.ToleranceMinor
	}
	return nil
}

func validateLedgerFacts(ledger []LedgerFact) error {
	for i := range ledger {
		if err := ledger[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateExternalLines(external []entity.ExternalStatementLine) error {
	for i := range external {
		if err := external[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

func sortedLedgerFacts(in []LedgerFact) []LedgerFact {
	out := append([]LedgerFact(nil), in...)
	slices.SortFunc(out, func(a, b LedgerFact) int {
		if c := strings.Compare(a.Reference, b.Reference); c != 0 {
			return c
		}
		if c := cmp.Compare(a.AmountMinor, b.AmountMinor); c != 0 {
			return c
		}
		if !a.EffectiveAt.Equal(b.EffectiveAt) {
			if a.EffectiveAt.Before(b.EffectiveAt) {
				return -1
			}
			return 1
		}
		return strings.Compare(a.PostingID, b.PostingID)
	})
	return out
}

func sortedExternalLines(in []entity.ExternalStatementLine) []entity.ExternalStatementLine {
	out := append([]entity.ExternalStatementLine(nil), in...)
	slices.SortFunc(out, func(a, b entity.ExternalStatementLine) int {
		if c := strings.Compare(a.Reference, b.Reference); c != 0 {
			return c
		}
		if c := cmp.Compare(a.AmountMinor, b.AmountMinor); c != 0 {
			return c
		}
		if !a.EffectiveAt.Equal(b.EffectiveAt) {
			if a.EffectiveAt.Before(b.EffectiveAt) {
				return -1
			}
			return 1
		}
		return strings.Compare(a.LineID, b.LineID)
	})
	return out
}

func splitDuplicates(sorted []entity.ExternalStatementLine) ([]entity.ExternalStatementLine, []entity.ExternalStatementLine) {
	seen := make(map[string]string)
	var duplicates []entity.ExternalStatementLine
	var deduped []entity.ExternalStatementLine
	for _, line := range sorted {
		key := line.Reference + "\x00" + line.AssetCode + "\x00" + line.Hash
		if prev, ok := seen[key]; ok && prev != "" {
			duplicates = append(duplicates, line)
			continue
		}
		seen[key] = line.LineID
		deduped = append(deduped, line)
	}
	return duplicates, deduped
}

func duplicateBreaks(duplicates []entity.ExternalStatementLine, cfg MatchConfig) []entity.ReconciliationBreak {
	breaks := make([]entity.ReconciliationBreak, 0, len(duplicates))
	for i, line := range duplicates {
		breaks = append(breaks, entity.ReconciliationBreak{
			BreakID:     "brk:" + cfg.RunID + ":DUPLICATE:" + line.Reference + ":" + strconv.Itoa(i),
			RunID:       cfg.RunID,
			TenantID:    cfg.TenantID,
			Type:        valueobject.BreakDuplicate,
			ExternalRef: line.LineID,
			Evidence: map[string]string{
				"duplicate_of": line.Reference,
				"hash":         line.Hash,
			},
			RuleVersion: cfg.RuleVersion,
		})
	}
	return breaks
}

func matchByReference(ledger []LedgerFact, external []entity.ExternalStatementLine, cfg MatchConfig) ([]MatchGroup, []entity.ReconciliationBreak, error) {
	ledgerByRef := groupLedger(ledger)
	externalByRef := groupExternal(external)
	refs := unionRefs(ledgerByRef, externalByRef)
	var matched []MatchGroup
	var breaks []entity.ReconciliationBreak
	for _, ref := range refs {
		lfacts := ledgerByRef[ref]
		elines := externalByRef[ref]
		groups, refBreaks, err := matchOneReference(ref, lfacts, elines, cfg)
		if err != nil {
			return nil, nil, err
		}
		matched = append(matched, groups...)
		breaks = append(breaks, refBreaks...)
	}
	if matched == nil {
		matched = []MatchGroup{}
	}
	if breaks == nil {
		breaks = []entity.ReconciliationBreak{}
	}
	return matched, breaks, nil
}

func groupLedger(ledger []LedgerFact) map[string][]LedgerFact {
	out := make(map[string][]LedgerFact)
	for _, f := range ledger {
		out[f.Reference] = append(out[f.Reference], f)
	}
	return out
}

func groupExternal(external []entity.ExternalStatementLine) map[string][]entity.ExternalStatementLine {
	out := make(map[string][]entity.ExternalStatementLine)
	for _, l := range external {
		out[l.Reference] = append(out[l.Reference], l)
	}
	return out
}

func unionRefs(a map[string][]LedgerFact, b map[string][]entity.ExternalStatementLine) []string {
	set := make(map[string]struct{})
	for k := range a {
		set[k] = struct{}{}
	}
	for k := range b {
		set[k] = struct{}{}
	}
	refs := make([]string, 0, len(set))
	for k := range set {
		refs = append(refs, k)
	}
	slices.Sort(refs)
	return refs
}

func matchOneReference(ref string, lfacts []LedgerFact, elines []entity.ExternalStatementLine, cfg MatchConfig) ([]MatchGroup, []entity.ReconciliationBreak, error) {
	if len(lfacts) == 0 {
		return nil, missingLedgerBreaks(ref, elines, cfg), nil
	}
	if len(elines) == 0 {
		return nil, missingBankBreaks(ref, lfacts, cfg), nil
	}
	asset := lfacts[0].AssetCode
	if !allAssetsEqual(lfacts, elines, asset) {
		ledgerSum, externalSum, sumErr := checkedSums(lfacts, elines)
		if sumErr != nil {
			return nil, nil, sumErr
		}
		return nil, []entity.ReconciliationBreak{amountBreak(ref, lfacts, elines, ledgerSum, externalSum, cfg, 0)}, nil
	}
	tolerance, err := toleranceFor(cfg, asset)
	if err != nil {
		return nil, nil, err
	}
	ledgerSum, externalSum, sumErr := checkedSums(lfacts, elines)
	if sumErr != nil {
		return nil, nil, sumErr
	}
	skew := maxSkew(lfacts, elines)
	diff, diffErr := checkedDiff(ledgerSum, externalSum)
	if diffErr != nil {
		return nil, nil, diffErr
	}
	if diff > tolerance {
		return nil, []entity.ReconciliationBreak{amountBreak(ref, lfacts, elines, ledgerSum, externalSum, cfg, tolerance)}, nil
	}
	if skew > cfg.TimingWindow {
		return nil, []entity.ReconciliationBreak{dateBreak(ref, lfacts, elines, ledgerSum, externalSum, cfg)}, nil
	}
	return []MatchGroup{matchedGroup(ref, lfacts, elines, ledgerSum, externalSum, cfg)}, nil, nil
}

func missingLedgerBreaks(ref string, elines []entity.ExternalStatementLine, cfg MatchConfig) []entity.ReconciliationBreak {
	breaks := make([]entity.ReconciliationBreak, 0, len(elines))
	for i, line := range elines {
		breaks = append(breaks, entity.ReconciliationBreak{
			BreakID:     "brk:" + cfg.RunID + ":MISSING_IN_LEDGER:" + ref + ":" + strconv.Itoa(i),
			RunID:       cfg.RunID,
			TenantID:    cfg.TenantID,
			Type:        valueobject.BreakMissingInLedger,
			ExternalRef: line.LineID,
			ActualMinor: line.AmountMinor,
			Evidence:    map[string]string{evidenceReferenceKey: ref},
			RuleVersion: cfg.RuleVersion,
		})
	}
	return breaks
}

func missingBankBreaks(ref string, lfacts []LedgerFact, cfg MatchConfig) []entity.ReconciliationBreak {
	breaks := make([]entity.ReconciliationBreak, 0, len(lfacts))
	for i, f := range lfacts {
		breaks = append(breaks, entity.ReconciliationBreak{
			BreakID:       "brk:" + cfg.RunID + ":MISSING_IN_BANK:" + ref + ":" + strconv.Itoa(i),
			RunID:         cfg.RunID,
			TenantID:      cfg.TenantID,
			Type:          valueobject.BreakMissingInBank,
			LedgerRef:     f.PostingID,
			ExpectedMinor: f.AmountMinor,
			Evidence:      map[string]string{evidenceReferenceKey: ref},
			RuleVersion:   cfg.RuleVersion,
		})
	}
	return breaks
}

func amountBreak(ref string, lfacts []LedgerFact, elines []entity.ExternalStatementLine, ledgerSum, externalSum int64, cfg MatchConfig, tolerance int64) entity.ReconciliationBreak {
	return entity.ReconciliationBreak{
		BreakID:       "brk:" + cfg.RunID + ":AMOUNT_MISMATCH:" + ref + ":0",
		RunID:         cfg.RunID,
		TenantID:      cfg.TenantID,
		Type:          valueobject.BreakAmountMismatch,
		LedgerRef:     lfacts[0].PostingID,
		ExternalRef:   elines[0].LineID,
		ExpectedMinor: ledgerSum,
		ActualMinor:   externalSum,
		Evidence: map[string]string{
			evidenceReferenceKey: ref,
			"tolerance_minor":    strconv.FormatInt(tolerance, 10),
		},
		RuleVersion: cfg.RuleVersion,
	}
}

func dateBreak(ref string, lfacts []LedgerFact, elines []entity.ExternalStatementLine, ledgerSum, externalSum int64, cfg MatchConfig) entity.ReconciliationBreak {
	return entity.ReconciliationBreak{
		BreakID:       "brk:" + cfg.RunID + ":DATE_MISMATCH:" + ref + ":0",
		RunID:         cfg.RunID,
		TenantID:      cfg.TenantID,
		Type:          valueobject.BreakDateMismatch,
		LedgerRef:     lfacts[0].PostingID,
		ExternalRef:   elines[0].LineID,
		ExpectedMinor: ledgerSum,
		ActualMinor:   externalSum,
		Evidence: map[string]string{
			evidenceReferenceKey: ref,
			"timing_window":      cfg.TimingWindow.String(),
		},
		RuleVersion: cfg.RuleVersion,
	}
}

func matchedGroup(ref string, lfacts []LedgerFact, elines []entity.ExternalStatementLine, ledgerSum, externalSum int64, cfg MatchConfig) MatchGroup {
	confidence := "HIGH"
	if len(lfacts) != 1 || len(elines) != 1 {
		confidence = "MEDIUM"
	}
	ledgerRefs := make([]string, 0, len(lfacts))
	for _, f := range lfacts {
		ledgerRefs = append(ledgerRefs, f.PostingID)
	}
	externalRefs := make([]string, 0, len(elines))
	for _, l := range elines {
		externalRefs = append(externalRefs, l.LineID)
	}
	return MatchGroup{
		GroupID:       "grp:" + cfg.RunID + ":" + ref + ":0",
		RuleVersion:   cfg.RuleVersion,
		Confidence:    confidence,
		Decision:      valueobject.MatchMatched,
		LedgerRefs:    ledgerRefs,
		ExternalRefs:  externalRefs,
		LedgerSum:     ledgerSum,
		ExternalSum:   externalSum,
		DecisionActor: cfg.Actor,
	}
}

func toleranceFor(cfg MatchConfig, asset string) (int64, error) {
	for _, t := range cfg.Tolerances {
		if t.Rail == cfg.Rail && t.AssetCode == asset {
			if t.ToleranceMinor < 0 {
				return 0, entity.NewError("TOLERANCE_POLICY_REQUIRED", "tolerance must be non-negative")
			}
			return t.ToleranceMinor, nil
		}
	}
	return 0, entity.NewError("TOLERANCE_POLICY_REQUIRED", "tolerance policy is required for rail and asset")
}

func checkedSums(lfacts []LedgerFact, elines []entity.ExternalStatementLine) (int64, int64, error) {
	ledgerSum, err := sumLedgerChecked(lfacts)
	if err != nil {
		return 0, 0, err
	}
	externalSum, err := sumExternalChecked(elines)
	if err != nil {
		return 0, 0, err
	}
	return ledgerSum, externalSum, nil
}

func sumLedgerChecked(lfacts []LedgerFact) (int64, error) {
	var sum int64
	for _, f := range lfacts {
		next, ok := checkedAdd(sum, f.AmountMinor)
		if !ok {
			return 0, entity.NewError("AMOUNT_OVERFLOW", "ledger sum overflowed int64")
		}
		sum = next
	}
	return sum, nil
}

func sumExternalChecked(elines []entity.ExternalStatementLine) (int64, error) {
	var sum int64
	for _, l := range elines {
		next, ok := checkedAdd(sum, l.AmountMinor)
		if !ok {
			return 0, entity.NewError("AMOUNT_OVERFLOW", "external sum overflowed int64")
		}
		sum = next
	}
	return sum, nil
}

func allAssetsEqual(lfacts []LedgerFact, elines []entity.ExternalStatementLine, asset string) bool {
	for _, f := range lfacts {
		if f.AssetCode != asset {
			return false
		}
	}
	for _, l := range elines {
		if l.AssetCode != asset {
			return false
		}
	}
	return true
}

func maxSkew(lfacts []LedgerFact, elines []entity.ExternalStatementLine) time.Duration {
	var max time.Duration
	for _, f := range lfacts {
		for _, l := range elines {
			skew := f.EffectiveAt.Sub(l.EffectiveAt)
			if skew < 0 {
				skew = -skew
			}
			if skew > max {
				max = skew
			}
		}
	}
	return max
}

func typeAllowed(t valueobject.BreakType, allowed []valueobject.BreakType) bool {
	return slices.Contains(allowed, t)
}

func checkedDiff(a, b int64) (int64, error) {
	diff := a - b
	if (a > 0 && b < 0 && diff < 0) || (a < 0 && b > 0 && diff > 0) {
		return 0, entity.NewError("AMOUNT_OVERFLOW", "amount difference overflowed int64")
	}
	if diff < 0 {
		if diff == math.MinInt64 {
			return 0, entity.NewError("AMOUNT_OVERFLOW", "amount difference overflowed int64")
		}
		return -diff, nil
	}
	return diff, nil
}
