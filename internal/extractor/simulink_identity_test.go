package extractor

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// simulinkIdentityGolden holds the calibration models' per-model counts, no model content.
const simulinkIdentityGolden = "testdata/simulink_identity.json"

// simulinkCounts is what the identity golden records of one model.
type simulinkCounts struct {
	Blocks    int `json:"blocks"`
	Signals   int `json:"signals"` // distinct
	ModelRefs int `json:"model_refs"`
	Calls     int `json:"calls"` // the Calls ExtractDoc emits, summed over its nodes
	CrossFile int `json:"cross_file"`
}

// TestSimulinkCalibrationIdentity is S4-R4: every .slx and .mdl under CALIBRATION_DIR (the
// simulink subjects, `task calibrate-clone -- simulink`) reads and extracts to the counts the
// golden records per model, so a reader change proves identity on record. It runs only when
// CALIBRATION_DIR is set; SIMULINK_IDENTITY_WRITE=1 rewrites the golden.
func TestSimulinkCalibrationIdentity(t *testing.T) {
	dir := os.Getenv("CALIBRATION_DIR")
	if dir == "" {
		t.Skip("CALIBRATION_DIR is not set")
	}
	got := map[string]simulinkCounts{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || (filepath.Ext(p) != ".slx" && filepath.Ext(p) != ".mdl") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		m, _ := ReadSimulinkModel(rel, src)
		r, err := NewSimulinkExtractor().ExtractDoc("svc", rel, src)
		if err != nil {
			return err
		}
		c := simulinkCounts{Blocks: len(m.Blocks), Signals: len(m.Signals), ModelRefs: len(m.ModelRefs), CrossFile: len(r.CrossFile)}
		for _, n := range r.Nodes {
			c.Calls += len(n.Calls)
		}
		got[rel] = c
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatalf("no .slx or .mdl under %s", dir)
	}
	var total simulinkCounts
	for _, c := range got {
		total.Blocks += c.Blocks
		total.Signals += c.Signals
		total.ModelRefs += c.ModelRefs
		total.Calls += c.Calls
		total.CrossFile += c.CrossFile
	}
	t.Logf("%d models: %d blocks, %d distinct signals, %d model refs, %d calls, %d cross-file refs",
		len(got), total.Blocks, total.Signals, total.ModelRefs, total.Calls, total.CrossFile)
	if os.Getenv("SIMULINK_IDENTITY_WRITE") == "1" {
		b, err := json.MarshalIndent(got, "", "  ") // map keys marshal sorted
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(simulinkIdentityGolden, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(simulinkIdentityGolden)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]simulinkCounts{}
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	var diffs []string
	for rel, w := range want {
		if g, ok := got[rel]; !ok {
			diffs = append(diffs, rel+": missing")
		} else if g != w {
			b1, _ := json.Marshal(g)
			b2, _ := json.Marshal(w)
			diffs = append(diffs, rel+": "+string(b1)+", want "+string(b2))
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			diffs = append(diffs, rel+": not in the golden")
		}
	}
	sort.Strings(diffs)
	if len(diffs) > 0 {
		t.Errorf("%d of %d models differ from %s:\n%s", len(diffs), len(want), simulinkIdentityGolden, strings.Join(diffs, "\n"))
	}
}
