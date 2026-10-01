package main

// observe_test.go — `fathom observe` writes one exclusive-create file per
// entry under .fathom/observations/ (EW-32): create, incrementing ids,
// byte-for-byte preservation of prior entries, the legacy log read but
// never written.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// writeObservationsFixture writes a valid .fathom/observations.md under
// root with nOpen entries whose Status is "open" and nResolved entries
// whose Status is "resolved" — shared by FW-1's report tests (open-count)
// and FW-2's own tests.
func writeObservationsFixture(t *testing.T, root string, nOpen, nResolved int) {
	t.Helper()
	var b strings.Builder
	b.WriteString(observationsHeader)
	id := 1
	for i := 0; i < nOpen; i++ {
		fmt.Fprintf(&b, "\n### O%d — fixture open entry %d\n\n- **Symptom:** x\n- **Status:** open\n", id, i)
		id++
	}
	for i := 0; i < nResolved; i++ {
		fmt.Fprintf(&b, "\n### O%d — fixture resolved entry %d\n\n- **Symptom:** x\n- **Status:** resolved\n", id, i)
		id++
	}
	path := observationsPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestObserve_CreatesFileWithHeaderAndFirstEntry(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := cmdObserve([]string{
		"--root", root,
		"--symptom", "find_symbol missed a method",
		"--tool", "find_symbol",
		"--expected", "the method to be found",
		"--actual", "empty result",
		"--repro", "find_symbol(svc, Foo.Bar)",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(observationsDir(root), "O1.md"))
	if err != nil {
		t.Fatalf("O1.md not created: %v", err)
	}
	content := string(data)
	if _, err := os.Stat(observationsPath(root)); err == nil {
		t.Error("the legacy observations.md must not be created")
	}
	readme, err := os.ReadFile(filepath.Join(observationsDir(root), "README.md"))
	if err != nil || !strings.Contains(string(readme), observationsMarker) {
		t.Errorf("README.md header missing: %v", err)
	}
	for _, want := range []string{
		"### O1 — find_symbol missed a method",
		"- **Symptom:** find_symbol missed a method",
		"- **Tool:** find_symbol",
		"- **Expected:** the method to be found",
		"- **Actual:** empty result",
		"- **Repro:** find_symbol(svc, Foo.Bar)",
		"- **Root cause:** TODO",
		"- **Fix:** TODO",
		"- **Status:** open",
		"- **Impact:** TODO",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("observations.md missing %q:\n%s", want, content)
		}
	}
	if !strings.Contains(stdout.String(), "recorded O1") {
		t.Errorf("stdout = %q, want confirmation of O1", stdout.String())
	}
}

func TestObserve_OptionalFieldsDefaultToTODO(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := cmdObserve([]string{"--root", root, "--symptom", "something broke"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	content, _, err := readObservations(root)
	if err != nil {
		t.Fatalf("readObservations: %v", err)
	}
	for _, want := range []string{"- **Tool:** TODO", "- **Expected:** TODO", "- **Actual:** TODO", "- **Repro:** TODO"} {
		if !strings.Contains(content, want) {
			t.Errorf("content missing %q:\n%s", want, content)
		}
	}
}

func TestObserve_RequiresSymptom(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := cmdObserve([]string{"--root", root}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("observe with no --symptom: want non-zero exit")
	}
	if _, err := os.Stat(observationsDir(root)); err == nil {
		t.Error("observations/ must not be created when --symptom is missing")
	}
}

func TestObserve_AppendsSecondEntryWithIncrementingIDAndNeverRewrites(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := cmdObserve([]string{"--root", root, "--symptom", "first issue"}, &stdout, &stderr); code != 0 {
		t.Fatalf("first observe: exit = %d, stderr: %s", code, stderr.String())
	}
	firstContent, _, err := readObservations(root)
	if err != nil {
		t.Fatalf("readObservations after first: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := cmdObserve([]string{"--root", root, "--symptom", "second issue"}, &stdout, &stderr); code != 0 {
		t.Fatalf("second observe: exit = %d, stderr: %s", code, stderr.String())
	}
	secondContent, _, err := readObservations(root)
	if err != nil {
		t.Fatalf("readObservations after second: %v", err)
	}

	// Append-only: everything written by the first call must survive
	// byte-for-byte as a prefix of the file after the second call.
	if !strings.HasPrefix(secondContent, firstContent) {
		t.Fatalf("second call did not purely append — first call's content was not preserved as a prefix.\nfirst:\n%s\nsecond:\n%s", firstContent, secondContent)
	}
	if !strings.Contains(secondContent, "### O1 — first issue") {
		t.Errorf("O1 missing after second call:\n%s", secondContent)
	}
	if !strings.Contains(secondContent, "### O2 — second issue") {
		t.Errorf("O2 missing after second call:\n%s", secondContent)
	}
	if !strings.Contains(stdout.String(), "recorded O2") {
		t.Errorf("stdout = %q, want confirmation of O2", stdout.String())
	}
}

func TestObserve_ForeignLegacyFileIsNeverTouched(t *testing.T) {
	root := t.TempDir()
	path := observationsPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	original := "# Someone else's notes\n\nThis file has nothing to do with fathom.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if code := cmdObserve([]string{"--root", root, "--symptom", "recorded elsewhere"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != original {
		t.Errorf("legacy file modified.\nwant: %q\ngot:  %q (%v)", original, string(after), err)
	}
	if _, err := os.Stat(filepath.Join(observationsDir(root), "O1.md")); err != nil {
		t.Errorf("O1.md missing: %v", err)
	}
}

// TestObserveWritesPerEntryFileAndReadersReadBoth is EW-32's proof: the
// legacy log ends at O274 and the directory holds O290.md; observe mints
// O291.md, never touches the legacy file, and every reader counts both.
func TestObserveWritesPerEntryFileAndReadersReadBoth(t *testing.T) {
	root := t.TempDir()
	legacy := observationsPath(root)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyBytes := observationsHeader + "\n### O273 — legacy open\n\n- **Status:** open\n\n### O274 — legacy done\n\n- **Status:** resolved\n"
	if err := os.WriteFile(legacy, []byte(legacyBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := observationsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "O290.md"), []byte("### O290 — dir open\n\n- **Status:** open\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := cmdObserve([]string{"--root", root, "--symptom", "new one"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "recorded O291") {
		t.Errorf("stdout = %q, want O291", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "O291.md")); err != nil {
		t.Fatalf("O291.md not written: %v", err)
	}
	after, _ := os.ReadFile(legacy)
	if string(after) != legacyBytes {
		t.Error("the legacy log was modified")
	}

	content, existed, err := readObservations(root)
	if err != nil || !existed {
		t.Fatalf("readObservations: existed=%v err=%v", existed, err)
	}
	if got := countOpenObservations(content); got != 3 {
		t.Errorf("countOpenObservations = %d, want 3 (O273, O290, O291)", got)
	}
	if got := nextObservationID(content); got != 292 {
		t.Errorf("nextObservationID = %d, want 292", got)
	}
	if n, note := serviceOpenObservations(root); n == nil || *n != 3 || note != "" {
		t.Errorf("serviceOpenObservations = %v, %q; want 3", n, note)
	}
}

// An id taken between the reader's scan and the create moves to the next.
func TestObserveSkipsATakenID(t *testing.T) {
	root := t.TempDir()
	dir := observationsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// An empty (mid-write) O1.md has no header, so a scan alone would re-mint 1.
	if err := os.WriteFile(filepath.Join(dir, "O1.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	id, _, err := mintObservationEntry(root, "x", "", "", "", "")
	if err != nil || id != 2 {
		t.Fatalf("id = %d, err = %v; want 2", id, err)
	}
}

// An unreadable entry file is named, never folded into a smaller total (I38).
func TestReadObservationsNamesUnreadableEntry(t *testing.T) {
	root := t.TempDir()
	dir := observationsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "O5.md")
	if err := os.WriteFile(p, []byte("### O5 — x\n- **Status:** open\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if f, err := os.Open(p); err == nil {
		f.Close()
		t.Skip("running as a user that can read mode 000")
	}
	_, _, err := readObservations(root)
	if err == nil || !strings.Contains(err.Error(), "O5.md") {
		t.Fatalf("err = %v, want one naming O5.md", err)
	}
	if n, note := serviceOpenObservations(root); n != nil || !strings.Contains(note, "O5.md") {
		t.Errorf("serviceOpenObservations = %v, %q; want nil and a note naming O5.md", n, note)
	}
}

// TestObserve_ConcurrentFirstCreationLosesNothing is B1's regression test
// (critic blocker, proven 13/20 trials pre-fix): two observes racing
// first-time creation in a repo with NO .fathom/observations/ yet. The
// pre-fix os.WriteFile (O_TRUNC) path let both see existed=false, and the
// second silently clobbered the first's entry — both exiting 0. With
// O_CREATE|O_EXCL exactly one wins creation; the loser appends. Both
// entries must be present, the header exactly once, both exits 0. Entry
// Ids never collide now (exclusive create), but the
// test asserts entry COUNT and BOTH symptoms, never distinct ids.
// Repeated a few times per run to actually exercise the racy window.
func TestObserve_ConcurrentFirstCreationLosesNothing(t *testing.T) {
	for trial := 0; trial < 5; trial++ {
		root := t.TempDir() // fresh: no .fathom/observations.md — the exact B1 scenario
		const n = 2
		var wg sync.WaitGroup
		codes := make([]int, n)
		stderrs := make([]bytes.Buffer, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				var stdout bytes.Buffer
				codes[i] = cmdObserve([]string{"--root", root, "--symptom", fmt.Sprintf("trial %d concurrent issue %d", trial, i)}, &stdout, &stderrs[i])
			}(i)
		}
		wg.Wait()

		for i := 0; i < n; i++ {
			if codes[i] != 0 {
				t.Fatalf("trial %d: observe %d exit = %d, stderr: %s", trial, i, codes[i], stderrs[i].String())
			}
		}
		content, existed, err := readObservations(root)
		if err != nil || !existed {
			t.Fatalf("trial %d: readObservations: existed=%v err=%v", trial, existed, err)
		}
		for i := 0; i < n; i++ {
			if !strings.Contains(content, fmt.Sprintf("trial %d concurrent issue %d", trial, i)) {
				t.Errorf("trial %d: entry %d LOST — the B1 silent-data-loss bug:\n%s", trial, i, content)
			}
		}
		if got := len(observationEntryHeader.FindAllString(content, -1)); got != n {
			t.Errorf("trial %d: %d entry headers, want %d", trial, got, n)
		}
	}
}

func TestObserve_ViaTopLevelRunDispatch(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"observe", "--root", root, "--symptom", "dispatch check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(observationsDir(root), "O1.md")); err != nil {
		t.Errorf("O1.md not created via top-level dispatch: %v", err)
	}
}

// ─── observations.go helpers, exercised directly ───────────────────────────

func TestCountOpenObservations(t *testing.T) {
	root := t.TempDir()
	writeObservationsFixture(t, root, 2, 1)
	content, existed, err := readObservations(root)
	if err != nil || !existed {
		t.Fatalf("readObservations: existed=%v err=%v", existed, err)
	}
	if got := countOpenObservations(content); got != 2 {
		t.Errorf("countOpenObservations = %d, want 2", got)
	}
}

func TestNextObservationID_EmptyContentStartsAtOne(t *testing.T) {
	if got := nextObservationID(""); got != 1 {
		t.Errorf("nextObservationID(\"\") = %d, want 1", got)
	}
}
