package facts

import "testing"

func check(t *testing.T, ok bool) {
	t.Helper()
	if !ok {
		t.Fatal("not ok")
	}
}

func useTB(tb testing.TB) { tb.Log("x") }

// want: unchecked=yes wrapped=- executes_only=yes locked=- threaded=-
func TestOnlyRuns(t *testing.T) {
	_ = wrapsW(nil)
	t.Log("ran")
}

// want: unchecked=- wrapped=- executes_only=no locked=- threaded=-
func TestErrorf(t *testing.T) {
	if got := noErrorCalls("a"); got != "A" {
		t.Errorf("got %q", got)
	}
}

// want: unchecked=- wrapped=- executes_only=no locked=- threaded=-
// Passing t to a helper counts as asserting.
func TestHelperAsserts(t *testing.T) {
	check(t, noErrorCalls("a") == "A")
}

// want: unchecked=- wrapped=- executes_only=no locked=- threaded=-
// The assertion is in a subtest's function literal.
func TestSubtest(t *testing.T) {
	t.Run("a", func(t *testing.T) {
		if noErrorCalls("a") != "A" {
			t.Fatalf("bad")
		}
	})
}

// want: unchecked=- wrapped=- executes_only=yes locked=- threaded=-
// Tricky negative: Helper, TempDir, Cleanup and Run are not assertions.
func TestSubtestRunsOnly(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(func() {})
	t.Run("a", func(t *testing.T) { _ = noErrorCalls(dir) })
}

// want: unchecked=- wrapped=- executes_only=no locked=- threaded=-
func TestSkips(t *testing.T) {
	t.Skip("later")
}

// want: unchecked=- wrapped=- executes_only=no locked=- threaded=-
func TestPassesTB(t *testing.T) {
	var tb testing.TB = t
	useTB(tb)
}

// want: unchecked=- wrapped=- executes_only=- locked=- threaded=-
// A benchmark is not a Test func.
func BenchmarkUpper(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = noErrorCalls("a")
	}
}

// want: unchecked=- wrapped=- executes_only=- locked=- threaded=-
// Tricky negative: "Test" followed by a lower-case letter is not a test.
func Testimony(t *testing.T) {
	t.Log("x")
}
