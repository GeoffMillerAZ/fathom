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
	return decision.Question{ID: id, Kind: decision.KindNoul}
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
				{ID: "", Kind: decision.KindNoul},
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
			name: "score Levels with 11 entries is too many",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Levels: []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}},
			}},
			wantErr: true,
		},
		{
			name: "score Levels with 10 entries is valid (upper bound inclusive)",
			req: decision.Request{State: "s", Questions: []decision.Question{
				{ID: "q1", Kind: decision.KindScore, Levels: []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}},
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
