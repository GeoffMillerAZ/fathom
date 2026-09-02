package a

import "testing"

func TestFoo(t *testing.T) {
	if Foo() != 1 {
		t.Fatal("foo")
	}
}
