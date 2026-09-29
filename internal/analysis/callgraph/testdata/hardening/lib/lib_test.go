package lib

import "testing"

func TestLib(t *testing.T) {
	_ = Wired()
	_ = DeadViaBlank()
	_ = Render()
	_ = DeadViaUncalledMainHelper()
	_ = GenOnly()
	_ = TestOnly()
	_ = Stack{}.Len()
}
