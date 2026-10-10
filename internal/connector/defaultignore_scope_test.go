package connector

// defaultignore_scope_test.go — SEC-2 (CF-17): `[connector] include` is a
// repo-path field. An entry that leaves the corpus (absolute, a .. escape,
// or a symlink out) is refused at the use site; the rest still re-include.

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestLoadConnectorConfig_RefusesIncludeEntriesOutsideTheCorpus(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linkout")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "[connector]\ninclude = [\"coverage\", \"compute/.tools\", " +
		"\"" + filepath.Join(outside, ".cache") + "\", \"../sibling/.cache\", \"linkout/.cache\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".fathom", "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	include, _ := loadConnectorConfig(root)
	var got []string
	for k := range include {
		got = append(got, k)
	}
	sort.Strings(got)
	if want := []string{"compute/.tools", "coverage"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("include = %v, want %v (absolute, .. and symlinked-out entries refused)", got, want)
	}
}
