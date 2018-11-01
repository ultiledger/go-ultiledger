package db

// Getter wraps the database read operation.
type Getter interface {
	Get(bucket string, key []byte) ([]byte, error)
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
	Close()
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
