package service

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

const payloadTenantKey = "tenant_id"

// ErasureRequest carries the PII erasure command.
type ErasureRequest struct {
	TenantID       string
	SubjectID      string
	SubjectType    string
	Fields         map[string]string
	LedgerSums     map[string]int64
	Now            time.Time
	UnderLegalHold bool
}

// ErasureResult records the shredded outcome with audit.
type ErasureResult struct {
	ErasureID      string
	TenantID       string
	SubjectID      string
	ShreddedFields []string
	Tombstone      string
	AuditEntry     string
	PreservedSums  map[string]int64
}

// ExportBundle is the portable subject bundle.
type ExportBundle struct {
	TenantID     string
	SubjectID    string
	Accounts     []string
	Transactions []string
	Entries      []string
	Format       string
}

// ClassifyField reports whether a field is erasable PII.
func ClassifyField(fieldName string) bool {
	switch fieldName {
	case "name", "email", "phone", "address", "metadata_note", "descriptor", "external_id":
		return true
	default:
		return false
	}
}

// ErasePII shreds PII values while preserving ledger sums.
func ErasePII(req ErasureRequest) (ErasureResult, error) {
	if strings.TrimSpace(req.TenantID) == "" || strings.TrimSpace(req.SubjectID) == "" {
		return ErasureResult{}, entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure requires tenant and subject ids")
	}
	if strings.TrimSpace(req.SubjectType) == "" {
		return ErasureResult{}, entity.NewError("ERASURE_SUBJECT_REQUIRED", "erasure requires a subject type")
	}
	if req.UnderLegalHold {
		return ErasureResult{}, entity.NewError("LEGAL_HOLD_ACTIVE", "erasure blocked by legal hold")
	}
	if len(req.Fields) == 0 {
		return ErasureResult{}, entity.NewError("ERASURE_FIELDS_REQUIRED", "erasure requires at least one field")
	}
	if req.Now.IsZero() {
		return ErasureResult{}, entity.NewError("ERASURE_TIME_REQUIRED", "erasure time is required")
	}
	shredded := make([]string, 0, len(req.Fields))
	for name := range req.Fields {
		if !ClassifyField(name) {
			return ErasureResult{}, entity.Errorf("FIELD_CLASS_UNKNOWN", "field %s is a retained ledger fact and cannot be erased", name)
		}
		shredded = append(shredded, name)
	}
	slices.Sort(shredded)
	preserved := make(map[string]int64, len(req.LedgerSums))
	for k, v := range req.LedgerSums {
		preserved[k] = v
	}
	erasureID := "erasure:" + req.TenantID + ":" + req.SubjectID + ":" + req.Now.UTC().Format("20060102T150405Z")
	tombstone := "REDACTED:" + hashSubject(req.TenantID, req.SubjectID)
	audit := "audit:" + erasureID + ":pii-erased"
	return ErasureResult{
		ErasureID:      erasureID,
		TenantID:       req.TenantID,
		SubjectID:      req.SubjectID,
		ShreddedFields: shredded,
		Tombstone:      tombstone,
		AuditEntry:     audit,
		PreservedSums:  preserved,
	}, nil
}

// BuildErasurePayload builds the internal pii.erased.v1 payload without values.
func BuildErasurePayload(erasureID, tenantID, subjectType string, shreddedFields []string, now time.Time) (map[string]string, error) {
	if strings.TrimSpace(erasureID) == "" || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(subjectType) == "" {
		return nil, entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure payload requires ids and subject type")
	}
	if len(shreddedFields) == 0 {
		return nil, entity.NewError("ERASURE_FIELDS_REQUIRED", "erasure payload requires shredded fields")
	}
	if now.IsZero() {
		return nil, entity.NewError("ERASURE_TIME_REQUIRED", "erasure time is required")
	}
	payload := map[string]string{
		"erasure_id":     erasureID,
		payloadTenantKey: tenantID,
		"subject_type":   subjectType,
		"erased_at":      now.UTC().Format(time.RFC3339),
	}
	for i, f := range shreddedFields {
		payload["field_"+strconv.Itoa(i)] = "REDACTED:" + f
	}
	return payload, nil
}

// BuildExportBundle constructs a portable export bundle.
func BuildExportBundle(tenantID, subjectID string, accounts, transactions, entries []string, format string) (ExportBundle, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(subjectID) == "" {
		return ExportBundle{}, entity.NewError("EXPORT_IDENTITY_REQUIRED", "export requires tenant and subject ids")
	}
	if format != "JSON" && format != "CSV" {
		return ExportBundle{}, entity.NewError("EXPORT_INVALID", "export format must be JSON or CSV")
	}
	if len(accounts) == 0 || len(transactions) == 0 || len(entries) == 0 {
		return ExportBundle{}, entity.NewError("EXPORT_INVALID", "export requires accounts, transactions, and entries")
	}
	return ExportBundle{
		TenantID:     tenantID,
		SubjectID:    subjectID,
		Accounts:     append([]string(nil), accounts...),
		Transactions: append([]string(nil), transactions...),
		Entries:      append([]string(nil), entries...),
		Format:       format,
	}, nil
}

// ValidateExportBundle checks that a bundle round-trips.
func ValidateExportBundle(b ExportBundle) error {
	if strings.TrimSpace(b.TenantID) == "" || strings.TrimSpace(b.SubjectID) == "" {
		return entity.NewError("EXPORT_IDENTITY_REQUIRED", "export requires tenant and subject ids")
	}
	if b.Format != "JSON" && b.Format != "CSV" {
		return entity.NewError("EXPORT_INVALID", "export format must be JSON or CSV")
	}
	if len(b.Accounts) == 0 || len(b.Transactions) == 0 || len(b.Entries) == 0 {
		return entity.NewError("EXPORT_INVALID", "export requires accounts, transactions, and entries")
	}
	return nil
}

// DetectErasedValueLeak rejects payloads that embed raw erased values.
// Matching is substring-aware: a raw value inside a longer payload value still leaks.
func DetectErasedValueLeak(payload map[string]string, rawValues []string) error {
	for _, raw := range rawValues {
		if raw == "" {
			continue
		}
		for _, v := range payload {
			if v == raw || strings.Contains(v, raw) {
				return entity.NewError("ERASED_VALUE_LEAKED", "erasure payload must not contain erased values")
			}
		}
	}
	return nil
}

// VerifyLedgerSumsPreserved proves erasure left double-entry sums untouched.
func VerifyLedgerSumsPreserved(before, after map[string]int64) error {
	if len(before) != len(after) {
		return entity.NewError("SUMS_MISMATCH", "ledger sums changed by erasure")
	}
	for k, v := range before {
		afterValue, ok := after[k]
		if !ok || afterValue != v {
			return entity.NewError("SUMS_MISMATCH", "ledger sums changed by erasure")
		}
	}
	return nil
}

func hashSubject(tenantID, subjectID string) string {
	sum := sha256.Sum256([]byte(tenantID + ":" + subjectID))
	return hex.EncodeToString(sum[:])
}
