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

func TestReadIsStableAndConsistentWithVersion(t *testing.T) {
	s := Read()
	if s2 := Read(); s2 != s {
		t.Errorf("Read not stable: %+v then %+v", s, s2)
	}
	v := Version()
	if s.Revision == "" && v != "unknown" {
		t.Errorf("no revision stamped but Version() = %q, want unknown", v)
	}
	if s.Revision != "" && !SameRevision(v, s.Revision) {
		t.Errorf("Version() %q does not name Read().Revision %q", v, s.Revision)
	}
}

func TestSameRevision(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"", "", false},
		{"abc", "", false},
		{"6c6f4377b53a", "6c6f4377b53aad47cf6ab86224721df8592cadf0", true},
		{"6c6f4377b53a-dirty", "6c6f4377b53aad47cf6ab86224721df8592cadf0", true},
		{"3f0c3b1", "6c6f4377b53a", false},
		{"6c6f4377b53a", "6c6f4377b53a", true},
	}
	for _, c := range cases {
		if got := SameRevision(c.a, c.b); got != c.want {
			t.Errorf("SameRevision(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
