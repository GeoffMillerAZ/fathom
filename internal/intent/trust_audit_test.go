package intent_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

func TestAuditConfinesConditionAnnotations(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"ADR.md": "# Decisions\n\n" +
			"## ADR-001 Foreign service\n\nRevisit when: query another service {oracle: index, service: other, instrument: foreign, threshold: \"> 0\"}\n\n" +
			"## ADR-002 Escaping path\n\nRevisit when: read outside {oracle: lexical, paths: ../x, instrument: escape, threshold: \"> 0\"}\n\n" +
			"## ADR-003 Safe condition\n\nRevisit when: a local count changes {oracle: index, service: local, instrument: safe, threshold: \"> 0\"}\n",
	})
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Conditions) != 3 {
		t.Fatalf("conditions = %+v, want three", c.Conditions)
	}
	var reads []string
	a := intent.RunAudit(intent.AuditInput{
		Corpus: c, Now: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		LocalServices: map[string]bool{"local": true},
		Instruments: func(name string) (intent.InstrumentReading, bool, error) {
			reads = append(reads, name)
			return intent.InstrumentReading{Value: 1, Reading: "one local event"}, true, nil
		},
	})
	if !reflect.DeepEqual(reads, []string{"safe"}) {
		t.Errorf("instrument reads = %v, want only safe", reads)
	}
	verdicts := map[string]intent.TriggerVerdict{}
	for _, v := range a.Triggers {
		verdicts[v.Record] = v
	}
	for i, reason := range []string{"cross-repo refused", "escapes the repository root"} {
		cond := c.Conditions[i]
		v := verdicts[cond.Record]
		err := intent.ConfineAnnotation(root, cond.Ann, map[string]bool{})
		if err == nil {
			t.Fatalf("fixture annotation must be refused: %+v", cond)
		}
		if v.State != "unevaluable" || v.Reason != err.Error() || !strings.Contains(v.Reason, reason) {
			t.Errorf("%s verdict = %+v, want unevaluable with confinement reason %q", cond.Record, v, err)
		}
		if v.File != cond.File || v.Line != cond.Line {
			t.Errorf("refusal lost its citation: %+v", v)
		}
		if v.Instrument != cond.Ann["instrument"] || v.Threshold != cond.Ann["threshold"] {
			t.Errorf("refusal lost its instrument or threshold: %+v", v)
		}
	}
	if v := verdicts["ADR-003"]; v.State != intent.TriggerFired || v.Reading != "one local event" {
		t.Errorf("safe condition did not evaluate: %+v", v)
	}
	if len(c.Unparsed) != 1 || c.Unparsed[0].File != "ADR.md" || c.Unparsed[0].Line != c.Conditions[1].Line || !strings.Contains(c.Unparsed[0].Reason, "escapes the repository root") {
		t.Errorf("escaping path must be a located lint finding: %+v", c.Unparsed)
	}
}

func TestAuditLintsUnsafeAnnotationPaths(t *testing.T) {
	for _, key := range []string{"path", "paths", "plan"} {
		for _, tc := range []struct{ name, path string }{
			{"parent", "../x"},
			{"absolute", filepath.Join(t.TempDir(), "x")},
		} {
			t.Run(key+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeTree(t, root, map[string]string{
					"ADR.md": "# Decisions\n\n## ADR-001 Unsafe path\n\nDone when: evidence appears {oracle: lexical, " + key + ": " + tc.path + "}\n",
				})
				c, err := intent.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				a := intent.RunAudit(intent.AuditInput{Corpus: c, Now: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)})
				if len(a.Triggers) != 1 || a.Triggers[0].State != "unevaluable" || !strings.Contains(a.Triggers[0].Reason, "refused") {
					t.Errorf("unsafe annotation must be unevaluable even without a trigger oracle: %+v", a.Triggers)
				}
				if len(c.Unparsed) != 1 || !strings.Contains(c.Unparsed[0].Reason, "refused") {
					t.Errorf("unsafe path must lint loudly: %+v", c.Unparsed)
				}
			})
		}
	}
}

func TestAuditConfinementKeepsIncompleteTriggersProse(t *testing.T) {
	a := intent.RunAudit(intent.AuditInput{
		Corpus: intent.Corpus{Root: t.TempDir(), Conditions: []intent.Condition{{
			ID: "ADR-001#1", Record: "ADR-001", Label: "Revisit when",
			Ann: map[string]string{"instrument": "safe"},
		}}},
		Now: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	})
	if len(a.Triggers) != 1 || a.Triggers[0].State != intent.TriggerUnevaluated || a.Triggers[0].Instrument != "" || !strings.Contains(a.Triggers[0].Reason, "prose") {
		t.Errorf("an incomplete trigger must remain prose: %+v", a.Triggers)
	}
}
