package service

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
)

// AssessTransactionFee computes amount×bps/10000 clamped to [floor, cap] with
// checked int64 math. bps is basis points (290 = 2.90%).
func AssessTransactionFee(amountMinor, bps, floorMinor, capMinor int64) (int64, error) {
	if amountMinor <= 0 {
		return 0, entity.NewError("INVALID_FEE_AMOUNT", "fee amount must be positive")
	}
	if bps < 0 {
		return 0, entity.NewError("INVALID_FEE_BPS", "fee basis points must be non-negative")
	}
	if bps == 0 {
		return 0, nil
	}
	if floorMinor < 0 || capMinor < 0 || (capMinor > 0 && capMinor < floorMinor) {
		return 0, entity.NewError("INVALID_FEE_BOUNDS", "fee cap/floor bounds are invalid")
	}
	hi, ok := checkedMul(amountMinor, bps)
	if !ok {
		return 0, entity.NewError("FEE_OVERFLOW", "fee computation overflowed")
	}
	fee := max(hi/10000, floorMinor)
	if capMinor > 0 {
		fee = min(fee, capMinor)
	}
	return fee, nil
}

// MonthlyTier is one volume-tier fee schedule row.
type MonthlyTier struct {
	// UpToMinor is the inclusive volume ceiling; 0 means unbounded top tier.
	UpToMinor int64
	BPS       int64
}

// AssessMonthlyFee selects the tier by volume and applies its bps schedule.
func AssessMonthlyFee(volumeMinor int64, tiers []MonthlyTier) (int64, error) {
	if volumeMinor < 0 {
		return 0, entity.NewError("INVALID_FEE_AMOUNT", "fee volume must be non-negative")
	}
	if len(tiers) == 0 {
		return 0, entity.NewError("FEE_SCHEDULE_REQUIRED", "monthly fee schedule is required")
	}
	selected := tiers[len(tiers)-1]
	for _, t := range tiers {
		if t.BPS < 0 || t.UpToMinor < 0 {
			return 0, entity.NewError("FEE_SCHEDULE_INVALID", "fee schedule tier is invalid")
		}
		if t.UpToMinor == 0 || volumeMinor <= t.UpToMinor {
			selected = t
			break
		}
	}
	hi, ok := checkedMul(volumeMinor, selected.BPS)
	if !ok {
		return 0, entity.NewError("FEE_OVERFLOW", "fee computation overflowed")
	}
	return hi / 10000, nil
}

// FeeSplit separates a total fee into processor/network and platform parts:
// explicit policy data, never an assumed proportion (ledger-core §6.4).
type FeeSplit struct {
	TotalMinor     int64
	ProcessorMinor int64
	PlatformMinor  int64
}

// SplitFee builds a validated fee split: platform is total minus processor.
func SplitFee(totalMinor, processorMinor int64) (FeeSplit, error) {
	if totalMinor < 0 || processorMinor < 0 || processorMinor > totalMinor {
		return FeeSplit{}, entity.NewError("FEE_SPLIT_INVALID", "fee split requires 0 <= processor <= total")
	}
	return FeeSplit{TotalMinor: totalMinor, ProcessorMinor: processorMinor, PlatformMinor: totalMinor - processorMinor}, nil
}
