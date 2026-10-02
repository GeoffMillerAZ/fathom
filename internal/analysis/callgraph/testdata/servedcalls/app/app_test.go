package app

import "testing"

func TestBuild(t *testing.T) {
	if Build() == nil {
		t.Fatal("nil")
	}
}

func TestSame(t *testing.T) {
	if !Same(nil, nil) {
		t.Fatal("not same")
	}
}
