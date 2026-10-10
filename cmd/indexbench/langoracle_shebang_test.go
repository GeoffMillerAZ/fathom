package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestOwnedSourceRefsByShebang (D4, O756; LB-F23: the harness measures what
// ingest dispatches): a script with no extension is measured as the dialect
// ingest's shebang rule reads it with (fusion.Pipeline.ShebangDialect), so
// bats-core's bin/bats and libexec/bats-core/* count for bash; a perl script,
// a text file with no shebang and a file whose extension no dialect claims do
// not, and python owns its extensionless script, not bash's.
func TestOwnedSourceRefsByShebang(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) core.SourceRef {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return core.SourceRef{Path: rel}
	}
	refs := []core.SourceRef{
		write("lib/bats-core/common.bash", "bats_x() {\n  :\n}\n"),
		write("bin/bats", "#!/usr/bin/env bash\nmain() {\n  :\n}\n"),
		write("libexec/bats-core/bats-exec-file", "#!/bin/sh\nrun() {\n  :\n}\n"),
		write("test/fixture.pl", "#!/usr/bin/env bash\nnot_bash() { :; }\n"),
		write("contrib/legacy", "#!/usr/bin/perl\nsub legacy { 1 }\n"),
		write("contrib/report", "#!/usr/bin/env python3\ndef report():\n    pass\n"),
		write("README", "main() {\n  :\n}\n"),
	}
	pipe := fusion.New(nil, nil, fusion.Config{})
	defer pipe.Close()
	content := contentDialectOf(pipe, refs)
	owned := func(m langbench.Manifest) string {
		var got []string
		for _, r := range ownedSourceRefs(m, root, refs, nil, content) {
			got = append(got, r.Path)
		}
		return strings.Join(got, ",")
	}
	bash := langbench.Manifest{ID: "bash", Engine: "tags", Match: langbench.ManifestMatch{Extensions: []string{".sh", ".bash"}}}
	if got, want := owned(bash), "lib/bats-core/common.bash,bin/bats,libexec/bats-core/bats-exec-file"; got != want {
		t.Errorf("bash owns [%s], want [%s]", got, want)
	}
	python := langbench.Manifest{ID: "python", Engine: "tags", Match: langbench.ManifestMatch{Extensions: []string{".py"}}}
	if got, want := owned(python), "contrib/report"; got != want {
		t.Errorf("python owns [%s], want [%s]", got, want)
	}
	// -only still restricts an extensionless script.
	var got []string
	for _, r := range ownedSourceRefs(bash, root, refs, []string{"lib/**"}, content) {
		got = append(got, r.Path)
	}
	if strings.Join(got, ",") != "lib/bats-core/common.bash" {
		t.Errorf("bash owns %v under -only lib/**, want [lib/bats-core/common.bash]", got)
	}
}
