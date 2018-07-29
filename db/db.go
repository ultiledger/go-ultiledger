package db

import (
	"fmt"
)

var constructors = make(map[string]DBCtor)

// generic database operation interface
type DB interface {
	Set(key []byte, val []byte) error
	Get(key []byte) ([]byte, bool)
	Close()
}

// database backend should call this function to register itself
// in order to be used by application
func Register(dbName string, ctor DBCtor) {
	constructors[dbName] = ctor
}

// create a new db in specified file path
type DBCtor func(string) DB

func GetDB(dbName string) (DBCtor, error) {
	if _, ok := constructors[dbName]; !ok {
		return nil, fmt.Errorf("database %s not registered", dbName)
	}
	return constructors[dbName], nil
}
