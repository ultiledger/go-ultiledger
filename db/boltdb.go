package db

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/boltdb/bolt"
)

type boltdb struct {
	db *bolt.DB
}

// NewBoltDB creates a new boltdb instance which can be used by multiple
// goroutines of the same process, BoltDB obtains a file lock on the data
// file so multiple processes cannot open the same database at the same time.
// It will panic if the database cannot be created or opened.
func NewBoltDB(path string) Database {
	// open a database in specified path
	bt, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	return &boltdb{db: bt}
}

func (bt *boltdb) NewBucket(name string) error {
	if bt.db == nil {
		return errors.New("database is not initialized")
	}
	if name == "" {
		return errors.New("database bucket name is empty")
	}

	if err := bt.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(name))
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// Put writes the key/value pair to database.
func (bt *boltdb) Put(bucket string, key, value []byte) error {
	if bt.db == nil {
		return errors.New("database is not initialized")
	}

	if err := bt.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		err := b.Put(key, value)
		return err
	}); err != nil {
		return err
	}
	return nil
}

// Delete deletes the key from the database.
func (bt *boltdb) Delete(bucket string, key []byte) error {
	if bt.db == nil {
		return errors.New("database is not initialized")
	}

	if err := bt.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		err := b.Delete(key)
		return err
	}); err != nil {
		return err
	}
	return nil
}

// Get retrieves the value of the key from database.
func (bt *boltdb) Get(bucket string, key []byte) ([]byte, error) {
	if bt.db == nil {
		return nil, errors.New("database is not initialized")
	}

	var val []byte
	if err := bt.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		val = b.Get(key)
		return nil
	}); err != nil {
		return nil, err
	}
	return val, nil
}

// Close closes the underlying database.
func (bt *boltdb) Close() {
	if bt.db != nil {
		bt.db.Close()
	}
}

// NewBatch creates a Batch object which can be used for batch writes.
func (bt *boltdb) NewBatch() Batch {
	return &boltdbBatch{db: bt.db}
}

type kv struct {
	bucket string
	key    []byte
	value  []byte
	delete bool
}

type boltdbBatch struct {
	db   *bolt.DB
	kvs  []*kv
	size int
}

func (bb *boltdbBatch) Put(bucket string, key, value []byte) error {
	if bucket == "" {
		return errors.New("bucket is empty")
	}
	if len(key) == 0 {
		return errors.New("key is empty")
	}
	bb.kvs = append(bb.kvs, &kv{
		bucket: bucket,
		key:    key,
		value:  value,
		delete: false,
	})
	bb.size += len(value)
	return nil
}

func (bb *boltdbBatch) Delete(bucket string, key []byte) error {
	bb.kvs = append(bb.kvs, &kv{
		bucket: bucket,
		key:    key,
		delete: true,
	})
	bb.size += 1
	return nil
}

func (bb *boltdbBatch) Write() error {
	if len(bb.kvs) == 0 {
		return nil
	}

	if err := bb.db.Batch(func(tx *bolt.Tx) error {
		for _, kv := range bb.kvs {
			b := tx.Bucket([]byte(kv.bucket))
			if b == nil {
				return fmt.Errorf("bucket %s not exist", kv.bucket)
			}
			if kv.delete {
				err := b.Delete(kv.key)
				if err != nil {
					return fmt.Errorf("delete key %s failed: %v", string(kv.key), err)
				}
				continue
			}
			err := b.Put(kv.key, kv.value)
			if err != nil {
				return fmt.Errorf("put key %s failed: %v", string(kv.key), err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (bb *boltdbBatch) ValueSize() int {
	return bb.size
}

func (bb *boltdbBatch) Reset() {
	bb.kvs = nil
	bb.size = 0
}
