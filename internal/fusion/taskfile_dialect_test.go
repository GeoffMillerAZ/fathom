package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

var taskfileBasenames = []string{
	"Taskfile.yml", "Taskfile.yaml", "taskfile.yml", "taskfile.yaml",
	"Taskfile.dist.yml", "Taskfile.dist.yaml", "taskfile.dist.yml", "taskfile.dist.yaml",
}

// TestTaskfileClassification (LB-Q-taskfile, design §5): the manifest claims a
// Taskfile by any of go-task's eight basenames, at any depth. An included
// Taskfile under another name, a Taskfile with no extension, a backup and a
// YAML file that only lives beside one are not claimed.
func TestTaskfileClassification(t *testing.T) {
	rules := dialectRules()
	for _, name := range taskfileBasenames {
		for _, p := range []string{name, "deploy/" + name, "a/b/c/" + name} {
			v := connector.ClassifyDialect(rules, p, nil)
			if v.Dialect != "taskfile" || v.By != connector.ClassifiedByBasename {
				t.Errorf("ClassifyDialect(%q) = {%q %q}, want taskfile by basename", p, v.Dialect, v.By)
			}
			if lang := connector.LanguageForPath(p); lang != "taskfile" {
				t.Errorf("LanguageForPath(%q) = %q, want taskfile", p, lang)
			}
		}
	}
	for _, p := range []string{"Taskfile", "Taskfile.docker.yml", "tasks/build.yml", "Taskfile.yml.bak", "Taskfiles.yml", "taskfile.json"} {
		if v := connector.ClassifyDialect(rules, p, nil); v.Dialect == "taskfile" {
			t.Errorf("ClassifyDialect(%q) = taskfile; only the eight basenames are a Taskfile", p)
		}
	}
	if got := connector.TypedEdgeProducer("taskfile"); got != "syntax" {
		t.Errorf("TypedEdgeProducer(taskfile) = %q, want syntax", got)
	}
}

