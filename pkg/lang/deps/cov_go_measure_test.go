package deps_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

type covGoPin struct {
	Subject, Commit string
	Files           int
	SelectionSHA256 string `json:"selection_sha256"`
}

func covGoPins(t *testing.T) []covGoPin {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(covOracleDir, "cov_go_pins.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pins struct{ Subjects []covGoPin }
	if json.Unmarshal(data, &pins) != nil || len(pins.Subjects) != 3 {
		t.Fatal("invalid Go preregistration")
	}
	return pins.Subjects
}

// TestCOVGoCapture runs native tools only, before any corpus reader is built.
func TestCOVGoCapture(t *testing.T) {
	if os.Getenv("FATHOM_COV_GO_CAPTURE") != "1" {
		t.Skip("opt-in COV-M3 native capture")
	}
	source := requiredPythonEnv(t, "FATHOM_COV_GO_SOURCES")
	output := requiredPythonEnv(t, "FATHOM_COV_GO_ROOT")
	script, err := filepath.Abs(filepath.Join(covOracleDir, "cov_go.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "python3", "-B", script, source, output)
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native capture failed: %v\n%s", err, data)
	}
	t.Log(strings.TrimSpace(string(data)))
}

type covGoNative struct {
	Truth []depbench.TruthFile
	Gaps  map[string]string
}

type covGoSubjectReading struct {
	Subject, Commit, SelectionSHA256 string
	Primary, CGO                     depbench.COVReading
	GapsPrimary, GapsCGO             map[string]int
	CompleteContextDisagreements     int
	Receipt                          json.RawMessage
}

// TestCOVGoAdmission exposes candidate completeness with an explicit trust opt-in.
// It cannot change the reader's admission flag or the public measured set.
func TestCOVGoAdmission(t *testing.T) {
	if os.Getenv("FATHOM_COV_GO_MEASURE") != "1" {
		t.Skip("opt-in COV-M3 measurement")
	}
	root := requiredPythonEnv(t, "FATHOM_COV_GO_ROOT")
	marker, err := os.OpenFile(filepath.Join(root, "measurement.started"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal("Go measurement root already used; preserve it and prepare a fresh campaign")
	}
	if err := marker.Close(); err != nil {
		t.Fatal(err)
	}
	revision, err := exec.CommandContext(t.Context(), "git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	var pooled, cgoPooled depbench.Observation
	var subjects []covGoSubjectReading
	var external, disagreements, rejectedTotal int
	allRejectedStable := true
	for _, pin := range covGoPins(t) {
		dir := filepath.Join(root, pin.Subject)
		primary, relocated := filepath.Join(dir, "primary"), filepath.Join(dir, "relocated")
		url, err := exec.CommandContext(t.Context(), "git", "-C", primary, "remote", "get-url", "origin").Output()
		if err != nil {
			t.Fatal(err)
		}
		subject := covSubject{tsjsSubject: tsjsSubject{Name: pin.Subject, URL: strings.TrimSpace(string(url)), Commit: pin.Commit, Dialect: "go", Prefixes: []string{""}, Suffixes: []string{".go"}, Files: pin.Files, SelectionSHA256: pin.SelectionSHA256}}
		first, selected := covFreeze(t, subject, primary)
		second, again := covFreeze(t, subject, relocated)
		if !slices.Equal(selected, again) || !bytes.Equal(asJSON(t, first), asJSON(t, second)) {
			t.Fatal("Go relocated inputs differ")
		}
		a, ea := os.Stat(primary)
		b, eb := os.Stat(relocated)
		if ea != nil || eb != nil || os.SameFile(a, b) {
			t.Fatal("Go relocation is not physically distinct")
		}
		receipt, err := os.ReadFile(filepath.Join(dir, "receipt.json"))
		if err != nil {
			t.Fatal(err)
		}
		var provenance struct {
			Captures []struct {
				CGO         string
				RawSHA256   string `json:"raw_sha256"`
				TruthSHA256 string `json:"truth_sha256"`
			}
		}
		if json.Unmarshal(receipt, &provenance) != nil || len(provenance.Captures) != 2 {
			t.Fatal("Go native provenance unavailable")
		}
		var native [2]covGoNative
		gaps := [2]map[string]int{{}, {}}
		for i := range native {
			data, err := os.ReadFile(filepath.Join(dir, "truth-cgo"+string(rune('0'+i))+".json"))
			if err != nil || json.Unmarshal(data, &native[i]) != nil {
				t.Fatal("Go native evidence unavailable")
			}
			capture := provenance.Captures[i]
			if capture.CGO != fmt.Sprint(i) || fmt.Sprintf("%x", sha256.Sum256(data)) != capture.TruthSHA256 {
				t.Fatal("Go adapted truth digest mismatch")
			}
			raw, err := os.ReadFile(filepath.Join(dir, "native-cgo"+fmt.Sprint(i)+".json"))
			if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != capture.RawSHA256 {
				t.Fatal("Go raw native digest mismatch")
			}

			membership := map[string]int{}
			for _, f := range native[i].Truth {
				membership[f.Path]++
			}
			for file, gap := range native[i].Gaps {
				membership[file]++
				gaps[i][gap]++
			}
			if len(membership) != len(selected) {
				t.Fatal("Go native denominator differs from selection")
			}
			for _, file := range selected {
				if membership[file] != 1 {
					t.Fatal("Go native coverage is not a partition")
				}
			}
		}
		reader, err := deps.New(deps.TrustUnmeasured("go"))
		if err != nil {
			t.Fatal(err)
		}
		read := func(view fs.FS, paths []string) depbench.COVBatch {
			inputs := make([]deps.Input, len(paths))
			for i, file := range paths {
				inputs[i] = deps.Input{Path: file, Dialect: "go"}
			}
			batch, err := depbench.COVReadIsolated(context.Background(), reader, view, inputs)
			if err != nil {
				t.Fatal("Go product observation failed")
			}
			return batch
		}
		reverse := slices.Clone(selected)
		slices.Reverse(reverse)
		one, two, three := read(covView(first), selected), read(covView(first), reverse), read(covView(second), selected)
		stable := depbench.COVSameRejected(one.Rejected, two.Rejected, three.Rejected)
		allRejectedStable = allRejectedStable && stable
		o := depbench.Observation{Files: depbench.COVIncludeRejected(one), Repeat: depbench.COVIncludeRejected(two), Relocated: depbench.COVIncludeRejected(three), Truth: native[0].Truth}
		inventory := depbench.COVGoInventory(covTree(first))
		score := depbench.COVGoScore(o, inventory)
		score.B3 = score.B3 && stable
		score.RejectedInputs = len(one.Rejected)
		alternative := o
		alternative.Truth = native[1].Truth
		cgoScore := depbench.COVGoScore(alternative, inventory)
		cgoScore.B3 = cgoScore.B3 && stable
		cgoScore.RejectedInputs = len(one.Rejected)
		truthMaps := [2]map[string][]depbench.TruthEdge{{}, {}}
		for i := range native {
			for _, f := range native[i].Truth {
				truthMaps[i][f.Path] = f.Edges
			}
		}
		differing := 0
		for _, f := range o.Files {
			if f.Complete {
				left, lok := truthMaps[0][f.Path]
				right, rok := truthMaps[1][f.Path]
				if lok && rok && !bytes.Equal(asJSON(t, left), asJSON(t, right)) {
					differing++
				}
			}
		}
		subjects = append(subjects, covGoSubjectReading{pin.Subject, pin.Commit, pin.SelectionSHA256, score, cgoScore, gaps[0], gaps[1], differing, receipt})
		covAppend(&pooled, o, pin.Subject+"/")
		covAppend(&cgoPooled, alternative, pin.Subject+"/")
		external += score.ExternalAsLocal
		disagreements += differing
		rejectedTotal += len(one.Rejected)
		covWrite(t, filepath.Join(dir, "observation.json"), asJSON(t, o))
		covWrite(t, filepath.Join(dir, "reader-primary.json"), asJSON(t, o.Files))
		covWrite(t, filepath.Join(dir, "reader-repeat.json"), asJSON(t, o.Repeat))
		covWrite(t, filepath.Join(dir, "reader-relocated.json"), asJSON(t, o.Relocated))
		covFreeze(t, subject, primary)
		covFreeze(t, subject, relocated)
	}
	score := depbench.COVScore(pooled, nil)
	score.ExternalAsLocal = external
	score.B1 = score.B1 && external == 0
	score.B3 = score.B3 && allRejectedStable
	score.RejectedInputs = rejectedTotal
	cgoScore := depbench.COVScore(cgoPooled, nil)
	cgoScore.ExternalAsLocal = external
	cgoScore.B1 = cgoScore.B1 && external == 0
	cgoScore.B3 = cgoScore.B3 && allRejectedStable
	cgoScore.RejectedInputs = rejectedTotal
	aggregate := struct {
		Preregistration, ReaderBase, HarnessRevision string
		Subjects                                     []covGoSubjectReading
		Primary, CGO                                 depbench.COVReading
		CompleteContextDisagreements                 int
	}{"cea1adaeb", "7f70e23d7feb3bd272fdf80cc1300035c48fb93c", strings.TrimSpace(string(revision)), subjects, score, cgoScore, disagreements}
	data := asJSON(t, aggregate)
	covWrite(t, filepath.Join(root, "aggregate.json"), data)
	t.Logf("COV-M3 aggregate SHA256 %x", sha256.Sum256(data))
	t.Logf("COV-M3 files=%d complete=%d oracle=%d complete-without-truth=%d missing=%d wrong=%d inconclusive=%d external-as-local=%d B1=%t B2=%t B3=%t", score.Files, score.Complete, score.OracleFiles, score.CompleteWithoutOracle, score.Missing, score.Wrong, score.B2Inconclusive, score.ExternalAsLocal, score.B1, score.B2, score.B3)
}
