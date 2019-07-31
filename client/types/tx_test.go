package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTxStatusCode(t *testing.T) {
	statusCode := NotExist
	assert.Equal(t, "not exist", statusCode.String())
	statusCode = Rejected
	assert.Equal(t, "rejected", statusCode.String())
	statusCode = Accepted
	assert.Equal(t, "accepted", statusCode.String())
	statusCode = Confirmed
	assert.Equal(t, "confirmed", statusCode.String())
	statusCode = Failed
	assert.Equal(t, "failed", statusCode.String())
	statusCode = Unknown
	assert.Equal(t, "unknown", statusCode.String())
}
