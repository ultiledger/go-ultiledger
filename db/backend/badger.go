package backend

import (
	// "github.com/dgraph-io/badger"

	"github.com/ultiledger/go-ultiledger/db"
)

func init() {
	db.Register("badger", NewBadger)
}

type badgerWrapper struct{}

func NewBadger(path string) db.DB {
	return &badgerWrapper{}
}

// write key-value to db
func (bw *badgerWrapper) Set(key []byte, val []byte) error {
	return nil
}

// get value of the key from db
func (bw *badgerWrapper) Get(key []byte) ([]byte, bool) {
	return nil, false
}

func (bw *badgerWrapper) Close() {}
