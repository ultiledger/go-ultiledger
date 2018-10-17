package db

// Putter wraps the database write operation.
type Putter interface {
	Put(bucket string, key []byte, value []byte) error
}

// Deleter wraps the database delete operation.
type Deleter interface {
	Delete(bucket string, key []byte) error
}

// Generic database operation interface.
type Database interface {
	Putter
	Deleter
	Get(bucket string, key []byte) ([]byte, error)
	Close()
	NewBatch() Batch
	NewBucket(bucket string) error
}

// Batch combines multiple updates and writes them to database.
type Batch interface {
	Putter
	Deleter
	ValueSize() int
	Write() error
	Reset()
}
