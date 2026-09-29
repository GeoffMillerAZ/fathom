package wiring

// agentsmd_test.go — ADR-059's engine additions (docs/plans/agents-md.md):
// AG-5 Region.Remove, AG-7 Link, and the json-value key Gemini CLI's
// context.fileName rides (AG-6). Every assertion is law 15's: a removal or
// a link is planned only over what fathom can prove it wrote, the receipt
// carries enough to put everything back, and a stop leaves the bytes alone.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRegionRemoveDeletesOwnedRegionAndUndoRestoresIt pins AG-5: a tracked,
// undrifted region is deleted with every other byte kept, the receipt holds
// the removed text, and `fleet undo` restores the file exactly.
func TestRegionRemoveDeletesOwnedRegionAndUndoRestoresIt(t *testing.T) {
	root := t.TempDir()
	prior := "# demo\n\n" + "@AGENTS.md\n\n" + renderRegion(HTMLMarkers, "v3", stanzaBody, "\n") + "\n"
	write(t, root, "CLAUDE.md", prior)
	spec := Spec{Regions: []Region{{File: "CLAUDE.md", Syntax: HTMLMarkers, Version: "v3", Remove: true}}}

	plan, err := PlanWiring(root, spec)
	if err != nil {
		t.Fatal(err)
	}
	it := itemFor(t, plan, "CLAUDE.md", "fathom:region")
	if it.Verdict != VerdictApply || it.Before != stanzaBody || it.After != "" {
		t.Fatalf("removal item = %+v, want apply with the body as Before and nothing After", it)
	}
	res, err := ApplyWiring(root, spec, ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, root, "CLAUDE.md"), "# demo\n\n"+"@AGENTS.md\n"; got != want {
		t.Fatalf("after removal CLAUDE.md = %q, want %q", got, want)
	}
	if res.Receipt == nil || len(res.Receipt.Files) != 1 || res.Receipt.Files[0].Before != prior {
		t.Fatalf("receipt does not carry the prior bytes: %+v", res.Receipt)
	}
	again, _ := PlanWiring(root, spec)
	if v := itemFor(t, again, "CLAUDE.md", "fathom:region").Verdict; v != VerdictSkip {
		t.Errorf("second plan verdict = %s, want skip (nothing left to remove)", v)
	}
	if _, err := Undo(root, res.Receipt.ID, ApplyOptions{Now: fixedClock()}); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := read(t, root, "CLAUDE.md"); got != prior {
		t.Fatalf("undo did not restore the region:\n got %q\nwant %q", got, prior)
	}
}

// TestRegionRemoveBlocksOnDrift pins law 15 for removals: a tracked region
// a human edited is never deleted.
func TestRegionRemoveBlocksOnDrift(t *testing.T) {
	root := t.TempDir()
	region := renderRegion(HTMLMarkers, "v3", stanzaBody, "\n")
	edited := strings.Replace(region, "Indexed as", "Hand-edited: indexed as", 1)
	prior := "# demo\n\n" + edited + "\n"
	write(t, root, "CLAUDE.md", prior)
	spec := Spec{Regions: []Region{{File: "CLAUDE.md", Syntax: HTMLMarkers, Version: "v3", Remove: true, AdoptUntracked: true}}}

	res, err := ApplyWiring(root, spec, ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Blocked) != 1 || res.Blocked[0].Cause != CauseDrift {
		t.Fatalf("blocked = %+v, want one drift", res.Blocked)
	}
	if got := read(t, root, "CLAUDE.md"); got != prior {
		t.Fatalf("a drifted region was touched:\n got %q\nwant %q", got, prior)
	}
}

// TestRegionRemoveSkipsAbsentFileAndRegion: nothing to remove is skip, and
// Remove never creates the file.
func TestRegionRemoveSkipsAbsentFileAndRegion(t *testing.T) {
	root := t.TempDir()
	spec := Spec{Regions: []Region{{File: "CLAUDE.md", Syntax: HTMLMarkers, Version: "v3", Remove: true}}}
	plan, _ := PlanWiring(root, spec)
	if v := itemFor(t, plan, "CLAUDE.md", "fathom:region").Verdict; v != VerdictSkip {
		t.Fatalf("absent file verdict = %s, want skip", v)
	}
	write(t, root, "CLAUDE.md", "# only prose\n")
	plan, _ = PlanWiring(root, spec)
	if v := itemFor(t, plan, "CLAUDE.md", "fathom:region").Verdict; v != VerdictSkip {
		t.Fatalf("no-region verdict = %s, want skip", v)
	}
	if _, err := ApplyWiring(root, spec, ApplyOptions{Now: fixedClock()}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, root, "CLAUDE.md"); got != "# only prose\n" {
		t.Fatalf("file changed: %q", got)
	}
}

