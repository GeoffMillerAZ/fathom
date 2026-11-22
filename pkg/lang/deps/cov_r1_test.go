package deps_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

// TestCOVR1Development never reads a campaign manifest, holdout pin or sealed
// artifact. Its only population source is the original DEP-M1 development pins.
func TestCOVR1Development(t *testing.T) {
	if os.Getenv("FATHOM_COV_R1") != "development-only" {
		t.Skip("set FATHOM_COV_R1=development-only for the isolated development proof")
	}
	root := requiredPythonEnv(t, "FATHOM_COV_R1_ROOT")
	marker, err := os.ReadFile(filepath.Join(root, "R1-DEVELOPMENT-ONLY"))
	if err != nil || string(marker) != "DEP-M1 development pins only\n" {
		t.Fatal("R1 requires an isolated development root")
	}
	compiler := requiredPythonEnv(t, "FATHOM_COV_COMPILER")
	artifacts := filepath.Join(root, "reading")
	if os.Mkdir(artifacts, 0o700) != nil {
		t.Fatal("R1 proof destination must be fresh")
	}
	data, err := os.ReadFile(filepath.Join(tsjsOracleDir, "pins.json"))
	var pins struct{ Subjects []covSubject }
	if err != nil || json.Unmarshal(data, &pins) != nil {
		t.Fatal("development pins unavailable")
	}
	baseline := map[string]depbench.Observation{}
	for _, dialect := range []string{"typescript", "javascript"} {
		// This directory is explicitly the persisted development capture. Read
		// neither its aggregate sibling nor any other baseline artifact.
		data, err := os.ReadFile(filepath.Join(requiredPythonEnv(t, "FATHOM_COV_R1_BASELINE"), "development-"+dialect+"-observations.json"))
		var o depbench.Observation
		if err != nil || json.Unmarshal(data, &o) != nil {
			t.Fatal("frozen development baseline unavailable")
		}
		baseline[dialect] = o
	}
	pools := map[string]*depbench.Observation{"typescript": {}, "javascript": {}}
	externals := map[string]int{}
	var subjects []covSubjectReading
	for _, s := range pins.Subjects {
		primary := filepath.Join(root, "development", "primary", s.Name)
		relocated := filepath.Join(root, "development", "relocated", s.Name)
		first, paths := covFreeze(t, s, primary)
		second, again := covFreeze(t, s, relocated)
		if !slices.Equal(paths, again) || !bytes.Equal(asJSON(t, first), asJSON(t, second)) {
			t.Fatal("development relocation input mismatch")
		}
		a, ea := os.Stat(primary)
		b, eb := os.Stat(relocated)
		if ea != nil || eb != nil || os.SameFile(a, b) {
			t.Fatal("development arms must be physically distinct")
		}
		truth, _ := covTruth(t, "", compiler, "development", s, primary, paths)
		read := func(view fs.FS, names []string) depbench.COVBatch {
			inputs := make([]deps.Input, len(names))
			for i, name := range names {
				inputs[i] = deps.Input{Path: name, Dialect: s.Dialect}
			}
			batch, err := depbench.COVReadIsolated(t.Context(), &deps.Reader{}, view, inputs)
			if err != nil {
				t.Fatal("R1 development product read failed")
			}
			return batch
		}
		reverse := slices.Clone(paths)
		slices.Reverse(reverse)
		aBatch, bBatch, cBatch := read(covView(first), paths), read(covView(first), reverse), read(covView(second), paths)
		o := depbench.Observation{Files: depbench.COVIncludeRejected(aBatch), Repeat: depbench.COVIncludeRejected(bBatch), Relocated: depbench.COVIncludeRejected(cBatch), Truth: truth}
		inventory := depbench.COVInventory(covTree(first), nil)
		score := depbench.COVScore(o, inventory)
		score.B3 = score.B3 && depbench.COVSameRejected(aBatch.Rejected, bBatch.Rejected, cBatch.Rejected)
		score.RejectedInputs = len(aBatch.Rejected)
		externals[s.Dialect] += score.ExternalAsLocal
		covAppend(pools[s.Dialect], o, s.Name+"/")
		subjects = append(subjects, covSubjectReading{Name: s.Name, Dialect: s.Dialect, Commit: s.Commit, SelectionSHA256: s.SelectionSHA256, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(asJSON(t, first))), Score: score})
		// Check immutable development inputs after native and product reads.
		after, _ := covFreeze(t, s, primary)
		afterRelocated, _ := covFreeze(t, s, relocated)
		if !bytes.Equal(asJSON(t, first), asJSON(t, after)) || !bytes.Equal(asJSON(t, second), asJSON(t, afterRelocated)) {
			t.Fatal("development input drift")
		}
	}
	proof := map[string]any{"purpose": "R1 development only; no R5 or final reading", "subjects": subjects}
	for _, dialect := range []string{"typescript", "javascript"} {
		o := *pools[dialect]
		score := depbench.COVScore(o, nil)
		score.ExternalAsLocal = externals[dialect]
		score.B1 = score.B1 && score.ExternalAsLocal == 0
		before := depbench.COVScore(baseline[dialect], nil)
		proof[dialect] = struct{ Before, After depbench.COVReading }{before, score}
		if score.Files != before.Files || score.OracleFiles != before.OracleFiles || !bytes.Equal(asJSON(t, o.Truth), asJSON(t, baseline[dialect].Truth)) {
			t.Errorf("%s development denominator/native evidence changed", dialect)
		}
		t.Logf("%s files=%d oracle=%d complete=%d before=%d missing=%d external-as-local=%d wrong=%d inconclusive=%d B1=%v B2=%v B3=%v", dialect, score.Files, score.OracleFiles, score.Complete, before.Complete, score.Missing, score.ExternalAsLocal, score.Wrong, score.B2Inconclusive, score.B1, score.B2, score.B3)
		if !score.B1 || !score.B2 || !score.B3 || score.CompleteWithoutOracle != 0 || score.ExternalAsLocal != 0 {
			t.Errorf("%s R1 development proof fails or is inconclusive", dialect)
		}
		covWrite(t, filepath.Join(artifacts, "development-"+dialect+"-observations.json"), asJSON(t, o))
	}
	covWrite(t, filepath.Join(artifacts, "r1-development.json"), asJSON(t, proof))
	t.Logf("R1 development proof SHA256 %x", sha256.Sum256(asJSON(t, proof)))
}
