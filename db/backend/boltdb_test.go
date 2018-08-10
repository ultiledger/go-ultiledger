package backend

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// test validity of supplied key
func TestDBOps(t *testing.T) {
	// open the database
	db := NewBoltDB("test.db")

	// create bucket
	err := db.CreateBucket("TEST")
	assert.Equal(t, nil, err)

	// test get nonexistance key
	val, ok := db.Get("TEST", []byte("none"))
	assert.Equal(t, false, ok)
	assert.Equal(t, []byte(nil), val)

	// test set key/value pair
	err = db.Set("TEST", []byte("testKey"), []byte("testValue"))
	assert.Equal(t, nil, err)

	// test get value of key
	val, ok = db.Get("TEST", []byte("testKey"))
	assert.Equal(t, true, ok)
	assert.Equal(t, []byte("testValue"), val)

	// remove the test db
	os.Remove("test.db")
}
