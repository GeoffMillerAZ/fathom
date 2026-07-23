package main

// observe_test.go — FW-2 (docs/plans/pack-taxonomy-adoption.md): `fathom
// observe`. Tested per the plan's own list: create, append (with
// incrementing IDs and byte-for-byte preservation of prior entries),
// malformed-existing-file refusal (never clobber).

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

	data, err := os.ReadFile(observationsPath(root))
	if err != nil {
		t.Fatalf("observations.md not created: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		observationsMarker,
		"## Open",
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
	if _, err := os.Stat(observationsPath(root)); err == nil {
		t.Error("observations.md must not be created when --symptom is missing")
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

func TestObserve_RefusesMalformedExistingFile(t *testing.T) {
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
	code := cmdObserve([]string{"--root", root, "--symptom", "should not be recorded"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("observe against a malformed pre-existing file: want non-zero exit")
	}
	if !strings.Contains(stderr.String(), "refusing to append") {
		t.Errorf("stderr = %q, want a refusal message", stderr.String())
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after refusal: %v", err)
	}
	if string(after) != original {
		t.Errorf("file was modified despite refusal.\nwant: %q\ngot:  %q", original, string(after))
	}
}

// TestObserve_ConcurrentFirstCreationLosesNothing is B1's regression test
// (critic blocker, proven 13/20 trials pre-fix): two observes racing
// first-time creation in a repo with NO .fathom/observations.md yet. The
// pre-fix os.WriteFile (O_TRUNC) path let both see existed=false, and the
// second silently clobbered the first's entry — both exiting 0. With
// O_CREATE|O_EXCL exactly one wins creation; the loser appends. Both
// entries must be present, the header exactly once, both exits 0. Entry
// IDs may collide under this race — documented as advisory (H1) — so the
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
		if got := strings.Count(content, observationsMarker); got != 1 {
			t.Errorf("trial %d: marker appears %d times, want exactly 1 (no clobber, no double header)", trial, got)
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
	if _, err := os.Stat(observationsPath(root)); err != nil {
		t.Errorf("observations.md not created via top-level dispatch: %v", err)
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
