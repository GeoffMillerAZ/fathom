package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSessionPageSeeksIndexedBeats (RT-114, design §14): a session's beats
// come from a per-session offset index into hooks.jsonl and its rotated
// generation, extended by what was appended since the last read — never a
// whole-log read per page. A rotation reuses the rotated file's index; the
// replay and the legacy /observatory/session story read the same index,
// older generation first; a log that cannot be read is an error, never "no
// beats".
func TestSessionPageSeeksIndexedBeats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", path)
	beat := func(session, at, kind string) string {
		return `{"channel":"edit-hook","kind":"` + kind + `","at":"` + at + `","meta":{"file":"a.go","session":"` + session + `"}}` + "\n"
	}
	mustWrite(t, path+".1", beat("s-a", "2026-09-26T10:00:00Z", "seen")+beat("s-b", "2026-09-26T10:01:00Z", "seen"))
	mustWrite(t, path, beat("s-a", "2026-09-27T10:00:00Z", "emitted")+"not json\n"+beat("s-b", "2026-09-27T10:01:00Z", "seen"))
	ix := &beatOffsetIndex{}

	lines, partial, err := ix.lines(path, "s-a")
	if err != nil || partial != nil || len(lines) != 2 || !strings.Contains(string(lines[0]), "2026-09-26") || !strings.Contains(string(lines[1]), "2026-09-27") {
		t.Fatalf("s-a = %q, %v, %v; want its two beats, the rotated generation first", lines, partial, err)
	}
	indexed := ix.parsed
	if _, _, err := ix.lines(path, "s-b"); err != nil || ix.parsed != indexed {
		t.Fatalf("a second session re-read the log: parsed %d → %d (%v)", indexed, ix.parsed, err)
	}

	// An append is indexed alone.
	app := beat("s-b", "2026-09-27T11:00:00Z", "error")
	fh, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	fh.WriteString(app)
	fh.Close()
	lines, _, _ = ix.lines(path, "s-b")
	if len(lines) != 3 || ix.parsed-indexed != int64(len(app)) {
		t.Fatalf("after an append: %d lines, parsed %d bytes; want 3 lines and only the %d appended bytes", len(lines), ix.parsed-indexed, len(app))
	}

	// Rotation: the current file becomes .1 (its index moves with it) and
	// a new current starts; only the new file is read.
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	fresh := beat("s-a", "2026-09-27T12:00:00Z", "seen")
	mustWrite(t, path, fresh)
	before := ix.parsed
	lines, _, _ = ix.lines(path, "s-a")
	if len(lines) != 2 || !strings.Contains(string(lines[0]), "10:00:00") || !strings.Contains(string(lines[1]), "12:00:00") {
		t.Fatalf("after rotation s-a = %q", lines)
	}
	if got := ix.parsed - before; got != int64(len(fresh)) {
		t.Errorf("rotation parsed %d bytes; want only the new current file's %d", got, len(fresh))
	}

	// The replay and the legacy story read the shared index.
	prev := hooksBeatIndex
	hooksBeatIndex = ix
	t.Cleanup(func() { hooksBeatIndex = prev })
	events, _, err := readSessionEvents(path, "s-a")
	if err != nil || len(events) != 2 || events[1].Kind != "seen" {
		t.Fatalf("replay = %+v, %v", events, err)
	}
	st := buildSessionStory("s-b")
	if len(st.Beats) != 2 || st.Beats[1].Kind != "error" {
		t.Fatalf("story beats = %+v", st.Beats)
	}
	if ix.parsed != before+int64(len(fresh)) {
		t.Errorf("a page read re-read the log: parsed %d more bytes", ix.parsed-before-int64(len(fresh)))
	}

	// Missing: not-exist; unreadable current: an error, never no beats.
	if _, _, err := ix.lines(filepath.Join(dir, "none.jsonl"), "s-a"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing log = %v", err)
	}
	if os.Geteuid() != 0 {
		os.Chmod(path, 0o000)
		defer os.Chmod(path, 0o644)
		if _, _, err := ix.lines(path, "s-a"); err == nil {
			t.Error("an unreadable current log must be an error")
		}
	}
}

