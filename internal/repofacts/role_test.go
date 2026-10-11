package repofacts

import (
	"strings"
	"testing"
)

// TestRoleCountsUnknown (RF-1, I65): Role answers one value of the closed
// vocabulary for every path, `unknown` included, and a Tally of an answer's
// files prints the unknown count even when it is zero.
func TestRoleCountsUnknown(t *testing.T) {
	cases := []struct {
		path, dialect string
		want          Fact
	}{
		{"internal/repofacts/role.go", "go", Fact{Source, ByConvention}},
		{"internal/repofacts/role_test.go", "go", Fact{Test, ByConvention}},
		{"web/src/app.test.ts", "ts", Fact{Test, ByConvention}},
		{"pkg/__tests__/app.js", "js", Fact{Test, ByConvention}},
		{"tests/test_api.py", "py", Fact{Test, ByConvention}},
		{"tests/helpers.py", "py", Fact{Fixture, ByConvention}},
		{"conftest.py", "py", Fact{Fixture, ByConvention}},
		{"src/test/java/a/FooTest.java", "java", Fact{Test, ByConvention}},
		{"tests/Unit/FooTest.php", "php", Fact{Test, ByConvention}},
		{"scripts/gate-lease.sh", "sh", Fact{Script, ByConvention}},
		{"bin/deploy", "bash", Fact{Script, ByConvention}},
		{"db/migrations/0001_init.sql", "sql", Fact{Migration, ByConvention}},
		{"schema/plan.proto", "proto", Fact{Schema, ByConvention}},
		{".github/workflows/ci.yml", "github-actions", Fact{CI, ByDialect}},
		{"Taskfile.yml", "taskfile", Fact{Build, ByDialect}},
		{"deploy/main.tf", "terraform", Fact{IaC, ByDialect}},
		{"config/app.toml", "toml", Fact{Config, ByConvention}},
		{"assets/logo.bin", "", Fact{Unknown, ""}},
		{"data/points.csv", "", Fact{Unknown, ""}},
	}
	var tally Tally
	for _, c := range cases {
		got := Role(c.path, c.dialect, Attributes{}, nil)
		if got != c.want {
			t.Errorf("Role(%q, %q) = %v, want %v", c.path, c.dialect, got, c.want)
		}
		if !inVocabulary(got.Role) {
			t.Errorf("Role(%q) = %q, outside the closed vocabulary", c.path, got.Role)
		}
		tally.Add(got)
	}
	if tally.Unknown() != 2 {
		t.Errorf("tally unknown = %d, want 2", tally.Unknown())
	}
	if s := tally.String(); !strings.Contains(s, "unknown 2") {
		t.Errorf("tally %q does not print the unknown count", s)
	}
	var empty Tally
	empty.Add(Fact{Source, ByConvention})
	if s := empty.String(); !strings.Contains(s, "unknown 0") {
		t.Errorf("a tally with no unknown file prints %q, want the zero count printed (I65)", s)
	}
	if len(Vocabulary()) != 14 || Vocabulary()[len(Vocabulary())-1] != Unknown {
		t.Errorf("vocabulary = %v, want the 14 roles ending in unknown", Vocabulary())
	}
}

func inVocabulary(r RoleName) bool {
	for _, v := range Vocabulary() {
		if v == r {
			return true
		}
	}
	return false
}
