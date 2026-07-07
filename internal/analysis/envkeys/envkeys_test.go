package envkeys_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/envkeys"
)

// ─── Go patterns ─────────────────────────────────────────────────────────────

const goSrc = `package main

import "os"

func main() {
	db := os.Getenv("DATABASE_URL")
	port := os.Getenv("PORT")
	secret, ok := os.LookupEnv("SECRET_KEY")
	_ = db; _ = port; _ = secret; _ = ok
}
`

func TestScan_Go_Getenv(t *testing.T) {
	keys := envkeys.Scan("main.go", goSrc)
	names := nameSet(keys)
	for _, want := range []string{"DATABASE_URL", "PORT", "SECRET_KEY"} {
		if !names[want] {
			t.Errorf("expected %q in results, got %v", want, names)
		}
	}
}

func TestScan_Go_LineNumbers(t *testing.T) {
	keys := envkeys.Scan("main.go", goSrc)
	lineOf := make(map[string]int)
	for _, k := range keys {
		lineOf[k.Name] = k.Line
	}
	if lineOf["DATABASE_URL"] != 6 {
		t.Errorf("DATABASE_URL on line %d, want 6", lineOf["DATABASE_URL"])
	}
	if lineOf["PORT"] != 7 {
		t.Errorf("PORT on line %d, want 7", lineOf["PORT"])
	}
	if lineOf["SECRET_KEY"] != 8 {
		t.Errorf("SECRET_KEY on line %d, want 8", lineOf["SECRET_KEY"])
	}
}

func TestScan_Go_FilePath(t *testing.T) {
	keys := envkeys.Scan("cmd/main.go", goSrc)
	for _, k := range keys {
		if k.File != "cmd/main.go" {
			t.Errorf("expected File=cmd/main.go, got %q", k.File)
		}
	}
}

// ─── JavaScript / TypeScript ─────────────────────────────────────────────────

const jsSrc = `const db = process.env.DATABASE_URL;
const port = process.env["PORT"];
const secret = process.env['SECRET_KEY'];
`

func TestScan_JS_Bare(t *testing.T) {
	keys := envkeys.Scan("config.js", jsSrc)
	names := nameSet(keys)
	if !names["DATABASE_URL"] {
		t.Errorf("expected DATABASE_URL (bare), got %v", names)
	}
}

func TestScan_JS_BracketDouble(t *testing.T) {
	keys := envkeys.Scan("config.js", jsSrc)
	names := nameSet(keys)
	if !names["PORT"] {
		t.Errorf("expected PORT (bracket double-quote), got %v", names)
	}
}

func TestScan_JS_BracketSingle(t *testing.T) {
	keys := envkeys.Scan("config.js", jsSrc)
	names := nameSet(keys)
	if !names["SECRET_KEY"] {
		t.Errorf("expected SECRET_KEY (bracket single-quote), got %v", names)
	}
}

// ─── Python ──────────────────────────────────────────────────────────────────

const pySrc = `import os

db = os.environ["DATABASE_URL"]
port = os.environ.get('PORT')
secret = os.getenv("SECRET_KEY")
`

func TestScan_Python_Environ(t *testing.T) {
	keys := envkeys.Scan("app.py", pySrc)
	names := nameSet(keys)
	if !names["DATABASE_URL"] {
		t.Errorf("expected DATABASE_URL from os.environ[], got %v", names)
	}
}

func TestScan_Python_EnvironGet(t *testing.T) {
	keys := envkeys.Scan("app.py", pySrc)
	names := nameSet(keys)
	if !names["PORT"] {
		t.Errorf("expected PORT from os.environ.get(), got %v", names)
	}
}

func TestScan_Python_Getenv(t *testing.T) {
	keys := envkeys.Scan("app.py", pySrc)
	names := nameSet(keys)
	if !names["SECRET_KEY"] {
		t.Errorf("expected SECRET_KEY from os.getenv(), got %v", names)
	}
}

// ─── Multiple reads same line ─────────────────────────────────────────────────

func TestScan_MultipleOnOneLine(t *testing.T) {
	src := `x := os.Getenv("A") + os.Getenv("B")` + "\n"
	keys := envkeys.Scan("f.go", src)
	names := nameSet(keys)
	if !names["A"] || !names["B"] {
		t.Errorf("expected A and B, got %v", names)
	}
}

// ─── Deduplication on same line ───────────────────────────────────────────────

func TestScan_DeduplicateSameLine(t *testing.T) {
	src := `x := os.Getenv("X") + os.Getenv("X")` + "\n"
	keys := envkeys.Scan("f.go", src)
	count := 0
	for _, k := range keys {
		if k.Name == "X" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected X to appear once, got %d times", count)
	}
}

// ─── Sorted output ────────────────────────────────────────────────────────────

func TestScan_Sorted(t *testing.T) {
	keys := envkeys.Scan("f.go", goSrc)
	for i := 1; i < len(keys); i++ {
		prev, cur := keys[i-1], keys[i]
		if prev.File > cur.File {
			t.Errorf("not sorted by File at %d", i)
		}
		if prev.File == cur.File && prev.Line > cur.Line {
			t.Errorf("not sorted by Line at %d", i)
		}
		if prev.File == cur.File && prev.Line == cur.Line && prev.Name > cur.Name {
			t.Errorf("not sorted by Name at %d", i)
		}
	}
}

// ─── Edge cases ───────────────────────────────────────────────────────────────

func TestScan_EmptySource(t *testing.T) {
	keys := envkeys.Scan("e.go", "")
	if len(keys) != 0 {
		t.Errorf("expected no keys for empty source, got %v", keys)
	}
}

func TestScan_NoEnvReads(t *testing.T) {
	src := "package foo\nfunc f() {}\n"
	keys := envkeys.Scan("f.go", src)
	if len(keys) != 0 {
		t.Errorf("expected no keys, got %v", keys)
	}
}

// ─── Determinism ─────────────────────────────────────────────────────────────

func TestScan_Determinism(t *testing.T) {
	a := envkeys.Scan("main.go", goSrc)
	b := envkeys.Scan("main.go", goSrc)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic length: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("key[%d] differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func nameSet(keys []envkeys.EnvKey) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k.Name] = true
	}
	return m
}
