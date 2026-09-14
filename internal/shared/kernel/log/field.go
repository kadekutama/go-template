package log

// Standardized field keys for structured logging.
const (
	FieldError    = "error"
	FieldMetadata = "metadata"
	FieldRequest  = "request"
	FieldResponse = "response"
)

// Field represents a structured key-value attribute for logging.
type Field struct {
	Key   string
	Value any
}

// Err constructs a standardized Field with key "error".
// If err is nil, Value is nil and the adapter omits the field.
func Err(err error) Field {
	return Field{Key: FieldError, Value: err}
}

// Metadata constructs a standardized Field with key "metadata".
// The value can be an arbitrary struct, map, slice, or custom object.
func Metadata(v any) Field {
	return Field{Key: FieldMetadata, Value: v}
}

// Request constructs a standardized Field with key "request".
func Request(v any) Field {
	return Field{Key: FieldRequest, Value: v}
}

// Response constructs a standardized Field with key "response".
func Response(v any) Field {
	return Field{Key: FieldResponse, Value: v}
}

// Any constructs a custom Field with an explicit key and arbitrary value.
func Any(key string, val any) Field {
	return Field{Key: key, Value: val}
}
