package iacfacts

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

// chkResource is a made-up resource contract: name is required, token is sensitive, old is deprecated, and the rule
// block has a required port.
func chkResource() map[string]any {
	return map[string]any{"version": 1, "block": map[string]any{
		"attributes": map[string]any{
			"id":    fxAttr(map[string]any{"computed": true}),
			"name":  fxAttr(map[string]any{"required": true, "optional": false}),
			"size":  fxAttr(map[string]any{"type": "number"}),
			"token": fxAttr(map[string]any{"computed": true, "sensitive": true}),
			"old":   fxAttr(map[string]any{"deprecated": true}),
		},
		"block_types": map[string]any{
			"rule": map[string]any{"nesting_mode": "list", "block": map[string]any{
				"attributes": map[string]any{
					"port":  fxAttr(map[string]any{"type": "number", "required": true, "optional": false}),
					"proto": fxAttr(nil),
				},
			}},
		},
	}}
}

// chkCatalogs installs a catalog of example/widget 1.2.0 holding the given resource schemas and returns the
// directory and the listing.
func chkCatalogs(t testing.TB, resources map[string]any) (string, []IaCCatalogInfo) {
	t.Helper()
	dir := t.TempDir()
	exp := fxExport(t, fxSource, fxProvider(), resources, map[string]any{})
	p := fxCatalog(t, dir, DefaultIaCLimits(), map[string][]byte{"1.2.0": exp})
	cats := t.TempDir()
	if _, err := AddIaCCatalog(cats, p); err != nil {
		t.Fatal(err)
	}
	list, bad := ListIaCCatalogs(cats, false)
	if len(bad) > 0 || len(list) != 1 {
		t.Fatalf("catalogs: %v %v", list, bad)
	}
	return cats, list
}

const chkProviders = `
terraform {
  required_providers {
    widget = { source = "example/widget", version = "~> 1.0" }
  }
}
`

