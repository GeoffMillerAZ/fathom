package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// TestCmdLangOracleFilesWithError (LB-F59, O203): each subject counts the
// measured files whose tree on the dialect's own pinned grammar holds an
// ERROR or MISSING node, and the reading sums it next to faults, so §14.2's
// parse-error refusal line is read off the reading.
func TestCmdLangOracleFilesWithError(t *testing.T) {
	subject := t.TempDir()
	writeTree(t, subject, protoFixture)
	broken := map[string]string{
		// a field without its `;`: the parser inserts a MISSING node, and no ERROR
		"broken/missing.proto": "syntax = \"proto3\";\nmessage C {\n  string c = 1\n}\n",
		// a stray token: an ERROR node
		"broken/error.proto": "syntax = \"proto3\";\nmessage D {\n  string d = 1;\n  )\n}\n",
	}
	writeTree(t, subject, broken)
	out := filepath.Join(t.TempDir(), "protobuf.json")
	args := []string{"-dialect", "protobuf", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "protobuf", "dialect.json"),
		"-subject", subject, "-subject-name", "fixture/proto", "-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	if n := r.Tenancy.FilesWithError; n == nil || *n != 2 {
		t.Errorf("reading files_with_error = %v, want 2 of 4\n%s", n, raw)
	}
	if len(r.Subjects) != 1 || r.Subjects[0].Files != 4 || r.Subjects[0].Tenancy == nil || r.Subjects[0].Tenancy.FilesWithError == nil || *r.Subjects[0].Tenancy.FilesWithError != 2 {
		t.Errorf("subject %+v, want 2 of 4 files with an error", r.Subjects)
	}
	// Why the count is a query and not lang-probe's S-expression: the
	// S-expression renders named nodes only, so a MISSING `;` never shows.
	target, err := extractor.ProbeTargetFor("protobuf", "broken/missing.proto")
	if err != nil {
		t.Fatal(err)
	}
	lines, err := probeLines(target, []byte(broken["broken/missing.proto"]))
	if err != nil {
		t.Fatal(err)
	}
	if s := strings.Join(lines, "\n"); strings.Contains(s, "ERROR") || strings.Contains(s, "MISSING") {
		t.Errorf("lang-probe now shows the missing node; the S-expression could count it:\n%s", s)
	}
	_ = os.Remove(out)
}
