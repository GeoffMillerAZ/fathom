package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// jenkinsEdgesFrom maps each typed doc-xref edge's origin symbol to its
// targets, for the edges leaving the given file.
func jenkinsEdgesFrom(t *testing.T, st store.Store, svc, file string) map[string][]string {
	t.Helper()
	all, err := st.TypedEdges(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved && e.FromFile == file {
			out[e.FromSymbol] = append(out[e.FromSymbol], e.ToSymbol)
		}
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// TestIngest_JenkinsfileRunSteps (LB-F374): a literal sh, bat or powershell
// script's first commands reach a Makefile target, a Taskfile task, a
// package.json script and a script file's main through the GHA run-step token
// rule (ghaRunRefs), from the definition that encloses the step, under
// dir()'s working directory; a GString's ${...} word and a dir("${X}") resolve
// to nothing, nor does a target nothing defines, and a shared library's step
// runs in its caller's workspace, so its shell steps join nothing in the
// library's repository.
func TestIngest_JenkinsfileRunSteps(t *testing.T) {
	_, st, _ := jenkinsIngest(t)
	edges := jenkinsEdgesFrom(t, st, "jk", "Jenkinsfile")
	build := edges["jk::Jenkinsfile.Build.steps"]
	for _, want := range []string{"jk::Makefile.test", "jk::Taskfile.lint", "jk::package.build", "jk::scripts/ci.main", "jk::web/package.bundle"} {
		if !jenkinsHas(build, want) {
			t.Errorf("Build.steps lacks an edge to %s; edges from the Jenkinsfile: %v", want, edges)
		}
	}
	for _, to := range build {
		if strings.Contains(to, "ghost") || strings.Contains(to, "Makefile.dist") {
			t.Errorf("Build.steps reaches %s, which no literal word of its steps names", to)
		}
	}
	if got := edges["jk::Jenkinsfile.Package.steps"]; !reflect.DeepEqual(got, []string{"jk::Makefile.dist"}) {
		t.Errorf("Package.steps (powershell) edges = %v, want [jk::Makefile.dist]", got)
	}
	if got := jenkinsEdgesFrom(t, st, "jk", "ci/release.jenkinsfile")["jk::ci/release.Release.steps"]; !reflect.DeepEqual(got, []string{"jk::Makefile.dist"}) {
		t.Errorf("a .jenkinsfile's bat step edges = %v, want [jk::Makefile.dist]", got)
	}
	for from, tos := range jenkinsEdgesFrom(t, st, "jk", "vars/buildApp.groovy") {
		for _, to := range tos {
			if strings.HasPrefix(to, "jk::Makefile.") {
				t.Errorf("a library step's shell command joined the library's own Makefile: %s -> %s", from, to)
			}
		}
	}
}

// TestIngest_JenkinsSharedLibraryVars (LB-F375): a step call whose name is a
// vars/<name>.groovy of the indexed repository reaches that variable's call,
// from a Jenkinsfile and from another variable; a call no variable declares
// reaches nothing.
func TestIngest_JenkinsSharedLibraryVars(t *testing.T) {
	_, st, _ := jenkinsIngest(t)
	if got := jenkinsEdgesFrom(t, st, "jk", "Jenkinsfile")["jk::Jenkinsfile.Build.steps"]; !jenkinsHas(got, "jk::vars/buildApp.call") {
		t.Errorf("buildApp(...) does not reach vars/buildApp.groovy's call: %v", got)
	}
	if got := jenkinsEdgesFrom(t, st, "jk", "vars/buildApp.groovy")["jk::vars/buildApp.call"]; !reflect.DeepEqual(got, []string{"jk::vars/deployAll.call"}) {
		t.Errorf("a variable calling another = %v, want [jk::vars/deployAll.call]", got)
	}
	for _, tos := range jenkinsEdgesFrom(t, st, "jk", "Jenkinsfile") {
		for _, to := range tos {
			if strings.Contains(to, "unknownStep") || strings.Contains(to, "rollback") || strings.Contains(to, "Util") {
				t.Errorf("an edge to %s, which no vars/ file's call declares", to)
			}
		}
	}
}

// TestResolveJenkins_BuildJobsUnresolved (LB-F376): `build job:` is recorded
// and, with no in-repository job declaration to join (LB-Q-jcasc deferred),
// counted unresolved by its reason: a literal name undeclared, a GString
// templated.
func TestResolveJenkins_BuildJobsUnresolved(t *testing.T) {
	src := "pipeline {\n  stages {\n    stage('D') {\n      steps {\n        build job: 'deploy-prod'\n        build(job: \"deploy-${ENV}\", wait: false)\n        build 'legacy-job'\n      }\n    }\n  }\n}\n"
	shells, jobs := jenkinsSteps([]byte(src))
	if len(shells) != 0 {
		t.Errorf("shells = %+v, want none", shells)
	}
	var names []string
	for _, j := range jobs {
		names = append(names, j.Name)
	}
	if want := []string{"deploy-prod", "deploy-${ENV}", "legacy-job"}; !reflect.DeepEqual(names, want) {
		t.Errorf("build jobs = %v, want %v", names, want)
	}
	res := resolveJenkins([]jenkinsFile{{Path: "Jenkinsfile", Content: []byte(src)}}, nil, map[string]bool{"Jenkinsfile": true}, nil)
	if want := map[string]int{jenkinsJobUndeclared: 2, jenkinsJobTemplated: 1}; !reflect.DeepEqual(res.Unresolved, want) {
		t.Errorf("unresolved = %v, want %v", res.Unresolved, want)
	}
}

// TestJenkinsSteps_Scanner pins the scanner's reading of the step forms: the
// literal scripts (quoted, triple-quoted, named script:), escapes, a GString's
// ${...} kept as written, dir() nesting, and what is not a step (a receiver
// call, a comment, a variable script).
func TestJenkinsSteps_Scanner(t *testing.T) {
	src := `// sh 'make commented'
/* sh 'make block-commented' */
node {
  sh 'make a'
  sh label: 'two', script: "make b"
  sh(script: '''make c''', returnStdout: true).trim()
  dir('x') {
    bat "make d \"quoted\""
    dir('y/z') { powershell 'make e' }
  }
  sh "make ${T} f"
  steps.sh 'make receiver'
  sh cmd
  def v = "sh 'make in-string'"
  sh "echo \$HOME && make g"
}
`
	shells, _ := jenkinsSteps([]byte(src))
	var got []string
	for _, s := range shells {
		got = append(got, s.Dir+" | "+strings.TrimSpace(s.Script))
	}
	want := []string{
		". | make a",
		". | make b",
		". | make c",
		"x | make d \"quoted\"",
		"x/y/z | make e",
		". | make ${T} f",
		". | echo $HOME && make g",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("shell steps:\n got %q\nwant %q", got, want)
	}
}

// TestReingestFile_JenkinsfileInvalidatesThePass: a one-file refresh of a
// Jenkins file leaves the doc-xref pass to re-run (its state invalidated), so
// the next ingest re-reads its edges instead of keeping stale ones.
func TestReingestFile_JenkinsfileInvalidatesThePass(t *testing.T) {
	p, st, dir := jenkinsIngest(t)
	ctx := context.Background()
	for _, rel := range []string{"Jenkinsfile", "ci/release.jenkinsfile", "vars/buildApp.groovy"} {
		if _, err := p.Ingest(ctx, core.ServiceRef{ID: "jk", Path: dir}); err != nil {
			t.Fatal(err)
		}
		if !p.passStateOK(ctx, "jk", docXrefStateKey) {
			t.Fatalf("%s: the doc-xref pass reads as not done after a clean ingest", rel)
		}
		path := filepath.Join(dir, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, []byte("// edited\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "jk", dir, rel); err != nil {
			t.Fatal(err)
		}
		if p.passStateOK(ctx, "jk", docXrefStateKey) {
			t.Errorf("%s: the doc-xref pass still reads as done after the file changed", rel)
		}
	}
}

// TestIngest_JenkinsfileEditRerunsThePass: an ingest after only the
// Jenkinsfile changed re-runs the doc-xref pass, so a new shell step's edge
// lands.
func TestIngest_JenkinsfileEditRerunsThePass(t *testing.T) {
	p, st, dir := jenkinsIngest(t)
	ctx := context.Background()
	path := filepath.Join(dir, "ci", "release.jenkinsfile")
	src := "pipeline {\n  agent any\n  stages {\n    stage('Release') {\n      steps {\n        sh 'make test'\n      }\n    }\n  }\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "jk", Path: dir}); err != nil {
		t.Fatal(err)
	}
	if got := jenkinsEdgesFrom(t, st, "jk", "ci/release.jenkinsfile")["jk::ci/release.Release.steps"]; !reflect.DeepEqual(got, []string{"jk::Makefile.test"}) {
		t.Errorf("after the edit, Release.steps edges = %v, want [jk::Makefile.test]", got)
	}
}

func jenkinsHas(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
