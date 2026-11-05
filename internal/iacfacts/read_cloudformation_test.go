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

const cfnYAML = `AWSTemplateFormatVersion: "2010-09-09"
Parameters:
  Env:
    Type: String
    Default: dev
  Key:
    Type: String
  Ami:
    Type: AWS::SSM::Parameter::Value<String>
    Default: /path/to/ami
Conditions:
  IsProd: !Equals [!Ref Env, prod]
Resources:
  Bucket:
    Type: AWS::S3::Bucket
    Condition: IsProd
    Properties:
      BucketName: !Sub "${Env}-${AWS::Region}-logs"
      VersioningConfiguration:
        Status: Enabled
      Tags:
        - Key: env
          Value: !Ref Env
        - Key: owner
          Value: !GetAtt Role.Arn
      NotificationConfiguration: {}
      Replicas: []
      KmsKey: !Ref Key
      Image: !Ref Ami
      Region: !Ref AWS::Region
      Acl: !If [IsProd, private, !Ref "AWS::NoValue"]
      Long:
        Fn::Join: ["", [a, !Ref Env]]
      Exp: 42
      Flag: true
      Nothing:
      Text: |
        line one
        line two
      Quoted: 'it''s'
      Attr: !GetAtt [Role, Arn]
      Imp: !ImportValue shared-vpc
      Odd: !Frobnicate x
      Plain: hello world # trailing comment
  Role:
    Type: AWS::IAM::Role
    Properties:
      RoleName: fixed
`

func TestReadCloudFormationYAML(t *testing.T) {
	set, err := ReadCloudFormationFile("t.yaml", []byte(cfnYAML), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Resources) != 2 || set.SyntaxErrors != 0 {
		t.Fatalf("resources %d, syntax %d", len(set.Resources), set.SyntaxErrors)
	}
	b := set.Resources[0]
	if b.Name != "Bucket" || b.Type != "AWS::S3::Bucket" || b.Tool != ToolCloudFormation {
		t.Fatalf("header %+v", b)
	}
	if b.Multiplicity.Kind != MultiCondition || b.Multiplicity.Class != ClassExpression || b.Multiplicity.Refs[0] != "Env" && b.Multiplicity.Refs[0] != "IsProd" {
		t.Fatalf("multiplicity %+v", b.Multiplicity)
	}
	for _, c := range []struct {
		path string
		want Class
		refs []string
		val  any
	}{
		{"BucketName", ClassExpression, []string{"AWS::Region", "Env"}, nil},
		{"VersioningConfiguration.Status", ClassLiteral, nil, "Enabled"},
		{"Tags[*].Key", ClassLiteral, nil, "env"},
		{"Tags[*].Value", ClassDefaulted, []string{"Env"}, "dev"},
		{"Tags[*].Value", ClassReference, []string{"Role.Arn"}, nil},
		{"NotificationConfiguration", ClassLiteral, nil, map[string]any{}},
		{"Replicas", ClassLiteral, nil, []any{}},
		{"KmsKey", ClassReference, []string{"Key"}, nil},
		{"Image", ClassReference, []string{"Ami"}, nil},
		{"Region", ClassExpression, []string{"AWS::Region"}, nil},
		{"Acl", ClassExpression, []string{"AWS::NoValue", "IsProd"}, nil},
		{"Long", ClassExpression, []string{"Env"}, nil},
		{"Exp", ClassLiteral, nil, int64(42)},
		{"Flag", ClassLiteral, nil, true},
		{"Nothing", ClassLiteral, nil, nil},
		{"Text", ClassLiteral, nil, "line one\nline two\n"},
		{"Quoted", ClassLiteral, nil, "it's"},
		{"Attr", ClassReference, []string{"Role.Arn"}, nil},
		{"Imp", ClassReference, []string{"export:shared-vpc"}, nil},
		{"Odd", ClassUnknown, nil, nil},
		{"Plain", ClassLiteral, nil, "hello world"},
	} {
		var got *Attr
		for i, a := range b.Attrs {
			if got == nil && a.Path == c.path && a.Class == c.want && reflect.DeepEqual(a.Refs, c.refs) {
				got = &b.Attrs[i]
			}
		}
		if got == nil {
			t.Errorf("%s: no attribute with class %s refs %v in %+v", c.path, c.want, c.refs, b.Attrs)
			continue
		}
		if !reflect.DeepEqual(got.Value, c.val) {
			t.Errorf("%s: value %#v, want %#v", c.path, got.Value, c.val)
		}
	}
	// Short and long forms of an intrinsic read the same.
	if r := set.Resources[1]; attrOf(t, r, "RoleName").Class != ClassLiteral {
		t.Fatal("second resource")
	}
}

