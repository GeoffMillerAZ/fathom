package mcp_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/iacfacts"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// A made-up provider, written as a `tofu providers schema -json` export, the one input `fathom iac import` takes.
const iacTestSource = "registry.opentofu.org/example/widget"

func iacTestExport(t *testing.T) string {
	t.Helper()
	attr := func(extra map[string]any) map[string]any {
		a := map[string]any{"type": "string", "optional": true, "description": "prose the pruner drops"}
		for k, v := range extra {
			a[k] = v
		}
		return a
	}
	thing := map[string]any{"version": 1, "block": map[string]any{
		"attributes": map[string]any{
			"id":    attr(map[string]any{"computed": true}),
			"name":  attr(map[string]any{"required": true, "optional": false}),
			"size":  attr(map[string]any{"type": "number"}),
			"token": attr(map[string]any{"computed": true, "sensitive": true}),
			"old":   attr(map[string]any{"deprecated": true}),
		},
		"block_types": map[string]any{"rule": map[string]any{"nesting_mode": "list", "block": map[string]any{
			"attributes": map[string]any{
				"port":  attr(map[string]any{"type": "number", "required": true, "optional": false}),
				"proto": attr(nil),
			},
		}}},
	}}
	exp := map[string]any{"format_version": "1.0", "provider_schemas": map[string]any{iacTestSource: map[string]any{
		"provider":            map[string]any{"version": 0, "block": map[string]any{"attributes": map[string]any{}}},
		"resource_schemas":    map[string]any{"widget_thing": thing},
		"data_source_schemas": map[string]any{},
	}}}
	b, err := json.Marshal(exp)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// iacHome points FATHOM_HOME at a fresh directory and, when install is set, imports a catalog of the made-up
// provider at 1.2.0 into it, the only way a product path finds one.
func iacHome(t *testing.T, install bool) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	if !install {
		return
	}
	_, _, err := iacfacts.ImportIaCExports(filepath.Join(home, "iac"), []iacfacts.IaCImportInput{{Version: "1.2.0", Path: iacTestExport(t)}}, iacfacts.IaCWriterOptions{Tool: "test"})
	if err != nil {
		t.Fatal(err)
	}
}

func iacRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const iacProviders = `terraform {
  required_providers {
    widget = { source = "example/widget", version = "~> 1.0" }
  }
}
`

const iacMain = iacProviders + `
resource "widget_thing" "a" {
  size  = 1
  bogus = "x"
  old   = "y"
  rule {
    proto = "tcp"
  }
}

output "tok" {
  value = widget_thing.a.token
}
`

func iacLock(version string) string {
	return fmt.Sprintf("provider %q {\n  version = %q\n  constraints = \"~> 1.0\"\n  hashes = [\"h1:abc\"]\n}\n", iacTestSource, version)
}

func iacServer(root string) *mcp.Server {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "main", Name: "widget_thing.a", Kind: core.KindTerraformResource, FilePath: "main.tf", Line: 8, EndLine: 15},
		{Service: "svc", Package: "main", Name: "output.tok", Kind: core.KindTerraformOutput, FilePath: "main.tf", Line: 17, EndLine: 19},
	}
	return mcp.NewServer(&reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{nodes: nodes, services: []string{"svc"}, svcPath: root}})
}

func iacCall(t *testing.T, srv *mcp.Server, tool string, args map[string]any) (map[string]any, int) {
	t.Helper()
	resp := roundtrip(t, srv, request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": tool, "arguments": args}})
	p := extToolPayload(t, resp)
	raw, _ := json.Marshal(resp["result"])
	return p, len(raw)
}

func iacBlock(t *testing.T, p map[string]any) map[string]any {
	t.Helper()
	iac, ok := p["iac"].(map[string]any)
	if !ok {
		t.Fatalf("no iac block in %v", p)
	}
	return iac
}

func citedFact(t *testing.T, v any) map[string]any {
	t.Helper()
	f, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not a fact: %v", v)
	}
	for _, k := range []string{"catalog", "version", "digest", "path"} {
		if s, _ := f[k].(string); s == "" {
			t.Fatalf("fact lacks %s: %v", k, f)
		}
	}
	if len(f["digest"].(string)) != 64 {
		t.Fatalf("digest %v", f["digest"])
	}
	return f
}

