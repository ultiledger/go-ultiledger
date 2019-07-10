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

	m.db[string(key)] = value
	return nil
}

// Delete deletes the key from the database.
func (m *memdb) Delete(bucket string, key []byte) error {
	m.Lock()
	defer m.Unlock()

	if m.db == nil {
		return fmt.Errorf("Memdb is closed.")
	}

	delete(m.db, string(key))
	return nil
}

// Get retrieves the value of the key from database.
func (m *memdb) Get(bucket string, key []byte) ([]byte, error) {
	m.Lock()
	defer m.Unlock()

	if m.db == nil {
		return nil, fmt.Errorf("Memdb is closed.")
	}

	if val, ok := m.db[string(key)]; ok {
		return val, nil
	}
	return nil, nil
}

// Get retrieves the values of the keys with prefix from database.
func (m *memdb) GetAll(bucket string, keyPrefix []byte) ([][]byte, error) {
	m.Lock()
	defer m.Unlock()

	if m.db == nil {
		return nil, fmt.Errorf("Memdb is closed.")
	}

	var vals [][]byte
	for k, v := range m.db {
		if strings.HasPrefix(k, string(keyPrefix)) {
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

// memdbTx mocks at the transactions of real db.
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
