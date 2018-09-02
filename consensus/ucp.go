package consensus

import (
	"github.com/ultiledger/go-ultiledger/db"
)

// Ultiledger Consensus Protocol
type UCP struct {
	store  db.DB
	bucket string

	decrees map[uint64]*Decree

	// previous proposal
	prev []byte
}

func NewUCP(d db.DB) *UCP {
	u := &UCP{
		store:  d,
		bucket: "UCP",
	}
	return u
}

func (u *UCP) Propose(index uint64, value []byte) error {
	return nil
}
