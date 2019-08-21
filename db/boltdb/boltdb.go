// Copyright 2019 The go-ultiledger Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package boltdb

import (
	"bytes"
	"log"
	"time"

	"github.com/boltdb/bolt"

	"github.com/ultiledger/go-ultiledger/db"
)

type boltdb struct {
	db *bolt.DB
}

// New creates a new boltdb instance which can be used by multiple
// goroutines of the same process, BoltDB obtains a file lock on the data
// file so multiple processes cannot open the same database at the same time.
// It will panic if the database cannot be created or opened.
func New(path string) db.Database {
	// open a database in specified path
	bt, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	return &boltdb{db: bt}
}

func (bt *boltdb) NewBucket(name string) error {
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

// Get retrieves the values of the keys with prefix from database.
func (bt *boltdb) GetAll(bucket string, keyPrefix []byte) ([][]byte, error) {
	var vals [][]byte
	if err := bt.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(bucket)).Cursor()
		for k, v := c.Seek(keyPrefix); k != nil && bytes.HasPrefix(k, keyPrefix); k, v = c.Next() {
			vals = append(vals, v)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return vals, nil
}

// Close closes the underlying database.
func (bt *boltdb) Close() error {
	if bt.db != nil {
		bt.db.Close()
	}
	return nil
}

// Begin returns a writable database transaction object
// which can be used to manually managing transaction.
func (bt *boltdb) Begin() (db.Tx, error) {
	tx, err := bt.db.Begin(true)
	if err != nil {
		return nil, err
	}
	btx := &boltdbTx{tx: tx}
	return btx, nil
}

// boltdbTx wraps the boltdb transaction to provide the desired interface.
type boltdbTx struct {
	tx *bolt.Tx
}

func (btx *boltdbTx) Get(bucket string, key []byte) ([]byte, error) {
	b := btx.tx.Bucket([]byte(bucket))
	v := b.Get(key)
	return v, nil
}

func (btx *boltdbTx) GetAll(bucket string, keyPrefix []byte) ([][]byte, error) {
	var vals [][]byte
	c := btx.tx.Bucket([]byte(bucket)).Cursor()
	for k, v := c.Seek(keyPrefix); k != nil && bytes.HasPrefix(k, keyPrefix); k, v = c.Next() {
		vals = append(vals, v)
	}
	return vals, nil
}

func (btx *boltdbTx) Put(bucket string, key, value []byte) error {
	b := btx.tx.Bucket([]byte(bucket))
	err := b.Put(key, value)
	if err != nil {
		return err
	}
	return nil
}

func (btx *boltdbTx) Delete(bucket string, key []byte) error {
	b := btx.tx.Bucket([]byte(bucket))
	err := b.Delete(key)
	if err != nil {
		return err
	}
	return nil
}

func (btx *boltdbTx) Rollback() error {
	return btx.tx.Rollback()
}

func (btx *boltdbTx) Commit() error {
	return btx.tx.Commit()
}
