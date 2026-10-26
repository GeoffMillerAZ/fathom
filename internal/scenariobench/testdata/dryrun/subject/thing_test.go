package thing

import "testing"

func TestStable(t *testing.T) {
	if Add(0, 0) != 0 {
		t.Fatal("zero sum changed")
	}
}
