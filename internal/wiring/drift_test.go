package wiring

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMCPEntryDigestNamesNotValues pins PC-28's one hard rule: env and
// header VALUES never reach the digest — a rotated token is not drift and
// cannot leak through the ledger — while a new env NAME, a changed argv or
// a changed URL is drift. Key order never matters.
func TestMCPEntryDigestNamesNotValues(t *testing.T) {
	base := map[string]any{
		"type":    "http",
		"url":     "http://localhost:7575/rpc/agent",
		"env":     map[string]any{"TOKEN": "secret-1", "A": "1"},
		"headers": map[string]any{"Authorization": "Bearer x"},
	}
	d1 := mcpEntryDigest(base)
	rotated := map[string]any{
		"headers": map[string]any{"Authorization": "Bearer y"},
		"env":     map[string]any{"A": "2", "TOKEN": "secret-2"},
		"url":     "http://localhost:7575/rpc/agent",
		"type":    "http",
	}
	if d2 := mcpEntryDigest(rotated); d2 != d1 {
		t.Errorf("rotated values / reordered keys must not change the digest: %s vs %s", d1, d2)
	}
	newName := map[string]any{
		"type": "http", "url": "http://localhost:7575/rpc/agent",
		"env": map[string]any{"TOKEN": "secret-1", "A": "1", "LD_PRELOAD": "x"}, "headers": map[string]any{"Authorization": "Bearer x"},
	}
	if d3 := mcpEntryDigest(newName); d3 == d1 {
		t.Error("a new env NAME must change the digest")
	}
	otherURL := map[string]any{"type": "http", "url": "http://evil.example/rpc", "env": map[string]any{"TOKEN": "secret-1", "A": "1"}, "headers": map[string]any{"Authorization": "Bearer x"}}
	if d4 := mcpEntryDigest(otherURL); d4 == d1 {
		t.Error("a changed url must change the digest")
	}
	argv1 := mcpEntryDigest(map[string]any{"command": "fathom", "args": []any{"serve-mcp", "--store", "a.sqlite"}})
	argv2 := mcpEntryDigest(map[string]any{"command": "fathom", "args": []any{"serve-mcp", "--store", "b.sqlite"}})
	if argv1 == argv2 {
		t.Error("a changed argv must change the digest")
	}
	if mcpEntryDigest(base) != d1 {
		t.Error("digest must be deterministic across calls")
	}
}

func TestMCPEntryDigestJSONAndTOML(t *testing.T) {
	js := []byte(`{"mcpServers":{"fathom":{"type":"http","url":"http://localhost:7575/rpc/agent"},"other":{}}}`)
	dj, present, err := MCPEntryDigest(js, "json", []string{"mcpServers", "fathom"})
	if err != nil || !present || dj == "" {
		t.Fatalf("json: dig=%q present=%v err=%v", dj, present, err)
	}
	tm := []byte("[mcp_servers.fathom]\ntype = \"http\"\nurl = \"http://localhost:7575/rpc/agent\"\n")
	dt, present, err := MCPEntryDigest(tm, "toml", []string{"mcp_servers", "fathom"})
	if err != nil || !present {
		t.Fatalf("toml: present=%v err=%v", present, err)
	}
	if dt != dj {
		t.Errorf("the same entry in json and toml must digest alike: %s vs %s", dj, dt)
	}
	if _, present, err := MCPEntryDigest(js, "json", []string{"mcpServers", "absent"}); err != nil || present {
		t.Errorf("absent entry: present=%v err=%v", present, err)
	}
	if _, _, err := MCPEntryDigest([]byte("{not json"), "json", []string{"a"}); err == nil {
		t.Error("unparsable json must error, never read as absent")
	}
}

