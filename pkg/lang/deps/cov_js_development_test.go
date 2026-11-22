package deps_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

const covJSDispatch = "54fcfb42b725372ccf2f11480ad389514d8f0a7d"
const covJSDispatchEvidence = "c380c71a1d0da43e219dd9d0f56e49ff40bbdc870a7ba76878f5eb0ffc789632"

var covJSOraclePins = map[string]string{
	"pins.json": "223b056dd6fb6de720e24f6e28f05770e1c951b061ea801964098087a2993ca2",
	"trace.cjs": "325b6c2f80c13ca8bc8eac23b2fd4e744af5e1e76bd6ae59ad4e883eed722938",
}

type covJSSubjectProof struct {
	Name, Dialect, Commit, SelectionSHA256, SourceSHA256, ReaderSHA256, NativeSHA256 string
	Score                                                                            depbench.COVReading
	Closure                                                                          depbench.COVClosureCounts
	Gained, Lost, Ungraded                                                           int
}

type covJSPoolProof struct {
	Score                  depbench.COVReading
	Closure                depbench.COVClosureCounts
	Residual               []depbench.COVBucket
	Greedy                 []depbench.COVStep
	Gained, Lost, Ungraded int
	B4Delta                float64
}

type covJSProof struct {
	Dispatch, BaselineSHA256 string
	Subjects                 map[string]covJSSubjectProof
	Pools                    map[string]covJSPoolProof
}

