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

	// test get nonexistance key
	val, ok := db.Get([]byte("none"))
	assert.Equal(t, false, ok)
	assert.Equal(t, []byte(nil), val)

	// test set key/value pair
	err := db.Set([]byte("testKey"), []byte("testValue"))
	assert.Equal(t, nil, err)

	// test get value of key
	val, ok = db.Get([]byte("testKey"))
	assert.Equal(t, true, ok)
	assert.Equal(t, []byte("testValue"), val)

	// remove the test db
	os.Remove("test.db")
}
