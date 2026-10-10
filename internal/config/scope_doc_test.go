package config

// scope_doc_test.go — SEC-2 (CF-18): docs/configuration.md's scope table
// carries one row per registry field with its class, so the Scope column
// never drifts from the registry by hand typing.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

const scopeDocHeading = "## Scope: what a repository's config may set"

func TestConfigurationDocScopeColumn(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	doc, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "docs", "configuration.md"))
	if err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	want.WriteString("| Field | Scope | Why |\n|---|---|---|\n")
	for _, fs := range FieldScopes() {
		fmt.Fprintf(&want, "| `%s` | %s | %s |\n", fs.Key, fs.Class, fs.Why)
	}
	text := string(doc)
	i := strings.Index(text, scopeDocHeading)
	if i < 0 {
		t.Fatalf("docs/configuration.md has no %q section; add it with this table:\n%s", scopeDocHeading, want.String())
	}
	section := text[i+len(scopeDocHeading):]
	if j := strings.Index(section, "\n## "); j >= 0 {
		section = section[:j]
	}
	row := regexp.MustCompile("(?m)^\\| `([^`]+)` \\| ([a-z-]+) \\|")
	got := map[string]string{}
	for _, m := range row.FindAllStringSubmatch(section, -1) {
		if _, dup := got[m[1]]; dup {
			t.Errorf("field %s has two rows", m[1])
		}
		got[m[1]] = m[2]
	}
	for _, fs := range FieldScopes() {
		if c, ok := got[fs.Key]; !ok {
			t.Errorf("the scope table has no row for %s (%s)", fs.Key, fs.Class)
		} else if c != string(fs.Class) {
			t.Errorf("the scope table says %s is %s; the registry says %s", fs.Key, c, fs.Class)
		}
		delete(got, fs.Key)
	}
	for k := range got {
		t.Errorf("the scope table has a row for %s, which the registry does not classify", k)
	}
	if t.Failed() {
		t.Logf("the table the registry implies:\n%s", want.String())
	}
}
