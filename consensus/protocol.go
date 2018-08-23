package consensus

import (
	"go.uber.org/zap"

	"github.com/ultiledger/go-ultiledger/db"
)

// Ultiledger Consensus Protocol
type ucp struct {
	store  db.DB
	bucket string

	logger *zap.SugaredLogger
}

func newUCP(d db.DB, l *zap.SugaredLogger) *ucp {
	u := &ucp{
		store:  d,
		logger: l,
		bucket: "UCP",
	}
	return u
}
