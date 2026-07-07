package antipattern_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/antipattern"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func ruleNames(fs []antipattern.Finding) []string {
	names := make([]string, len(fs))
	for i, f := range fs {
		names[i] = f.Rule
	}
	return names
}

func hasRule(fs []antipattern.Finding, rule string) bool {
	for _, f := range fs {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

// ─── MagicNumbers ────────────────────────────────────────────────────────────

const magicSrc = `package foo

import "fmt"

func compute(x int) int {
	return x * 42        // magic
}

func threshold() float64 {
	return 3.14          // magic
}

const allowed = 999     // OK — in const decl
var alsoOK = 7          // OK — in var decl

func trivials() {
	x := 0              // OK — trivial
	y := 1              // OK — trivial
	_ = fmt.Sprintf("%d %d", x, y)
}
`

func TestMagicNumbers_Flags(t *testing.T) {
	fs := antipattern.MagicNumbers("foo.go", magicSrc)
	if !hasRule(fs, "magic-number") {
		t.Fatal("expected magic-number findings, got none")
	}
}

func TestMagicNumbers_ConstVarOK(t *testing.T) {
	// 999 and 7 are in const/var decls — must NOT be flagged.
	fs := antipattern.MagicNumbers("foo.go", magicSrc)
	for _, f := range fs {
		if f.Message == "magic number 999" || f.Message == "magic number 7" {
			t.Errorf("const/var literal incorrectly flagged: %+v", f)
		}
	}
}

func TestMagicNumbers_TrivialOK(t *testing.T) {
	src := `package foo
func f() int { return 0 + 1 }
`
	fs := antipattern.MagicNumbers("foo.go", src)
	if len(fs) != 0 {
		t.Errorf("expected no findings for 0 and 1, got %+v", fs)
	}
}

func TestMagicNumbers_Determinism(t *testing.T) {
	a := antipattern.MagicNumbers("foo.go", magicSrc)
	b := antipattern.MagicNumbers("foo.go", magicSrc)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding[%d] differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// ─── PanicSites ──────────────────────────────────────────────────────────────

const panicSrc = `package foo

func risky(x int) {
	if x < 0 {
		panic("negative x")
	}
}

func safe(x int) int { return x + 1 }
`

func TestPanicSites_Flags(t *testing.T) {
	fs := antipattern.PanicSites("foo.go", panicSrc)
	if !hasRule(fs, "panic-site") {
		t.Fatalf("expected panic-site finding, got %+v", fs)
	}
	if len(fs) != 1 {
		t.Errorf("expected 1 finding, got %d", len(fs))
	}
}

func TestPanicSites_Benign(t *testing.T) {
	fs := antipattern.PanicSites("foo.go", "package foo\nfunc safe() int { return 1 }\n")
	if len(fs) != 0 {
		t.Errorf("expected no findings, got %+v", fs)
	}
}

func TestPanicSites_Determinism(t *testing.T) {
	a := antipattern.PanicSites("foo.go", panicSrc)
	b := antipattern.PanicSites("foo.go", panicSrc)
	if len(a) != len(b) {
		t.Fatal("non-deterministic length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding[%d] differs", i)
		}
	}
}

// ─── Goroutines ───────────────────────────────────────────────────────────────

const goroutineSrc = `package foo

func startWorker(ch chan int) {
	go worker(ch)
}

func worker(ch chan int) {
	ch <- 1
}
`

func TestGoroutines_Flags(t *testing.T) {
	fs := antipattern.Goroutines("foo.go", goroutineSrc)
	if !hasRule(fs, "goroutine") {
		t.Fatalf("expected goroutine finding, got %+v", fs)
	}
}

func TestGoroutines_Benign(t *testing.T) {
	fs := antipattern.Goroutines("foo.go", "package foo\nfunc f() {}\n")
	if len(fs) != 0 {
		t.Errorf("expected no findings, got %+v", fs)
	}
}

func TestGoroutines_Determinism(t *testing.T) {
	a := antipattern.Goroutines("foo.go", goroutineSrc)
	b := antipattern.Goroutines("foo.go", goroutineSrc)
	if len(a) != len(b) {
		t.Fatal("non-deterministic length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding[%d] differs", i)
		}
	}
}

// ─── RegexInLoop ─────────────────────────────────────────────────────────────

const regexLoopSrc = `package foo

import "regexp"

func bad(words []string) {
	for _, w := range words {
		re := regexp.MustCompile(w)  // flagged
		_ = re
	}
}

var precompiled = regexp.MustCompile("hello")  // NOT flagged

func good(words []string) {
	re := regexp.MustCompile("fixed")  // NOT in loop — NOT flagged
	for _, w := range words {
		_ = re.MatchString(w)
	}
}
`

func TestRegexInLoop_Flags(t *testing.T) {
	fs := antipattern.RegexInLoop("foo.go", regexLoopSrc)
	if !hasRule(fs, "regex-in-loop") {
		t.Fatalf("expected regex-in-loop finding, got %+v", fs)
	}
}

func TestRegexInLoop_PrecompiledOK(t *testing.T) {
	fs := antipattern.RegexInLoop("foo.go", regexLoopSrc)
	// Should be exactly 1 finding (the MustCompile inside the range body).
	if len(fs) != 1 {
		t.Errorf("expected 1 finding, got %d: %+v", len(fs), fs)
	}
}

func TestRegexInLoop_Benign(t *testing.T) {
	src := `package foo
import "regexp"
var re = regexp.MustCompile("x")
func f() bool { return re.MatchString("y") }
`
	fs := antipattern.RegexInLoop("foo.go", src)
	if len(fs) != 0 {
		t.Errorf("expected no findings, got %+v", fs)
	}
}

func TestRegexInLoop_Determinism(t *testing.T) {
	a := antipattern.RegexInLoop("foo.go", regexLoopSrc)
	b := antipattern.RegexInLoop("foo.go", regexLoopSrc)
	if len(a) != len(b) {
		t.Fatal("non-deterministic length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding[%d] differs", i)
		}
	}
}

