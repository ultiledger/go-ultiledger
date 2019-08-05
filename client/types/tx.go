package types

import (
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// TxStatusCode represents the status of a tx in node.
// Each tx will first go through some preliminary checks
// which we call them admission checks. Only after the tx
// passes the admission checks that it will be sent to
// internal tx manager for later processing.
type TxStatusCode uint8

const (
	// The tx does not exist.
	NotExist TxStatusCode = iota
	// The tx failed to pass the admission checks.
	Rejected
	// The tx has passed the admission checks and has been
	// added to the internal tx manager.
	Accepted
	// The tx has been applied successfully.
	Confirmed
	// The tx is failed to be applied for some errors.
	Failed
	// The tx is failed because of some internal errors.
	Unknown
)

func (ts TxStatusCode) String() string {
	switch ts {
	case NotExist:
		return "not exist"
	case Rejected:
		return "rejected"
	case Accepted:
		return "accepted"
	case Confirmed:
		return "confirmed"
	case Failed:
		return "failed"
	case Unknown:
		return "unknown"
	}
	return ""
}

// TxStatus represents the status of current tx in node.
type TxStatus struct {
	StatusCode   TxStatusCode
	ErrorMessage string
	Tx           *ultpb.Tx
}
