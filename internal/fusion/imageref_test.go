package fusion

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestNormalizeImageRef (LB-X-docker-image-ref): an image reference reads as
// Docker reads it — docker.io/library/ and docker.io/ (and a bare library/)
// dropped, a missing tag read as :latest, a digest kept as written, a registry
// port not mistaken for a tag — and a reference holding an unexpanded ${X},
// $(X) or $X is never normalized (it is not one image, it is a template).
func TestNormalizeImageRef(t *testing.T) {
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"alpine", "alpine:latest", true},
		{"alpine:3.19", "alpine:3.19", true},
		{"docker.io/library/alpine:3.19", "alpine:3.19", true},
		{"index.docker.io/library/alpine", "alpine:latest", true},
		{"library/alpine", "alpine:latest", true},
		{"docker.io/acme/base", "acme/base:latest", true},
		{"acme/base:1.0", "acme/base:1.0", true},
		{"registry.example.com:5000/acme/app", "registry.example.com:5000/acme/app:latest", true},
		{"registry.example.com:5000/acme/app:2", "registry.example.com:5000/acme/app:2", true},
		{"acme/base@sha256:0123abcd", "acme/base@sha256:0123abcd", true},
		{"docker.io/acme/base:1.0@sha256:0123abcd", "acme/base:1.0@sha256:0123abcd", true},
		{"  acme/base  ", "acme/base:latest", true},
		{"${REGISTRY}/app:1", "", false},
		{"acme/app:${TAG}", "", false},
		{"$(IMAGE)", "", false},
		{"$IMAGE", "", false},
		{"acme/app:$TAG", "", false},
		{"", "", false},
		{"acme/app:", "", false},
	} {
		got, ok := NormalizeImageRef(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeImageRef(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// dockerfileDocs is each Dockerfile as the doc-xref pass synthesizes it from
// the dispatched dockerfile tags dialect's own nodes.
func dockerfileDocs(t *testing.T, files map[string]string) []DocFile {
	t.Helper()
	x, err := extractor.NewTSExtractor("dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(x.Close)
	var out []DocFile
	for _, p := range sortedMapKeys(files) {
		nodes, err := x.Extract("svc", p, []byte(files[p]))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, DockerfileDocFile(p, []byte(files[p]), nodes))
	}
	return out
}

// lookups is every tag's Lookup, "" for no answer.
func lookups(idx *ImageIndex, tags ...string) map[string]string {
	got := map[string]string{}
	for _, tag := range tags {
		f, _ := idx.Lookup(tag)
		got[tag] = f
	}
	return got
}

// TestImageIndexFeeders (LB-X-docker-image-ref, design D7): the three build
// sites a tag is joined to the Dockerfile that builds it through — a compose
// service carrying both image: and build: (context, then dockerfile: relative
// to it), a docker-bake target (tags, context, dockerfile relative to the
// context; HCL and JSON, inherits followed), and a literal `docker build` or
// `docker buildx build` with -t and the context (and -f) in a shell script, a
// Makefile recipe or a CI run step. A build whose Dockerfile is not a file of
// the set builds nothing the index can name.
func TestImageIndexFeeders(t *testing.T) {
	files := docFiles(t, "docker-compose", map[string]string{
		"compose.yaml": "services:\n  base:\n    image: acme/base:1.0\n    build: ./base\n" +
			"  api:\n    image: docker.io/acme/api\n    build:\n      context: ./api\n      dockerfile: build/Dockerfile\n" +
			"  root:\n    image: acme/root:2\n    build: .\n" +
			"  nodf:\n    image: acme/nodf\n    build:\n      dockerfile: worker.dockerfile\n" +
			"  ghost:\n    image: acme/ghost\n    build: ./missing\n" +
			"  runonly:\n    image: acme/runonly\n",
	})
	files = append(files, dockerfileDocs(t, map[string]string{
		"Dockerfile":                 "FROM alpine:3.19\n",
		"base/Dockerfile":            "FROM alpine:3.19 AS base\n",
		"api/build/Dockerfile":       "FROM golang:1.22 AS build\n",
		"worker.dockerfile":          "FROM alpine:3.19 AS worker\n",
		"bake/web/Dockerfile":        "FROM nginx:1.27 AS web\n",
		"bake/web/Dockerfile.edge":   "FROM nginx:1.27 AS edge\n",
		"bake/json/Dockerfile":       "FROM nginx:1.27 AS j\n",
		"tools/Dockerfile":           "FROM alpine:3.19 AS tool\n",
		"mk/Dockerfile":              "FROM alpine:3.19 AS mk\n",
		"ci/Dockerfile":              "FROM alpine:3.19 AS ci\n",
		"scripts/local/Dockerfile":   "FROM alpine:3.19 AS local\n",
		"bake/inherit/Dockerfile.ci": "FROM alpine:3.19 AS inh\n",
	})...)
	files = append(files,
		DocFile{Path: "bake/docker-bake.hcl", Builds: BakeBuilds("bake/docker-bake.hcl", []byte(`
group "default" { targets = ["web", "edge"] }
target "web" {
  context = "web"
  tags = ["acme/web:1", "registry.example.com/acme/web:1"]
}
target "edge" {
  context = "web"
  dockerfile = "Dockerfile.edge"
  tags = ["acme/edge"]
}
target "_common" {
  context = "inherit"
  dockerfile = "Dockerfile.ci"
}
target "inh" {
  inherits = ["_common"]
  tags = ["acme/inh:3"]
}
`))},
		DocFile{Path: "bake/docker-bake.json", Builds: BakeBuilds("bake/docker-bake.json", []byte(`{"target":{"j":{"context":"json","tags":["acme/json:1"]}}}`))},
		DocFile{Path: "scripts/build.sh", Builds: ShellBuilds("scripts/build.sh", []byte("#!/bin/sh\nset -e\ndocker build -t acme/tool:0.1 -f tools/Dockerfile tools\n"+
			"docker build \\\n  --build-arg X=1 \\\n  --tag acme/local \\\n  local && echo done\n"))},
		DocFile{Path: "mk/Makefile", Builds: ShellBuilds("mk/Makefile", []byte("image:\n\t@docker buildx build --push --platform linux/amd64 -t acme/mk:1.0 .\n"))},
		DocFile{Path: ".github/workflows/ci.yml", Builds: ShellBuilds(".github/workflows/ci.yml", []byte("jobs:\n  b:\n    steps:\n      - run: docker build --tag=acme/ci:latest ./ci\n      - run: |\n          docker build -q -t acme/ci2 \\\n            -f ci/Dockerfile ci\n"))},
	)
	idx := BuildImageIndex(files)
	got := lookups(idx,
		"acme/base:1.0", "acme/api", "acme/root:2", "acme/nodf", "acme/ghost", "acme/runonly",
		"acme/web:1", "registry.example.com/acme/web:1", "acme/edge:latest", "acme/inh:3", "acme/json:1",
		"acme/tool:0.1", "acme/local", "acme/mk:1.0", "acme/ci", "acme/ci2", "docker.io/library/alpine:3.19")
	want := map[string]string{
		"acme/base:1.0":                   "base/Dockerfile",
		"acme/api":                        "api/build/Dockerfile",
		"acme/root:2":                     "Dockerfile",
		"acme/nodf":                       "worker.dockerfile",
		"acme/ghost":                      "",
		"acme/runonly":                    "",
		"acme/web:1":                      "bake/web/Dockerfile",
		"registry.example.com/acme/web:1": "bake/web/Dockerfile",
		"acme/edge:latest":                "bake/web/Dockerfile.edge",
		"acme/inh:3":                      "bake/inherit/Dockerfile.ci",
		"acme/json:1":                     "bake/json/Dockerfile",
		"acme/tool:0.1":                   "tools/Dockerfile",
		"acme/local":                      "scripts/local/Dockerfile",
		"acme/mk:1.0":                     "mk/Dockerfile",
		"acme/ci":                         "ci/Dockerfile",
		"acme/ci2":                        "ci/Dockerfile",
		"docker.io/library/alpine:3.19":   "",
	}
	if !reflect.DeepEqual(got, want) {
		for k := range want {
			if got[k] != want[k] {
				t.Errorf("Lookup(%q) = %q, want %q", k, got[k], want[k])
			}
		}
	}
}

// TestImageIndexSkipsTemplatedTags: a tag holding an unexpanded ${X}, $(X) or
// $X is never joined, wherever it is written, and the index counts each one so
// the pass can disclose it.
func TestImageIndexSkipsTemplatedTags(t *testing.T) {
	files := docFiles(t, "docker-compose", map[string]string{
		"compose.yaml": "services:\n  a:\n    image: ${REGISTRY}/acme/a:1\n    build: ./a\n  b:\n    image: acme/b:1\n    build: ./a\n",
	})
	files = append(files, dockerfileDocs(t, map[string]string{"a/Dockerfile": "FROM alpine:3.19 AS a\n"})...)
	files = append(files,
		DocFile{Path: "docker-bake.hcl", Builds: BakeBuilds("docker-bake.hcl", []byte("variable \"TAG\" { default = \"1\" }\ntarget \"a\" {\n  context = \"a\"\n  tags = [\"acme/a:${TAG}\", \"acme/c:1\"]\n}\n"))},
		DocFile{Path: "Makefile", Builds: ShellBuilds("Makefile", []byte("build:\n\tdocker build -t $(IMAGE) a\n\tdocker build -t $IMAGE:dev a\n"))},
	)
	idx := BuildImageIndex(files)
	if idx.TemplatedBuilds != 4 {
		t.Errorf("TemplatedBuilds = %d, want 4 (compose ${REGISTRY}, bake ${TAG}, make $(IMAGE) and $IMAGE)", idx.TemplatedBuilds)
	}
	got := lookups(idx, "acme/b:1", "acme/c:1", "acme/a:1", "${REGISTRY}/acme/a:1")
	want := map[string]string{"acme/b:1": "a/Dockerfile", "acme/c:1": "a/Dockerfile", "acme/a:1": "", "${REGISTRY}/acme/a:1": ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lookups %v, want %v", got, want)
	}
}

// TestImageIndexAmbiguousTag: two build sites naming two Dockerfiles for one
// tag are ambiguous, and the tag joins nothing; several sites agreeing on one
// Dockerfile are one build.
func TestImageIndexAmbiguousTag(t *testing.T) {
	files := dockerfileDocs(t, map[string]string{
		"a/Dockerfile": "FROM alpine:3.19 AS a\n",
		"b/Dockerfile": "FROM alpine:3.19 AS b\n",
	})
	files = append(files, docFiles(t, "docker-compose", map[string]string{
		"compose.yaml": "services:\n  a:\n    image: acme/dup\n    build: ./a\n  same:\n    image: acme/same:1\n    build: ./b\n",
	})...)
	files = append(files, DocFile{Path: "build.sh", Builds: ShellBuilds("build.sh", []byte("docker build -t acme/dup:latest b\ndocker build -t acme/same:1 ./b\n"))})
	idx := BuildImageIndex(files)
	if f, ok := idx.Lookup("acme/dup"); ok {
		t.Errorf("Lookup(acme/dup) = %q; two Dockerfiles build it, want no answer", f)
	}
	if f, ok := idx.Lookup("acme/same:1"); !ok || f != "b/Dockerfile" {
		t.Errorf("Lookup(acme/same:1) = %q, %v; want b/Dockerfile (two sites, one Dockerfile)", f, ok)
	}
	if idx.Ambiguous != 1 {
		t.Errorf("Ambiguous = %d, want 1", idx.Ambiguous)
	}
}

// TestShellBuildsParse: the command-line reading a shell feeder relies on —
// flags that take a value are skipped with it, the first other word is the
// context, -t may repeat, `docker image build` and `sudo docker build` count, a
// command ends at ;, &&, || or |, a comment is not a command, docker must be the
// command word (an echo of one is not a build), and a build with no context or
// a stdin context builds nothing.
func TestShellBuildsParse(t *testing.T) {
	src := "# docker build -t acme/commented .\n" +
		"sudo docker build --network host -t acme/a -t acme/a:2 --target final ctx; docker build -t acme/after ctx2\n" +
		"docker image build --platform=linux/arm64 --tag acme/img ./img\n" +
		"docker build -t acme/stdin - < Dockerfile\n" +
		"docker build -t acme/noctx\n" +
		"echo docker build -t acme/echo . | tee log\n" +
		"docker buildx bake -f docker-bake.hcl\n" +
		"docker build -t \"acme/quoted:1\" 'q dir'\n"
	var got []string
	for _, b := range ShellBuilds("x/build.sh", []byte(src)) {
		got = append(got, b.Tag+" "+b.Dockerfiles[0])
	}
	sort.Strings(got)
	want := []string{
		"acme/a x/ctx/Dockerfile",
		"acme/a:2 x/ctx/Dockerfile",
		"acme/after x/ctx2/Dockerfile",
		"acme/img x/img/Dockerfile",
		"acme/quoted:1 x/q dir/Dockerfile",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ShellBuilds\n got: %q\nwant: %q", got, want)
	}
}

// imageRefRepoCorpus is a generic repository in which acme/base is built by
// compose, a bake target, a script and a Makefile — every site naming the same
// Dockerfile — and app/Dockerfile builds FROM it and copies out of it.
func imageRefRepoCorpus() map[string]string {
	return map[string]string{
		"compose.yaml":     "include:\n  - ./ops/compose.yaml\nservices:\n  base:\n    image: acme/base\n    build: ./base\n  app:\n    build: ./app\n    depends_on: [base]\n    extends:\n      file: common.yaml\n      service: defaults\n  db:\n    image: postgres:16\n",
		"ops/compose.yaml": "services:\n  proxy:\n    image: docker.io/acme/base:latest\n",
		"common.yaml":      "services:\n  defaults:\n    restart: unless-stopped\n",
		"docker-bake.hcl":  "target \"base\" {\n  context = \"base\"\n  tags = [\"acme/base:latest\"]\n}\n",
		"scripts/build.sh": "#!/bin/sh\nset -eu\ndocker build -t acme/base ./base\n",
		"Makefile":         "image:\n\tdocker build -t docker.io/acme/base:latest base\n",
		"base/Dockerfile":  "FROM alpine:3.19\nRUN apk add --no-cache curl\n",
		"app/Dockerfile":   "FROM acme/base AS runtime\nCOPY --from=acme/base:latest /etc/ssl /etc/ssl\nCOPY . /app\n",
	}
}

// storedDocXrefs is the stored doc-xref edges as "file:name -> file:name",
// sorted; an endpoint that is no stored symbol fails the test.
func storedDocXrefs(t *testing.T, st store.Store, svc string) []string {
	t.Helper()
	ctx := context.Background()
	obs, err := st.QueryObservations(ctx, svc, "")
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]string{}
	for _, o := range obs {
		n := o.ToSymbolNode()
		byKey[n.Key()] = n.FilePath + ":" + n.Name
	}
	all, err := st.TypedEdges(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range all {
		if e.Kind != core.EdgeFileResolved {
			continue
		}
		from, ok1 := byKey[e.FromSymbol]
		to, ok2 := byKey[e.ToSymbol]
		if !ok1 || !ok2 {
			t.Errorf("edge %s -> %s: an endpoint is no stored symbol", e.FromSymbol, e.ToSymbol)
			continue
		}
		got = append(got, from+" -> "+to)
	}
	sort.Strings(got)
	return got
}

// TestIngest_ImageRefEdgesReachTheStore (LB-X-docker-image-ref and LB-F89,
// the product-reach proof): an ingest builds the image-tag index from the
// compose service, the bake target, the script and the Makefile, reads each
// Dockerfile through the dispatched dockerfile dialect and each compose file
// through the dispatched docker-compose dialect (a fragment with an arbitrary
// name by its content), and stores, all between stored symbol keys: app's FROM
// acme/base (and its COPY --from of the same image) to the root stage of
// base/Dockerfile; each compose build: to the Dockerfile it names, and an
// image: the repository builds (another file's included service too) to its
// Dockerfile; the include: to the included file's root and the extends.file
// to the fragment's root. An image nothing builds (postgres) gives nothing.
func TestIngest_ImageRefEdgesReachTheStore(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, imageRefRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "img", Path: dir}); err != nil {
		t.Fatal(err)
	}
	got := storedDocXrefs(t, st, "img")
	want := []string{
		"app/Dockerfile:runtime -> base/Dockerfile:alpine:3.19",
		"compose.yaml:app -> app/Dockerfile:runtime",
		"compose.yaml:app -> common.yaml:common.yaml",
		"compose.yaml:base -> base/Dockerfile:alpine:3.19",
		"compose.yaml:compose.yaml -> ops/compose.yaml:compose.yaml",
		"ops/compose.yaml:proxy -> base/Dockerfile:alpine:3.19",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", got, want)
	}
}
