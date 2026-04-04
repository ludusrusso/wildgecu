package home

import "testing"

// NewTmpHome returns a Home backed by a temporary directory for testing.
func NewTmpHome(t testing.TB) *Home {
	t.Helper()
	h, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("NewTmpHome: %v", err)
	}
	return h
}
