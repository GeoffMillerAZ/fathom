package extractor

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// naiveLineOf is the locator as it was: every call scanned every line, and built
// a string per line, so a manifest of n keys cost n*n (39,900 dependencies under
// [dependencies], 428 KB, took 27 s). The indexed locator must answer what it
// answered.
func naiveLineOf(l *tomlLocator, keyPath []string, fallback int) int {
	full := strings.Join(keyPath, ".")
	explicit := func() (int, bool) {
		for _, t := range l.lines {
			if t.isHeader && t.table == full {
				return t.line, true
			}
		}
		for _, t := range l.lines {
			if t.isHeader {
				continue
			}
			if t.table == "" && t.key == full {
				return t.line, true
			}
			if t.table != "" && strings.HasPrefix(full, t.table+".") && t.key == strings.TrimPrefix(full, t.table+".") {
				if t.line >= fallback {
					return t.line, true
				}
			}
		}
		return 0, false
	}
	if line, ok := explicit(); ok {
		return line
	}
	prefix := full + "."
	for _, t := range l.lines {
		f := t.table
		if !t.isHeader {
			f = t.key
			if t.table != "" {
				f = t.table + "." + t.key
			}
		}
		if strings.HasPrefix(f, prefix) {
			return t.line
		}
	}
	return fallback
}

func TestTOMLLocatorAnswersAsTheScanDid(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	names := []string{"a", "b", "c", "tool", "x.y", `"q.k"`, "dependencies", "a.b"}
	for round := 0; round < 200; round++ {
		var b strings.Builder
		for i := 0; i < 5+rng.Intn(40); i++ {
			switch rng.Intn(5) {
			case 0:
				fmt.Fprintf(&b, "[%s]\n", names[rng.Intn(len(names))])
			case 1:
				fmt.Fprintf(&b, "[[%s]]\n", names[rng.Intn(len(names))])
			case 2:
				fmt.Fprintf(&b, "%s = 1\n", names[rng.Intn(len(names))])
			case 3:
				fmt.Fprintf(&b, "%s.%s = \"v\"\n", names[rng.Intn(len(names))], names[rng.Intn(len(names))])
			default:
				b.WriteString("\n# c\n")
			}
		}
		loc := newTOMLLocator([]byte(b.String()))
		var paths [][]string
		for _, n := range names {
			paths = append(paths, strings.Split(normTOMLKey(n), "."))
			for _, m := range names {
				paths = append(paths, append(strings.Split(normTOMLKey(n), "."), strings.Split(normTOMLKey(m), ".")...))
			}
		}
		for _, p := range paths {
			for _, fb := range []int{0, 1, 5, 20, 99} {
				if got, want := loc.lineOf(p, fb), naiveLineOf(loc, p, fb); got != want {
					t.Fatalf("lineOf(%v, %d) = %d, the scan said %d, in:\n%s", p, fb, got, want, b.String())
				}
			}
		}
	}
}

// The locator is asked for every key of the document, so it answers in constant
// time: 39,900 dependencies, inside the key bound, read in 27 s before.
func TestTOMLLocatorIsLinear(t *testing.T) {
	var b strings.Builder
	b.WriteString("[package]\nname = \"x\"\nversion = \"1.0.0\"\n\n[dependencies]\n")
	const n = 39_900
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "dep%d = \"1.%d\"\n", i, i)
	}
	start := time.Now()
	loc := newTOMLLocator([]byte(b.String()))
	for i := 0; i < n; i++ {
		if got, want := loc.lineOf([]string{"dependencies", fmt.Sprintf("dep%d", i)}, 1), 6+i; got != want {
			t.Fatalf("dep%d is on line %d, not %d", i, got, want)
		}
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("%d lookups took %v", n, d)
	}
}
