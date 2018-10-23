package op

import (
	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Trust manages create, update and delete of the
// trust to issued assets.
type Trust struct {
	AM    *account.Manager
	Asset *ultpb.Asset
	Limit uint64
}
