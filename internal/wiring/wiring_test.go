package wiring

// wiring_test.go — phase U, rows SH-1..SH-5 of docs/plans/ambient-fathom.md.
//
// Every test here is an instance of law 15's asymmetry: a false stop costs a
// human one diff, a false merge silently destroys a configuration fathom was
// never invited to own. So the assertions are all one-sided — the file on
// disk must be byte-identical whenever fathom is not certain.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedClock() func() time.Time {
	at := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return at }
}

func write(t *testing.T, root, rel, content string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return p
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// stanzaBody stands in for the CLAUDE.md operating contract: the engine is
// indifferent to its content, and a neutral fixture keeps ADR-011.
const stanzaBody = "## fathom code intelligence\n\nIndexed as service \"demo\"."

const stanzaBodyV2 = "## fathom code intelligence\n\nIndexed as service \"demo\" (older wording)."

func demoSpec() Spec {
	return Spec{
		Regions: []Region{{
			File:    "CLAUDE.md",
			Syntax:  HTMLMarkers,
			Version: "v3",
			Body:    stanzaBody,
			Known:   []string{stanzaBodyV2},
		}},
		Keys: []Key{
			{
				File: "lefthook.yml",
				Kind: KindYAMLCommand,
				Path: []string{"pre-push", "commands", "fathom-verify"},
				Body: "run: fathom verify --service demo",
			},
			{
				File:    ".claude/settings.json",
				Kind:    KindJSONHook,
				Path:    []string{"PostToolUse", "Edit|Write|MultiEdit"},
				Marker:  "fathom edit-hook",
				Body:    "fathom edit-hook",
				Timeout: 5,
			},
		},
	}
}

func itemFor(t *testing.T, p Plan, file, target string) Item {
	t.Helper()
	for _, it := range p.Items {
		if it.File == file && it.Target == target {
			return it
		}
	}
	t.Fatalf("no plan item for %s / %s; items: %+v", file, target, p.Items)
	return Item{}
}

// ─── SH-1 ────────────────────────────────────────────────────────────────

// TestOwnedRegionDigestDetectsHandEdit pins marker format v3: the begin
// marker carries the template version AND the sha256 of the body as fathom
// last wrote it, so a hand edit inside fathom's own region is detectable
// with no history, no daemon and no network.
func TestOwnedRegionDigestDetectsHandEdit(t *testing.T) {
	region := renderRegion(HTMLMarkers, "v3", stanzaBody, "\n")
	content := "# demo\n\nprose above\n\n" + region + "\n\nprose below\n"

	got, err := parseOwnedRegion(content, HTMLMarkers)
	if err != nil {
		t.Fatalf("parseOwnedRegion: %v", err)
	}
	if !got.Present {
		t.Fatal("region not found in content that carries one")
	}
	if got.Version != "v3" {
		t.Errorf("version = %q, want v3", got.Version)
	}
	if got.Digest == "" {
		t.Fatal("v3 marker recorded no digest — the drift detector has nothing to compare against")
	}
	if got.Digest != got.BodyDigest {
		t.Errorf("freshly written region already reads as drifted: recorded %s, body %s", got.Digest, got.BodyDigest)
	}
	if got.Drifted() {
		t.Error("freshly written region reports Drifted()")
	}
	if got.Body != stanzaBody {
		t.Errorf("body round-trip broken:\n got %q\nwant %q", got.Body, stanzaBody)
	}

	// A human edits one word INSIDE fathom's region.
	edited := strings.Replace(content, "Indexed as service", "Indexed (by hand) as service", 1)
	if edited == content {
		t.Fatal("fixture did not change — the hand-edit probe is vacuous")
	}
	got2, err := parseOwnedRegion(edited, HTMLMarkers)
	if err != nil {
		t.Fatalf("parseOwnedRegion(edited): %v", err)
	}
	if !got2.Drifted() {
		t.Fatal("a hand edit inside fathom's region was NOT detected as drift — law 15 cannot be enforced")
	}
	if got2.Digest == got2.BodyDigest {
		t.Error("digest comparison is vacuous: recorded == recomputed after an edit")
	}
	if !strings.Contains(got2.Body, "by hand") {
		t.Error("the human's version is not carried out of the parse — BLOCKED could not show both versions")
	}

	// A legacy (pre-v3) marker records no digest: unknown provenance, which
	// is NOT the same as drift. Absence is named, never guessed (law 3).
	legacy := "# demo\n\n<!-- fathom:begin -->\n" + stanzaBody + "\n<!-- fathom:end -->\n"
	got3, err := parseOwnedRegion(legacy, HTMLMarkers)
	if err != nil {
		t.Fatalf("parseOwnedRegion(legacy): %v", err)
	}
	if !got3.Present || got3.Tracked() {
		t.Errorf("legacy marker: Present=%v Tracked=%v, want true/false", got3.Present, got3.Tracked())
	}
	if got3.Drifted() {
		t.Error("legacy marker reported as drifted — a missing digest is unknown provenance, not evidence of an edit")
	}

	// Corruption is refused, never guessed at.
	for _, tc := range []struct {
		name    string
		content string
		wantErr bool
	}{
		{"begin only", "x\n<!-- fathom:begin v3 sha256:aa -->\nbody\n", true},
		{"end only", "x\n" + stanzaBody + "\n<!-- fathom:end -->\n", true},
		{"reversed", "<!-- fathom:end -->\nbody\n<!-- fathom:begin v3 sha256:aa -->\n", true},
		{"two begins", region + "\n" + region + "\n", true},
		{"unterminated begin line", "<!-- fathom:begin v3 sha256:aa -->", true},
		{"neither marker", "# demo\n\nnothing to see\n", false},
	} {
		_, err := parseOwnedRegion(tc.content, HTMLMarkers)
		if tc.wantErr && err == nil {
			t.Errorf("%s: parse accepted a corrupted marker pair", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("%s: unexpected error %v (an unwired file is normal, not a defect)", tc.name, err)
		}
	}
}

// ─── SH-2 ────────────────────────────────────────────────────────────────

// TestOwnedKeysBoundsStructuredEdits pins the ledger: fathom touches only
// keys .fathom/owned.json says it added. A key carrying fathom's own NAME
// that the ledger does not record is someone else's, and is invisible.
func TestOwnedKeysBoundsStructuredEdits(t *testing.T) {
	l := NewLedger()
	l.Record("lefthook.yml", OwnedKey{ID: "pre-push.commands.fathom-verify", Digest: "sha256:aa", Wrote: "2026-08-08T12:00:00Z"})
	l.Record("lefthook.yml", OwnedKey{ID: "pre-commit.commands.fathom-fmt", Digest: "sha256:bb", Wrote: "2026-08-08T12:00:00Z"})
	l.Record(".claude/settings.json", OwnedKey{ID: "Stop||fathom stop-hook", Digest: "sha256:cc", Wrote: "2026-08-08T12:00:00Z"})

	keys := OwnedKeys(l, "lefthook.yml")
	if len(keys) != 2 {
		t.Fatalf("OwnedKeys(lefthook.yml) = %d keys, want 2", len(keys))
	}
	if keys[0].ID > keys[1].ID {
		t.Errorf("OwnedKeys is not sorted: %q then %q", keys[0].ID, keys[1].ID)
	}
	if got := OwnedKeys(l, "Taskfile.yml"); len(got) != 0 {
		t.Errorf("OwnedKeys for an unrecorded file = %v, want empty", got)
	}

	// The bounding property, end to end. A repository already carries a
	// command with fathom's exact name, written by somebody else.
	root := t.TempDir()
	write(t, root, "lefthook.yml", "pre-push:\n  commands:\n    fathom-verify:\n      run: ./scripts/my-own-verify.sh\n")
	spec := Spec{Keys: demoSpec().Keys[:1]}

	plan, err := PlanWiring(root, spec)
	if err != nil {
		t.Fatalf("PlanWiring: %v", err)
	}
	it := itemFor(t, plan, "lefthook.yml", "pre-push.commands.fathom-verify")
	if it.Verdict != VerdictSkip {
		t.Fatalf("unledgered key verdict = %s (%s), want skip — fathom claimed a key it never wrote", it.Verdict, it.Reason)
	}

	before := read(t, root, "lefthook.yml")
	if _, err := ApplyWiring(root, spec, ApplyOptions{Now: fixedClock()}); err != nil {
		t.Fatalf("ApplyWiring: %v", err)
	}
	if after := read(t, root, "lefthook.yml"); after != before {
		t.Fatalf("fathom rewrote a key it does not own:\n--- before\n%s\n--- after\n%s", before, after)
	}

	// Now record it as fathom's. The SAME key becomes fathom's to update.
	led, err := LoadLedger(root)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	led.Record("lefthook.yml", OwnedKey{ID: "pre-push.commands.fathom-verify", Digest: digest("run: ./scripts/my-own-verify.sh"), Wrote: "2026-08-08T12:00:00Z"})
	if err := SaveLedger(root, led); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}
	plan2, err := PlanWiring(root, spec)
	if err != nil {
		t.Fatalf("PlanWiring (ledgered): %v", err)
	}
	if it := itemFor(t, plan2, "lefthook.yml", "pre-push.commands.fathom-verify"); it.Verdict != VerdictApply {
		t.Fatalf("ledgered key verdict = %s (%s), want apply", it.Verdict, it.Reason)
	}
}

// ─── SH-3 ────────────────────────────────────────────────────────────────

// TestDriftStopsAndReportsNeverMerges is law 15 itself: a hand-edited region
// is reported with BOTH versions and the file is byte-identical afterwards,
// while every other item in the SAME run still applies.
func TestDriftStopsAndReportsNeverMerges(t *testing.T) {
	root := t.TempDir()
	region := renderRegion(HTMLMarkers, "v3", stanzaBody, "\n")
	drifted := strings.Replace(region, "Indexed as service", "Indexed as service (our house rule: never auto-ingest)", 1)
	claude := "# demo repo\n\nhouse notes\n\n" + drifted + "\n\ntrailing notes\n"
	write(t, root, "CLAUDE.md", claude)
	write(t, root, "lefthook.yml", "pre-commit:\n  commands:\n    fmt:\n      run: gofmt -l .\n")

	spec := demoSpec()
	plan, err := PlanWiring(root, spec)
	if err != nil {
		t.Fatalf("PlanWiring: %v", err)
	}

	blocked := plan.Blocked()
	if len(blocked) != 1 {
		t.Fatalf("blocked items = %d, want exactly 1: %+v", len(blocked), blocked)
	}
	b := blocked[0]
	if b.File != "CLAUDE.md" {
		t.Errorf("blocked file = %s, want CLAUDE.md", b.File)
	}
	if !strings.Contains(b.Before, "house rule") {
		t.Error("BLOCKED item does not carry the human's version — a human cannot choose between versions they cannot see")
	}
	if !strings.Contains(b.After, "Indexed as service \"demo\".") {
		t.Error("BLOCKED item does not carry fathom's version")
	}
	for _, it := range plan.Items {
		if it.Reason == "" {
			t.Errorf("item %s/%s carries no reason (law 3)", it.File, it.Target)
		}
	}

	res, err := ApplyWiring(root, spec, ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatalf("ApplyWiring: %v", err)
	}
	if len(res.Blocked) != 1 {
		t.Fatalf("apply reported %d blocked, want 1", len(res.Blocked))
	}
	if got := read(t, root, "CLAUDE.md"); got != claude {
		t.Fatalf("fathom MERGED a drifted region — the file changed:\n--- want\n%s\n--- got\n%s", claude, got)
	}

	// One BLOCKED item does not block the others in the same run.
	if len(res.Applied) != 2 {
		t.Fatalf("applied %d items, want 2 (the lefthook command and the settings hook); one BLOCKED item must not stop them", len(res.Applied))
	}
	if !strings.Contains(read(t, root, "lefthook.yml"), "fathom-verify") {
		t.Error("the un-drifted lefthook item did not apply alongside a BLOCKED one")
	}
	if !strings.Contains(read(t, root, ".claude/settings.json"), "fathom edit-hook") {
		t.Error("the un-drifted settings item did not apply alongside a BLOCKED one")
	}
}

// ─── SH-4 ────────────────────────────────────────────────────────────────

// TestApplyPreservesForeignContent: everything outside fathom's regions and
// keys survives byte-for-byte, line endings included, and other tools' YAML
// commands keep their order.
func TestApplyPreservesForeignContent(t *testing.T) {
	t.Run("bytes outside the region, CRLF and trailing whitespace", func(t *testing.T) {
		root := t.TempDir()
		prefix := "# demo\r\n\r\nhouse prose with trailing spaces   \r\n\r\n"
		suffix := "\r\n\r\nmore prose\t\r\nlast line without newline"
		old := renderRegion(HTMLMarkers, "v2", stanzaBodyV2, "\r\n")
		write(t, root, "CLAUDE.md", prefix+old+suffix)

		res, err := ApplyWiring(root, demoSpec(), ApplyOptions{Now: fixedClock()})
		if err != nil {
			t.Fatalf("ApplyWiring: %v", err)
		}
		if len(res.Blocked) != 0 {
			t.Fatalf("unexpected BLOCKED: %+v", res.Blocked)
		}
		got := read(t, root, "CLAUDE.md")
		if !strings.HasPrefix(got, prefix) {
			t.Errorf("content before the region was rewritten:\n got %q\nwant prefix %q", got, prefix)
		}
		if !strings.HasSuffix(got, suffix) {
			t.Errorf("content after the region was rewritten:\n got %q\nwant suffix %q", got, suffix)
		}
		if !strings.Contains(got, strings.ReplaceAll(stanzaBody, "\n", "\r\n")) {
			t.Errorf("the upgrade did not land with the file's own line endings:\n%q", got)
		}
	})

	t.Run("appending into an unwired file keeps every existing byte", func(t *testing.T) {
		for name, prior := range map[string]string{
			"prose ending in one newline":    "# demo\n\nhouse prose\n",
			"prose ending in two newlines":   "# demo\n\nhouse prose\n\n",
			"prose with no trailing newline": "# demo\n\nhouse prose",
			"whitespace only":                "   \n\t\n",
			"empty":                          "",
		} {
			root := t.TempDir()
			write(t, root, "CLAUDE.md", prior)
			if _, err := ApplyWiring(root, Spec{Regions: demoSpec().Regions}, ApplyOptions{Now: fixedClock()}); err != nil {
				t.Fatalf("%s: ApplyWiring: %v", name, err)
			}
			got := read(t, root, "CLAUDE.md")
			if !strings.HasPrefix(got, prior) {
				t.Errorf("%s: existing bytes were rewritten:\n got %q\nwant prefix %q", name, got, prior)
			}
			if !strings.Contains(got, stanzaBody) {
				t.Errorf("%s: the region was not appended:\n%q", name, got)
			}
		}
	})

	t.Run("other tools' yaml commands keep their order", func(t *testing.T) {
		root := t.TempDir()
		yml := "pre-commit:\n  commands:\n    fmt:\n      run: gofmt -l .\n    vet:\n      run: go vet ./...\npre-push:\n  commands:\n    unit:\n      run: go test ./...\n"
		write(t, root, "lefthook.yml", yml)

		if _, err := ApplyWiring(root, Spec{Keys: demoSpec().Keys[:1]}, ApplyOptions{Now: fixedClock()}); err != nil {
			t.Fatalf("ApplyWiring: %v", err)
		}
		got := read(t, root, "lefthook.yml")
		iFmt, iVet, iUnit := strings.Index(got, "fmt:"), strings.Index(got, "vet:"), strings.Index(got, "unit:")
		if iFmt < 0 || iVet < 0 || iUnit < 0 {
			t.Fatalf("a foreign command was dropped:\n%s", got)
		}
		if !(iFmt < iVet && iVet < iUnit) {
			t.Errorf("foreign commands were reordered:\n%s", got)
		}
		if !strings.Contains(got, "    fathom-verify:\n      run: fathom verify --service demo\n") {
			t.Errorf("fathom's command was not added at the right indent:\n%s", got)
		}
		if !strings.Contains(got, "      run: go test ./...\n") {
			t.Errorf("the foreign block was damaged:\n%s", got)
		}
	})

	t.Run("other tools' json hooks survive", func(t *testing.T) {
		root := t.TempDir()
		settings := `{
  "permissions": {
    "allow": [
      "Bash(ls:*)"
    ]
  },
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "my-own-linter"
          }
        ]
      }
    ]
  }
}
`
		write(t, root, ".claude/settings.json", settings)
		if _, err := ApplyWiring(root, Spec{Keys: demoSpec().Keys[1:]}, ApplyOptions{Now: fixedClock()}); err != nil {
			t.Fatalf("ApplyWiring: %v", err)
		}
		got := read(t, root, ".claude/settings.json")
		var parsed map[string]any
		if err := json.Unmarshal([]byte(got), &parsed); err != nil {
			t.Fatalf("settings.json is no longer valid JSON: %v\n%s", err, got)
		}
		if !strings.Contains(got, "my-own-linter") {
			t.Errorf("another tool's hook was destroyed:\n%s", got)
		}
		if !strings.Contains(got, "Bash(ls:*)") {
			t.Errorf("an unrelated top-level key was destroyed:\n%s", got)
		}
		if n := strings.Count(got, "fathom edit-hook"); n != 1 {
			t.Errorf("fathom's hook appears %d times, want 1:\n%s", n, got)
		}
	})

	t.Run("plan is computed before any mutation", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "CLAUDE.md", "# demo\n")
		res, err := ApplyWiring(root, demoSpec(), ApplyOptions{Now: fixedClock()})
		if err != nil {
			t.Fatalf("ApplyWiring: %v", err)
		}
		if res.Plan.Digest == "" || len(res.Plan.Items) == 0 {
			t.Fatal("apply returned no plan — there is no mutation path that skips the diff")
		}
		// A plan digest from a stale preview must refuse.
		if _, err := ApplyWiring(root, demoSpec(), ApplyOptions{Now: fixedClock(), ExpectDigest: res.Plan.Digest}); err == nil {
			t.Fatal("apply accepted a plan digest that no longer describes the repository")
		}
	})
}

