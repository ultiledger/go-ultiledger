package consensus

import (
	"github.com/ultiledger/go-ultiledger/db"
)

// Ultiledger Consensus Protocol
type ucp struct {
	store  db.DB
	bucket string
}

func newUCP(d db.DB) *ucp {
	u := &ucp{
		store:  d,
		bucket: "UCP",
	}
	return u
}
