package op

import "github.com/ultiledger/go-ultiledger/db"

// Op represents the interface with which various transaction
// operations should comply with.
type Op interface {
	Apply(dt db.Tx) error
}
