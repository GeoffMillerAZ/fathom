package fusion

import (
	"strings"
	"testing"
)

// rec builds one raw git-log record in parseGitLog's STX/ETX-delimited
// format: \x02<hash>\x1f<author>\x1f<date>\x1f<body>\x03\n<files...>\n\n
func rec(hash, author, date, body string, files ...string) string {
	var b strings.Builder
	b.WriteByte('\x02')
	b.WriteString(hash)
	b.WriteByte('\x1f')
	b.WriteString(author)
	b.WriteByte('\x1f')
	b.WriteString(date)
	b.WriteByte('\x1f')
	b.WriteString(body)
	b.WriteByte('\x03')
	b.WriteByte('\n')
	for _, f := range files {
		b.WriteString(f)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

func TestParseGitLog_MultiCommit(t *testing.T) {
	raw := rec("abc1234f", "Alice", "2024-01-15T10:00:00Z", "Fix auth M18.2", "pkg/auth/auth.go", "pkg/auth/token.go") +
		rec("def5678a", "Bob", "2024-01-14T09:00:00Z", "Refactor", "pkg/util/util.go")
	commits := parseGitLog("svc", raw)
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	c0 := commits[0]
	if c0.Hash != "abc1234f" {
		t.Errorf("hash = %q, want abc1234f", c0.Hash)
	}
	if c0.Author != "Alice" {
		t.Errorf("author = %q, want Alice", c0.Author)
	}
	if c0.Message != "Fix auth M18.2" {
		t.Errorf("message = %q, want 'Fix auth M18.2'", c0.Message)
	}
	if len(c0.Files) != 2 {
		t.Errorf("files = %v, want 2", c0.Files)
	}
	if len(c0.SpecIDs) != 1 || c0.SpecIDs[0] != "M18.2" {
		t.Errorf("spec_ids = %v, want [M18.2]", c0.SpecIDs)
	}
	if c0.Date == 0 {
		t.Error("date should be non-zero for valid ISO8601 input")
	}
	c1 := commits[1]
	if c1.Hash != "def5678a" {
		t.Errorf("hash[1] = %q, want def5678a", c1.Hash)
	}
	if len(c1.Files) != 1 {
		t.Errorf("files[1] = %v, want 1", c1.Files)
	}
	if len(c1.SpecIDs) != 0 {
		t.Errorf("spec_ids[1] should be empty, got %v", c1.SpecIDs)
	}
}

func TestParseGitLog_Empty(t *testing.T) {
	commits := parseGitLog("svc", "")
	if len(commits) != 0 {
		t.Errorf("expected 0 commits for empty input, got %d", len(commits))
	}
}

func TestParseGitLog_MalformedLine(t *testing.T) {
	// A record with no \x1f separators in its header — should skip
	// gracefully — followed by one well-formed record.
	raw := "\x02notavalidheader\x03\n\n" + rec("abc1234f", "Alice", "2024-01-15T10:00:00Z", "Good commit")
	commits := parseGitLog("svc", raw)
	// The valid record should parse; the malformed one should be skipped.
	if len(commits) != 1 {
		t.Errorf("expected 1 valid commit, got %d", len(commits))
	}
}

func TestParseGitLog_TruncatedRecord(t *testing.T) {
	// A record missing its \x03 terminator entirely (truncated git output)
	// must be skipped, not mis-parsed as a bogus file list.
	raw := "\x02abc1234f\x1fAlice\x1f2024-01-15T10:00:00Z\x1fno terminator" +
		rec("def5678a", "Bob", "2024-01-14T09:00:00Z", "Good commit")
	commits := parseGitLog("svc", raw)
	if len(commits) != 1 {
		t.Fatalf("expected 1 valid commit, got %d", len(commits))
	}
	if commits[0].Hash != "def5678a" {
		t.Errorf("hash = %q, want def5678a", commits[0].Hash)
	}
}

func TestParseGitLog_ServiceID(t *testing.T) {
	raw := rec("abc1234f", "Alice", "2024-01-15T10:00:00Z", "Add feature")
	commits := parseGitLog("my-svc", raw)
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if commits[0].ServiceID != "my-svc" {
		t.Errorf("serviceID = %q, want my-svc", commits[0].ServiceID)
	}
}

// TestParseGitLog_FullBody is E2-1's core assertion: a multi-paragraph
// commit body — subject, blank line, prose, blank line, a Co-Authored-By
// trailer — round-trips into Message intact (not truncated to the subject),
// and blank lines INSIDE the body never fool the parser into splitting one
// commit into several or bleeding into the next record.
func TestParseGitLog_FullBody(t *testing.T) {
	body := "Fix auth M18.2\n\nLonger explanation of the change goes here,\nacross two lines.\n\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
	raw := rec("abc1234f", "Alice", "2024-01-15T10:00:00Z", body, "pkg/auth/auth.go") +
		rec("def5678a", "Bob", "2024-01-14T09:00:00Z", "Refactor", "pkg/util/util.go")

	commits := parseGitLog("svc", raw)
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	c0 := commits[0]
	if c0.Message != body {
		t.Errorf("message = %q, want %q", c0.Message, body)
	}
	if !strings.Contains(c0.Message, "\n") {
		t.Error("message should contain a newline (full body, not just subject)")
	}
	if !strings.Contains(c0.Message, "Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>") {
		t.Errorf("message missing trailer line: %q", c0.Message)
	}
	// The blank lines inside c0's body must not have split it into extra
	// bogus commits, and the second record must still parse cleanly.
	if commits[1].Hash != "def5678a" || commits[1].Message != "Refactor" {
		t.Errorf("second commit corrupted by first commit's multi-paragraph body: %+v", commits[1])
	}
	if len(c0.Files) != 1 || c0.Files[0] != "pkg/auth/auth.go" {
		t.Errorf("files = %v, want [pkg/auth/auth.go]", c0.Files)
	}
}

// TestParseGitLog_SpecIDInBody asserts the widened recall: a spec ID
// mentioned only in the body (never in the subject) is now found, whereas
// the old subject-only capture could never have seen it.
func TestParseGitLog_SpecIDInBody(t *testing.T) {
	body := "Tidy imports\n\nFollow-up to M20.1; no behavior change."
	raw := rec("abc1234f", "Alice", "2024-01-15T10:00:00Z", body)
	commits := parseGitLog("svc", raw)
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if len(commits[0].SpecIDs) != 1 || commits[0].SpecIDs[0] != "M20.1" {
		t.Errorf("spec_ids = %v, want [M20.1]", commits[0].SpecIDs)
	}
}
