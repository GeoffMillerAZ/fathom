package decide_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

type recalRow struct {
	Site             string `json:"site"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	QuestionID       string `json:"question_id"`
	Error            string `json:"error"`
	RecalibrationDue bool   `json:"recalibration_due"`
}

func recalRows(t *testing.T, buf *bytes.Buffer) []recalRow {
	t.Helper()
	var out []recalRow
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var r recalRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("row %q: %v", line, err)
		}
		out = append(out, r)
	}
	buf.Reset()
	return out
}

func answered(provider, model string, ids ...string) decision.Response {
	resp := decision.Response{Provider: provider, Model: model, Answers: map[string]decision.Answer{}}
	for _, id := range ids {
		resp.Answers[id] = decision.Answer{ID: id, Kind: decision.KindPredicate, Noul: 0.95, Confidence: 0.95}
	}
	return resp
}

// TestRecorderMarksRecalibrationDueOnModelMove pins JF-10
// (docs/design/judgment-foundations.md R8): every decisions.jsonl row
// carries the resolved model (resp.Model), and the first row a site logs
// under a model (or provider) other than the one it last logged is marked
// "recalibration_due": a calibration reading taken under the old model no
// longer speaks for the site. The comparison lives in the Recorder,
// seeded from the existing log's tail; no table, memo or background job
// (R5). A failed call (no model) neither marks nor moves anything.
func TestRecorderMarksRecalibrationDueOnModelMove(t *testing.T) {
	var buf bytes.Buffer
	rec := decide.NewRecorder(&buf, func() time.Time { return time.Unix(0, 0).UTC() }, "test")
	always := func(decision.Answer) bool { return true }

	rec.Record("route", answered("typesafe", "jev-1.13.0", "a", "b"), always)
	for _, r := range recalRows(t, &buf) {
		if r.Model != "jev-1.13.0" {
			t.Errorf("row %s model = %q, want the resolved model jev-1.13.0", r.QuestionID, r.Model)
		}
		if r.RecalibrationDue {
			t.Errorf("row %s marked due on the site's first logged model, want unmarked (nothing to recalibrate from)", r.QuestionID)
		}
	}

	rec.Record("route", answered("typesafe", "jev-1.13.0", "a"), always)
	rec.Record("certify_suggest", answered("typesafe", "jev-1.14.0", "x"), always)
	for _, r := range recalRows(t, &buf) {
		if r.RecalibrationDue {
			t.Errorf("%s row marked due with no move of its own site's model", r.Site)
		}
	}

	rec.RecordError("route", "typesafe", "", 1, time.Millisecond, errors.New("typesafe: status 429"))
	if rows := recalRows(t, &buf); len(rows) != 1 || rows[0].RecalibrationDue {
		t.Fatalf("a failed call's row = %+v, want one unmarked row", rows)
	}

	rec.Record("route", answered("typesafe", "jev-1.14.0", "a", "b"), always)
	rows := recalRows(t, &buf)
	if len(rows) != 2 || !rows[0].RecalibrationDue || rows[1].RecalibrationDue {
		t.Fatalf("after route moved to jev-1.14.0: %+v, want the first row marked due and only the first", rows)
	}
	rec.Record("route", answered("typesafe", "jev-1.14.0", "a"), always)
	if rows := recalRows(t, &buf); rows[0].RecalibrationDue {
		t.Error("route still marked due after it logged the new model once")
	}

	rec.Record("route", answered("llm", "jev-1.14.0", "a"), always)
	if rows := recalRows(t, &buf); !rows[0].RecalibrationDue {
		t.Error("a provider move kept the site unmarked, want due (the reading's key moved)")
	}

	rec.Append([]decision.Record{{Site: "obligations", Provider: "typesafe", Model: "jev-1.13.0", QuestionID: "r1", Kind: decision.KindPredicate, Confidence: 0.95}})
	rec.Append([]decision.Record{{Site: "obligations", Provider: "typesafe", Model: "jev-1.14.0", QuestionID: "r1", Kind: decision.KindPredicate, Confidence: 0.95}})
	if rows := recalRows(t, &buf); len(rows) != 2 || rows[0].RecalibrationDue || !rows[1].RecalibrationDue {
		t.Errorf("Append rows = %+v, want the move marked on the second", rows)
	}

	t.Run("seeded from the existing log's tail", func(t *testing.T) {
		tail := strings.Join([]string{
			`{"schema_version":1,"site":"route","provider":"typesafe","model":"jev-1.12.0","kind":"noul"}`,
			`{"schema_version":2,"site":"route","provider":"typesafe","model":"jev-1.13.0","kind":"choice"}`,
			`{"schema_version":2,"site":"route","provider":"typesafe","model":"","error":"timeout","questions":1}`,
			`not json`,
		}, "\n") + "\n"
		var out bytes.Buffer
		seeded := decide.NewRecorder(&out, func() time.Time { return time.Unix(0, 0).UTC() }, "test")
		seeded.Seed(strings.NewReader(tail))
		seeded.Record("route", answered("typesafe", "jev-1.13.0", "a"), always)
		if rows := recalRows(t, &out); rows[0].RecalibrationDue {
			t.Error("the same model as the log's last route row is marked due")
		}
		seeded.Record("route", answered("typesafe", "jev-1.14.0", "a"), always)
		if rows := recalRows(t, &out); !rows[0].RecalibrationDue {
			t.Error("a move from the log's last route model is not marked due")
		}
	})
}
