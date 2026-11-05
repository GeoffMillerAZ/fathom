package lang_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// manyDefs is a Python file of n small functions.
func manyDefs(n int) string { return strings.Repeat("def f():\n    return 1\n\n", n) }

// N6: a file the engine cannot read within its budgets is incomplete, with one
// line that names the budget and no wasm stack. A 690 KB Python file of 30,000
// functions is under the 2 MiB file limit and over the grammar module's 64 MiB
// memory limit: the guest traps on the allocation that does not fit.
func TestMemoryBudgetIsNamedNotATrap(t *testing.T) {
	r := newReader(t)
	src := manyDefs(30000)
	if len(src) > lang.DefaultMaxFileBytes {
		t.Fatalf("setup: %d bytes is over the file limit", len(src))
	}
	got := extract(t, r, "big/defs.py", src)
	if got.Skipped != "" || got.Dialect != "python" {
		t.Fatalf("setup: skipped %q dialect %q", got.Skipped, got.Dialect)
	}
	if !got.Incomplete || len(got.Symbols) != 0 {
		t.Fatalf("a file the engine could not read: incomplete %v, %d symbols", got.Incomplete, len(got.Symbols))
	}
	if len(got.Notes) != 1 {
		t.Fatalf("notes %q, want one line", got.Notes)
	}
	n := got.Notes[0]
	if strings.ContainsAny(n, "\n\t") || strings.Contains(n, "wasm") || strings.Contains(n, "stack") {
		t.Errorf("the note carries a stack: %q", n)
	}
	// A slow runner under -race can spend the 10 s parse deadline first: the
	// note then names the deadline, which is the other budget the engine names.
	if !strings.Contains(n, "memory limit (64 MiB, MemoryLimitPages)") && !strings.Contains(n, "parse and query deadline (ParseTimeout)") {
		t.Errorf("the note does not name the budget spent: %q", n)
	}
	// A smaller file of the same shape reads fully.
	if ok := extract(t, r, "big/defs.py", manyDefs(2000)); ok.Incomplete || len(ok.Symbols) < 2000 {
		t.Errorf("a 2,000 function file: incomplete %v, %d symbols, notes %v", ok.Incomplete, len(ok.Symbols), ok.Notes)
	}
}

// N1: an injected range whose extraction fails (here the memory budget, spent by
// 40,000 functions inside a <script>) leaves the host's symbols and sets
// Incomplete with the range named, where it used to be skipped silently.
func TestFailedInjectedRangeIsIncomplete(t *testing.T) {
	r := newReader(t)
	script := strings.Repeat("function f() { return 1 }\n", 40000)
	src := "<html><body><h1 id=\"t\">x</h1><script>\n" + script + "</script></body></html>\n"
	if len(src) > lang.DefaultMaxFileBytes {
		t.Fatalf("setup: %d bytes is over the file limit", len(src))
	}
	got := extract(t, r, "page.html", src)
	if got.Skipped != "" || got.Dialect != "html" {
		t.Fatalf("setup: skipped %q dialect %q", got.Skipped, got.Dialect)
	}
	if !got.Incomplete || len(got.Notes) != 1 || !strings.Contains(got.Notes[0], "injected") || strings.Contains(got.Notes[0], "\n") {
		t.Fatalf("a failed <script> reads incomplete=%v notes=%q", got.Incomplete, got.Notes)
	}
	ok := extract(t, r, "page.html", "<html><body><script>\nfunction f() { return 1 }\n</script></body></html>\n")
	if ok.Incomplete || len(ok.Notes) != 0 {
		t.Errorf("a small page reads incomplete: %v", ok.Notes)
	}
}

// The ParseTimeout deadline is the one wall-clock input to a Result: past it a
// file reads incomplete with the deadline named, so a slower machine can read
// fewer files fully than a faster one. The design states it as the exception to
// byte-equal output.
func TestTimeBudgetIsNamed(t *testing.T) {
	r := newReader(t)
	defer shrinkParseTimeout()()
	// The guest's own budget fires (a null tree), and the hard backstop that
	// covers a phase the guest cannot check is set out of the way.
	old := tsengine.ParseHardBackstop
	tsengine.ParseHardBackstop = 30 * time.Second
	defer func() { tsengine.ParseHardBackstop = old }()
	for _, backstop := range []time.Duration{30 * time.Second, 0} { // 0 is twice the deadline: the backstop wins
		tsengine.ParseHardBackstop = backstop
		got := extract(t, r, "big/defs.py", manyDefs(2000))
		if !got.Incomplete || len(got.Notes) != 1 || !strings.Contains(got.Notes[0], "parse and query deadline (ParseTimeout)") || strings.Contains(got.Notes[0], "\n") {
			t.Fatalf("a nanosecond deadline with backstop %v: incomplete %v, notes %q", backstop, got.Incomplete, got.Notes)
		}
	}
}

// ctx is checked on entry and between the stages of a file (after dispatch and
// after the extractor), never inside a parse.
func TestContextIsHonoredBetweenStages(t *testing.T) {
	r := newReader(t)

	// A deadline that expires during a parse returns its error once the parse
	// ends, not nil: the result of a file read past its deadline is not returned.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := r.Extract(ctx, lang.File{Path: "big/defs.py", Src: []byte(manyDefs(8000))})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a 1 ms deadline over a multi-millisecond parse returned %v, want DeadlineExceeded", err)
	}

	// A context cancelled while a Go extractor runs is reported after it returns.
	ctx2, cancel2 := context.WithCancel(context.Background())
	nr := newReader(t, lang.WithExtractor(rowMatch("row-spec"), cancelingExtractor{cancel2}))
	if _, err := nr.Extract(ctx2, lang.File{Path: "a/b.row", Src: []byte("def Main\n")}); !errors.Is(err, context.Canceled) {
		t.Errorf("a context cancelled inside the extractor returned %v, want Canceled", err)
	}

	// A live context changes nothing.
	if _, err := r.Extract(context.Background(), lang.File{Path: "a.py", Src: []byte("def f():\n    pass\n")}); err != nil {
		t.Errorf("a live context: %v", err)
	}
}

type cancelingExtractor struct{ cancel context.CancelFunc }

func (c cancelingExtractor) Extract(context.Context, lang.File) (lang.Extracted, error) {
	c.cancel()
	return lang.Extracted{}, nil
}
