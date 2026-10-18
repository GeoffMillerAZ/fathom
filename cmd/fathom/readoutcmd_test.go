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

// readoutTestRules is a user rule file the readout verb tests load with
// --rules: a diagnostic rule (make after quiet prefixes) and a content one
// (grep matches in test files dropped).
const readoutTestRules = `schema = 1

[[rule]]
id = "make-quiet"
version = 3
about = "make: drop compile lines"
match.argv0 = ["make"]
match.compound = "after_quiet"
recovery = "repeat"

[[rule.stage]]
kind = "drop"
pattern = '^(cc|---) '

[[rule.test]]
input = "cc -c a.c\nok\n"
expect = "ok\n[readout make-quiet@3 observe: 2→1 lines; dropped 1 (plain 1); raw: repeat the command]\n"

[[rule.test]]
input = "--- FAIL: TestA\n"
trip = true

[[rule]]
id = "grep-skip-tests"
version = 1
about = "grep: drop matches in test files"
match.argv0 = ["grep", "rg"]
match.compound = "display_tail"
recovery = "repeat"

[[rule.stage]]
kind = "drop"
pattern = '^\S+_test\.go:'

[[rule.test]]
input = "a.go:12:\treturn err\nb_test.go:3:\tt.Fatal(errors.New(\"x\"))\n"
expect = "a.go:12:\treturn err\n[readout grep-skip-tests@1 observe: 2→1 lines; dropped 1 (error 1); raw: repeat the command]\n"
stream = "content"
`

func writeReadoutRules(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "rules.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// replayFixture writes Claude Code transcript lines for the replay test.
type replayFixture struct {
	lines   []string
	session string
}

func (f *replayFixture) use(t *testing.T, ts, id, tool string, input map[string]any) {
	t.Helper()
	line := map[string]any{
		"type": "assistant", "sessionId": f.session, "timestamp": ts,
		"message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": id, "name": tool, "input": input},
		}},
	}
	f.add(t, line)
}

func (f *replayFixture) bash(t *testing.T, ts, id, command, stdout string, extra map[string]any) {
	t.Helper()
	f.use(t, ts, id, "Bash", map[string]any{"command": command, "description": "a step"})
	result := map[string]any{"stdout": stdout, "stderr": "", "interrupted": false, "isImage": false}
	for k, v := range extra {
		result[k] = v
	}
	f.add(t, map[string]any{
		"type": "user", "sessionId": f.session, "timestamp": ts,
		"message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": id, "content": stdout, "is_error": false},
		}},
		"toolUseResult": result,
	})
}

func (f *replayFixture) add(t *testing.T, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	f.lines = append(f.lines, string(b))
}

