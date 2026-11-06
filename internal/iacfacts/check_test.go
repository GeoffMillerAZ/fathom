package iacfacts

import (
	"encoding/json"
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

// IF-8: a list or set of objects is an attribute that tofu also accepts as blocks, static or dynamic. That is not a
// wrong form. A scalar or a list of scalars written as a block still is.
func TestCheckAttributeUseAcceptsAttributesAsBlocks(t *testing.T) {
	res := map[string]any{"version": 1, "block": map[string]any{
		"attributes": map[string]any{
			"ingress": fxAttr(map[string]any{"type": []any{"set", []any{"object", map[string]any{"port": "number"}}}, "computed": true}),
			"egress":  fxAttr(map[string]any{"type": []any{"list", []any{"object", map[string]any{"port": "number"}}}}),
			"size":    fxAttr(map[string]any{"type": "number"}),
			"names":   fxAttr(map[string]any{"type": []any{"list", "string"}}),
			"tags":    fxAttr(map[string]any{"type": []any{"map", "string"}}),
		},
	}}
	cats, list := chkCatalogs(t, map[string]any{"widget_thing": res})
	repo := fstest.MapFS{"main.tf": file(chkProviders + `
resource "widget_thing" "a" {
  ingress {
    port = 1
  }
  ingress {
    port = 2
  }
  dynamic "egress" {
    for_each = [1]
    content {
      port = egress.value
    }
  }
}
resource "widget_thing" "b" {
  size {
    x = 1
  }
  names {
    x = 1
  }
  tags {
    x = 1
  }
}
`)}
	got := chkModule(t, repo, cats, list, IaCModuleQuery{})
	byBlock := map[string]IaCBlockCheck{}
	for _, b := range got.Blocks {
		byBlock[b.Block] = b
	}
	if a := byBlock["widget_thing.a"]; len(a.Check.Findings) != 0 {
		t.Errorf("attributes written as blocks are no finding: %+v", a.Check.Findings)
	}
	for _, p := range []string{"size[*].x", "names[*].x", "tags[*].x"} {
		if !hasFinding(byBlock["widget_thing.b"].Check, FindingUnknownAttribute, p) {
			t.Errorf("%s: no wrong-form finding in %+v", p, byBlock["widget_thing.b"].Check.Findings)
		}
	}
}

// IF-8: a required list or set of objects written as blocks, static or dynamic, is written. Block form writes only
// rule[*].member paths, so the attribute path itself never appears.
func TestCheckAttributeUseRequiredAttributeAsBlocks(t *testing.T) {
	obj := []any{"object", map[string]any{"port": "number"}}
	res := map[string]any{"version": 1, "block": map[string]any{
		"attributes": map[string]any{
			"ingress": fxAttr(map[string]any{"type": []any{"set", obj}, "required": true, "optional": false}),
			"egress":  fxAttr(map[string]any{"type": []any{"list", obj}, "required": true, "optional": false}),
		},
	}}
	cats, list := chkCatalogs(t, map[string]any{"widget_thing": res})
	repo := fstest.MapFS{"main.tf": file(chkProviders + `
resource "widget_thing" "a" {
  ingress {
    port = 1
  }
  dynamic "egress" {
    for_each = [1]
    content {
      port = egress.value
    }
  }
}
resource "widget_thing" "b" {
}
`)}
	got := chkModule(t, repo, cats, list, IaCModuleQuery{})
	byBlock := map[string]IaCBlockCheck{}
	for _, b := range got.Blocks {
		byBlock[b.Block] = b
	}
	if a := byBlock["widget_thing.a"]; len(a.Check.Findings) != 0 {
		t.Errorf("required attributes written as blocks are set: %+v", a.Check.Findings)
	}
	for _, p := range []string{"ingress", "egress"} {
		if !hasFinding(byBlock["widget_thing.b"].Check, FindingRequiredUnset, p) {
			t.Errorf("%s: an unwritten required attribute stays a finding: %+v", p, byBlock["widget_thing.b"].Check.Findings)
		}
	}
}

// IF-8: a member of a block-form attribute is judged against the attribute's object type, as tofu does: a member
// the type names reads known with only the flags the type states, a member it does not name is an unknown
// attribute citing the type, and a deprecated attribute written as blocks is a deprecated finding.
func TestCheckAttributeUseBlockFormMembers(t *testing.T) {
	obj := []any{"object", map[string]any{"port": "number", "proto": "string"}, []any{"proto"}}
	res := map[string]any{"version": 1, "block": map[string]any{
		"attributes": map[string]any{
			"ingress": fxAttr(map[string]any{"type": []any{"set", obj}, "computed": true}),
			"old":     fxAttr(map[string]any{"type": []any{"list", []any{"object", map[string]any{"x": "number"}}}, "deprecated": true}),
		},
	}}
	cats, list := chkCatalogs(t, map[string]any{"widget_thing": res})
	repo := fstest.MapFS{"main.tf": file(chkProviders + `
resource "widget_thing" "a" {
  ingress {
    port         = 1
    proto        = "tcp"
    not_a_field  = 2
  }
  old {
    x = 1
  }
}
`)}
	got := chkModule(t, repo, cats, list, IaCModuleQuery{})
	if len(got.Blocks) != 1 {
		t.Fatalf("blocks: %+v", got.Blocks)
	}
	c := got.Blocks[0].Check
	written := map[string]IaCWrittenAttr{}
	for _, w := range c.Written {
		written[w.Path] = w
	}
	if w := written["ingress[*].port"]; !w.Known || len(w.Flags) != 0 || w.Fact == nil || !strings.HasSuffix(w.Fact.Path, "attributes.ingress.type") {
		t.Errorf("a member the type names reads known with no borrowed flags: %+v", w)
	}
	if w := written["ingress[*].proto"]; !w.Known || strings.Join(w.Flags, ",") != "optional" {
		t.Errorf("an optional object member reads optional: %+v", w)
	}
	if w := written["ingress[*].not_a_field"]; w.Known || w.Fact != nil {
		t.Errorf("a misspelled member reads unknown: %+v", w)
	}
	fs := findingsOf(c, FindingUnknownAttribute)
	if len(fs) != 1 || fs[0].Path != "ingress[*].not_a_field" || !strings.HasSuffix(fs[0].Fact.Path, "attributes.ingress.type") {
		t.Errorf("unknown-attribute findings: %+v", fs)
	}
	if !hasFinding(c, FindingDeprecated, "old[*]") && !hasFinding(c, FindingDeprecated, "old") {
		t.Errorf("a deprecated attribute written as blocks is deprecated: %+v", c.Findings)
	}
}

// F2-depth: every segment of a block-form path is judged against the type tree, not only the first member. The
// types are shaped like the aws ones tofu validate rejects at depth: a member the object does not have, a list of
// strings written as a block, and a map written as a block.
func TestCheckAttributeUseBlockFormDepth(t *testing.T) {
	str := "string"
	jar := []any{"object", map[string]any{
		"jar":        str,
		"args":       []any{"list", str},
		"properties": []any{"map", str},
		"inner":      []any{"list", []any{"object", map[string]any{"deep": str}}},
	}, []any{"args", "properties"}}
	step := []any{"object", map[string]any{
		"name":            str,
		"hadoop_jar_step": []any{"list", jar},
	}}
	ing := []any{"object", map[string]any{"cidr_blocks": []any{"list", str}, "from_port": "number"}}
	creds := []any{"object", map[string]any{"user": str}}
	res := map[string]any{"version": 1, "block": map[string]any{
		"attributes": map[string]any{
			"step":    fxAttr(map[string]any{"type": []any{"set", step}}),
			"ingress": fxAttr(map[string]any{"type": []any{"set", ing}}),
			"creds":   fxAttr(map[string]any{"type": []any{"set", creds}, "sensitive": true, "write_only": true}),
		},
	}}
	cats, list := chkCatalogs(t, map[string]any{"widget_thing": res})
	repo := fstest.MapFS{"main.tf": file(chkProviders + `
resource "widget_thing" "ok" {
  step {
    name = "a"
    hadoop_jar_step {
      jar = "j"
      inner {
        deep = "d"
      }
    }
  }
  ingress {
    from_port = 1
  }
  creds {
    user = "u"
  }
}
resource "widget_thing" "bad" {
  step {
    hadoop_jar_step {
      bogus_nested = 1
    }
  }
  step {
    hadoop_jar_step {
      properties {
        a = "b"
      }
    }
  }
  step {
    hadoop_jar_step {
      inner {
        nope = 1
      }
    }
  }
  ingress {
    cidr_blocks {
      x = 1
    }
  }
}
`)}
	got := chkModule(t, repo, cats, list, IaCModuleQuery{})
	byBlock := map[string]IaCBlockCheck{}
	for _, b := range got.Blocks {
		byBlock[b.Block] = b
	}
	ok := byBlock["widget_thing.ok"].Check
	if len(ok.Findings) != 0 {
		t.Errorf("valid nested block form is no finding: %+v", ok.Findings)
	}
	written := map[string]IaCWrittenAttr{}
	for _, w := range ok.Written {
		written[w.Path] = w
	}
	for _, p := range []string{"step[*].name", "step[*].hadoop_jar_step[*].jar", "step[*].hadoop_jar_step[*].inner[*].deep", "ingress[*].from_port"} {
		if w, found := written[p]; !found || !w.Known || w.Fact == nil {
			t.Errorf("%s: a member the type names reads known: %+v", p, w)
		}
	}
	if w := written["creds[*].user"]; !w.Known || strings.Join(w.Flags, ",") != "sensitive,write_only" {
		t.Errorf("a member of a sensitive write-only value carries both flags: %+v", w)
	}
	bad := byBlock["widget_thing.bad"].Check
	badWritten := map[string]IaCWrittenAttr{}
	for _, w := range bad.Written {
		badWritten[w.Path] = w
	}
	for _, p := range []string{
		"step[*].hadoop_jar_step[*].bogus_nested",
		"step[*].hadoop_jar_step[*].properties[*].a",
		"step[*].hadoop_jar_step[*].inner[*].nope",
		"ingress[*].cidr_blocks[*].x",
	} {
		if w := badWritten[p]; w.Known || w.Fact != nil {
			t.Errorf("%s: tofu rejects it, so it must not read known: %+v", p, w)
		}
		if !hasFinding(bad, FindingUnknownAttribute, p) {
			t.Errorf("%s: no unknown-attribute finding in %+v", p, bad.Findings)
		}
	}
	for _, f := range findingsOf(bad, FindingUnknownAttribute) {
		if !strings.Contains(f.Fact.Path, "attributes.") || !strings.HasSuffix(f.Fact.Path, ".type") {
			t.Errorf("%s: the finding cites the type path, got %q", f.Path, f.Fact.Path)
		}
	}
}

// IF-8: tofu accepts block syntax only for a list or set whose element type is an object, so a member of cty type
// dynamic written as a block is in the wrong form like any other member that is not a list or set of objects. A
// dynamic member written as an argument stays known: any value fits it.
func TestCheckAttributeUseBlockFormDynamicMemberIsWrongForm(t *testing.T) {
	cfg := []any{"object", map[string]any{"dyn": "dynamic", "name": "string"}}
	res := map[string]any{"version": 1, "block": map[string]any{
		"attributes": map[string]any{"cfg": fxAttr(map[string]any{"type": []any{"set", cfg}})},
	}}
	cats, list := chkCatalogs(t, map[string]any{"widget_thing": res})
	repo := fstest.MapFS{"main.tf": file(chkProviders + `
resource "widget_thing" "bad" {
  cfg {
    dyn {
      y = 1
    }
  }
}
resource "widget_thing" "ok" {
  cfg {
    dyn = 1
  }
}
`)}
	got := chkModule(t, repo, cats, list, IaCModuleQuery{})
	byBlock := map[string]IaCBlockCheck{}
	for _, b := range got.Blocks {
		byBlock[b.Block] = b
	}
	bad := byBlock["widget_thing.bad"].Check
	for _, w := range bad.Written {
		if w.Path == "cfg[*].dyn[*].y" && w.Known {
			t.Errorf("a dynamic member written as a block must not read known: %+v", w)
		}
	}
	fs := findingsOf(bad, FindingUnknownAttribute)
	if len(fs) != 1 || fs[0].Path != "cfg[*].dyn[*].y" || !strings.Contains(fs[0].Message, "wrong form") {
		t.Errorf("a dynamic member written as a block is wrong form: %+v", fs)
	}
	ok := byBlock["widget_thing.ok"].Check
	if len(ok.Findings) != 0 {
		t.Errorf("a dynamic member written as an argument is no finding: %+v", ok.Findings)
	}
	for _, w := range ok.Written {
		if w.Path == "cfg[*].dyn" && !w.Known {
			t.Errorf("a dynamic member written as an argument reads known: %+v", w)
		}
	}
}

// nestedIaCType spells a cty type that nests list-of-object levels deep, as JSON.
func nestedIaCType(levels int) []any {
	t := []any{"object", map[string]any{"leaf": "string"}}
	for i := 0; i < levels; i++ {
		t = []any{"object", map[string]any{"m": []any{"list", t}}}
	}
	return t
}

// A type nested past the decode cap has its tail unstated: the cap reads as nil, not as the cty type dynamic, so
// one value never means both "the provider said dynamic" and "this was cut off".
func TestParseIaCTypeDepthCapStatesNothing(t *testing.T) {
	raw, err := json.Marshal(nestedIaCType(80))
	if err != nil {
		t.Fatal(err)
	}
	ty := parseIaCType(raw)
	var walk func(*IaCType, int) int
	walk = func(ty *IaCType, depth int) int {
		if ty == nil {
			return depth
		}
		if ty.Kind == "dynamic" {
			t.Fatalf("the cap produced a dynamic type at depth %d", depth)
		}
		switch {
		case ty.Kind == "object" && ty.Members != nil:
			return walk(ty.Members["m"].Type, depth+1)
		case ty.Elem != nil || ty.Kind == "list":
			return walk(ty.Elem, depth+1)
		}
		return depth
	}
	if d := walk(ty, 0); d > maxIaCTypeDepth+2 {
		t.Errorf("walk ended at %d, past the cap", d)
	}
}
