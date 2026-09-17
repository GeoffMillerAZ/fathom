package custodian

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// CC-15: a handoff note that claims shipped work must cite an instrument
// — a commit sha, a plan row, `fathom verify`, or a retro artifact. The
// thread log and the clock are injected; the repo is identified the way
// `fathom thread note` records it (the lower-cased root basename).
func TestCompletionCitesInstrumentsConcern(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Widget")
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "# widget\n")
	// The repo keeps plans: CC-14 is a row of one, XY-9 is nobody's.
	writeTestFile(t, filepath.Join(root, "docs", "plans", "custodian.md"),
		"# custodian\n\n- [x] CC-14 stanza concern {id: CC-14, symbol: checkStanzaToolsServed, package: internal/custodian}\n")
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) string { return now.Add(-d).Format(time.RFC3339) }
	day := 24 * time.Hour

	log := strings.Join([]string{
		// Claims shipping, cites nothing → the finding.
		`{"schema_version":1,"id":"th-aaaa0001","thread":"parser","kind":"handoff-note","text":"shipped the parser rewrite; next session picks up the lexer","pointers":{"repo":"widget","session_id":"fff83839-04b7-40fd-8bd9-2576f2e33540"},"filed_at":"` + ago(36*time.Hour) + `","filed_by":"agent"}`,
		// Claims shipping, cites a plan row the repo's plans carry → fine.
		`{"schema_version":1,"id":"th-aaaa0002","thread":"custodian","kind":"handoff-note","text":"landed CC-14 stanza concern","pointers":{"repo":"widget"},"filed_at":"` + ago(2*day) + `"}`,
		// Another repo's note → not this audit's business.
		`{"schema_version":1,"id":"th-aaaa0003","thread":"x","kind":"handoff-note","text":"done, no citation at all","pointers":{"repo":"other"},"filed_at":"` + ago(1*day) + `"}`,
		// Older than the window → aged out.
		`{"schema_version":1,"id":"th-aaaa0004","thread":"x","kind":"handoff-note","text":"fixed everything","pointers":{"repo":"widget"},"filed_at":"` + ago(30*day) + `"}`,
		// Wrong kind → not a handoff claim.
		`{"schema_version":1,"id":"th-aaaa0005","thread":"x","kind":"decision-pending","text":"is the fold done?","pointers":{"repo":"widget"},"filed_at":"` + ago(1*day) + `"}`,
		// No shipping claim → nothing to cite.
		`{"schema_version":1,"id":"th-aaaa0006","thread":"x","kind":"handoff-note","text":"next: look at the lexer's error recovery","pointers":{"repo":"widget"},"filed_at":"` + ago(1*day) + `"}`,
		// Claims shipping; a later closure row cites the commit → the
		// remediation path is `fathom thread close --why <sha>`.
		`{"schema_version":1,"id":"th-aaaa0007","thread":"x","kind":"handoff-note","text":"implemented the fold","pointers":{"repo":"widget"},"filed_at":"` + ago(3*day) + `"}`,
		`{"schema_version":1,"id":"th-aaaa0007","closed_at":"` + ago(2*day) + `","closed_by":"agent","closed_why":"landed in abc1234f"}`,
		// A retro artifact path counts as a citation.
		`{"schema_version":1,"id":"th-aaaa0008","thread":"x","kind":"handoff-note","text":"closed the bench; retro at workspace/artifacts/retros/2026-08-24.md","pointers":{"repo":"widget"},"filed_at":"` + ago(1*day) + `"}`,
		// Look-alikes are NOT citations: an encoding name, a digest name,
		// an all-letter hex word, a bare number, a plan-shaped id no plan
		// carries, another thread's id.
		`{"schema_version":1,"id":"th-aaaa0009","thread":"x","kind":"handoff-note","text":"fixed the UTF-8 decoder and the SHA-256 digest path","pointers":{"repo":"widget"},"filed_at":"` + ago(2*day) + `"}`,
		`{"schema_version":1,"id":"th-aaaa0010","thread":"x","kind":"handoff-note","text":"shipped deadbeef handling; ticket 1234567 closed","pointers":{"repo":"widget"},"filed_at":"` + ago(2*day) + `"}`,
		`{"schema_version":1,"id":"th-aaaa0011","thread":"x","kind":"handoff-note","text":"landed XY-9 (see th-9cfeffd7)","pointers":{"repo":"widget"},"filed_at":"` + ago(2*day) + `"}`,
		// An undated row cannot be placed in the window: skipped, but
		// counted and named — never silently.
		`{"schema_version":1,"id":"th-aaaa0012","thread":"x","kind":"handoff-note","text":"shipped something, sometime","pointers":{"repo":"widget"},"filed_at":"yesterday"}`,
		"",
	}, "\n")
	threads := filepath.Join(t.TempDir(), "threads.jsonl")
	writeTestFile(t, threads, log)

	c := Concern{
		ID: "completion_cites", Want: "a handoff note claiming shipped work cites an instrument",
		Check: Check{Kind: KindHandoffCitesInstruments},
	}

	fs := Audit(root, []Concern{c}, WithThreadsPath(threads), WithNow(func() time.Time { return now }))
	if len(fs) != 1 || fs[0].Status != StatusViolated {
		t.Fatalf("uncited shipping claim must violate, got %+v", fs)
	}
	// Fold order: filed_at ascending, then id — the two-day look-alikes
	// precede the 36-hour note.
	wantIDs := []string{"th-aaaa0009", "th-aaaa0010", "th-aaaa0011", "th-aaaa0001"}
	if len(fs[0].Evidence) != len(wantIDs) {
		t.Fatalf("evidence must list exactly the uncited items %v, got %v", wantIDs, fs[0].Evidence)
	}
	for i, id := range wantIDs {
		if !strings.HasPrefix(fs[0].Evidence[i], id+" ") {
			t.Fatalf("evidence[%d] must be %s, got %q", i, id, fs[0].Evidence[i])
		}
	}
	if !strings.Contains(fs[0].Detail, "4 of 8 handoff note(s)") || !strings.Contains(fs[0].Detail, "7 day") {
		t.Fatalf("detail must count the inspected notes and the window, got %q", fs[0].Detail)
	}
	if !strings.Contains(fs[0].Detail, "1 handoff note(s) from this repo skipped") || !strings.Contains(fs[0].Detail, "th-aaaa0012") || !strings.Contains(fs[0].Detail, "filed_at") {
		t.Fatalf("detail must count and name the undated row, got %q", fs[0].Detail)
	}
	for _, id := range []string{"th-aaaa0002", "th-aaaa0007", "th-aaaa0008"} {
		if strings.Contains(strings.Join(fs[0].Evidence, "\n"), id) {
			t.Fatalf("cited note %s must not be evidence: %v", id, fs[0].Evidence)
		}
	}
	// Determinism: a second run yields the identical finding.
	again := Audit(root, []Concern{c}, WithThreadsPath(threads), WithNow(func() time.Time { return now }))
	if again[0].Detail != fs[0].Detail || strings.Join(again[0].Evidence, "|") != strings.Join(fs[0].Evidence, "|") {
		t.Fatalf("audit must be deterministic: %+v vs %+v", again[0], fs[0])
	}

	// A narrower window (1 day) that ages every offender out → holds,
	// naming the window it inspected — and still naming the undated row.
	narrow := c
	narrow.Check.Days = 1
	fs = Audit(root, []Concern{narrow}, WithThreadsPath(threads), WithNow(func() time.Time { return now }))
	if fs[0].Status != StatusHolds || !strings.Contains(fs[0].Detail, "1 day") || !strings.Contains(fs[0].Detail, "th-aaaa0012") {
		t.Fatalf("window must be honoured and the undated row still named, got %+v", fs[0])
	}

	// No thread log at all → holds with the absence named, never a
	// violation and never a silent pass.
	fs = Audit(root, []Concern{c}, WithThreadsPath(filepath.Join(t.TempDir(), "absent.jsonl")))
	if fs[0].Status != StatusHolds || !strings.Contains(fs[0].Detail, "nothing to audit") {
		t.Fatalf("absent log must hold naming the absence, got %+v", fs[0])
	}
}

