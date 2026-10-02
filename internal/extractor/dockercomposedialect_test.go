package extractor

// dockercomposedialect_test.go — goldens for the docker-compose doc-engine
// dialect (LB-Q-docker-compose, a port of composeextract.go, design §13.2
// steps 10/12/14/15). Same golden shape as the kubernetes and github-actions
// dialects: symbols (name/kind/package/line/end_line/signature/calls/tier)
// and the cross-file references the engine records (whether or not
// fusion.ResolveDocXrefs can resolve them — see PROVENANCE.md).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

type composeGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type composeGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type composeGolden struct {
	Symbols   []composeGoldenSymbol `json:"symbols"`
	CrossFile []composeGoldenCross  `json:"cross_file"`
}

func loadDockerComposeExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "docker-compose" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(docker-compose): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"docker-compose\" not found in LoadDialects()")
	return nil
}

func dockerComposeFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "docker-compose", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", dir)
	}
	return out
}

// TestDockerComposeDialectGoldens: every testdata/*.yaml fixture's services
// and cross-file references against a hand-written golden. relPath is the
// fixture's own basename, so a cross-file target resolves against the
// repository root.
func TestDockerComposeDialectGoldens(t *testing.T) {
	x := loadDockerComposeExtractor(t)
	for _, fx := range dockerComposeFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			relPath := filepath.Base(fx)
			res, err := x.ExtractDoc("svc", relPath, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			var got composeGolden
			got.Symbols = []composeGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != relPath {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, composeGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			got.CrossFile = []composeGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, composeGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want composeGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []composeGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []composeGoldenCross{}
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("%s: symbols\n got: %+v\nwant: %+v", fx, got.Symbols, want.Symbols)
			}
			if !reflect.DeepEqual(got.CrossFile, want.CrossFile) {
				t.Errorf("%s: cross_file\n got: %+v\nwant: %+v", fx, got.CrossFile, want.CrossFile)
			}
		})
	}
}

