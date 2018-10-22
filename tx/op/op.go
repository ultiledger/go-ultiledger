package op

import "github.com/ultiledger/go-ultiledger/db"

// Op represents the interface with which various transaction
// operations should comply with. Validate check the validity
// of the operation defined and Apply applies the operations.
type Op interface {
	Apply(dt db.Tx) error
}
