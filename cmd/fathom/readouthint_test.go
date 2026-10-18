package main

// readouthint_test.go: H8's Read outline hint (docs/design/readouts.md
// section 20, docs/plans/readouts.md): on PreToolUse(Read) with no offset
// or limit, a long, unedited source file gets one line, its top-level
// symbols with line ranges from the file on disk and the Read offset and
// limit recipe; everything else is silent.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
)

// readOutlineHintSource is a Go file of n functions, F0 to F(n-1), each 17
// lines (its signature, 15 body lines, its brace) and a blank line after:
// F<i> spans 3+18i to 19+18i.
func readOutlineHintSource(n int) string {
	var b strings.Builder
	b.WriteString("package big\n\n")
	for i := range n {
		fmt.Fprintf(&b, "func F%d() {\n", i)
		for range 15 {
			b.WriteString("\t_ = 0\n")
		}
		b.WriteString("}\n\n")
	}
	return b.String()
}

// readOutlineHintEnv isolates one run: HOME, the hook log and the edit set
// are private to the test, and the repository's config switches the hint
// on when on is set.
func readOutlineHintEnv(t *testing.T, on bool) string {
	t.Helper()
	isolateHookRun(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOME", filepath.Join(t.TempDir(), ".fathom"))
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	t.Setenv("FATHOM_CHANNELS", "")
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	origSet := editSetDir
	editSetDir = filepath.Join(t.TempDir(), "edit-set")
	t.Cleanup(func() { editSetDir = origSet })
	// The 150 ms join deadline and the 300 ms edit check are wall-clock
	// budgets a loaded -race shard misses, which reads as a silent hint
	// (CI on 897da463). These cases assert what the hint says, not its
	// speed; the deadline cases set their own.
	origDeadline, origBudget := readOutlineHintDeadline, readOutlineHintEditBudget
	readOutlineHintDeadline, readOutlineHintEditBudget = time.Minute, time.Minute
	t.Cleanup(func() { readOutlineHintDeadline, readOutlineHintEditBudget = origDeadline, origBudget })
	dir := t.TempDir()
	if on {
		writeRepoFile(t, dir, ".fathom/config.toml", "[hooks]\nread_outline_hint = true\n")
	}
	return dir
}

// readOutlineHintEnvelope is a Claude Code PreToolUse(Read) envelope.
func readOutlineHintEnvelope(t *testing.T, cwd, file string, extra map[string]any) string {
	t.Helper()
	input := map[string]any{"file_path": file}
	for k, v := range extra {
		input[k] = v
	}
	b, err := json.Marshal(map[string]any{"session_id": "hint-session", "cwd": cwd, "hook_event_name": "PreToolUse",
		"tool_name": "Read", "tool_use_id": "toolu_hint", "tool_input": input})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// runReadOutlineHint runs `readout-hook --event pre-read` in process.
func runReadOutlineHint(t *testing.T, payload string) (int, string) {
	t.Helper()
	var out, errb strings.Builder
	code := cmdReadoutHook([]string{"--event", "pre-read"}, strings.NewReader(payload), &out, &errb)
	return code, out.String()
}

// readOutlineHintRows reads the hook log's rows on the hint's channel.
func readOutlineHintRows(t *testing.T) []readoutTestRow {
	t.Helper()
	f, err := os.Open(os.Getenv("FATHOM_HOOKS_LOG"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []readoutTestRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var row struct {
			Channel string `json:"channel"`
			readoutTestRow
		}
		if json.Unmarshal(sc.Bytes(), &row) == nil && row.Channel == readOutlineHintChannel {
			out = append(out, row.readoutTestRow)
		}
	}
	return out
}

func hintContext(t *testing.T, stdout string) string {
	t.Helper()
	if strings.TrimSpace(stdout) == "" {
		return ""
	}
	var env struct {
		HookSpecificOutput map[string]any `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("hook stdout is not one JSON object: %v\n%s", err, stdout)
	}
	for _, k := range []string{"permissionDecision", "updatedInput"} {
		if _, ok := env.HookSpecificOutput[k]; ok {
			t.Errorf("hook output carries %s; the hint is context only: %s", k, stdout)
		}
	}
	s, _ := env.HookSpecificOutput["additionalContext"].(string)
	return s
}

func TestReadOutlineHintOnlyForWholeReadsOfLongUneditedSourceFiles(t *testing.T) {
	dir := readOutlineHintEnv(t, true)
	big := readOutlineHintSource(14) // 254 lines
	writeRepoFile(t, dir, "big.go", big)
	bigPath := filepath.Join(dir, "big.go")

	// A whole read of a long Go file: one line, the ranges right, capped.
	code, out := runReadOutlineHint(t, readOutlineHintEnvelope(t, dir, bigPath, nil))
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	line := hintContext(t, out)
	t.Logf("hint: %s", line)
	if line == "" || strings.Contains(line, "\n") {
		t.Fatalf("whole read of a 254-line Go file: context = %q, want one line", line)
	}
	for _, want := range []string{"big.go", "254 lines", "F0 3-19", "F1 21-37", "F11 201-217", "and 2 more", "offset=3 limit=17"} {
		if !strings.Contains(line, want) {
			t.Errorf("hint lacks %q: %s", want, line)
		}
	}
	if strings.Contains(line, "F12") || strings.Contains(line, "F13") {
		t.Errorf("hint lists past its cap of %d: %s", readOutlineHintMaxSymbols, line)
	}
	for _, never := range []string{"sav", "token"} {
		if strings.Contains(strings.ToLower(line), never) {
			t.Errorf("hint claims a saving (%q): %s", never, line)
		}
	}
	rows := readOutlineHintRows(t)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want one per evaluated call", len(rows))
	}
	m := rows[0].Meta
	if m["file_sha"] != sha256Hex(big) || m["lines"] != "254" || m["symbols"] != "14" || m["lang"] != "go" || m["emitted"] != "true" {
		t.Errorf("row meta = %v, want the file's sha, 254 lines, 14 symbols, go, emitted", m)
	}
	if m["ms"] == "" || m["missed"] == "true" {
		t.Errorf("row meta = %v, want ms and no miss", m)
	}
	for k, v := range m {
		if strings.Contains(v, "func F") || (k != "file" && k != "project_dir" && strings.Contains(v, "big.go")) {
			t.Errorf("row %s = %q carries file text or a path beyond the file key", k, v)
		}
	}

	silent := func(name, payload string) {
		t.Helper()
		before := len(readOutlineHintRows(t))
		code, out := runReadOutlineHint(t, payload)
		if code != 0 || out != "" {
			t.Errorf("%s: exit %d, stdout %q, want exit 0 and nothing", name, code, out)
		}
		if got := len(readOutlineHintRows(t)); got != before {
			t.Errorf("%s: %d new rows, want none (not an evaluated call)", name, got-before)
		}
	}

	// Offset or limit set: the agent already reads a range.
	silent("offset set", readOutlineHintEnvelope(t, dir, bigPath, map[string]any{"offset": 10}))
	silent("limit set", readOutlineHintEnvelope(t, dir, bigPath, map[string]any{"limit": 40}))

	// 150 lines on disk: under the floor.
	small := readOutlineHintSource(8) + "\n\n\n\n" // 146 + 4
	writeRepoFile(t, dir, "small.go", small)
	silent("150-line file", readOutlineHintEnvelope(t, dir, filepath.Join(dir, "small.go"), nil))

	// A file this session edited.
	writeRepoFile(t, dir, "edited.go", big)
	markFileEdited("hint-session", "edited.go", time.Now())
	silent("edited this session", readOutlineHintEnvelope(t, dir, filepath.Join(dir, "edited.go"), nil))

	// Not a source file.
	writeRepoFile(t, dir, "notes.txt", big)
	silent("non-source file", readOutlineHintEnvelope(t, dir, filepath.Join(dir, "notes.txt"), nil))

	// Malformed stdin.
	silent("malformed stdin", "{not json")

	// A join past its deadline: silent, and the row says missed.
	origJoin, origDeadline := readOutlineHintJoin, readOutlineHintDeadline
	t.Cleanup(func() { readOutlineHintJoin, readOutlineHintDeadline = origJoin, origDeadline })
	readOutlineHintDeadline = 5 * time.Millisecond
	readOutlineHintJoin = func(ctx context.Context, path, text string) ([]string, bool) {
		<-ctx.Done()
		return readout.Outline(ctx, path, text)
	}
	before := len(readOutlineHintRows(t))
	code, out = runReadOutlineHint(t, readOutlineHintEnvelope(t, dir, bigPath, nil))
	if code != 0 || out != "" {
		t.Errorf("slow join: exit %d, stdout %q, want exit 0 and nothing", code, out)
	}
	rows = readOutlineHintRows(t)
	if len(rows) != before+1 || rows[len(rows)-1].Meta["missed"] != "true" || rows[len(rows)-1].Meta["emitted"] != "false" {
		t.Errorf("slow join: rows %d (want %d), last %v, want missed=true emitted=false", len(rows), before+1, rows[len(rows)-1:])
	}
	readOutlineHintJoin, readOutlineHintDeadline = origJoin, origDeadline

	// A join that ignores its context and blocks: the hint still returns at
	// the deadline, silent, missed, and the row's ms is the wait, not the join's.
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	readOutlineHintDeadline = 20 * time.Millisecond
	readOutlineHintJoin = func(ctx context.Context, path, text string) ([]string, bool) {
		// Ignores ctx. The 10 s fallback makes a hint that waits for the join
		// fail this test by name instead of hanging the package to its timeout.
		select {
		case <-release:
		case <-time.After(10 * time.Second):
		}
		return []string{"x"}, false
	}
	t0 := time.Now()
	code, out = runReadOutlineHint(t, readOutlineHintEnvelope(t, dir, bigPath, nil))
	if waited := time.Since(t0); waited > 5*time.Second {
		t.Errorf("blocked join held the hint %v, want return at the deadline", waited)
	}
	if code != 0 || out != "" {
		t.Errorf("blocked join: exit %d, stdout %q, want exit 0 and nothing", code, out)
	}
	rows = readOutlineHintRows(t)
	if last := rows[len(rows)-1]; last.Meta["missed"] != "true" || last.Meta["emitted"] != "false" {
		t.Errorf("blocked join: last row %v, want missed=true emitted=false", last.Meta)
	}
	readOutlineHintJoin, readOutlineHintDeadline = origJoin, origDeadline

	// The key off: silent, and init writes no entry for the hint.
	off := readOutlineHintEnv(t, false)
	writeRepoFile(t, off, "big.go", big)
	silent("key off", readOutlineHintEnvelope(t, off, filepath.Join(off, "big.go"), nil))
	targets, err := initTargetsFor(harness.ClaudeCode, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	hintEntries := func(root string) int {
		spec, _ := initWiringSpecFor(root, "svc", testBinPath, targets, hooksScopeAuto)
		n := 0
		for _, k := range spec.Keys {
			if len(k.Path) > 0 && k.Path[0] == "PreToolUse" && strings.Contains(k.Body, "readout-hook --event pre-read") {
				n++
			}
		}
		return n
	}
	if n := hintEntries(off); n != 0 {
		t.Errorf("init plans %d hint entries with read_outline_hint off, want none", n)
	}
	if n := hintEntries(dir); n != 1 {
		t.Errorf("init plans %d hint entries with read_outline_hint on, want one", n)
	}
}

// O1230: a Read of a file in another work tree than the session's (a
// subagent's scratch clone) reads that tree's [hooks] read_outline_hint,
// not the session's.
func TestReadOutlineHintReadsTheConfigOfTheFilesRepo(t *testing.T) {
	a := readOutlineHintEnv(t, false) // the session's cwd: the hint is off there
	b := t.TempDir()
	gitInitTempRepo(t, b)
	writeRepoFile(t, b, ".fathom/config.toml", "[hooks]\nread_outline_hint = true\n")
	writeRepoFile(t, b, "big.go", readOutlineHintSource(14))
	commitAll(t, b)
	if _, out := runReadOutlineHint(t, readOutlineHintEnvelope(t, a, filepath.Join(b, "big.go"), nil)); hintContext(t, out) == "" {
		t.Errorf("cwd A (hint off), a whole read of B's long file (hint on in B): no hint, want B's config to rule")
	}
	writeRepoFile(t, a, "big.go", readOutlineHintSource(14))
	if _, out := runReadOutlineHint(t, readOutlineHintEnvelope(t, a, filepath.Join(a, "big.go"), nil)); hintContext(t, out) != "" {
		t.Errorf("a read in A, where the hint is off: %q, want none", out)
	}
}