// TestComposeDialectKeepsIncumbentContracts carries composeextract.go's own
// cases, retired with it (LB-Q-docker-compose, design §11 step 3; before the
// retirement TestDockerComposeDialectIncumbentParity read both extractors on
// these fixtures and lang-parity read MERGE on five subjects): one
// ComposeService per service at its key line, tier verified, image: as the
// signature, depends_on in its list and map forms as sorted Calls, nothing but
// the file's root entity for a file with no services, no error on malformed
// YAML, and the same output twice. The one divergence is unchanged: a build-only
// service is signed by its bare context (the incumbent spelled "build: <ctx>"),
// and a build map with no context reads "" (the incumbent's "build: .").
func TestComposeDialectKeepsIncumbentContracts(t *testing.T) {
	x := loadDockerComposeExtractor(t)
	services := func(t *testing.T, src string) map[string]core.SymbolNode {
		t.Helper()
		nodes, err := x.Extract("svc", "docker-compose.yml", []byte(src))
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		out := map[string]core.SymbolNode{}
		for _, n := range nodes {
			if n.Kind == core.KindComposeService {
				out[n.Name] = n
			} else if n.Kind != "ComposeFile" {
				t.Errorf("unexpected kind %q (%s)", n.Kind, n.Name)
			}
		}
		return out
	}
	t.Run("images and depends_on", func(t *testing.T) {
		got := services(t, `version: "3.9"
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_DB: appdb

  cache:
    image: redis:7-alpine
    depends_on:
      - db

  app:
    image: registry.example.com/app:latest
    depends_on:
      db:
        condition: service_healthy
      cache:
        condition: service_started
`)
		want := map[string]struct {
			line      int
			signature string
			calls     []string
		}{
			"db":    {3, "postgres:16", nil},
			"cache": {8, "redis:7-alpine", []string{"db"}},
			"app":   {13, "registry.example.com/app:latest", []string{"cache", "db"}},
		}
		if len(got) != len(want) {
			t.Fatalf("services %d, want %d", len(got), len(want))
		}
		for name, w := range want {
			n, ok := got[name]
			if !ok {
				t.Fatalf("missing service %q", name)
			}
			if n.Line != w.line || n.Signature != w.signature || n.Tier != core.TierVerified || !reflect.DeepEqual(n.Calls, w.calls) {
				t.Errorf("%s: line %d sig %q tier %q calls %v; want %d %q verified %v", name, n.Line, n.Signature, n.Tier, n.Calls, w.line, w.signature, w.calls)
			}
		}
	})
	t.Run("build forms", func(t *testing.T) {
		got := services(t, `services:
  builder:
    build: ./myapp
  helper:
    build:
      context: ./tools
      dockerfile: Dockerfile.dev
  bare:
    build:
      dockerfile: Dockerfile.only
  both:
    image: keep/this:1
    build: ./ignored-for-signature
`)
		for name, sig := range map[string]string{"builder": "./myapp", "helper": "./tools", "bare": "", "both": "keep/this:1"} {
			if n, ok := got[name]; !ok || n.Signature != sig {
				t.Errorf("%s: signature %q (present %v), want %q", name, n.Signature, ok, sig)
			}
		}
	})
	t.Run("no services", func(t *testing.T) {
		if got := services(t, "version: \"3.9\"\nnetworks:\n  default: {}\n"); len(got) != 0 {
			t.Errorf("services %v, want none", got)
		}
	})
	t.Run("malformed", func(t *testing.T) {
		if _, err := x.Extract("svc", "compose.yml", []byte("not: valid: yaml: :::")); err != nil {
			t.Errorf("Extract must not error on malformed YAML: %v", err)
		}
	})
	t.Run("determinism", func(t *testing.T) {
		src := []byte("services:\n  a:\n    image: x\n    depends_on: [b, c]\n  b:\n    image: y\n  c:\n    build: .\n")
		n1, _ := x.Extract("svc", "docker-compose.yml", src)
		n2, _ := x.Extract("svc", "docker-compose.yml", src)
		if !reflect.DeepEqual(n1, n2) {
			t.Errorf("non-deterministic:\n%+v\n%+v", n1, n2)
		}
	})
}

// TestDockerComposeDialectRootAndBuildRefs (LB-X-docker-image-ref): every
// compose file has a root entity named by the file (ComposeFile, signed by
// the optional top-level name:), the origin of its include: references and
// the target an include or an extends.file resolves to; a service's image:
// is an image-ref reference, and its build — a context string, a map's
// context, a map's dockerfile (relative to the context) — repo-path
// references the doc-xref resolver joins into one Dockerfile.
func TestDockerComposeDialectRootAndBuildRefs(t *testing.T) {
	x := loadDockerComposeExtractor(t)
	src := "name: shop\nservices:\n  a:\n    image: acme/a:1\n    build: ./a\n" +
		"  b:\n    build:\n      context: ./b\n      dockerfile: docker/Dockerfile\n" +
		"  c:\n    build:\n      dockerfile: c.dockerfile\n"
	res, err := x.ExtractDoc("svc", "deploy/compose.prod.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Roots, []string{"compose.prod.yaml"}) {
		t.Fatalf("roots %q, want the file's own name", res.Roots)
	}
	root := res.Nodes[0]
	if root.Kind != "ComposeFile" || root.Name != "compose.prod.yaml" || root.Signature != "shop" || root.Line != 1 {
		t.Fatalf("root entity %+v", root)
	}
	var got []string
	for _, c := range res.CrossFile {
		got = append(got, c.From+" "+c.Edge+" "+c.Strategy+" "+c.Raw)
	}
	want := []string{
		"a runs image-ref acme/a:1",
		"a build repo-path ./a",
		"b build repo-path ./b",
		"b build_dockerfile repo-path docker/Dockerfile",
		"c build_dockerfile repo-path c.dockerfile",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cross_file\n got: %q\nwant: %q", got, want)
	}
}
