package extractor

// gradledialect_test.go — LB-Q-gradle (design §13.2 steps 10 and 12). The per-fixture goldens run
// in the generic TestDialectGoldens (dialects/gradle/testdata); this file holds what a golden
// cannot express: the parser survives a syntax error, the shapes that look like a declaration
// but are not one, and the files the manifest claims.

import (
	"os"
	"slices"
	"sort"
	"testing"
)

// TestGradleExtractor_SurvivesSyntaxError: a comma before a trailing closure (`api
// project(':x'), { }`, an ERROR node) loses neither the entry before it nor the ones after.
func TestGradleExtractor_SurvivesSyntaxError(t *testing.T) {
	e, err := NewTSExtractor("gradle")
	if err != nil {
		t.Fatalf("NewTSExtractor(gradle): %v", err)
	}
	defer e.Close()
	src, err := os.ReadFile("dialects/gradle/testdata/broken.gradle")
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := e.Extract("svc", "build.gradle", src)
	if err != nil {
		t.Fatalf("Extract must not error on a recoverable syntax error: %v", err)
	}
	var names []string
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	if want := []string{"java", ":first", "org.example:after-error:1.0", "afterLambda"}; !slices.Equal(names, want) {
		t.Fatalf("symbols around the error = %v, want %v", names, want)
	}
}

// TestGradleDialect_NotDefinitions pins what reads like a declaration and is not one: a version
// declared in pluginManagement, a copy spec's include pattern, a task configured by tasks.named,
// a dependency constraint, a catalog accessor, the map notation, a plugin applied by class, and
// a string with no ':' in a dependencies block.
func TestGradleDialect_NotDefinitions(t *testing.T) {
	e, err := NewTSExtractor("gradle")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	src := `pluginManagement {
    plugins {
        id 'org.example.managed' version '1.0'
    }
}
tasks.register('docs', Copy) {
    from('src') {
        include '**/*.md'
    }
}
tasks.named('docs') {
    dependsOn 'other'
}
dependencies {
    constraints {
        implementation 'org.example:constrained:1.0'
    }
    implementation libs.guava
    runtimeOnly group: 'org.example', name: 'runtime', version: '1.0'
    add 'implementation', 'org.example:added:1.0'
}
apply plugin: org.example.CustomPlugin
`
	nodes, err := e.Extract("svc", "build.gradle", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range nodes {
		got = append(got, string(n.Kind)+" "+n.Name)
	}
	sort.Strings(got)
	want := []string{"GradleDependency org.example:added:1.0", "GradleTask docs"}
	if !slices.Equal(got, want) {
		t.Fatalf("definitions = %v, want %v", got, want)
	}
	for _, n := range nodes {
		if n.Name == "docs" && len(n.Calls) != 0 {
			t.Errorf("docs.Calls = %v: tasks.named('docs') { dependsOn } is outside the declaration", n.Calls)
		}
	}
}

// TestGradleDialect_ClaimsItsFiles: every *.gradle script (build.gradle, settings.gradle, a script
// plugin) is the gradle dialect's; .kts stays Kotlin's, so a Kotlin DSL script is read by the
// kotlin dialect (LB-Q-gradle-rest).
func TestGradleDialect_ClaimsItsFiles(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	owner := map[string]string{}
	for _, d := range ds {
		for _, x := range d.Match.Extensions {
			owner[x] = d.ID
		}
		for _, b := range d.Match.Basenames {
			owner[b] = d.ID
		}
	}
	if owner[".gradle"] != "gradle" {
		t.Errorf(".gradle is claimed by %q, want gradle", owner[".gradle"])
	}
	if owner[".kts"] != "kotlin" {
		t.Errorf(".kts is claimed by %q, want kotlin", owner[".kts"])
	}
	for _, b := range []string{"build.gradle.kts", "settings.gradle.kts"} {
		if o, ok := owner[b]; ok {
			t.Errorf("%s is claimed by %s; no Kotlin DSL dialect ships (LB-Q-gradle-rest)", b, o)
		}
	}
}
