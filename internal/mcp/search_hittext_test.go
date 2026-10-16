package mcp

// search_hittext_test.go — O1070: MCP search returned the literal text "d"
// for a hit at tests/test_requests.py:692 in psf/requests (611c6162). The
// per-hit byte budget shrank Text one rune at a time while every receipt
// field (a long test path, a long enclosing method, the test flag, the
// class) stayed exact, until the line was its first rune: the hit's one
// piece of content cut to nothing, and the match itself cut away.

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestRenderSearchHit_TextKeepsTheMatchUnderTheByteBudget(t *testing.T) {
	root := t.TempDir()
	lines := make([]string, 700)
	lines[684] = "    def test_basicauth_with_netrc_leak_regression_never_sends_auth(self, httpbin):"
	lines[691] = "            def get_netrc_auth_mock(url):"
	writeFlagFixture(t, root, "tests/test_requests.py", strings.Join(lines, "\n"))
	byFile := map[string][]core.SymbolNode{"tests/test_requests.py": {{
		Name: "TestRequests.test_basicauth_with_netrc_leak_regression_never_sends_auth", Kind: core.KindPyFunc,
		FilePath: "tests/test_requests.py", Line: 685, EndLine: 699,
	}}}
	re := regexp.MustCompile("get_netrc_auth")
	rh := searchRawHit{File: "tests/test_requests.py", Line: 692, Text: lines[691]}

	h := renderSearchHit(context.Background(), rh, re, byFile, map[string][]string{}, map[string][]string{}, root)
	if h.Sym == "" || h.Flags == "" {
		t.Fatalf("fixture does not reproduce the long receipt (sym %q, flags %q)", h.Sym, h.Flags)
	}
	if !strings.Contains(h.Text, "get_netrc_auth") {
		t.Fatalf("hit text %q lost the match (loc %s)", h.Text, h.Loc)
	}
}

// A long line whose match sits past the 80-character render: the window
// keeps the match, marked as cut on the side it was cut.
func TestRenderSearchHit_LongLineWindowsOnTheMatch(t *testing.T) {
	root := t.TempDir()
	line := strings.Repeat("x = 1; ", 20) + "needle_here = True"
	writeFlagFixture(t, root, "pkg/a.py", line+"\n")
	re := regexp.MustCompile("needle_here")
	h := renderSearchHit(context.Background(), searchRawHit{File: "pkg/a.py", Line: 1, Text: line}, re, nil, map[string][]string{}, map[string][]string{}, root)
	if !strings.Contains(h.Text, "needle_here") || !strings.HasPrefix(h.Text, "…") {
		t.Fatalf("hit text %q, want a window around the match opening with …", h.Text)
	}
	if n := len([]rune(h.Text)); n > searchTextMaxChars {
		t.Fatalf("hit text %q is %d runes, over the %d-rune line bound", h.Text, n, searchTextMaxChars)
	}
}
