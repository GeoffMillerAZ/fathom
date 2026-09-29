package iface

import "testing"

func TestFake(t *testing.T) {
	var d Doer = Fake{}
	_ = d.Do()
	_ = FakeOnly()
}
