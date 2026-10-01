package decision_test

import (
	"encoding/json"
	"strings"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

// TestParseKindNoulAliasDecodesToPredicate pins DP-12: the generic yes/no
// kind is KindPredicate, "noul" is a decode alias for it (wire, config and
// stored rows). The former constant KindNoul is gone (DP-14); the alias
// lives on only as decoded text.
func TestParseKindNoulAliasDecodesToPredicate(t *testing.T) {
	tests := []struct {
		in      string
		want    decision.Kind
		wantErr bool
	}{
		{in: "choice", want: decision.KindChoice},
		{in: "score", want: decision.KindScore},
		{in: "predicate", want: decision.KindPredicate},
		{in: "noul", want: decision.KindPredicate},
		{in: "maybe", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		got, err := decision.ParseKind(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseKind(%q) = %q, nil; want an error", tt.in, got)
				continue
			}
			if !strings.Contains(err.Error(), `"`+tt.in+`"`) {
				t.Errorf("ParseKind(%q) error = %q; want it to name the value", tt.in, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("ParseKind(%q) = %q, %v; want %q, nil", tt.in, got, err, tt.want)
		}
	}

	if decision.KindPredicate != "predicate" {
		t.Errorf("KindPredicate = %q, want the canonical value %q", decision.KindPredicate, "predicate")
	}

	var q decision.Question
	if err := json.Unmarshal([]byte(`{"ID":"q1","Kind":"noul"}`), &q); err != nil {
		t.Fatalf("json.Unmarshal(Question kind noul) error = %v", err)
	}
	if q.Kind != decision.KindPredicate {
		t.Errorf("Question.Kind decoded from \"noul\" = %q, want %q", q.Kind, decision.KindPredicate)
	}

	var rec decision.Record
	if err := json.Unmarshal([]byte(`{"QuestionID":"q1","Kind":"noul","Noul":0.9}`), &rec); err != nil {
		t.Fatalf("json.Unmarshal(Record kind noul) error = %v", err)
	}
	if rec.Kind != decision.KindPredicate {
		t.Errorf("stored Record.Kind decoded from \"noul\" = %q, want %q", rec.Kind, decision.KindPredicate)
	}

	if err := json.Unmarshal([]byte(`{"ID":"q1","Kind":"maybe"}`), &q); err == nil || !strings.Contains(err.Error(), `"maybe"`) {
		t.Errorf("json.Unmarshal(Question kind maybe) error = %v; want one naming \"maybe\"", err)
	}
}

// TestKindUnmarshalTextRoundTripsZeroKind pins that an unset Kind survives a
// JSON round trip: the zero Kind marshals as "" and must decode back to the
// zero Kind rather than fail the whole document. Validate still rejects it.
func TestKindUnmarshalTextRoundTripsZeroKind(t *testing.T) {
	b, err := json.Marshal(decision.Answer{ID: "q1"})
	if err != nil {
		t.Fatal(err)
	}
	var a decision.Answer
	if err := json.Unmarshal(b, &a); err != nil {
		t.Fatalf("json round trip of a zero-Kind Answer error = %v", err)
	}
	if a.Kind != "" {
		t.Errorf("Kind = %q, want the zero Kind", a.Kind)
	}

	b, err = json.Marshal(decision.Answer{ID: "q1", Kind: decision.KindPredicate})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"Kind":"predicate"`) {
		t.Errorf("marshaled Answer = %s, want the canonical kind \"predicate\"", b)
	}
}
