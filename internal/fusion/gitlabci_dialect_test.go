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

// TestGitLabCIClassification (LB-Q-gitlab-ci, design §5): the manifest's
// basenames and globs claim a pipeline's conventional files — the root
// .gitlab-ci.yml, everything under .gitlab/ci/, and any *.gitlab-ci.yml — and
// leave the neighbouring YAML under .gitlab/ and the other CI systems' files
// to the dialects that own them.
func TestGitLabCIClassification(t *testing.T) {
	rules := dialectRules()
	cases := []struct {
		path, want string
		by         string
	}{
		{".gitlab-ci.yml", "gitlab-ci", connector.ClassifiedByBasename},
		{"svc/api/.gitlab-ci.yaml", "gitlab-ci", connector.ClassifiedByBasename},
		{".gitlab/ci/build.yml", "gitlab-ci", connector.ClassifiedByGlob},
		{".gitlab/ci/environments/gke.amd64.gitlab-ci.yml", "gitlab-ci", connector.ClassifiedByGlob},
		{"ci/test.gitlab-ci.yml", "gitlab-ci", connector.ClassifiedByGlob},
		{".gitlab/ci/rules.yaml", "gitlab-ci", connector.ClassifiedByGlob},
		{".gitlab/changelog_config.yml", "kubernetes", connector.ClassifiedByExtension},
		{".gitlab/duo/agent-config.yml", "kubernetes", connector.ClassifiedByExtension},
		{"gitlab-ci.yml", "kubernetes", connector.ClassifiedByExtension},
		{".github/workflows/ci.yml", "github-actions", connector.ClassifiedByGlob},
		{".gitlab-ci.yml.bak", "", connector.ClassifiedNone},
	}
	for _, c := range cases {
		v := connector.ClassifyDialect(rules, c.path, nil)
		if v.Dialect != c.want || v.By != c.by {
			t.Errorf("ClassifyDialect(%q) = {%q %q}, want {%q %q}", c.path, v.Dialect, v.By, c.want, c.by)
		}
	}
}

