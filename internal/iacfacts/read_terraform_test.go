package iacfacts

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func attrOf(t *testing.T, r Resource, path string) Attr {
	t.Helper()
	for _, a := range r.Attrs {
		if a.Path == path {
			return a
		}
	}
	t.Fatalf("no attribute %q in %v", path, attrPaths(r))
	return Attr{}
}

func attrPaths(r Resource) []string {
	var out []string
	for _, a := range r.Attrs {
		out = append(out, a.Path)
	}
	return out
}

func TestReadTerraformFileClasses(t *testing.T) {
	src := `
resource "aws_s3_bucket" "logs" {
  bucket    = "my-logs"
  a         = var.x
  b         = ["a", var.x]
  c         = "pre-${var.x}"
  d         = true ? "a" : "b"
  e         = upper("x")
  f         = aws_s3_bucket.other.id
  g         = "${var.x}"
  h         = ["a", "b"]
  i         = { Name = "x", Env = "y" }
  j         = { Name = var.x }
  k         = [aws_s3_bucket.a.id, aws_s3_bucket.b.id]
  l         = aws_instance.web[*].id
  m         = local.name
  n         = data.aws_caller_identity.me.account_id
  o         = module.net.vpc_id
  p         = 8080
  q         = true
  r         = null
  s         = var.x[var.y]
  t         = [for s in var.list : upper(s)]
  u         = ""
  v         = (var.x)
  provider  = aws.west
  count     = var.n
  depends_on = [aws_iam_role.r]

  versioning {
    enabled = true
  }
  rule {
    id = "one"
  }
  rule {
    id = "two"
  }
  dynamic "grant" {
    for_each = var.grants
    content {
      id = grant.value.id
      p  = "read"
    }
  }
  lifecycle {
    prevent_destroy = true
    ignore_changes  = [tags, ami]
  }
  provisioner "local-exec" {
    command = "echo secret"
  }
}

data "aws_ami" "a" {
  most_recent = true
}

module "net" {
  source   = "./net"
  for_each = toset(["a"])
  cidr     = "10.0.0.0/16"
}

variable "x" {
  default = "d"
}
`
	set, err := ReadTerraformFile("main.tf", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if set.SyntaxErrors != 0 {
		t.Fatalf("syntax errors %d", set.SyntaxErrors)
	}
	if len(set.Resources) != 3 {
		t.Fatalf("resources %d", len(set.Resources))
	}
	// Sorted by span.
	if set.Resources[0].Mode != ModeManaged || set.Resources[1].Mode != ModeData || set.Resources[2].Mode != ModeModule {
		t.Fatalf("modes %v %v %v", set.Resources[0].Mode, set.Resources[1].Mode, set.Resources[2].Mode)
	}
	b := set.Resources[0]
	if b.Provider != "aws.west" || b.Type != "aws_s3_bucket" || b.Name != "logs" || b.Tool != ToolTerraform {
		t.Fatalf("header %+v", b)
	}
	if b.Multiplicity.Kind != MultiCount || b.Multiplicity.Class != ClassReference || !reflect.DeepEqual(b.Multiplicity.Refs, []string{"var.n"}) {
		t.Fatalf("multiplicity %+v", b.Multiplicity)
	}
	for _, c := range []struct {
		path string
		want Class
		refs []string
	}{
		{"bucket", ClassLiteral, nil},
		{"a", ClassReference, []string{"var.x"}},
		{"b", ClassExpression, []string{"var.x"}},
		{"c", ClassExpression, []string{"var.x"}},
		{"d", ClassExpression, nil},
		{"e", ClassExpression, nil},
		{"f", ClassReference, []string{"aws_s3_bucket.other.id"}},
		{"g", ClassReference, []string{"var.x"}},
		{"h", ClassLiteral, nil},
		{"i", ClassLiteral, nil},
		{"j", ClassReference, []string{"var.x"}},
		{"k", ClassReference, []string{"aws_s3_bucket.a.id", "aws_s3_bucket.b.id"}},
		{"l", ClassReference, []string{"aws_instance.web"}},
		{"m", ClassReference, []string{"local.name"}},
		{"n", ClassReference, []string{"data.aws_caller_identity.me.account_id"}},
		{"o", ClassReference, []string{"module.net.vpc_id"}},
		{"p", ClassLiteral, nil},
		{"q", ClassLiteral, nil},
		{"r", ClassLiteral, nil},
		{"s", ClassExpression, []string{"var.x", "var.y"}},
		{"t", ClassExpression, []string{"var.list"}},
		{"u", ClassLiteral, nil},
		{"v", ClassReference, []string{"var.x"}},
		{"depends_on", ClassReference, []string{"aws_iam_role.r"}},
		{"versioning[*].enabled", ClassLiteral, nil},
		{"grant[*]", ClassReference, []string{"var.grants"}},
		{"grant[*].id", ClassExpression, nil},
		{"grant[*].p", ClassLiteral, nil},
		{"lifecycle.prevent_destroy", ClassLiteral, nil},
		{"lifecycle.ignore_changes", ClassLiteral, nil},
	} {
		a := attrOf(t, b, c.path)
		if a.Class != c.want || !reflect.DeepEqual(a.Refs, c.refs) {
			t.Errorf("%s: class %s refs %v, want %s %v", c.path, a.Class, a.Refs, c.want, c.refs)
		}
		if a.Value != nil {
			t.Errorf("%s: value kept without WithValues", c.path)
		}
		if a.Span.StartLine == 0 || a.Span.File != "main.tf" {
			t.Errorf("%s: span %+v", c.path, a.Span)
		}
	}
	// Meta-arguments and provisioners are not attributes.
	for _, p := range attrPaths(b) {
		if p == "count" || p == "provider" || strings.HasPrefix(p, "provisioner") {
			t.Errorf("unexpected attribute %q", p)
		}
	}
	// Every nested block takes [*], one block or several; the resource's own lifecycle block does not.
	var ids int
	for _, a := range b.Attrs {
		if a.Path == "rule[*].id" {
			ids++
		}
	}
	if ids != 2 {
		t.Errorf("rule[*].id read %d times", ids)
	}
	// Module call: a relation with for_each, never expanded.
	m := set.Resources[2]
	if m.Mode != ModeModule || m.Name != "net" || m.Multiplicity.Kind != MultiForEach || m.Multiplicity.Class != ClassExpression {
		t.Fatalf("module %+v", m)
	}
	if attrOf(t, m, "source").Class != ClassLiteral || attrOf(t, m, "cidr").Class != ClassLiteral {
		t.Fatal("module inputs")
	}
	// A bare name inside an object is a key, not a reference.
	if len(attrOf(t, b, "i").Refs) != 0 {
		t.Fatal("object key read as reference")
	}
}

func TestReadTerraformValues(t *testing.T) {
	src := `resource "x" "y" {
  s = "a"
  n = 3
  f = 1.5
  b = true
  l = ["a", 2]
  o = { k = "v" }
  c = "pre-${"mid"}-post"
}`
	set, err := ReadTerraformFile("a.tf", []byte(src), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	r := set.Resources[0]
	for path, want := range map[string]any{
		"s": "a", "n": int64(3), "f": 1.5, "b": true,
		"l": []any{"a", int64(2)}, "o": map[string]any{"k": "v"}, "c": "pre-mid-post",
	} {
		if got := attrOf(t, r, path).Value; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: value %#v, want %#v", path, got, want)
		}
	}
	set, err = ReadTerraformFile("a.tf", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range set.Resources[0].Attrs {
		if a.Value != nil {
			t.Errorf("%s keeps a value without WithValues", a.Path)
		}
	}
}

func TestReadTerraformModuleDefaulted(t *testing.T) {
	fsys := fstest.MapFS{
		"m/main.tf": {Data: []byte(`
resource "aws_s3_bucket" "b" {
  bucket = var.name
  acl    = var.acl
  tags   = local.tags
  region = local.region
  other  = local.mix
  req    = var.required
  list   = ["a", var.name]
  str    = "pre-${var.name}"
  cyc    = local.a
  call   = upper(var.name)
  mod    = module.child.out
}
module "child" {
  source = "./child"
  name   = var.name
}
`)},
		"m/vars.tf": {Data: []byte(`
variable "name" { default = "dflt" }
variable "acl" { default = "private" }
variable "required" {}
locals {
  base   = var.name
  tags   = { Name = local.base }
  region = "us-east-1"
  mix    = aws_s3_bucket.z.id
  a      = local.b
  b      = local.a
}
`)},
		"m/notes.txt":    {Data: []byte("ignored")},
		"m/sub/inner.tf": {Data: []byte(`resource "x" "y" {}`)},
	}
	set, err := ReadTerraformModule(fsys, "m", WithValues())
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Resources) != 2 {
		t.Fatalf("resources %d (a subdirectory must not be read)", len(set.Resources))
	}
	b := set.Resources[0]
	for _, c := range []struct {
		path string
		want Class
		val  any
	}{
		{"bucket", ClassDefaulted, "dflt"},
		{"acl", ClassDefaulted, "private"},
		{"tags", ClassDefaulted, map[string]any{"Name": "dflt"}},
		{"region", ClassLiteral, "us-east-1"},
		{"other", ClassReference, nil},
		{"req", ClassReference, nil},
		{"list", ClassDefaulted, []any{"a", "dflt"}},
		{"str", ClassDefaulted, "pre-dflt"},
		{"cyc", ClassUnknown, nil},
		{"call", ClassExpression, nil},
		{"mod", ClassReference, nil},
	} {
		a := attrOf(t, b, c.path)
		if a.Class != c.want {
			t.Errorf("%s: class %s, want %s", c.path, a.Class, c.want)
		}
		if !reflect.DeepEqual(a.Value, c.val) {
			t.Errorf("%s: value %#v, want %#v", c.path, a.Value, c.val)
		}
		if a.Span.File != "m/main.tf" {
			t.Errorf("%s: file %q", c.path, a.Span.File)
		}
	}
	// The module call is a relation: its input reads through the parent's default, and
	// nothing from ./child is read.
	m := set.Resources[1]
	if m.Mode != ModeModule || attrOf(t, m, "name").Class != ClassDefaulted {
		t.Fatalf("module call %+v", m)
	}
}

func TestReadTerraformDeterministic(t *testing.T) {
	src := []byte("resource \"a\" \"b\" {\n z = 1\n a = 2\n blk {\n y = 1\n }\n blk {\n y = 2\n }\n}\nresource \"a\" \"c\" {\n q = var.x\n}\n")
	first, err := ReadTerraformFile("a.tf", src)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, _ := ReadTerraformFile("a.tf", src)
		if !reflect.DeepEqual(first, again) {
			t.Fatal("reads differ")
		}
	}
	paths := attrPaths(first.Resources[0])
	if !reflect.DeepEqual(paths, []string{"a", "blk[*].y", "blk[*].y", "z"}) {
		t.Fatalf("paths %v", paths)
	}
}

