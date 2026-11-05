package iacfacts

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func classOf(t *testing.T, set *ResourceSet, path string) Attr {
	t.Helper()
	if len(set.Resources) == 0 {
		t.Fatalf("no resources: %+v", set)
	}
	return attrOf(t, set.Resources[0], path)
}

// OpenTofu reports constant_value -1 for -1, -1.5, [-1, 2] and {a = -1}. from_port = -1 is common, and reading it
// as an expression would count against the class-agreement bar.
func TestReadTerraformNegativeNumbersAreLiteral(t *testing.T) {
	src := "resource \"t\" \"n\" {\n  a = -1\n  b = -1.5\n  c = [-1, 2]\n  d = {x = -1}\n  e = \"p-${-1}\"\n}\n"
	set, err := ReadTerraformFile("a.tf", []byte(src), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	if a := classOf(t, set, "a"); a.Class != ClassLiteral || a.Value != int64(-1) {
		t.Errorf("a %+v", a)
	}
	if a := classOf(t, set, "b"); a.Class != ClassLiteral || a.Value != -1.5 {
		t.Errorf("b %+v", a)
	}
	if a := classOf(t, set, "c"); a.Class != ClassLiteral {
		t.Errorf("c %+v", a)
	} else if l, ok := a.Value.([]any); !ok || len(l) != 2 || l[0] != int64(-1) || l[1] != int64(2) {
		t.Errorf("c value %#v", a.Value)
	}
	if a := classOf(t, set, "d"); a.Class != ClassLiteral {
		t.Errorf("d %+v", a)
	} else if m, ok := a.Value.(map[string]any); !ok || m["x"] != int64(-1) {
		t.Errorf("d value %#v", a.Value)
	}
	if a := classOf(t, set, "e"); a.Class != ClassLiteral || a.Value != "p--1" {
		t.Errorf("e %+v", a)
	}
}

// These fold in OpenTofu too and read expression here. Section 5 exempts them from class agreement in writing.
func TestReadTerraformFoldableOperatorsStayExpression(t *testing.T) {
	src := "resource \"t\" \"n\" {\n  a = !true\n  b = 1 + 2\n  c = [for x in [\"a\"] : x]\n  d = -var.n\n  e = --1\n  f = -(1)\n}\n"
	set, err := ReadTerraformFile("a.tf", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"a", "b", "c", "d", "e", "f"} {
		if a := classOf(t, set, p); a.Class != ClassExpression {
			t.Errorf("%s: %+v", p, a)
		}
	}
}

// The parser recovers from a broken expression with a placeholder or a partial value. What it hands back says
// nothing about what was written, so the attribute reads unknown and the error is counted.
func TestReadTerraformBrokenExpressionsReadUnknown(t *testing.T) {
	cases := map[string]string{
		"close paren":   "resource \"t\" \"n\" {\n  x = )\n  y = 1\n}\n",
		"at sign":       "resource \"t\" \"n\" {\n  x = @true\n  y = 1\n}\n",
		"two operands":  "resource \"t\" \"n\" {\n  x = a b\n  y = 1\n}\n",
		"dangling plus": "resource \"t\" \"n\" {\n  x = 1 +\n  y = 1\n}\n",
	}
	for name, src := range cases {
		set, err := ReadTerraformFile("a.tf", []byte(src), WithValues())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if set.SyntaxErrors == 0 {
			t.Errorf("%s: not counted", name)
		}
		if a := classOf(t, set, "x"); a.Class != ClassUnknown || a.Value != nil {
			t.Errorf("%s: x %+v", name, a)
		}
		if a := classOf(t, set, "y"); a.Class != ClassLiteral {
			t.Errorf("%s: the neighbour must still read: %+v", name, a)
		}
	}
	// An unterminated string swallows what follows it: the attribute reads unknown, never "abc".
	for name, src := range map[string]string{
		"string":  "resource \"t\" \"n\" {\n  x = \"abc\n  y = 1\n}\n",
		"bracket": "resource \"t\" \"n\" {\n  x = [1, 2\n  y = 1\n}\n",
	} {
		set, err := ReadTerraformFile("a.tf", []byte(src), WithValues())
		if err != nil || set.SyntaxErrors == 0 {
			t.Fatalf("%s: %v %d", name, err, set.SyntaxErrors)
		}
		if a := classOf(t, set, "x"); a.Class != ClassUnknown || a.Value != nil {
			t.Errorf("%s: x %+v", name, a)
		}
	}
}

// One schema attribute has one spelling, whether a resource writes one block or two.
func TestReadTerraformBlockPathsAlwaysTakeStar(t *testing.T) {
	one, err := ReadTerraformFile("a.tf", []byte("resource \"t\" \"n\" {\n  subject {\n    common_name = \"a\"\n  }\n  lifecycle {\n    prevent_destroy = true\n  }\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := ReadTerraformFile("a.tf", []byte("resource \"t\" \"n\" {\n  subject {\n    common_name = \"a\"\n  }\n  subject {\n    common_name = \"b\"\n  }\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	for name, set := range map[string]*ResourceSet{"one": one, "two": two} {
		if a := classOf(t, set, "subject[*].common_name"); a.Class != ClassLiteral {
			t.Errorf("%s: %+v", name, a)
		}
	}
	classOf(t, one, "lifecycle.prevent_destroy")
}

func readModule(t *testing.T, files map[string]string, opts ...Option) *ResourceSet {
	t.Helper()
	m := fstest.MapFS{}
	for n, c := range files {
		m[n] = file(c)
	}
	set, err := ReadTerraformModule(m, ".", opts...)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// OpenTofu 1.13 (tofu show -json -config) loads only main.tofu when main.tf sits beside it: one resource, the
// .tofu value, and nothing from the .tf file.
func TestReadTerraformModuleTofuShadowsTf(t *testing.T) {
	set := readModule(t, map[string]string{
		"main.tf":    "resource \"terraform_data\" \"a\" {\n  input = 1\n}\nresource \"terraform_data\" \"b\" {\n  input = 1\n}\n",
		"main.tofu":  "resource \"terraform_data\" \"a\" {\n  input = 2\n}\n",
		"other.tf":   "resource \"terraform_data\" \"c\" {\n  input = 3\n}\n",
		"other.tofu": "# nothing here\n",
	}, WithValues())
	if len(set.Resources) != 1 || set.Resources[0].Name != "a" {
		t.Fatalf("%+v", set.Resources)
	}
	if a := classOf(t, set, "input"); a.Value != int64(2) || a.Span.File != "main.tofu" {
		t.Fatalf("%+v", a)
	}
}

// An override file adds no resource. tofu merges its attributes into the base; the reader does not guess the
// merge, so what an override names reads unknown and everything else keeps its read.
func TestReadTerraformModuleOverrides(t *testing.T) {
	set := readModule(t, map[string]string{
		"x.tf": `resource "terraform_data" "a" {
  input            = false
  triggers_replace = [1, 2]
  versioning {
    enabled    = true
    mfa_delete = false
  }
  tags = { k = var.v }
}
variable "v" {
  default = "a"
}
locals { l = "base" }
resource "terraform_data" "b" {
  input = local.l
}
resource "terraform_data" "c" {
  input = "kept"
}
`,
		"x_override.tf": `resource "terraform_data" "a" {
  input = true
  versioning {
    enabled = false
  }
  added = 1
}
variable "v" {
  default = "b"
}
locals { l = "over" }
resource "terraform_data" "nobase" {
  input = 1
}
`,
		"override.tofu": `resource "terraform_data" "a" {
  for_each = { k = 1 }
}
`,
	}, WithValues())
	if len(set.Resources) != 3 {
		t.Fatalf("one read per resource, and no resource from an override: %+v", set.Resources)
	}
	a := set.Resources[0]
	for _, p := range []string{"input", "versioning[*].enabled", "versioning[*].mfa_delete", "added", "tags"} {
		if at := attrOf(t, a, p); at.Class != ClassUnknown || at.Value != nil {
			t.Errorf("%s should read unknown: %+v", p, at)
		}
	}
	if at := attrOf(t, a, "triggers_replace"); at.Class != ClassLiteral {
		t.Errorf("triggers_replace is not named by an override: %+v", at)
	}
	if a.Multiplicity.Kind != MultiForEach || a.Multiplicity.Class != ClassUnknown {
		t.Errorf("multiplicity %+v", a.Multiplicity)
	}
	if at := attrOf(t, set.Resources[1], "input"); at.Class != ClassUnknown {
		t.Errorf("an overridden local reads unknown: %+v", at)
	}
	if at := attrOf(t, set.Resources[2], "input"); at.Class != ClassLiteral || at.Value != "kept" {
		t.Errorf("c %+v", at)
	}
}

// The reader withholds what its source marks secret, before any contract is bound: sensitive and ephemeral
// Terraform variables, CloudFormation NoEcho parameters, Pulumi fn::secret and secret configuration. The class
// stays, so a check still learns that a literal or a default is there.
func TestReadWithholdsSecretsUnderWithValues(t *testing.T) {
	tf := readModule(t, map[string]string{"main.tf": `variable "a" {
  default   = "hunter2"
  sensitive = true
}
variable "b" {
  default   = "hunter2"
  ephemeral = true
}
variable "c" {
  default   = "public"
  sensitive = false
}
locals { l = var.a }
resource "t" "n" {
  a = var.a
  b = var.b
  c = var.c
  d = local.l
  e = "x-${var.a}"
}
`}, WithValues())
	for p, want := range map[string]Class{"a": ClassDefaulted, "b": ClassDefaulted, "c": ClassDefaulted, "d": ClassDefaulted, "e": ClassDefaulted} {
		at := classOf(t, tf, p)
		if at.Class != want {
			t.Errorf("tf %s: class %s", p, at.Class)
		}
		if p == "c" {
			if at.Value != "public" {
				t.Errorf("tf c is not secret: %#v", at.Value)
			}
		} else if at.Value != nil {
			t.Errorf("tf %s kept a secret: %#v", p, at.Value)
		}
	}

	cf, err := ReadCloudFormationFile("t.yaml", []byte(`Parameters:
  Pw:
    Type: String
    NoEcho: true
    Default: hunter2
  Open:
    Type: String
    Default: public
Resources:
  B:
    Type: AWS::X::Y
    Properties:
      A: !Ref Pw
      B: !Ref Open
      C: !Sub "x-${Pw}"
`), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]any{"A": nil, "B": "public", "C": nil} {
		at := classOf(t, cf, p)
		if at.Class != ClassDefaulted && at.Class != ClassExpression {
			t.Errorf("cfn %s: class %s", p, at.Class)
		}
		if at.Value != want {
			t.Errorf("cfn %s: value %#v, want %#v", p, at.Value, want)
		}
	}

	pu, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(`configuration:
  pw:
    default: hunter2
    secret: true
  open:
    default: public
variables:
  s:
    fn::secret: hunter2
resources:
  r:
    type: aws:s3/bucket:Bucket
    properties:
      a: ${pw}
      b: ${open}
      c: ${s}
      d:
        fn::secret: hunter2
`), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]any{"a": nil, "b": "public", "c": nil, "d": nil} {
		if at := classOf(t, pu, p); at.Value != want {
			t.Errorf("pulumi %s: value %#v, want %#v (%s)", p, at.Value, want, at.Class)
		}
	}
}

// A shape the reader cannot walk is never silent: the resource gets one unknown attribute at the root, and a
// resource that is not read at all is named in Dropped.
func TestReadCloudFormationUnwalkableShapesAreNotSilent(t *testing.T) {
	set, err := ReadCloudFormationFile("t.yaml", []byte(`Mappings: &base {Type: AWS::X::Y}
Metadata: &anchor {k: v}
Resources:
  Cond:
    Type: AWS::X::Y
    Properties: !If [c, {A: 1}, {A: 2}]
  Alias:
    Type: AWS::X::Y
    Properties: *anchor
  Scalar:
    Type: AWS::X::Y
    Properties: text
  Empty:
    Type: AWS::X::Y
    Properties:
  Fine:
    Type: AWS::X::Y
    Properties: {A: 1}
  Fn::ForEach::Bs:
    - Name
    - [a, b]
    - {B: {Type: AWS::X::Y}}
  AliasResource: *base
  MergeOnly:
    <<: *base
  NotAMap: 3
`), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Resource{}
	for _, r := range set.Resources {
		byName[r.Name] = r
	}
	for _, n := range []string{"Cond", "Alias", "Scalar"} {
		r, ok := byName[n]
		if !ok || len(r.Attrs) != 1 || r.Attrs[0].Path != "" || r.Attrs[0].Class != ClassUnknown || r.Attrs[0].Value != nil {
			t.Errorf("%s: want one unknown attribute at the root, got %+v", n, r.Attrs)
		}
	}
	if len(byName["Empty"].Attrs) != 0 || len(byName["Fine"].Attrs) != 1 || byName["Fine"].Attrs[0].Path != "A" {
		t.Errorf("Empty and Fine: %+v %+v", byName["Empty"].Attrs, byName["Fine"].Attrs)
	}
	if set.SyntaxErrors != 3 {
		t.Errorf("the three unwalkable values count: %d", set.SyntaxErrors)
	}
	want := []string{"AliasResource", "Fn::ForEach::Bs", "MergeOnly", "NotAMap"}
	if !reflect.DeepEqual(set.Dropped, want) {
		t.Errorf("dropped %v, want %v", set.Dropped, want)
	}
}

func TestReadPulumiUnwalkableShapesAreNotSilent(t *testing.T) {
	set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(`variables:
  v:
    a: 1
resources:
  Interp:
    type: aws:s3/bucket:Bucket
    properties: ${v}
  Call:
    type: aws:s3/bucket:Bucket
    properties:
      fn::invoke:
        function: x
  Scalar:
    type: aws:s3/bucket:Bucket
    properties: text
  Fine:
    type: aws:s3/bucket:Bucket
    properties:
      a: 1
  NoType:
    properties:
      a: 1
  NotAMap: 3
`))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Resource{}
	for _, r := range set.Resources {
		byName[r.Name] = r
	}
	for _, n := range []string{"Interp", "Call", "Scalar"} {
		r := byName[n]
		if len(r.Attrs) != 1 || r.Attrs[0].Path != "" || r.Attrs[0].Class != ClassUnknown {
			t.Errorf("%s: %+v", n, r.Attrs)
		}
	}
	if set.SyntaxErrors != 3 || !reflect.DeepEqual(set.Dropped, []string{"NoType", "NotAMap"}) {
		t.Errorf("syntax %d dropped %v", set.SyntaxErrors, set.Dropped)
	}
}

// pulumi-yaml reads both configuration: and config:. A name that merely starts with pulumi is a resource, and
// only the pulumi namespace is not a reference.
func TestReadPulumiConfigSpellingsAndPulumiPrefix(t *testing.T) {
	set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(`config:
  region:
    default: us-east-1
configuration:
  zone:
    default: a
resources:
  pulumiThing:
    type: aws:s3/bucket:Bucket
  r:
    type: aws:s3/bucket:Bucket
    properties:
      a: ${region}
      b: ${zone}
      c:
        fn::join: ["-", ["${pulumiThing.id}", "${pulumi.stack}", "${pulumi}"]]
`), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	r := set.Resources[1]
	if a := attrOf(t, r, "a"); a.Class != ClassDefaulted || a.Value != "us-east-1" {
		t.Errorf("a %+v", a)
	}
	if a := attrOf(t, r, "b"); a.Class != ClassDefaulted || a.Value != "a" {
		t.Errorf("b %+v", a)
	}
	if a := attrOf(t, r, "c"); !reflect.DeepEqual(a.Refs, []string{"pulumiThing.id"}) {
		t.Errorf("c refs %v", a.Refs)
	}
}

// A repeated key counts, in YAML and in JSON, and so does a second YAML document.
func TestReadTemplatesCountDuplicateKeysAndExtraDocuments(t *testing.T) {
	yamlDup := "Resources:\n  B:\n    Type: AWS::X::Y\n    Properties:\n      A: 1\n      A: 2\n"
	set, err := ReadCloudFormationFile("t.yaml", []byte(yamlDup))
	if err != nil || set.SyntaxErrors != 1 {
		t.Errorf("yaml dup: %v %d", err, set.SyntaxErrors)
	}
	jsonDup := `{"Resources": {"B": {"Type": "AWS::X::Y", "Type": "AWS::X::Z", "Properties": {}}}}`
	set, err = ReadCloudFormationFile("t.json", []byte(jsonDup))
	if err != nil || set.SyntaxErrors != 1 {
		t.Errorf("json dup: %v %d", err, set.SyntaxErrors)
	}
	set, err = ReadCloudFormationFile("t.yaml", []byte("Resources:\n  B:\n    Type: AWS::X::Y\n---\nResources:\n  C:\n    Type: AWS::X::Y\n"))
	if err != nil || set.SyntaxErrors != 1 || len(set.Resources) != 1 || set.Resources[0].Name != "B" {
		t.Errorf("second document: %v %d %+v", err, set.SyntaxErrors, set.Resources)
	}
	set, err = ReadPulumiYAMLFile("Pulumi.yaml", []byte("resources:\n  r:\n    type: a:b:C\n    type: a:b:D\n"))
	if err != nil || set.SyntaxErrors != 1 {
		t.Errorf("pulumi dup: %v %d", err, set.SyntaxErrors)
	}
	// No false positives.
	set, err = ReadCloudFormationFile("t.yaml", []byte("Resources:\n  B:\n    Type: AWS::X::Y\n    Properties: {A: 1, B: 2}\n  C:\n    Type: AWS::X::Y\n"))
	if err != nil || set.SyntaxErrors != 0 {
		t.Errorf("clean: %v %d", err, set.SyntaxErrors)
	}
}

// A file with only CR line endings gave every span as 1:67-1:67.
func TestReadCloudFormationCROnlySpans(t *testing.T) {
	lf := "Resources:\n  B:\n    Type: AWS::X::Y\n    Properties:\n      A: 1\n      C: [x, y]\n"
	for name, src := range map[string]string{"lf": lf, "cr": strings.ReplaceAll(lf, "\n", "\r"), "crlf": strings.ReplaceAll(lf, "\n", "\r\n")} {
		set, err := ReadCloudFormationFile("t.yaml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		a := attrOf(t, set.Resources[0], "A")
		if a.Span.StartLine != 5 || a.Span.StartCol != 7 || a.Span.EndLine != 5 || a.Span.EndCol != 11 {
			t.Errorf("%s: A span %+v", name, a.Span)
		}
		c := attrOf(t, set.Resources[0], "C[*]")
		if c.Span.StartLine != 6 || c.Span.StartCol != 11 || c.Span.EndCol != 12 {
			t.Errorf("%s: C[*] span %+v", name, c.Span)
		}
	}
}

func TestReadTerraformEphemeralBlocksAndErrorFileName(t *testing.T) {
	set, err := ReadTerraformFile("e.tf", []byte("ephemeral \"random_password\" \"p\" {\n  length = 16\n}\n"))
	if err != nil || len(set.Resources) != 1 || set.Resources[0].Mode != ModeEphemeral || set.Resources[0].Type != "random_password" {
		t.Fatalf("%v %+v", err, set)
	}
	// An expression bound that trips names the file.
	var tf strings.Builder
	tf.WriteString("locals {\n  l0 = \"x\"\n")
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&tf, "  l%d = \"${local.l%d}${local.l%d}\"\n", i, i-1, i-1)
	}
	tf.WriteString("}\nresource \"t\" \"n\" {\n  x = local.l30\n}\n")
	_, err = ReadTerraformModule(fstest.MapFS{"main.tf": file(tf.String())}, ".")
	if !errors.Is(err, ErrLimit) || !strings.Contains(err.Error(), "main.tf") {
		t.Fatalf("want ErrLimit naming main.tf, got %v", err)
	}
}

// 40,000 resources and 10,000 references took 1.5 s with a linear name lookup. The size is bounded by the file cap.
func TestReadPulumiNameLookupIsConstantTime(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("resources:\n")
	const n = 30000
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "  r%d: {type: a:b:C, properties: {p: \"${r%d.id}\"}}\n", i, n-1-i)
	}
	within(t, 5*time.Second, "pulumi lookup", func() {
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(sb.String()))
		if err != nil || len(set.Resources) != n {
			t.Errorf("%v", err)
		}
	})
}