// TestResolveDocXrefs_GitLabCI pins LB-Q-gitlab-ci's L3: a job's extends,
// needs and dependencies that name an entity declared in exactly one other
// file (service-name) resolve to it, entity to entity; a name declared in two
// other files, a name the repository does not hold (a remote template, a
// component's job) and a name declared only in the referring file resolve to
// nothing. include:local and trigger:include name a file, and a pipeline file
// has no root entity, so they are recorded but never resolve (PROVENANCE.md).
func TestResolveDocXrefs_GitLabCI(t *testing.T) {
	x, err := extractor.NewDocDialectExtractor("gitlab-ci")
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{
		".gitlab-ci.yml": `include:
  - local: /.gitlab/ci/rules.yml
  - local: /.gitlab/ci/build.yml
stages: [build, test]
unit:
  extends: [.rules, .sast]
  needs: [compile, unit-local]
  dependencies: [compile]
  script: [go test]
unit-local:
  script: [go test]
`,
		".gitlab/ci/rules.yml": `.rules:
  rules:
    - if: $CI_COMMIT_BRANCH
.shared:
  script: [echo]
`,
		".gitlab/ci/build.yml": `compile:
  extends: .rules
  stage: build
  script: [go build]
.shared:
  script: [echo again]
`,
		"other/.gitlab-ci.yml": `job:
  extends: .shared
  script: [x]
`,
	}
	var files []DocFile
	for _, rel := range []string{".gitlab-ci.yml", ".gitlab/ci/rules.yml", ".gitlab/ci/build.yml", "other/.gitlab-ci.yml"} {
		r, err := x.ExtractDoc("svc", rel, []byte(sources[rel]))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, DocFile{Path: rel, Result: r})
	}
	got := xrefBrief(ResolveDocXrefs(files))
	want := []string{
		".gitlab-ci.yml:unit -dependencies-> .gitlab/ci/build.yml:compile",
		".gitlab-ci.yml:unit -needs-> .gitlab/ci/build.yml:compile",
		".gitlab-ci.yml:unit -extends-> .gitlab/ci/rules.yml:.rules",
		".gitlab/ci/build.yml:compile -extends-> .gitlab/ci/rules.yml:.rules",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// TestClassify_GitLabCI (LB-Q-gitlab-ci): a pipeline file is recognized by its
// path — the root .gitlab-ci.yml, everything under .gitlab/ci/, any
// *.gitlab-ci.yml — ahead of the content signals; a Kubernetes manifest that
// happens to sit under .gitlab/ci/ stays one, and YAML elsewhere under
// .gitlab/ is not a pipeline. Outside those paths a global `services:` list
// is not Compose either (O324: Compose's services is a mapping), while a
// services mapping is.
func TestClassify_GitLabCI(t *testing.T) {
	jobs := []byte("build:\n  script: [make]\n")
	services := []byte("services:\n  - postgres:16\nbuild:\n  script: [make]\n")
	k8s := []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: x\n")
	cases := []struct {
		path    string
		content []byte
		want    ToolKind
	}{
		{".gitlab-ci.yml", jobs, ToolGitLabCI},
		{"svc/.gitlab-ci.yaml", jobs, ToolGitLabCI},
		{".gitlab-ci.yml", services, ToolGitLabCI},
		{".gitlab/ci/build.yml", jobs, ToolGitLabCI},
		{".gitlab/ci/env/gke.gitlab-ci.yml", services, ToolGitLabCI},
		{"ci/test.gitlab-ci.yml", jobs, ToolGitLabCI},
		{".gitlab/ci/deploy/deployment.yaml", k8s, ToolK8sManifest},
		{".gitlab/changelog_config.yml", []byte("categories:\n  added: Added\n"), ToolUnclassified},
		{"gitlab-ci.yml", jobs, ToolUnclassified},
		{"docs/compose.yml", services, ToolUnclassified},
		{"docs/stack.yml", []byte("services:\n  db:\n    image: postgres:16\n"), ToolCompose},
	}
	for _, c := range cases {
		if got := Classify(c.path, c.content, nil); got != c.want {
			t.Errorf("Classify(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func gitlabCIPipelineCorpus() map[string]string {
	return map[string]string{
		".gitlab-ci.yml": `include:
  - local: /.gitlab/ci/rules.gitlab-ci.yml
stages: [build, test]
services:
  - postgres:16
compile:
  extends: .rules
  stage: build
  script: [make]
unit:
  extends: .rules
  needs: [compile]
  stage: test
  script: [make test]
`,
		".gitlab/ci/rules.gitlab-ci.yml": `.rules:
  rules:
    - if: $CI_COMMIT_BRANCH
`,
		".gitlab/changelog_config.yml": "categories:\n  added: Added\n",
	}
}

// TestIngest_GitLabCIPipeline: an ingest extracts a pipeline's jobs and hidden
// templates through the gitlab-ci dialect (in the root file and in an included
// one), tags them "gitlab-ci", leaves the neighbouring .gitlab/ YAML alone, and
// stores the cross-file references that resolve — a job extending a template
// in another file — as file-tier typed edges. The intra-file references ride
// on Calls.
func TestIngest_GitLabCIPipeline(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, gitlabCIPipelineCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "gl", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "gl", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		if o.ToolKind != "" && o.ToolKind != string(ToolGitLabCI) {
			t.Errorf("%s|%s: ToolKind %q, want %q", o.SourceRef, o.Symbol, o.ToolKind, ToolGitLabCI)
		}
	}
	want := map[string]string{
		".gitlab-ci.yml|compile":                "GitLabCIJob",
		".gitlab-ci.yml|unit":                   "GitLabCIJob",
		".gitlab/ci/rules.gitlab-ci.yml|.rules": "GitLabCITemplate",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("observations\n got: %v\nwant: %v", got, want)
	}
	if rep.DocXrefEdges != 2 {
		t.Errorf("DocXrefEdges = %d, want 2 (compile and unit extend .rules in another file)", rep.DocXrefEdges)
	}
	all, _ := st.TypedEdges(ctx, "gl")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	wantEdges := []string{
		".gitlab-ci.yml|gl::.gitlab-ci.compile->gl::.gitlab/ci/rules.gitlab-ci..rules",
		".gitlab-ci.yml|gl::.gitlab-ci.unit->gl::.gitlab/ci/rules.gitlab-ci..rules",
	}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", edges, wantEdges)
	}
}

// TestReingestFile_GitLabCIIncludedFile: the one-file refresh reads a pipeline
// file through the same dialect an ingest does, including one named only by
// its path (.gitlab/ci/…), not by the generic YAML extractor the .yml
// extension alone would pick.
func TestReingestFile_GitLabCIIncludedFile(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, gitlabCIPipelineCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "gl", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := ".gitlab/ci/rules.gitlab-ci.yml"
	edited := ".rules:\n  rules:\n    - if: $CI_COMMIT_BRANCH\n.fast:\n  extends: .rules\n"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{rel, ".gitlab-ci.yml"} {
		if _, err := p.ReingestFile(ctx, st, "gl", dir, path); err != nil {
			t.Fatal(err)
		}
	}
	obs, err := st.QueryObservations(ctx, "gl", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
	}
	want := map[string]string{
		".gitlab-ci.yml|compile":                "GitLabCIJob",
		".gitlab-ci.yml|unit":                   "GitLabCIJob",
		".gitlab/ci/rules.gitlab-ci.yml|.rules": "GitLabCITemplate",
		".gitlab/ci/rules.gitlab-ci.yml|.fast":  "GitLabCITemplate",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("observations after ReingestFile\n got: %v\nwant: %v", got, want)
	}
}
