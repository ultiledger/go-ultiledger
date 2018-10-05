package backend

import (
	"errors"
	"log"
	"time"

	"github.com/boltdb/bolt"

	"github.com/ultiledger/go-ultiledger/db"
)

func init() {
	db.Register("boltdb", NewBoltDB)
}

type boltdbWrapper struct {
	internalDB *bolt.DB
}

// NewBoltDB creates a new boltdb instance which can be used by multiple
// goroutines of the same process, BoltDB obtains a file lock on the data
// file so multiple processes cannot open the same database at the same time.
// It will panic if the database cannot be created or opened.
func NewBoltDB(path string) db.DB {
	// open a database in specified path
	b, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	return &boltdbWrapper{internalDB: b}
}

func (bw *boltdbWrapper) CreateBucket(name string) error {
	if bw.internalDB == nil {
		log.Fatal(errors.New("database is not initialized"))
	}
	if name == "" {
		log.Fatal(errors.New("database bucket name is empty"))
	}

	bw.internalDB.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(name))
		if err != nil {
			return err
		}
		return nil
	})
	return nil
}

// Set writes the key/value pairs to database.
func (bw *boltdbWrapper) Set(bucket string, key []byte, val []byte) error {
	if bw.internalDB == nil {
		log.Fatal(errors.New("database is not initialized"))
	}

	bw.internalDB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		err := b.Put(key, val)
		return err
	})
	return nil
}

// Get retrieves the value of the key from database.
func (bw *boltdbWrapper) Get(bucket string, key []byte) ([]byte, bool) {
	if bw.internalDB == nil {
		log.Fatal(errors.New("database is not initialized"))
	}

	var val []byte
	bw.internalDB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		val = b.Get(key)
		return nil
	})
	if val != nil {
		return val, true
	}
	return val, false
}

// Close closes the underlying database.
func (bw *boltdbWrapper) Close() {
	if bw.internalDB != nil {
		bw.internalDB.Close()
	}
}
