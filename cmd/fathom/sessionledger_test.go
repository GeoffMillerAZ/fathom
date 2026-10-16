package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

// syscallMkfifo creates a FIFO at path for TestTranscriptSummaryFrom_NonRegularFile_Refused.
func syscallMkfifo(path string) error {
	return syscall.Mkfifo(path, 0o600)
}

func withTempSessionLedger(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session-ledger.jsonl")
	t.Setenv("FATHOM_SESSION_LEDGER", path)
	return path
}

// ─── transcriptSummaryFrom ──────────────────────────────────────────────────

func TestTranscriptSummaryFrom_NoPath_AllUnavailable(t *testing.T) {
	ts := transcriptSummaryFrom("")
	if ts.DurationMS != nil || ts.TurnCount != nil || ts.TokensTotal != nil {
		t.Fatalf("ts = %+v, want all nil", ts)
	}
	if ts.DurationNote == "" || ts.TurnNote == "" || ts.TokensNote == "" {
		t.Errorf("ts = %+v, want every note populated", ts)
	}
}

func TestTranscriptSummaryFrom_UnreadableFile_AllUnavailable(t *testing.T) {
	ts := transcriptSummaryFrom(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
	if ts.TokensTotal != nil {
		t.Errorf("TokensTotal = %v, want nil", ts.TokensTotal)
	}
	if ts.TokensNote == "" {
		t.Error("expected a TokensNote")
	}
}

func TestTranscriptSummaryFrom_ValidTranscript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	lines := []string{
		`{"type":"user","timestamp":"2026-07-29T12:00:00Z"}`,
		`{"type":"assistant","timestamp":"2026-07-29T12:00:05Z","message":{"usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":0,"cache_read_input_tokens":10}}}`,
		`{"type":"user","timestamp":"2026-07-29T12:00:10Z"}`,
		`{"type":"assistant","timestamp":"2026-07-29T12:01:00Z","message":{"usage":{"input_tokens":200,"output_tokens":80,"cache_creation_input_tokens":5,"cache_read_input_tokens":0}}}`,
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ts := transcriptSummaryFrom(path)
	if ts.TurnCount == nil || *ts.TurnCount != 2 {
		t.Fatalf("TurnCount = %v, want 2", ts.TurnCount)
	}
	wantTokens := 100 + 50 + 10 + 200 + 80 + 5
	if ts.TokensTotal == nil || *ts.TokensTotal != wantTokens {
		t.Fatalf("TokensTotal = %v, want %d", ts.TokensTotal, wantTokens)
	}
	if ts.DurationMS == nil || *ts.DurationMS != 60000 {
		t.Fatalf("DurationMS = %v, want 60000 (12:00:00 to 12:01:00)", ts.DurationMS)
	}
	if ts.TurnNote != "" || ts.TokensNote != "" || ts.DurationNote != "" {
		t.Errorf("expected no notes on a fully-resolved transcript, got turn=%q tokens=%q duration=%q", ts.TurnNote, ts.TokensNote, ts.DurationNote)
	}
}

func TestTranscriptSummaryFrom_NoUsageData_TokensUnavailableNeverZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	content := `{"type":"user","timestamp":"2026-07-29T12:00:00Z"}` + "\n" +
		`{"type":"assistant","timestamp":"2026-07-29T12:00:05Z"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ts := transcriptSummaryFrom(path)
	if ts.TokensTotal != nil {
		t.Fatalf("TokensTotal = %v, want nil (never a fabricated zero)", ts.TokensTotal)
	}
	if ts.TokensNote == "" {
		t.Error("expected a TokensNote explaining unavailability")
	}
	// Turn count IS determinable even with no usage data.
	if ts.TurnCount == nil || *ts.TurnCount != 1 {
		t.Errorf("TurnCount = %v, want 1", ts.TurnCount)
	}
}

func TestTranscriptSummaryFrom_MalformedLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	content := "not json at all\n" +
		`{"type":"assistant","timestamp":"2026-07-29T12:00:05Z","message":{"usage":{"input_tokens":10,"output_tokens":5}}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ts := transcriptSummaryFrom(path)
	if ts.TurnCount == nil || *ts.TurnCount != 1 {
		t.Fatalf("TurnCount = %v, want 1 (malformed line skipped, not fatal)", ts.TurnCount)
	}
	if ts.TokensTotal == nil || *ts.TokensTotal != 15 {
		t.Fatalf("TokensTotal = %v, want 15", ts.TokensTotal)
	}
}

// ─── buildSessionLedgerEntry / sumSessionBeats ─────────────────────────────

func TestBuildSessionLedgerEntry_JoinsInstrumentAAndB(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	beats := []heartbeat.Beat{
		{Channel: "payload-cost", Kind: "post-edit", At: now, Meta: map[string]string{"session": "sess-1", "tokens_estimate": "42"}},
		{Channel: "payload-cost", Kind: "pre-edit-guard", At: now, Meta: map[string]string{"session": "sess-1", "tokens_estimate": "8"}},
		{Channel: "payload-cost", Kind: "post-edit", At: now, Meta: map[string]string{"session": "sess-OTHER", "tokens_estimate": "999"}},
		{Channel: "discovery", Kind: "grep", At: now, Meta: map[string]string{"session": "sess-1"}},
		{Channel: "discovery", Kind: "read", At: now, Meta: map[string]string{"session": "sess-1"}},
		{Channel: "discovery", Kind: "grep", At: now, Meta: map[string]string{"session": "sess-OTHER"}},
	}
	ts := transcriptSummary{TokensNote: "unavailable: test", TurnNote: "unavailable: test", DurationNote: "unavailable: test"}
	entry := buildSessionLedgerEntry("sess-1", "fathom", ts, beats, now)

	if entry.SessionID != "sess-1" || entry.Repo != "fathom" {
		t.Fatalf("entry = %+v", entry)
	}
	if entry.PayloadCostEstimateTotal == nil || *entry.PayloadCostEstimateTotal != 50 {
		t.Fatalf("PayloadCostEstimateTotal = %v, want 50", entry.PayloadCostEstimateTotal)
	}
	if entry.DiscoveryActionsTotal == nil || *entry.DiscoveryActionsTotal != 2 {
		t.Fatalf("DiscoveryActionsTotal = %v, want 2", entry.DiscoveryActionsTotal)
	}
	if entry.TokensNote == "" || entry.TokensTotal != nil {
		t.Errorf("expected TokensTotal nil + a note, got %+v", entry)
	}
}

func TestBuildSessionLedgerEntry_NoSessionID_JoinNoted(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	entry := buildSessionLedgerEntry("", "fathom", transcriptSummary{}, nil, now)
	if entry.PayloadCostEstimateTotal != nil || entry.DiscoveryActionsTotal != nil {
		t.Errorf("entry = %+v, want nil A/B totals with no session_id", entry)
	}
	if entry.JoinNote == "" {
		t.Error("expected a JoinNote explaining why A/B could not be attributed")
	}
}

// ─── appendSessionLedger / sessionLedgerPath ───────────────────────────────

func TestAppendSessionLedger_WritesOneJSONLine(t *testing.T) {
	path := withTempSessionLedger(t)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	appendSessionLedger(buildSessionLedgerEntry("sess-1", "fathom", transcriptSummary{TokensNote: "unavailable"}, nil, now))
	appendSessionLedger(buildSessionLedgerEntry("sess-2", "fathom", transcriptSummary{TokensNote: "unavailable"}, nil, now))

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []sessionLedgerEntry
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var e sessionLedgerEntry
		if err := dec.Decode(&e); err != nil {
			break
		}
		lines = append(lines, e)
	}
	if len(lines) != 2 {
		t.Fatalf("wrote %d line(s), want 2", len(lines))
	}
	if lines[0].SessionID != "sess-1" || lines[1].SessionID != "sess-2" {
		t.Errorf("lines = %+v", lines)
	}
}