// ─── SH-5 ────────────────────────────────────────────────────────────────

// TestApplyIsIdempotent pins the `fathom init` hook-duplication regression:
// applying twice adds fathom's entries exactly once — including when the
// ownership ledger has been lost between the two runs.
func TestApplyIsIdempotent(t *testing.T) {
	for _, tc := range []struct {
		name       string
		loseLedger bool
	}{
		{"twice", false},
		{"twice with the ledger lost in between", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "CLAUDE.md", "# demo\n\nhouse prose\n")
			write(t, root, "lefthook.yml", "pre-commit:\n  commands:\n    fmt:\n      run: gofmt -l .\n")

			if _, err := ApplyWiring(root, demoSpec(), ApplyOptions{Now: fixedClock()}); err != nil {
				t.Fatalf("first apply: %v", err)
			}
			first := map[string]string{
				"CLAUDE.md":             read(t, root, "CLAUDE.md"),
				"lefthook.yml":          read(t, root, "lefthook.yml"),
				".claude/settings.json": read(t, root, ".claude/settings.json"),
			}
			if tc.loseLedger {
				if err := os.Remove(ledgerPath(root)); err != nil {
					t.Fatalf("removing the ledger: %v", err)
				}
			}

			res, err := ApplyWiring(root, demoSpec(), ApplyOptions{Now: fixedClock()})
			if err != nil {
				t.Fatalf("second apply: %v", err)
			}
			if len(res.Blocked) != 0 {
				t.Fatalf("second apply reported BLOCKED: %+v", res.Blocked)
			}
			for rel, want := range first {
				if got := read(t, root, rel); got != want {
					t.Errorf("%s changed on the second apply:\n--- first\n%s\n--- second\n%s", rel, want, got)
				}
			}
			if n := strings.Count(read(t, root, "CLAUDE.md"), "fathom:begin"); n != 1 {
				t.Errorf("after two applies CLAUDE.md carries %d fathom regions, want 1", n)
			}
			if n := strings.Count(read(t, root, "lefthook.yml"), "fathom-verify:"); n != 1 {
				t.Errorf("lefthook.yml carries %d fathom commands, want 1", n)
			}
			if n := strings.Count(read(t, root, ".claude/settings.json"), "fathom edit-hook"); n != 1 {
				t.Errorf("settings.json carries %d fathom hook entries, want 1 (the init duplication regression)", n)
			}
			if !tc.loseLedger {
				// The second run found nothing to do at all.
				if len(res.Applied) != 0 {
					t.Errorf("second apply mutated %d item(s): %+v", len(res.Applied), res.Applied)
				}
				if res.Receipt != nil {
					t.Error("second apply wrote a receipt for a no-op")
				}
			}
		})
	}
}

