package main

// Regression tests for the findings of the 2026-10-19 adversarial review.
// Each one fails if its defect comes back.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestContainmentAlwaysCarriesTheSavedPath(t *testing.T) {
	ta := newTestApp(t)
	out := decodeOutput(t, ta.hook(t, hostClaude, bashPayload("", "ok\n", strings.Repeat("e", 20000))))
	var resp map[string]any
	if err := json.Unmarshal(out.HookSpecificOutput.UpdatedToolOutput, &resp); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := resp["stdout"].(string), resp["stderr"].(string)
	all := stdout + stderr
	if strings.Count(all, "Full output: ") != 1 || stdout != "ok\n" {
		t.Fatalf("small stdout with large stderr must keep stdout whole and carry the path exactly once:\nstdout %q\nstderr head %.200q", stdout, stderr)
	}
	m := regexp.MustCompile(`(\d+) of (\d+) characters kept inline`).FindStringSubmatch(all)
	if m == nil || m[1] != "8000" || m[2] != "20003" {
		t.Fatalf("notice counts = %v, want 8000 of 20003", m)
	}
}

func TestContainmentSkipsImages(t *testing.T) {
	ta := newTestApp(t)
	p := bashPayload("", "data:image/png;base64,"+strings.Repeat("A", 30000), "")
	p["tool_response"].(map[string]any)["isImage"] = true
	if out := ta.hook(t, hostClaude, p); out != "" {
		t.Fatalf("an image result was replaced: %.120s", out)
	}
}

func TestSecondRotationNeverEvictsAFullLedger(t *testing.T) {
	ta := newTestApp(t)
	cfg := LedgerConfig{Enabled: true, MaxBytes: 1 << 20, KeepRotated: 1}
	path := ta.ledgerPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	full := bytes.Repeat([]byte("x"), int(cfg.MaxBytes))
	_ = os.WriteFile(path, full, 0o600)
	ta.rotateLedger(path, cfg.MaxBytes, cfg.KeepRotated) // process A rotates
	_ = os.WriteFile(path, []byte(`{"small":true}`+"\n"), 0o600)
	ta.now = func() time.Time { return fixedNow.Add(time.Second) }
	ta.rotateLedger(path, cfg.MaxBytes, cfg.KeepRotated) // process B saw the old size
	rotated := rotatedLedgers(path)
	if len(rotated) != 1 {
		t.Fatalf("rotated files = %v", rotated)
	}
	if info, _ := os.Stat(rotated[0]); info.Size() != cfg.MaxBytes {
		t.Fatalf("the full rotated ledger was evicted by a second rotation (kept %d bytes)", info.Size())
	}
}

func TestReportCountsASyncSpawnOnce(t *testing.T) {
	ta := newTestApp(t)
	transcript := writeFile(t, ta.dir, "agent.jsonl", `{"message":{"id":"m","model":"claude-haiku-5-5","usage":{"input_tokens":500}}}`)
	cfg := builtinPolicy().Ledger
	ta.appendLedger(cfg, ledgerRecord{Host: hostClaude, Kind: kindSubagentStop, AgentID: "ag1", AgentType: "Explore", ReportChars: 40, Transcript: transcript})
	ta.appendLedger(cfg, ledgerRecord{Host: hostClaude, Kind: kindSpawnResult, Status: "completed", SpawnedAgent: "ag1", AgentType: "Explore", Resolved: "claude-haiku-5-5", TotalTokens: 500, ReportChars: 40})
	ta.run([]string{"report"})
	out := ta.out.String()
	if !regexp.MustCompile(`claude-haiku-5-5\s+500\n`).MatchString(out) || !regexp.MustCompile(`claude Explore\s+40 / 40 / 1`).MatchString(out) {
		t.Fatalf("a sync spawn was counted twice:\n%s", out)
	}
}

