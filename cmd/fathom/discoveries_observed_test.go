package main

// discoveries_observed_test.go — RC-56: extraction (observedDiscoveryPointers,
// bashObservedReadPath, discoveryToolLabel), the write/read round trip, and
// rotation.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// pinNoEnclosingRepo mirrors edithook_precision_test.go's own guard: it
// pins git's upward discovery at the temp root so a test's project dir
// never accidentally resolves an ENCLOSING repo (this worktree, or
// whatever holds it) as its git toplevel.
func pinNoEnclosingRepo(t *testing.T) {
	t.Helper()
	ceil := os.TempDir()
	if resolved, err := filepath.EvalSymlinks(ceil); err == nil {
		ceil = resolved
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", ceil)
}

// ─── observedDiscoveryPointers: one case per tool shape ──────────────────

func TestObservedDiscoveryPointers_Read(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	in := harness.HookInput{
		Tool:     harness.ToolDiscoveryRead,
		FilePath: filepath.Join(dir, "internal", "core", "model.go"),
		Cwd:      dir,
	}
	got := observedDiscoveryPointers(in)
	if len(got) != 1 || got[0] != filepath.Join("internal", "core", "model.go") {
		t.Fatalf("observedDiscoveryPointers(Read) = %v, want [internal/core/model.go]", got)
	}
}

func TestObservedDiscoveryPointers_GrepWithPath(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	in := harness.HookInput{
		Tool: harness.ToolDiscoveryGrep,
		Path: filepath.Join(dir, "internal", "mcp"),
		Cwd:  dir,
	}
	got := observedDiscoveryPointers(in)
	if len(got) != 1 || got[0] != filepath.Join("internal", "mcp") {
		t.Fatalf("observedDiscoveryPointers(Grep, path) = %v, want [internal/mcp]", got)
	}
}

// TestObservedDiscoveryPointers_GrepPatternNeverRecorded proves the search
// PATTERN never becomes a pointer even when path is absent — "a pattern is
// not a location" (RC-56).
func TestObservedDiscoveryPointers_GrepPatternNeverRecorded(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	in := harness.HookInput{Tool: harness.ToolDiscoveryGrep, Path: "", Cwd: dir}
	if got := observedDiscoveryPointers(in); got != nil {
		t.Fatalf("observedDiscoveryPointers(Grep, no path) = %v, want nil", got)
	}
}

func TestObservedDiscoveryPointers_Glob(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	in := harness.HookInput{
		Tool: harness.ToolDiscoveryGlob,
		Path: filepath.Join(dir, "cmd", "fathom"),
		Cwd:  dir,
	}
	got := observedDiscoveryPointers(in)
	if len(got) != 1 || got[0] != filepath.Join("cmd", "fathom") {
		t.Fatalf("observedDiscoveryPointers(Glob) = %v, want [cmd/fathom]", got)
	}
}

func TestObservedDiscoveryPointers_BashRecognizedRead(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "internal", "core", "model.go")
	in := harness.HookInput{Tool: harness.ToolShell, Command: "cat " + target, Cwd: dir}
	got := observedDiscoveryPointers(in)
	if len(got) != 1 || got[0] != filepath.Join("internal", "core", "model.go") {
		t.Fatalf("observedDiscoveryPointers(Bash cat) = %v, want [internal/core/model.go]", got)
	}
}

func TestObservedDiscoveryPointers_BashRecognizedGrep(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "internal", "mcp")
	in := harness.HookInput{Tool: harness.ToolShell, Command: "grep -rn foo " + target, Cwd: dir}
	got := observedDiscoveryPointers(in)
	if len(got) != 1 || got[0] != filepath.Join("internal", "mcp") {
		t.Fatalf("observedDiscoveryPointers(Bash grep) = %v, want [internal/mcp]", got)
	}
}

func TestObservedDiscoveryPointers_BashPipelineNotRecorded(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "internal", "core", "model.go")
	in := harness.HookInput{Tool: harness.ToolShell, Command: "cat " + target + " | head -5", Cwd: dir}
	if got := observedDiscoveryPointers(in); got != nil {
		t.Fatalf("observedDiscoveryPointers(Bash pipeline) = %v, want nil", got)
	}
}

