package repofacts

import (
	"os/exec"
	"path"
	"strings"
	"testing"
)

// splitCases pin the R1 split (docs/bench/readings/repository-facts/roles-r1.md,
// pre-registered): each path's convention role after the split, and what
// IsTest and IsTestMaterial answered BEFORE it (frozen from the classifier at
// 3263f01f). The split partitions the former `test` into `test` and
// `test-support`, so the two delegates' answers never move.
var splitCases = []struct {
	path          string
	isTest, isMat bool
	want          RoleName
}{
	// Go: every _test.go a package holds is collected; the go tool ignores
	// _- and .-prefixed directories and files.
	{"a/b_test.go", true, true, Test},
	{"_examples/x_test.go", true, true, TestSupport},
	{".hidden/x_test.go", true, true, TestSupport},
	{"a/_x_test.go", true, true, TestSupport},
	{"a/.x_test.go", true, true, TestSupport},
	{"examples/a/x_test.go", true, true, Test},
	{"internal/testdata/x_test.go", false, true, Fixture},
	{"a/b.go", false, false, Source},
	// Python: example and documentation suites, pytest's norecursedirs.
	{"tests/test_a.py", true, true, Test},
	{"pkg/tests/a_test.py", true, true, Test},
	{"examples/tutorial/tests/test_app.py", true, true, TestSupport},
	{"docs_src/a/test_main.py", true, true, TestSupport},
	{"docs/x/test_y.py", true, true, TestSupport},
	{"samples/test_a.py", true, true, TestSupport},
	{"build/lib/tests/test_a.py", true, true, TestSupport},
	{".tox/x/test_a.py", true, true, TestSupport},
	{"venv/x/test_a.py", true, true, TestSupport},
	{"pkg.egg/test_a.py", true, true, TestSupport},
	{"dist/test_a.py", true, true, TestSupport},
	{"tests/conftest.py", false, true, Fixture},
	{"tests/helpers.py", false, true, Fixture},
	// TypeScript and JavaScript: __tests__/ helpers, example suites,
	// vitest's default excludes.
	{"src/a.test.ts", true, true, Test},
	{"web/a.spec.js", true, true, Test},
	{"src/__tests__/a.ts", true, true, Test},
	{"src/__tests__/a.test.ts", true, true, Test},
	{"src/__tests__/utils/a.test.ts", true, true, Test},
	{"src/__tests__/components/Foo.tsx", true, true, Test},
	{"src/__tests__/setup.ts", true, true, TestSupport},
	{"src/__tests__/utils/render.tsx", true, true, TestSupport},
	{"src/__tests__/_helpers.ts", true, true, TestSupport},
	{"src/__tests__/types.d.ts", true, true, TestSupport},
	{"src/__tests__/TestUtils.tsx", true, true, TestSupport},
	{"src/__tests__/index.js", true, true, TestSupport},
	{"src/__tests__/__mocks__/x.ts", true, true, TestSupport},
	{"examples/demo/src/a.test.tsx", true, true, TestSupport},
	{"docs/a.test.ts", true, true, TestSupport},
	{"dist/a.test.js", true, true, TestSupport},
	{"cypress/e2e/a.spec.ts", true, true, TestSupport},
	{".storybook/a.test.ts", true, true, TestSupport},
	{"__mocks__/x.ts", false, true, Fixture},
	// Java: abstract bases, *TestCase, Test* helpers, and a test name
	// outside a test tree; *IT stays a test (Failsafe's).
	{"src/test/java/a/FooTest.java", true, true, Test},
	{"src/test/java/a/FooTests.java", true, true, Test},
	{"src/test/java/a/FooIT.java", true, true, Test},
	{"examples/src/test/java/a/FooTest.java", true, true, Test},
	{"src/test/java/a/FooTestCase.java", true, true, TestSupport},
	{"src/test/java/a/AbstractFooTest.java", true, true, TestSupport},
	{"src/test/java/a/TestUtils.java", true, true, TestSupport},
	{"src/main/java/a/FooTest.java", true, true, TestSupport},
	{"src/test/java/a/Helper.java", false, true, Fixture},
	// PHP: abstract bases, a test name outside a test tree, example suites.
	{"tests/Unit/FooTest.php", true, true, Test},
	{"tests/AbstractFooTest.php", true, true, TestSupport},
	{"src/Foo/CreatesMatchingTest.php", true, true, TestSupport},
	{"examples/tests/FooTest.php", true, true, TestSupport},
	{"Tests/Feature/routes.php", false, true, Fixture},
	{"src/Foo.php", false, false, Source},
	// Packs RF-M8 does not grade keep their former answer.
	{"spec/models/user_spec.rb", true, true, Test},
	{"Tests/FooTests.cs", true, true, Test},
	{"crate/tests/it.rs", true, true, Test},
}

// TestConventionSplitsTestFromTestSupport (RF-M8, R1): the convention role
// reads `test` only for a file the runner collects, and `test-support` for
// test code it does not; RF-2's delegates answer exactly what they answered
// before the split.
func TestConventionSplitsTestFromTestSupport(t *testing.T) {
	for _, c := range splitCases {
		if got := ConventionRole(c.path); got != c.want {
			t.Errorf("ConventionRole(%q) = %q, want %q", c.path, got, c.want)
		}
		if got := Role(c.path, "", Attributes{}, nil); got.Basis == ByConvention && got.Role != c.want {
			t.Errorf("Role(%q) = %v, want the convention's %q", c.path, got, c.want)
		}
		if IsTest(c.path) != c.isTest || IsTestMaterial(c.path) != c.isMat {
			t.Errorf("%q: IsTest %v IsTestMaterial %v, want %v %v as before the split",
				c.path, IsTest(c.path), IsTestMaterial(c.path), c.isTest, c.isMat)
		}
	}
	if !IsTestEntry("TestX", "src/test/java/a/AbstractFooTest.java") || !IsTestEntry("TestX", "src/__tests__/setup.ts") {
		t.Error("IsTestEntry: a Test-named function in test-support code is still a test entry (IsTest reads test code)")
	}
}

// TestSplitIsAPartitionOfTheFormerTestRole: over every file this repository
// tracks, test-support is only ever carved out of what the pack patterns call
// a test, and IsTest is exactly test or test-support.
func TestSplitIsAPartitionOfTheFormerTestRole(t *testing.T) {
	out, err := exec.Command("git", "-C", "../..", "ls-files").Output()
	if err != nil {
		t.Skipf("git ls-files: %v", err)
	}
	paths := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, c := range splitCases {
		paths = append(paths, c.path)
	}
	for _, p := range paths {
		r := ConventionRole(p)
		code := r == Test || r == TestSupport
		if IsTest(p) != code {
			t.Errorf("IsTest(%q) = %v, but its convention role is %q", p, IsTest(p), r)
		}
		if IsTestMaterial(p) != (code || r == Fixture) {
			t.Errorf("IsTestMaterial(%q) = %v, but its convention role is %q", p, IsTestMaterial(p), r)
		}
		if r == TestSupport && !formerTest(p) {
			t.Errorf("%q reads test-support but no pack pattern calls it a test: the split moved a file across the old boundary", p)
		}
	}
}

// formerTest is the convention's test decision as it stood before the split:
// outside a vendored, generated or fixture tree, the file's pack patterns
// match it.
func formerTest(rel string) bool {
	base := path.Base(rel)
	p := packByExt[strings.ToLower(path.Ext(base))]
	if p == nil || anyDir(rel, vendorDirs, false) || anyDir(rel, generatedDirs, false) || anyDir(rel, fixtureDirs, false) {
		return false
	}
	return packTest(p, rel, base) != noTest
}
