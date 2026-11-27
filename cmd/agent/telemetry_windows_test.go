package main

import "testing"

func TestPercentage(t *testing.T) {
	if percentage(1, 0) != nil || percentage(2, 1) != nil {
		t.Fatal("invalid counter accepted")
	}
	if v := percentage(0, 10); v == nil || *v != 0 {
		t.Fatal("zero lost")
	}
	if v := percentage(3, 8); v == nil || *v != 38 {
		t.Fatal("rounding failed")
	}
}
