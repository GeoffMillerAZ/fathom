package decide

// caps_internal_test.go — KS-10's deadline rule on the function alone (no
// hook-path site asks by default, caps.go): a hook-path ask's deadline is
// its ceiling less its joins' measured time, never a constant past the
// ceiling.

import (
	"testing"
	"time"
)

func TestAskDeadlineIsTheCeilingLessItsJoins(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	ceiling := start.Add(600 * time.Millisecond)
	for _, tc := range []struct {
		name        string
		joins       time.Duration
		siteTimeout time.Duration
		want        time.Duration
		ok          bool
	}{
		{"what the joins left", 450 * time.Millisecond, 0, 150 * time.Millisecond, true},
		{"never the site's constant past the ceiling", 450 * time.Millisecond, 1500 * time.Millisecond, 150 * time.Millisecond, true},
		{"the site's constant when it is shorter", 100 * time.Millisecond, 200 * time.Millisecond, 200 * time.Millisecond, true},
		{"joins that spent the ceiling ask nothing", 600 * time.Millisecond, 1500 * time.Millisecond, 0, false},
		{"joins past the ceiling ask nothing", 700 * time.Millisecond, 0, 0, false},
	} {
		got, ok := askDeadline(ceiling, start.Add(tc.joins), tc.siteTimeout)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: askDeadline = %v, %v; want %v, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}
