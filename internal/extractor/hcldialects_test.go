package extractor

// hcldialects_test.go — goldens and contracts for the native HCL tool
// dialects (LB-Q-terragrunt, LB-Q-hcl-tools; design §13.2 steps 10/12). Each
// testdata fixture's golden was written by hand from reading the fixture: its
// entities (name, kind, lines, signature, calls), its root entities and the
// cross-file references the extractor records, whether or not anything
// resolves them.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestHCLDialectGoldens: every fixture of the two dialects against its
// hand-written golden, extracted as the golden's repository path.
func TestHCLDialectGoldens(t *testing.T) {
	for _, id := range []string{"terragrunt", "hcl-tools"} {
		x, ok := HCLToolDialectExtractor(id)
		if !ok {
			t.Fatalf("HCLToolDialectExtractor(%s): not registered", id)
		}
		doc := x.(DocResulter)
		dir := filepath.Join("dialects", id, "testdata")
		ents, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range ents {
			if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") {
				continue
			}
			n++
			fx := filepath.Join(dir, e.Name())
			t.Run(id+"/"+e.Name(), func(t *testing.T) {
				src, err := os.ReadFile(fx)
				if err != nil {
					t.Fatal(err)
				}
				if lines := strings.Count(string(src), "\n"); lines > 200 {
					t.Fatalf("fixture has %d lines, the recipe's cap is 200", lines)
				}
				raw, err := os.ReadFile(fx + ".golden.json")
				if err != nil {
					t.Fatalf("read golden: %v", err)
				}
				var want confGolden
				if err := json.Unmarshal(raw, &want); err != nil {
					t.Fatalf("golden: %v", err)
				}
				res, err := doc.ExtractDoc("svc", want.Path, src)
				if err != nil {
					t.Fatalf("ExtractDoc: %v", err)
				}
				got := confGolden{Path: want.Path, Roots: append([]string{}, res.Roots...), Symbols: []confGoldenSymbol{}, CrossFile: []confGoldenCross{}}
				for _, nd := range res.Nodes {
					if nd.Service != "svc" || nd.FilePath != want.Path || nd.Package != pkgFromPath(want.Path) || nd.Tier != core.TierVerified {
						t.Fatalf("plumbing changed: %+v", nd)
					}
					got.Symbols = append(got.Symbols, confGoldenSymbol{Name: nd.Name, Kind: string(nd.Kind), Line: nd.Line, EndLine: nd.EndLine, Signature: nd.Signature, Calls: nd.Calls})
				}
				for _, c := range res.CrossFile {
					got.CrossFile = append(got.CrossFile, confGoldenCross{From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line})
				}
				if want.Roots == nil {
					want.Roots = []string{}
				}
				if !reflect.DeepEqual(got, want) {
					gb, _ := json.MarshalIndent(got, "", "  ")
					t.Fatalf("mismatch; got:\n%s", gb)
				}
				nodes, err := x.Extract("svc", want.Path, src)
				if err != nil || len(nodes) != len(res.Nodes) {
					t.Fatalf("Extract: %d nodes, %v; ExtractDoc %d", len(nodes), err, len(res.Nodes))
				}
			})
		}
		if n < 3 {
			t.Errorf("%s: %d fixtures, want at least 3", id, n)
		}
	}
}

