package wiring

// ownedfile_test.go — RP-4 (docs/plans/reach-parity.md): a whole owned
// file rewrites only under its marker, a hand-authored file by the same
// name is BLOCKED by name, a hand edit under the marker is BLOCKED as
// drift, and every write is receipted and undoable.

import (
	"strings"
	"testing"
)

const testMarker = "<!-- fathom:owned -->"

func skillSpec(body string) Spec {
	return Spec{Files: []OwnedFile{{
		File:   ".agents/skills/fathom-x/SKILL.md",
		Marker: testMarker,
		Body:   body,
	}}}
}

func TestOwnedFileCreatesRewritesAndUndoes(t *testing.T) {
	root := t.TempDir()
	v1 := "---\nname: fathom-x\n---\n" + testMarker + "\nv1\n"
	v2 := "---\nname: fathom-x\n---\n" + testMarker + "\nv2\n"

	// Absent → created, receipted.
	res, err := ApplyWiring(root, skillSpec(v1), ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != 1 || res.Applied[0].Kind != KindFile || res.Receipt == nil {
		t.Fatalf("create: applied=%v receipt=%v", res.Applied, res.Receipt)
	}
	if got := read(t, root, ".agents/skills/fathom-x/SKILL.md"); got != v1 {
		t.Fatalf("created content = %q", got)
	}
	// The ledger records the digest fathom wrote.
	ledger, lerr := LoadLedger(root)
	if lerr != nil {
		t.Fatal(lerr)
	}
	if k, ok := ownedKey(ledger, ".agents/skills/fathom-x/SKILL.md", ownedFileID); !ok || k.Digest != digest(v1) {
		t.Fatalf("ledger = %+v, %v", k, ok)
	}

	// Same bytes → skip, nothing written, no receipt.
	res, err = ApplyWiring(root, skillSpec(v1), ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != 0 || res.Receipt != nil || len(res.Skipped) != 1 || res.Skipped[0].Reason != "already current" {
		t.Fatalf("idempotent: %+v", res)
	}

	// A newer template rewrites the marker-bearing file.
	res, err = ApplyWiring(root, skillSpec(v2), ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != 1 || read(t, root, ".agents/skills/fathom-x/SKILL.md") != v2 {
		t.Fatalf("rewrite: %+v", res)
	}
	id := res.Receipt.ID

	// Undo restores v1 exactly.
	if _, err := Undo(root, id, ApplyOptions{Now: fixedClock()}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, root, ".agents/skills/fathom-x/SKILL.md"); got != v1 {
		t.Fatalf("after undo = %q, want v1", got)
	}
}

func TestOwnedFileBlocksHandAuthoredCopyByName(t *testing.T) {
	root := t.TempDir()
	mine := "---\nname: fathom-x\ndescription: my own skill\n---\nmine\n"
	write(t, root, ".agents/skills/fathom-x/SKILL.md", mine)

	// A second, absent file in the same spec still applies around the block.
	spec := skillSpec("---\nname: fathom-x\n---\n" + testMarker + "\nfathom's\n")
	spec.Files = append(spec.Files, OwnedFile{File: ".agents/skills/fathom-y/SKILL.md", Marker: testMarker, Body: testMarker + "\ny\n"})

	res, err := ApplyWiring(root, spec, ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Blocked) != 1 || res.Blocked[0].Cause != CauseUnproven || !strings.Contains(res.Blocked[0].Reason, ".agents/skills/fathom-x/SKILL.md") {
		t.Fatalf("blocked = %+v", res.Blocked)
	}
	if got := read(t, root, ".agents/skills/fathom-x/SKILL.md"); got != mine {
		t.Fatalf("the hand-authored file was touched: %q", got)
	}
	if len(res.Applied) != 1 || res.Applied[0].File != ".agents/skills/fathom-y/SKILL.md" {
		t.Fatalf("the other file must still apply: %+v", res.Applied)
	}
}

func TestOwnedFileBlocksHandEditUnderMarkerAsDrift(t *testing.T) {
	root := t.TempDir()
	v1 := "---\nname: fathom-x\n---\n" + testMarker + "\nv1\n"
	if _, err := ApplyWiring(root, skillSpec(v1), ApplyOptions{Now: fixedClock()}); err != nil {
		t.Fatal(err)
	}
	edited := v1 + "my note under the marker\n"
	write(t, root, ".agents/skills/fathom-x/SKILL.md", edited)

	plan, err := PlanWiring(root, skillSpec("---\nname: fathom-x\n---\n"+testMarker+"\nv2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 || plan.Items[0].Verdict != VerdictBlocked || plan.Items[0].Cause != CauseDrift {
		t.Fatalf("plan = %+v", plan.Items)
	}
	res, err := ApplyWiring(root, skillSpec("---\nname: fathom-x\n---\n"+testMarker+"\nv2\n"), ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Receipt != nil || read(t, root, ".agents/skills/fathom-x/SKILL.md") != edited {
		t.Fatalf("a drifted file must stay byte-identical: %+v", res)
	}

	// A marker-bearing file fathom never recorded (pre-engine install) is
	// adopted by its marker, not reported as drift.
	root2 := t.TempDir()
	write(t, root2, ".agents/skills/fathom-x/SKILL.md", v1)
	plan, err = PlanWiring(root2, skillSpec("---\nname: fathom-x\n---\n"+testMarker+"\nv2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Items[0].Verdict != VerdictApply {
		t.Fatalf("unrecorded marker-bearing file must be adopted: %+v", plan.Items[0])
	}
}