// The citation grammar, shape by shape. A commit sha is 7–12 or 40 hex
// characters with a letter AND a digit; a plan row id is held to the
// repo's plans when it keeps any, and to the look-alike denylist always.
func TestInstrumentCitationShapes(t *testing.T) {
	withPlans := planRows{present: true, ids: map[string]struct{}{"CC-14": {}, "IT-3": {}, "A1-2": {}}}
	noPlans := planRows{}

	cases := []struct {
		text              string
		withPlans, noPlan bool
	}{
		// commit shas
		{"landed in abc1234f", true, true},
		{"landed in a1b2c3d", true, true},
		{"landed in 0123456789abcdef0123456789abcdef01234567", true, true},
		{"shipped deadbeef handling", false, false},    // all letters
		{"ticket 1234567 closed", false, false},        // all digits
		{"digest 0123456789abcdef01", false, false},    // 20 chars: neither abbreviated nor full
		{"see th-9cfeffd7 for the rest", false, false}, // a thread id, not a commit
		// plan rows
		{"landed CC-14", true, true},
		{"landed A1-2 and IT-3", true, true},
		{"landed XY-9", false, true}, // shape only: plans reject it, no-plans cannot
		{"fixed the UTF-8 decoder", false, false},
		{"SHA-256 digests everywhere", false, false},
		{"ISO-8601 timestamps; RFC-3339 too", false, false},
		{"TLS-1 and IPV-6 and ECDSA-256 and ED-25519", false, false},
		// literals
		{"gated by fathom verify", true, true},
		{"retro at workspace/artifacts/retros/x.md", true, true},
		// O63 item 5: retro.go now writes under statedir.Resolve(root) by
		// default, which for most repos lives outside the tree entirely —
		// the legacy in-tree prefix is no longer the only shape a citation
		// carries, so any path ending in artifacts/retros must count.
		{"retro at /Users/geoff/.local/state/fathom/9f8e7d6c5b4a3210/artifacts/retros/2026-09-17.md", true, true},
		{"", false, false},
	}
	for _, tc := range cases {
		if got := citesInstrument(tc.text, withPlans); got != tc.withPlans {
			t.Errorf("with plans: citesInstrument(%q) = %v, want %v", tc.text, got, tc.withPlans)
		}
		if got := citesInstrument(tc.text, noPlans); got != tc.noPlan {
			t.Errorf("no plans: citesInstrument(%q) = %v, want %v", tc.text, got, tc.noPlan)
		}
	}
}

