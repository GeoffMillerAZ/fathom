package fusion

import (
	"testing"
)

func TestParseGitLog_MultiCommit(t *testing.T) {
	raw := "abc1234f\x1fAlice\x1f2024-01-15T10:00:00Z\x1fFix auth M18.2\npkg/auth/auth.go\npkg/auth/token.go\n\ndef5678a\x1fBob\x1f2024-01-14T09:00:00Z\x1fRefactor\npkg/util/util.go\n"
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
	// Block with no \x1f separator — should skip gracefully
	raw := "notavalidheader\n\nabc1234f\x1fAlice\x1f2024-01-15T10:00:00Z\x1fGood commit\n"
	commits := parseGitLog("svc", raw)
	// The valid block should parse; the malformed one should be skipped
	if len(commits) != 1 {
		t.Errorf("expected 1 valid commit, got %d", len(commits))
	}
}

func TestParseGitLog_ServiceID(t *testing.T) {
	raw := "abc1234f\x1fAlice\x1f2024-01-15T10:00:00Z\x1fAdd feature\n"
	commits := parseGitLog("my-svc", raw)
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if commits[0].ServiceID != "my-svc" {
		t.Errorf("serviceID = %q, want my-svc", commits[0].ServiceID)
	}
}
