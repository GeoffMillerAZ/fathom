package readout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// testRules are rule files the engine tests load as a user's own.
const testRules = `schema = 1

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

[[rule]]
id = "ls-head"
version = 1
about = "ls: the first two entries"
match.argv0 = ["ls"]
recovery = "repeat"

[[rule.stage]]
kind = "cap"
head = 2

[[rule.test]]
input = "a.go\nb.go\nerror.log\n"
expect = "a.go\nb.go\n[readout ls-head@1 observe: 3→2 lines; dropped 1 (error 1); raw: repeat the command]\n"

[[rule]]
id = "go-vet-records"
version = 1
about = "go vet: findings as records, repeats collapsed, grouped by file"
match.argv0 = ["go"]
match.sub = ["vet"]

[[rule.stage]]
kind = "extract"
pattern = '^(?P<file>[^:\s]+\.go):(?P<line>\d+)'
captures = ["file", "line"]
record = "vet_finding"

[[rule.stage]]
kind = "collapse"

[[rule.stage]]
kind = "group"
key = '^[^:\s]+\.go:'

[[rule.test]]
input = "# pkg\n# pkg\nx.go:3:1: bad\n"
expect = "# pkg ×2\nx.go:3:1: bad\n[readout go-vet-records@1 observe: 2→2 lines; nothing dropped]\n"
`

// writeFiles writes files (relative path to content) under dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// loadTestRules loads testRules as a user's readouts.toml and fails on any
// refusal.
func loadTestRules(t *testing.T) *RuleSet {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{UserFile: testRules})
	rs, err := LoadRules(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Refused) > 0 {
		t.Fatalf("refused: %+v", rs.Refused)
	}
	return rs
}

func lexed(s string) *shellcmd.Command {
	c := shellcmd.ParseCommand(s)
	return &c
}

func lines(prefix string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString(prefix)
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteString("\n")
	}
	return b.String()
}