func TestReadCloudFormationSpans(t *testing.T) {
	set, err := ReadCloudFormationFile("t.yaml", []byte(cfnYAML))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(cfnYAML, "\n")
	b := set.Resources[0]
	for path, want := range map[string]string{
		"VersioningConfiguration.Status": "Status: Enabled",
		"KmsKey":                         "KmsKey: !Ref Key",
		"Plain":                          "Plain: hello world",
		"Quoted":                         "Quoted: 'it''s'",
		"NotificationConfiguration":      "NotificationConfiguration: {}",
		"Acl":                            `Acl: !If [IsProd, private, !Ref "AWS::NoValue"]`,
		"Attr":                           "Attr: !GetAtt [Role, Arn]",
		"BucketName":                     `BucketName: !Sub "${Env}-${AWS::Region}-logs"`,
	} {
		a := attrOf(t, b, path)
		if a.Span.StartLine != a.Span.EndLine {
			t.Errorf("%s: multi-line span %+v", path, a.Span)
			continue
		}
		got := lines[a.Span.StartLine-1][a.Span.StartCol-1 : a.Span.EndCol-1]
		if got != want {
			t.Errorf("%s: span reads %q, want %q", path, got, want)
		}
	}
	// A block scalar spans its lines.
	a := attrOf(t, b, "Text")
	if a.Span.StartLine == a.Span.EndLine || a.Span.EndLine != a.Span.StartLine+2 {
		t.Errorf("block scalar span %+v", a.Span)
	}
	// The resource spans its whole entry.
	if b.Span.StartLine != 14 || b.Span.EndLine < 40 {
		t.Errorf("resource span %+v", b.Span)
	}
}

func TestReadCloudFormationJSON(t *testing.T) {
	src := "{\n\t\"Parameters\": {\"Env\": {\"Type\": \"String\", \"Default\": \"dev\"}},\n" +
		"\t\"Resources\": {\n\t\t\"B\": {\n\t\t\t\"Type\": \"AWS::S3::Bucket\",\n\t\t\t\"Properties\": {\n" +
		"\t\t\t\t\"BucketName\": {\"Fn::Sub\": \"${Env}-x\"},\n" +
		"\t\t\t\t\"Arn\": \"arn:aws:s3:::b\\/*\",\n" +
		"\t\t\t\t\"Role\": {\"Fn::GetAtt\": [\"R\", \"Arn\"]},\n" +
		"\t\t\t\t\"Ref\": {\"Ref\": \"Env\"},\n" +
		"\t\t\t\t\"Uni\": \"caf\u00e9 \\ud83d\\ude00\", \"After\": 1\n" +
		"\t\t\t}\n\t\t}\n\t}\n}\n"
	set, err := ReadCloudFormationFile("t.json", []byte(src), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Resources) != 1 || set.SyntaxErrors != 0 {
		t.Fatalf("%+v", set)
	}
	b := set.Resources[0]
	if a := attrOf(t, b, "BucketName"); a.Class != ClassDefaulted || a.Value != "dev-x" {
		t.Errorf("BucketName %+v", a)
	}
	if a := attrOf(t, b, "Arn"); a.Class != ClassLiteral || a.Value != "arn:aws:s3:::b/*" {
		t.Errorf("Arn %+v", a)
	}
	if a := attrOf(t, b, "Role"); a.Class != ClassReference || a.Refs[0] != "R.Arn" {
		t.Errorf("Role %+v", a)
	}
	if a := attrOf(t, b, "Ref"); a.Class != ClassDefaulted {
		t.Errorf("Ref %+v", a)
	}
	// Spans count characters, so a column after non-ASCII text is exact.
	lines := strings.Split(src, "\n")
	a := attrOf(t, b, "After")
	runes := []rune(lines[a.Span.StartLine-1])
	if got := string(runes[a.Span.StartCol-1 : a.Span.EndCol-1]); got != `"After": 1` {
		t.Errorf("After span reads %q", got)
	}
	if a.Value != int64(1) {
		t.Errorf("After value %#v", a.Value)
	}
	for _, at := range b.Attrs {
		if at.Span.File != "t.json" || at.Span.StartLine < 6 {
			t.Errorf("%s span %+v", at.Path, at.Span)
		}
	}
}