func TestSessionLedgerPath_EnvOverride(t *testing.T) {
	t.Setenv("FATHOM_SESSION_LEDGER", "/custom/path.jsonl")
	if got := sessionLedgerPath(); got != "/custom/path.jsonl" {
		t.Errorf("sessionLedgerPath() = %q", got)
	}
}

// ─── end-to-end: cmdStopHook appends a ledger line ─────────────────────────

func TestStopHook_AppendsSessionLedgerEntry(t *testing.T) {
	ledgerPath := withTempSessionLedger(t)
	withTempBeatLog(t)
	dir := t.TempDir()

	hookInput := `{"cwd":"` + dir + `","session_id":"sess-e2e"}`
	hookStdin(t, hookInput)
	runWithStdin([]string{"stop-hook", "--url=http://127.0.0.1:1"}, nil, io.Discard, io.Discard)

	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("expected the ledger to be written: %v", err)
	}
	var entry sessionLedgerEntry
	if err := json.Unmarshal(data[:len(data)-1], &entry); err != nil { // trailing \n
		t.Fatalf("unmarshal ledger line: %v\n%s", err, data)
	}
	if entry.SessionID != "sess-e2e" {
		t.Errorf("SessionID = %q, want sess-e2e", entry.SessionID)
	}
	if entry.TokensNote == "" && entry.TokensTotal == nil {
		t.Error("expected either a real TokensTotal or a TokensNote, never neither")
	}
}

