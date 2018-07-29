package backend

import (
	// "github.com/golang/leveldb"

	"github.com/ultiledger/go-ultiledger/db"
)

func init() {
	db.Register("leveldb", NewLevelDB)
}

type leveldbWrapper struct{}

func NewLevelDB(path string) db.DB {
	return &leveldbWrapper{}
}

// write key-value to db
func (lw *leveldbWrapper) Set(key []byte, val []byte) error {
	return nil
}

// get value of the key from db
func (lw *leveldbWrapper) Get(key []byte) ([]byte, bool) {
	return nil, false
}

func (lw *leveldbWrapper) Close() {}