func TestReadCloudFormationNotATemplateAndErrors(t *testing.T) {
	for name, src := range map[string]string{
		"empty": "", "scalar": "hello", "list": "- a\n- b\n", "no resources": "Parameters: {}\n",
		"resources not a map": "Resources: [a]\n", "no type": "Resources:\n  A:\n    Properties: {a: 1}\n",
	} {
		set, err := ReadCloudFormationFile("t.yaml", []byte(src))
		if err != nil || len(set.Resources) != 0 || set.SyntaxErrors != 0 {
			t.Errorf("%s: %+v %v", name, set, err)
		}
	}
	set, err := ReadCloudFormationFile("t.yaml", []byte("a: [unclosed\nb: }{"))
	if err != nil || set.SyntaxErrors != 1 {
		t.Errorf("broken yaml: %+v %v", set, err)
	}
	set, err = ReadCloudFormationFile("t.json", []byte(`{"Resources": {"A": `))
	if err != nil || set.SyntaxErrors != 1 {
		t.Errorf("broken json: %+v %v", set, err)
	}
	deep := strings.Repeat("[", MaxDepth+5) + strings.Repeat("]", MaxDepth+5)
	for name, src := range map[string]string{
		"oversize":   "# " + strings.Repeat("x", MaxFileBytes),
		"deep yaml":  "Resources:\n  A:\n    Type: T\n    Properties:\n      X: " + deep + "\n",
		"deep json":  `{"Resources": {"A": {"Type": "T", "Properties": {"X": ` + deep + `}}}}`,
		"many nodes": "Resources:\n  A:\n    Type: T\n    Properties: {" + strings.Repeat("a: 1, ", MaxNodes/2+1) + "}\n",
	} {
		if _, err := ReadCloudFormationFile("t.yaml", []byte(src)); !errors.Is(err, ErrLimit) {
			t.Errorf("%s: err %v, want ErrLimit", name, err)
		}
	}
}