func TestStopHook_ChannelsOff_NoSessionLedgerEntry(t *testing.T) {
	ledgerPath := withTempSessionLedger(t)
	t.Setenv("FATHOM_CHANNELS", "off")
	dir := t.TempDir()
	hookInput := `{"cwd":"` + dir + `","session_id":"sess-off"}`
	hookStdin(t, hookInput)
	runWithStdin([]string{"stop-hook"}, nil, io.Discard, io.Discard)

	if _, err := os.Stat(ledgerPath); err == nil {
		t.Fatal("expected no ledger file when FATHOM_CHANNELS=off")
	}
}

// ─── critic finding F3 (BLOCKING): no raw OS error text ever reaches the ledger ───

// TestTranscriptSummaryFrom_UnreadablePath_NeverLeaksErrorText proves the
// unreadable-file note is a FIXED string, never err.Error() (which on a
// real machine embeds the full $HOME, username, and repo layout — exactly
// the class of leak this ledger, written 0600 but still a plain file,
// must never carry).
func TestTranscriptSummaryFrom_UnreadablePath_NeverLeaksErrorText(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "Users", "jdoe", "secret-project", "does-not-exist.jsonl")
	ts := transcriptSummaryFrom(secretPath)
	for _, note := range []string{ts.DurationNote, ts.TurnNote, ts.TokensNote} {
		if strings.Contains(note, "jdoe") || strings.Contains(note, "secret-project") || strings.Contains(note, secretPath) {
			t.Fatalf("note leaks the path: %q", note)
		}
	}
}

// TestAppendSessionLedger_FilePermissions and
// TestAppendSessionLedger_DirPermissions prove the ledger gets the SAME
// hardening guardstamp.go/identcache.go use — 0600 file in a 0700 dir,
// not the original 0644/0755.
func TestAppendSessionLedger_FilePermissions(t *testing.T) {
	// A nested, not-yet-existing subdirectory: os.MkdirAll only sets the
	// mode on directories it actually CREATES, so the dir-permission
	// assertion below is only meaningful against a fresh path — not one
	// t.TempDir() (0755-ish by default) already created for us.
	path := filepath.Join(t.TempDir(), "fresh-home", ".fathom", "session-ledger.jsonl")
	t.Setenv("FATHOM_SESSION_LEDGER", path)
	appendSessionLedger(buildSessionLedgerEntry("sess-1", "fathom", transcriptSummary{TokensNote: "unavailable"}, nil, time.Now()))

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("ledger file perm = %o, want 0600", perm)
	}
	dfi, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if perm := dfi.Mode().Perm(); perm != 0o700 {
		t.Errorf("ledger dir perm = %o, want 0700", perm)
	}
}

// ─── critic finding F5 (BLOCKING): safe transcript reads ───────────────────

