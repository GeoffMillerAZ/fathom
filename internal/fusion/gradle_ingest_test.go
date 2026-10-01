package fusion_test

// gradle_ingest_test.go — LB-Q-gradle (LB-F94): a Gradle build's scripts reach the gradle dialect
// through discovery and dispatch (they were "no extractor registered"), what the dialect extracts
// reaches the store, and the project paths a settings include and a project dependency name become
// file-resolved edges to the named project's build script under the rule the file resolver applies
// to every whole-module import: a call through the import resolves to the target's one top-level
// symbol, so a build script that declares more than one resolves no edge (the ceiling
// LB-Q-gradle-rest records). A Groovy source stays disclosed as unsupported (LB-Q-groovy-rest) and
// a Kotlin DSL script stays with the kotlin dialect.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_GradleBuildReachesTheGradleDialect(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"settings.gradle": "rootProject.name = 'demo'\ninclude ':app', ':lib', ':util'\n",
		"app/build.gradle": `plugins {
    id 'application'
}

dependencies {
    implementation project(':lib')
    implementation(project(':util'))
    implementation 'org.slf4j:slf4j-api:2.0.9'
}

tasks.register('hello') {
    dependsOn 'compileJava'
}
`,
		// One top-level symbol: the file resolver binds a call through an
		// import of this script to it.
		"lib/build.gradle": "plugins {\n    id 'java-library'\n}\n",
		// Two: an import of this script binds no call (the rule this test pins).
		"util/build.gradle":          "plugins {\n    id 'java'\n}\n\ntasks.register('extra')\n",
		"src/main/groovy/Foo.groovy": "class Foo {\n    def bar() { 1 }\n}\n",
		"tools/build.gradle.kts":     "val toolVersion = \"1.0\"\n",
	}
	for rel, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if lang, ok := connector.UnsupportedLanguageForPath("src/main/groovy/Foo.groovy"); !ok || lang != "groovy" {
		t.Errorf("a .groovy file is not disclosed as unsupported groovy (got %q, %v): LB-Q-groovy is refused", lang, ok)
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "gradle-svc", Path: dir}
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl.Close)
	ctx := context.Background()
	if _, err := pl.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.SourceKind{}
	for _, o := range obs {
		got[o.SourceRef+"#"+o.Symbol] = o.SourceKind
	}
	for key, kind := range map[string]core.SourceKind{
		"settings.gradle#:app":                       "GradleInclude",
		"settings.gradle#:lib":                       "GradleInclude",
		"settings.gradle#:util":                      "GradleInclude",
		"app/build.gradle#application":               "GradlePlugin",
		"app/build.gradle#:lib":                      "GradleDependency",
		"app/build.gradle#:util":                     "GradleDependency",
		"app/build.gradle#org.slf4j:slf4j-api:2.0.9": "GradleDependency",
		"app/build.gradle#hello":                     "GradleTask",
		"lib/build.gradle#java-library":              "GradlePlugin",
		"util/build.gradle#java":                     "GradlePlugin",
		"util/build.gradle#extra":                    "GradleTask",
		"tools/build.gradle.kts#toolVersion":         "KotlinProperty",
	} {
		if got[key] != kind {
			t.Errorf("%s: kind %q, want %q", key, got[key], kind)
		}
	}
	for key := range got {
		if strings.HasPrefix(key, "src/main/groovy/") {
			t.Errorf("a Groovy source has no dialect (LB-Q-groovy is refused), but produced %s", key)
		}
	}

	edges, err := st.TypedEdges(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	var resolved []string
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved {
			continue
		}
		resolved = append(resolved, e.FromFile+" -> "+e.ToSymbol)
	}
	sort.Strings(resolved)
	toLib := map[string]bool{}
	for _, r := range resolved {
		from, to, _ := strings.Cut(r, " -> ")
		// A symbol key is service::path-without-extension.name.
		if strings.HasSuffix(to, "::lib/build.java-library") {
			toLib[from] = true
		}
		if strings.Contains(to, "::util/build.") {
			t.Errorf("an edge resolved into util/build.gradle, which declares two top-level symbols: %s", r)
		}
	}
	for _, from := range []string{"settings.gradle", "app/build.gradle"} {
		if !toLib[from] {
			t.Errorf("no file-resolved edge from %s to lib/build.gradle's java-library plugin (its one top-level symbol); file-resolved edges %v", from, resolved)
		}
	}
}
