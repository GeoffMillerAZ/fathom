package deps_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

type covFinalPythonCapture struct {
	Original, Extended pythonOracleSubject
	Compared           map[string]covPythonCrossCheck
}

func covFinalDualTruth(t *testing.T, original, extended pythonOracleSubject, compared map[string]covPythonCrossCheck) ([]depbench.TruthFile, map[string]int) {
	t.Helper()
	if !bytes.Equal(original.Manifest, extended.Manifest) || len(original.Rows) != len(extended.Rows) {
		t.Fatal("COV Python oracle modes disagree on the population")
	}
	var truth []depbench.TruthFile
	classes := map[string]int{}
	for i, old := range original.Rows {
		newRow := extended.Rows[i]
		if old.Path != newRow.Path {
			t.Fatal("COV Python oracle modes differ on selected paths")
		}
		check := compared[old.Path]
		class := check.Class
		if class == "" {
			class = "missing-graph"
		}
		if old.Gap == "" {
			if !bytes.Equal(asJSON(t, old), asJSON(t, newRow)) {
				t.Fatal("COV Python extension changed originally graded truth")
			}
			truth = append(truth, depbench.TruthFile{Path: old.Path, Edges: old.Edges})
			classes["original/"+class]++
			continue
		}
		if newRow.Gap == "" {
			row, ok := covPythonDualTruth(newRow, check)
			if ok && check.Cohort == "new" {
				truth = append(truth, row)
			} else if class == "exact" || class == "initializer-plus-submodule" {
				class = "unproven-binding"
			}
		} else {
			class = "native-gap"
		}
		classes["added/"+class]++
	}
	return truth, classes
}

func covFinalSubject(t *testing.T, config covFinalConfig, base, pool string, subject covSubject, skipNative bool) (covFinalRow, depbench.Observation, bool) {
	t.Helper()
	primary := filepath.Join(base, "primary", subject.Name)
	relocated := filepath.Join(base, "relocated", subject.Name)
	config.progress(t, pool, "freeze")
	if config.Mode == "rehearse" {
		covFinalRehearsalDirectories(t, base, primary, relocated)
	}
	a := covFinalPhysical(t, primary)
	b := covFinalPhysical(t, relocated)
	if os.SameFile(a, b) {
		t.Fatal("COV final relocation requires distinct physical checkouts")
	}
	first, paths := covFreeze(t, subject, primary)
	second, again := covFreeze(t, subject, relocated)
	if !slices.Equal(paths, again) || !bytes.Equal(asJSON(t, first), asJSON(t, second)) {
		t.Fatal("COV final relocated tracked inputs differ")
	}
	var truth []depbench.TruthFile
	var classes map[string]int
	ungraded := skipNative
	if !ungraded {
		config.progress(t, pool, "native")
		truth, classes, ungraded = covFinalNative(t, config, pool, subject, primary, relocated, paths)
	}
	// The native captures and private scratch are gone before the reader is created.
	config.progress(t, pool, "read")
	var row covFinalRow
	var observation depbench.Observation
	var rejectedB3 bool
	if ungraded {
		batch := covFinalReadBatch(t, &deps.Reader{}, subject, first, paths)
		exception := "ExitError"
		if subject.Dialect == "python" {
			exception = "NativeNonzero"
		}
		row.ungraded = &covFinalUngraded{NativeException: exception, Files: len(paths),
			ReaderCompleteUngraded: covFinalComplete(batch.Files)}
	} else {
		row, observation, rejectedB3 = covFinalRead(t, subject, first, second, paths, truth)
		row.NativeClasses = classes
	}
	config.progress(t, pool, "refreeze")
	covFreeze(t, subject, primary)
	covFreeze(t, subject, relocated)
	return row, observation, rejectedB3
}

