package decision

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// limitsLevels returns n distinct level descriptions.
func limitsLevels(n int) []string {
	levels := make([]string, n)
	for i := range levels {
		levels[i] = strconv.Itoa(i)
	}
	return levels
}

// TestOutsideDeclaredLimits drives the generic "question outside the declared
// Capabilities" check directly: kinds, MaxOptions, and Min/MaxScoreLevels,
// counted from explicit Levels or an integral [Min, Max] range.
func TestOutsideDeclaredLimits(t *testing.T) {
	options := func(n int) []Option {
		opts := make([]Option, n)
		for i := range opts {
			opts[i] = Option{Name: strconv.Itoa(i)}
		}
		return opts
	}
	scoreLimits := Capabilities{Declared: true, Kinds: []Kind{KindScore}, MinScoreLevels: 2, MaxScoreLevels: 10}

	tests := []struct {
		name    string
		caps    Capabilities
		q       Question
		outside bool
		// reasonHas are substrings the refusal reason must carry.
		reasonHas []string
	}{
		{"undeclared capabilities declare no limit", Capabilities{}, Question{Kind: KindScore, Levels: limitsLevels(50)}, false, nil},
		{"declared, no limits set: any level count is inside", Capabilities{Declared: true, Kinds: []Kind{KindScore}}, Question{Kind: KindScore, Levels: limitsLevels(50)}, false, nil},

		{"kind not answered", Capabilities{Declared: true, Kinds: []Kind{KindChoice}}, Question{Kind: KindScore, Levels: limitsLevels(3)}, true, []string{"score", "not answered"}},
		{"declared with no kinds answers nothing", Capabilities{Declared: true}, Question{Kind: KindPredicate}, true, []string{"predicate", "not answered"}},
		{"raw noul alias is the predicate kind", Capabilities{Declared: true, Kinds: []Kind{KindPredicate}}, Question{Kind: Kind("noul")}, false, nil},
		{"declared noul alias covers the predicate kind", Capabilities{Declared: true, Kinds: []Kind{Kind("noul")}}, Question{Kind: KindPredicate}, false, nil},

		{"choice at MaxOptions is inside", Capabilities{Declared: true, Kinds: []Kind{KindChoice}, MaxOptions: 3}, Question{Kind: KindChoice, Options: options(3)}, false, nil},
		{"choice over MaxOptions", Capabilities{Declared: true, Kinds: []Kind{KindChoice}, MaxOptions: 3}, Question{Kind: KindChoice, Options: options(4)}, true, []string{"4 options", "3"}},
		{"MaxOptions 0 is no limit", Capabilities{Declared: true, Kinds: []Kind{KindChoice}}, Question{Kind: KindChoice, Options: options(300)}, false, nil},

		{"explicit 10 levels is inside", scoreLimits, Question{Kind: KindScore, Levels: limitsLevels(10)}, false, nil},
		{"explicit 11 levels", scoreLimits, Question{Kind: KindScore, Levels: limitsLevels(11)}, true, []string{"11 levels", "at most 10"}},
		{"explicit 20 levels", scoreLimits, Question{Kind: KindScore, Levels: limitsLevels(20)}, true, []string{"20 levels", "at most 10"}},
		{"explicit 2 levels is inside", scoreLimits, Question{Kind: KindScore, Levels: limitsLevels(2)}, false, nil},
		{"explicit 1 level is under the minimum", scoreLimits, Question{Kind: KindScore, Levels: limitsLevels(1)}, true, []string{"1 level", "at least 2"}},
		{"range 1..10 is 10 levels", scoreLimits, Question{Kind: KindScore, Min: 1, Max: 10}, false, nil},
		{"range 0..10 is 11 levels", scoreLimits, Question{Kind: KindScore, Min: 0, Max: 10}, true, []string{"11 levels", "at most 10"}},
		{"range -5..5 is 11 levels", scoreLimits, Question{Kind: KindScore, Min: -5, Max: 5}, true, []string{"11 levels"}},
		{"huge range counts as over the maximum", scoreLimits, Question{Kind: KindScore, Min: 0, Max: 1e30}, true, []string{"at most 10"}},
		{"infinite range counts as over the maximum", scoreLimits, Question{Kind: KindScore, Min: 0, Max: math.Inf(1)}, true, []string{"at most 10"}},
		{"non-integral range has no level count to judge", scoreLimits, Question{Kind: KindScore, Min: 0, Max: 1.5}, false, nil},
		{"only a minimum: 2 levels under a minimum of 3", Capabilities{Declared: true, Kinds: []Kind{KindScore}, MinScoreLevels: 3}, Question{Kind: KindScore, Levels: limitsLevels(2)}, true, []string{"at least 3"}},
		{"only a maximum: 2 levels is inside", Capabilities{Declared: true, Kinds: []Kind{KindScore}, MaxScoreLevels: 10}, Question{Kind: KindScore, Levels: limitsLevels(2)}, false, nil},

		{"level limits never apply to a predicate", Capabilities{Declared: true, Kinds: []Kind{KindPredicate}, MinScoreLevels: 2, MaxScoreLevels: 10}, Question{Kind: KindPredicate, Levels: limitsLevels(20)}, false, nil},
		{"option limits never apply to a score", Capabilities{Declared: true, Kinds: []Kind{KindScore}, MaxOptions: 2}, Question{Kind: KindScore, Options: options(9), Levels: limitsLevels(3)}, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, outside := outsideDeclaredLimits("acme", tt.caps, tt.q)
			if outside != tt.outside {
				t.Fatalf("outsideDeclaredLimits() outside = %v (reason %q), want %v", outside, reason, tt.outside)
			}
			if !outside {
				if reason != "" {
					t.Errorf("reason = %q, want empty for an in-limit question", reason)
				}
				return
			}
			if !strings.Contains(reason, "acme") {
				t.Errorf("reason %q should name the provider", reason)
			}
			for _, want := range tt.reasonHas {
				if !strings.Contains(reason, want) {
					t.Errorf("reason %q should contain %q", reason, want)
				}
			}
		})
	}
}
