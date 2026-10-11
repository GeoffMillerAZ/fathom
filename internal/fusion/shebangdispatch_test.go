package fusion

// shebangdispatch_test.go — D4 (O756, K15): a script with no extension is
// read by the dialect whose manifest declares its `#!` interpreter, in ingest
// and in ReingestFile alike; a file with an extension, a text file with no
// shebang, a binary blob and an interpreter no dialect declares stay as they
// were.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// shebangCorpus is a repository of scripts named without an extension, one
// per interpreter a dialect declares (bash, sh, python3, ruby, node) and one
// no dialect declares (perl); a text file with no shebang whose content would
// parse as shell; a binary blob that opens with a shell shebang; and two files
// whose extension decides, whatever their first line says.
func shebangCorpus() map[string]string {
	return map[string]string{
		"bin/tool":         "#!/usr/bin/env bash\nset -e\n\nusage() {\n  echo \"usage: tool\"\n}\n\nusage\n",
		"scripts/posix":    "#!/bin/sh\ncleanup() {\n  rm -rf \"$TMP\"\n}\n",
		"scripts/report":   "#!/usr/bin/env python3\n\ndef render(rows):\n    return len(rows)\n",
		"scripts/release":  "#!/usr/bin/env ruby\n\ndef bump(version)\n  version.succ\nend\n",
		"scripts/serve":    "#!/usr/bin/env node\n\nfunction listen(port) {\n  return port\n}\n",
		"scripts/legacy":   "#!/usr/bin/perl\nsub legacy { return 1; }\n",
		"notes/TODO":       "render() {\n  nothing\n}\n",
		"data/blob":        "#!/bin/sh\n\x00\x01\x02\x00helper() { :; }\n",
		"tools/run.txt":    "#!/usr/bin/env bash\nhidden() {\n  :\n}\n",
		"tools/helper.py":  "#!/usr/bin/env bash\ndef helper():\n    pass\n",
		"tools/zshrc-like": "#!/bin/zsh\nprompt_setup() {\n  :\n}\n",
	}
}

// TestShebangDispatch pins the one rule: sniffedExtractor accepts a shebang
// verdict only for a file with no extension, and dispatches it to the
// extractor ingest already runs for the dialect's first extension.
func TestShebangDispatch(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	corpus := shebangCorpus()
	for rel, want := range map[string]string{
		"bin/tool":        "bash",
		"scripts/posix":   "bash",
		"scripts/report":  "python",
		"scripts/release": "ruby",
		"scripts/serve":   "javascript",
		"tools/helper.py": "python", // the extension decides
	} {
		e, _, ok, _ := p.resolveExtractor(rel, []byte(corpus[rel]), buildSiblingIndex(nil))
		if !ok {
			t.Errorf("%s: no extractor, want the %s dialect", rel, want)
			continue
		}
		if ts, isTS := e.(*extractor.TSExtractor); !isTS || ts.Name() != want {
			t.Errorf("%s dispatches to %T, want the %s tags dialect", rel, e, want)
		}
		// The extractor ingest already runs for the extension: no new instance.
		if rel != "tools/helper.py" {
			if got := p.ShebangDialect(rel, []byte(corpus[rel])); got != want {
				t.Errorf("ShebangDialect(%s) = %q, want %q", rel, got, want)
			}
		}
	}
	if e, _, _, _ := p.resolveExtractor("scripts/report", []byte(corpus["scripts/report"]), nil); e != p.exts[".py"] {
		t.Errorf("scripts/report reads with a new extractor, want p.exts[\".py\"]'s")
	}
	for _, rel := range []string{"scripts/legacy", "notes/TODO", "tools/run.txt", "tools/zshrc-like"} {
		if e, _, ok, _ := p.resolveExtractor(rel, []byte(corpus[rel]), buildSiblingIndex(nil)); ok {
			t.Errorf("%s dispatches to %T, want no extractor", rel, e)
		}
		if got := p.ShebangDialect(rel, []byte(corpus[rel])); got != "" {
			t.Errorf("ShebangDialect(%s) = %q, want none", rel, got)
		}
	}
	if got := p.ShebangDialect("data/blob", []byte(corpus["data/blob"])); got != "" {
		t.Errorf("ShebangDialect(data/blob) = %q, want none for a binary file", got)
	}
}