func skillOwned(rel, body string) OwnedFile {
	return OwnedFile{File: rel, Marker: "<!-- fathom:owned", Body: body}
}

const ownedSkill = "---\nname: demo\ndescription: d\n---\n<!-- fathom:owned (delete this line to adopt) -->\nbody\n"

func linkSpec() Spec {
	return Spec{Links: []Link{{
		Path:   ".agents/skills",
		Target: "../.claude/skills",
		Owned:  []OwnedFile{skillOwned(".agents/skills/demo/SKILL.md", ownedSkill)},
	}}}
}

// TestLinkCreateSkipAndBlock pins AG-7's three verdicts: absent is created
// as a relative link, the same target is skip, anything else is blocked.
func TestLinkCreateSkipAndBlock(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".claude/skills/demo/SKILL.md", ownedSkill)

	res, err := ApplyWiring(root, linkSpec(), ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	if isLink, target := LinkState(root, ".agents/skills"); !isLink || target != "../.claude/skills" {
		t.Fatalf("link state = %v %q, want a link to ../.claude/skills", isLink, target)
	}
	if got := read(t, root, ".agents/skills/demo/SKILL.md"); got != ownedSkill {
		t.Fatalf("the link does not resolve to the canonical copy: %q", got)
	}
	if res.Receipt == nil {
		t.Fatal("no receipt for a created link")
	}
	plan, _ := PlanWiring(root, linkSpec())
	if v := itemFor(t, plan, ".agents/skills", "fathom:link").Verdict; v != VerdictSkip {
		t.Fatalf("same-target verdict = %s, want skip", v)
	}

	// A link to anywhere else is someone else's.
	other := t.TempDir()
	if err := os.Symlink("../elsewhere", filepath.Join(other, "skills")); err != nil {
		t.Fatal(err)
	}
	spec := Spec{Links: []Link{{Path: "skills", Target: "../.claude/skills"}}}
	plan, _ = PlanWiring(other, spec)
	if it := itemFor(t, plan, "skills", "fathom:link"); it.Verdict != VerdictBlocked || it.Cause != CauseUnproven {
		t.Fatalf("foreign link = %+v, want blocked unproven", it)
	}

	// Undo removes the link and nothing else.
	if _, err := Undo(root, res.Receipt.ID, ApplyOptions{Now: fixedClock()}); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".agents", "skills")); !os.IsNotExist(err) {
		t.Fatalf("undo left the link: %v", err)
	}
	if got := read(t, root, ".claude/skills/demo/SKILL.md"); got != ownedSkill {
		t.Fatalf("undo touched the canonical copy: %q", got)
	}
}

// TestLinkBlocksADirectoryWithForeignEntries: a real directory holding one
// file fathom did not write blocks the link and names the file; nothing
// is removed.
func TestLinkBlocksADirectoryWithForeignEntries(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".claude/skills/demo/SKILL.md", ownedSkill)
	write(t, root, ".agents/skills/demo/SKILL.md", ownedSkill)
	write(t, root, ".agents/skills/mine/SKILL.md", "---\nname: mine\n---\nhand-written\n")

	res, err := ApplyWiring(root, linkSpec(), ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Blocked) != 1 || !strings.Contains(res.Blocked[0].Reason, ".agents/skills/mine/SKILL.md") {
		t.Fatalf("blocked = %+v, want the link blocked naming the foreign file", res.Blocked)
	}
	if isLink, _ := LinkState(root, ".agents/skills"); isLink {
		t.Fatal("a directory with a foreign entry was replaced")
	}
	if got := read(t, root, ".agents/skills/demo/SKILL.md"); got != ownedSkill {
		t.Fatal("an owned file was removed from a blocked directory")
	}
}

