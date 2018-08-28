package future

import (
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// allow a future to respond an error in the future
type deferError struct {
	err       error
	errChan   chan error
	responded bool
}

func (d *deferError) Init() {
	d.errChan = make(chan error, 1)
}

func (d *deferError) Respond(err error) {
	if d.errChan == nil || d.responded {
		return
	}
	d.errChan <- err
	close(d.errChan)
	d.responded = true
}

func (d *deferError) Error() error {
	if d.err != nil {
		return d.err
	}
	if d.errChan == nil {
		panic("waiting for response on nil channel")
	}
	d.err = <-d.errChan
	return d.err
}

// future for adding transaction
type Tx struct {
	deferError
	Tx *ultpb.Tx
}

// future for adding new peer
type Peer struct {
	deferError
	Addr string
}