// TestDriftMCPAndSkills: an unrecorded entry is not drift; a recorded one
// that changed is drift by kind and path; an unreadable file is named
// (I38) and never counted as current; a missing file is nothing.
func TestDriftMCPAndSkills(t *testing.T) {
	root := t.TempDir()
	mcpPath := filepath.Join(root, ".mcp.json")
	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(mcpPath, `{"mcpServers":{"fathom":{"type":"http","url":"http://localhost:7575/rpc/agent"}}}`)
	skillPath := filepath.Join(root, ".claude", "skills", "fathom-doctor", "SKILL.md")
	write(skillPath, "<!-- fathom:owned v1 -->\n# doctor\n")

	mcp := []MCPFile{
		{Path: mcpPath, Label: ".mcp.json", Format: "json", KeyPath: []string{"mcpServers", "fathom"}},
		{Path: filepath.Join(root, "missing.json"), Label: "~/.claude.json", Format: "json", KeyPath: []string{"mcpServers", "fathom"}},
	}
	skills := []SkillFile{{Path: skillPath, Label: ".claude/skills/fathom-doctor/SKILL.md"}}

	ledger, _ := LoadLedger(root)
	rep := DriftMCP(ledger, mcp, nil)
	rep.Merge(DriftSkills(ledger, skills, nil))
	if len(rep.Drifts) != 0 || len(rep.Unreadable) != 0 {
		t.Fatalf("fresh repo must have no drift: %+v", rep)
	}
	if strings.Join(rep.Unrecorded, ";") != "mcp .mcp.json mcpServers.fathom;skill .claude/skills/fathom-doctor/SKILL.md" {
		t.Errorf("unrecorded = %v", rep.Unrecorded)
	}

	recorded, err := RecordBaselines(root, mcp, skills, "2026-09-02", nil, false)
	if err != nil || len(recorded) != 2 {
		t.Fatalf("RecordBaselines = %v, %v", recorded, err)
	}
	ledger, _ = LoadLedger(root)
	rep = DriftMCP(ledger, mcp, nil)
	rep.Merge(DriftSkills(ledger, skills, nil))
	if len(rep.Drifts) != 0 || len(rep.Unrecorded) != 0 {
		t.Fatalf("after baseline: %+v", rep)
	}

	// Change the entry's argv (a different server!) and the skill body.
	write(mcpPath, `{"mcpServers":{"fathom":{"type":"stdio","command":"curl","args":["http://evil.example"]}}}`)
	write(skillPath, "<!-- fathom:owned v1 -->\n---\nhooks: [x]\n---\n# doctor\n")
	rep = DriftMCP(ledger, mcp, nil)
	rep.Merge(DriftSkills(ledger, skills, nil))
	lines := rep.Lines()
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "mcp .mcp.json mcpServers.fathom — ") || !strings.HasPrefix(lines[1], "skill .claude/skills/fathom-doctor/SKILL.md — ") {
		t.Errorf("drift lines = %v", lines)
	}
	for _, l := range lines {
		if strings.Contains(l, "evil.example") {
			t.Errorf("a drift line must name kind and path, never echo the new entry: %q", l)
		}
	}

	// Unreadable: the reader errors with something that is not ENOENT.
	eacces := func(string) ([]byte, error) {
		return nil, &os.PathError{Op: "open", Path: mcpPath, Err: errors.New("permission denied")}
	}
	rep = DriftMCP(ledger, mcp, eacces)
	if len(rep.Drifts) != 0 || len(rep.Unreadable) != 2 || !strings.Contains(rep.Unreadable[0], "permission denied") {
		t.Errorf("unreadable must be named, never current: %+v", rep)
	}
	// Recorded baselines are not overwritten by a plain re-record.
	if again, _ := RecordBaselines(root, mcp, skills, "2026-09-03", nil, false); len(again) != 0 {
		t.Errorf("plain re-record must not rebaseline a drifted entry: %v", again)
	}
	if again, _ := RecordBaselines(root, mcp, skills, "2026-09-03", nil, true); len(again) != 2 {
		t.Errorf("explicit rebaseline records both: %v", again)
	}
}