// ─── WallClock ────────────────────────────────────────────────────────────────

const wallClockSrc = `package service

import "time"

func processOrder() {
	t := time.Now()   // flagged in business logic
	_ = t
}
`

func TestWallClock_Flags(t *testing.T) {
	fs := antipattern.WallClock("service.go", wallClockSrc)
	if !hasRule(fs, "wall-clock") {
		t.Fatalf("expected wall-clock finding, got %+v", fs)
	}
}

func TestWallClock_TestFileOK(t *testing.T) {
	fs := antipattern.WallClock("service_test.go", wallClockSrc)
	if len(fs) != 0 {
		t.Errorf("expected no findings in test file, got %+v", fs)
	}
}

func TestWallClock_MainFileOK(t *testing.T) {
	mainSrc := `package main
import "time"
func main() { t := time.Now(); _ = t }
`
	fs := antipattern.WallClock("main.go", mainSrc)
	if len(fs) != 0 {
		t.Errorf("expected no findings in main file, got %+v", fs)
	}
}

func TestWallClock_Determinism(t *testing.T) {
	a := antipattern.WallClock("service.go", wallClockSrc)
	b := antipattern.WallClock("service.go", wallClockSrc)
	if len(a) != len(b) {
		t.Fatal("non-deterministic length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding[%d] differs", i)
		}
	}
}

// ─── MutexAudit ──────────────────────────────────────────────────────────────

const mutexSrc = `package foo

import "sync"

type Cache struct {
	mu   sync.Mutex   // flagged: mutex-field
	data map[string]string
}

func (c *Cache) Get(key string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data[key]
}

func (c *Cache) BrokenSet(key, val string) {
	c.mu.Lock()
	// forgot Unlock — imbalance
	c.data[key] = val
}
`