// loadPlanRows reads row ids from active AND archived plans (a plan
// absorbed into the changelog inside the window is still the instrument)
// and reports absence when the directory does not exist.
func TestLoadPlanRows(t *testing.T) {
	root := t.TempDir()
	if p := loadPlanRows(root); p.present || len(p.ids) != 0 {
		t.Fatalf("no docs/plans must be absent, got %+v", p)
	}
	writeTestFile(t, filepath.Join(root, "docs", "plans", "a.md"), "- [ ] CC-1 x {id: CC-1}\n")
	writeTestFile(t, filepath.Join(root, "docs", "plans", "archive", "b.md"), "- [x] OLD-7 y {id: OLD-7}\n")
	p := loadPlanRows(root)
	if !p.present {
		t.Fatal("docs/plans must be present")
	}
	for _, id := range []string{"CC-1", "OLD-7"} {
		if _, ok := p.ids[id]; !ok {
			t.Fatalf("plan row %s must be loaded, got %v", id, p.ids)
		}
	}
	if _, ok := p.ids["ZZ-1"]; ok {
		t.Fatal("an id no plan names must not be loaded")
	}
}

// The loader accepts `days` for handoff_cites_instruments (defaulting it
// to 7), refuses a non-positive value, and refuses the key on any other
// kind — the schema stays closed.
func TestLoadHandoffDays(t *testing.T) {
	p := filepath.Join(t.TempDir(), "concerns.cue")
	writeTestFile(t, p, `concerns: {
	completion_cites: {
		want:  "handoff notes cite instruments"
		check: {kind: "handoff_cites_instruments", days: 14}
	}
	defaulted: {
		want:  "handoff notes cite instruments"
		check: {kind: "handoff_cites_instruments"}
	}
	stanza_current: {
		want:  "the stanza names only served tools"
		check: {kind: "stanza_tools_served"}
	}
}
`)
	cs, err := LoadFile(p, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 3 || cs[0].ID != "completion_cites" || cs[0].Check.Days != 14 {
		t.Fatalf("days not carried: %+v", cs)
	}
	if cs[1].ID != "defaulted" || cs[1].Check.Days != 7 {
		t.Fatalf("days must default to 7, got %+v", cs[1])
	}
	if cs[2].ID != "stanza_current" || cs[2].Check.Where != CorpusAgentConfig {
		t.Fatalf("stanza kind must default its corpus, got %+v", cs[2])
	}

	for name, src := range map[string]string{
		"zero":       `concerns: {a: {want: "w", check: {kind: "handoff_cites_instruments", days: 0}}}`,
		"wrong-kind": `concerns: {a: {want: "w", check: {kind: "max_lines", file: "CLAUDE.md", lines: 3, days: 7}}}`,
	} {
		p := filepath.Join(t.TempDir(), "concerns.cue")
		writeTestFile(t, p, src+"\n")
		if _, err := LoadFile(p, ScopeUser); err == nil || !strings.Contains(err.Error(), "days") {
			t.Fatalf("%s: days misuse not refused: %v", name, err)
		}
	}
}
