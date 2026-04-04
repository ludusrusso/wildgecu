package file

import "errors"

// ErrNotFound is returned when the file does not exist.
var ErrNotFound = errors.New("file not found")

// ErrNotUnique is returned by Replace when old appears more than once.
var ErrNotUnique = errors.New("old string is not unique")

// ErrOldNotFound is returned by Replace when old is not found in the file.
var ErrOldNotFound = errors.New("old string not found")

// File abstracts text file operations on a single file path.
type File interface {
	Get() (string, error)              // returns ("", nil) when the file does not exist
	Write(content string) error        // creates or overwrites the file
	Replace(old, replacement string) error // errors if old is not found or appears more than once
}