func TestObservedDiscoveryPointers_BashUnrecognizedBinaryNotRecorded(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	in := harness.HookInput{Tool: harness.ToolShell, Command: "go test ./...", Cwd: dir}
	if got := observedDiscoveryPointers(in); got != nil {
		t.Fatalf("observedDiscoveryPointers(Bash go test) = %v, want nil", got)
	}
}

func TestObservedDiscoveryPointers_PathOutsideAnyRootYieldsNil(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	other := t.TempDir()
	in := harness.HookInput{
		Tool:     harness.ToolDiscoveryRead,
		FilePath: filepath.Join(other, "unrelated.go"),
		Cwd:      dir,
	}
	if got := observedDiscoveryPointers(in); got != nil {
		t.Fatalf("observedDiscoveryPointers(outside root) = %v, want nil", got)
	}
}

// ─── bashObservedReadPath ──────────────────────────────────────────────────

func TestBashObservedReadPath(t *testing.T) {
	cases := []struct {
		command    string
		wantTarget string
		wantTool   string
	}{
		{"cat a/b.go", "a/b.go", "read"},
		{"head -20 a/b.go", "a/b.go", "read"},
		{"tail -f a/b.go", "a/b.go", "read"},
		{"sed -n '1,5p' a/b.go", "a/b.go", "read"},
		{"sed a/b.go", "", ""}, // no -n script: RC-11's 4-field floor
		{"rg --hidden foo a/b", "a/b", "grep"},
		{"grep -rn foo a/b", "a/b", "grep"},
		{"cat a.go | head -5", "", ""},   // pipe disqualifies
		{"cat $FILE", "", ""},            // variable disqualifies
		{"cat *.go", "", ""},             // glob disqualifies
		{"go test ./...", "", ""},        // unrecognized binary
		{"less a/b.go", "", ""},          // not in the RC-56 list
		{"awk '{print}' a/b.go", "", ""}, // not in the RC-56 list
		{"", "", ""},
		{"cat", "", ""}, // no path at all
		{"cat -n", "", ""},
	}
	for _, c := range cases {
		target, tool := bashObservedReadPath(c.command)
		if target != c.wantTarget || tool != c.wantTool {
			t.Errorf("bashObservedReadPath(%q) = (%q, %q), want (%q, %q)", c.command, target, tool, c.wantTarget, c.wantTool)
		}
	}
}

// ─── discoveryToolLabel ─────────────────────────────────────────────────────