func TestPolicyParityWithSchema(t *testing.T) {
	var base map[string]any
	_ = json.Unmarshal(defaultPolicyJSON, &base)
	clone := func() map[string]any {
		raw, _ := json.Marshal(base)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		return m
	}
	reject := map[string]func(m map[string]any){
		"missing require_model":   func(m map[string]any) { delete(m["spawn"].(map[string]any), "require_model") },
		"missing main_thread":     func(m map[string]any) { delete(m, "main_thread") },
		"missing ledger.enabled":  func(m map[string]any) { delete(m["ledger"].(map[string]any), "enabled") },
		"missing deny_loop_limit": func(m map[string]any) { delete(m["spawn"].(map[string]any), "deny_loop_limit") },
		"key case VERSION":        func(m map[string]any) { m["VERSION"] = m["version"]; delete(m, "version") },
		"key case in a tier":      func(m map[string]any) { m["tiers"].([]any)[0].(map[string]any)["Use_For"] = "x" },
		"Inherit model":           func(m map[string]any) { m["tiers"].([]any)[0].(map[string]any)["claude"] = []any{"Inherit"} },
		"form feed in a model":    func(m map[string]any) { m["tiers"].([]any)[0].(map[string]any)["claude"] = []any{"a\fb"} },
		"duplicate effort hosts":  func(m map[string]any) { m["spawn"].(map[string]any)["require_effort"] = []any{"codex", "codex"} },
		"empty unless": func(m map[string]any) {
			m["advice"] = []any{map[string]any{"id": "x", "when": "y", "unless": "", "message": "z"}}
		},
	}
	for name, mutate := range reject {
		m := clone()
		mutate(m)
		raw, _ := json.Marshal(m)
		if _, err := parsePolicy(raw); err == nil {
			t.Errorf("%s: accepted, but schema.cue rejects it", name)
		}
	}
	m := clone()
	m["tiers"].([]any)[0].(map[string]any)["use_for"] = strings.Repeat("é", 600) // 600 runes, 1200 bytes
	raw, _ := json.Marshal(m)
	if _, err := parsePolicy(raw); err != nil {
		t.Errorf("600-rune text rejected, but the schema counts runes (MaxRunes 1024): %v", err)
	}
}

