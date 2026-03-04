package homer

import "testing"

func TestFSHomer(t *testing.T) {
	dir := t.TempDir()
	h, err := New(dir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	RunHomerSpec(t, h)
}
