package main

// init_wiring_test.go — INC-12 (docs/intent/incidents.md): `fathom init`
// writes CLAUDE.md, AGENTS.md and .claude/settings.json THROUGH
// internal/wiring, so every write is receipted and `fathom fleet undo`
// reverses it. The incident's "would have caught it" line is this test.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

func TestInitWritesReceipt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 7)
	defer srv.Close()

	prior := "# house rules\n\nnever auto-ingest here\n"
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	if code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errBuf); code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}

	receipts, err := wiring.LoadReceipts(dir)
	if err != nil {
		t.Fatalf("LoadReceipts: %v", err)
	}
	if len(receipts) != 1 {
		t.Fatalf("receipts = %d, want exactly 1 for one init (files touched: %v)", len(receipts), receipts)
	}
	r := receipts[0]
	files := map[string]wiring.ReceiptFile{}
	for _, f := range r.Files {
		files[f.Path] = f
	}
	claude, ok := files["CLAUDE.md"]
	if !ok {
		t.Fatalf("receipt carries no CLAUDE.md entry: %+v", r.Files)
	}
	if !claude.Existed || claude.Before != prior {
		t.Errorf("receipt prior content for CLAUDE.md: existed=%v before=%q, want the exact prior bytes %q", claude.Existed, claude.Before, prior)
	}
	if len(claude.Touched) != 1 || claude.Touched[0].Kind != wiring.KindRegion || claude.Touched[0].Present {
		t.Errorf("CLAUDE.md touched = %+v, want one appended (not previously present) region", claude.Touched)
	}
	for _, created := range []string{"AGENTS.md", ".claude/settings.json"} {
		f, ok := files[created]
		if !ok {
			t.Errorf("receipt carries no %s entry", created)
			continue
		}
		if f.Existed {
			t.Errorf("%s recorded as pre-existing; init created it", created)
		}
	}

	// The receipt is the undo: `fathom fleet undo --last` restores the
	// prior CLAUDE.md exactly and removes what init created.
	out.Reset()
	errBuf.Reset()
	if code := cmdFleetUndo([]string{"--repo", dir, "--last"}, &out, &errBuf); code != 0 {
		t.Fatalf("fleet undo exit = %d: %s%s", code, out.String(), errBuf.String())
	}
	got, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != prior {
		t.Errorf("undo did not restore CLAUDE.md:\n got %q\nwant %q", got, prior)
	}
	for _, created := range []string{"AGENTS.md", filepath.Join(".claude", "settings.json")} {
		if _, err := os.Stat(filepath.Join(dir, created)); !os.IsNotExist(err) {
			t.Errorf("%s still exists after undo (err=%v); the receipt said init created it", created, err)
		}
	}
}

// TestInitContinuesPastBlockedFiles pins the law-15 outcome on init's own
// path: a BLOCKED wiring file (a tracked CLAUDE.md region edited by hand,
// AND a .claude/settings.json that will not parse) is recorded and reported
// at the end — one line per FILE, each naming its cause — while every
// other step still runs: AGENTS.md is written, the config scaffold lands,
// --skills installs, the manual steps print, and the exit is 1. Before
// this, init returned at the first BLOCKED file and skipped all of that,
// counted eight refused hook keys as "8 file(s)", and called an unparsable
// settings.json a hand-edited region.
func TestInitContinuesPastBlockedFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 7)
	defer srv.Close()

	// CLAUDE.md: tracked by fathom, then edited by hand inside the region.
	claudePath := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte("# repo\n\nhouse prose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyStanza(claudePath, fathomStanza(svc), &bytes.Buffer{}); err != nil {
		t.Fatalf("applyStanza: %v", err)
	}
	written, _ := os.ReadFile(claudePath)
	edited := strings.Replace(string(written), "Read files to *edit* them", "Read files to *edit* them (house rule)", 1)
	if edited == string(written) {
		t.Fatal("fixture: the hand edit changed nothing")
	}
	if err := os.WriteFile(claudePath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	// .claude/settings.json: not JSON at all.
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	broken := "{ this is not json\n"
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	if err := os.WriteFile(settingsPath, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath, "--skills=claude"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (a BLOCKED file is a real problem, not operational); stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}

	// Nothing was written to the blocked files.
	if got, _ := os.ReadFile(claudePath); string(got) != edited {
		t.Errorf("init MERGED over the hand-edited tracked region:\n%s", got)
	}
	if got, _ := os.ReadFile(settingsPath); string(got) != broken {
		t.Errorf("init rewrote the unparsable settings.json:\n%s", got)
	}

	// Every other step ran.
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md was not written alongside the BLOCKED files: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".fathom", "config.toml")); err != nil {
		t.Errorf("the config.toml scaffold was skipped after a BLOCKED file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "fathom-verify", "SKILL.md")); err != nil {
		t.Errorf("--skills=claude was skipped after a BLOCKED file: %v", err)
	}
	for _, want := range []string{"MANUAL: register the fathom MCP server", "shim"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout lacks %q — the steps after the BLOCKED file did not run:\n%s", want, out.String())
		}
	}

	// The report: one line per file, cause named, no item counting.
	stderr := errBuf.String()
	var fileLines []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(line, "error:   ") {
			fileLines = append(fileLines, line)
		}
	}
	if len(fileLines) != 2 {
		t.Fatalf("blocked-file lines = %d, want exactly 2 (one per file, not one per refused hook key):\n%s", len(fileLines), stderr)
	}
	if !strings.HasPrefix(fileLines[0], "error:   .claude/settings.json — could not be read/parsed:") {
		t.Errorf("settings.json line = %q, want it named as unreadable, not as a hand-edited region", fileLines[0])
	}
	if !strings.HasPrefix(fileLines[1], "error:   CLAUDE.md — drifted (hand-edited) region:") || !strings.Contains(fileLines[1], "edited by hand") {
		t.Errorf("CLAUDE.md line = %q, want it named as drift", fileLines[1])
	}
	if !strings.Contains(stderr, "2 files BLOCKED") {
		t.Errorf("the summary does not count FILES:\n%s", stderr)
	}
	if strings.Contains(stderr, "8 file") || strings.Contains(stderr, "9 file") {
		t.Errorf("the summary counts items as files:\n%s", stderr)
	}
	// stdout's per-file narration is deduplicated the same way.
	if n := strings.Count(out.String(), "BLOCKED .claude/settings.json"); n != 1 {
		t.Errorf("stdout names the unparsable settings.json %d times, want once:\n%s", n, out.String())
	}
}