func TestTranscriptSummaryFrom_SymlinkRefused(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.jsonl")
	if err := os.WriteFile(real, []byte(`{"type":"assistant","timestamp":"2026-07-29T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.jsonl")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}

	ts := transcriptSummaryFrom(link)
	if ts.TurnCount != nil {
		t.Fatal("expected a symlinked transcript_path to be refused, not followed")
	}
	if !strings.Contains(ts.TurnNote, "unavailable") {
		t.Errorf("TurnNote = %q, want an unavailable note", ts.TurnNote)
	}
}

func TestTranscriptSummaryFrom_NonRegularFile_Refused(t *testing.T) {
	dir := t.TempDir()
	fifoPath := filepath.Join(dir, "fifo")
	if err := syscallMkfifo(fifoPath); err != nil {
		t.Skipf("mkfifo unsupported here: %v", err)
	}

	done := make(chan transcriptSummary, 1)
	go func() { done <- transcriptSummaryFrom(fifoPath) }()

	select {
	case ts := <-done:
		if ts.TurnCount != nil {
			t.Fatal("expected a FIFO to be refused as non-regular, not read")
		}
	case <-time.After(waitSafety):
		t.Fatal("transcriptSummaryFrom hung reading a FIFO — the IsRegular guard did not fire")
	}
}

func TestTranscriptSummaryFrom_OversizeFile_TruncatedNotHung(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	line := []byte(`{"type":"assistant","timestamp":"2026-07-29T12:00:00Z"}` + "\n")
	target := int64(transcriptReadMaxBytes) + int64(len(line))*2
	for written := int64(0); written < target; written += int64(len(line)) {
		if _, err := f.Write(line); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()

	ts := transcriptSummaryFrom(path)
	if !ts.Truncated {
		t.Error("expected Truncated=true for a file over the read cap")
	}
	if ts.TurnCount == nil || *ts.TurnCount == 0 {
		t.Error("expected a real (nonzero) TurnCount from the truncated-but-still-parsed portion")
	}
}

func TestBuildSessionLedgerEntry_TranscriptSchemaNote(t *testing.T) {
	now := time.Now()
	tc := 3
	ts := transcriptSummary{TurnCount: &tc, DurationNote: "unavailable: x", TokensNote: "unavailable: x"}
	entry := buildSessionLedgerEntry("sess-1", "fathom", ts, nil, now)
	if entry.TranscriptSchemaNote == "" {
		t.Error("expected a TranscriptSchemaNote whenever any transcript field resolved to a real value")
	}
}

func TestBuildSessionLedgerEntry_NoTranscript_NoSchemaNote(t *testing.T) {
	now := time.Now()
	ts := transcriptSummary{DurationNote: "unavailable: x", TurnNote: "unavailable: x", TokensNote: "unavailable: x"}
	entry := buildSessionLedgerEntry("sess-1", "fathom", ts, nil, now)
	if entry.TranscriptSchemaNote != "" {
		t.Errorf("TranscriptSchemaNote = %q, want empty when no transcript field resolved at all", entry.TranscriptSchemaNote)
	}
}

// TestLedgerAttribution pins IO-15: the ledger row carries the arm it ran
// under and the verify pass's catch count — nil catches means verify never
// ran (unknown), never a fabricated zero.
func TestLedgerAttribution(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	t.Setenv("FATHOM_SESSION_LEDGER", ledger)
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(t.TempDir(), "arm.json"))

	two := 2
	recordSessionLedgerEntry("claude-code", "sess-1", "", t.TempDir(), time.Now(), &two)
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatal(err)
	}
	if row["verify_catches"] != float64(2) {
		t.Errorf("verify_catches = %v, want 2", row["verify_catches"])
	}
	if row["arm"] != "on" {
		t.Errorf("arm = %v, want on (no arm active)", row["arm"])
	}
	if row["schema_version"] != float64(2) || row["fathom_version"] == "" {
		t.Errorf("stamps missing: %v", row)
	}

	t.Run("nil_catches_is_absent_not_zero", func(t *testing.T) {
		ledger2 := filepath.Join(t.TempDir(), "l2.jsonl")
		t.Setenv("FATHOM_SESSION_LEDGER", ledger2)
		recordSessionLedgerEntry("claude-code", "sess-2", "", t.TempDir(), time.Now(), nil)
		data, _ := os.ReadFile(ledger2)
		var row map[string]any
		if err := json.Unmarshal(data, &row); err != nil {
			t.Fatal(err)
		}
		if _, present := row["verify_catches"]; present {
			t.Error("verify_catches must be ABSENT when verify never ran")
		}
	})
}

// TestFirstEditTurn pins IQ-2 (docs/plans/impact-questions.md): the ledger
// learns how many assistant turns a session spent before its first
// file-changing tool call — the warm-up fathom promises to collapse.
// nil-with-note when the transcript has no edit (or no transcript at all).
func TestFirstEditTurn(t *testing.T) {
	mk := func(lines ...string) []byte { return []byte(strings.Join(lines, "\n")) }

	t.Run("edit on turn 3", func(t *testing.T) {
		data := mk(
			`{"type":"assistant","message":{"content":[{"type":"text"}]}}`,
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read"}]}}`,
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit"}]}}`,
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash"}]}}`,
		)
		s := summarizeTranscriptBytes(data)
		if s.FirstEditTurn == nil || *s.FirstEditTurn != 3 {
			t.Fatalf("FirstEditTurn = %v, want 3", s.FirstEditTurn)
		}
	})

	t.Run("Write and NotebookEdit count as edits", func(t *testing.T) {
		data := mk(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}`)
		s := summarizeTranscriptBytes(data)
		if s.FirstEditTurn == nil || *s.FirstEditTurn != 1 {
			t.Fatalf("Write: FirstEditTurn = %v, want 1", s.FirstEditTurn)
		}
	})

	t.Run("no edit is nil with a note, never zero", func(t *testing.T) {
		data := mk(
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read"}]}}`,
			`{"type":"assistant","message":{"content":[{"type":"text"}]}}`,
		)
		s := summarizeTranscriptBytes(data)
		if s.FirstEditTurn != nil {
			t.Fatalf("FirstEditTurn = %v, want nil", *s.FirstEditTurn)
		}
		if s.FirstEditNote == "" {
			t.Fatal("nil FirstEditTurn must carry a note")
		}
	})
}