func TestMutexAudit_FieldFlagged(t *testing.T) {
	fs := antipattern.MutexAudit("foo.go", mutexSrc)
	if !hasRule(fs, "mutex-field") {
		t.Fatalf("expected mutex-field finding, got %+v", fs)
	}
}

func TestMutexAudit_Imbalance(t *testing.T) {
	fs := antipattern.MutexAudit("foo.go", mutexSrc)
	if !hasRule(fs, "mutex-imbalance") {
		t.Fatalf("expected mutex-imbalance finding, got %+v", fs)
	}
}

func TestMutexAudit_BalancedOK(t *testing.T) {
	src := `package foo
import "sync"
var mu sync.Mutex
func f() { mu.Lock(); defer mu.Unlock() }
`
	fs := antipattern.MutexAudit("foo.go", src)
	for _, f := range fs {
		if f.Rule == "mutex-imbalance" {
			t.Errorf("balanced Lock/Unlock incorrectly flagged: %+v", f)
		}
	}
}

func TestMutexAudit_Determinism(t *testing.T) {
	a := antipattern.MutexAudit("foo.go", mutexSrc)
	b := antipattern.MutexAudit("foo.go", mutexSrc)
	if len(a) != len(b) {
		t.Fatal("non-deterministic length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding[%d] differs", i)
		}
	}
}

// ─── ScanAll ─────────────────────────────────────────────────────────────────

const allSrc = `package svc

import (
	"regexp"
	"sync"
	"time"
)

type S struct{ mu sync.Mutex }

func (s *S) Run(items []string) {
	t := time.Now()
	_ = t
	go func() {}()
	for _, item := range items {
		re := regexp.MustCompile(item)
		_ = re
	}
	panic("abort")
	_ = 42
}
`

func TestScanAll_MultipleRules(t *testing.T) {
	fs := antipattern.ScanAll("svc.go", allSrc)
	want := []string{"goroutine", "magic-number", "mutex-field", "panic-site", "regex-in-loop", "wall-clock"}
	found := map[string]bool{}
	for _, f := range fs {
		found[f.Rule] = true
	}
	for _, r := range want {
		if !found[r] {
			t.Errorf("expected rule %q in ScanAll output", r)
		}
	}
}

func TestScanAll_Sorted(t *testing.T) {
	fs := antipattern.ScanAll("svc.go", allSrc)
	for i := 1; i < len(fs); i++ {
		prev, cur := fs[i-1], fs[i]
		if prev.File > cur.File {
			t.Errorf("not sorted by File at %d", i)
		}
		if prev.File == cur.File && prev.Line > cur.Line {
			t.Errorf("not sorted by Line at %d", i)
		}
		if prev.File == cur.File && prev.Line == cur.Line && prev.Rule > cur.Rule {
			t.Errorf("not sorted by Rule at %d", i)
		}
	}
}

func TestScanAll_Determinism(t *testing.T) {
	a := antipattern.ScanAll("svc.go", allSrc)
	b := antipattern.ScanAll("svc.go", allSrc)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding[%d] differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// ─── invalid source (no panic) ───────────────────────────────────────────────

func TestScanners_InvalidSrc(t *testing.T) {
	garbage := "this is not valid Go {{{{{"
	// None should panic; all should return nil/empty.
	_ = antipattern.MagicNumbers("x.go", garbage)
	_ = antipattern.PanicSites("x.go", garbage)
	_ = antipattern.Goroutines("x.go", garbage)
	_ = antipattern.RegexInLoop("x.go", garbage)
	_ = antipattern.WallClock("x.go", garbage)
	_ = antipattern.MutexAudit("x.go", garbage)
	_ = antipattern.ScanAll("x.go", garbage)
}

// ─── unused import check (ruleNames used) ────────────────────────────────────

var _ = ruleNames // suppress "declared and not used" if linter complains
