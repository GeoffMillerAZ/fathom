package rulereplay

import "testing"

func TestResolveSince(t *testing.T) {
	cases := []struct{ in, want string }{
		{"200", "last 200 commits"},
		{" 5 ", "last 5 commits"},
		{"0", "0..HEAD"},   // not positive — treated as a ref, not a count
		{"-3", "-3..HEAD"}, // negative — treated as a ref, not a count
		{"deadbeef", "deadbeef..HEAD"},
		{"v1.2.3", "v1.2.3..HEAD"},
		{"HEAD~10..HEAD", "HEAD~10..HEAD"}, // already a range — passed through
		{"a..b", "a..b"},
	}
	for _, tc := range cases {
		if got := ResolveSince(tc.in); got != tc.want {
			t.Errorf("ResolveSince(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