// TestLinkReplacesFathomsOwnRenderingAndUndoRestoresIt pins the migration:
// a real directory holding only fathom-owned, unedited files is replaced by
// the link, and undo restores the directory as real files — never through
// the link into the canonical copy.
func TestLinkReplacesFathomsOwnRenderingAndUndoRestoresIt(t *testing.T) {
	root := t.TempDir()
	canonical := strings.Replace(ownedSkill, "body", "canonical body", 1)
	write(t, root, ".claude/skills/demo/SKILL.md", canonical)
	write(t, root, ".agents/skills/demo/SKILL.md", ownedSkill)

	plan, _ := PlanWiring(root, linkSpec())
	if v := itemFor(t, plan, ".agents/skills", "fathom:link").Verdict; v != VerdictApply {
		t.Fatalf("migration verdict = %s, want apply", v)
	}
	res, err := ApplyWiring(root, linkSpec(), ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	if isLink, _ := LinkState(root, ".agents/skills"); !isLink {
		t.Fatal("the owned rendering was not replaced by the link")
	}
	if len(res.Receipt.Files) != 2 {
		t.Fatalf("receipt files = %+v, want the removed file and the link", res.Receipt.Files)
	}

	undo, err := Undo(root, res.Receipt.ID, ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatalf("Undo: %v (refused %+v)", err, undo.Refused)
	}
	if isLink, _ := LinkState(root, ".agents/skills"); isLink {
		t.Fatal("undo left the link")
	}
	if got := read(t, root, ".agents/skills/demo/SKILL.md"); got != ownedSkill {
		t.Fatalf("undo did not restore the rendered copy: %q", got)
	}
	if got := read(t, root, ".claude/skills/demo/SKILL.md"); got != canonical {
		t.Fatalf("undo wrote through the link into the canonical copy: %q", got)
	}

	// The undo is itself undoable: the link comes back.
	if _, err := Undo(root, undo.Receipted.ID, ApplyOptions{Now: fixedClock()}); err != nil {
		t.Fatalf("undo of the undo: %v", err)
	}
	if isLink, target := LinkState(root, ".agents/skills"); !isLink || target != "../.claude/skills" {
		t.Fatalf("undo of the undo: link state %v %q", isLink, target)
	}
}

// TestLinkSpecRefusesAWriterThroughTheLink: no owned file may sit under a
// link in the same spec.
func TestLinkSpecRefusesAWriterThroughTheLink(t *testing.T) {
	spec := linkSpec()
	spec.Files = []OwnedFile{skillOwned(".agents/skills/demo/SKILL.md", ownedSkill)}
	if _, err := PlanWiring(t.TempDir(), spec); err == nil {
		t.Fatal("a spec writing through its own link was accepted")
	}
}

// TestJSONValueKeySetsOneValue pins the json-value key: one value at a key
// path, every other key kept, idempotent, and a hand edit after fathom
// recorded it is drift.
func TestJSONValueKeySetsOneValue(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".gemini/settings.json", "{\"theme\": \"dark\", \"context\": {\"other\": 1}}\n")
	k := Key{File: ".gemini/settings.json", Kind: KindJSONValue, Path: []string{"context", "fileName"}, Body: `["AGENTS.md","GEMINI.md"]`}
	spec := Spec{Keys: []Key{k}}
	if _, err := ApplyWiring(root, spec, ApplyOptions{Now: fixedClock()}); err != nil {
		t.Fatal(err)
	}
	got := read(t, root, ".gemini/settings.json")
	for _, want := range []string{`"theme": "dark"`, `"other": 1`, `"fileName": [`, `"AGENTS.md"`, `"GEMINI.md"`} {
		if !strings.Contains(got, want) {
			t.Errorf("settings missing %s:\n%s", want, got)
		}
	}
	plan, _ := PlanWiring(root, spec)
	if v := itemFor(t, plan, ".gemini/settings.json", "context.fileName").Verdict; v != VerdictSkip {
		t.Fatalf("second plan = %s, want skip", v)
	}
	write(t, root, ".gemini/settings.json", strings.Replace(got, `"GEMINI.md"`, `"OTHER.md"`, 1))
	plan, _ = PlanWiring(root, spec)
	if it := itemFor(t, plan, ".gemini/settings.json", "context.fileName"); it.Verdict != VerdictBlocked || it.Cause != CauseDrift {
		t.Fatalf("hand-edited value = %+v, want blocked drift", it)
	}
}
