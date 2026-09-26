package wiring

import "testing"

// .fathom/owned.json is per checkout (gitignored). When fathom's own
// template moves in git — a hook's timeout, a skill file's body — the
// checkout holds exactly what fathom would write while its recorded digest
// still names the older release. That is current, not drift: reading it as
// drift made `config audit` fail in the operator's checkout on six hook
// rows the project's own commits had updated (OWN-4). An entry that
// matches neither the recorded digest nor the current template still
// drifts (TestRecordKeyBaselines, TestOwnedFileBlocksHandEditUnderMarkerAsDrift).
func TestPlan_CurrentTemplateWithLaggingLedgerIsNotDrift(t *testing.T) {
	t.Run("hook key", func(t *testing.T) {
		root := t.TempDir()
		rel := ".claude/settings.json"
		old := `{"hooks":{"PostToolUse":[{"matcher":"Edit|Write|MultiEdit","hooks":[{"type":"command","command":"fathom edit-hook","timeout":5}]}]}}`
		key := Key{
			File: rel, Kind: KindJSONHook,
			Path:   []string{"PostToolUse", "Edit|Write|MultiEdit"},
			Marker: "fathom edit-hook", Body: "fathom edit-hook", Timeout: 5,
		}
		cur, _, err := key.current(old)
		if err != nil {
			t.Fatal(err)
		}
		ledger := NewLedger()
		ledger.Record(rel, OwnedKey{ID: key.ID(), Digest: digest(cur), Wrote: "2026-09-12"})
		if err := SaveLedger(root, ledger); err != nil {
			t.Fatal(err)
		}

		// The template now ships timeout 45, and the checkout carries it.
		key.Timeout = 45
		write(t, root, rel, `{"hooks":{"PostToolUse":[{"matcher":"Edit|Write|MultiEdit","hooks":[{"type":"command","command":"fathom edit-hook","timeout":45}]}]}}`)
		p, err := PlanWiring(root, Spec{Keys: []Key{key}})
		if err != nil {
			t.Fatal(err)
		}
		if it := itemFor(t, p, rel, key.ID()); it.Verdict != VerdictSkip {
			t.Fatalf("an entry equal to the current template read as %s/%s: %s", it.Verdict, it.Cause, it.Reason)
		}

		// A caller that adopts the file's timeout (reconcile keeps an
		// operator's timeout) still sees a non-template value as drift.
		write(t, root, rel, `{"hooks":{"PostToolUse":[{"matcher":"Edit|Write|MultiEdit","hooks":[{"type":"command","command":"fathom edit-hook","timeout":82}]}]}}`)
		adopted := key
		adopted.ShippedTimeout, adopted.Timeout = 45, 82
		p, err = PlanWiring(root, Spec{Keys: []Key{adopted}})
		if err != nil {
			t.Fatal(err)
		}
		if it := itemFor(t, p, rel, adopted.ID()); it.Verdict != VerdictBlocked || it.Cause != CauseDrift {
			t.Fatalf("a hand-set timeout read as %s/%s: %s", it.Verdict, it.Cause, it.Reason)
		}
	})

	t.Run("owned file", func(t *testing.T) {
		root := t.TempDir()
		v1 := "---\nname: fathom-x\n---\n" + testMarker + "\nv1\n"
		if _, err := ApplyWiring(root, skillSpec(v1), ApplyOptions{Now: fixedClock()}); err != nil {
			t.Fatal(err)
		}
		v2 := "---\nname: fathom-x\n---\n" + testMarker + "\nv2\n"
		write(t, root, ".agents/skills/fathom-x/SKILL.md", v2)
		p, err := PlanWiring(root, skillSpec(v2))
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Items) != 1 || p.Items[0].Verdict != VerdictSkip {
			t.Fatalf("a file equal to the shipped source read as drift: %+v", p.Items)
		}
	})
}
