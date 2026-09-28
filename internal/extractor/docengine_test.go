package extractor

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func mustDoc(t *testing.T, spec string) *DocExtractor {
	t.Helper()
	s, err := ParseDocSpec([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	x, err := NewDocExtractor(s)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

// brief renders nodes as "Kind Name@Line-EndLine sig=… calls=[…]".
func brief(nodes []core.SymbolNode) []string {
	var out []string
	for _, n := range nodes {
		s := fmt.Sprintf("%s %s@%d-%d", n.Kind, n.Name, n.Line, n.EndLine)
		if n.Signature != "" {
			s += " sig=" + n.Signature
		}
		if len(n.Calls) > 0 {
			s += " calls=" + strings.Join(n.Calls, ",")
		}
		out = append(out, s)
	}
	return out
}

// gitlabSpec is design §4's example doc.json.
const gitlabSpec = `{
  "decode": "yaml",
  "multi_document": true,
  "detect": { "basenames": [".gitlab-ci.yml"], "globs": [".gitlab/ci/**/*.yml"],
              "require_keys": [], "content_sniff": null },
  "entities": [
    { "select": "$.*", "where": { "is_map": true, "not_key_in": ["stages","variables","default","include","workflow"], "key_not_prefix": "." },
      "kind": "job", "source_kind": "GitLabCIJob", "name": "$key", "signature": "$.stage" },
    { "select": "$.*", "where": { "is_map": true, "key_prefix": "." },
      "kind": "job", "source_kind": "GitLabCITemplate", "name": "$key" }
  ],
  "references": [
    { "from": "job", "select": "$.needs[*]", "value": "$ | .job", "to_kind": "job", "edge": "needs" },
    { "from": "job", "select": "$.extends", "value": "$", "to_kind": "job", "edge": "extends" }
  ],
  "cross_file": [
    { "select": "$.include[*]", "value": "$.local | $", "resolve": "repo-path", "edge": "includes" }
  ]
}`

// TestDocExtractorSelectors pins LB-F8 (design §4): the selector subset
// ($, .key, .*, [*], $key, alternation) over YAML (multi-document), JSON,
// JSONC, TOML and XML; where filters; intra-file references into Calls;
// cross-file references resolved by strategy; lines and end lines from the
// source.
func TestDocExtractorSelectors(t *testing.T) {
	t.Run("yaml gitlab-ci", func(t *testing.T) {
		src := `stages: [build, test]
include:
  - local: /ci/common.yml
  - ci/other.yml
  - remote: https://example.invalid/x.yml
.base:
  image: alpine
build:
  stage: build
  extends: .base
  script: [make]
test:
  stage: test
  needs:
    - build
    - job: lint
      optional: true
lint:
  stage: test
  needs: build
`
		r, err := mustDoc(t, gitlabSpec).ExtractDoc("svc", "sub/.gitlab-ci.yml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			"GitLabCITemplate .base@6-7",
			"GitLabCIJob build@8-11 sig=build calls=.base",
			"GitLabCIJob test@12-17 sig=test calls=build,lint",
			"GitLabCIJob lint@18-20 sig=test calls=build",
		}
		if got := brief(r.Nodes); !reflect.DeepEqual(got, want) {
			t.Fatalf("nodes:\n got %q\nwant %q", got, want)
		}
		var cross []string
		for _, c := range r.CrossFile {
			cross = append(cross, fmt.Sprintf("%s %s->%s@%d", c.Edge, c.Raw, c.Target, c.Line))
		}
		wantCross := []string{"includes /ci/common.yml->ci/common.yml@3", "includes ci/other.yml->sub/ci/other.yml@4"}
		if !reflect.DeepEqual(cross, wantCross) {
			t.Fatalf("cross:\n got %q\nwant %q", cross, wantCross)
		}
		for _, n := range r.Nodes {
			if n.Tier != core.TierVerified || n.FilePath != "sub/.gitlab-ci.yml" || n.Service != "svc" {
				t.Fatalf("node identity: %+v", n)
			}
		}
	})

	t.Run("yaml multi-document with require_keys and equals", func(t *testing.T) {
		x := mustDoc(t, `{"decode":"yaml","multi_document":true,"detect":{"require_keys":["kind","metadata"]},
 "entities":[
  {"select":"$","where":{"equals":{"$.kind":"Deployment"}},"kind":"resource","source_kind":"K8sDeploy","name":".metadata.name","signature":"$.kind"},
  {"select":"$.spec.template.spec.containers[*]","kind":"element","source_kind":"K8sContainer","name":".name","signature":".image"}],
 "cross_file":[{"from":"element","select":"$.image","value":"$","resolve":"image-ref","edge":"runs"}]}`)
		src := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      containers:
        - name: app
          image: example/app:1
---
kind: Service
metadata:
  name: web-svc
---
just: a map without kind
`
		r, _ := x.ExtractDoc("s", "k8s/web.yaml", []byte(src))
		want := []string{"K8sDeploy web@1-10 sig=Deployment", "K8sContainer app@9-10 sig=example/app:1"}
		if got := brief(r.Nodes); !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q want %q", got, want)
		}
		if len(r.CrossFile) != 1 || r.CrossFile[0].From != "app" || r.CrossFile[0].Target != "example/app:1" {
			t.Fatalf("cross = %+v", r.CrossFile)
		}
	})

	t.Run("json and jsonc", func(t *testing.T) {
		spec := `{"decode":"%s","detect":{},"entities":[
  {"select":"$.scripts.*","kind":"task","source_kind":"NpmScript","name":"$key","signature":"$"},
  {"select":"$.workspaces[*]","kind":"module","source_kind":"NpmWorkspace","name":"$"}],
 "cross_file":[{"select":"$.workspaces[*]","value":"$","resolve":"repo-path","edge":"workspace"}]}`
		json := "{\n  \"name\": \"x\",\n  \"scripts\": {\n    \"build\": \"tsc\",\n    \"test\": \"vitest\"\n  },\n  \"workspaces\": [\"packages/a\", \"../out\"]\n}\n"
		jsonc := "// top comment\n{\n  \"scripts\": { /* inline */\n    \"build\": \"tsc\", // trailing\n    \"test\": \"vitest\",\n  },\n  \"workspaces\": [\"packages/a\", \"../out\",],\n}\n"
		r, _ := mustDoc(t, fmt.Sprintf(spec, "json")).ExtractDoc("s", "package.json", []byte(json))
		want := []string{"NpmScript build@4-4 sig=tsc", "NpmScript test@5-5 sig=vitest", "NpmWorkspace ../out@7-7", "NpmWorkspace packages/a@7-7"}
		if got := brief(r.Nodes); !reflect.DeepEqual(got, want) {
			t.Fatalf("json: got %q want %q", got, want)
		}
		if len(r.CrossFile) != 2 || r.CrossFile[0].Target != "" || r.CrossFile[1].Target != "packages/a" {
			// sorted by line then raw: "../out" (escapes the repo: no target) before "packages/a".
			t.Fatalf("json cross = %+v", r.CrossFile)
		}
		r, _ = mustDoc(t, fmt.Sprintf(spec, "jsonc")).ExtractDoc("s", "tsconfig.json", []byte(jsonc))
		want = []string{"NpmScript build@4-4 sig=tsc", "NpmScript test@5-5 sig=vitest", "NpmWorkspace ../out@7-7", "NpmWorkspace packages/a@7-7"}
		if got := brief(r.Nodes); !reflect.DeepEqual(got, want) {
			t.Fatalf("jsonc: got %q want %q", got, want)
		}
		if nodes, _ := mustDoc(t, fmt.Sprintf(spec, "json")).Extract("s", "bad.json", []byte("{ not json")); nodes != nil {
			t.Fatalf("malformed json yielded %v", nodes)
		}
	})

	t.Run("toml", func(t *testing.T) {
		x := mustDoc(t, `{"decode":"toml","detect":{},"entities":[
  {"select":"$.package","kind":"package","source_kind":"CargoPackage","name":".name","signature":".version"},
  {"select":"$.bin[*]","kind":"target","source_kind":"CargoBin","name":".name"},
  {"select":"$.dependencies.*","kind":"module","source_kind":"CargoDep","name":"$key","signature":"$ | .version"}],
 "cross_file":[{"select":"$.dependencies.*.path","value":"$","resolve":"module-source","edge":"path-dep"},
               {"select":"$.workspace.members[*]","value":"$","resolve":"repo-path","edge":"member"}]}`)
		src := `[package]
name = "demo"
version = "0.1.0"

[workspace]
members = ["crates/a"]

[dependencies]
serde = "1"
local = { path = "../local", version = "0.2" }

[[bin]]
name = "one"

[[bin]]
name = "two"
`
		r, err := x.ExtractDoc("s", "Cargo.toml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"CargoPackage demo@1-3 sig=0.1.0", "CargoDep serde@9-9 sig=1", "CargoDep local@10-10 sig=0.2", "CargoBin one@12-13", "CargoBin two@15-16"}
		if got := brief(r.Nodes); !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q want %q", got, want)
		}
		var targets []string
		for _, c := range r.CrossFile {
			targets = append(targets, c.Edge+":"+c.Target)
		}
		sort.Strings(targets)
		if !reflect.DeepEqual(targets, []string{"member:crates/a", "path-dep:"}) {
			t.Fatalf("cross = %q", targets)
		}
	})

	// A table reached only through deeper headers ([tool.black.sub]) or
	// dotted keys (isort.profile under [tool]) is implicit: it lives where
	// it is first written, never at its parent's line (LB-Q-python-packaging).
	t.Run("toml implicit tables", func(t *testing.T) {
		x := mustDoc(t, `{"decode":"toml","detect":{},"entities":[
  {"select":"$.tool.*","kind":"section","source_kind":"PyProjectTool","name":"$key"}]}`)
		src := `[project]
name = "demo"

[tool]
isort.profile = "black"

[tool.black.sub]
x = 1

[tool.ruff]
line-length = 100

[tool.ruff.lint]
select = ["E"]
`
		r, err := x.ExtractDoc("s", "pyproject.toml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		lines := map[string]int{}
		for _, n := range r.Nodes {
			lines[n.Name] = n.Line
		}
		if want := map[string]int{"isort": 5, "black": 7, "ruff": 10}; !reflect.DeepEqual(lines, want) {
			t.Fatalf("lines = %v, want %v", lines, want)
		}
	})

	t.Run("xml", func(t *testing.T) {
		x := mustDoc(t, `{"decode":"xml","detect":{},"entities":[
  {"select":"$","kind":"module","source_kind":"MavenProject","name":".artifactId","signature":".groupId"},
  {"select":"$.dependencies.dependency[*]","kind":"module","source_kind":"MavenDep","name":".artifactId","signature":".version | .@scope"},
  {"select":"$.modules.module[*]","kind":"module","source_kind":"MavenModule","name":"$"}],
 "cross_file":[{"select":"$.modules.module[*]","value":"$","resolve":"repo-path","edge":"module"}]}`)
		src := `<?xml version="1.0"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <groupId>org.example</groupId>
  <artifactId>app</artifactId>
  <modules>
    <module>core</module>
  </modules>
  <dependencies>
    <dependency>
      <artifactId>junit</artifactId>
      <version>4.13</version>
    </dependency>
    <dependency>
      <artifactId>guava</artifactId>
    </dependency>
  </dependencies>
</project>
`
		r, err := x.ExtractDoc("s", "svc/pom.xml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"MavenProject app@2-17 sig=org.example", "MavenModule core@6-6", "MavenDep junit@9-12 sig=4.13", "MavenDep guava@13-15"}
		if got := brief(r.Nodes); !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q want %q", got, want)
		}
		if len(r.CrossFile) != 1 || r.CrossFile[0].Target != "svc/core" {
			t.Fatalf("cross = %+v", r.CrossFile)
		}
	})
}

// TestDocExtractorSpecRefusals: a spec outside the subset is refused at
// construction, naming the field — never a silently empty extractor.
func TestDocExtractorSpecRefusals(t *testing.T) {
	cases := map[string]string{
		`{"decode":"ini","detect":{},"entities":[{"select":"$","kind":"key","source_kind":"K","name":"$key"}]}`:                        "decode",
		`{"decode":"json","multi_document":true,"detect":{},"entities":[{"select":"$","kind":"key","source_kind":"K","name":"$key"}]}`: "multi_document",
		`{"decode":"yaml","detect":{},"entities":[]}`:                                                                                                                                    "no entities",
		`{"decode":"yaml","detect":{},"entities":[{"select":"$..x","kind":"key","source_kind":"K","name":"$key"}]}`:                                                                      "select",
		`{"decode":"yaml","detect":{},"entities":[{"select":"$.a[0]","kind":"key","source_kind":"K","name":"$key"}]}`:                                                                    "select",
		`{"decode":"yaml","detect":{},"entities":[{"select":"$","kind":"gizmo","source_kind":"K","name":"$key"}]}`:                                                                       "canonical",
		`{"decode":"yaml","detect":{},"entities":[{"select":"$","kind":"key","source_kind":"K","name":"$key"}],"references":[{"from":"nope","select":"$","value":"$","to_kind":"key"}]}`: "names no entity kind",
		`{"decode":"yaml","detect":{},"entities":[{"select":"$","kind":"key","source_kind":"K","name":"$key"}],"cross_file":[{"select":"$","value":"$","resolve":"magic","edge":"x"}]}`:  "resolve",
		`{"decode":"yaml","detect":{},"entities":[{"select":"$","kind":"key","source_kind":"K","name":"$key","template":"{{x}}"}]}`:                                                      "unknown field",
		`{"decode":"yaml","detect":{},"entities":[{"select":"$","kind":"key","source_kind":"K","name":"$key"}],"labels":[{"from":"nope","select":"$"}]}`:                                 "labels[0]: from",
	}
	for spec, want := range cases {
		s, err := ParseDocSpec([]byte(spec))
		if err == nil {
			_, err = NewDocExtractor(s)
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("spec %s: err = %v, want one containing %q", spec, err, want)
		}
	}
}

// TestDocExtractorParity is P20's proof that the engine generalizes the
// bespoke config extractors: a doc.json reproduces the Compose extractor's
// services (name, line, depends_on), the GitHub Actions extractor's jobs
// (name, line, runs-on, needs) and actions (uses, line), and the Kustomize
// extractor's resources, patches and images (name, line) on their
// fixtures. Signatures the bespoke extractors compose from several fields
// (Compose's "build: <ctx>", Kustomize's "newName:newTag") are outside the
// selector subset by design (§4: no templating) and not compared.
func TestDocExtractorParity(t *testing.T) {
	type pair struct {
		spec, fixtureGlob string
		bespoke           Extractor
		kinds             []core.SourceKind
		withSig           bool
	}
	pairs := []pair{
		{"testdata/doc/compose.doc.json", "testdata/doc/compose/*.yml", NewComposeExtractor(), []core.SourceKind{core.KindComposeService}, false},
		{"testdata/doc/gha.doc.json", "testdata/doc/gha/*.yml", NewGHAExtractor(), []core.SourceKind{core.KindGHAJob, core.KindGHAAction}, true},
		{"testdata/doc/kustomize.doc.json", "testdata/doc/kustomize/*.yaml", NewKustomizeExtractor(),
			[]core.SourceKind{core.KindKustomizeResource, core.KindKustomizePatch, core.KindKustomizeImage}, false},
	}
	for _, p := range pairs {
		b, err := os.ReadFile(p.spec)
		if err != nil {
			t.Fatal(err)
		}
		x := mustDoc(t, string(b))
		fixtures, _ := filepath.Glob(p.fixtureGlob)
		if len(fixtures) == 0 {
			t.Fatalf("no fixtures for %s", p.fixtureGlob)
		}
		for _, f := range fixtures {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			old, _ := p.bespoke.Extract("s", f, src)
			neu, _ := x.Extract("s", f, src)
			key := func(ns []core.SymbolNode) []string {
				var out []string
				for _, n := range ns {
					keep := false
					for _, k := range p.kinds {
						keep = keep || n.Kind == k
					}
					if !keep {
						continue
					}
					calls := append([]string(nil), n.Calls...)
					sort.Strings(calls) // the doc engine's Calls are a sorted set
					s := fmt.Sprintf("%s %s@%d calls=%v", n.Kind, n.Name, n.Line, calls)
					if p.withSig && n.Kind == core.KindGHAJob {
						s += " sig=" + n.Signature
					}
					out = append(out, s)
				}
				sort.Strings(out)
				return out
			}
			if got, want := key(neu), key(old); !reflect.DeepEqual(got, want) {
				t.Errorf("%s:\n doc engine %q\n bespoke    %q", f, got, want)
			}
			if len(key(old)) == 0 {
				t.Errorf("%s: the bespoke extractor found nothing — not a parity fixture", f)
			}
		}
	}
}