func TestReadCloudFormationAliasAndTags(t *testing.T) {
	src := "Resources:\n  A:\n    Type: T\n    Properties:\n      X: &x one\n      Y: *x\n      Z: !!str 5\n"
	set, err := ReadCloudFormationFile("t.yaml", []byte(src), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	r := set.Resources[0]
	if attrOf(t, r, "X").Class != ClassLiteral || attrOf(t, r, "Y").Class != ClassUnknown || attrOf(t, r, "Z").Value != "5" {
		t.Fatalf("%+v", r.Attrs)
	}
}

func TestReadCloudFormationNoPanicOnJunk(t *testing.T) {
	for _, src := range []string{
		"Resources:\n  A: !Ref\n", "Resources: !If [a]\n", "Resources:\n  A:\n    Type: !Ref x\n    Properties: !If [a, b]\n",
		"Resources:\n  A:\n    Type: T\n    Properties:\n      X: !Sub [a]\n      Y: !Sub 5\n      Z: !Sub \"${\"\n      W: !GetAtt [a]\n",
		"Resources:\n  A:\n    Type: T\n    Condition: [x]\n", "Resources:\n  A:\n    Type: T\n    Condition: Missing\n",
		"? [a]\n: b\n", "\x00\x01", "{\"a\"", "{\"a\":}", "{\"a\": 1,}", "[1,", "{}", "[]", "null",
		"Resources:\n  A:\n    Type: T\n    Properties:\n      X: |\n", "Resources:\n  A:\n    Type: T\n    Properties:\n      <<: *m\n",
	} {
		_, _ = ReadCloudFormationFile("j.yaml", []byte(src))
	}
}

// FuzzReadTemplates runs every reader on the same bytes, with and without values, and fails on a panic, an
// error that is not ErrLimit, a read that takes over fuzzReadBudget, or a returned value outside the bounds. The
// seeds are the shapes that once killed or hung a reader: operator and bracket runs, nested intrinsics, and
// doubling chains of variables, locals and Sub definitions.
func FuzzReadTemplates(f *testing.F) {
	f.Add([]byte(cfnYAML))
	f.Add([]byte(`{"Resources":{"A":{"Type":"T","Properties":{"X":{"Ref":"Y"}}}}}`))
	f.Add([]byte(pulumiYAML))
	f.Add([]byte("resource \"a\" \"b\" {\n x = var.y\n}\n"))
	f.Add(tfWith(strings.Repeat("!", 5000) + "true"))
	f.Add(tfWith(strings.Repeat("-", 5000) + "1"))
	f.Add(tfWith(strings.Repeat("!-", 2500) + "1"))
	f.Add(tfWith(strings.Repeat("(", 2000)))
	f.Add(tfWith(strings.Repeat("-(", 100) + "1" + strings.Repeat(")", 100)))
	f.Add([]byte(`{"Resources":{"B":{"Type":"T","Properties":{"P":` + strings.Repeat(`{"Fn::Base64":`, 100) + `"x"` + strings.Repeat("}", 100) + `}}}}`))
	f.Add([]byte(cfnSubDoubling(30)))
	f.Add(pulumiDoubling(40))
	f.Add([]byte("locals {\n  l0 = \"x\"\n  l1 = \"${local.l0}${local.l0}\"\n  l2 = \"${local.l1}${local.l1}\"\n  l3 = \"${local.l2}${local.l2}\"\n}\nresource \"t\" \"n\" {\n  x = local.l3\n}\n"))
	f.Add([]byte("resource \"t\" \"n\" {\n  x = \"abc\n  y = 1\n}\n"))
	f.Add([]byte("Resources:\r  B:\r    Type: T\r---\rResources: {}\r"))
	f.Fuzz(func(t *testing.T, b []byte) {
		start := time.Now()
		for _, opts := range [][]Option{nil, {WithValues()}} {
			sets := make([]*ResourceSet, 0, 4)
			errs := make([]error, 0, 4)
			add := func(set *ResourceSet, err error) { sets, errs = append(sets, set), append(errs, err) }
			add(ReadCloudFormationFile("f.yaml", b, opts...))
			add(ReadPulumiYAMLFile("f.yaml", b, opts...))
			add(ReadTerraformFile("f.tf", b, opts...))
			add(ReadTerraformModule(fstest.MapFS{"main.tf": file(string(b)), "x_override.tf": file(string(b))}, ".", opts...))
			for i, err := range errs {
				if err != nil && !errors.Is(err, ErrLimit) {
					t.Fatalf("read %d: error that is not a limit: %v", i, err)
				}
				if err == nil && sets[i] == nil {
					t.Fatalf("read %d: nil set and nil error", i)
				}
			}
		}
		if d := time.Since(start); d > fuzzReadBudget {
			t.Fatalf("a read took %v, over %v", d, fuzzReadBudget)
		}
	})
}

// fuzzReadBudget is the wall-clock bound on all the reads of one fuzz input. The exponential shapes took
// minutes; a linear read of a 2 MiB input takes well under a second.
const fuzzReadBudget = 20 * time.Second

func TestReadCloudFormationYAMLSpansAfterNonASCII(t *testing.T) {
	src := "Resources:\n  A:\n    Type: T\n    Properties:\n      Desc: \"caf\u00e9 \u2603\" # note\n      Next: {a: \u00fc, b: [x, \u00fc]}\n"
	set, err := ReadCloudFormationFile("t.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(src, "\n")
	r := set.Resources[0]
	for path, want := range map[string]string{
		"Desc":      "Desc: \"caf\u00e9 \u2603\"",
		"Next.a":    "a: \u00fc",
		"Next.b[*]": "x",
	} {
		a := attrOf(t, r, path)
		runes := []rune(lines[a.Span.StartLine-1])
		if got := string(runes[a.Span.StartCol-1 : a.Span.EndCol-1]); got != want {
			t.Errorf("%s: span reads %q, want %q", path, got, want)
		}
	}
}

func TestReadsAreDeterministicUnderConcurrency(t *testing.T) {
	want := mustJSON(t, func() (*ResourceSet, error) { return ReadCloudFormationFile("t.yaml", []byte(cfnYAML), WithValues()) })
	wantP := mustJSON(t, func() (*ResourceSet, error) { return ReadPulumiYAMLFile("p.yaml", []byte(pulumiYAML), WithValues()) })
	done := make(chan error, 16)
	for i := 0; i < 16; i++ {
		go func() {
			a := mustJSONNoT(func() (*ResourceSet, error) { return ReadCloudFormationFile("t.yaml", []byte(cfnYAML), WithValues()) })
			b := mustJSONNoT(func() (*ResourceSet, error) { return ReadPulumiYAMLFile("p.yaml", []byte(pulumiYAML), WithValues()) })
			if a != want || b != wantP {
				done <- errors.New("reads differ")
				return
			}
			done <- nil
		}()
	}
	for i := 0; i < 16; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func mustJSON(t *testing.T, f func() (*ResourceSet, error)) string {
	t.Helper()
	return mustJSONNoT(f)
}

func mustJSONNoT(f func() (*ResourceSet, error)) string {
	set, err := f()
	if err != nil {
		return "error"
	}
	return fmt.Sprintf("%#v", *set)
}
