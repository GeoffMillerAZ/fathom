package main

import (
	"os"
	"reflect"
	"testing"
)

const fixtures = "../../../internal/extractor/dialects/gradle/testdata/"

func names(es []entity) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.name)
	}
	return out
}

func lines(es []entity) []int {
	var out []int
	for _, e := range es {
		out = append(out, e.line)
	}
	return out
}

func read(t *testing.T, f string) string {
	t.Helper()
	b, err := os.ReadFile(fixtures + f)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The line scan is independent of tree-sitter; on the dialect's own fixtures it finds the
// entities the hand-written goldens list (before Gradle attests any of them).
func TestScanSettings(t *testing.T) {
	s := scanScript(read(t, "settings.gradle"), true)
	if got, want := names(s.includes), []string{":app", ":lib", "core:util", "tools:cli", ":docs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("includes = %v, want %v", got, want)
	}
	if got, want := lines(s.includes), []int{17, 17, 18, 19, 22}; !reflect.DeepEqual(got, want) {
		t.Errorf("include lines = %v, want %v", got, want)
	}
	if got, want := names(s.plugins), []string{"org.example.settings-conventions"}; !reflect.DeepEqual(got, want) {
		t.Errorf("plugins = %v, want %v (pluginManagement's are version declarations)", got, want)
	}
}

func TestScanBuildScript(t *testing.T) {
	s := scanScript(read(t, "app.gradle"), false)
	if got, want := names(s.plugins), []string{"java-library", "org.example.conventions", "org.example.quality", "maven-publish", "signing"}; !reflect.DeepEqual(got, want) {
		t.Errorf("plugins = %v, want %v", got, want)
	}
	if !s.plugins[1].applyFalse || s.plugins[0].applyFalse {
		t.Errorf("apply false = %v", s.plugins)
	}
	var got []string
	for _, e := range s.deps {
		got = append(got, e.config+" "+e.name)
	}
	want := []string{
		"buildscript:classpath org.example:build-helper:2.1",
		"api :lib",
		"implementation :core:util",
		"implementation org.slf4j:slf4j-api:2.0.9",
		"implementation com.google.guava:guava:33.0-jre",
		"testImplementation junit:junit:${junitVersion}",
		"implementation :platform",
		"api org.example:bom:1.0",
		"testImplementation :lib",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dependencies = %v\nwant %v", got, want)
	}
}

func TestScanTasks(t *testing.T) {
	s := scanScript(read(t, "tasks.gradle"), false)
	if got, want := names(s.tasks), []string{"hello", "copyDocs", "quoted", "bare", "generate", "report", "packageAll", "verify"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %v, want %v", got, want)
	}
	r := scanScript(read(t, "root.gradle"), false)
	if got, want := names(r.plugins), []string{"java", "scala"}; !reflect.DeepEqual(got, want) {
		t.Errorf("root plugins = %v, want %v", got, want)
	}
	if got, want := names(r.deps), []string{"org.junit.jupiter:junit-jupiter:5.10.0", ":clients", "org.lz4:lz4-java:1.8.0"}; !reflect.DeepEqual(got, want) {
		t.Errorf("root dependencies = %v, want %v", got, want)
	}
	if got, want := names(r.tasks), []string{"coreJar"}; !reflect.DeepEqual(got, want) {
		t.Errorf("root tasks = %v, want %v", got, want)
	}
}

func TestCommentsAndStringsDoNotCount(t *testing.T) {
	src := "// task commented\n/* dependencies {\n implementation 'a:b:1' */\ndependencies {\n    implementation \"x:y:1\" // trailing\n    constraints {\n        implementation 'c:d:1'\n    }\n}\ndef s = '{'\ntask after\n"
	s := scanScript(src, false)
	if got, want := names(s.deps), []string{"x:y:1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("deps = %v, want %v", got, want)
	}
	if got, want := names(s.tasks), []string{"after"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %v, want %v", got, want)
	}
}

func TestAbsPath(t *testing.T) {
	for _, c := range []struct{ written, from, want string }{
		{":a:b", ":x", ":a:b"}, {"a:b", ":", ":a:b"}, {"sub", ":x", ":x:sub"},
	} {
		if got := absPath(c.written, c.from); got != c.want {
			t.Errorf("absPath(%q, %q) = %q, want %q", c.written, c.from, got, c.want)
		}
	}
}
