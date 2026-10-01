package decision_test

import (
	"fmt"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

func validChoiceQuestion(id string) decision.Question {
	return decision.Question{
		ID:   id,
		Kind: decision.KindChoice,
		Options: []decision.Option{
			{Name: "yes"},
			{Name: "no"},
		},
	}
}

func validScoreQuestion(id string) decision.Question {
	return decision.Question{ID: id, Kind: decision.KindScore, Min: 0, Max: 10}
}

func validNoulQuestion(id string) decision.Question {
	return decision.Question{ID: id, Kind: decision.KindPredicate}
}

// scoreLevels returns n distinct level descriptions "0".."n-1".
func scoreLevels(n int) []string {
	levels := make([]string, n)
	for i := range levels {
		levels[i] = fmt.Sprintf("%d", i)
	}
	return levels
}

func TestValidate(t *testing.T) {
	manyOptions := func(n int) []decision.Option {
		opts := make([]decision.Option, n)
		for i := range opts {
			opts[i] = decision.Option{Name: fmt.Sprintf("opt%d", i)}
		}
		return opts
	}

	tests := []struct {
		name    string
		req     decision.Request
		wantErr bool
	}{
		{
			name: "valid mixed batch",
			req: decision.Request{
				State: "some state",
				Questions: []decision.Question{
					validChoiceQuestion("q1"),
					validScoreQuestion("q2"),
					validNoulQuestion("q3"),
				},
			},
			wantErr: false,
		},
		{
			name: "raw noul kind built without ParseKind is a predicate",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.Kind("noul")},
			}},
			wantErr: false,
		},
		{
			name:    "empty questions is valid (degenerate but harmless)",
			req:     decision.Request{State: "some state"},
			wantErr: false,
		},
		{
			name:    "empty state",
			req:     decision.Request{State: "", Questions: []decision.Question{validNoulQuestion("q1")}},
			wantErr: true,
		},
		{
			name: "empty question id",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "", Kind: decision.KindPredicate},
			}},
			wantErr: true,
		},
		{
			name: "duplicate question id",
			req: decision.Request{State: "s", Questions: []decision.Question{
				validNoulQuestion("dup"),
				validNoulQuestion("dup"),
			}},
			wantErr: true,
		},
		{
			name: "unknown kind",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.Kind("maybe")},
			}},
			wantErr: true,
		},
		{
			name: "choice with 1 option",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindChoice, Options: []decision.Option{{Name: "only"}}},
			}},
			wantErr: true,
		},
		{
			name: "choice with 0 options",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindChoice},
			}},
			wantErr: true,
		},
		{
			name: "choice with 255 options is valid (upper bound inclusive)",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindChoice, Options: manyOptions(255)},
			}},
			wantErr: false,
		},
		{
			name: "choice with 256 options exceeds the bound",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindChoice, Options: manyOptions(256)},
			}},
			wantErr: true,
		},
		{
			name: "choice with duplicate option names",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindChoice, Options: []decision.Option{{Name: "a"}, {Name: "a"}}},
			}},
			wantErr: true,
		},
		{
			name: "choice with empty option name",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindChoice, Options: []decision.Option{{Name: "a"}, {Name: ""}}},
			}},
			wantErr: true,
		},
		{
			name: "score Max equal to Min",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Min: 5, Max: 5},
			}},
			wantErr: true,
		},
		{
			name: "score Max less than Min",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Min: 10, Max: 0},
			}},
			wantErr: true,
		},
		{
			name: "score Max greater than Min",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Min: 0, Max: 1},
			}},
			wantErr: false,
		},
		{
			name: "noul has no additional constraints",
			req: decision.Request{State: "s", Questions: []decision.Question{
				validNoulQuestion("q1"),
			}},
			wantErr: false,
		},
		{
			name: "score Levels with 1 entry is too few",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Levels: []string{"only"}},
			}},
			wantErr: true,
		},
		{
			name: "score Levels with 11 entries is valid (no vendor level cap in the core; see DP-13)",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Levels: scoreLevels(11)},
			}},
			wantErr: false,
		},
		{
			name: "score Levels with 10 entries is valid",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Levels: scoreLevels(10)},
			}},
			wantErr: false,
		},
		{
			name: "score Levels with Min/Max unset is valid",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Levels: []string{"lo", "hi"}},
			}},
			wantErr: false,
		},
		{
			name: "score Levels with Min/Max matching len(Levels)-1 is valid",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Levels: []string{"lo", "mid", "hi"}, Min: 0, Max: 2},
			}},
			wantErr: false,
		},
		{
			name: "score Levels with Max not matching len(Levels)-1 is invalid",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Levels: []string{"lo", "mid", "hi"}, Min: 0, Max: 10},
			}},
			wantErr: true,
		},
		{
			name: "score Levels with nonzero Min is invalid",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Levels: []string{"lo", "hi"}, Min: 1, Max: 1},
			}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := decision.Validate(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateHasNoVendorScoreLevelLimit pins DP-13: Score's 2..10 levels is
// TypeSafe's vendor limit, declared by that adapter (Capabilities) and
// enforced there, not a rule of the fast-decision contract. Validate accepts
// any ordered list of at least two levels; what stays a domain rule is the
// structure — Min/Max must match Levels, and one level is not an ordered
// score (the range form likewise needs Max > Min).
func TestValidateHasNoVendorScoreLevelLimit(t *testing.T) {
	score := func(levels []string, min, max float64) decision.Request {
		return decision.Request{State: "s", Questions: []decision.Question{
			{ID: "q1", Kind: decision.KindScore, Levels: levels, Min: min, Max: max},
		}}
	}

	valid := []struct {
		name string
		req  decision.Request
	}{
		{"11 levels, Min/Max unset", score(scoreLevels(11), 0, 0)},
		{"11 levels, Min/Max matching (0,10)", score(scoreLevels(11), 0, 10)},
		{"20 levels, Min/Max unset", score(scoreLevels(20), 0, 0)},
		{"20 levels, Min/Max matching (0,19)", score(scoreLevels(20), 0, 19)},
		{"1000 levels", score(scoreLevels(1000), 0, 0)},
		{"2 levels (the smallest ordered score)", score(scoreLevels(2), 0, 0)},
	}
	for _, tt := range valid {
		t.Run("valid/"+tt.name, func(t *testing.T) {
			if err := decision.Validate(tt.req); err != nil {
				t.Errorf("Validate() error = %v, want nil (the core has no level cap)", err)
			}
		})
	}

	// The Min/Max-must-match-Levels rule is structural and stays, at any size.
	invalid := []struct {
		name string
		req  decision.Request
	}{
		{"11 levels, Max not len-1", score(scoreLevels(11), 0, 9)},
		{"20 levels, Max not len-1", score(scoreLevels(20), 0, 10)},
		{"20 levels, nonzero Min", score(scoreLevels(20), 1, 19)},
		// One level is not an ordered score: the range form needs Max > Min for
		// the same reason. This is a domain rule, not a vendor limit, so it
		// stays in Validate.
		{"1 level", score(scoreLevels(1), 0, 0)},
	}
	for _, tt := range invalid {
		t.Run("invalid/"+tt.name, func(t *testing.T) {
			if err := decision.Validate(tt.req); err == nil {
				t.Error("Validate() error = nil, want non-nil")
			}
		})
	}
}