// TestHooksLogReadersUseTheIndex (RT-114, design §14: "nothing reads
// hooks.jsonl whole"): the catches timeline's guard beats, the impact
// model's per-session beat-windows and the flywheel's last custodian audit
// come from the hooks-log index's digests — only appended bytes are
// decoded — and answer what the whole-file readers answered: the guard
// beats over both generations, the windows and audits over the current
// file (readRetroBeats'), an unterminated last line counted.
func TestHooksLogReadersUseTheIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.jsonl")
	guard := func(at, kind, session string) string {
		return `{"channel":"edit-hook","kind":"` + kind + `","at":"` + at + `","meta":{"phase":"pre","target":"a.go","project_dir":"/r/pay","session":"` + session + `"}}` + "\n"
	}
	cust := func(at string) string {
		return `{"channel":"custodian","kind":"audit","at":"` + at + `","meta":{"project_dir":"/r/pay"}}` + "\n"
	}
	mustWrite(t, path+".1", guard("2026-09-20T10:00:00Z", "seen", "s1")+cust("2026-09-20T11:00:00Z"))
	mustWrite(t, path, guard("2026-09-26T10:00:00Z", "answered", "s1")+"not json\n"+
		`{"channel":"edit-hook","kind":"seen","at":"2026-09-26T10:05:00Z","meta":{"phase":"post","project_dir":"/r/pay","session":"s1"}}`+"\n"+
		cust("2026-09-25T09:00:00Z"))
	ix := &beatOffsetIndex{}
	prev := hooksBeatIndex
	hooksBeatIndex = ix
	t.Cleanup(func() { hooksBeatIndex = prev })
	label := func(base string) string { return map[string]string{"pay": "payments"}[base] }

	beats, partial, err := readGuardBeats(path, label)
	if err != nil || partial != nil || len(beats) != 2 || beats[0].Kind != "seen" || beats[1].Kind != "answered" || beats[1].Service != "payments" || beats[1].Session != "s1" {
		t.Fatalf("guard beats = %+v, %v, %v; want the rotated beat then the current one, named payments", beats, partial, err)
	}
	if ix.parsed == 0 {
		t.Fatal("the guard beats were not read through the index")
	}
	// The impact model's window and the flywheel's audit: the current
	// file, as readRetroBeats reads it.
	whole, _ := readRetroBeats(path)
	start, end, ok := ix.sessionSpan(path, "s1", "/r/pay")
	if !ok || !start.Equal(time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)) || !end.Equal(time.Date(2026, 9, 26, 10, 5, 0, 0, time.UTC)) {
		t.Errorf("span = %s..%s %v; want the current file's 10:00..10:05", start, end, ok)
	}
	var wantAudit time.Time
	for _, b := range whole {
		if b.Channel == "custodian" && b.At.After(wantAudit) {
			wantAudit = b.At
		}
	}
	if got := ix.lastCustodianAudit(path, "/r/pay"); !got.Equal(wantAudit) {
		t.Errorf("last custodian audit = %s, want %s", got, wantAudit)
	}

	// An append is decoded alone; an unterminated last line counts.
	before := ix.parsed
	app := guard("2026-09-27T10:00:00Z", "seen", "s2")
	fh, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	fh.WriteString(app + strings.TrimSuffix(cust("2026-09-27T12:00:00Z"), "\n"))
	fh.Close()
	beats, _, _ = readGuardBeats(path, label)
	if len(beats) != 3 || ix.parsed-before != int64(len(app)) {
		t.Fatalf("after an append: %d beats, %d bytes decoded; want 3 and only the %d appended", len(beats), ix.parsed-before, len(app))
	}
	if got := ix.lastCustodianAudit(path, "/r/pay"); !got.Equal(time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("an unterminated audit line was not counted: %s", got)
	}
	if _, _, ok := ix.sessionSpan(path, "nobody", "/r/pay"); ok {
		t.Error("a session with no beats has a window")
	}
	if _, _, err := readGuardBeats(filepath.Join(dir, "none.jsonl"), label); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing log = %v", err)
	}
}

