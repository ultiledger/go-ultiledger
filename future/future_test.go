package future

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTxFuture(t *testing.T) {
	txf := Tx{}
	// test respond without Init will panic
	assert.Panics(t, func() { txf.Error() })
	// test error response
	txf.Init()
	txf.Respond(errors.New("tx error"))
	assert.Error(t, txf.Error())
}

func TestPeerFuture(t *testing.T) {
	pf := Peer{}
	// test respond without Init will panic
	assert.Panics(t, func() { pf.Error() })
	// test nil response
	pf.Init()
	pf.Respond(nil)
	assert.NoError(t, pf.Error())
}

func TestStatementFuture(t *testing.T) {
	sf := Statement{}
	// test respond without Init will panic
	assert.Panics(t, func() { sf.Error() })
	// test error response
	sf.Init()
	sf.Respond(errors.New("statement error"))
	assert.Error(t, sf.Error())
	// test reuse the same future will have no effect,
	// we still will get the first error
	sf.Respond(errors.New("another statement error"))
	assert.Error(t, sf.Error())
	assert.Equal(t, "statement error", sf.Error().Error())
}
