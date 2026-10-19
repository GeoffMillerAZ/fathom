package repofacts

import "testing"

// delegateCases pin what RF-2's delegates answer, frozen at c39875f3 (before
// RF-M8's split) and unchanged through it and through the CUT that removed
// RF-1's role classifier (docs/bench/readings/repository-facts/roles-r1.md):
// the paths RF-M8 pre-registered its split on, every ecosystem's test,
// test-support and fixture shapes among them.
var delegateCases = []struct {
	path          string
	isTest, isMat bool
}{
	{"a/b_test.go", true, true},
	{"_examples/x_test.go", true, true},
	{".hidden/x_test.go", true, true},
	{"a/_x_test.go", true, true},
	{"a/.x_test.go", true, true},
	{"examples/a/x_test.go", true, true},
	{"internal/testdata/x_test.go", false, true},
	{"a/b.go", false, false},
	{"tests/test_a.py", true, true},
	{"pkg/tests/a_test.py", true, true},
	{"examples/tutorial/tests/test_app.py", true, true},
	{"docs_src/a/test_main.py", true, true},
	{"docs/x/test_y.py", true, true},
	{"samples/test_a.py", true, true},
	{"build/lib/tests/test_a.py", true, true},
	{".tox/x/test_a.py", true, true},
	{"venv/x/test_a.py", true, true},
	{"pkg.egg/test_a.py", true, true},
	{"dist/test_a.py", true, true},
	{"tests/conftest.py", false, true},
	{"tests/helpers.py", false, true},
	{"src/a.test.ts", true, true},
	{"web/a.spec.js", true, true},
	{"src/__tests__/a.ts", true, true},
	{"src/__tests__/a.test.ts", true, true},
	{"src/__tests__/utils/a.test.ts", true, true},
	{"src/__tests__/components/Foo.tsx", true, true},
	{"src/__tests__/setup.ts", true, true},
	{"src/__tests__/utils/render.tsx", true, true},
	{"src/__tests__/_helpers.ts", true, true},
	{"src/__tests__/types.d.ts", true, true},
	{"src/__tests__/TestUtils.tsx", true, true},
	{"src/__tests__/index.js", true, true},
	{"src/__tests__/__mocks__/x.ts", true, true},
	{"examples/demo/src/a.test.tsx", true, true},
	{"docs/a.test.ts", true, true},
	{"dist/a.test.js", true, true},
	{"cypress/e2e/a.spec.ts", true, true},
	{".storybook/a.test.ts", true, true},
	{"__mocks__/x.ts", false, true},
	{"src/test/java/a/FooTest.java", true, true},
	{"src/test/java/a/FooTests.java", true, true},
	{"src/test/java/a/FooIT.java", true, true},
	{"examples/src/test/java/a/FooTest.java", true, true},
	{"src/test/java/a/FooTestCase.java", true, true},
	{"src/test/java/a/AbstractFooTest.java", true, true},
	{"src/test/java/a/TestUtils.java", true, true},
	{"src/main/java/a/FooTest.java", true, true},
	{"src/test/java/a/Helper.java", false, true},
	{"tests/Unit/FooTest.php", true, true},
	{"tests/AbstractFooTest.php", true, true},
	{"src/Foo/CreatesMatchingTest.php", true, true},
	{"examples/tests/FooTest.php", true, true},
	{"Tests/Feature/routes.php", false, true},
	{"src/Foo.php", false, false},
	{"spec/models/user_spec.rb", true, true},
	{"Tests/FooTests.cs", true, true},
	{"crate/tests/it.rs", true, true},
}

// TestDelegatesAnswerAsBeforeTheCut (RF-2, RF-M8 CUT): IsTest,
// IsTestMaterial and IsTestEntry answer every pinned path as they did
// before RF-1's classifier was split and then deleted.
func TestDelegatesAnswerAsBeforeTheCut(t *testing.T) {
	for _, c := range delegateCases {
		if IsTest(c.path) != c.isTest || IsTestMaterial(c.path) != c.isMat {
			t.Errorf("%q: IsTest %v IsTestMaterial %v, want %v %v", c.path, IsTest(c.path), IsTestMaterial(c.path), c.isTest, c.isMat)
		}
		if IsTestEntry("TestX", c.path, "") != c.isTest {
			t.Errorf("IsTestEntry(TestX, %q) = %v, want %v", c.path, !c.isTest, c.isTest)
		}
	}
}
