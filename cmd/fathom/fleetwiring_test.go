package main

// fleetwiring_test.go — SH-6, docs/plans/ambient-fathom.md: every apply
// writes a receipt carrying the full prior content of every region it
// touched; `fathom fleet receipts` lists them; `fathom fleet undo` restores
// exactly, and REFUSES when a file has changed since.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

func undoFixtureSpec() wiring.Spec {
	return wiring.Spec{
		Regions: []wiring.Region{{
			File:    "CLAUDE.md",
			Syntax:  wiring.HTMLMarkers,
			Version: "v3",
			Body:    "## fathom code intelligence\n\nIndexed as service \"demo\".",
		}},
		Keys: []wiring.Key{{
			File: "lefthook.yml",
			Kind: wiring.KindYAMLCommand,
			Path: []string{"pre-push", "commands", "fathom-verify"},
			Body: "run: fathom verify --service demo",
		}},
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestUndoRestoresOrRefuses drives the two CLI verbs over a real apply.
func TestUndoRestoresOrRefuses(t *testing.T) {
	root := t.TempDir()
	claudePath := filepath.Join(root, "CLAUDE.md")
	lefthookPath := filepath.Join(root, "lefthook.yml")
	claudeBefore := "# demo repo\n\nhouse prose that fathom must never touch\n"
	lefthookBefore := "pre-commit:\n  commands:\n    fmt:\n      run: gofmt -l .\n"
	mustWriteFile(t, claudePath, claudeBefore)
	mustWriteFile(t, lefthookPath, lefthookBefore)

	clock := func() time.Time { return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC) }
	res, err := wiring.ApplyWiring(root, undoFixtureSpec(), wiring.ApplyOptions{Now: clock})
	if err != nil {
		t.Fatalf("ApplyWiring: %v", err)
	}
	if res.Receipt == nil {
		t.Fatal("apply wrote no receipt — there is nothing to undo")
	}
	if mustReadFile(t, claudePath) == claudeBefore {
		t.Fatal("apply changed nothing — the undo probe would be vacuous")
	}

	// ─── fathom fleet receipts ───────────────────────────────────────────
	var out, errBuf bytes.Buffer
	if code := cmdFleetReceipts([]string{"--repo", root}, &out, &errBuf); code != 0 {
		t.Fatalf("fleet receipts exit = %d, stderr: %s", code, errBuf.String())
	}
	listing := out.String()
	if !strings.Contains(listing, res.Receipt.ID) {
		t.Errorf("receipts listing does not name the receipt %s:\n%s", res.Receipt.ID, listing)
	}
	if !strings.Contains(listing, "CLAUDE.md") || !strings.Contains(listing, "lefthook.yml") {
		t.Errorf("receipts listing does not name the files touched:\n%s", listing)
	}

	// ─── fathom fleet undo --last ────────────────────────────────────────
	out.Reset()
	errBuf.Reset()
	if code := cmdFleetUndo([]string{"--repo", root, "--last"}, &out, &errBuf); code != 0 {
		t.Fatalf("fleet undo exit = %d, stderr: %s\nstdout: %s", code, errBuf.String(), out.String())
	}
	if got := mustReadFile(t, claudePath); got != claudeBefore {
		t.Errorf("CLAUDE.md not restored to its exact prior bytes:\n got %q\nwant %q", got, claudeBefore)
	}
	if got := mustReadFile(t, lefthookPath); got != lefthookBefore {
		t.Errorf("lefthook.yml not restored to its exact prior bytes:\n got %q\nwant %q", got, lefthookBefore)
	}

	// ─── it refuses when a file changed since ────────────────────────────
	res2, err := wiring.ApplyWiring(root, undoFixtureSpec(), wiring.ApplyOptions{Now: clock})
	if err != nil {
		t.Fatalf("second ApplyWiring: %v", err)
	}
	if res2.Receipt == nil {
		t.Fatal("re-apply after undo wrote no receipt")
	}
	afterApply := mustReadFile(t, claudePath)
	handEdited := afterApply + "\na human appended a line after fathom's write\n"
	mustWriteFile(t, claudePath, handEdited)
	lefthookAfterApply := mustReadFile(t, lefthookPath)

	out.Reset()
	errBuf.Reset()
	code := cmdFleetUndo([]string{"--repo", root, "--last"}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("undo succeeded over a changed file — it must refuse\nstdout: %s", out.String())
	}
	said := out.String() + errBuf.String()
	if !strings.Contains(said, "CLAUDE.md") {
		t.Errorf("refusal does not name the file that changed:\n%s", said)
	}
	if !strings.Contains(strings.ToLower(said), "changed") {
		t.Errorf("refusal does not say why:\n%s", said)
	}
	if got := mustReadFile(t, claudePath); got != handEdited {
		t.Errorf("a refused undo still wrote to CLAUDE.md:\n got %q\nwant %q", got, handEdited)
	}
	if got := mustReadFile(t, lefthookPath); got != lefthookAfterApply {
		t.Error("a refused undo restored a DIFFERENT file — undo is all-or-nothing")
	}

	// ─── an unknown receipt id is an error, not a guess ──────────────────
	out.Reset()
	errBuf.Reset()
	if code := cmdFleetUndo([]string{"--repo", root, "deadbeefdead"}, &out, &errBuf); code == 0 {
		t.Error("undo accepted an unknown receipt id")
	}
}

// TestReceiptsLimitDisclosesWhatItHid pins U2, found in review: --limit
// truncated the list and then printed the TRUNCATED length as the total.
// Law 3 applied to a count — the number rendered must be the number
// measured. A reader concluded fathom had written twice where it wrote
// five times, and never learned three still-undoable receipts existed, in
// the verb whose only job is to tell the truth about what fathom changed.
func TestReceiptsLimitDisclosesWhatItHid(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	for i := range 5 {
		r := wiring.Receipt{
			Time: at.Add(time.Duration(i) * time.Minute).UTC().Format(time.RFC3339),
			Root: root,
			Files: []wiring.ReceiptFile{{
				Path: "CLAUDE.md", Existed: true, Before: "before", AfterExists: true,
			}},
		}
		if _, err := wiring.AppendReceipt(root, r, 0); err != nil {
			t.Fatal(err)
		}
	}

	var out, errb bytes.Buffer
	if code := cmdFleetReceipts([]string{"--repo", root, "--limit", "2"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "5 receipt(s) total") {
		t.Errorf("the TOTAL must be stated, not the truncated length:\n%s", got)
	}
	if !strings.Contains(got, "3 hidden by --limit") {
		t.Errorf("what the limit hid must be disclosed:\n%s", got)
	}
	if !strings.Contains(got, "still undoable") {
		t.Errorf("a reader must learn the hidden receipts are still actionable:\n%s", got)
	}

	t.Run("json carries the same three numbers", func(t *testing.T) {
		var jout, jerr bytes.Buffer
		if code := cmdFleetReceipts([]string{"--repo", root, "--limit", "2", "--json"}, &jout, &jerr); code != 0 {
			t.Fatalf("exit %d: %s", code, jerr.String())
		}
		var m struct {
			Total  int `json:"total"`
			Shown  int `json:"shown"`
			Hidden int `json:"hidden_by_limit"`
		}
		if err := json.Unmarshal(jout.Bytes(), &m); err != nil {
			t.Fatalf("bad json: %v", err)
		}
		if m.Total != 5 || m.Shown != 2 || m.Hidden != 3 {
			t.Errorf("got total=%d shown=%d hidden=%d, want 5/2/3", m.Total, m.Shown, m.Hidden)
		}
	})

	t.Run("no limit reports the plain total", func(t *testing.T) {
		var o, e bytes.Buffer
		if code := cmdFleetReceipts([]string{"--repo", root}, &o, &e); code != 0 {
			t.Fatalf("exit %d: %s", code, e.String())
		}
		if !strings.Contains(o.String(), "5 receipt(s), newest first") {
			t.Errorf("an untruncated list keeps its plain phrasing:\n%s", o.String())
		}
	})
}
