package db

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

// Test database batch ops.
func TestDBBatch(t *testing.T) {
	// open the database
	db := NewBoltDB("test.db")

	// create bucket
	err := db.NewBucket("TEST1")
	assert.Equal(t, nil, err)

	err = db.NewBucket("TEST2")
	assert.Equal(t, nil, err)

	batch := db.NewBatch()
	batch.Put("TEST1", []byte("Hello"), []byte("World"))
	batch.Put("TEST2", []byte("Mona"), []byte("Lisa"))
	batch.Delete("TEST1", []byte("Hello"))
	batch.Put("TEST1", []byte("Go"), []byte("Cool"))
	err = batch.Write()
	assert.Equal(t, nil, err)

	val, err := db.Get("TEST1", []byte("Hello"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte(nil), val)

	val, err = db.Get("TEST2", []byte("Mona"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("Lisa"), val)

	val, err = db.Get("TEST1", []byte("Go"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("Cool"), val)

	// remove the test db
	os.Remove("test.db")
}
