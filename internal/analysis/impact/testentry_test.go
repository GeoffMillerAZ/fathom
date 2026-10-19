package impact

import (
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/repofacts"
)

// testEntryFixture is one framework's test file and a production file that
// declares symbols of the same names outside a test file.
type testEntryFixture struct {
	framework string
	lang      string // extractor dialect
	testFile  string
	testSrc   string
	prodFile  string
	prodSrc   string
	entries   []string // the test file's entries, by symbol name
	notEntry  []string // the test file's other symbols the rule must refuse
	reached   []string // entries the blast radius of prodFile names
}

var testEntryFixtures = []testEntryFixture{
	{
		framework: "pytest", lang: "python",
		testFile: "tests/test_calc.py",
		testSrc: `from calc import add


def helper():
    return 0


def test_add():
    assert add(1, 2) == 3


class TestCalc:
    def test_sub(self):
        assert add(1, -1) == 0
`,
		prodFile: "src/calc.py",
		prodSrc: `def add(a, b):
    return a + b


def test_add():
    return 0


class TestCalc:
    def test_sub(self):
        return 0
`,
		entries:  []string{"TestCalc", "TestCalc.test_sub", "test_add"},
		notEntry: []string{"helper"},
		reached:  []string{"test_add"},
	},
	{
		framework: "jest", lang: "typescript",
		testFile: "__tests__/sum.test.ts",
		testSrc: `import { sum } from "../src/sum";

describe("sum", () => {
  it("adds", () => {
    expect(sum(1, 2)).toBe(3);
  });
  test("zero", () => {
    expect(sum(0, 0)).toBe(0);
  });
});

function helper(): number {
  return 0;
}
`,
		prodFile: "src/sum.ts",
		prodSrc: `export function sum(a: number, b: number): number {
  return a + b;
}

export function it(x: number): number {
  return x;
}

test("adds", () => sum(1, 1));
`,
		entries:  []string{`"sum"`, `"sum" "adds"`, `"sum" "zero"`},
		notEntry: []string{"helper"},
		reached:  []string{`"sum" "adds"`, `"sum" "zero"`},
	},
	{
		framework: "vitest", lang: "typescript",
		testFile: "src/mul.spec.ts",
		testSrc: `import { describe, it, expect } from "vitest";
import { mul } from "./mul";

describe.concurrent("mul", () => {
  it.skip("doubles", async () => {
    expect(mul(2, 2)).toBe(4);
  });
});
`,
		prodFile: "src/mul.ts",
		prodSrc: `export function mul(a: number, b: number): number {
  return a * b;
}

describe("mul", () => mul(1, 1));
`,
		entries: []string{`"mul"`, `"mul" "doubles"`},
		reached: []string{`"mul" "doubles"`},
	},
	{
		framework: "mocha", lang: "javascript",
		testFile: "test/calc.spec.js",
		testSrc: `import { add } from "../lib/calc.js";

describe("calc", function () {
  it("adds", function () {
    add(1, 2);
  });
});
`,
		prodFile: "lib/calc.js",
		prodSrc: `export function add(a, b) {
  return a + b;
}

it("adds", function () {
  add(1, 1);
});
`,
		entries: []string{`"calc"`, `"calc" "adds"`},
		reached: []string{`"calc" "adds"`},
	},
	{
		framework: "junit", lang: "java",
		testFile: "src/test/java/com/ex/CalcTest.java",
		testSrc: `package com.ex;

import org.junit.jupiter.api.Test;

class CalcTest {
    @Test
    void adds() {
        Calc.add(1, 2);
    }

    void helper() {
        Calc.add(0, 0);
    }
}
`,
		prodFile: "src/main/java/com/ex/Calc.java",
		prodSrc: `package com.ex;

class Calc {
    static int add(int a, int b) {
        return a + b;
    }

    @Test
    void adds() {
    }
}
`,
		entries:  []string{"CalcTest.adds"},
		notEntry: []string{"CalcTest.helper"},
		reached:  []string{"CalcTest.adds"},
	},
	{
		framework: "xunit-nunit", lang: "csharp",
		testFile: "Tests/CalcTests.cs",
		testSrc: `using Xunit;

public class CalcTests {
    [Fact]
    public void Adds() { Calc.Add(1, 2); }

    [Theory]
    [InlineData(1)]
    public void AddsMany(int a) { Calc.Add(a, a); }

    [Test]
    public void AddsNUnit() { Calc.Add(1, 1); }

    public void Helper() { Calc.Add(0, 0); }
}
`,
		prodFile: "src/Calc.cs",
		prodSrc: `public class Calc {
    public static int Add(int a, int b) { return a + b; }

    [Fact]
    public void Adds() { }
}
`,
		entries:  []string{"CalcTests.Adds", "CalcTests.AddsMany", "CalcTests.AddsNUnit"},
		notEntry: []string{"CalcTests.Helper"},
		reached:  []string{"CalcTests.Adds"},
	},
	{
		framework: "phpunit", lang: "php",
		testFile: "tests/CalcTest.php",
		testSrc: `<?php
use PHPUnit\Framework\TestCase;
use PHPUnit\Framework\Attributes\Test;

class CalcTest extends TestCase {
    public function testAdd(): void { Calc::add(1, 2); }

    #[Test]
    public function addsByAttribute(): void { Calc::add(1, 1); }

    public function helper(): void { Calc::add(0, 0); }
}
`,
		prodFile: "src/Calc.php",
		prodSrc: `<?php
class Calc {
    public static function add(int $a, int $b): int { return $a + $b; }

    public function testAdd(): void { }
}
`,
		entries:  []string{"CalcTest.addsByAttribute", "CalcTest.testAdd"},
		notEntry: []string{"CalcTest.helper"},
	},
	{
		framework: "rust", lang: "rust",
		testFile: "tests/integ.rs",
		testSrc: `use calc::add;

/// Adds two numbers.
#[test]
fn integ_adds() {
    assert_eq!(add(1, 2), 3);
}

#[test]
#[should_panic]
fn integ_panics() {
    add(0, 0);
    panic!("boom");
}

fn helper() -> i32 {
    add(0, 0)
}
`,
		prodFile: "src/lib.rs",
		prodSrc: `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

#[test]
fn integ_adds() {}
`,
		entries:  []string{"integ_adds", "integ_panics"},
		notEntry: []string{"helper"},
		reached:  []string{"integ_adds", "integ_panics"},
	},
}

