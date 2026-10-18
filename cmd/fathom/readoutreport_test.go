package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// reportEnvelope is a PostToolUse(Bash) envelope for session s-1 with the
// given tool-use id.
func reportEnvelope(t *testing.T, cwd, id, command, stdout string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"session_id": "s-1", "cwd": cwd, "hook_event_name": "PostToolUse", "tool_name": "Bash", "tool_use_id": id,
		"tool_input":    map[string]any{"command": command},
		"tool_response": map[string]any{"stdout": stdout, "stderr": "", "interrupted": false, "isImage": false},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func runReadoutReport(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := cmdReadout(append([]string{"report"}, args...), strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

func reportJSON(t *testing.T, args ...string) roReport {
	t.Helper()
	code, out, errOut := runReadoutReport(t, append(args, "--json")...)
	if code != 0 {
		t.Fatalf("readout report --json: exit %d, stderr %q", code, errOut)
	}
	var rep roReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("readout report --json: %v\n%s", err, out)
	}
	return rep
}

func reportRule(t *testing.T, rep roReport, ref string) roReportRule {
	t.Helper()
	for _, r := range rep.Rules {
		if r.Rule == ref {
			return r
		}
	}
	t.Fatalf("no rule %s in the report: %+v", ref, rep.Rules)
	return roReportRule{}
}

// TestReadoutReportJoinsBytesToReferenceAndRecovery: the hook's rows,
// across both log generations, join to the transcript by tool-use id; a
// drop a later call names is referenced, a drop no call names is clean,
// and a row whose rule text changed or whose transcript is gone counts
// apart, never as clean. With no lean row the recovery reads `no lean
// rows`, never 0; lean rows then join a repeat within five calls to the
// recovery rate and charge its bytes against the bytes removed.
func TestReadoutReportJoinsBytesToReferenceAndRecovery(t *testing.T) {
	fh := readoutHookEnv(t, "[readouts]\nmode = \"observe\"\n")
	cwd := t.TempDir()
	root := t.TempDir()
	useReadoutReplayEnv(t, root, time.Now().Add(time.Hour))
	listing := readoutListing("")
	hook := func(id string) {
		t.Helper()
		code, out, errOut, _ := runReadoutHook(t, []string{"--event", "post-tool"}, reportEnvelope(t, cwd, id, "ls -1", listing))
		if code != 0 || out != "" || errOut != "" {
			t.Fatalf("readout-hook %s: exit %d, stdout %q, stderr %q", id, code, out, errOut)
		}
	}
	rules := filepath.Join(fh, "readouts.toml")
	earlier := strings.Replace(readoutTestRule, `about = "ls: the first two entries"`, `about = "ls: two entries"`, 1)
	if err := os.WriteFile(rules, []byte(earlier), 0o644); err != nil {
		t.Fatal(err)
	}
	hook("toolu_changed")
	logPath := os.Getenv("FATHOM_HOOKS_LOG")
	if err := os.Rename(logPath, logPath+".1"); err != nil { // the rotated generation holds the earlier rule's row
		t.Fatal(err)
	}
	if err := os.WriteFile(rules, []byte(readoutTestRule), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"toolu_ref", "toolu_clean", "toolu_gone", "toolu_differs"} {
		hook(id)
	}
	rows := readoutRows(t)
	if len(rows) != 4 || rows[0].Meta["rule_sha"] == "" {
		t.Fatalf("live generation rows %+v, want 4 with a rule_sha", rows)
	}
	removed := 0
	for _, r := range rows {
		var in, out int
		fmt.Sscan(r.Meta["bytes_in"], &in)
		fmt.Sscan(r.Meta["bytes_out"], &out)
		removed = in - out
	}
	if removed <= 0 {
		t.Fatalf("rows remove %d bytes, want a lossy readout", removed)
	}

	tr := &replayFixture{session: "s-1"}
	tr.bash(t, "2026-10-18T10:00:00Z", "toolu_changed", "ls -1", listing, nil)
	tr.bash(t, "2026-10-18T10:00:01Z", "toolu_ref", "ls -1", listing, nil)
	tr.use(t, "2026-10-18T10:00:02Z", "toolu_r1", "Read", map[string]any{"file_path": "/repo/internal/some/longer/path/file_07.go"})
	tr.bash(t, "2026-10-18T10:00:03Z", "toolu_clean", "ls -1", listing, nil)
	tr.use(t, "2026-10-18T10:00:04Z", "toolu_r2", "Read", map[string]any{"file_path": "/repo/internal/some/longer/path/file_00.go"})
	tr.bash(t, "2026-10-18T10:00:05Z", "toolu_b1", "go vet ./...", "", nil)
	// the transcript holds another output than the hook saw: it cannot replay.
	tr.bash(t, "2026-10-18T10:00:06Z", "toolu_differs", "ls -1", readoutListing("extra.go\n"), nil)
	tr.write(t, filepath.Join(root, "-repo", "s-1.jsonl"))

	rep := reportJSON(t)
	r := reportRule(t, rep, "ls-head@1")
	if r.Hits != 5 || r.Served != 5 {
		t.Errorf("hits %d served %d, want 5 and 5", r.Hits, r.Served)
	}
	if r.BytesRemoved != int64(5*removed) || r.EstTokens != int64(5*removed/4) {
		t.Errorf("bytes removed %d est. tokens %d, want %d and %d", r.BytesRemoved, r.EstTokens, 5*removed, 5*removed/4)
	}
	if r.Reference.Referenced != 1 || r.Reference.Of != 2 {
		t.Errorf("reference %+v, want 1 of 2 (the clean drop counted, the not computable ones apart)", r.Reference)
	}
	nc := r.NotComputable
	if nc.Total != 3 || nc.RuleChanged != 1 || nc.TranscriptGone != 1 || nc.OutputDiffers != 1 {
		t.Errorf("not computable %+v, want 3: rule changed, transcript gone and output differs 1 each", nc)
	}
	if r.Recovery != nil || r.RecoveryNote != "no lean rows" {
		t.Errorf("recovery %+v note %q, want none and `no lean rows`", r.Recovery, r.RecoveryNote)
	}
	if r.Verdict != "insufficient" {
		t.Errorf("verdict %q, want insufficient under 200 replayed readouts", r.Verdict)
	}
	if rep.Window.From == "" || rep.Window.Incomplete {
		t.Errorf("window %+v, want one asserted from the first row", rep.Window)
	}

	code, text, _ := runReadoutReport(t)
	if code != 0 {
		t.Fatalf("readout report: exit %d", code)
	}
	for _, want := range []string{"ls-head@1", "verdict insufficient", fmt.Sprintf("bytes removed %d bytes", 5*removed),
		"est. tokens", "estimate", "1 of 2", "not computable 3", "rule changed 1", "transcript gone 1", "output differs 1", "recovery: no lean rows",
		"net bytes: no lean rows"} {
		if !strings.Contains(text, want) {
			t.Errorf("report text lacks %q:\n%s", want, text)
		}
	}
	for _, never := range []string{"s-1", "toolu_", "ls -1", "file_07", "/repo"} {
		if strings.Contains(text, never) {
			t.Errorf("report text carries %q (aggregates only):\n%s", never, text)
		}
	}

	// Lean rows: one followed by a repeat of its command, one not.
	rs, err := readout.LoadRules(fathomhome.FathomHome())
	if err != nil {
		t.Fatal(err)
	}
	rule := readoutRuleByRef(rs, "ls-head@1")
	if rule == nil {
		t.Fatal("ls-head@1 not loaded")
	}
	cmd := shellcmd.ParseCommand("ls -1")
	ro := readout.Render(&readout.RuleSet{Rules: []*readout.Rule{rule}}, readout.Input{Command: &cmd, Stdout: listing, Mode: readout.ModeLean})
	if ro.Raw || ro.Mode != readout.ModeLean {
		t.Fatalf("lean render raw %v mode %q (%s), want a lean readout", ro.Raw, ro.Mode, ro.Reason)
	}
	for _, id := range []string{"toolu_lean", "toolu_lean2"} {
		logReadoutRow(readoutRow{event: "post-tool", configured: "lean", cmd: &cmd, ro: ro, rule: rule,
			in: harness.HookInput{Command: "ls -1", SessionID: "s-1", ToolUseID: id}})
	}
	// toolu_lean2's next five calls never repeat it; toolu_lean's next one does.
	tr.bash(t, "2026-10-18T10:01:00Z", "toolu_lean2", "ls -1", listing, nil)
	for i := range 5 {
		tr.bash(t, "2026-10-18T10:01:01Z", fmt.Sprintf("toolu_v%d", i), "go vet ./...", "", nil)
	}
	tr.bash(t, "2026-10-18T10:01:02Z", "toolu_lean", "ls -1", listing, nil)
	tr.bash(t, "2026-10-18T10:01:03Z", "toolu_repeat", "ls -1", listing, nil)
	tr.write(t, filepath.Join(root, "-repo", "s-1.jsonl"))

	r = reportRule(t, reportJSON(t), "ls-head@1")
	if r.Recovery == nil || r.RecoveryNote != "" {
		t.Fatalf("recovery %+v note %q, want the lean rows' recovery", r.Recovery, r.RecoveryNote)
	}
	leanRemoved := int64(ro.BytesIn - ro.BytesOut)
	want := roRecovery{LeanRows: 2, Recovered: 1, Of: 2, NetBytes: 2*leanRemoved - int64(len(listing))}
	if got := *r.Recovery; got.LeanRows != want.LeanRows || got.Recovered != want.Recovered || got.Of != want.Of || got.NetBytes != want.NetBytes {
		t.Errorf("recovery %+v, want %+v (the repeat's bytes charged against the bytes removed)", got, want)
	}
	if r.Hits != 7 || r.Reference.Of != 4 {
		t.Errorf("hits %d reference of %d, want 7 and 4", r.Hits, r.Reference.Of)
	}
}

// TestReadoutReportVerdictBoundaries pins ADR-071 decision 9's bars: 200
// replayed readouts and a clean share whose 95% Wilson lower bound is at
// least 0.90 promote; a lean recovery rate whose lower bound exceeds 5%
// demotes.
func TestReadoutReportVerdictBoundaries(t *testing.T) {
	for _, c := range []struct {
		name                string
		of, referenced      int
		recovered, leanRows int
		want                string
	}{
		{"199 clean replayed is under the 200 bar", 199, 0, 0, 0, "insufficient"},
		{"200 all clean promote", 200, 0, 0, 0, "promote"},
		{"189 of 200 clean reads a lower bound of 0.904", 200, 11, 0, 0, "promote"},
		{"188 of 200 clean reads 0.898, under the bar", 200, 12, 0, 0, "keep"},
		{"3 of 20 recovered reads a lower bound of 0.052", 200, 0, 3, 20, "demote"},
		{"2 of 20 recovered reads 0.028", 200, 0, 2, 20, "promote"},
		{"demotion stands under the hit bar", 10, 0, 3, 20, "demote"},
		{"no lean rows never demotes", 10, 0, 0, 0, "insufficient"},
	} {
		var rec *roRecovery
		if c.leanRows > 0 {
			rec = &roRecovery{LeanRows: c.leanRows, Of: c.leanRows, Recovered: c.recovered}
		}
		if got := readoutVerdict(c.of, c.referenced, rec); got != c.want {
			t.Errorf("%s: verdict %q, want %q", c.name, got, c.want)
		}
	}
}

// TestReadoutReportSinceBoundsTheWindow: --since drops older rows, and a
// log that does not reach back to the window's start reads incomplete.
func TestReadoutReportSinceBoundsTheWindow(t *testing.T) {
	readoutHookEnv(t, "")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	useReadoutReplayEnv(t, t.TempDir(), now)
	logPath := os.Getenv("FATHOM_HOOKS_LOG")
	beat := func(at time.Time, channel, rule string) string {
		meta := map[string]string{}
		if rule != "" {
			meta = map[string]string{"rule": rule, "rule_sha": "00", "raw": "false", "bytes_in": "100", "bytes_out": "40",
				"mode": "observe", "tool_use_id": "toolu_x", "session": "s-1"}
		}
		b, err := json.Marshal(map[string]any{"channel": channel, "kind": "observed", "at": at, "meta": meta})
		if err != nil {
			t.Fatal(err)
		}
		return string(b) + "\n"
	}
	write := func(p, body string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(logPath+".1", beat(now.Add(-10*24*time.Hour), readoutChannel, "old@1"))
	write(logPath, beat(now.Add(-24*time.Hour), readoutChannel, "new@1"))

	rep := reportJSON(t, "--since", "7d")
	if len(rep.Rules) != 1 || rep.Rules[0].Rule != "new@1" || rep.Window.Incomplete {
		t.Errorf("--since 7d: rules %+v window %+v, want new@1 alone and a complete window", rep.Rules, rep.Window)
	}
	if rep.Rules[0].NotComputable.RuleChanged != 1 || rep.Rules[0].Reference.Of != 0 {
		t.Errorf("an unknown rule_sha: %+v, want not computable (rule changed), never clean", rep.Rules[0])
	}
	if all := reportJSON(t); len(all.Rules) != 2 || all.Rules[0].Rule != "new@1" || all.Rules[1].Rule != "old@1" {
		t.Errorf("no --since: rules %+v, want both generations, sorted", all.Rules)
	}

	write(logPath+".1", beat(now.Add(-3*24*time.Hour), "edit-hook", ""))
	if rep := reportJSON(t, "--since", "7d"); !rep.Window.Incomplete || rep.Window.CoveredFrom == "" {
		t.Errorf("a log reaching back 3 days under --since 7d: window %+v, want incomplete", rep.Window)
	}
	code, text, _ := runReadoutReport(t, "--since", "7d")
	if code != 0 || !strings.Contains(text, "INCOMPLETE") {
		t.Errorf("exit %d, text lacks INCOMPLETE:\n%s", code, text)
	}
	if code, _, _ := runReadoutReport(t, "--since", "soon"); code != 1 {
		t.Errorf("--since soon: exit %d, want 1", code)
	}
}