// ─── determinism ─────────────────────────────────────────────────────────

// TestPlanIsDeterministic: same repository, same plan — items sorted, digest
// stable. Determinism is a contract.
func TestPlanIsDeterministic(t *testing.T) {
	root := t.TempDir()
	write(t, root, "CLAUDE.md", "# demo\n")
	write(t, root, "lefthook.yml", "pre-commit:\n  commands:\n    fmt:\n      run: gofmt -l .\n")

	a, err := PlanWiring(root, demoSpec())
	if err != nil {
		t.Fatalf("PlanWiring: %v", err)
	}
	b, err := PlanWiring(root, demoSpec())
	if err != nil {
		t.Fatalf("PlanWiring: %v", err)
	}
	if a.Digest != b.Digest {
		t.Errorf("plan digest is not stable: %s vs %s", a.Digest, b.Digest)
	}
	for i := 1; i < len(a.Items); i++ {
		if a.Items[i-1].File > a.Items[i].File {
			t.Errorf("plan items are not sorted by file: %q then %q", a.Items[i-1].File, a.Items[i].File)
		}
	}
}

// ─── receipts (SH-6's library half) ──────────────────────────────────────

// TestReceiptCarriesPriorContent: the receipt IS the undo, so it must carry
// the full prior content of every file and region it touched.
func TestReceiptCarriesPriorContent(t *testing.T) {
	root := t.TempDir()
	prior := "# demo\n\n" + renderRegion(HTMLMarkers, "v2", stanzaBodyV2, "\n") + "\n"
	write(t, root, "CLAUDE.md", prior)

	res, err := ApplyWiring(root, demoSpec(), ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatalf("ApplyWiring: %v", err)
	}
	if res.Receipt == nil {
		t.Fatal("apply mutated files and wrote no receipt")
	}
	var found bool
	for _, f := range res.Receipt.Files {
		if f.Path != "CLAUDE.md" {
			continue
		}
		found = true
		if f.Before != prior {
			t.Errorf("receipt prior content is not the file's exact prior bytes:\n got %q\nwant %q", f.Before, prior)
		}
		if len(f.Touched) == 0 {
			t.Error("receipt records no touched region")
		}
		for _, r := range f.Touched {
			if r.Before != stanzaBodyV2 {
				t.Errorf("touched region prior body = %q, want %q", r.Before, stanzaBodyV2)
			}
		}
	}
	if !found {
		t.Fatalf("receipt has no entry for CLAUDE.md: %+v", res.Receipt.Files)
	}

	loaded, err := LoadReceipts(root)
	if err != nil {
		t.Fatalf("LoadReceipts: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != res.Receipt.ID {
		t.Fatalf("LoadReceipts = %d receipts, want the one just written", len(loaded))
	}
	if loaded[0].Time != "2026-08-08T12:00:00Z" {
		t.Errorf("receipt time = %q — the clock is not injected", loaded[0].Time)
	}
}

// TestTornApplyStillWritesAReceipt pins U1, found in review: an apply that
// fails partway used to discard the receipt entirely, leaving files already
// rewritten with no way to reverse them — while `fathom fleet receipts`
// stated, falsely, that fathom had written nothing there.
//
// That is this phase's own covenant failing on an ordinary error. The undo
// path is the entire justification for letting fathom edit a repository
// (law 15), so it must survive the failure that makes it necessary.
func TestTornApplyStillWritesAReceipt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: a read-only file would still be writable")
	}
	root := t.TempDir()

	// CLAUDE.md is writable and sorts before lefthook.yml; lefthook.yml is
	// read-only, so the loop rewrites the first and then fails on the
	// second — the ordinary torn-apply shape.
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# repo\n\nhand-written.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lefthook.yml"), []byte("pre-push:\n  commands:\n    mine:\n      run: echo hi\n"), 0o444); err != nil {
		t.Fatal(err)
	}

	// The region (CLAUDE.md) and one key (lefthook.yml) together, so the
	// loop touches the writable file before hitting the read-only one.
	spec := Spec{Regions: demoSpec().Regions, Keys: demoSpec().Keys[:1]}
	_, err := ApplyWiring(root, spec, ApplyOptions{Now: fixedClock()})
	if err == nil {
		t.Fatal("expected the read-only file to fail the apply")
	}

	receipts, lerr := LoadReceipts(root)
	if lerr != nil {
		t.Fatalf("loading receipts: %v", lerr)
	}
	if len(receipts) == 0 {
		t.Fatal("a torn apply wrote files and NO receipt — undo is impossible and `fleet receipts` would report nothing was written")
	}
	var sawClaude bool
	for _, f := range receipts[0].Files {
		if f.Path == "CLAUDE.md" {
			sawClaude = true
			if f.Before == "" {
				t.Error("the receipt must carry the file's prior content, or undo cannot restore it")
			}
		}
	}
	if !sawClaude {
		t.Errorf("the receipt must name the file that WAS written; got %+v", receipts[0].Files)
	}
}

