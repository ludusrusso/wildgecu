package home

import "testing"

func TestFSHome(t *testing.T) {
	dir := t.TempDir()
	h, err := New(dir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	RunHomeSpec(t, h)
}
