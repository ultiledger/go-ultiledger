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

// Package db defines the interfaces for using the underlying database.
package db

// Getter wraps the database read operation.
type Getter interface {
	// Get the value of key in bucket.
	Get(bucket string, key []byte) ([]byte, error)
	// Get all values of keys with the prefix in bucket.
	GetAll(bucket string, keyPrefix []byte) ([][]byte, error)
}

// Putter wraps the database write operation.
type Putter interface {
	Put(bucket string, key []byte, value []byte) error
}

// Deleter wraps the database delete operation.
type Deleter interface {
	Delete(bucket string, key []byte) error
}

// Generic database operations interface.
type Database interface {
	Getter
	Putter
	Deleter
	Close() error
	Begin() (Tx, error)
	NewBucket(bucket string) error
}

// Generic database transaction operations interface.
type Tx interface {
	Getter
	Putter
	Deleter
	Rollback() error
	Commit() error
}