func TestSymbolContextCarriesTheIaCBlock(t *testing.T) {
	iacHome(t, true)
	root := iacRepo(t, map[string]string{"main.tf": iacMain})
	p, _ := iacCall(t, iacServer(root), "symbol_context", map[string]any{"service": "svc", "symbol": "widget_thing.a"})
	iac := iacBlock(t, p)
	b := iac["binding"].(map[string]any)
	if b["status"] != "constraint-matched" || b["version"] != "1.2.0" || b["source"] == "" {
		t.Fatalf("binding %v", b)
	}
	citedFact(t, iac["contract"])
	unset := iac["required_unset"].([]any)
	var paths []string
	for _, u := range unset {
		f := u.(map[string]any)
		paths = append(paths, f["path"].(string))
		citedFact(t, f["fact"])
	}
	if strings.Join(paths, ",") != "name,rule[*].port" {
		t.Fatalf("required_unset %v", paths)
	}
	flags := map[string]any{}
	for _, w := range iac["written"].([]any) {
		m := w.(map[string]any)
		flags[m["path"].(string)] = m
	}
	if old := flags["old"].(map[string]any); old["known"] != true || !strings.Contains(fmt.Sprint(old["flags"]), "deprecated") {
		t.Fatalf("old %v", old)
	}
	if bogus := flags["bogus"].(map[string]any); bogus["known"] != false {
		t.Fatalf("bogus %v", bogus)
	}
	var kinds []string
	for _, f := range iac["findings"].([]any) {
		kinds = append(kinds, f.(map[string]any)["kind"].(string))
		citedFact(t, f.(map[string]any)["fact"])
	}
	if strings.Join(kinds, ",") != "unknown-attribute,deprecated" && strings.Join(kinds, ",") != "deprecated,unknown-attribute" {
		t.Fatalf("findings %v", kinds)
	}
}

// The reusable module plus examples layout, reached through the product's own surface.
func TestSymbolContextIaCReusableModuleWithExamples(t *testing.T) {
	iacHome(t, true)
	files := func(extra map[string]string) map[string]string {
		m := map[string]string{"main.tf": iacMain, "examples/basic/main.tf": `module "m" { source = "../.." }`}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	for _, c := range []struct {
		name   string
		extra  map[string]string
		status string
		ver    string
	}{
		{"no lockfile reaches the module", nil, "constraint-matched", "1.2.0"},
		{"one example locks it", map[string]string{"examples/basic/.terraform.lock.hcl": iacLock("1.2.0")}, "locked", "1.2.0"},
		{"two examples disagree", map[string]string{
			"examples/basic/.terraform.lock.hcl": iacLock("1.2.0"),
			"examples/other/main.tf":             `module "m" { source = "../.." }`,
			"examples/other/.terraform.lock.hcl": iacLock("1.3.0"),
		}, "ambiguous", ""},
		{"the lock pins a version no catalog holds", map[string]string{"examples/basic/.terraform.lock.hcl": iacLock("1.9.0")}, "unbound", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := iacRepo(t, files(c.extra))
			p, _ := iacCall(t, iacServer(root), "symbol_context", map[string]any{"service": "svc", "symbol": "widget_thing.a"})
			iac := iacBlock(t, p)
			b := iac["binding"].(map[string]any)
			if b["status"] != c.status || (c.ver != "" && b["version"] != c.ver) || (c.ver == "" && b["version"] != nil) {
				t.Fatalf("binding %v", b)
			}
			if c.status == "locked" || c.status == "constraint-matched" {
				return
			}
			for _, k := range []string{"contract", "required_unset", "written", "findings"} {
				if iac[k] != nil {
					t.Fatalf("%s binding must carry no facts, has %s: %v", c.status, k, iac[k])
				}
			}
			if c.status == "unbound" && b["add_line"] != "1" && b["add_line"] != "1.x" {
				t.Fatalf("unbound names the line to add: %v", b)
			}
		})
	}
}

