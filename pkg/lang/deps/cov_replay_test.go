package deps_test

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

// TestCOVFrozenDevelopmentReplay recovers only persisted development evidence
// after a campaign refusal. It invokes neither a Reader nor a native oracle.
// Lost closure and native gap detail remain absent, never fabricated as zero.
func TestCOVFrozenDevelopmentReplay(t *testing.T) {
	if os.Getenv("FATHOM_COV_FROZEN_REPLAY") != "1" {
		t.Skip("set FATHOM_COV_FROZEN_REPLAY=1 to analyze already captured development observations")
	}
	root := requiredPythonEnv(t, "FATHOM_COV_ROOT")
	expected := covExpectedSubjects(t)
	public := map[string]any{"status": "incomplete campaign; frozen development recovery only", "holdout": nil}
	pools := map[string]any{}
	var subjects []any
	for _, dialect := range []string{"python", "javascript", "typescript"} {
		data, err := os.ReadFile(filepath.Join(root, "reading", "development-"+dialect+"-observations.json"))
		if err != nil {
			t.Fatal("frozen development observation unavailable")
		}
		var o depbench.Observation
		if json.Unmarshal(data, &o) != nil {
			t.Fatal("frozen development observation malformed")
		}
		shapes := map[string][]string{}
		gaps := map[string]string{}
		truth := map[string]bool{}
		for _, row := range o.Truth {
			truth[row.Path] = true
		}
		for _, file := range o.Files {
			if !truth[file.Path] {
				gaps[file.Path] = "native-no-truth"
			}
		}
		external, total := 0, 0
		for _, s := range expected["development"] {
			if s.Dialect != dialect {
				continue
			}
			tree, selected := covFreeze(t, s, filepath.Join(root, "development", "primary", s.Name))
			total += len(selected)
			prefix := s.Name + "/"
			filterFiles := func(files []deps.FileDeps) []deps.FileDeps {
				return slices.DeleteFunc(slices.Clone(files), func(f deps.FileDeps) bool { return !strings.HasPrefix(f.Path, prefix) })
			}
			subject := depbench.Observation{Files: filterFiles(o.Files), Repeat: filterFiles(o.Repeat), Relocated: filterFiles(o.Relocated), Truth: slices.DeleteFunc(slices.Clone(o.Truth), func(f depbench.TruthFile) bool { return !strings.HasPrefix(f.Path, prefix) })}
			for _, file := range subject.Files {
				rel := strings.TrimPrefix(file.Path, prefix)
				entry := tree[rel]
				if entry == nil {
					t.Fatal("frozen development path absent from pinned tree")
				}
				for _, u := range file.Unknowns {
					shapes[file.Path] = append(shapes[file.Path], covShape(entry.Data, u))
				}
			}
			score := depbench.COVScore(subject, depbench.COVInventory(covTree(tree), s.Roots))
			external += score.ExternalAsLocal
			subjects = append(subjects, struct {
				Name, Dialect, Commit string
				Score                 depbench.COVReading
				Closure               any
			}{s.Name, dialect, s.Commit, score, nil})
		}
		if total != len(o.Files) {
			t.Fatal("frozen development population count mismatch")
		}
		analysis := depbench.COVAnalyze(o, shapes, gaps, nil)
		analysis.Score.ExternalAsLocal = external
		analysis.Score.B1 = analysis.Score.B1 && external == 0
		pools[dialect] = struct {
			Analysis depbench.COVDevelopment
			Closure  any
		}{analysis, nil}
	}
	public["development"] = pools
	public["development_subjects"] = subjects
	public["lost_evidence"] = []string{"development and holdout closure aggregates", "per-file native gap reasons", "pooled holdout scores"}
	// Bind to the actual committed baseline harness, not the working tree.
	rev, err := exec.CommandContext(t.Context(), "git", "rev-parse", "a1faba52a").Output()
	if err != nil {
		t.Fatal("baseline harness commit unavailable")
	}
	public["baseline_harness_commit"] = strings.TrimSpace(string(rev))
	bytes := asJSON(t, public)
	destination := filepath.Join(root, "reading", "development-recovered.json")
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal("frozen development recovery already exists or cannot be created")
	}
	if _, err = file.Write(bytes); err != nil {
		file.Close()
		t.Fatal("frozen development recovery write failed")
	}
	if file.Close() != nil {
		t.Fatal("frozen development recovery close failed")
	}
	t.Logf("COV frozen development aggregate SHA256 %x", sha256.Sum256(bytes))
}
