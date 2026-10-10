package wiring

// baseline_after_write_test.go — FA-23 and FA-24 (docs/plans/
// fleet-adoption-fixes.md Bug 5, O657). The 2026-09-28 fleet pass rewrote
// every adopter's skills and left the skill baseline at the bytes it
// replaced: reconcile recorded baselines BEFORE the apply, and the apply
// moved only the owned-file id. Every write now records each baseline that
// watches the file from the bytes written, in the same ledger flush, and a
// rebaseline re-records a skill only when its content is one fathom
// shipped.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const skillLabel = ".claude/skills/fathom-x/SKILL.md"

func writeFile(t *testing.T, root, rel, body string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// An apply that rewrites an owned file moves BOTH baselines that watch it —
// the owned-file id the planner reads and the skill id DriftSkills reads —
// to the digest of the bytes written, in the receipted ledger flush.
func TestApplyRecordsEveryBaselineFromTheBytesWritten(t *testing.T) {
	root := t.TempDir()
	older := testMarker + "\nthe render two releases back\n"
	old := testMarker + "\nthe render one release back\n"
	cur := testMarker + "\nthe render this binary ships\n"
	writeFile(t, root, skillLabel, old)
	ledger := NewLedger()
	ledger.Record(skillLabel, OwnedKey{ID: ownedFileID, Digest: digest(old), Wrote: "2026-09-28T18:50:14Z"})
	ledger.Record(skillLabel, OwnedKey{ID: skillKeyID, Digest: digest(older), Wrote: "2026-09-28"})
	if err := SaveLedger(root, ledger); err != nil {
		t.Fatal(err)
	}

	res, err := ApplyWiring(root, Spec{Files: []OwnedFile{{File: skillLabel, Marker: testMarker, Body: cur}}}, ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != 1 || res.Receipt == nil {
		t.Fatalf("apply = %+v", res)
	}
	ledger, _ = LoadLedger(root)
	for _, id := range []string{ownedFileID, skillKeyID} {
		if k, ok := ownedKey(ledger, skillLabel, id); !ok || k.Digest != digest(cur) {
			t.Errorf("ledger %s = %+v, %v; want the digest of the bytes written", id, k, ok)
		}
	}
	// The ledger is in the receipt, so undo restores both together.
	found := false
	for _, f := range res.Receipt.Files {
		found = found || f.Path == ledgerRelPath
	}
	if !found {
		t.Errorf("receipt %v does not carry %s", res.Receipt.Files, ledgerRelPath)
	}
	// A later binary that ships something else reads the file as fathom's
	// own older render, not as a hand edit.
	rep := DriftSkills(ledger, []SkillFile{{Path: filepath.Join(root, filepath.FromSlash(skillLabel)), Label: skillLabel, Shipped: []byte(testMarker + "\nnext release\n")}}, nil)
	if len(rep.Drifts) != 0 {
		t.Errorf("drift after a recorded write: %+v", rep.Drifts)
	}
}

// Content fathom shipped in an earlier release (Known), or bytes the ledger
// records fathom writing as the whole owned file, are fathom's own: stale,
// never drift. Anything else under the marker is drift.
func TestDriftSkillsFathomRendersAreNotDrift(t *testing.T) {
	root := t.TempDir()
	prior := testMarker + "\na render an earlier release shipped\n"
	path := writeFile(t, root, skillLabel, prior)
	ledger := NewLedger()
	ledger.Record(skillLabel, OwnedKey{ID: skillKeyID, Digest: digest("something older"), Wrote: "2026-09-28"})
	f := SkillFile{Path: path, Label: skillLabel, Shipped: []byte(testMarker + "\ncurrent\n")}

	if rep := DriftSkills(ledger, []SkillFile{f}, nil); len(rep.Drifts) != 1 {
		t.Fatalf("an unknown render must drift: %+v", rep)
	}
	f.Known = map[string]bool{digest(prior): true}
	if rep := DriftSkills(ledger, []SkillFile{f}, nil); len(rep.Drifts) != 0 {
		t.Errorf("a known prior render read as drift: %+v", rep.Drifts)
	}
	f.Known = nil
	ledger.Record(skillLabel, OwnedKey{ID: ownedFileID, Digest: digest(prior), Wrote: "2026-09-28T18:50:14Z"})
	if rep := DriftSkills(ledger, []SkillFile{f}, nil); len(rep.Drifts) != 0 {
		t.Errorf("bytes the ledger records fathom writing read as drift: %+v", rep.Drifts)
	}
	// And the planner rewrites a known prior render instead of blocking it.
	of := OwnedFile{File: skillLabel, Marker: testMarker, Body: string(f.Shipped), Known: map[string]bool{digest(prior): true}}
	other := NewLedger()
	other.Record(skillLabel, OwnedKey{ID: ownedFileID, Digest: digest("an older write"), Wrote: "x"})
	if it := planOwnedFile(root, of, other); it.Verdict != VerdictApply {
		t.Errorf("a known prior render = %s %q, want apply", it.Verdict, it.Reason)
	}
	of.Known = nil
	if it := planOwnedFile(root, of, other); it.Verdict != VerdictBlocked || it.Cause != CauseDrift {
		t.Errorf("an unknown render = %s %q, want blocked drift", it.Verdict, it.Reason)
	}
}

// FA-24: --rebaseline re-records a skill only when its content is a render
// fathom shipped (this binary's or a known prior one); any other content
// stays DRIFT and is named. A lagging baseline under a known render is
// re-recorded without --rebaseline. MCP entries keep the operator's
// rebaseline: fathom renders none.
func TestRebaselineReRecordsOnlyFathomRenders(t *testing.T) {
	root := t.TempDir()
	mcpPath := writeFile(t, root, ".mcp.json", `{"mcpServers":{"fathom":{"type":"http","url":"http://127.0.0.1:7737/rpc"}}}`)
	edited := testMarker + "\n---\nhooks: [x]\n---\n"
	skillPath := writeFile(t, root, skillLabel, edited)
	mcp := []MCPFile{{Path: mcpPath, Label: ".mcp.json", Format: "json", KeyPath: []string{"mcpServers", "fathom"}}}
	prior := testMarker + "\nan earlier render\n"
	skills := []SkillFile{{Path: skillPath, Label: skillLabel, Shipped: []byte(testMarker + "\ncurrent\n"), Known: map[string]bool{digest(prior): true}}}

	ledger := NewLedger()
	ledger.Record(".mcp.json", OwnedKey{ID: mcpKeyID([]string{"mcpServers", "fathom"}), Digest: "sha256:old", Wrote: "2026-09-28"})
	ledger.Record(skillLabel, OwnedKey{ID: skillKeyID, Digest: digest("an even older render"), Wrote: "2026-09-28"})
	if err := SaveLedger(root, ledger); err != nil {
		t.Fatal(err)
	}

	recorded, refused, err := RecordBaselinesDetail(root, mcp, skills, "2026-10-09", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(recorded, ";") != "mcp .mcp.json mcpServers.fathom" {
		t.Errorf("recorded = %v, want only the MCP entry", recorded)
	}
	if len(refused) != 1 || !strings.HasPrefix(refused[0], "skill "+skillLabel) || !strings.Contains(refused[0], "not a render fathom shipped") {
		t.Errorf("refused = %v, want the edited skill named", refused)
	}
	ledger, _ = LoadLedger(root)
	if rep := DriftSkills(ledger, skills, nil); len(rep.Drifts) != 1 {
		t.Errorf("the refused skill must stay DRIFT: %+v", rep)
	}

	// The file goes back to a known prior render: re-recorded, even
	// without --rebaseline.
	writeFile(t, root, skillLabel, prior)
	recorded, refused, err = RecordBaselinesDetail(root, mcp, skills, "2026-10-09", nil, false)
	if err != nil || strings.Join(recorded, ";") != "skill "+skillLabel || len(refused) != 0 {
		t.Fatalf("recorded = %v refused = %v err = %v; want the known render re-recorded", recorded, refused, err)
	}
}

// RecordWritten is the writer's own record (fathom init --skills): after
// the write, each file's skill baseline and, for a repository file, its
// owned-file baseline are the digest of the bytes on disk — only when they
// are the render fathom ships.
func TestRecordWrittenBaselinesFromDisk(t *testing.T) {
	root := t.TempDir()
	cur := testMarker + "\ncurrent\n"
	path := writeFile(t, root, skillLabel, cur)
	home := t.TempDir()
	promptPath := writeFile(t, home, ".codex/prompts/fathom-x.md", cur)
	foreign := writeFile(t, root, ".claude/skills/fathom-y/SKILL.md", testMarker+"\nraced by a hand edit\n")
	files := []SkillFile{
		{Path: path, Label: skillLabel, Shipped: []byte(cur)},
		{Path: promptPath, Label: "~/.codex/prompts/fathom-x.md", Shipped: []byte(cur)},
		{Path: foreign, Label: ".claude/skills/fathom-y/SKILL.md", Shipped: []byte(cur)},
	}
	recorded, err := RecordWritten(root, files, "2026-10-09T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(recorded, ";") != "skill .claude/skills/fathom-x/SKILL.md;skill ~/.codex/prompts/fathom-x.md" {
		t.Errorf("recorded = %v", recorded)
	}
	ledger, _ := LoadLedger(root)
	for _, id := range []string{ownedFileID, skillKeyID} {
		if k, ok := ownedKey(ledger, skillLabel, id); !ok || k.Digest != digest(cur) {
			t.Errorf("%s %s = %+v, %v", skillLabel, id, k, ok)
		}
	}
	if _, ok := ownedKey(ledger, "~/.codex/prompts/fathom-x.md", ownedFileID); ok {
		t.Error("a user-scope prompt got an owned-file id: the planner never writes it")
	}
	if _, ok := ownedKey(ledger, ".claude/skills/fathom-y/SKILL.md", skillKeyID); ok {
		t.Error("bytes that are not fathom's render were recorded as written")
	}
}
