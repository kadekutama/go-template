package service

import (
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// SplitRequest is a Connect-style platform-split capture command: the charge
// lands on the platform, the application fee is carved at charge time, and
// the connected merchant is credited net. Funds snapshots stay with the
// caller; this pure function builds posting lines only, never persists.
type SplitRequest struct {
	PlatformAccount  valueobject.AccountID
	ConnectedAccount valueobject.AccountID
	ProcessorAccount valueobject.AccountID
	GrossMinor       int64
	FeeMinor         int64
	AssetCode        valueobject.AssetCode
	Grants           map[string]bool
	FXRate           valueobject.FxRate
	FXRatePresent    bool
	At               time.Time
}

// SplitLines is the capture posting shape per ledger-core §6.1: DR Processor
// Receivable gross; CR Connected Merchant Payable net; CR Platform Fee Revenue
// fee. Cross-currency splits additionally carry the converted settlement legs
// with an explicit gain/loss drift leg so both lots balance independently.
type SplitLines struct {
	DebitAccount        valueobject.AccountID
	DebitAmountMinor    int64
	ConnectedAccount    valueobject.AccountID
	NetMinor            int64
	PlatformAccount     valueobject.AccountID
	FeeMinor            int64
	AssetCode           valueobject.AssetCode
	SettlementAsset     valueobject.AssetCode
	ConvertedGrossMinor int64
	ConvertedNetMinor   int64
	ConvertedFeeMinor   int64
	GainLossMinor       int64
	FXRateID            string
}

// SplitRefundRequest reverses a split capture with an explicit fee-refund
// policy: the fee refund is caller-supplied, never assumed proportional.
type SplitRefundRequest struct {
	Original         SplitLines
	RefundGrossMinor int64
	FeeRefundMinor   int64
	FXRate           valueobject.FxRate
	FXRatePresent    bool
	At               time.Time
}

// SplitRefundLines reverses all three capture legs: CR Processor Receivable,
// DR Connected Merchant Payable, DR Platform Fee Revenue.
type SplitRefundLines struct {
	ProcessorAccount          valueobject.AccountID
	MerchantAccount           valueobject.AccountID
	PlatformAccount           valueobject.AccountID
	RefundGrossMinor          int64
	RefundNetMinor            int64
	FeeRefundMinor            int64
	AssetCode                 valueobject.AssetCode
	SettlementAsset           valueobject.AssetCode
	ConvertedRefundGrossMinor int64
	ConvertedRefundNetMinor   int64
	ConvertedFeeRefundMinor   int64
	GainLossMinor             int64
}

// ValidateSplit checks a platform-split capture and builds its posting lines.
func ValidateSplit(req SplitRequest, accounts map[valueobject.AccountID]entity.AccountData) (SplitLines, error) {
	if err := validateSplitShape(req); err != nil {
		return SplitLines{}, err
	}
	connected, err := lookupSplitAccounts(req, accounts)
	if err != nil {
		return SplitLines{}, err
	}
	if err := ValidateTransferGrant(req.Grants, string(connected.TenantID)); err != nil {
		return SplitLines{}, err
	}
	lines := SplitLines{
		DebitAccount:     req.ProcessorAccount,
		DebitAmountMinor: req.GrossMinor,
		ConnectedAccount: req.ConnectedAccount,
		NetMinor:         req.GrossMinor - req.FeeMinor,
		PlatformAccount:  req.PlatformAccount,
		FeeMinor:         req.FeeMinor,
		AssetCode:        req.AssetCode,
	}
	return settleSplitLegs(req, connected, lines)
}

// validateSplitShape enforces amount, asset, identity, and distinctness rules.
func validateSplitShape(req SplitRequest) error {
	if req.GrossMinor <= 0 {
		return entity.NewError("INVALID_SPLIT_AMOUNT", "split gross amount must be positive")
	}
	if req.FeeMinor < 0 || req.FeeMinor > req.GrossMinor {
		return entity.NewError("SPLIT_MISMATCH", "split fee must satisfy 0 <= fee <= gross")
	}
	if strings.TrimSpace(string(req.AssetCode)) == "" {
		return entity.NewError("SPLIT_ASSET_REQUIRED", "split requires an asset code")
	}
	if req.PlatformAccount == "" || req.ConnectedAccount == "" || req.ProcessorAccount == "" {
		return entity.NewError("SPLIT_ACCOUNT_REQUIRED", "split requires platform, connected, and processor accounts")
	}
	if req.PlatformAccount == req.ConnectedAccount || req.PlatformAccount == req.ProcessorAccount || req.ConnectedAccount == req.ProcessorAccount {
		return entity.NewError("SPLIT_ACCOUNT_INVALID", "split posting accounts must be distinct")
	}
	return nil
}

// lookupSplitAccounts resolves the three posting accounts, enforces ACTIVE
// status, and requires the processor debit and platform fee legs to post in
// the charge asset (ledger-core §6.1). It returns the connected account whose
// tenant grant and settlement asset drive the remaining checks.
func lookupSplitAccounts(req SplitRequest, accounts map[valueobject.AccountID]entity.AccountData) (entity.AccountData, error) {
	platform, ok := accounts[req.PlatformAccount]
	if !ok {
		return entity.AccountData{}, entity.NewError("PLATFORM_ACCOUNT_NOT_FOUND", "platform account is unknown")
	}
	connected, ok := accounts[req.ConnectedAccount]
	if !ok {
		return entity.AccountData{}, entity.NewError("CONNECTED_ACCOUNT_NOT_FOUND", "connected account is unknown")
	}
	processor, ok := accounts[req.ProcessorAccount]
	if !ok {
		return entity.AccountData{}, entity.NewError("PROCESSOR_ACCOUNT_NOT_FOUND", "processor account is unknown")
	}
	if err := checkTransferAccountActive(platform); err != nil {
		return entity.AccountData{}, err
	}
	if err := checkTransferAccountActive(connected); err != nil {
		return entity.AccountData{}, err
	}
	if err := checkTransferAccountActive(processor); err != nil {
		return entity.AccountData{}, err
	}
	if processor.AssetCode != req.AssetCode || platform.AssetCode != req.AssetCode {
		return entity.AccountData{}, entity.NewError("CURRENCY_MISMATCH", "split processor and platform legs must post in the charge asset")
	}
	return connected, nil
}

// settleSplitLegs fills the settlement legs: same-currency splits pass the
// charge amounts through, cross-currency splits convert each leg and record
// the half-up rounding drift as an explicit gain/loss leg.
func settleSplitLegs(req SplitRequest, connected entity.AccountData, lines SplitLines) (SplitLines, error) {
	if connected.AssetCode == req.AssetCode {
		lines.SettlementAsset = req.AssetCode
		lines.ConvertedGrossMinor = lines.DebitAmountMinor
		lines.ConvertedNetMinor = lines.NetMinor
		lines.ConvertedFeeMinor = lines.FeeMinor
		return lines, nil
	}
	if !req.FXRatePresent {
		return SplitLines{}, entity.NewError("FX_RATE_MISSING", "split requires an FX rate for the settlement asset")
	}
	if req.FXRate.Pair.Base != req.AssetCode || req.FXRate.Pair.Quote != connected.AssetCode {
		return SplitLines{}, entity.NewError("FX_RATE_MISMATCH", "fx rate pair must convert charge asset to settlement asset")
	}
	convertedGross, err := Convert(lines.DebitAmountMinor, req.FXRate, req.At)
	if err != nil {
		return SplitLines{}, err
	}
	convertedNet, err := Convert(lines.NetMinor, req.FXRate, req.At)
	if err != nil {
		return SplitLines{}, err
	}
	convertedFee, err := Convert(lines.FeeMinor, req.FXRate, req.At)
	if err != nil {
		return SplitLines{}, err
	}
	drift, err := splitDrift(convertedGross, convertedNet, convertedFee)
	if err != nil {
		return SplitLines{}, err
	}
	lines.SettlementAsset = connected.AssetCode
	lines.ConvertedGrossMinor = convertedGross
	lines.ConvertedNetMinor = convertedNet
	lines.ConvertedFeeMinor = convertedFee
	lines.GainLossMinor = drift
	lines.FXRateID = req.FXRate.ID
	return lines, nil
}

// splitDrift derives the settlement-lot gain/loss leg so convertedGross ==
// convertedNet + convertedFee + drift. Half-up per-leg rounding can leave a
// small drift; it posts explicitly instead of being netted away.
func splitDrift(convertedGross, convertedNet, convertedFee int64) (int64, error) {
	sum, ok := CheckedAdd(convertedNet, convertedFee)
	if !ok {
		return 0, entity.NewError("SPLIT_FX_UNBALANCED", "split settlement legs overflowed")
	}
	gain, ok := CheckedSub(convertedGross, sum)
	if !ok {
		return 0, entity.NewError("SPLIT_FX_UNBALANCED", "split settlement legs overflowed")
	}
	return gain, nil
}

// ValidateSplitRefund reverses a split capture's three legs under an explicit
// fee-refund policy.
func ValidateSplitRefund(req SplitRefundRequest) (SplitRefundLines, error) {
	if err := validateSplitRefundShape(req); err != nil {
		return SplitRefundLines{}, err
	}
	lines := SplitRefundLines{
		ProcessorAccount: req.Original.DebitAccount,
		MerchantAccount:  req.Original.ConnectedAccount,
		PlatformAccount:  req.Original.PlatformAccount,
		RefundGrossMinor: req.RefundGrossMinor,
		RefundNetMinor:   req.RefundGrossMinor - req.FeeRefundMinor,
		FeeRefundMinor:   req.FeeRefundMinor,
		AssetCode:        req.Original.AssetCode,
	}
	return settleSplitRefundLegs(req, lines)
}

// validateSplitRefundShape enforces refund bounds against the original lines
// and rejects inconsistent originals.
func validateSplitRefundShape(req SplitRefundRequest) error {
	orig := req.Original
	if orig.DebitAmountMinor != orig.FeeMinor+orig.NetMinor {
		return entity.NewError("SPLIT_MISMATCH", "original split lines are inconsistent")
	}
	if orig.FXRateID != "" && orig.ConvertedGrossMinor != orig.ConvertedNetMinor+orig.ConvertedFeeMinor+orig.GainLossMinor {
		return entity.NewError("SPLIT_MISMATCH", "original split settlement legs are inconsistent")
	}
	if req.RefundGrossMinor <= 0 {
		return entity.NewError("INVALID_REFUND_AMOUNT", "split refund gross must be positive")
	}
	if req.RefundGrossMinor > orig.DebitAmountMinor {
		return entity.NewError("REFUND_EXCEEDS_ORIGINAL", "split refund exceeds the original gross")
	}
	if req.FeeRefundMinor < 0 || req.FeeRefundMinor > req.RefundGrossMinor || req.FeeRefundMinor > orig.FeeMinor {
		return entity.NewError("FEE_REFUND_INVALID", "split fee refund must satisfy 0 <= fee <= min(refund gross, original fee)")
	}
	return nil
}

// settleSplitRefundLegs mirrors the capture settlement: same-currency refunds
// pass amounts through, cross-currency refunds reconvert each refund leg with
// an explicit drift leg.
func settleSplitRefundLegs(req SplitRefundRequest, lines SplitRefundLines) (SplitRefundLines, error) {
	if req.Original.FXRateID == "" {
		lines.SettlementAsset = req.Original.SettlementAsset
		if lines.SettlementAsset == "" {
			lines.SettlementAsset = req.Original.AssetCode
		}
		lines.ConvertedRefundGrossMinor = lines.RefundGrossMinor
		lines.ConvertedRefundNetMinor = lines.RefundNetMinor
		lines.ConvertedFeeRefundMinor = lines.FeeRefundMinor
		return lines, nil
	}
	if !req.FXRatePresent {
		return SplitRefundLines{}, entity.NewError("FX_RATE_MISSING", "split refund requires an FX rate for the settlement asset")
	}
	if req.FXRate.Pair.Base != req.Original.AssetCode || req.FXRate.Pair.Quote != req.Original.SettlementAsset {
		return SplitRefundLines{}, entity.NewError("FX_RATE_MISMATCH", "fx rate pair must convert charge asset to settlement asset")
	}
	convertedGross, err := Convert(lines.RefundGrossMinor, req.FXRate, req.At)
	if err != nil {
		return SplitRefundLines{}, err
	}
	convertedNet, err := Convert(lines.RefundNetMinor, req.FXRate, req.At)
	if err != nil {
		return SplitRefundLines{}, err
	}
	convertedFee, err := Convert(lines.FeeRefundMinor, req.FXRate, req.At)
	if err != nil {
		return SplitRefundLines{}, err
	}
	drift, err := splitDrift(convertedGross, convertedNet, convertedFee)
	if err != nil {
		return SplitRefundLines{}, err
	}
	lines.SettlementAsset = req.Original.SettlementAsset
	lines.ConvertedRefundGrossMinor = convertedGross
	lines.ConvertedRefundNetMinor = convertedNet
	lines.ConvertedFeeRefundMinor = convertedFee
	lines.GainLossMinor = drift
	return lines, nil
}
