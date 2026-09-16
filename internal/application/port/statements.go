package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// StatementFormat names one supported bank-statement wire format. Formats are
// versioned strings owned here; parser quirks never leak to callers.
type StatementFormat string

// Supported statement formats.
const (
	StatementFormatCAMT053 StatementFormat = "camt.053"
	StatementFormatBAI2    StatementFormat = "bai2"
	StatementFormatCSV     StatementFormat = "csv"
)

// StatementLine is one parsed bank-statement line with the bank's reference.
type StatementLine struct {
	Reference   string
	AmountMinor int64
	AssetCode   valueobject.AssetCode
	BookedAt    time.Time
	Description string
}

// BankStatement is one parsed statement file with its lines in file order.
type BankStatement struct {
	Format      StatementFormat
	AccountIBAN string
	Lines       []StatementLine
}

// StatementParser is the bank-statement ingestion boundary (E10 implements
// one parser per format). Parsing is pure and deterministic: same bytes yield
// the same statement. Amount signs follow the format spec and are normalized
// to minor units here, never with floats.
type StatementParser interface {
	// Parse decodes one statement file. Pure; no I/O beyond its inputs.
	Parse(ctx context.Context, format StatementFormat, data []byte) (BankStatement, error)
}