// TestResolveDocXrefs_Taskfile pins LB-Q-taskfile's L3: an include that names a
// Taskfile, or the directory that holds one, resolves entity to entity (the
// included file's Taskfile entity), whether it is written as a string or under
// `taskfile:`; a remote include, one written with a template, one that names
// nothing the repository holds and one that names its own file resolve to
// nothing.
func TestResolveDocXrefs_Taskfile(t *testing.T) {
	files := docFiles(t, "taskfile", map[string]string{
		"Taskfile.yml": "version: '3'\nincludes:\n  docs: ./docs\n  api:\n    taskfile: ./api/Taskfile.yaml\n    dir: ./api\n" +
			"  remote:\n    taskfile: https://example.com/Taskfile.yml\n  env:\n    taskfile: ./{{.ENV}}/Taskfile.yml\n  gone: ./gone\n  self:\n    taskfile: ./Taskfile.yml\n  both: ./both\n" +
			"tasks:\n  default:\n    cmds:\n      - task: docs:build\n",
		"docs/Taskfile.yml":        "version: '3'\ntasks:\n  build:\n    cmds: [x]\n",
		"api/Taskfile.yaml":        "version: '3'\nincludes:\n  docs: ../docs\ntasks:\n  serve:\n    cmds: [x]\n",
		"both/Taskfile.yml":        "version: '3'\n",
		"both/Taskfile.dist.yml":   "version: '3'\n",
		"unrelated/Taskfile.yml":   "version: '3'\n",
		"api/deep/Taskfile.yml":    "version: '3'\nincludes:\n  up: ../../Taskfile.yml\n",
		"api/deep2/Taskfile.yml":   "version: '3'\nincludes:\n  up: ../..\n",
		"other/taskfile.dist.yaml": "version: '3'\nincludes:\n  api: ../api\n",
	})
	got := xrefBrief(ResolveDocXrefs(files))
	want := []string{
		"Taskfile.yml:Taskfile.yml -includes-> api/Taskfile.yaml:Taskfile.yaml",
		"Taskfile.yml:Taskfile.yml -includes-> docs/Taskfile.yml:Taskfile.yml",
		"api/Taskfile.yaml:Taskfile.yaml -includes-> docs/Taskfile.yml:Taskfile.yml",
		"api/deep/Taskfile.yml:Taskfile.yml -includes-> Taskfile.yml:Taskfile.yml",
		"other/taskfile.dist.yaml:taskfile.dist.yaml -includes-> api/Taskfile.yaml:Taskfile.yaml",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	// ./both holds two Taskfiles, which design section 4 does not resolve a
	// directory to, and api/deep2's ../.. is the repository root as a directory
	// ("."), a path the resolver does not name a file for
}

// TestTaskfileDirectoryNeedsTheOneFileRule pins what fusion.ResolveDocXrefs does
// with an include of a directory that holds a Taskfile and any other file a doc
// dialect reads (a package.json, a pom.xml, a Cargo.toml): a directory resolves
// to the one doc file in it (design §4), so it names no file. go-task's own
// website/ (a Taskfile.yml beside a package.json, the include `taskfile:
// ./website`) is the case. An include that names the Taskfile, `./website/
// Taskfile.yml`, resolves either way. LB-Q-taskfile-rest names the trigger:
// flip this assertion when the resolver prefers the file the referring dialect
// owns.
func TestTaskfileDirectoryNeedsTheOneFileRule(t *testing.T) {
	tf := docFiles(t, "taskfile", map[string]string{
		"Taskfile.yml":         "version: '3'\nincludes:\n  dir: ./website\n  file:\n    taskfile: ./other/Taskfile.yml\n",
		"website/Taskfile.yml": "version: '3'\n",
		"other/Taskfile.yml":   "version: '3'\n",
	})
	if got := xrefBrief(ResolveDocXrefs(tf)); len(got) != 2 {
		t.Fatalf("with only Taskfiles in the set both includes resolve: got %q", got)
	}
	pkg := docFiles(t, "package-json", map[string]string{
		"website/package.json": `{"name":"website","version":"1.0.0"}`,
		"other/package.json":   `{"name":"other","version":"1.0.0"}`,
	})
	want := []string{"Taskfile.yml:Taskfile.yml -includes-> other/Taskfile.yml:Taskfile.yml"}
	if got := xrefBrief(ResolveDocXrefs(append(tf, pkg...))); !reflect.DeepEqual(got, want) {
		t.Fatalf("a directory beside a package.json resolves to nothing (design §4's one-file rule); the file form still resolves\n got: %q\nwant: %q", got, want)
	}
}

func taskfileRepoCorpus() map[string]string {
	return map[string]string{
		"Taskfile.yml": "version: '3'\nincludes:\n  api: ./api\n  docs:\n    taskfile: ./docs/Taskfile.yaml\n    optional: true\n  ghost: ./ghost\n  remote:\n    taskfile: https://example.com/Taskfile.yml\n" +
			"tasks:\n  default:\n    desc: Build everything\n    deps: [build]\n    cmds:\n      - task: api:serve\n  build:\n    cmds:\n      - go build ./...\n  internal:hidden:\n    internal: true\n    cmds:\n      - echo\n",
		"api/Taskfile.yml":        "version: '3'\ntasks:\n  serve:\n    deps: [gen]\n    cmds:\n      - go run .\n  gen:\n    cmds:\n      - go generate ./...\n",
		"docs/Taskfile.yaml":      "version: '3'\ntasks:\n  build:\n    desc: Build the docs\n    cmds:\n      - mkdocs build\n",
		"tools/taskfile.dist.yml": "version: '3'\nincludes:\n  root: ../Taskfile.yml\n",
		"Taskfile":                "version: '3'\ntasks:\n  not-read:\n    cmds: [x]\n",
		"Taskfile.docker.yml":     "version: '3'\ntasks:\n  not-read:\n    cmds: [x]\n",
		"Taskfile.yml.bak":        "version: '3'\ntasks:\n  not-read:\n    cmds: [x]\n",
	}
}

// TestIngest_TaskfileRepo (the product-reach proof): an ingest extracts every
// Taskfile of go-task's eight basenames through the taskfile dialect, leaves an
// extensionless Taskfile, a backup and an included file under another name
// alone, and stores the includes that resolve, a Taskfile or the directory of
// one, as file-tier typed edges from the including Taskfile to the included
// one. Tasks ride with their names as written and their deps and task calls on
// Calls.
func TestIngest_TaskfileRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, taskfileRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "tf", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "tf", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	calls := map[string][]string{}
	for _, o := range obs {
		switch o.SourceKind {
		case core.KindTaskfile, "TaskfileTask":
			got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = true
			calls[o.SourceRef+"|"+o.Symbol] = o.Calls
		}
		switch o.SourceRef {
		case "Taskfile", "Taskfile.docker.yml", "Taskfile.yml.bak":
			t.Errorf("%s|%s: a file that is not one of the eight basenames was extracted as %s", o.SourceRef, o.Symbol, o.SourceKind)
		}
	}
	for _, w := range []string{
		"Taskfile.yml|Taskfile|Taskfile.yml",
		"Taskfile.yml|TaskfileTask|default",
		"Taskfile.yml|TaskfileTask|build",
		"Taskfile.yml|TaskfileTask|internal:hidden",
		"api/Taskfile.yml|Taskfile|Taskfile.yml",
		"api/Taskfile.yml|TaskfileTask|serve",
		"api/Taskfile.yml|TaskfileTask|gen",
		"docs/Taskfile.yaml|Taskfile|Taskfile.yaml",
		"docs/Taskfile.yaml|TaskfileTask|build",
		"tools/taskfile.dist.yml|Taskfile|taskfile.dist.yml",
	} {
		if !got[w] {
			t.Errorf("missing observation %s\n have: %v", w, taskfileSortedKeys(got))
		}
	}
	if want := []string{"api:serve", "build"}; !reflect.DeepEqual(calls["Taskfile.yml|default"], want) {
		t.Errorf("default calls = %v, want %v", calls["Taskfile.yml|default"], want)
	}
	if want := []string{"gen"}; !reflect.DeepEqual(calls["api/Taskfile.yml|serve"], want) {
		t.Errorf("serve calls = %v, want %v", calls["api/Taskfile.yml|serve"], want)
	}
	all, _ := st.TypedEdges(ctx, "tf")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	wantEdges := []string{
		"Taskfile.yml->tf::api/Taskfile.Taskfile.yml",
		"Taskfile.yml->tf::docs/Taskfile.Taskfile.yaml",
		"tools/taskfile.dist.yml->tf::Taskfile.Taskfile.yml",
	}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", edges, wantEdges)
	}
	if rep.DocXrefEdges != 3 {
		t.Errorf("DocXrefEdges = %d, want 3", rep.DocXrefEdges)
	}
}

