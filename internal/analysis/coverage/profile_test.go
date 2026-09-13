package coverage

import (
	"strings"
	"testing"
)

func TestParseProfile_Set(t *testing.T) {
	in := "mode: set\n" +
		"github.com/geoffmilleraz/fathom/internal/analysis/coverage/join.go:10.34,12.2 2 1\n" +
		"github.com/geoffmilleraz/fathom/internal/analysis/coverage/join.go:14.2,16.3 1 0\n"
	p, err := ParseProfile(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseProfile: %v", err)
	}
	if p.Mode != ModeSet {
		t.Fatalf("mode = %q, want set", p.Mode)
	}
	if len(p.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(p.Blocks))
	}
	b := p.Blocks[0]
	if b.ImportPath != "github.com/geoffmilleraz/fathom/internal/analysis/coverage/join.go" {
		t.Errorf("ImportPath = %q", b.ImportPath)
	}
	if b.StartLine != 10 || b.StartCol != 34 || b.EndLine != 12 || b.EndCol != 2 || b.NumStmt != 2 || b.Count != 1 {
		t.Errorf("block fields = %+v", b)
	}
	if p.Blocks[1].Count != 0 {
		t.Errorf("second block count = %d, want 0", p.Blocks[1].Count)
	}
}

func TestParseProfile_CountAndAtomic(t *testing.T) {
	for _, mode := range []string{"count", "atomic"} {
		p, err := ParseProfile(strings.NewReader("mode: " + mode + "\nfile.go:1.1,2.2 1 5\n"))
		if err != nil {
			t.Fatalf("mode %s: %v", mode, err)
		}
		if string(p.Mode) != mode {
			t.Errorf("mode = %q, want %q", p.Mode, mode)
		}
	}
}

func TestParseProfile_BlankLinesTolerated(t *testing.T) {
	in := "mode: set\n\nfile.go:1.1,2.2 1 1\n\n"
	p, err := ParseProfile(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseProfile: %v", err)
	}
	if len(p.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(p.Blocks))
	}
}

func TestParseProfile_MissingModeHeader(t *testing.T) {
	_, err := ParseProfile(strings.NewReader("file.go:1.1,2.2 1 1\n"))
	if err == nil {
		t.Fatal("expected an error for a missing mode header")
	}
	if !strings.Contains(err.Error(), "mode:") {
		t.Errorf("error %q does not mention the missing mode header", err)
	}
}

func TestParseProfile_UnknownMode(t *testing.T) {
	_, err := ParseProfile(strings.NewReader("mode: bogus\n"))
	if err == nil {
		t.Fatal("expected an error for an unknown mode")
	}
}

func TestParseProfile_MalformedBlockLine(t *testing.T) {
	_, err := ParseProfile(strings.NewReader("mode: set\nnot a block line\n"))
	if err == nil {
		t.Fatal("expected an error for a malformed block line")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q does not name the offending line", err)
	}
}

// CRITICAL fix (review_tool.go finding 3): ParseProfile is called on an
// MCP-supplied path (review_diff's coverage_profiles) — its errors must
// name the line number and a byte length, never echo the offending line's
// own content, or a wrong/sensitive path leaks its content through the
// MCP error channel.
func TestParseProfile_MalformedLineErrorCarriesNoContent(t *testing.T) {
	const secret = "super-sensitive-token-should-never-appear"
	_, err := ParseProfile(strings.NewReader("mode: set\n" + secret + "\n"))
	if err == nil {
		t.Fatal("expected an error for a malformed block line")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("ParseProfile error leaked the offending line's content: %q", err)
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q does not name the offending line number", err)
	}
	if !strings.Contains(err.Error(), "bytes") {
		t.Errorf("error %q does not report a byte length", err)
	}
}

func TestParseProfile_MissingModeHeaderErrorCarriesNoContent(t *testing.T) {
	const secret = "AKIA-fake-secret-shaped-header-line"
	_, err := ParseProfile(strings.NewReader(secret + "\n"))
	if err == nil {
		t.Fatal("expected an error for a missing mode header")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("ParseProfile error leaked the offending line's content: %q", err)
	}
}

func TestParseProfile_UnknownModeErrorCarriesNoContent(t *testing.T) {
	const secret = "secret-mode-value"
	_, err := ParseProfile(strings.NewReader("mode: " + secret + "\n"))
	if err == nil {
		t.Fatal("expected an error for an unknown mode")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("ParseProfile error leaked the mode value's content: %q", err)
	}
}

func TestParseProfile_EmptyInput(t *testing.T) {
	_, err := ParseProfile(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected an error for empty input")
	}
}
