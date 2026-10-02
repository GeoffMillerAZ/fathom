package fusion_test

// LB-F197 (O287, O288, O289): Rust's use-path roots, crate roots and
// re-exports, resolved over a workspace fixture: a facade crate re-exporting a
// type from a child module, a binary target named by a [[bin]] path, an
// integration test root, an inline `mod tests` and an external crate.

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

var rustWorkspaceFiles = []string{
	"crates/a/src/lib.rs", "crates/a/src/inner.rs", "crates/a/src/x.rs", "crates/b/src/lib.rs",
	"crates/core/main.rs", "crates/core/flags.rs", "tests/tests.rs", "tests/util.rs",
}

func rustWorkspaceImports(t *testing.T) map[string]string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "rustwsfixture"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "rust", rustWorkspaceFiles)
	ris, report := fusion.ResolveImportSpecs(context.Background(), "rust", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	got := map[string]string{}
	for _, ri := range ris {
		k := fmt.Sprintf("%s:%d:%s<-%s", ri.File, ri.StartLine, ri.Spec.LocalName, ri.Spec.Module)
		v := strings.Join(ri.Targets, ",")
		if ri.External {
			v += "(external)"
		}
		got[k] = v
	}
	return got
}

func TestResolveImportSpecs_RustWorkspace(t *testing.T) {
	got := rustWorkspaceImports(t)
	for k, want := range map[string]string{
		// O287: a one-segment path root is a crate, not a child module.
		"crates/b/src/lib.rs:1:S<-a":         "crates/a/src/lib.rs",
		"crates/a/src/x.rs:1:S<-crate":       "crates/a/src/lib.rs",
		"crates/a/src/lib.rs:1:inner<-inner": "crates/a/src/inner.rs", // `mod inner;` is still a child module
		// O288: a [[bin]] path is a crate root; a tests/*.rs file is its own root.
		"crates/core/main.rs:3:Flag<-crate::flags": "crates/core/flags.rs",
		"tests/tests.rs:1:util<-util":              "tests/util.rs",
		"tests/tests.rs:3:helper<-crate::util":     "tests/util.rs",
		// O288: `use super::*` in an inline mod of the root is the root itself,
		// which is no target of its own file.
		"crates/a/src/lib.rs:10:*<-super": "",
		// O289: a named import follows the facade's `pub use` to the defining file.
		"crates/b/src/lib.rs:2:T<-a": "crates/a/src/inner.rs,crates/a/src/lib.rs",
		// An external crate is out of scope, never unresolved.
		"crates/b/src/lib.rs:3:fmt<-std":         "(external)",
		"crates/b/src/lib.rs:4:Serialize<-serde": "(external)",
	} {
		if got[k] != want {
			t.Errorf("%s => %q, want %q", k, got[k], want)
		}
	}
}