// TestIngest_ShebangDispatch is the product-reach proof: an ingest stores the
// symbols of every extensionless script whose interpreter a dialect declares,
// and none of the others; ReingestFile decides as ingest does.
func TestIngest_ShebangDispatch(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, shebangCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "sb", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "sb", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = true
		switch o.SourceRef {
		case "scripts/legacy", "notes/TODO", "data/blob", "tools/run.txt", "tools/zshrc-like":
			t.Errorf("%s|%s: a file the rule leaves alone was extracted as %s", o.SourceRef, o.Symbol, o.SourceKind)
		case "tools/helper.py":
			if o.SourceKind == "BashFunction" {
				t.Errorf("tools/helper.py: the shebang overrode the extension (%s)", o.Symbol)
			}
		}
	}
	want := []string{
		"bin/tool|BashFunction|usage",
		"scripts/posix|BashFunction|cleanup",
		"scripts/report|PythonFunc|render",
		"scripts/release|RubyMethod|bump",
		"scripts/serve|JSFunc|listen",
		"tools/helper.py|PythonFunc|helper",
	}
	for _, w := range want {
		if !got[w] {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("%s not stored; have %q", w, have)
		}
	}

	// ReingestFile decides as ingest does (reingest_file.go reaches the same
	// sniffedExtractor): an edit to an extensionless script is re-extracted,
	// and a perl script stays unextracted.
	edits := map[string]string{
		"bin/tool":       "#!/usr/bin/env bash\nhelp_text() {\n  echo \"help\"\n}\n",
		"scripts/legacy": "#!/usr/bin/perl\nsub edited { return 2; }\n",
	}
	for rel, body := range edits {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "sb", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	obs, err = st.QueryObservations(ctx, "sb", "")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, o := range obs {
		seen[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = true
		if o.SourceRef == "scripts/legacy" {
			t.Errorf("after ReingestFile: scripts/legacy extracted as %s %s", o.SourceKind, o.Symbol)
		}
	}
	if !seen["bin/tool|BashFunction|help_text"] {
		t.Error("after ReingestFile: bin/tool's new function missing")
	}
	if seen["bin/tool|BashFunction|usage"] {
		t.Error("after ReingestFile: bin/tool's old function survived")
	}
}

// TestIngest_ShebangFileResolve is O793's product-reach proof: an
// extensionless script the shebang rule dispatches enters the file-resolved
// pass under its dialect's language tag, as an importer (a python3 script's
// `from pkg.util import fmt_row` binds its call to pkg/util.py) and as a
// target (a Ruby file's `load 'bin/helper'` brings the ruby script's top-level
// methods into scope); an edit to the extensionless script alone runs the pass
// again, and a perl script and a file with no shebang stay out of it.
func TestIngest_ShebangFileResolve(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"scripts/report":  "#!/usr/bin/env python3\nfrom pkg.util import fmt_row\n\ndef render(rows):\n    return fmt_row(rows)\n",
		"pkg/__init__.py": "",
		"pkg/util.py":     "def fmt_row(r):\n    return r\n",
		"pkg/extra.py":    "def extra_fn():\n    return 2\n",
		"lib/app.rb":      "load 'bin/helper'\n\ndef run\n  assist(1)\nend\n",
		"bin/helper":      "#!/usr/bin/env ruby\n\ndef assist(x)\n  x\nend\n",
		"scripts/legacy":  "#!/usr/bin/perl\nsub legacy { return 1; }\n",
		"notes/TODO":      "fmt_row() {\n  nothing\n}\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	svc := core.ServiceRef{ID: "sbr", Path: dir}
	edges := func() map[string]bool {
		t.Helper()
		all, err := st.TypedEdges(ctx, svc.ID)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, e := range all {
			if e.Kind == core.EdgeFileResolved {
				out[e.FromFile+"|"+e.FromSymbol+" -> "+e.ToSymbol] = true
			}
		}
		return out
	}
	// A symbol's key is service::<file less its extension>.<name>.
	has := func(got map[string]bool, from, fromSym, toFile, toSym string) bool {
		return got[from+"|sbr::"+strings.TrimSuffix(from, filepath.Ext(from))+"."+fromSym+" -> sbr::"+strings.TrimSuffix(toFile, filepath.Ext(toFile))+"."+toSym]
	}
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	got := edges()
	if !has(got, "scripts/report", "render", "pkg/util.py", "fmt_row") {
		t.Errorf("the python3 script's import bound no edge to pkg/util.py's fmt_row; file-resolved edges %v", got)
	}
	if !has(got, "lib/app.rb", "run", "bin/helper", "assist") {
		t.Errorf("lib/app.rb's load of the ruby script bound no edge to bin/helper's assist; file-resolved edges %v", got)
	}
	for k := range got {
		if strings.Contains(k, "scripts/legacy") || strings.Contains(k, "notes/TODO") {
			t.Errorf("a file the shebang rule leaves alone drew an edge: %s", k)
		}
	}

	// Only the extensionless script changes: the pass runs and its new import binds.
	writeCorpus(t, dir, map[string]string{
		"scripts/report": "#!/usr/bin/env python3\nfrom pkg.util import fmt_row\nfrom pkg.extra import extra_fn\n\ndef render(rows):\n    return fmt_row(rows) + extra_fn()\n",
	})
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.FileResolveSkipped != "" {
		t.Fatalf("an edit to an extensionless python3 script left the file-resolved pass skipped: %s", rep.FileResolveSkipped)
	}
	if got := edges(); !has(got, "scripts/report", "render", "pkg/extra.py", "extra_fn") {
		t.Errorf("after the edit, no edge to pkg/extra.py's extra_fn; file-resolved edges %v", got)
	}
}
