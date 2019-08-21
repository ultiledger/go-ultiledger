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
