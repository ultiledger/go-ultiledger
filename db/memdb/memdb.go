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

package memdb

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ultiledger/go-ultiledger/db"
)

type memdb struct {
	db map[string][]byte
	sync.RWMutex
}

// New creates a memory-based key-value store
// which is mainly used for testing.
func New() db.Database {
	return &memdb{db: make(map[string][]byte)}
}

func (m *memdb) NewBucket(name string) error {
	return nil
}

// Put writes the key/value pair to database.
func (m *memdb) Put(bucket string, key, value []byte) error {
	m.Lock()
	defer m.Unlock()

	if m.db == nil {
		return fmt.Errorf("Memdb is closed.")
	}

	k := bucket + "#" + string(key)

	m.db[k] = value
	return nil
}

// Delete deletes the key from the database.
func (m *memdb) Delete(bucket string, key []byte) error {
	m.Lock()
	defer m.Unlock()

	if m.db == nil {
		return fmt.Errorf("Memdb is closed.")
	}

	k := bucket + "#" + string(key)

	delete(m.db, k)
	return nil
}

// Get retrieves the value of the key from database.
func (m *memdb) Get(bucket string, key []byte) ([]byte, error) {
	m.Lock()
	defer m.Unlock()

	if m.db == nil {
		return nil, fmt.Errorf("Memdb is closed.")
	}

	k := bucket + "#" + string(key)

	if val, ok := m.db[k]; ok {
		return val, nil
	}
	return nil, nil
}

// GetAll retrieves the values of the keys with prefix from database.
func (m *memdb) GetAll(bucket string, keyPrefix []byte) ([][]byte, error) {
	m.Lock()
	defer m.Unlock()

	if m.db == nil {
		return nil, fmt.Errorf("Memdb is closed.")
	}

	prefix := bucket + "#" + string(keyPrefix)

	var vals [][]byte
	for k, v := range m.db {
		if strings.HasPrefix(k, prefix) {
			vals = append(vals, v)
		}
	}
	return vals, nil
}

// Close closes the underlying database.
func (m *memdb) Close() error {
	m.Lock()
	defer m.Unlock()

	m.db = nil
	return nil
}

// Placeholders for meeting the requirements of the db interface.
func (m *memdb) Begin() (db.Tx, error) {
	mtx := &memdbTx{mdb: m}
	return mtx, nil
}

// memdbTx mocks at the transaction of a real db.
type memdbTx struct{ mdb *memdb }

func (m *memdbTx) Get(bucket string, key []byte) ([]byte, error) {
	return m.mdb.Get(bucket, key)
}

func (m *memdbTx) GetAll(bucket string, keyPrefix []byte) ([][]byte, error) {
	return m.mdb.GetAll(bucket, keyPrefix)
}

func (m *memdbTx) Put(bucket string, key, value []byte) error {
	return m.mdb.Put(bucket, key, value)
}

func (m *memdbTx) Delete(bucket string, key []byte) error {
	return m.mdb.Delete(bucket, key)
}

func (m *memdbTx) Rollback() error {
	return nil
}

func (m *memdbTx) Commit() error {
	return nil
}
