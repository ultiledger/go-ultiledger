package memdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test Memdb.
func TestMemDB(t *testing.T) {
	// open the database
	db := New()

	// test get nonexistance key
	val, err := db.Get("TEST", []byte("none"))
	assert.NotEqual(t, nil, err)
	assert.Equal(t, []byte(nil), val)

	// test set key/value pair
	err = db.Put("TEST", []byte("testKey"), []byte("testValue"))
	assert.Equal(t, nil, err)

	// test get value of key
	val, err = db.Get("TEST", []byte("testKey"))
	assert.Equal(t, err, nil)
	assert.Equal(t, []byte("testValue"), val)
}
