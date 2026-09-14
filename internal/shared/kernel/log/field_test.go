package log_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

func TestFieldConstructors(t *testing.T) {
	t.Parallel()

	err := errors.New("sample error")
	fErr := log.Err(err)
	assert.Equal(t, log.FieldError, fErr.Key)
	assert.Equal(t, err, fErr.Value)

	fNilErr := log.Err(nil)
	assert.Equal(t, log.FieldError, fNilErr.Key)
	assert.Nil(t, fNilErr.Value)

	type dummyMeta struct {
		ID int
	}
	meta := dummyMeta{ID: 42}
	fMeta := log.Metadata(meta)
	assert.Equal(t, log.FieldMetadata, fMeta.Key)
	assert.Equal(t, meta, fMeta.Value)

	fReq := log.Request("req-data")
	assert.Equal(t, log.FieldRequest, fReq.Key)
	assert.Equal(t, "req-data", fReq.Value)

	fResp := log.Response("resp-data")
	assert.Equal(t, log.FieldResponse, fResp.Key)
	assert.Equal(t, "resp-data", fResp.Value)

	fAny := log.Any("custom_key", 123)
	assert.Equal(t, "custom_key", fAny.Key)
	assert.Equal(t, 123, fAny.Value)
}
