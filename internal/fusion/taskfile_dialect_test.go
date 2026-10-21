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

func TestTaskfileUnnamedContentDispatch(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	for _, tc := range []struct {
		path, src string
		want      bool
	}{
		{"taskfiles/envkit.gen.yml", "version: '3'\ntasks:\n  check:\n    cmds: [echo ok]\n", true},
		{"taskfiles/nested.yaml", "version: 3\nincludes:\n  child: ./child.yml\n", true},
		{"taskfiles/quoted.yml", "\"version\": \"3\"\n\"tasks\":\n  check:\n    cmds: [echo ok]\n", true},
		{"taskfiles/spaced.yml", "version : \"3\"\ntasks:\n  check:\n    cmds: [echo ok]\n", true},
		{"config.yml", "version: '3'\nname: ordinary\n", false},
		{"comment.yml", "# version: '3'\ntasks:\n  x: {}\n", false},
		{"scalar.yml", "message: |\n  version: '3'\n  tasks:\n", false},
		{"wrong-version.yml", "version: '2'\ntasks:\n  x: {}\n", false},
		{"bad-tasks.yml", "version: '3'\ntasks: [x]\n", false},
		{"malformed.yml", "version: '3'\ntasks: [\n", false},
		{"duplicate.yml", "version: '3'\nversion: '3'\ntasks:\n  check: {}\n", false},
		{"duplicate-body.yml", "version: '3'\ntasks:\n  check: {}\ntasks:\n  other: {}\n", false},
		{"multi-doc.yml", "version: '3'\ntasks:\n  check: {}\n---\nversion: '3'\ntasks:\n  other: {}\n", false},
		{"k8s.yml", "apiVersion: v1\nkind: ConfigMap\nversion: '3'\ntasks:\n  x: {}\n", false},
		{"compose.yml", "version: '3'\nservices:\n  app: {}\ntasks:\n  x: {}\n", false},
	} {
		e, kind, ok, _ := p.resolveExtractor(tc.path, []byte(tc.src), buildSiblingIndex(nil))
		_, isDoc := e.(extractor.DocResulter)
		got := ok && isDoc && kind == ToolTaskfile
		if got != tc.want {
			t.Errorf("%s: taskfile dispatch = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestIngest_TaskfileUnnamedIncludeAndReingest(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"Taskfile.yml":             "version: '3'\nincludes:\n  env: ./taskfiles/envkit.gen.yml\n  missing: ./taskfiles/gone.yml\ntasks:\n  all:\n    cmds:\n      - task: env:check\n",
		"taskfiles/envkit.gen.yml": "\"version\" : \"3\"\n\"tasks\":\n  check:\n    cmds: [echo ok]\n",
		"taskfiles/ordinary.yml":   "version: '3'\nname: ordinary\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "tf", Path: dir}); err != nil {
		t.Fatal(err)
	}
	check := func(wantTask string, wantEdge bool) {
		t.Helper()
		obs, err := st.QueryObservations(ctx, "tf", "")
		if err != nil {
			t.Fatal(err)
		}
		var tasks []string
		for _, o := range obs {
			if o.SourceRef == "taskfiles/ordinary.yml" && o.SourceKind == core.KindTaskfile {
				t.Error("ordinary YAML became a Taskfile")
			}
			if o.SourceRef == "taskfiles/envkit.gen.yml" && o.SourceKind == "TaskfileTask" {
				tasks = append(tasks, o.Symbol)
			}
		}
		if !reflect.DeepEqual(tasks, []string{wantTask}) {
			t.Errorf("child tasks = %q, want %q", tasks, wantTask)
		}
		all, err := st.TypedEdges(ctx, "tf")
		if err != nil {
			t.Fatal(err)
		}
		var edges []string
		for _, e := range all {
			if e.Kind == core.EdgeFileResolved {
				edges = append(edges, e.FromFile+"->"+e.ToSymbol)
			}
		}
		sort.Strings(edges)
		var want []string
		if wantEdge {
			want = []string{"Taskfile.yml->tf::taskfiles/envkit.gen.envkit.gen.yml"}
		}
		if !reflect.DeepEqual(edges, want) {
			t.Errorf("include edges = %q, want %q", edges, want)
		}
	}
	check("check", true)
	rel := "taskfiles/envkit.gen.yml"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte("version: '3'\ntasks:\n  lint:\n    cmds: [echo ok]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "tf", dir, rel); err != nil {
		t.Fatal(err)
	}
	check("lint", true)
	root := "Taskfile.yml"
	if err := os.WriteFile(filepath.Join(dir, root), []byte("version: '3'\ntasks:\n  all:\n    cmds: [echo ok]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "tf", dir, root); err != nil {
		t.Fatal(err)
	}
	// ReingestFile refreshes symbols only; the next full ingest replaces typed edges.
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "tf", Path: dir}); err != nil {
		t.Fatal(err)
	}
	check("lint", false)
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
// dialect reads (a package.json, a pom.xml, a Cargo.toml): of the doc files in
// the directory, the one the referring dialect owns by its basenames and globs
// is the one it names (O165), so the include resolves whether the set holds
// only Taskfiles (the harness's) or every doc file (an ingest's). go-task's own
// website/ (a Taskfile.yml beside a package.json, the include `taskfile:
// ./website`) is the case. A directory holding two Taskfiles still names none.
func TestTaskfileDirectoryNeedsTheOneFileRule(t *testing.T) {
	tf := docFiles(t, "taskfile", map[string]string{
		"Taskfile.yml":           "version: '3'\nincludes:\n  dir: ./website\n  file:\n    taskfile: ./other/Taskfile.yml\n  two: ./two\n",
		"website/Taskfile.yml":   "version: '3'\n",
		"other/Taskfile.yml":     "version: '3'\n",
		"two/Taskfile.yml":       "version: '3'\n",
		"two/Taskfile.dist.yaml": "version: '3'\n",
	})
	want := []string{
		"Taskfile.yml:Taskfile.yml -includes-> other/Taskfile.yml:Taskfile.yml",
		"Taskfile.yml:Taskfile.yml -includes-> website/Taskfile.yml:Taskfile.yml",
	}
	if got := xrefBrief(ResolveDocXrefs(tf)); !reflect.DeepEqual(got, want) {
		t.Fatalf("with only Taskfiles in the set\n got: %q\nwant: %q", got, want)
	}
	pkg := docFiles(t, "package-json", map[string]string{
		"website/package.json": `{"name":"website","version":"1.0.0"}`,
		"other/package.json":   `{"name":"other","version":"1.0.0"}`,
	})
	if got := xrefBrief(ResolveDocXrefs(append(tf, pkg...))); !reflect.DeepEqual(got, want) {
		t.Fatalf("a directory beside a package.json resolves to its Taskfile (O165)\n got: %q\nwant: %q", got, want)
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
// Taskfile of go-task's eight basenames through the taskfile dialect, also
// reads an arbitrary-name YAML with a version-3 Taskfile shape, and leaves an
// extensionless Taskfile and a backup alone. It stores includes that resolve
// to a file or the directory of one as file-tier typed edges. Tasks retain
// their written names and deps and task calls on Calls.
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
		case "Taskfile", "Taskfile.yml.bak":
			t.Errorf("%s|%s: a non-YAML candidate was extracted as %s", o.SourceRef, o.Symbol, o.SourceKind)
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
		"Taskfile.docker.yml|Taskfile|Taskfile.docker.yml",
		"Taskfile.docker.yml|TaskfileTask|not-read",
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