// ─── INC-12: the v1→v3 migration switch ──────────────────────────────────

// TestAdoptUntrackedUpgradesLegacyRegion pins the production path's
// compatibility rule: a pre-v3 region (marker pair, no digest) is fathom's
// by its markers when the caller says so — upgraded and digest-marked on
// the first write, never reported as drift — while the engine default
// stays strict, and a TRACKED region that differs is still drift.
func TestAdoptUntrackedUpgradesLegacyRegion(t *testing.T) {
	legacyBody := "## fathom code intelligence\n\nwording no shipped template ever carried"
	legacy := "# demo\n\nhouse prose\n\n<!-- fathom:begin -->\n" + legacyBody + "\n<!-- fathom:end -->\n\ntrailing\n"

	strict := Region{File: "CLAUDE.md", Syntax: HTMLMarkers, Version: "v3", Body: stanzaBody}
	adopt := strict
	adopt.AdoptUntracked = true

	// Default: unknown provenance stays BLOCKED.
	root := t.TempDir()
	write(t, root, "CLAUDE.md", legacy)
	plan, err := PlanWiring(root, Spec{Regions: []Region{strict}})
	if err != nil {
		t.Fatalf("PlanWiring(strict): %v", err)
	}
	if got := itemFor(t, plan, "CLAUDE.md", "fathom:region").Verdict; got != VerdictBlocked {
		t.Fatalf("strict engine verdict = %s, want blocked (unknown provenance is not adopted by default)", got)
	}

	// AdoptUntracked: apply, upgrade, record — and it is NOT drift.
	plan, err = PlanWiring(root, Spec{Regions: []Region{adopt}})
	if err != nil {
		t.Fatalf("PlanWiring(adopt): %v", err)
	}
	it := itemFor(t, plan, "CLAUDE.md", "fathom:region")
	if it.Verdict != VerdictApply || !it.Present {
		t.Fatalf("adopt verdict = %s present=%v, want apply/present", it.Verdict, it.Present)
	}
	if strings.Contains(strings.ToLower(it.Reason), "edited by hand") {
		t.Errorf("an untracked region was described as drift: %q", it.Reason)
	}
	res, err := ApplyWiring(root, Spec{Regions: []Region{adopt}}, ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatalf("ApplyWiring(adopt): %v", err)
	}
	if res.Receipt == nil || len(res.Applied) != 1 {
		t.Fatalf("adopt wrote applied=%d receipt=%v, want 1 applied with a receipt", len(res.Applied), res.Receipt != nil)
	}
	got := read(t, root, "CLAUDE.md")
	if !strings.HasPrefix(got, "# demo\n\nhouse prose\n\n") || !strings.HasSuffix(got, "\n\ntrailing\n") {
		t.Errorf("bytes outside the region changed:\n%s", got)
	}
	reg, err := parseOwnedRegion(got, HTMLMarkers)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !reg.Tracked() || reg.Version != "v3" || reg.Body != stanzaBody {
		t.Fatalf("upgraded region tracked=%v version=%q body=%q — the first write must record a digest and the template", reg.Tracked(), reg.Version, reg.Body)
	}

	// Now it is tracked: current → skip; a hand edit → BLOCKED as drift.
	plan, _ = PlanWiring(root, Spec{Regions: []Region{adopt}})
	if got := itemFor(t, plan, "CLAUDE.md", "fathom:region").Verdict; got != VerdictSkip {
		t.Errorf("second plan verdict = %s, want skip", got)
	}
	write(t, root, "CLAUDE.md", strings.Replace(got, "Indexed as service", "Indexed (by hand) as service", 1))
	plan, _ = PlanWiring(root, Spec{Regions: []Region{adopt}})
	it = itemFor(t, plan, "CLAUDE.md", "fathom:region")
	if it.Verdict != VerdictBlocked || !strings.Contains(it.Reason, "edited by hand") {
		t.Errorf("tracked+edited verdict = %s (%s), want blocked as drift even with AdoptUntracked", it.Verdict, it.Reason)
	}
}