func TestCommandHeadNeverStoresPartOfAQuotedValue(t *testing.T) {
	for in, want := range map[string]string{
		`{"command":"PASSWORD='correct horse battery' ./deploy.sh"}`: "deploy.sh",
		`{"command":"TOKEN=\"a b\" env X='y z' make test"}`:          "make",
		`{"command":"echo 'quoted program name'"}`:                   "echo",
	} {
		if got := commandHead(json.RawMessage(in)); got != want {
			t.Errorf("commandHead(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestOneOversizedLedgerLineHidesNothing(t *testing.T) {
	ta := newTestApp(t)
	path := ta.ledgerPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	good := func(n int) string {
		b, _ := json.Marshal(ledgerRecord{TS: fixedNow.Format(time.RFC3339Nano), Host: hostClaude, Kind: kindSpawn, BriefChars: n})
		return string(b)
	}
	content := good(1) + "\n" + `{"huge":"` + strings.Repeat("x", 2<<20) + `"}` + "\n" + good(2) + "\n"
	_ = os.WriteFile(path, []byte(content), 0o600)
	var seen []int
	scanLedger(path, func(r ledgerRecord, _ time.Time) { seen = append(seen, r.BriefChars) })
	if len(seen) != 2 || seen[1] != 2 {
		t.Fatalf("records after an oversized line were lost: %v", seen)
	}
}

func TestLedgerLabelsAreCapped(t *testing.T) {
	ta := newTestApp(t)
	ta.hook(t, hostCodex, codexSpawn("s", map[string]any{"message": "x", "model": strings.Repeat("m", 5000), "reasoning_effort": "low"}))
	if r := ta.ledger(t)[0]; len([]rune(r.Model)) != maxLabel {
		t.Fatalf("model label is %d characters, want capped at %d", len([]rune(r.Model)), maxLabel)
	}
}

func TestDailyStampsAndTrashAreCleaned(t *testing.T) {
	ta := newTestApp(t)
	root := filepath.Join(ta.stateDir(), "sessions")
	_ = os.MkdirAll(filepath.Join(root, ".trash-old"), 0o700)
	_ = os.WriteFile(filepath.Join(root, ".pruned-20200101"), nil, 0o600)
	ta.pruneDaily(root)
	entries, _ := os.ReadDir(root)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != ".pruned-"+fixedNow.Format("20060102") {
		t.Fatalf("after pruning: %v, want only today's stamp", names)
	}
}

func TestPruneKeepsADirectoryWithARecentFile(t *testing.T) {
	ta := newTestApp(t)
	root := filepath.Join(ta.stateDir(), "outputs")
	dir := filepath.Join(root, "session")
	_ = os.MkdirAll(dir, 0o700)
	_ = os.WriteFile(filepath.Join(dir, "fresh.txt"), []byte("evidence"), 0o600)
	old := fixedNow.Add(-30 * 24 * time.Hour)
	_ = os.Chtimes(dir, old, old) // the directory looks old; its file is new
	ta.pruneDaily(root)
	if _, err := os.Stat(filepath.Join(dir, "fresh.txt")); err != nil {
		t.Fatal("pruning deleted a saved output that is still fresh")
	}
}

func TestRetypedUnusedFieldStillGates(t *testing.T) {
	ta := newTestApp(t)
	p := claudeSpawn("s", map[string]any{"prompt": "find it"})
	p["cwd"] = 5                       // a future host retypes a field this binary does not need
	p["permission_mode"] = []any{1, 2} // and adds or changes others
	if out := decodeOutput(t, ta.hook(t, hostClaude, p)); out.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("a retyped unrelated field ungated the spawn: %+v", out.HookSpecificOutput)
	}
}

func TestDenyLoopBreakerReleasesAndResets(t *testing.T) {
	ta := newTestApp(t)
	ta.writePolicy(t, func(m map[string]any) { m["spawn"].(map[string]any)["deny_loop_limit"] = 2 })
	unset := claudeSpawn("s", map[string]any{"prompt": "find it"})
	for i := range 2 {
		if d := decodeOutput(t, ta.hook(t, hostClaude, unset)).HookSpecificOutput.PermissionDecision; d != "deny" {
			t.Fatalf("deny %d: decision %q", i+1, d)
		}
	}
	released := decodeOutput(t, ta.hook(t, hostClaude, unset)).HookSpecificOutput
	if released.PermissionDecision != "" || !strings.Contains(released.AdditionalContext, "2 spawns in a row were denied") {
		t.Fatalf("the third spawn was not released with a note: %+v", released)
	}
	if recs := ta.ledger(t); !contains(recs[2].Codes, codeDenyLoopReleased) || recs[2].Decision != "allow" {
		t.Fatalf("release not recorded: %+v", recs[2])
	}
	ta.hook(t, hostClaude, claudeSpawn("s", map[string]any{"prompt": "find it", "model": "haiku"})) // a compliant spawn resets
	if d := decodeOutput(t, ta.hook(t, hostClaude, unset)).HookSpecificOutput.PermissionDecision; d != "deny" {
		t.Fatalf("after a compliant spawn the streak must reset; decision %q", d)
	}
}

func TestDenyNamesAnAgentsPinnedModel(t *testing.T) {
	ta := newTestApp(t)
	agents := filepath.Join(ta.claudeHome(), "agents")
	_ = os.MkdirAll(agents, 0o700)
	_ = os.WriteFile(filepath.Join(agents, "cavecrew-investigator.md"), []byte("---\nname: cavecrew-investigator\nmodel: haiku\n---\nbody"), 0o600)
	out := decodeOutput(t, ta.hook(t, hostClaude, claudeSpawn("s", map[string]any{"prompt": "find it", "subagent_type": "cavecrew-investigator"})))
	if !strings.Contains(out.HookSpecificOutput.PermissionDecisionReason, `agent "cavecrew-investigator" defines model "haiku"`) {
		t.Fatalf("deny reason does not name the pinned model:\n%s", out.HookSpecificOutput.PermissionDecisionReason)
	}
	for _, c := range []struct{ host, text, want string }{
		{hostClaude, "---\nmodel: inherit\n---", ""},
		{hostClaude, "no frontmatter\nmodel: haiku", ""},
		{hostCodex, "name = \"x\"\nmodel = \"cx-small\"\n", "cx-small"},
		{hostCodex, "model = cx-small", ""},
	} {
		if got := agentFileModel(c.host, c.text); got != c.want {
			t.Errorf("agentFileModel(%s, %q) = %q, want %q", c.host, c.text, got, c.want)
		}
	}
	if ta.definedModel(hostClaude, "", "../escape") != "" {
		t.Error("an unsafe agent name was used to build a path")
	}
}

func TestDoctorFailsWhenAllHooksAreDisabled(t *testing.T) {
	ta := newTestApp(t)
	installHooks(t, ta, hostClaude, nil)
	path := filepath.Join(ta.claudeHome(), "settings.json")
	raw, _ := os.ReadFile(path)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	doc["disableAllHooks"] = true
	raw, _ = json.Marshal(doc)
	_ = os.WriteFile(path, raw, 0o600)
	if code := ta.run([]string{"doctor", "--host", hostClaude}); code != 1 || !strings.Contains(ta.out.String(), "disableAllHooks") {
		t.Fatalf("doctor exit %d:\n%s", code, ta.out.String())
	}
}

func TestWireKeepsCommandBytes(t *testing.T) {
	ta := newTestApp(t)
	original := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"cd /x && run --a<b>","timeout":5}]}]}}`
	path := writeHostFile(t, ta, hostCodex, original)
	ta.run([]string{"wire", "--host", hostCodex})
	raw, _ := os.ReadFile(path)
	if !bytes.Contains(raw, []byte(`"cd /x && run --a<b>"`)) {
		t.Fatalf("an existing hook command was re-escaped (its trust hash could change):\n%s", raw)
	}
}

func TestWireWritesThroughASymlink(t *testing.T) {
	ta := newTestApp(t)
	target := filepath.Join(ta.dir, "managed", "hooks.json")
	_ = os.MkdirAll(filepath.Dir(target), 0o700)
	_ = os.WriteFile(target, []byte(`{}`), 0o600)
	link := ta.hostHookFile(hostCodex)
	_ = os.MkdirAll(filepath.Dir(link), 0o700)
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	ta.run([]string{"wire", "--host", hostCodex})
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("wire replaced the symlink with a regular file")
	}
	if raw, _ := os.ReadFile(target); !bytes.Contains(raw, []byte("agent-hooks")) {
		t.Fatal("the symlink's target was not wired")
	}
}

func TestRunsAgentHooksFlagForms(t *testing.T) {
	for cmd, want := range map[string]bool{
		"/bin/agent-hooks hook --host claude": true,
		"/bin/agent-hooks hook --host=claude": true,
		"/bin/agent-hooks hook --host codex":  false,
		"/bin/other hook --host claude":       false,
	} {
		if got := runsAgentHooks(cmd, hostClaude); got != want {
			t.Errorf("runsAgentHooks(%q) = %v", cmd, got)
		}
	}
}

func TestShellWordsAndEachLine(t *testing.T) {
	got := shellWords(`a 'b c' "d e" f\ g`, 0)
	if strings.Join(got, "|") != "a|b c|d e|f g" {
		t.Fatalf("shellWords = %q", got)
	}
	var lines []string
	err := eachLine(strings.NewReader("one\n"+strings.Repeat("x", 100)+"\nthree"), 10, func(b []byte) { lines = append(lines, string(b)) })
	if err != nil || strings.Join(lines, "|") != "one|three" {
		t.Fatalf("eachLine = %q, %v", lines, err)
	}
}
