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
	"strings"
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

// TestDockerComposeDialectIncumbentParity (design §13.2 step 15, port of
// composeextract.go): on the incumbent's own test fixtures the dialect emits
// the same services — SourceKind, name, line, tier and depends_on calls
// identical. Signature is identical for an image service; for a build-only
// service the incumbent spells "build: <context>" and the doc engine (§4: no
// literals, no templating) can only emit the bare context, a documented
// divergence (PROVENANCE.md), and a build map with no context reads "build: ."
// from the incumbent and "" here.
func TestDockerComposeDialectIncumbentParity(t *testing.T) {
	fixtures := map[string]string{
		"images-and-depends-on": `version: "3.9"
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
`,
		"build-forms": `services:
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
`,
		"no-services": `version: "3.9"
networks:
  default: {}
`,
	}
	x := loadDockerComposeExtractor(t)
	inc := NewComposeExtractor()
	for name, src := range fixtures {
		t.Run(name, func(t *testing.T) {
			old, err := inc.Extract("svc", "docker-compose.yml", []byte(src))
			if err != nil {
				t.Fatal(err)
			}
			neu, err := x.Extract("svc", "docker-compose.yml", []byte(src))
			if err != nil {
				t.Fatal(err)
			}
			if len(old) != len(neu) {
				t.Fatalf("service count: incumbent %d, dialect %d", len(old), len(neu))
			}
			byName := map[string]core.SymbolNode{}
			for _, n := range neu {
				byName[n.Name] = n
			}
			for _, o := range old {
				n, ok := byName[o.Name]
				if !ok {
					t.Errorf("dialect missed service %q", o.Name)
					continue
				}
				if n.Kind != o.Kind || n.Line != o.Line || n.Tier != o.Tier {
					t.Errorf("%s: kind/line/tier dialect %q/%d/%q, incumbent %q/%d/%q", o.Name, n.Kind, n.Line, n.Tier, o.Kind, o.Line, o.Tier)
				}
				if !reflect.DeepEqual(n.Calls, o.Calls) {
					t.Errorf("%s: calls dialect %v, incumbent %v", o.Name, n.Calls, o.Calls)
				}
				want := o.Signature
				if strings.HasPrefix(want, "build: ") {
					want = strings.TrimPrefix(want, "build: ")
					if want == "." && n.Signature == "" {
						want = ""
					}
				}
				if n.Signature != want {
					t.Errorf("%s: signature dialect %q, incumbent %q (want %q)", o.Name, n.Signature, o.Signature, want)
				}
			}
		})
	}
}