// TestRecordKeyBaselines pins the Key counterpart of RecordBaselines: before
// this, a hook entry law 15 blocked (its ledger digest no longer matched
// what was on disk) had NO rebaseline path at all — RecordBaselines only
// knew MCP entries and owned skills, so `fleet reconcile --apply
// --rebaseline` could never clear a drifted hook (the observed bug: six
// PROJECT-scope hook entries drifted against .fathom/owned.json after the
// project's own committed edit — e.g. "hook rows in shim spelling" — moved
// them outside fathom's apply path). rebaseline=true must move ONLY the
// ledger digest, to the value the file NOW carries, and never write the
// file itself.
func TestRecordKeyBaselines(t *testing.T) {
	root := t.TempDir()
	rel := ".claude/settings.json"
	original := `{"hooks":{"PostToolUse":[{"matcher":"Edit|Write|MultiEdit","hooks":[{"type":"command","command":"fathom edit-hook","timeout":5}]}]}}`
	write(t, root, rel, original)

	key := Key{
		File: rel, Kind: KindJSONHook,
		Path:   []string{"PostToolUse", "Edit|Write|MultiEdit"},
		Marker: "fathom edit-hook", Body: "fathom edit-hook", Timeout: 5,
	}
	mustPlan := func() Plan {
		t.Helper()
		p, err := PlanWiring(root, Spec{Keys: []Key{key}})
		if err != nil {
			t.Fatalf("PlanWiring: %v", err)
		}
		return p
	}

	// Record the original baseline directly, as ApplyWiring would have the
	// first time fathom wrote this entry.
	cur, present, cerr := key.current(original)
	if cerr != nil || !present {
		t.Fatalf("current: %q %v %v", cur, present, cerr)
	}
	ledger := NewLedger()
	ledger.Record(rel, OwnedKey{ID: key.ID(), Digest: digest(cur), Wrote: "2026-09-12"})
	if err := SaveLedger(root, ledger); err != nil {
		t.Fatal(err)
	}
	if it := itemFor(t, mustPlan(), rel, key.ID()); it.Verdict != VerdictSkip {
		t.Fatalf("fresh baseline should read current, got %+v", it)
	}

	// The project's own committed edit moves the entry without going
	// through fathom's apply path.
	edited := `{"hooks":{"PostToolUse":[{"matcher":"Edit|Write|MultiEdit","hooks":[{"type":"command","command":"fathom edit-hook","timeout":45}]}]}}`
	write(t, root, rel, edited)

	it := itemFor(t, mustPlan(), rel, key.ID())
	if it.Verdict != VerdictBlocked || it.Cause != CauseDrift {
		t.Fatalf("edited entry should be BLOCKED/drift, got %+v", it)
	}

	// rebaseline=false is a deliberate no-op — law 15: fathom never
	// re-records a drifted entry on its own.
	if recorded, rerr := RecordKeyBaselines(root, []Key{key}, "2026-09-24", nil, false); rerr != nil || len(recorded) != 0 {
		t.Fatalf("rebaseline=false must record nothing: %v, %v", recorded, rerr)
	}
	if it := itemFor(t, mustPlan(), rel, key.ID()); it.Verdict != VerdictBlocked {
		t.Fatalf("rebaseline=false must leave the drift in place, got %+v", it)
	}

	// rebaseline=true records the CURRENT digest and touches nothing else.
	before := read(t, root, rel)
	recorded, rerr := RecordKeyBaselines(root, []Key{key}, "2026-09-24", nil, true)
	if rerr != nil || len(recorded) != 1 || recorded[0] != DriftKindHook+" "+rel+" "+key.ID() {
		t.Fatalf("RecordKeyBaselines(true) = %v, %v", recorded, rerr)
	}
	if got := read(t, root, rel); got != before {
		t.Errorf("RecordKeyBaselines rewrote the entry; only the ledger may move:\nbefore: %s\nafter:  %s", before, got)
	}
	if it := itemFor(t, mustPlan(), rel, key.ID()); it.Verdict == VerdictBlocked {
		t.Errorf("drift not cleared after rebaseline: %+v", it)
	}

	// A second rebaseline over a now-clean entry records nothing.
	if recorded, rerr := RecordKeyBaselines(root, []Key{key}, "2026-09-25", nil, true); rerr != nil || len(recorded) != 0 {
		t.Errorf("rebaseline over a clean entry should record nothing: %v, %v", recorded, rerr)
	}
}

// TestDriftFromPlan lifts law-15 verdicts: a drifted region is stanza
// drift, an unreadable key file is unreadable, applicable/skip are neither.
func TestDriftFromPlan(t *testing.T) {
	p := Plan{Items: []Item{
		{File: "CLAUDE.md", Kind: KindRegion, Target: "fathom", Verdict: VerdictBlocked, Cause: CauseDrift, Reason: "edited by hand"},
		{File: ".claude/settings.json", Kind: KindKey, Target: "PostToolUse|Edit|x", Verdict: VerdictBlocked, Cause: CauseUnreadable, Reason: "cannot be read: EACCES"},
		{File: "AGENTS.md", Kind: KindRegion, Verdict: VerdictSkip},
	}}
	rep := DriftFromPlan(p)
	if rep.Checked != 3 || len(rep.Drifts) != 1 || rep.Drifts[0].Kind != DriftKindStanza || len(rep.Unreadable) != 1 {
		t.Errorf("report = %+v", rep)
	}
}