func chkModule(t testing.TB, repo fstest.MapFS, cats string, list []IaCCatalogInfo, q IaCModuleQuery) IaCModuleCheck {
	t.Helper()
	q.Repo, q.CatalogDir, q.Catalogs = repo, cats, list
	if q.Dir == "" {
		q.Dir = "."
	}
	out, err := CheckIaCModule(q)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func findingsOf(c IaCCheck, kind string) []IaCFinding {
	var out []IaCFinding
	for _, f := range c.Findings {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

func hasFinding(c IaCCheck, kind, path string) bool {
	for _, f := range findingsOf(c, kind) {
		if f.Path == path {
			return true
		}
	}
	return false
}

func TestCheckAttributeUseFindsTheFourFacts(t *testing.T) {
	cats, list := chkCatalogs(t, map[string]any{"widget_thing": chkResource()})
	repo := fstest.MapFS{"main.tf": file(chkProviders + `
resource "widget_thing" "a" {
  size  = 1
  bogus = "x"
  old   = "y"
  rule {
    proto = "tcp"
  }
}
resource "widget_thing" "b" {
  name = "b"
  rule {
    port = 1
  }
}
resource "widget_thing" "c" {
  name = widget_thing.b.nope
  size = length(widget_thing.b.old)
}
output "tok" {
  value = widget_thing.b.token
}
output "marked" {
  value     = widget_thing.b.token
  sensitive = true
}
output "unwrapped" {
  value = nonsensitive(widget_thing.b.token)
}
output "plain" {
  value = widget_thing.b.size
}
output "indexed" {
  value = widget_thing.b[0].token
}
`)}
	got := chkModule(t, repo, cats, list, IaCModuleQuery{Outputs: true})
	byBlock := map[string]IaCBlockCheck{}
	for _, b := range got.Blocks {
		byBlock[b.Block] = b
	}
	a := byBlock["widget_thing.a"]
	if a.Binding.Status != BindConstraintMatched || a.Binding.Version != "1.2.0" {
		t.Fatalf("binding %+v", a.Binding)
	}
	for _, want := range []struct{ kind, path string }{
		{FindingUnknownAttribute, "bogus"}, {FindingDeprecated, "old"},
		{FindingRequiredUnset, "name"}, {FindingRequiredUnset, "rule[*].port"},
	} {
		if !hasFinding(a.Check, want.kind, want.path) {
			t.Errorf("a: missing %s %s in %+v", want.kind, want.path, a.Check.Findings)
		}
	}
	if len(a.Check.Findings) != 4 {
		t.Errorf("a has %d findings, want exactly 4: %+v", len(a.Check.Findings), a.Check.Findings)
	}
	for _, f := range a.Check.Findings {
		if f.Fact.Catalog == "" || f.Fact.Version != "1.2.0" || len(f.Fact.Digest) != 64 || f.Fact.Path == "" || f.Binding != BindConstraintMatched {
			t.Errorf("a finding must cite catalog, version, digest and path: %+v", f)
		}
	}
	if f := findingsOf(a.Check, FindingRequiredUnset); len(f) == 2 {
		paths := map[string]string{f[0].Path: f[0].Fact.Path, f[1].Path: f[1].Fact.Path}
		if paths["name"] != "block.attributes.name" || paths["rule[*].port"] != "block.block_types.rule.block.attributes.port" {
			t.Errorf("fact paths %v", paths)
		}
	}
	if b := byBlock["widget_thing.b"]; len(b.Check.Findings) != 0 {
		t.Errorf("b is valid and carries %+v", b.Check.Findings)
	}
	c := byBlock["widget_thing.c"]
	if !hasFinding(c.Check, FindingUnknownAttribute, "widget_thing.b.nope") || !hasFinding(c.Check, FindingDeprecated, "widget_thing.b.old") {
		t.Errorf("c: traversal findings missing: %+v", c.Check.Findings)
	}
	for _, f := range c.Check.Findings {
		if f.Via != ViaTraversal {
			t.Errorf("c finding %+v should be a traversal", f)
		}
	}
	// Only the unmarked, unwrapped read of a sensitive attribute is flagged. A read through an index is judged too.
	var outs []string
	for _, f := range got.Outputs.Findings {
		if f.Kind != FindingSensitiveOutput || f.Via != ViaOutput || f.Fact.Path != "block.attributes.token" {
			t.Errorf("output finding %+v", f)
		}
		outs = append(outs, f.Block)
	}
	if strings.Join(outs, ",") != "output.indexed,output.tok" && strings.Join(outs, ",") != "output.tok,output.indexed" {
		t.Errorf("sensitive-output findings %v, want tok and indexed only", outs)
	}
}

func TestCheckAttributeUseWrittenAttrsCarryFlags(t *testing.T) {
	cats, list := chkCatalogs(t, map[string]any{"widget_thing": chkResource()})
	repo := fstest.MapFS{"main.tf": file(chkProviders + `
resource "widget_thing" "a" {
  name = "n"
  old  = var.x
  rule {
    port = 2
  }
}
`)}
	got := chkModule(t, repo, cats, list, IaCModuleQuery{})
	w := map[string]IaCWrittenAttr{}
	for _, x := range got.Blocks[0].Check.Written {
		w[x.Path] = x
	}
	if n := w["name"]; !n.Known || strings.Join(n.Flags, ",") != "required" || n.Fact == nil || n.Fact.Path != "block.attributes.name" {
		t.Errorf("name %+v", n)
	}
	if o := w["old"]; !o.Known || !strings.Contains(strings.Join(o.Flags, ","), "deprecated") || o.Class != ClassReference {
		t.Errorf("old %+v", o)
	}
	if p := w["rule[*].port"]; !p.Known || p.Fact == nil || p.Fact.Path != "block.block_types.rule.block.attributes.port" {
		t.Errorf("port %+v", p)
	}
}

// Unset-required is reported only for a bound contract, and a level whose attributes include an unknown says so.
func TestCheckAttributeUseUnsetRequiredNeedsABoundContract(t *testing.T) {
	cats, list := chkCatalogs(t, map[string]any{"widget_thing": chkResource()})
	src := chkProviders + `resource "widget_thing" "a" { size = 1 }`

	t.Run("no catalog installed", func(t *testing.T) {
		got := chkModule(t, fstest.MapFS{"main.tf": file(src)}, t.TempDir(), nil, IaCModuleQuery{})
		b := got.Blocks[0]
		if b.Binding.Status != BindUnbound || len(b.Check.Findings) != 0 || len(b.Check.Written) != 0 || b.Contract != nil || got.BlobsRead != 0 {
			t.Fatalf("an unbound block carries no facts: %+v", b)
		}
		if len(b.Check.Disclosures) == 0 {
			t.Fatalf("an unbound block says why it has no facts")
		}
	})
	t.Run("no catalog for the exact lockfile version", func(t *testing.T) {
		repo := fstest.MapFS{"main.tf": file(src), LockfileName: lock("registry.opentofu.org/example/widget", "1.9.0")}
		b := chkModule(t, repo, cats, list, IaCModuleQuery{}).Blocks[0]
		if b.Binding.Status != BindUnbound || len(b.Check.Findings) != 0 || b.Binding.Version != "" {
			t.Fatalf("a nearest version must never stand in: %+v", b)
		}
	})
	t.Run("ambiguous roots carry no facts", func(t *testing.T) {
		repo := fstest.MapFS{
			"main.tf":                    file(src),
			"examples/a/main.tf":         file(exampleCall),
			"examples/a/" + LockfileName: lock("registry.opentofu.org/example/widget", "1.2.0"),
			"examples/b/main.tf":         file(exampleCall),
			"examples/b/" + LockfileName: lock("registry.opentofu.org/example/widget", "1.3.0"),
		}
		b := chkModule(t, repo, cats, list, IaCModuleQuery{}).Blocks[0]
		if b.Binding.Status != BindAmbiguous || len(b.Check.Findings) != 0 {
			t.Fatalf("%+v", b)
		}
	})
	t.Run("a locked root through a module call", func(t *testing.T) {
		repo := fstest.MapFS{
			"main.tf":                    file(src),
			"examples/a/main.tf":         file(exampleCall),
			"examples/a/" + LockfileName: lock("registry.opentofu.org/example/widget", "1.2.0"),
		}
		b := chkModule(t, repo, cats, list, IaCModuleQuery{}).Blocks[0]
		if b.Binding.Status != BindLocked || b.Binding.Version != "1.2.0" || !hasFinding(b.Check, FindingRequiredUnset, "name") {
			t.Fatalf("%+v", b)
		}
		if f := findingsOf(b.Check, FindingRequiredUnset)[0]; f.Binding != BindLocked {
			t.Fatalf("a finding names its binding: %+v", f)
		}
	})
	t.Run("an unknown attribute silences the level and says so", func(t *testing.T) {
		repo := fstest.MapFS{
			"main.tf":     file(src),
			"override.tf": file(`resource "widget_thing" "a" { size = 2 }`),
		}
		b := chkModule(t, repo, cats, list, IaCModuleQuery{}).Blocks[0]
		if len(findingsOf(b.Check, FindingRequiredUnset)) != 0 {
			t.Fatalf("an unknown attribute may be absent, so nothing is reported unset: %+v", b.Check.Findings)
		}
		if !strings.Contains(strings.Join(b.Check.Disclosures, "|"), "reads unknown") {
			t.Fatalf("the skipped check must say so: %v", b.Check.Disclosures)
		}
	})
	t.Run("expression attributes are named", func(t *testing.T) {
		repo := fstest.MapFS{"main.tf": file(chkProviders + `resource "widget_thing" "a" {
  name = join("-", [var.a, var.b])
}`)}
		b := chkModule(t, repo, cats, list, IaCModuleQuery{}).Blocks[0]
		if len(b.Check.Findings) != 0 {
			t.Fatalf("a written expression satisfies a required argument: %+v", b.Check.Findings)
		}
		if !strings.Contains(strings.Join(b.Check.Disclosures, "|"), "expressions") {
			t.Fatalf("expression attributes must be disclosed: %v", b.Check.Disclosures)
		}
	})
}

func TestCheckIaCModuleBlobCap(t *testing.T) {
	res := map[string]any{}
	var sb strings.Builder
	sb.WriteString(chkProviders)
	for i := 0; i < MaxCheckBlobs+20; i++ {
		typ := fmt.Sprintf("widget_t%03d", i)
		res[typ] = chkResource()
		fmt.Fprintf(&sb, "resource %q \"x\" {}\n", typ)
	}
	cats, list := chkCatalogs(t, res)
	got := chkModule(t, fstest.MapFS{"main.tf": file(sb.String())}, cats, list, IaCModuleQuery{})
	// Blobs are content-addressed, so the catalog shares one blob across the types, but the answer counts reads.
	if got.BlobsRead != MaxCheckBlobs {
		t.Fatalf("read %d blobs, want the cap %d", got.BlobsRead, MaxCheckBlobs)
	}
	if !strings.Contains(strings.Join(got.Disclosures, "|"), "most one answer reads") {
		t.Fatalf("the cap must be disclosed: %v", got.Disclosures)
	}
	if n := len(got.Blocks[len(got.Blocks)-1].Check.Findings); n != 0 {
		t.Fatalf("a block past the cap carries no facts, has %d", n)
	}
}

func TestReadTerraformOutputs(t *testing.T) {
	repo := fstest.MapFS{
		"main.tf": file(`
output "a" { value = aws_db_instance.m.password }
output "b" {
  value     = sensitive(aws_db_instance.m.password)
}
output "c" {
  value     = aws_db_instance.m.password
  sensitive = false
}
output "d" {
  value = nonsensitive(aws_db_instance.m.password)
}
`),
		"extra_override.tf": file(`output "a" { sensitive = true }`),
		"x.tf.json":         file(`{}`),
	}
	set, err := ReadTerraformOutputs(repo, ".")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]IaCOutput{}
	for _, o := range set.Outputs {
		got[o.Name] = o
	}
	if o := got["a"]; o.Marked || strings.Join(o.Reads, ",") != "aws_db_instance.m.password" {
		t.Errorf("a %+v", o)
	}
	if o := got["b"]; !o.Marked || len(o.Reads) != 0 {
		t.Errorf("b %+v", o)
	}
	if o := got["c"]; o.Marked {
		t.Errorf("sensitive = false does not mark: %+v", o)
	}
	if o := got["d"]; o.Marked || len(o.Reads) != 0 {
		t.Errorf("d %+v", o)
	}
	if len(set.Skipped) != 2 {
		t.Errorf("the override and JSON files are named, got %v", set.Skipped)
	}
}
