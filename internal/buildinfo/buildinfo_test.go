package buildinfo

import "testing"

func TestVersion(t *testing.T) {
	v := Version()
	if v == "" {
		t.Fatal("Version must never be empty — 'unknown' is the honest floor (law 3)")
	}
	if v2 := Version(); v2 != v {
		t.Errorf("Version not stable: %q then %q", v, v2)
	}
}