func (f *replayFixture) write(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(f.lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func useReadoutReplayEnv(t *testing.T, root string, now time.Time) {
	t.Helper()
	oldRoot, oldNow := readoutProjectsDir, readoutNow
	readoutProjectsDir = func() string { return root }
	readoutNow = func() time.Time { return now }
	t.Cleanup(func() { readoutProjectsDir, readoutNow = oldRoot, oldNow })
}

func TestCmdReadoutReplayPrintsAggregatesOnly(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "projects")
	rules := writeReadoutRules(t, dir, readoutTestRules)
	useReadoutReplayEnv(t, root, time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))

	// The grep: ten matches in a test file the rule drops, two kept.
	var grepOut strings.Builder
	grepOut.WriteString("pkg/alpha.go:12:\treturn needleValue\npkg/beta.go:40:\tneedleValue++\n")
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&grepOut, "pkg/secret_test.go:%d:\tif needleHelperZeta(needleValue) != %d {\n", 10+i, i)
	}
	// make's compile chatter, inline and persisted.
	var makeOut strings.Builder
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&makeOut, "cc -c confidential_%02d.c -o confidential_%02d.o\n", i, i)
	}
	makeOut.WriteString("ok build done\n")
	persisted := filepath.Join(dir, "tool-results", "persisted-secret.txt")
	if err := os.MkdirAll(filepath.Dir(persisted), 0o755); err != nil {
		t.Fatal(err)
	}
	var persistedOut strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&persistedOut, "cc -c hidden_unit_%02d.c\n", i)
	}
	persistedOut.WriteString("ok check done\n")
	if err := os.WriteFile(persisted, []byte(persistedOut.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	// A 250-line source file read whole, then a span of it.
	var bigOut strings.Builder
	for i := 0; i < 250; i++ {
		fmt.Fprintf(&bigOut, "\tsecretBodyLine%d()\n", i)
	}

	a := &replayFixture{session: "sess-secret-a"}
	a.bash(t, "2026-08-01T10:00:00.000Z", "toolu_old", "ls /very/old/path", "ancient.txt\n", nil)
	a.bash(t, "2026-10-10T10:00:00.000Z", "toolu_grep", "grep -rn needleValue pkg", grepOut.String(), nil)
	a.bash(t, "2026-10-10T10:00:05.000Z", "toolu_span", "sed -n 1,20p pkg/secret_test.go", "\tif needleHelperZeta(needleValue) {\n", nil)
	a.bash(t, "2026-10-10T10:01:00.000Z", "toolu_make", "cd /work/classified && make all", makeOut.String(), nil)
	a.bash(t, "2026-10-10T10:02:00.000Z", "toolu_cat", "cat internal/bigsecret.go", bigOut.String(), nil)
	a.use(t, "2026-10-10T10:02:30.000Z", "toolu_read", "Read", map[string]any{"file_path": "/work/classified/README.md"})
	a.bash(t, "2026-10-10T10:03:00.000Z", "toolu_reread", "sed -n 40,60p internal/bigsecret.go", "\tsecretBodyLine40()\n", nil)
	a.bash(t, "2026-10-10T10:04:00.000Z", "toolu_tcc", "tccutil reset SystemPolicyRemovableVolumes com.example.term && ls", "Successfully reset\nREADME.md\n", nil)
	a.bash(t, "2026-10-10T10:05:00.000Z", "toolu_persist", "make check", "<persisted-output>preview</persisted-output>\n",
		map[string]any{"persistedOutputPath": persisted, "persistedOutputSize": persistedOut.Len()})
	a.write(t, filepath.Join(root, "-work-proj-alpha-secret", "sess-secret-a.jsonl"))

	// A resumed session copies the grep's history, then adds one call.
	b := &replayFixture{session: "sess-secret-b"}
	b.bash(t, "2026-10-10T10:00:00.000Z", "toolu_grep", "grep -rn needleValue pkg", grepOut.String(), nil)
	b.bash(t, "2026-10-11T09:00:00.000Z", "toolu_test", "go test ./pkg/...", "ok  \texample/pkg\t0.1s\n", nil)
	b.write(t, filepath.Join(root, "-work-proj-alpha-secret", "sess-secret-b.jsonl"))
	// A project the --projects glob leaves out.
	c := &replayFixture{session: "sess-secret-c"}
	c.bash(t, "2026-10-12T09:00:00.000Z", "toolu_other", "cat other.go", "package other\n", nil)
	c.write(t, filepath.Join(root, "-work-proj-beta-hidden", "sess-secret-c.jsonl"))

	var stdout, stderr bytes.Buffer
	code := cmdReadout([]string{"replay", "--json", "--days", "30", "--projects", "*alpha*", "--rules", rules}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	out := stdout.String()
	for _, leak := range []string{"needle", "secret", "Secret", "classified", "confidential", "hidden", "SystemPolicy", "com.example", "persisted-secret", dir, "sess-", "proj-alpha", "proj-beta", "/work", "toolu_", "ancient", "README", "preview", "Successfully"} {
		if strings.Contains(out, leak) {
			t.Errorf("replay output carries input text %q:\n%s", leak, out)
		}
	}
	var rep replayReport
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if rep.Window.From != "2026-10-10T10:00:00.000Z" || rep.Window.To != "2026-10-11T09:00:00.000Z" || rep.Window.Days != 30 {
		t.Errorf("window = %+v", rep.Window)
	}
	if rep.Corpus.Projects != 1 || rep.Corpus.Sessions != 2 || rep.Corpus.Files != 2 {
		t.Errorf("corpus = %+v, want 1 project, 2 sessions, 2 files", rep.Corpus)
	}
	if rep.Bash.Calls != 8 || rep.Bash.Duplicates != 1 || rep.Bash.Persisted != 1 || rep.Bash.PersistedRead != 1 {
		t.Errorf("bash = %+v, want 8 calls, 1 duplicate, 1 persisted read", rep.Bash)
	}
	if rep.WholeFileReads.Calls != 1 || rep.WholeFileReads.Bytes != int64(bigOut.Len()) || rep.WholeFileReads.Share <= 0 {
		t.Errorf("whole-file reads = %+v, want the one 250-line cat of %d bytes", rep.WholeFileReads, bigOut.Len())
	}
	if rep.Reread.WholeReads != 1 || rep.Reread.FollowedBySed != 1 || rep.Reread.LongWholeReads != 1 || rep.Reread.LongFollowedBySed != 1 {
		t.Errorf("reread = %+v, want the cat followed by its span", rep.Reread)
	}
	if rep.Tccutil.Calls != 1 || rep.Tccutil.Compound != 1 {
		t.Errorf("tccutil = %+v", rep.Tccutil)
	}
	byRule := map[string]replayRule{}
	for _, r := range rep.Rules {
		byRule[r.Rule] = r
	}
	g := byRule["grep-skip-tests@1"]
	if g.Hits != 1 || g.Served != 1 || g.WouldRemove <= 0 || g.Reference.Of != 1 || g.Reference.Referenced != 1 || g.UnscopedTrips != 1 || g.Trips.Content != 0 {
		t.Errorf("grep rule = %+v, want 1 served hit referenced by the next call and 1 unscoped trip", g)
	}
	m := byRule["make-quiet@3"]
	if m.Hits != 2 || m.Served != 2 || m.Reference.Of != 2 || m.Reference.Referenced != 0 || m.Eligible {
		t.Errorf("make rule = %+v, want 2 clean served hits, not eligible under 200", m)
	}
	if m.BytesIn != int64(makeOut.Len()+persistedOut.Len()) {
		t.Errorf("make bytes in = %d, want the inline and the persisted file's %d", m.BytesIn, makeOut.Len()+persistedOut.Len())
	}
	if m.Reference.CleanWilsonLower <= 0 || m.Reference.CleanWilsonLower >= 1 {
		t.Errorf("make wilson lower = %v", m.Reference.CleanWilsonLower)
	}
	if rep.Eligible.WouldRemove != 0 || len(rep.Eligible.Rules) != 0 {
		t.Errorf("eligible = %+v, want none under 200 hits", rep.Eligible)
	}
	shapes := map[string]replayShapeRow{}
	for _, s := range rep.Shapes {
		shapes[s.Shape] = s
	}
	if s := shapes["sed -n"]; s.Calls != 2 || s.Simple != 2 {
		t.Errorf("sed -n shape = %+v", s)
	}
	if s := shapes["cd"]; s.Calls != 1 || s.List != 1 || s.CompoundShare != 1 {
		t.Errorf("cd shape = %+v", s)
	}
	if s := shapes["go test"]; s.Calls != 1 {
		t.Errorf("go test shape = %+v", s)
	}

	// Deterministic: the same corpus gives the same bytes.
	var again bytes.Buffer
	if code := cmdReadout([]string{"replay", "--json", "--days", "30", "--projects", "*alpha*", "--rules", rules}, strings.NewReader(""), &again, &stderr); code != 0 || again.String() != out {
		t.Errorf("second run differs (exit %d)", code)
	}
	// The text form carries no input text either.
	var text bytes.Buffer
	if code := cmdReadout([]string{"replay", "--projects", "*alpha*", "--rules", rules}, strings.NewReader(""), &text, &stderr); code != 0 {
		t.Fatalf("text exit %d: %s", code, stderr.String())
	}
	for _, leak := range []string{"needle", "secret", "classified", dir} {
		if strings.Contains(text.String(), leak) {
			t.Errorf("text output carries %q", leak)
		}
	}
	if !strings.Contains(text.String(), "grep-skip-tests@1") {
		t.Errorf("text output names no rule:\n%s", text.String())
	}
}

func TestCmdReadoutExplainExitCodes(t *testing.T) {
	dir := t.TempDir()
	rules := writeReadoutRules(t, dir, readoutTestRules)
	var in strings.Builder
	in.WriteString("pkg/a.go:1:\tkeep me\n")
	for i := 0; i < 6; i++ {
		fmt.Fprintf(&in, "pkg/a_test.go:%d:\tdrop me %d\n", i+1, i)
	}
	var stdout, stderr bytes.Buffer
	if code := cmdReadout([]string{"explain", "--rules", rules, "grep -rn me pkg"}, strings.NewReader(in.String()), &stdout, &stderr); code != 0 {
		t.Fatalf("explain exit %d: %s", code, stderr.String())
	}
	for _, want := range []string{"grep-skip-tests@1", "stream: content", "drop", "pkg/a_test.go:3:", "[readout grep-skip-tests@1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("explain output lacks %q:\n%s", want, stdout.String())
		}
	}
	stdout.Reset()
	if code := cmdReadout([]string{"explain", "--rules", rules, "echo 'unterminated"}, strings.NewReader("x\n"), &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "unreadable") {
		t.Errorf("unreadable command: exit %d, %s", code, stdout.String())
	}
	if code := cmdReadout([]string{"explain"}, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Errorf("no command: exit %d, want 1", code)
	}
	if code := cmdReadout([]string{"explain", "--rules", filepath.Join(dir, "missing.toml"), "ls"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Errorf("missing rules file: exit %d, want 2", code)
	}
}

func TestCmdReadoutTestExitCodes(t *testing.T) {
	dir := t.TempDir()
	good := writeReadoutRules(t, dir, readoutTestRules)
	bad := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(bad, []byte(strings.Replace(readoutTestRules, `expect = "ok\n`, `expect = "OK\n`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "broken.toml")
	if err := os.WriteFile(broken, []byte("schema = 1\n[[rule]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		args  []string
		code  int
		wants []string
	}{
		{"pass", []string{good}, 0, []string{"ok", "make-quiet@3", "lossiness lines", "recovery repeat"}},
		{"a fixture fails", []string{good, bad}, 1, []string{"FAIL", "make-quiet"}},
		{"a file does not parse", []string{good, broken}, 2, []string{"cannot load"}},
		{"a file is missing", []string{filepath.Join(dir, "missing.toml")}, 2, []string{"cannot load"}},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		code := cmdReadout(append([]string{"test"}, tc.args...), strings.NewReader(""), &stdout, &stderr)
		if code != tc.code {
			t.Errorf("%s: exit %d, want %d\n%s%s", tc.name, code, tc.code, stdout.String(), stderr.String())
		}
		for _, w := range tc.wants {
			if !strings.Contains(stdout.String()+stderr.String(), w) {
				t.Errorf("%s: output lacks %q:\n%s%s", tc.name, w, stdout.String(), stderr.String())
			}
		}
	}
	var stdout, stderr bytes.Buffer
	if code := cmdReadout([]string{"nope"}, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Errorf("unknown subverb: exit %d, want 1", code)
	}
}