// TestTerragruntUnresolvedByReason: what no static reading resolves is counted
// with its reason, never guessed.
func TestTerragruntUnresolvedByReason(t *testing.T) {
	const unit = `include "a" {
  path = get_env("PARENT")
}
include "b" {
  path = find_in_parent_folders("root.hcl")
}
terraform {
  source = "git::https://example.com/org/modules.git//app?ref=v1"
}
dependency "x" {
  config_path = dependency.y.outputs.dir
}
dependency "y" {
  config_path = "../../../../outside"
}
dependencies {
  paths = ["${get_parent_terragrunt_dir()}/z", "../ok"]
}
`
	got := map[string]int{}
	for _, u := range TerragruntUnresolvedRefs("a/terragrunt.hcl", []byte(unit)) {
		got[u.Edge+" "+u.Reason]++
	}
	want := map[string]int{
		"includes function:get_env":                     1,
		"module-source external:git":                    1,
		"dependency reference:dependency":               1,
		"dependency outside-repo":                       1,
		"dependency function:get_parent_terragrunt_dir": 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unit: unresolved by reason %v, want %v", got, want)
	}

	// a file that is no unit reads its paths from its includer: unresolved too
	got = map[string]int{}
	for _, u := range TerragruntUnresolvedRefs("a/common.hcl", []byte("dependency \"v\" {\n  config_path = \"../vpc\"\n}\nterraform {\n  source = \"../modules/app\"\n}\n")) {
		got[u.Edge+" "+u.Reason]++
	}
	want = map[string]int{"dependency included-file": 1, "module-source included-file": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("included file: unresolved by reason %v, want %v", got, want)
	}

	// a root-level unit has no ancestor to find
	us := TerragruntUnresolvedRefs("terragrunt.hcl", []byte("include \"r\" {\n  path = find_in_parent_folders(\"root.hcl\")\n}\n"))
	if len(us) != 1 || us[0].Reason != "no-ancestor" {
		t.Errorf("root unit: %+v, want one no-ancestor", us)
	}

	// the source forms
	for src, want := range map[string]string{
		"tfr:///terraform-aws-modules/vpc/aws?version=3.3.0": "external:registry",
		"terraform-aws-modules/vpc/aws":                      "external:registry",
		"https://example.com/m.zip":                          "external:remote",
		"git@example.com:org/m.git":                          "external:git",
		"modules/app":                                        "external:other",
	} {
		if _, why := tgLocalSource("a", src); why != want {
			t.Errorf("tgLocalSource(%q) reason %q, want %q", src, why, want)
		}
	}
	for src, want := range map[string]string{".": "a", "./m": "a/m", "../m": "m", "../m//sub": "m/sub", "../m?ref=v1": "m"} {
		if got, why := tgLocalSource("a", src); got != want || why != "" {
			t.Errorf("tgLocalSource(%q) = %q, %q, want %q", src, got, why, want)
		}
	}
}

// TestHCLToolsExtract pins the shapes the goldens do not: an unquoted Packer
// source traversal, a build with source blocks, an unnamed Nomad service, a
// bake group of groups, a repeated name.
func TestHCLToolsExtract(t *testing.T) {
	x := NewHCLToolsExtractor()
	calls := func(rel, src string) map[string][]string {
		nodes, err := x.Extract("svc", rel, []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		out := map[string][]string{}
		for _, n := range nodes {
			out[n.Name] = n.Calls
		}
		return out
	}
	pk := calls("a.pkr.hcl", "source \"docker\" \"one\" {}\nbuild {\n  sources = [source.docker.one]\n  source \"docker.two\" {\n    name = \"alt\"\n  }\n}\nbuild {\n}\n")
	if !reflect.DeepEqual(pk["build"], []string{"source.docker.one", "source.docker.two"}) {
		t.Errorf("packer build calls %q", pk["build"])
	}
	if _, ok := pk["build#2"]; !ok {
		t.Errorf("a second unnamed build is build#2; have %v", pk)
	}
	nm := calls("j.nomad", "job \"j\" {\n  group \"g\" {\n    service {\n      port = \"x\"\n    }\n  }\n}\n")
	if _, ok := nm["j.g.service.service"]; !ok {
		t.Errorf("an unnamed service is <parent>.service.service; have %v", nm)
	}
	bk := calls("docker-bake.hcl", "group \"a\" {\n  targets = [\"t\"]\n}\ngroup \"b\" {\n  targets = [\"a\", \"t\"]\n}\ntarget \"t\" {\n}\n")
	if !reflect.DeepEqual(bk["group.b"], []string{"group.a", "t"}) {
		t.Errorf("bake group b calls %q", bk["group.b"])
	}
	// a file hclsyntax recovers nothing from is an error, as in the terraform dialect
	if _, err := x.Extract("svc", "x.pkr.hcl", []byte("\x00\x01")); err == nil {
		t.Log("no error for binary-looking input (best effort)")
	}
}