// TestToolSearchTurns is BP-23's anchor (docs/plans/bench-pyramid.md,
// ADR-032 §5): the deferral tax O24 measured — turns spent loading deferred
// MCP tool schemas — counted per session from the same transcript the
// exploration ledger is parsed from.
func TestToolSearchTurns(t *testing.T) {
	mk := func(lines ...string) []byte { return []byte(strings.Join(lines, "\n")) }

	t.Run("two opening schema loads", func(t *testing.T) {
		data := mk(
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"ToolSearch","id":"t1"}]}}`,
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"ToolSearch","id":"t2"}]}}`,
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__fathom__find_symbol","id":"t3"}]}}`,
		)
		s := summarizeTranscriptBytes(data)
		if s.ToolSearchTurns != 2 {
			t.Fatalf("ToolSearchTurns = %d, want 2", s.ToolSearchTurns)
		}
		if s.FathomCalls != 1 {
			t.Errorf("FathomCalls = %d, want 1 — ToolSearch must not be counted as a fathom call", s.FathomCalls)
		}
	})

	t.Run("two loads in one turn are one turn", func(t *testing.T) {
		data := mk(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"ToolSearch","id":"a"},{"type":"tool_use","name":"ToolSearch","id":"b"}]}}`)
		s := summarizeTranscriptBytes(data)
		if s.ToolSearchTurns != 1 {
			t.Fatalf("ToolSearchTurns = %d, want 1 — the unit is the turn, not the call", s.ToolSearchTurns)
		}
	})

	t.Run("July's shape reads a true zero", func(t *testing.T) {
		data := mk(
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Grep","id":"g1"}]}}`,
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__fathom__references","id":"f1"}]}}`,
		)
		if s := summarizeTranscriptBytes(data); s.ToolSearchTurns != 0 {
			t.Fatalf("ToolSearchTurns = %d, want 0", s.ToolSearchTurns)
		}
	})

	t.Run("a user-role line is never a turn", func(t *testing.T) {
		var tl transcriptLine
		if err := json.Unmarshal([]byte(`{"type":"user","message":{"content":[{"type":"tool_use","name":"ToolSearch"}]}}`), &tl); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if n := toolSearchTurns(tl); n != 0 {
			t.Errorf("toolSearchTurns(user line) = %d, want 0", n)
		}
	})
}
