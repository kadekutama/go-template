package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
)

func TestErasePII(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	baseReq := service.ErasureRequest{
		TenantID:    "t-1",
		SubjectID:   "cust-1",
		SubjectType: "CUSTOMER",
		Fields: map[string]string{
			"name":  "Jane Doe",
			"email": "jane@example.com",
		},
		LedgerSums: map[string]int64{"USD": 15000},
		Now:        now,
	}

	type testCase struct {
		name           string
		req            service.ErasureRequest
		expectedResult service.ErasureResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "valid erasure preserves sums",
			req:  baseReq,
			expectedResult: service.ErasureResult{
				ErasureID:      "erasure:t-1:cust-1:20260914T000000Z",
				TenantID:       "t-1",
				SubjectID:      "cust-1",
				ShreddedFields: []string{"email", "name"},
				Tombstone:      "REDACTED:13a5c0ef3e67c90a497146429e539da3864841de9e6086a7b1ca3fdbc76dc195",
				AuditEntry:     "audit:erasure:t-1:cust-1:20260914T000000Z:pii-erased",
				PreservedSums:  map[string]int64{"USD": 15000},
			},
			expectedError: nil,
		},
		{
			name: "multi-field multi-asset erasure",
			req: func() service.ErasureRequest {
				r := baseReq
				r.Fields = map[string]string{
					"name":    "Jane",
					"email":   "j@e.c",
					"phone":   "1",
					"address": "x",
				}
				r.LedgerSums = map[string]int64{"USD": 15000, "EUR": 200}
				return r
			}(),
			expectedResult: service.ErasureResult{
				ErasureID:      "erasure:t-1:cust-1:20260914T000000Z",
				TenantID:       "t-1",
				SubjectID:      "cust-1",
				ShreddedFields: []string{"address", "email", "name", "phone"},
				Tombstone:      "REDACTED:13a5c0ef3e67c90a497146429e539da3864841de9e6086a7b1ca3fdbc76dc195",
				AuditEntry:     "audit:erasure:t-1:cust-1:20260914T000000Z:pii-erased",
				PreservedSums:  map[string]int64{"USD": 15000, "EUR": 200},
			},
			expectedError: nil,
		},
		{
			name: "retained ledger fact rejected",
			req: func() service.ErasureRequest {
				r := baseReq
				r.Fields = map[string]string{"amount_minor": "15000"}
				return r
			}(),
			expectedResult: service.ErasureResult{},
			expectedError:  entity.Errorf("FIELD_CLASS_UNKNOWN", "field %s is a retained ledger fact and cannot be erased", "amount_minor"),
		},
		{
			name: "missing tenant identity rejected",
			req: func() service.ErasureRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			expectedResult: service.ErasureResult{},
			expectedError:  entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure requires tenant and subject ids"),
		},
		{
			name: "whitespace tenant identity rejected",
			req: func() service.ErasureRequest {
				r := baseReq
				r.TenantID = "   "
				return r
			}(),
			expectedResult: service.ErasureResult{},
			expectedError:  entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure requires tenant and subject ids"),
		},
		{
			name: "missing subject identity rejected",
			req: func() service.ErasureRequest {
				r := baseReq
				r.SubjectID = ""
				return r
			}(),
			expectedResult: service.ErasureResult{},
			expectedError:  entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure requires tenant and subject ids"),
		},
		{
			name: "whitespace subject identity rejected",
			req: func() service.ErasureRequest {
				r := baseReq
				r.SubjectID = "   "
				return r
			}(),
			expectedResult: service.ErasureResult{},
			expectedError:  entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure requires tenant and subject ids"),
		},
		{
			name: "missing subject type rejected",
			req: func() service.ErasureRequest {
				r := baseReq
				r.SubjectType = ""
				return r
			}(),
			expectedResult: service.ErasureResult{},
			expectedError:  entity.NewError("ERASURE_SUBJECT_REQUIRED", "erasure requires a subject type"),
		},
		{
			name: "whitespace subject type rejected",
			req: func() service.ErasureRequest {
				r := baseReq
				r.SubjectType = "   "
				return r
			}(),
			expectedResult: service.ErasureResult{},
			expectedError:  entity.NewError("ERASURE_SUBJECT_REQUIRED", "erasure requires a subject type"),
		},
		{
			name: "empty fields rejected",
			req: func() service.ErasureRequest {
				r := baseReq
				r.Fields = map[string]string{}
				return r
			}(),
			expectedResult: service.ErasureResult{},
			expectedError:  entity.NewError("ERASURE_FIELDS_REQUIRED", "erasure requires at least one field"),
		},
		{
			name: "nil fields rejected",
			req: func() service.ErasureRequest {
				r := baseReq
				r.Fields = nil
				return r
			}(),
			expectedResult: service.ErasureResult{},
			expectedError:  entity.NewError("ERASURE_FIELDS_REQUIRED", "erasure requires at least one field"),
		},
		{
			name: "legal hold blocks erasure",
			req: func() service.ErasureRequest {
				r := baseReq
				r.Fields = map[string]string{"email": "a@b.c"}
				r.LedgerSums = map[string]int64{"USD": 100}
				r.UnderLegalHold = true
				return r
			}(),
			expectedResult: service.ErasureResult{},
			expectedError:  entity.NewError("LEGAL_HOLD_ACTIVE", "erasure blocked by legal hold"),
		},
		{
			name: "missing time rejected",
			req: func() service.ErasureRequest {
				r := baseReq
				r.Now = time.Time{}
				return r
			}(),
			expectedResult: service.ErasureResult{},
			expectedError:  entity.NewError("ERASURE_TIME_REQUIRED", "erasure time is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ErasePII(tc.req)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestGDPRClassifyField(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		fieldName      string
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "name erasable",
			fieldName:      "name",
			expectedResult: true,
		},
		{
			name:           "email erasable",
			fieldName:      "email",
			expectedResult: true,
		},
		{
			name:           "phone erasable",
			fieldName:      "phone",
			expectedResult: true,
		},
		{
			name:           "address erasable",
			fieldName:      "address",
			expectedResult: true,
		},
		{
			name:           "metadata_note erasable",
			fieldName:      "metadata_note",
			expectedResult: true,
		},
		{
			name:           "descriptor erasable",
			fieldName:      "descriptor",
			expectedResult: true,
		},
		{
			name:           "external_id erasable",
			fieldName:      "external_id",
			expectedResult: true,
		},
		{
			name:           "amount retained",
			fieldName:      "amount_minor",
			expectedResult: false,
		},
		{
			name:           "posting retained",
			fieldName:      "posting_id",
			expectedResult: false,
		},
		{
			name:           "empty retained",
			fieldName:      "",
			expectedResult: false,
		},
		{
			name:           "arbitrary unknown retained",
			fieldName:      "account_type",
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.ClassifyField(tc.fieldName)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestExportBundle(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenantID       string
		subjectID      string
		accounts       []string
		transactions   []string
		entries        []string
		format         string
		expectedResult service.ExportBundle
		expectedError  error
	}

	testCases := []testCase{
		{
			name:         "valid json bundle",
			tenantID:     "t-1",
			subjectID:    "cust-1",
			accounts:     []string{"a-1"},
			transactions: []string{"p-1"},
			entries:      []string{"e-1"},
			format:       "JSON",
			expectedResult: service.ExportBundle{
				TenantID:     "t-1",
				SubjectID:    "cust-1",
				Accounts:     []string{"a-1"},
				Transactions: []string{"p-1"},
				Entries:      []string{"e-1"},
				Format:       "JSON",
			},
			expectedError: nil,
		},
		{
			name:         "valid csv bundle",
			tenantID:     "t-1",
			subjectID:    "cust-1",
			accounts:     []string{"a-1"},
			transactions: []string{"p-1"},
			entries:      []string{"e-1"},
			format:       "CSV",
			expectedResult: service.ExportBundle{
				TenantID:     "t-1",
				SubjectID:    "cust-1",
				Accounts:     []string{"a-1"},
				Transactions: []string{"p-1"},
				Entries:      []string{"e-1"},
				Format:       "CSV",
			},
			expectedError: nil,
		},
		{
			name:         "valid multi-entry csv bundle",
			tenantID:     "t-1",
			subjectID:    "cust-1",
			accounts:     []string{"a-1", "a-2"},
			transactions: []string{"p-1"},
			entries:      []string{"e-1", "e-2"},
			format:       "CSV",
			expectedResult: service.ExportBundle{
				TenantID:     "t-1",
				SubjectID:    "cust-1",
				Accounts:     []string{"a-1", "a-2"},
				Transactions: []string{"p-1"},
				Entries:      []string{"e-1", "e-2"},
				Format:       "CSV",
			},
			expectedError: nil,
		},
		{
			name:           "bad format rejected",
			tenantID:       "t-1",
			subjectID:      "cust-1",
			accounts:       []string{"a-1"},
			transactions:   []string{"p-1"},
			entries:        []string{"e-1"},
			format:         "XML",
			expectedResult: service.ExportBundle{},
			expectedError:  entity.NewError("EXPORT_INVALID", "export format must be JSON or CSV"),
		},
		{
			name:           "lowercase format rejected",
			tenantID:       "t-1",
			subjectID:      "cust-1",
			accounts:       []string{"a-1"},
			transactions:   []string{"p-1"},
			entries:        []string{"e-1"},
			format:         "json",
			expectedResult: service.ExportBundle{},
			expectedError:  entity.NewError("EXPORT_INVALID", "export format must be JSON or CSV"),
		},
		{
			name:           "missing tenant rejected",
			tenantID:       "",
			subjectID:      "cust-1",
			accounts:       []string{"a-1"},
			transactions:   []string{"p-1"},
			entries:        []string{"e-1"},
			format:         "JSON",
			expectedResult: service.ExportBundle{},
			expectedError:  entity.NewError("EXPORT_IDENTITY_REQUIRED", "export requires tenant and subject ids"),
		},
		{
			name:           "whitespace tenant rejected",
			tenantID:       "   ",
			subjectID:      "cust-1",
			accounts:       []string{"a-1"},
			transactions:   []string{"p-1"},
			entries:        []string{"e-1"},
			format:         "JSON",
			expectedResult: service.ExportBundle{},
			expectedError:  entity.NewError("EXPORT_IDENTITY_REQUIRED", "export requires tenant and subject ids"),
		},
		{
			name:           "missing subject rejected",
			tenantID:       "t-1",
			subjectID:      "",
			accounts:       []string{"a-1"},
			transactions:   []string{"p-1"},
			entries:        []string{"e-1"},
			format:         "JSON",
			expectedResult: service.ExportBundle{},
			expectedError:  entity.NewError("EXPORT_IDENTITY_REQUIRED", "export requires tenant and subject ids"),
		},
		{
			name:           "whitespace subject rejected",
			tenantID:       "t-1",
			subjectID:      "   ",
			accounts:       []string{"a-1"},
			transactions:   []string{"p-1"},
			entries:        []string{"e-1"},
			format:         "JSON",
			expectedResult: service.ExportBundle{},
			expectedError:  entity.NewError("EXPORT_IDENTITY_REQUIRED", "export requires tenant and subject ids"),
		},
		{
			name:           "missing accounts rejected",
			tenantID:       "t-1",
			subjectID:      "cust-1",
			accounts:       nil,
			transactions:   []string{"p-1"},
			entries:        []string{"e-1"},
			format:         "JSON",
			expectedResult: service.ExportBundle{},
			expectedError:  entity.NewError("EXPORT_INVALID", "export requires accounts, transactions, and entries"),
		},
		{
			name:           "missing transactions rejected",
			tenantID:       "t-1",
			subjectID:      "cust-1",
			accounts:       []string{"a-1"},
			transactions:   nil,
			entries:        []string{"e-1"},
			format:         "JSON",
			expectedResult: service.ExportBundle{},
			expectedError:  entity.NewError("EXPORT_INVALID", "export requires accounts, transactions, and entries"),
		},
		{
			name:           "missing entries rejected",
			tenantID:       "t-1",
			subjectID:      "cust-1",
			accounts:       []string{"a-1"},
			transactions:   []string{"p-1"},
			entries:        nil,
			format:         "JSON",
			expectedResult: service.ExportBundle{},
			expectedError:  entity.NewError("EXPORT_INVALID", "export requires accounts, transactions, and entries"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.BuildExportBundle(tc.tenantID, tc.subjectID, tc.accounts, tc.transactions, tc.entries, tc.format)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestDetectErasedValueLeak(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		payload       map[string]string
		rawValues     []string
		expectedError error
	}

	testCases := []testCase{
		{
			name: "clean payload passes",
			payload: map[string]string{
				"erasure_id":   "erasure:t-1:cust-1:20260914T000000Z",
				"tenant_id":    "t-1",
				"subject_type": "CUSTOMER",
				"field_0":      "REDACTED:name",
			},
			rawValues:     []string{"Jane Doe", "jane@example.com"},
			expectedError: nil,
		},
		{
			name: "leaked value rejected",
			payload: map[string]string{
				"erasure_id":   "erasure:t-1:cust-1:20260914T000000Z",
				"tenant_id":    "t-1",
				"subject_type": "CUSTOMER",
				"field_0":      "Jane Doe",
			},
			rawValues:     []string{"Jane Doe"},
			expectedError: entity.NewError("ERASED_VALUE_LEAKED", "erasure payload must not contain erased values"),
		},
		{
			name:          "substring leak caught",
			payload:       map[string]string{"note": "customer Jane Doe called"},
			rawValues:     []string{"Jane Doe"},
			expectedError: entity.NewError("ERASED_VALUE_LEAKED", "erasure payload must not contain erased values"),
		},
		{
			name:          "clean passes",
			payload:       map[string]string{"note": "REDACTED:name"},
			rawValues:     []string{"Jane Doe"},
			expectedError: nil,
		},
		{
			name: "empty raws pass",
			payload: map[string]string{
				"erasure_id":   "erasure:t-1:cust-1:20260914T000000Z",
				"tenant_id":    "t-1",
				"subject_type": "CUSTOMER",
				"field_0":      "REDACTED:name",
			},
			rawValues:     nil,
			expectedError: nil,
		},
		{
			name: "empty strings in raw values ignored",
			payload: map[string]string{
				"erasure_id":   "erasure:t-1:cust-1:20260914T000000Z",
				"tenant_id":    "t-1",
				"subject_type": "CUSTOMER",
				"field_0":      "REDACTED:name",
			},
			rawValues:     []string{"", ""},
			expectedError: nil,
		},
		{
			name:          "empty payload passes",
			payload:       map[string]string{},
			rawValues:     []string{"Jane Doe"},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.DetectErasedValueLeak(tc.payload, tc.rawValues)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestVerifyLedgerSumsPreserved(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		before        map[string]int64
		after         map[string]int64
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "identical multi-asset sums pass",
			before:        map[string]int64{"USD": 15000, "EUR": 9223372036854775800},
			after:         map[string]int64{"USD": 15000, "EUR": 9223372036854775800},
			expectedError: nil,
		},
		{
			name:          "empty sums pass",
			before:        map[string]int64{},
			after:         map[string]int64{},
			expectedError: nil,
		},
		{
			name:          "negative amounts match pass",
			before:        map[string]int64{"USD": -500},
			after:         map[string]int64{"USD": -500},
			expectedError: nil,
		},
		{
			name:          "changed sum fails",
			before:        map[string]int64{"USD": 15000},
			after:         map[string]int64{"USD": 14999},
			expectedError: entity.NewError("SUMS_MISMATCH", "ledger sums changed by erasure"),
		},
		{
			name:          "negative amount mismatch fails",
			before:        map[string]int64{"USD": -500},
			after:         map[string]int64{"USD": -501},
			expectedError: entity.NewError("SUMS_MISMATCH", "ledger sums changed by erasure"),
		},
		{
			name:          "dropped asset fails",
			before:        map[string]int64{"USD": 1, "EUR": 2},
			after:         map[string]int64{"USD": 1},
			expectedError: entity.NewError("SUMS_MISMATCH", "ledger sums changed by erasure"),
		},
		{
			name:          "extra asset in after fails",
			before:        map[string]int64{"USD": 1},
			after:         map[string]int64{"USD": 1, "EUR": 2},
			expectedError: entity.NewError("SUMS_MISMATCH", "ledger sums changed by erasure"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.VerifyLedgerSumsPreserved(tc.before, tc.after)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestBuildErasurePayloadValidation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name           string
		erasureID      string
		tenantID       string
		subjectType    string
		shreddedFields []string
		now            time.Time
		expectedResult map[string]string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid payload",
			erasureID:      "erasure:t-1:c:20260914T000000Z",
			tenantID:       "t-1",
			subjectType:    "CUSTOMER",
			shreddedFields: []string{"email"},
			now:            now,
			expectedResult: map[string]string{
				"erasure_id":   "erasure:t-1:c:20260914T000000Z",
				"tenant_id":    "t-1",
				"subject_type": "CUSTOMER",
				"erased_at":    "2026-09-14T00:00:00Z",
				"field_0":      "REDACTED:email",
			},
			expectedError: nil,
		},
		{
			name:           "multiple shredded fields indexed",
			erasureID:      "erasure:t-1:c:20260914T000000Z",
			tenantID:       "t-1",
			subjectType:    "MERCHANT",
			shreddedFields: []string{"email", "name"},
			now:            now,
			expectedResult: map[string]string{
				"erasure_id":   "erasure:t-1:c:20260914T000000Z",
				"tenant_id":    "t-1",
				"subject_type": "MERCHANT",
				"erased_at":    "2026-09-14T00:00:00Z",
				"field_0":      "REDACTED:email",
				"field_1":      "REDACTED:name",
			},
			expectedError: nil,
		},
		{
			name:           "missing erasure id rejected",
			erasureID:      "",
			tenantID:       "t-1",
			subjectType:    "CUSTOMER",
			shreddedFields: []string{"email"},
			now:            now,
			expectedResult: nil,
			expectedError:  entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure payload requires ids and subject type"),
		},
		{
			name:           "whitespace erasure id rejected",
			erasureID:      "   ",
			tenantID:       "t-1",
			subjectType:    "CUSTOMER",
			shreddedFields: []string{"email"},
			now:            now,
			expectedResult: nil,
			expectedError:  entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure payload requires ids and subject type"),
		},
		{
			name:           "missing tenant id rejected",
			erasureID:      "e-1",
			tenantID:       "",
			subjectType:    "CUSTOMER",
			shreddedFields: []string{"email"},
			now:            now,
			expectedResult: nil,
			expectedError:  entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure payload requires ids and subject type"),
		},
		{
			name:           "whitespace tenant rejected",
			erasureID:      "e-1",
			tenantID:       "   ",
			subjectType:    "CUSTOMER",
			shreddedFields: []string{"email"},
			now:            now,
			expectedResult: nil,
			expectedError:  entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure payload requires ids and subject type"),
		},
		{
			name:           "missing subject type rejected",
			erasureID:      "e-1",
			tenantID:       "t-1",
			subjectType:    "",
			shreddedFields: []string{"email"},
			now:            now,
			expectedResult: nil,
			expectedError:  entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure payload requires ids and subject type"),
		},
		{
			name:           "whitespace subject type rejected",
			erasureID:      "e-1",
			tenantID:       "t-1",
			subjectType:    "   ",
			shreddedFields: []string{"email"},
			now:            now,
			expectedResult: nil,
			expectedError:  entity.NewError("ERASURE_IDENTITY_REQUIRED", "erasure payload requires ids and subject type"),
		},
		{
			name:           "nil fields rejected",
			erasureID:      "e-1",
			tenantID:       "t-1",
			subjectType:    "CUSTOMER",
			shreddedFields: nil,
			now:            now,
			expectedResult: nil,
			expectedError:  entity.NewError("ERASURE_FIELDS_REQUIRED", "erasure payload requires shredded fields"),
		},
		{
			name:           "empty fields rejected",
			erasureID:      "e-1",
			tenantID:       "t-1",
			subjectType:    "CUSTOMER",
			shreddedFields: []string{},
			now:            now,
			expectedResult: nil,
			expectedError:  entity.NewError("ERASURE_FIELDS_REQUIRED", "erasure payload requires shredded fields"),
		},
		{
			name:           "missing time rejected",
			erasureID:      "e-1",
			tenantID:       "t-1",
			subjectType:    "CUSTOMER",
			shreddedFields: []string{"email"},
			now:            time.Time{},
			expectedResult: nil,
			expectedError:  entity.NewError("ERASURE_TIME_REQUIRED", "erasure time is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.BuildErasurePayload(tc.erasureID, tc.tenantID, tc.subjectType, tc.shreddedFields, tc.now)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestValidateExportBundleDirect(t *testing.T) {
	t.Parallel()

	baseBundle := service.ExportBundle{
		TenantID:     "t-1",
		SubjectID:    "c-1",
		Accounts:     []string{"a-1"},
		Transactions: []string{"p-1"},
		Entries:      []string{"e-1"},
		Format:       "JSON",
	}

	type testCase struct {
		name          string
		b             service.ExportBundle
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid bundle",
			b:             baseBundle,
			expectedError: nil,
		},
		{
			name: "bad format",
			b: func() service.ExportBundle {
				b := baseBundle
				b.Format = "XML"
				return b
			}(),
			expectedError: entity.NewError("EXPORT_INVALID", "export format must be JSON or CSV"),
		},
		{
			name: "lowercase format rejected",
			b: func() service.ExportBundle {
				b := baseBundle
				b.Format = "json"
				return b
			}(),
			expectedError: entity.NewError("EXPORT_INVALID", "export format must be JSON or CSV"),
		},
		{
			name: "missing tenant identity",
			b: func() service.ExportBundle {
				b := baseBundle
				b.TenantID = ""
				return b
			}(),
			expectedError: entity.NewError("EXPORT_IDENTITY_REQUIRED", "export requires tenant and subject ids"),
		},
		{
			name: "whitespace tenant identity",
			b: func() service.ExportBundle {
				b := baseBundle
				b.TenantID = "  "
				return b
			}(),
			expectedError: entity.NewError("EXPORT_IDENTITY_REQUIRED", "export requires tenant and subject ids"),
		},
		{
			name: "missing subject identity",
			b: func() service.ExportBundle {
				b := baseBundle
				b.SubjectID = ""
				return b
			}(),
			expectedError: entity.NewError("EXPORT_IDENTITY_REQUIRED", "export requires tenant and subject ids"),
		},
		{
			name: "whitespace subject identity",
			b: func() service.ExportBundle {
				b := baseBundle
				b.SubjectID = "  "
				return b
			}(),
			expectedError: entity.NewError("EXPORT_IDENTITY_REQUIRED", "export requires tenant and subject ids"),
		},
		{
			name: "missing accounts",
			b: func() service.ExportBundle {
				b := baseBundle
				b.Accounts = nil
				return b
			}(),
			expectedError: entity.NewError("EXPORT_INVALID", "export requires accounts, transactions, and entries"),
		},
		{
			name: "empty accounts",
			b: func() service.ExportBundle {
				b := baseBundle
				b.Accounts = []string{}
				return b
			}(),
			expectedError: entity.NewError("EXPORT_INVALID", "export requires accounts, transactions, and entries"),
		},
		{
			name: "missing transactions",
			b: func() service.ExportBundle {
				b := baseBundle
				b.Transactions = nil
				return b
			}(),
			expectedError: entity.NewError("EXPORT_INVALID", "export requires accounts, transactions, and entries"),
		},
		{
			name: "empty transactions",
			b: func() service.ExportBundle {
				b := baseBundle
				b.Transactions = []string{}
				return b
			}(),
			expectedError: entity.NewError("EXPORT_INVALID", "export requires accounts, transactions, and entries"),
		},
		{
			name: "missing entries",
			b: func() service.ExportBundle {
				b := baseBundle
				b.Entries = nil
				return b
			}(),
			expectedError: entity.NewError("EXPORT_INVALID", "export requires accounts, transactions, and entries"),
		},
		{
			name: "empty entries",
			b: func() service.ExportBundle {
				b := baseBundle
				b.Entries = []string{}
				return b
			}(),
			expectedError: entity.NewError("EXPORT_INVALID", "export requires accounts, transactions, and entries"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateExportBundle(tc.b)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
