package home

import "testing"

func TestMemHome(t *testing.T) {
	h := NewMem()
	RunHomeSpec(t, h)
}