func covFinalNative(t *testing.T, config covFinalConfig, pool string, subject covSubject, primary, relocated string, paths []string) ([]depbench.TruthFile, map[string]int, bool) {
	t.Helper()
	if subject.Dialect == "python" {
		original, capture, failed := covFinalCapturePython(t, config, pool, subject.Name, "primary", paths)
		if failed {
			return nil, nil, true
		}
		truth, classes := covFinalDualTruth(t, capture.Original, capture.Extended, capture.Compared)
		moved, _, failed := covFinalCapturePython(t, config, pool, subject.Name, "relocated", paths)
		if failed {
			return nil, nil, true
		}
		if !bytes.Equal(original, moved) {
			t.Fatal("COV final relocated Python native evidence differs")
		}
		return truth, classes, false
	}

	capture := func(root string, inject bool) ([]depbench.TruthFile, map[string]string, *exec.ExitError) {
		t.Helper()
		truth, gaps, exit := covCaptureTruth(t, config.Python, config.Compiler, pool, subject, root, paths, config.isolatesNative(), inject)
		if exit != nil {
			// trace.cjs also checks tool identity. Keep a failed identity probe fatal.
			config.call(t, "_compiler_identity", pool)
		}
		return truth, gaps, exit
	}
	original, gaps, exit := capture(primary, false)
	if exit != nil {
		return nil, nil, true
	}
	inject := config.Mode == "rehearse" && config.InjectNativeFailure && pool == "holdout" && subject.Dialect == "javascript"
	moved, movedGaps, exit := capture(relocated, inject)
	if exit != nil {
		return nil, nil, true
	}
	if !bytes.Equal(asJSON(t, original), asJSON(t, moved)) || !bytes.Equal(asJSON(t, gaps), asJSON(t, movedGaps)) {
		t.Fatal("COV final relocated compiler truth differs")
	}
	classes := map[string]int{}
	for _, class := range gaps {
		classes[class]++
	}
	return original, classes, false
}

func covFinalCapturePython(t *testing.T, config covFinalConfig, pool, subject, arm string, paths []string) ([]byte, covFinalPythonCapture, bool) {
	t.Helper()
	var capture covFinalPythonCapture
	raw, err := config.output(t, "_python", pool, subject, arm)
	if err != nil {
		const nativeNonzeroStatus = 73 // Private exit status from cov_final.py _python.
		var exit *exec.ExitError
		if config.isolatesNative() && t.Context().Err() == nil && len(raw) == 0 &&
			errors.As(err, &exit) && exit.ExitCode() == nativeNonzeroStatus {
			return nil, capture, true
		}
		t.Fatal("COV final Python operation denied or failed")
	}
	if json.Unmarshal(raw, &capture) != nil || len(capture.Original.Rows) != len(paths) ||
		len(capture.Compared) != len(paths) {
		t.Fatal("COV final Python native capture malformed")
	}
	for i, file := range paths {
		if capture.Original.Rows[i].Path != file || capture.Compared[file].Class == "" {
			t.Fatal("COV final Python native population differs")
		}
	}
	return raw, capture, false
}

func covFinalRehearsalDirectories(t *testing.T, base, primary, relocated string) {
	t.Helper()
	// Lstat the enclosing components first so a symlink cannot redirect Git.
	for _, path := range []string{base, filepath.Dir(primary), filepath.Dir(relocated), primary, relocated} {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() {
			t.Fatal("COV rehearsal requires physical stage directories")
		}
	}
	for _, path := range []string{primary, relocated} {
		gitDir := filepath.Join(path, ".git")
		objects := filepath.Join(gitDir, "objects")
		for _, directory := range []string{gitDir, objects, filepath.Join(objects, "info")} {
			info, err := os.Lstat(directory)
			if err != nil || !info.IsDir() {
				t.Fatal("COV rehearsal requires a local Git object store")
			}
		}
		if _, err := os.Lstat(filepath.Join(gitDir, "commondir")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("COV rehearsal requires a local Git object store")
		}
		if _, err := os.Lstat(filepath.Join(objects, "info", "alternates")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("COV rehearsal requires an independent Git object store")
		}
		if err := filepath.WalkDir(objects, func(_ string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && !entry.Type().IsRegular() {
				return fs.ErrInvalid
			}
			return nil
		}); err != nil {
			t.Fatal("COV rehearsal requires a local Git object store")
		}
	}
	for _, path := range []string{primary, relocated} {
		objects := filepath.Join(path, ".git", "objects")
		effective, err := exec.CommandContext(t.Context(), "git", "-C", path, "rev-parse", "--path-format=absolute", "--git-path", "objects").Output()
		if err != nil || strings.TrimSpace(string(effective)) != objects {
			t.Fatal("COV rehearsal requires a local Git object store")
		}
		cmd := exec.CommandContext(t.Context(), "git", "-C", path, "symbolic-ref", "-q", "HEAD")
		var exit *exec.ExitError
		if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 1 {
			t.Fatal("COV rehearsal requires detached checkouts")
		}
	}
}

