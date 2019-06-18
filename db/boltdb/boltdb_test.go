package boltdb

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test validity of supplied key.
func TestDBOps(t *testing.T) {
	// open the database
	db := NewBoltDB("test.db")

	// create bucket
	err := db.NewBucket("TEST")
	assert.Equal(t, nil, err)

	// test get nonexistance key
	val, err := db.Get("TEST", []byte("none"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte(nil), val)

	// test set key/value pair
	err = db.Put("TEST", []byte("testKey"), []byte("testValue"))
	assert.Equal(t, nil, err)

	// test get value of key
	val, err = db.Get("TEST", []byte("testKey"))
	assert.Equal(t, err, nil)
	assert.Equal(t, []byte("testValue"), val)

	// remove the test db
	os.Remove("test.db")
}
