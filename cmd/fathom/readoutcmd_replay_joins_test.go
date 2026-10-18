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
)

// TestCmdReadoutReplayFillsDeclaredTestAndTheOutlineJoin: replay fills what
// the built-ins read: TR-1's declared-test answer from the call's cwd when it
// exists (else the built-in runner list alone), so `go test ./...` hits
// go-test as matched and, after `cd <dir> &&`, as projected, and a declared
// `task test` hits it as matched; the persisted text (a miss counted); HEAD's
// stamp, always missed for a past call; and the outline, computed for a
// whole-file read and left raw for a file the session edited earlier.
func TestCmdReadoutReplayFillsDeclaredTestAndTheOutlineJoin(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "projects")
	t.Setenv("FATHOM_HOME", filepath.Join(dir, "fathom-home"))
	useReadoutReplayEnv(t, root, time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(filepath.Join(work, "shelf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "Taskfile.yml"), []byte("version: '3'\ntasks:\n  test:\n    cmds:\n      - go test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := readOutlineSource()
	logOut := "commit 3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a\nAuthor: Dev <dev@example.com>\nDate:   Mon Oct 12 10:00:00 2026 -0700\n\n    init\n\n    Signed-off-by: Dev <dev@example.com>\n"
	var grepOut strings.Builder
	for i := range 12 {
		fmt.Fprintf(&grepOut, "pkg/file_%02d.go:%d:\treturn shelfCountOmega\n", i%3, i+1)
	}

	p := &replayFixture{session: "sess-j", cwd: work}
	calls := []struct {
		cwd, command, stdout string
		extra                map[string]any
	}{
		{work, "go test ./...", goTestPassing(), nil},
		{work, "cd " + work + " && go test ./...", goTestPassing(), nil},
		{work, "task test", goTestPassing(), nil},
		{filepath.Join(dir, "gone"), "gotestsum", goTestPassing(), nil},
		{work, "git log", logOut, nil},
		{work, "cat shelf/count.go", src, nil},
		{work, "grep -rn shelfCountOmega pkg", grepOut.String(), map[string]any{"persistedOutputPath": filepath.Join(dir, "gone.txt")}},
	}
	for i, c := range calls {
		p.cwd = c.cwd
		p.bash(t, fmt.Sprintf("2026-10-10T10:%02d:00.000Z", i), fmt.Sprintf("toolu_j%02d", i), c.command, c.stdout, c.extra)
	}
	p.cwd = work
	p.use(t, "2026-10-10T10:20:00.000Z", "toolu_edit", "Edit", map[string]any{"file_path": filepath.Join(work, "shelf", "edited.go"), "old_string": "a", "new_string": "b"})
	p.bash(t, "2026-10-10T10:21:00.000Z", "toolu_j20", "cat shelf/edited.go", src, nil)
	p.write(t, filepath.Join(root, "-work-proj-j", "sess-j.jsonl"))

	var stdout, stderr bytes.Buffer
	if code := cmdReadout([]string{"replay", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	out := stdout.String()
	for _, leak := range []string{"Count00", "shelf/", "shelfCountOmega", work} {
		if strings.Contains(out, leak) {
			t.Errorf("replay output carries input text %q", leak)
		}
	}
	var rep replayReport
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	byRule := map[string]replayRule{}
	for _, r := range rep.Rules {
		byRule[r.Rule] = r
	}
	if g := byRule["go-test@1"]; g.AsMatched.Hits != 3 || g.Projected.Hits != 4 {
		t.Errorf("go-test hits as matched %d, projected %d; want 3 (plain, declared, built-in runner) and 4 (after cd too)", g.AsMatched.Hits, g.Projected.Hits)
	}
	ro := byRule["read-outline@1"].AsMatched
	if ro.Hits != 2 || ro.Served != 1 || ro.Raw["edited"] != 1 || ro.WouldRemove <= 0 {
		t.Errorf("read-outline as matched = %+v, want 2 hits, the committed read served and the edited one raw", ro)
	}
	want := replayJoins{DeclaredTestByRoot: 3, DeclaredTestByBuiltin: 1, PersistedMissed: 1, HeadStampMissed: 1,
		OutlineComputed: 1, EditedRaw: 1, Note: replayJoinsNote}
	if rep.Joins != want {
		t.Errorf("joins = %+v, want %+v", rep.Joins, want)
	}
	var text bytes.Buffer
	if code := cmdReadout([]string{"replay"}, strings.NewReader(""), &text, &stderr); code != 0 || !strings.Contains(text.String(), "joins: declared test commands matched 3 by cwd, 1 by the built-in runners") {
		t.Errorf("text view (exit %d) lacks the joins line:\n%s", code, text.String())
	}
}