// TestNonGoTestEntriesPytestJestJUnitXUnitPHPUnitRust is TQ-27: each
// framework's test entries, extracted from a file the role classifier calls
// a test, are test entries by the one rule (repofacts.IsTestEntry), and the
// blast radius of the production file they call names them; a symbol of
// the same name outside a test file, or a helper inside one, is not.
func TestNonGoTestEntriesPytestJestJUnitXUnitPHPUnitRust(t *testing.T) {
	for _, fx := range testEntryFixtures {
		t.Run(fx.framework, func(t *testing.T) {
			if !repofacts.IsTest(fx.testFile) || repofacts.IsTestMaterial(fx.prodFile) {
				t.Fatalf("fixture paths: IsTest(%s) %v, IsTestMaterial(%s) %v", fx.testFile, repofacts.IsTest(fx.testFile), fx.prodFile, repofacts.IsTestMaterial(fx.prodFile))
			}
			e, err := extractor.NewTSExtractor(fx.lang)
			if err != nil {
				t.Fatalf("NewTSExtractor(%s): %v", fx.lang, err)
			}
			defer e.Close()
			var nodes []core.SymbolNode
			for _, f := range [][2]string{{fx.testFile, fx.testSrc}, {fx.prodFile, fx.prodSrc}} {
				got, err := e.Extract("fx", f[0], []byte(f[1]))
				if err != nil {
					t.Fatalf("extract %s: %v", f[0], err)
				}
				nodes = append(nodes, got...)
			}

			var entries, prodNames []string
			prodKeys := map[string]bool{}
			for _, n := range nodes {
				if n.FilePath == fx.prodFile {
					prodKeys[n.Key()] = true
				}
				switch {
				case strings.HasSuffix(string(n.Kind), "Import"):
					// An import is never called, so no walk reaches it; the core
					// Test prefix still reads org.junit.jupiter.api.Test as Test
					// (O1289), as it did before TQ-27.
				case n.FilePath == fx.testFile && repofacts.IsTestEntry(n.Name, n.FilePath, n.Signature):
					entries = append(entries, n.Name)
				case n.FilePath == fx.prodFile:
					if repofacts.IsTestEntry(n.Name, n.FilePath, n.Signature) {
						t.Errorf("%s %q (signature %q) is a test entry outside a test file", fx.prodFile, n.Name, n.Signature)
					}
					prodNames = append(prodNames, n.Name)
				}
			}
			sort.Strings(entries)
			if strings.Join(entries, ",") != strings.Join(fx.entries, ",") {
				t.Errorf("entries in %s = %q, want %q\nnodes: %s", fx.testFile, entries, fx.entries, describeNodes(nodes, fx.testFile))
			}
			for _, name := range fx.notEntry {
				for _, n := range nodes {
					if n.FilePath == fx.testFile && n.Name == name && repofacts.IsTestEntry(n.Name, n.FilePath, n.Signature) {
						t.Errorf("helper %q in %s is a test entry", name, fx.testFile)
					}
				}
			}
			// The same-named production symbol exists, so the refusal above
			// was asked of it rather than of nothing.
			if !sharesAName(prodNames, fx.entries) {
				t.Errorf("%s declares no symbol named like an entry %q: %q", fx.prodFile, fx.entries, prodNames)
			}

			res := BlastRadius(nodes, []string{fx.prodFile})
			for _, want := range fx.reached {
				found := false
				for _, k := range res.Tests {
					if strings.HasSuffix(k, "."+want) || strings.HasSuffix(k, "::"+want) {
						found = true
					}
				}
				if !found {
					t.Errorf("BlastRadius(%s).Tests = %q, want it to name %q\nnodes: %s", fx.prodFile, res.Tests, want, describeNodes(nodes, ""))
				}
			}
			for _, k := range res.Tests {
				if prodKeys[k] {
					t.Errorf("BlastRadius(%s).Tests names production symbol %q", fx.prodFile, k)
				}
			}
		})
	}
}

func describeNodes(nodes []core.SymbolNode, file string) string {
	var b strings.Builder
	for _, n := range nodes {
		if file == "" || n.FilePath == file {
			b.WriteString(string(n.Kind) + " " + n.Name + " [" + n.Signature + "] calls=" + strings.Join(n.Calls, "|") + "; ")
		}
	}
	return b.String()
}

func sharesAName(names, entries []string) bool {
	for _, e := range entries {
		for _, n := range names {
			if lastSegment(n) == lastSegment(e) {
				return true
			}
		}
	}
	return false
}

func lastSegment(name string) string {
	if i := strings.LastIndexAny(name, ". "); i >= 0 {
		return name[i+1:]
	}
	return name
}
