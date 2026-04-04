package home

import "errors"

// ErrNotFound is returned by Get when the requested file does not exist.
var ErrNotFound = errors.New("file not found")

// Home abstracts file operations on a directory.
type Home interface {
	Get(name string) ([]byte, error)        // returns ErrNotFound if missing
	Search(pattern string) ([]string, error) // glob, returns bare filenames
	Upsert(name string, data []byte) error   // create or overwrite
	Delete(name string) error                // returns ErrNotFound if missing
}