func TestDiscoveryToolLabel(t *testing.T) {
	cases := []struct {
		in   harness.HookInput
		want string
	}{
		{harness.HookInput{Tool: harness.ToolDiscoveryRead}, "read"},
		{harness.HookInput{Tool: harness.ToolDiscoveryGrep}, "grep"},
		{harness.HookInput{Tool: harness.ToolDiscoveryGlob}, "glob"},
		{harness.HookInput{Tool: harness.ToolShell, Command: "cat a.go"}, "read"},
		{harness.HookInput{Tool: harness.ToolShell, Command: "rg foo a"}, "grep"},
		{harness.HookInput{Tool: harness.ToolShell, Command: "go test ./..."}, ""},
	}
	for _, c := range cases {
		if got := discoveryToolLabel(c.in); got != c.want {
			t.Errorf("discoveryToolLabel(%+v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ─── write/read round trip ──────────────────────────────────────────────────

func TestRecordObservedDiscoveryIfAny_WriteReadRoundTrip(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	in := harness.HookInput{
		Tool:      harness.ToolDiscoveryRead,
		FilePath:  filepath.Join(dir, "internal", "core", "model.go"),
		Cwd:       dir,
		SessionID: "s1",
	}
	recordObservedDiscoveryIfAny(in)

	rows := readObservedDiscoveryRows(dir, time.Time{})
	if len(rows) != 1 {
		t.Fatalf("readObservedDiscoveryRows = %d rows, want 1: %+v", len(rows), rows)
	}
	r := rows[0]
	if r.SessionID != "s1" || r.Tool != "read" || len(r.Pointers) != 1 || r.Pointers[0] != filepath.Join("internal", "core", "model.go") {
		t.Fatalf("row = %+v, want session s1, tool read, pointer internal/core/model.go", r)
	}
}

// TestRecordObservedDiscoveryIfAny_NoPointerWritesNothing proves a call
// that names no pointer (a Grep with no path, an unrecognized Bash
// command) never touches the log at all.
func TestRecordObservedDiscoveryIfAny_NoPointerWritesNothing(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	recordObservedDiscoveryIfAny(harness.HookInput{Tool: harness.ToolDiscoveryGrep, Cwd: dir})
	if _, err := os.Stat(observedDiscoveriesLogPathFor(dir)); err == nil {
		t.Fatalf("expected no observed-discoveries log to be created")
	}
}

func TestReadObservedDiscoveryRows_HonoursSince(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	now := time.Now().UTC()
	recordObservedDiscovery(dir, observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion, TS: now.Add(-time.Hour).Format(time.RFC3339Nano),
		Tool: "read", Pointers: []string{"too/old.go"},
	})
	recordObservedDiscovery(dir, observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion, TS: now.Format(time.RFC3339Nano),
		Tool: "read", Pointers: []string{"recent.go"},
	})
	rows := readObservedDiscoveryRows(dir, now.Add(-time.Minute))
	if len(rows) != 1 || rows[0].Pointers[0] != "recent.go" {
		t.Fatalf("readObservedDiscoveryRows with since = %+v, want just recent.go", rows)
	}
}

func TestReadObservedDiscoveryRows_MissingLogIsEmptyNotError(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	if rows := readObservedDiscoveryRows(dir, time.Time{}); rows != nil {
		t.Fatalf("readObservedDiscoveryRows on missing log = %v, want nil", rows)
	}
}

func TestReadObservedDiscoveryRows_ToleratesMalformedLines(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	logPath := observedDiscoveriesLogPathFor(dir)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	content := strings.Join([]string{
		`not json at all`,
		`{"schema_version":1,"ts":"` + now.Format(time.RFC3339Nano) + `","tool":"read","pointers":[]}`, // no pointers
		`{"schema_version":1,"ts":"` + now.Format(time.RFC3339Nano) + `","tool":"read","pointers":["ok.go"]}`,
		``,
	}, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := readObservedDiscoveryRows(dir, time.Time{})
	if len(rows) != 1 || rows[0].Pointers[0] != "ok.go" {
		t.Fatalf("readObservedDiscoveryRows tolerant read = %+v, want just [ok.go]", rows)
	}
}

// ─── rotation ───────────────────────────────────────────────────────────────

func TestRotateObservedDiscoveriesIfNeeded(t *testing.T) {
	pinNoEnclosingRepo(t)
	dir := t.TempDir()
	logPath := observedDiscoveriesLogPathFor(dir)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}

	orig := observedDiscoveryRotateBytes
	observedDiscoveryRotateBytes = 32 // tiny, so one row already exceeds it
	t.Cleanup(func() { observedDiscoveryRotateBytes = orig })

	if err := os.WriteFile(logPath, []byte(strings.Repeat("x", 64)), 0o644); err != nil {
		t.Fatal(err)
	}
	recordObservedDiscovery(dir, observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion, TS: time.Now().UTC().Format(time.RFC3339Nano),
		Tool: "read", Pointers: []string{"fresh.go"},
	})

	backup, err := os.ReadFile(logPath + ".1")
	if err != nil || len(backup) != 64 {
		t.Fatalf("expected the old 64-byte generation rotated to .1, got err=%v len=%d", err, len(backup))
	}
	rows := readObservedDiscoveryRows(dir, time.Time{})
	if len(rows) != 1 || rows[0].Pointers[0] != "fresh.go" {
		t.Fatalf("post-rotation rows = %+v, want just the fresh row (the .1 backup is not read)", rows)
	}
}
