package connector

import "testing"

func TestClaimedModelContainer(t *testing.T) {
	rules := []DialectRule{{ID: "m", Extensions: []string{".slx", ".mdl"}, Containers: []string{".slx"}}}
	zipHead := []byte("PK\x03\x04\x14\x00\x00\x00")
	cases := []struct {
		path string
		data []byte
		want bool
	}{
		{"a/model.slx", zipHead, true},
		{"a/MODEL.SLX", zipHead, true},
		{"a/model.mdl", zipHead, false},                // not a container extension
		{"a/model.slx", []byte("\x00\x01\x02"), false}, // not a zip package
		{"a/archive.zip", zipHead, false},              // no dialect claims it
	}
	for _, c := range cases {
		if got := ClaimedModelContainer(rules, c.path, c.data); got != c.want {
			t.Errorf("ClaimedModelContainer(%s, %q) = %v, want %v", c.path, c.data, got, c.want)
		}
	}
	if ClaimedModelContainer(nil, "a/model.slx", zipHead) {
		t.Error("with no rules nothing is claimed")
	}
}