// TestShedKeepsTailsIncremental (RT-113, RT-114): the shed rung frees the
// console's derived caches and the beat index's cached lines, but keeps
// the tails' read positions and folds and the index's offsets — the next
// read under memory pressure reads only what was appended, never the whole
// call log, session ledger or hooks log again.
func TestShedKeepsTailsIncremental(t *testing.T) {
	dir := t.TempDir()
	hooks := filepath.Join(dir, "hooks.jsonl")
	mustWrite(t, hooks, `{"channel":"edit-hook","kind":"seen","at":"2026-09-27T10:00:00Z","meta":{"session":"s-a"}}`+"\n")
	calls := filepath.Join(dir, "calls.jsonl")
	mustWrite(t, calls, `{"ts":"2026-09-27T10:00:00Z","tool":"find_symbol","service":"svc"}`+"\n")
	ledger := filepath.Join(dir, "sessions.jsonl")
	mustWrite(t, ledger, `{"session_id":"a","repo":"svc","at":"2026-09-27T10:00:00Z"}`+"\n")
	hooksBeatIndex.reset()
	resetConsoleTails()
	t.Cleanup(func() { hooksBeatIndex.reset(); resetConsoleTails() })

	if _, _, err := hooksBeatIndex.lines(hooks, "s-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := consoleCallLogTail.read(calls); err != nil {
		t.Fatal(err)
	}
	if _, err := consoleSessionLedger.read(ledger); err != nil {
		t.Fatal(err)
	}
	beats, callBytes, ledgerBytes := hooksBeatIndex.parsed, consoleCallLogTail.parsed, consoleSessionLedger.parsed

	consoleMemoDrops()
	hooksBeatIndex.mu.Lock()
	cached := len(hooksBeatIndex.lru)
	hooksBeatIndex.mu.Unlock()
	if cached != 0 {
		t.Errorf("the shed kept %d sessions' cached lines; want them freed", cached)
	}
	consoleCallLogTail.mu.Lock()
	callAt := consoleCallLogTail.f.offset
	consoleCallLogTail.mu.Unlock()
	consoleSessionLedger.mu.Lock()
	ledgerAt := consoleSessionLedger.f.offset
	consoleSessionLedger.mu.Unlock()
	if callAt != callBytes || ledgerAt != ledgerBytes {
		t.Errorf("the shed dropped the tails' read positions: call log at %d of %d, ledger at %d of %d", callAt, callBytes, ledgerAt, ledgerBytes)
	}

	lines, _, err := hooksBeatIndex.lines(hooks, "s-a")
	if err != nil || len(lines) != 1 {
		t.Fatalf("after the shed s-a = %d lines, %v", len(lines), err)
	}
	snap, err := consoleCallLogTail.read(calls)
	if err != nil || len(snap.ring) != 1 {
		t.Fatalf("after the shed the call log = %d ring rows, %v", len(snap.ring), err)
	}
	if _, err := consoleSessionLedger.read(ledger); err != nil {
		t.Fatal(err)
	}
	if hooksBeatIndex.parsed != beats || consoleCallLogTail.parsed != callBytes || consoleSessionLedger.parsed != ledgerBytes {
		t.Fatalf("the shed made the next reads re-read whole logs: hooks %d→%d, call log %d→%d, ledger %d→%d bytes",
			beats, hooksBeatIndex.parsed, callBytes, consoleCallLogTail.parsed, ledgerBytes, consoleSessionLedger.parsed)
	}
}
