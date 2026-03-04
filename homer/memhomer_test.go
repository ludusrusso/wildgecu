package homer

import "testing"

func TestMemHomer(t *testing.T) {
	h := NewMem()
	RunHomerSpec(t, h)
}
