package extractor

// jsonschemadialect_test.go — goldens for the json-schema doc-engine dialect
// (LB-Q-json-schema; design §13.2 steps 10/12/14): each fixture's entities, the
// references they make to the document's own entities (Calls) and their
// cross-file references. A fixture is read as if it sat in schemas/<its path
// under testdata>, so a relative $ref resolves against a directory.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestJSONSchemaDialectGoldens(t *testing.T) {
	const id = "json-schema"
	x, err := NewDocDialectExtractor(id)
	if err != nil {
		t.Fatalf("NewDocDialectExtractor(%s): %v", id, err)
	}
	for _, fx := range appFixtures(t, id) {
		t.Run(fx, func(t *testing.T) {
			path := filepath.Join("dialects", id, "testdata", filepath.FromSlash(fx))
			rel := "schemas/" + fx
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			res, err := x.ExtractDoc("svc", rel, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			got := appGolden{Symbols: []appGoldenSymbol{}, CrossFile: []appGoldenCross{}}
			for _, n := range res.Nodes {
				got.Symbols = append(got.Symbols, appGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, appGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := path + ".golden.json"
			if *updateGoldens {
				b, _ := json.MarshalIndent(got, "", "  ")
				if err := os.WriteFile(gp, append(b, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v (run with -update to create)", err)
			}
			var want appGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []appGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []appGoldenCross{}
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("symbols\n got: %+v\nwant: %+v", got.Symbols, want.Symbols)
			}
			if !reflect.DeepEqual(got.CrossFile, want.CrossFile) {
				t.Errorf("cross_file\n got: %+v\nwant: %+v", got.CrossFile, want.CrossFile)
			}
		})
	}
}

// A document the decode refuses is an error, never a panic or a partial reading.
func TestJSONSchemaDialectSurvivesSyntaxError(t *testing.T) {
	x, err := NewDocDialectExtractor("json-schema")
	if err != nil {
		t.Fatal(err)
	}
	res, err := x.ExtractDoc("svc", "schemas/broken.schema.json", []byte("{\"$schema\": \"https://json-schema.org/draft/2020-12/schema\", \"$defs\": {\"A\": {\"type\": "))
	if err == nil && len(res.Nodes) != 0 {
		t.Fatalf("a truncated document read %d symbols with no error", len(res.Nodes))
	}
}