func covFinalPhysical(t *testing.T, root string) fs.FileInfo {
	t.Helper()
	physical, err := filepath.EvalSymlinks(root)
	info, statErr := os.Stat(root)
	if err != nil || physical != root || statErr != nil || !info.IsDir() {
		t.Fatal("COV final cohort requires physical checkout directories")
	}
	cmd := exec.CommandContext(t.Context(), "git", "-C", root, "rev-parse", "--show-toplevel")
	top, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(top)) != root {
		t.Fatal("COV final cohort requires a checkout root")
	}
	return info
}

func covFinalReadBatch(t *testing.T, reader *deps.Reader, subject covSubject, tree fstest.MapFS, selected []string) depbench.COVBatch {
	t.Helper()
	inputs := make([]deps.Input, len(selected))
	for i, file := range selected {
		inputs[i] = deps.Input{Path: file, Dialect: subject.Dialect}
	}
	batch, err := depbench.COVReadIsolated(t.Context(), reader, covView(tree), inputs)
	if err != nil {
		t.Fatal("COV final selected product read failed")
	}
	return batch
}

func covFinalComplete(files []deps.FileDeps) int {
	complete := 0
	for _, file := range files {
		if file.Complete {
			complete++
		}
	}
	return complete
}

func covFinalRead(t *testing.T, subject covSubject, first, second fstest.MapFS, paths []string, truth []depbench.TruthFile) (covFinalRow, depbench.Observation, bool) {
	t.Helper()
	primaryReader, repeatReader, movedReader := &deps.Reader{}, &deps.Reader{}, &deps.Reader{}
	reverse := slices.Clone(paths)
	slices.Reverse(reverse)
	primary := covFinalReadBatch(t, primaryReader, subject, first, paths)
	repeat := covFinalReadBatch(t, repeatReader, subject, first, reverse)
	moved := covFinalReadBatch(t, movedReader, subject, second, paths)
	observation := depbench.Observation{
		Files: depbench.COVIncludeRejected(primary), Repeat: depbench.COVIncludeRejected(repeat),
		Relocated: depbench.COVIncludeRejected(moved), Truth: truth,
	}
	rejected := func(batch depbench.COVBatch) map[string]bool {
		out := map[string]bool{}
		for _, input := range batch.Rejected {
			out[input.Path] = true
		}
		return out
	}
	refusals := rejected(primary)
	rejectedB3 := depbench.COVSameRejected(primary.Rejected, repeat.Rejected, moved.Rejected)
	score := depbench.COVScore(observation, depbench.COVInventory(covTree(first), subject.Roots))
	score.RejectedInputs = len(refusals)
	native := map[string]int{}
	for _, row := range truth {
		native[row.Path]++
	}
	for file := range refusals {
		if native[file] == 1 {
			score.RejectedOracleFiles++
		}
	}
	closure := covReach(t, primaryReader, first, paths, observation.Files, refusals)
	repeatClosure := covReach(t, repeatReader, first, reverse, observation.Repeat, rejected(repeat))
	movedClosure := covReach(t, movedReader, second, paths, observation.Relocated, rejected(moved))
	rejectedB3 = rejectedB3 && closure == repeatClosure && closure == movedClosure
	score.B3 = score.B3 && rejectedB3
	return covFinalRow{Score: score, Closure: closure}, observation, rejectedB3
}