func TestReadTerraformRefusals(t *testing.T) {
	deep := strings.Repeat("(", MaxDepth+2) + "1" + strings.Repeat(")", MaxDepth+2)
	for name, src := range map[string]string{
		"file over the cap": "# " + strings.Repeat("x", MaxFileBytes),
		"brackets too deep": `resource "a" "b" { x = ` + deep + ` }`,
		"blocks too deep":   `resource "a" "b" {` + strings.Repeat(" b {", MaxDepth) + strings.Repeat("}", MaxDepth) + `}`,
	} {
		_, err := ReadTerraformFile("a.tf", []byte(src))
		if !errors.Is(err, ErrLimit) {
			t.Errorf("%s: err %v, want ErrLimit", name, err)
		}
	}
}

func TestReadTerraformSyntaxErrorsAreCountedNotFatal(t *testing.T) {
	src := "resource \"a\" \"b\" {\n  ok = 1\n  bad = = =\n}\nresource \"a\" \"c\" {\n  also = 2\n}\n"
	set, err := ReadTerraformFile("a.tf", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if set.SyntaxErrors == 0 {
		t.Fatal("syntax error not counted")
	}
	if len(set.Resources) == 0 {
		t.Fatal("nothing read")
	}
}

func TestReadTerraformNoPanicOnJunk(t *testing.T) {
	for _, src := range []string{
		"", "{", "}", `resource`, `resource "a"`, `resource "a" "b"`, `resource "a" "b" {`,
		`module {}`, `data "a" {}`, `x = `, "\x00\xff\xfe", `resource "a" "b" { dynamic "x" {} }`,
		`resource "a" "b" { dynamic "x" { for_each = 1 content {} } }`,
		`resource "a" "b" { x = <<EOT
${var.x}
EOT
}`,
		`variable "x" { default = local.x }
locals { x = var.x }
resource "a" "b" { y = var.x }`,
	} {
		_, _ = ReadTerraformFile("j.tf", []byte(src))
	}
}

func TestReadTerraformModuleRefusals(t *testing.T) {
	big := make([]byte, MaxFileBytes+1)
	for i := range big {
		big[i] = '#'
	}
	fsys := fstest.MapFS{
		"d/a.tf": {Data: []byte(`resource "a" "b" {}`)},
		"d/z.tf": {Data: big},
	}
	if _, err := ReadTerraformModule(fsys, "d"); !errors.Is(err, ErrLimit) {
		t.Fatalf("err %v, want ErrLimit", err)
	}
	if _, err := ReadTerraformModule(fsys, "missing"); err == nil {
		t.Fatal("missing directory read")
	}
}

func TestErrorsCarryNoValues(t *testing.T) {
	const secret = "s3cr3t-sentinel"
	src := `resource "a" "b" { token = "` + secret + `"` + strings.Repeat(" b {", MaxDepth+1) + `}`
	_, err := ReadTerraformFile("a.tf", []byte(src))
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("err %v", err)
	}
}