// With no catalog installed both surfaces answer exactly as they do without this feature.
func TestIaCSurfacesUnchangedWithoutACatalog(t *testing.T) {
	root := iacRepo(t, map[string]string{"main.tf": iacMain})
	sym := map[string]any{"service": "svc", "symbol": "widget_thing.a"}
	rev := map[string]any{"service": "svc", "changed_files": []string{"main.tf"}}

	iacHome(t, false) // FATHOM_HOME exists, its iac directory does not
	symNone, _ := iacCall(t, iacServer(root), "symbol_context", sym)
	revNone, _ := iacCall(t, iacServer(root), "review_diff", rev)
	if os.MkdirAll(filepath.Join(os.Getenv("FATHOM_HOME"), "iac"), 0o755) != nil {
		t.Fatal("mkdir")
	}
	symEmpty, _ := iacCall(t, iacServer(root), "symbol_context", sym)
	revEmpty, _ := iacCall(t, iacServer(root), "review_diff", rev)
	for name, p := range map[string]map[string]any{"symbol none": symNone, "review none": revNone, "symbol empty": symEmpty, "review empty": revEmpty} {
		if _, has := p["iac"]; has {
			t.Fatalf("%s: an iac key appeared with no catalog: %v", name, p["iac"])
		}
	}
	if !reflect.DeepEqual(symNone, symEmpty) || !reflect.DeepEqual(revNone, revEmpty) {
		t.Fatal("a missing and an empty catalog directory must answer alike")
	}

	// A catalog adds the iac key and changes nothing else.
	iacHome(t, true)
	symWith, _ := iacCall(t, iacServer(root), "symbol_context", sym)
	revWith, _ := iacCall(t, iacServer(root), "review_diff", rev)
	iacBlock(t, symWith)
	iacBlock(t, revWith)
	delete(symWith, "iac")
	delete(revWith, "iac")
	if !reflect.DeepEqual(symNone, symWith) {
		t.Fatalf("symbol_context changed beyond the iac key:\n%v\n%v", symNone, symWith)
	}
	if !reflect.DeepEqual(stripVolatile(revNone), stripVolatile(revWith)) {
		t.Fatalf("review_diff changed beyond the iac key:\n%v\n%v", revNone, revWith)
	}
}

// stripVolatile drops what differs between two identical runs.
func stripVolatile(p map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range p {
		out[k] = v
	}
	delete(out, "elided")
	return out
}

func TestReviewDiffCarriesAttributeFindings(t *testing.T) {
	iacHome(t, true)
	root := iacRepo(t, map[string]string{"main.tf": iacMain, "other/readme.md": "x"})
	p, _ := iacCall(t, iacServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"main.tf", "other/readme.md"}})
	iac := iacBlock(t, p)
	got := map[string]bool{}
	for _, f := range iac["findings"].([]any) {
		m := f.(map[string]any)
		got[m["kind"].(string)+" "+m["path"].(string)] = true
		citedFact(t, m["fact"])
		if m["binding"] != "constraint-matched" {
			t.Fatalf("finding %v", m)
		}
	}
	for _, want := range []string{"unknown-attribute bogus", "deprecated old", "required-unset name", "required-unset rule[*].port", "sensitive-output widget_thing.a.token"} {
		if !got[want] {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	if iac["blobs_read"] != float64(1) {
		t.Errorf("blobs_read %v", iac["blobs_read"])
	}
	// A change to no Terraform file adds no section.
	p, _ = iacCall(t, iacServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"other/readme.md"}})
	if _, has := p["iac"]; has {
		t.Fatalf("iac section for a non-Terraform change: %v", p["iac"])
	}
}

// The findings sit under review_diff's existing response cap: a thousand findings are trimmed, and the trim is said.
func TestReviewDiffIaCFindingsStayUnderTheByteCap(t *testing.T) {
	iacHome(t, true)
	var sb strings.Builder
	sb.WriteString(iacProviders)
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "resource \"widget_thing\" \"r%03d\" {\n  bogus_%03d = 1\n}\n", i, i)
	}
	root := iacRepo(t, map[string]string{"big.tf": sb.String()})
	p, size := iacCall(t, iacServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"big.tf"}})
	if size > 16000 {
		t.Fatalf("the response is %d bytes, over the 16000 cap", size)
	}
	iac := iacBlock(t, p)
	n := len(iac["findings"].([]any))
	if n == 0 || n >= 800 {
		t.Fatalf("%d findings returned; want some, trimmed", n)
	}
	el, ok := p["elided"].(map[string]any)
	if !ok {
		t.Fatalf("the trim must be disclosed: %v", p["elided"])
	}
	if _, ok := el["fields"].(map[string]any)["/iac/findings"]; !ok {
		t.Fatalf("the elision names /iac/findings: %v", el)
	}
}

// A link out of the corpus is not followed.
func TestIaCReadsRefuseASymlinkOutOfTheCorpus(t *testing.T) {
	iacHome(t, true)
	outside := iacRepo(t, map[string]string{"evil.tf": `resource "widget_thing" "z" { leaked = 1 }`})
	root := iacRepo(t, map[string]string{"main.tf": iacMain})
	if err := os.Symlink(filepath.Join(outside, "evil.tf"), filepath.Join(root, "link.tf")); err != nil {
		t.Skip("symlinks unavailable")
	}
	p, _ := iacCall(t, iacServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"main.tf", "link.tf"}})
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "leaked") {
		t.Fatalf("a file outside the corpus was read: %s", b)
	}
}
