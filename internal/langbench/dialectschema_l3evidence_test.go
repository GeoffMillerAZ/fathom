package langbench

import (
	"encoding/json"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// TestDialectL3EvidenceSchema (S-R6, owner ruling 2026-10-25 S3-R4): l3_evidence is a
// manifest's own minimum-evidence bar for L3, both counts at least 1; each documented
// negative (schema/dialect/testdata/invalid.cue 16aa, 16ab) is refused, and the manifest's
// bar reaches the reading it starts.
func TestDialectL3EvidenceSchema(t *testing.T) {
	cctx := cuecontext.New()
	schema := dialectSchema(t, false)
	base := func(extra string) string {
		return `{id: "m", display: "M", kind: "tooling", engine: "native", lang_tag: "m",
		match: {extensions: [".m"]}, kinds: {model: {source_kind: "M", canonical: "module"}},
		required_kinds: ["model"], target_level: "L3", queue_id: "m"` + extra + `}`
	}
	for name, m := range map[string]string{
		"a bar":  base(`, l3_evidence: {min_references: 20, min_subjects: 2}`),
		"no bar": base(""),
	} {
		v := cctx.CompileString(schema + "\nok: #Dialect & " + m)
		if err := v.Validate(cue.Concrete(true)); err != nil {
			t.Errorf("%s must validate: %v", name, err)
		}
	}
	for name, m := range map[string]string{
		"16aa no references":   base(`, l3_evidence: {min_references: 0, min_subjects: 2}`),
		"16ab a missing count": base(`, l3_evidence: {min_references: 20}`),
	} {
		bad := cctx.CompileString(schema + "\nbad: #Dialect & " + m)
		if bad.Err() == nil && bad.Validate(cue.Concrete(true)) == nil {
			t.Errorf("%s: schema accepted %s", name, m)
		}
	}
	var man Manifest
	if err := json.Unmarshal([]byte(`{"id": "m", "kind": "tooling", "l3_evidence": {"min_references": 20, "min_subjects": 2}}`), &man); err != nil {
		t.Fatal(err)
	}
	if r := NewReading(man, "L3"); r.L3Evidence == nil || *r.L3Evidence != (EvidenceBar{MinReferences: 20, MinSubjects: 2}) {
		t.Errorf("reading bar %+v, want the manifest's", r.L3Evidence)
	}
}