// TestCOVJSDocDevelopment grades only the original development selections.
// Capture the dispatch before changing the product so out-of-selection closure
// targets, as well as the selected observations, have a true baseline.
func TestCOVJSDocDevelopment(t *testing.T) {
	mode := os.Getenv("FATHOM_COV_JS_DEVELOPMENT")
	if mode == "" {
		t.Skip("set FATHOM_COV_JS_DEVELOPMENT=baseline or candidate for the pinned development proof")
	}
	if mode != "baseline" && mode != "candidate" {
		t.Fatal("COV-3 mode must be baseline or candidate")
	}
	if mode == "baseline" {
		cmd := exec.CommandContext(t.Context(), "git", "diff", "--quiet", covJSDispatch, "--", "tsjs.go", "tsjs_resolve.go", "tsjs_config.go", "tsjs_locality.go", "tsjs_package.go")
		if err := cmd.Run(); err != nil {
			t.Fatal("COV-3 baseline reader differs from dispatch")
		}
		if _, err := os.Lstat("tsjs_jsdoc.go"); !os.IsNotExist(err) {
			t.Fatal("COV-3 baseline has candidate code")
		}
	}
	root := tsjsRequiredPath(t, "FATHOM_COV_JS_ARMS")
	compiler := tsjsRequiredPath(t, "FATHOM_COV_JS_COMPILER")
	for _, prefix := range []string{root, filepath.Join(root, "primary"), filepath.Join(root, "relocated")} {
		if real, err := filepath.EvalSymlinks(prefix); err != nil || filepath.Base(root) != "development" || real != prefix {
			t.Fatal("COV-3 requires physical development arms")
		}
	}
	out := os.Getenv("FATHOM_COV_JS_EVIDENCE")
	if !filepath.IsAbs(out) || os.Mkdir(out, 0o700) != nil {
		t.Fatal("COV-3 requires a fresh absolute evidence directory")
	}
	var pins struct{ Subjects []covSubject }
	for file, want := range covJSOraclePins {
		actual := covJSRead(t, filepath.Join(tsjsOracleDir, file))
		if covJSDigest(actual) != want || file == "pins.json" && json.Unmarshal(actual, &pins) != nil {
			t.Fatal("COV-3 native oracle identity differs")
		}
	}
	if len(pins.Subjects) != 7 {
		t.Fatal("COV-3 native population differs")
	}
	proof := covJSProof{Dispatch: covJSDispatch, Subjects: map[string]covJSSubjectProof{}, Pools: map[string]covJSPoolProof{}}
	var baseline covJSProof
	var baselineRoot string
	if mode == "candidate" {
		baselineRoot = tsjsRequiredPath(t, "FATHOM_COV_JS_BASELINE")
		data := covJSRead(t, filepath.Join(baselineRoot, "aggregate.json"))
		proof.BaselineSHA256 = covJSDigest(data)
		if proof.BaselineSHA256 != covJSDispatchEvidence || json.Unmarshal(data, &baseline) != nil || baseline.Dispatch != covJSDispatch || baseline.BaselineSHA256 != "" {
			t.Fatal("COV-3 dispatch evidence is invalid")
		}
	}

	pools := map[string]*covPool{}
	reader := &deps.Reader{}
	for _, subject := range pins.Subjects {
		s := subject
		t.Run(s.Name, func(t *testing.T) {
			if s.Dialect != "javascript" && s.Dialect != "typescript" || filepath.Base(s.Name) != s.Name {
				t.Fatal("unexpected COV-3 development subject")
			}
			primary := filepath.Join(root, "primary", s.Name)
			relocated := filepath.Join(root, "relocated", s.Name)
			for _, name := range []string{primary, relocated} {
				if real, err := filepath.EvalSymlinks(name); err != nil || real != name {
					t.Fatal("COV-3 subject is not a physical checkout")
				}
			}
			first, paths := covFreeze(t, s, primary)
			second, again := covFreeze(t, s, relocated)
			a, ea := os.Stat(primary)
			b, eb := os.Stat(relocated)
			source := asJSON(t, first)
			if ea != nil || eb != nil || os.SameFile(a, b) || !slices.Equal(paths, again) || !bytes.Equal(source, asJSON(t, second)) {
				t.Fatal("COV-3 frozen development arms differ or share a physical checkout")
			}
			work := filepath.Join(out, s.Name)
			native := tsjsNative(t, compiler, filepath.Join(work, "primary"), primary, paths)
			movedNative := tsjsNative(t, compiler, filepath.Join(work, "relocated"), relocated, paths)
			nativeData := asJSON(t, native)
			if !bytes.Equal(nativeData, asJSON(t, movedNative)) {
				t.Fatal("COV-3 native evidence differs across development arms")
			}
			graded := map[string]int{}
			for _, row := range native.Truth {
				if _, ok := slices.BinarySearch(paths, row.Path); !ok || graded[row.Path] != 0 {
					t.Fatal("COV-3 native row is duplicate or outside the frozen selection")
				}
				graded[row.Path]++
			}
			read := func(view *covFrozenView, files []string) depbench.COVBatch {
				inputs := make([]deps.Input, len(files))
				for i, file := range files {
					inputs[i] = deps.Input{Path: file, Dialect: s.Dialect}
				}
				batch, err := depbench.COVReadIsolated(t.Context(), reader, view, inputs)
				if err != nil {
					t.Fatal("COV-3 product read failed")
				}
				return batch
			}
			reverse := slices.Clone(paths)
			slices.Reverse(reverse)
			firstView := covView(first)
			initial, repeat, moved := read(firstView, paths), read(firstView, reverse), read(covView(second), again)
			o := depbench.Observation{Files: depbench.COVIncludeRejected(initial), Repeat: depbench.COVIncludeRejected(repeat),
				Relocated: depbench.COVIncludeRejected(moved), Truth: native.Truth}
			if len(o.Files) != len(paths) || !depbench.COVSameRejected(initial.Rejected, repeat.Rejected, moved.Rejected) {
				t.Fatal("COV-3 selected population or repeated rejections differ")
			}
			rejected := map[string]bool{}
			score := depbench.COVScore(o, depbench.COVInventory(covTree(first), s.Roots))
			for _, input := range initial.Rejected {
				rejected[input.Path] = true
				score.RejectedInputs++
				score.RejectedOracleFiles += graded[input.Path]
			}
			data := asJSON(t, o.Files)
			covWrite(t, filepath.Join(work, "reader.json"), data)
			row := covJSSubjectProof{Name: s.Name, Dialect: s.Dialect, Commit: s.Commit, SelectionSHA256: s.SelectionSHA256,
				SourceSHA256: covJSDigest(source), ReaderSHA256: covJSDigest(data), NativeSHA256: covJSDigest(nativeData),
				Score: score, Closure: covReach(t, reader, first, paths, o.Files, rejected)}
			if mode == "candidate" {
				old := baseline.Subjects[s.Name]
				if row.Name != old.Name || row.Dialect != old.Dialect || row.Commit != old.Commit || row.SelectionSHA256 != old.SelectionSHA256 ||
					row.SourceSHA256 != old.SourceSHA256 || row.NativeSHA256 != old.NativeSHA256 || score.RejectedInputs != old.Score.RejectedInputs {
					t.Fatal("COV-3 candidate changed development identity, native evidence or rejection count")
				}
				if s.Dialect == "typescript" && !bytes.Equal(asJSON(t, old), asJSON(t, row)) {
					t.Fatal("COV-3 changed TypeScript evidence")
				}
				oldData := covJSRead(t, filepath.Join(baselineRoot, s.Name, "reader.json"))
				var oldFiles []deps.FileDeps
				if covJSDigest(oldData) != old.ReaderSHA256 || json.Unmarshal(oldData, &oldFiles) != nil || len(oldFiles) != len(o.Files) {
					t.Fatal("COV-3 baseline observations differ")
				}
				for i, file := range o.Files {
					if oldFiles[i].Path != file.Path {
						t.Fatal("COV-3 baseline selected paths differ")
					}
					if !oldFiles[i].Complete && file.Complete {
						row.Gained++
						if graded[file.Path] != 1 {
							row.Ungraded++
						}
					}
					if oldFiles[i].Complete && !file.Complete {
						row.Lost++
					}
				}
			}
			if !score.B3 {
				t.Error("COV-3 development arm or repeat differs")
			}
			proof.Subjects[s.Name] = row
			p := pools[s.Dialect]
			if p == nil {
				p = &covPool{shapes: map[string][]string{}, gaps: map[string]string{}, rejected: map[string]bool{}}
				pools[s.Dialect] = p
			}
			prefix := s.Name + "/"
			for _, file := range o.Files {
				key := prefix + file.Path
				if rejected[file.Path] {
					p.rejected[key] = true
				}
				if graded[file.Path] == 0 {
					p.gaps[key] = "native-no-truth"
				}
				for _, u := range file.Unknowns {
					p.shapes[key] = append(p.shapes[key], covShape(first[file.Path].Data, u))
				}
			}
			p.external += score.ExternalAsLocal
			covAddClosure(&p.closure, row.Closure)
			covAppend(&p.observation, o, prefix)
			covFreeze(t, s, primary)
			covFreeze(t, s, relocated)
			t.Logf("files=%d truth=%d complete=%d closure=%d gained=%d lost=%d ungraded=%d B1=%t B2=%t B3=%t", score.Files, score.OracleFiles,
				score.Complete, row.Closure.Complete, row.Gained, row.Lost, row.Ungraded, score.B1, score.B2, score.B3)
		})
	}
	if len(proof.Subjects) != len(pins.Subjects) {
		t.Fatal("COV-3 development subject proof is incomplete")
	}
	for _, dialect := range []string{"javascript", "typescript"} {
		p := pools[dialect]
		analysis := depbench.COVAnalyzeRejected(p.observation, p.shapes, p.gaps, nil, p.rejected)
		score := analysis.Score
		score.ExternalAsLocal = p.external
		score.B1 = score.B1 && p.external == 0
		row := covJSPoolProof{Score: score, Closure: p.closure, Residual: analysis.Buckets, Greedy: analysis.Greedy}
		for _, subject := range proof.Subjects {
			if subject.Dialect == dialect {
				row.Gained += subject.Gained
				row.Lost += subject.Lost
				row.Ungraded += subject.Ungraded
			}
		}
		if mode == "candidate" {
			old := baseline.Pools[dialect].Score
			if old.B4ComparableEdges == 0 || score.B4ComparableEdges == 0 || score.Files != old.Files || score.OracleFiles != old.OracleFiles {
				t.Fatal("COV-3 development denominators differ")
			}
			row.B4Delta = float64(score.B4Extra)/float64(score.B4ComparableEdges) - float64(old.B4Extra)/float64(old.B4ComparableEdges)
		}
		proof.Pools[dialect] = row
		t.Logf("%s files=%d truth=%d complete=%d closure=%+v B1=%t B2=%t B3=%t miss/wrong/inconclusive=%d/%d/%d B4=%d/%d delta=%.6f gained=%d lost=%d ungraded=%d",
			dialect, score.Files, score.OracleFiles, score.Complete, row.Closure, score.B1, score.B2, score.B3,
			score.Missing, score.Wrong, score.B2Inconclusive, score.B4Extra, score.B4ComparableEdges, row.B4Delta, row.Gained, row.Lost, row.Ungraded)
		if !score.B1 || !score.B2 || !score.B3 || row.Lost != 0 || row.Ungraded != 0 || row.B4Delta > 0.05 {
			t.Errorf("COV-3 %s development bars failed", dialect)
		}
	}
	js, ts := proof.Pools["javascript"], proof.Pools["typescript"]
	if js.Score.Files != 1508 || ts.Score.Files != 1224 || ts.Score.Complete != 250 ||
		mode == "baseline" && js.Score.Complete != 153 ||
		mode == "candidate" && (baseline.Pools["javascript"].Score.Complete != 153 || (js.Score.Complete-153)*100 < 2*1508) {
		t.Error("COV-3 fixed populations, dispatch counts or two-point gain bar failed")
	}
	data := asJSON(t, proof)
	covWrite(t, filepath.Join(out, "aggregate.json"), data)
	t.Logf("COV-3 %s aggregate SHA-256 %s", mode, covJSDigest(data))
}

func covJSRead(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal("COV-3 development evidence unavailable")
	}
	return data
}

func covJSDigest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