// TestReingestFile_TaskfileRepo: the one-file refresh reads a Taskfile through the
// same dialect an ingest does (the basename, not the .yml extension's
// classifier, decides), so an edit that adds a task shows as a TaskfileTask.
func TestReingestFile_TaskfileRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, taskfileRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "tf", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := "api/Taskfile.yml"
	edited := "version: '3'\ntasks:\n  serve:\n    cmds: [x]\n  lint:\n    cmds: [y]\n"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "tf", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "tf", "")
	if err != nil {
		t.Fatal(err)
	}
	var tasks []string
	for _, o := range obs {
		if o.SourceRef == rel && o.SourceKind == "TaskfileTask" {
			tasks = append(tasks, o.Symbol)
		}
	}
	sort.Strings(tasks)
	if want := []string{"lint", "serve"}; !reflect.DeepEqual(tasks, want) {
		t.Fatalf("tasks of %s after ReingestFile = %v, want %v", rel, tasks, want)
	}
}

// TestTaskfileDialectIsWhatFusionDispatches: every one of the eight basenames
// reaches the doc-engine dialect (it carries a DocResult) through fusion.New's
// dispatch, in any directory.
func TestTaskfileDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	for _, name := range taskfileBasenames {
		for _, rel := range []string{name, "ops/" + name} {
			e, _, ok, unclassified := p.resolveExtractor(rel, []byte("version: '3'\n"), buildSiblingIndex(nil))
			if !ok || unclassified {
				t.Fatalf("%s: ok=%v unclassified=%v; want an extractor", rel, ok, unclassified)
			}
			if _, isDoc := e.(extractor.DocResulter); !isDoc {
				t.Errorf("%s dispatches to %T, want the doc-engine dialect", rel, e)
			}
		}
	}
	if _, _, ok, unclassified := p.resolveExtractor("Taskfile.docker.yml", []byte("version: '3'\n"), buildSiblingIndex(nil)); ok || !unclassified {
		t.Errorf("Taskfile.docker.yml: ok=%v unclassified=%v; an included file under another name stays unclassified", ok, unclassified)
	}
}

func taskfileSortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