// TestAdoptUntrackedUpdatesUnrecordedKey: a marker-identified hook entry a
// pre-ledger fathom wrote is updated and recorded when the caller adopts
// it; the engine default leaves it as someone else's.
func TestAdoptUntrackedUpdatesUnrecordedKey(t *testing.T) {
	stale := "{\n  \"hooks\": {\n    \"PostToolUse\": [{\"matcher\": \"Edit|Write|MultiEdit\", \"hooks\": [{\"type\": \"command\", \"command\": \"/Volumes/x/bin/fathom edit-hook\", \"timeout\": 5}]}]\n  }\n}\n"
	key := Key{File: ".claude/settings.json", Kind: KindJSONHook, Path: []string{"PostToolUse", "Edit|Write|MultiEdit"}, Marker: "fathom edit-hook", Body: "fathom edit-hook", Timeout: 5}

	root := t.TempDir()
	write(t, root, ".claude/settings.json", stale)
	plan, err := PlanWiring(root, Spec{Keys: []Key{key}})
	if err != nil {
		t.Fatalf("PlanWiring(strict): %v", err)
	}
	if got := itemFor(t, plan, ".claude/settings.json", key.ID()).Verdict; got != VerdictSkip {
		t.Fatalf("strict verdict = %s, want skip (unrecorded is someone else's by default)", got)
	}

	key.AdoptUntracked = true
	res, err := ApplyWiring(root, Spec{Keys: []Key{key}}, ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatalf("ApplyWiring(adopt): %v", err)
	}
	if len(res.Applied) != 1 || res.Receipt == nil {
		t.Fatalf("adopt applied=%d receipt=%v, want 1 with a receipt", len(res.Applied), res.Receipt != nil)
	}
	got := read(t, root, ".claude/settings.json")
	if strings.Contains(got, "/Volumes/x") || !strings.Contains(got, `"command": "fathom edit-hook"`) {
		t.Errorf("stale entry not updated to the template:\n%s", got)
	}
	ledger, err := LoadLedger(root)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if _, owned := ownedKey(ledger, key.File, key.ID()); !owned {
		t.Error("adopted key was not recorded in the ledger — a later hand edit would be invisible")
	}
	plan, _ = PlanWiring(root, Spec{Keys: []Key{key}})
	if got := itemFor(t, plan, ".claude/settings.json", key.ID()).Verdict; got != VerdictSkip {
		t.Errorf("post-adopt verdict = %s, want skip", got)
	}
}
