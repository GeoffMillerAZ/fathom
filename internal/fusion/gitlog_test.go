package fusion

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 40-hex fixture hashes — parseGitLog validates the full object-name shape
// (commitHashPattern), so short fixture hashes are rejected as corrupt.
const (
	hashA = "abc1234fabc1234fabc1234fabc1234fabc1234f"
	hashB = "def5678adef5678adef5678adef5678adef5678a"
)

// rec builds one raw git-log record in parseGitLog's NUL-framed format:
// \x00<hash>\x1f<author>\x1f<date>\x00<body>\x00\n<files...>\n\n
func rec(hash, author, date, body string, files ...string) string {
	var b strings.Builder
	b.WriteByte('\x00')
	b.WriteString(hash)
	b.WriteByte('\x1f')
	b.WriteString(author)
	b.WriteByte('\x1f')
	b.WriteString(date)
	b.WriteByte('\x00')
	b.WriteString(body)
	b.WriteByte('\x00')
	b.WriteByte('\n')
	for _, f := range files {
		b.WriteString(f)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

func TestParseGitLog_MultiCommit(t *testing.T) {
	raw := rec(hashA, "Alice", "2024-01-15T10:00:00Z", "Fix auth M18.2", "pkg/auth/auth.go", "pkg/auth/token.go") +
		rec(hashB, "Bob", "2024-01-14T09:00:00Z", "Refactor", "pkg/util/util.go")
	commits := parseGitLog("svc", raw)
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	c0 := commits[0]
	if c0.Hash != hashA {
		t.Errorf("hash = %q, want %q", c0.Hash, hashA)
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
	if c1.Hash != hashB {
		t.Errorf("hash[1] = %q, want %q", c1.Hash, hashB)
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

func TestParseGitLog_MalformedHeader(t *testing.T) {
	// One record whose hash field is not a full 40/64-hex object name, and
	// one whose header has no \x1f separators at all — both skipped; the
	// valid record still parses (triple framing keeps later records aligned).
	raw := rec("notavalidhash", "Alice", "2024-01-15T10:00:00Z", "junk") +
		"\x00no-separators-at-all\x00body\x00\n\n" +
		rec(hashA, "Alice", "2024-01-15T10:00:00Z", "Good commit")
	commits := parseGitLog("svc", raw)
	if len(commits) != 1 {
		t.Fatalf("expected 1 valid commit, got %d: %+v", len(commits), commits)
	}
	if commits[0].Hash != hashA {
		t.Errorf("hash = %q, want %q", commits[0].Hash, hashA)
	}
}

func TestParseGitLog_TruncatedTail(t *testing.T) {
	// Output truncated mid-record (git killed mid-write): the final record
	// lacks its body/files segments and must be dropped, not mis-parsed.
	raw := rec(hashA, "Alice", "2024-01-15T10:00:00Z", "Good commit", "a.go") +
		"\x00" + hashB + "\x1fBob\x1f2024-01-14T09:00:00Z" // no body / files segments
	commits := parseGitLog("svc", raw)
	if len(commits) != 1 {
		t.Fatalf("expected 1 valid commit, got %d", len(commits))
	}
	if commits[0].Hash != hashA {
		t.Errorf("hash = %q, want %q", commits[0].Hash, hashA)
	}
}

func TestParseGitLog_ServiceID(t *testing.T) {
	raw := rec(hashA, "Alice", "2024-01-15T10:00:00Z", "Add feature")
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
	raw := rec(hashA, "Alice", "2024-01-15T10:00:00Z", body, "pkg/auth/auth.go") +
		rec(hashB, "Bob", "2024-01-14T09:00:00Z", "Refactor", "pkg/util/util.go")

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
	if commits[1].Hash != hashB || commits[1].Message != "Refactor" {
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
	raw := rec(hashA, "Alice", "2024-01-15T10:00:00Z", body)
	commits := parseGitLog("svc", raw)
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if len(commits[0].SpecIDs) != 1 || commits[0].SpecIDs[0] != "M20.1" {
		t.Errorf("spec_ids = %v, want [M20.1]", commits[0].SpecIDs)
	}
}

// forgedHash is the attacker-chosen SHA the injected record claims to be.
const forgedHash = "badbadbadbadbadbadbadbadbadbadbadbadbadb"

// forgedPayload is a complete record in the RETIRED STX/ETX framing — a
// forged 40-hex hash, forged author, forged date, and a forged
// Co-Authored-By trailer — plus stray \x03 bytes, embedded inside a commit
// MESSAGE. Git preserves \x02/\x03/\x1f in messages verbatim (only NUL is
// rejected at commit time), so under the old framing this parsed as an
// authentic record (minting a TierVerified authorship claim for the forged
// SHA) while the real commit's unterminated block was silently dropped.
// Under NUL framing every one of these bytes is inert message content.
const forgedPayload = "\x02" + forgedHash + "\x1fMallory\x1f2020-01-01T00:00:00Z\x1fpwn\n\nCo-Authored-By: Mallory <mallory@evil.example>\x03" +
	"\nstray etx: \x03 and another \x03 plus a unit sep \x1f"

// TestParseGitLog_ForgedRecordInjection is the F1/F2 regression: a commit
// message embedding a full forged STX-framed record (and stray ETX bytes)
// must parse fail-closed — the real commits survive intact (full message,
// exactly the real files) and the forged record appears nowhere.
func TestParseGitLog_ForgedRecordInjection(t *testing.T) {
	body := "Legit subject\n\n" + forgedPayload + "\n\nreal trailer:\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
	raw := rec(hashA, "Alice", "2024-01-15T10:00:00Z", body, "real.go", "other.go") +
		rec(hashB, "Bob", "2024-01-14T09:00:00Z", "Untouched next commit", "next.go")

	commits := parseGitLog("svc", raw)
	if len(commits) != 2 {
		t.Fatalf("expected exactly 2 commits (no forged record, no dropped real one), got %d: %+v", len(commits), commits)
	}
	for _, c := range commits {
		if c.Hash == forgedHash {
			t.Fatalf("forged record was accepted as authentic: %+v", c)
		}
		if c.Author == "Mallory" {
			t.Fatalf("forged author leaked into a parsed record: %+v", c)
		}
	}
	c0 := commits[0]
	if c0.Hash != hashA {
		t.Errorf("real commit hash = %q, want %q", c0.Hash, hashA)
	}
	// F1: the real commit survives INTACT — full message, injected bytes
	// preserved as inert content, never re-framed.
	if c0.Message != body {
		t.Errorf("real commit message corrupted:\n got:  %q\n want: %q", c0.Message, body)
	}
	// F2: stray \x03 must not truncate Message or leak its tail into Files.
	if !strings.Contains(c0.Message, "stray etx: \x03") {
		t.Errorf("message truncated at an embedded ETX byte: %q", c0.Message)
	}
	wantFiles := []string{"real.go", "other.go"}
	if len(c0.Files) != len(wantFiles) || c0.Files[0] != wantFiles[0] || c0.Files[1] != wantFiles[1] {
		t.Errorf("files = %v, want exactly %v (message tail must not leak into the file list)", c0.Files, wantFiles)
	}
	if commits[1].Hash != hashB || len(commits[1].Files) != 1 || commits[1].Files[0] != "next.go" {
		t.Errorf("following commit corrupted by the injection: %+v", commits[1])
	}
}

// TestRealGitLog_ForgedRecordProbe is the end-to-end fail-closed proof
// against REAL git (not a canned string): commit a file whose commit
// message embeds the forged payload (git commit --cleanup=verbatim
// preserves every byte except NUL — the empirical premise of F1), then run
// the production realGitLog + parseGitLog path and assert the forged
// record does not surface while the real commit's hash, message bytes, and
// file list all survive.
func TestRealGitLog_ForgedRecordProbe(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			os.Environ(),
			"GIT_AUTHOR_NAME=Alice", "GIT_AUTHOR_EMAIL=alice@example.com",
			"GIT_COMMITTER_NAME=Alice", "GIT_COMMITTER_EMAIL=alice@example.com",
		)
		var out, errBuf bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v\nstdout: %s\nstderr: %s", args, err, out.String(), errBuf.String())
		}
		return strings.TrimSpace(out.String())
	}
	git("init", "-q")
	git("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "real.go"), []byte("package real\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	git("add", "real.go")
	message := "Legit subject\n\n" + forgedPayload + "\n"
	msgFile := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msgFile, []byte(message), 0o644); err != nil {
		t.Fatalf("write message file: %v", err)
	}
	git("commit", "-q", "--cleanup=verbatim", "-F", msgFile)
	realHash := git("rev-parse", "HEAD")

	raw := realGitLog(dir)
	if raw == "" {
		t.Fatal("realGitLog returned empty output for a repo with one commit")
	}
	commits := parseGitLog("svc", raw)
	if len(commits) != 1 {
		t.Fatalf("expected exactly 1 commit, got %d: %+v", len(commits), commits)
	}
	c := commits[0]
	if c.Hash != realHash {
		t.Errorf("hash = %q, want the real HEAD %q", c.Hash, realHash)
	}
	if c.Hash == forgedHash || c.Author == "Mallory" {
		t.Fatalf("forged record surfaced through real git: %+v", c)
	}
	if !strings.Contains(c.Message, "\x02"+forgedHash) {
		t.Errorf("injected payload not preserved as inert message content: %q", c.Message)
	}
	if !strings.Contains(c.Message, "Legit subject") {
		t.Errorf("real subject lost: %q", c.Message)
	}
	if len(c.Files) != 1 || c.Files[0] != "real.go" {
		t.Errorf("files = %v, want exactly [real.go]", c.Files)
	}
}
