package iac_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

const tfSrc = `
variable "env" {
  default = "dev"
}
variable "token" {
  default   = "TOKEN-DEFAULT-VALUE"
  sensitive = true
}
resource "aws_s3_bucket" "logs" {
  bucket = "my-logs"
  acl    = var.env
  tags   = { Name = "x" }
  versioning {
    enabled = true
  }
  password = "hunter2-literal"
}
`

const cfnSrc = `AWSTemplateFormatVersion: "2010-09-09"
Parameters:
  Env:
    Type: String
    Default: dev
Resources:
  Bucket:
    Type: AWS::S3::Bucket
    Properties:
      BucketName: logs
      Tag: !Ref Env
`

const pulumiSrc = `name: demo
runtime: yaml
resources:
  logs:
    type: aws:s3/bucket:Bucket
    properties:
      bucket: my-logs
`

func read(t *testing.T, p, src string, opts ...iac.Option) *iac.ResourceSet {
	t.Helper()
	set, err := iac.ReadFile(context.Background(), p, []byte(src), opts...)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", p, err)
	}
	return set
}

func attr(t *testing.T, r iac.Resource, path string) iac.Attr {
	t.Helper()
	for _, a := range r.Attrs {
		if a.Path == path {
			return a
		}
	}
	t.Fatalf("no attribute %q in %+v", path, r.Attrs)
	return iac.Attr{}
}

// LL-34: ReadFile routes by path and content.
func TestReadFileRoutes(t *testing.T) {
	for _, c := range []struct {
		path, src, tool, typ string
	}{
		{"infra/main.tf", tfSrc, iac.ToolTerraform, "aws_s3_bucket"},
		{"infra/main.tofu", tfSrc, iac.ToolTerraform, "aws_s3_bucket"},
		{"stack.yaml", cfnSrc, iac.ToolCloudFormation, "AWS::S3::Bucket"},
		{"stack.template", cfnSrc, iac.ToolCloudFormation, "AWS::S3::Bucket"},
		{"stack.json", `{"AWSTemplateFormatVersion":"2010-09-09","Resources":{"Bucket":{"Type":"AWS::S3::Bucket","Properties":{"BucketName":"x"}}}}`, iac.ToolCloudFormation, "AWS::S3::Bucket"},
		{"Pulumi.yaml", pulumiSrc, iac.ToolPulumi, "aws:s3/bucket:Bucket"},
		{"deep/dir/Pulumi.yaml", pulumiSrc, iac.ToolPulumi, "aws:s3/bucket:Bucket"},
	} {
		set := read(t, c.path, c.src)
		if len(set.Resources) == 0 {
			t.Errorf("%s: no resources", c.path)
			continue
		}
		var found bool
		for _, r := range set.Resources {
			found = found || (r.Tool == c.tool && r.Type == c.typ && r.Span.File == c.path)
		}
		if !found {
			t.Errorf("%s: no %s %s resource with its path in the span: %+v", c.path, c.tool, c.typ, set.Resources)
		}
	}
}

func TestReadFileJSONSyntaxIsNotReadButCounted(t *testing.T) {
	set := read(t, "main.tf.json", `{"resource":{"aws_s3_bucket":{"b":{"bucket":"x"}}}}`)
	if len(set.Resources) != 0 || set.SyntaxErrors != 1 {
		t.Errorf("a .tf.json file must read as no resource and one syntax error, got %+v", set)
	}
}

func TestReadFileNoReader(t *testing.T) {
	for _, c := range []struct{ path, src string }{
		{"README.md", "# hi\n"},
		{"main.go", "package main\n"},
		{"deploy.yaml", "apiVersion: v1\nkind: ConfigMap\n"},      // YAML that is no template
		{"Pulumi.dev.yaml", "config:\n  aws:region: us-east-1\n"}, // stack configuration is not a program
		{"data.json", `{"Resources": {"a": "b"}}`},                // Resources without a resource type
		{"main.tf.bak", tfSrc},
	} {
		_, err := iac.ReadFile(context.Background(), c.path, []byte(c.src))
		var nr *iac.NoReaderError
		if !errors.As(err, &nr) || nr.Path != c.path {
			t.Errorf("%s: want *NoReaderError, got %v", c.path, err)
		}
	}
}

func TestReadFileMisuse(t *testing.T) {
	if _, err := iac.ReadFile(context.Background(), "", []byte(tfSrc)); err == nil {
		t.Error("an empty path must be an error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := iac.ReadFile(ctx, "a.tf", []byte(tfSrc)); !errors.Is(err, context.Canceled) {
		t.Errorf("a done context: got %v", err)
	}
	// A nil context reads as Background, as pkg/lang does.
	if _, err := iac.ReadFile(nil, "a.tf", []byte(tfSrc)); err != nil { //nolint:staticcheck
		t.Errorf("nil context: %v", err)
	}
	if _, err := iac.ReadFile(context.Background(), "a.tf", []byte(tfSrc), iac.WithDeadline(time.Hour)); err == nil {
		t.Error("a deadline above the limit must be an error")
	}
	if _, err := iac.ReadFile(context.Background(), "a.tf", []byte(tfSrc), iac.WithDeadline(0)); err == nil {
		t.Error("a zero deadline must be an error")
	}
	if _, err := iac.ReadFile(context.Background(), "a.tf", []byte(tfSrc), nil, iac.WithDeadline(time.Second)); err != nil {
		t.Errorf("a lowered deadline and a nil option: %v", err)
	}
}

func TestReadFileRefusesPastSizeLimit(t *testing.T) {
	big := []byte("# " + strings.Repeat("x", iac.MaxFileBytes) + "\n")
	_, err := iac.ReadFile(context.Background(), "big.tf", big)
	if !errors.Is(err, iac.ErrLimit) {
		t.Errorf("a file over MaxFileBytes: want ErrLimit, got %v", err)
	}
	if !errors.Is(iac.ErrLimit, iacfacts.ErrLimit) {
		t.Error("iac.ErrLimit must be the reader's limit error")
	}
}

// Values are empty unless WithValues, which is the only way they are filled.
func TestValuesOnlyWithWithValues(t *testing.T) {
	plain := read(t, "main.tf", tfSrc)
	for _, a := range plain.Resources[0].Attrs {
		if a.Value != nil {
			t.Errorf("%s carries a value without WithValues: %v", a.Path, a.Value)
		}
	}
	if got := attr(t, plain.Resources[0], "bucket"); got.Class != iac.ClassLiteral {
		t.Errorf("bucket class %q", got.Class)
	}
	with := read(t, "main.tf", tfSrc, iac.WithValues())
	if got := attr(t, with.Resources[0], "bucket").Value; got != "my-logs" {
		t.Errorf("bucket value under WithValues = %v", got)
	}
	if got := attr(t, with.Resources[0], "acl"); got.Class != iac.ClassReference && got.Class != iac.ClassDefaulted {
		t.Errorf("acl class %q", got.Class)
	}
}

// The reader's own withholding applies under WithValues: a sensitive variable's default never becomes a value.
func TestWithValuesStillWithholdsSecrets(t *testing.T) {
	fsys := fstest.MapFS{"m/main.tf": {Data: []byte(`
variable "token" {
  default   = "TOKEN-DEFAULT-VALUE"
  sensitive = true
}
variable "env" {
  default = "dev"
}
resource "aws_s3_bucket" "b" {
  token = var.token
  env   = var.env
}
`)}}
	set, err := iac.ReadModule(context.Background(), fsys, "m", iac.WithValues())
	if err != nil {
		t.Fatal(err)
	}
	r := set.Resources[0]
	tok := attr(t, r, "token")
	if tok.Class != iac.ClassDefaulted || tok.Value != nil {
		t.Errorf("a sensitive default must read defaulted with no value, got %+v", tok)
	}
	if env := attr(t, r, "env"); env.Class != iac.ClassDefaulted || env.Value != "dev" {
		t.Errorf("an ordinary default must read defaulted with its value, got %+v", env)
	}
	b, _ := json.Marshal(set)
	if bytes.Contains(b, []byte("TOKEN-DEFAULT-VALUE")) {
		t.Error("the sensitive default is in the JSON")
	}
}

func TestReadModule(t *testing.T) {
	fsys := fstest.MapFS{
		"infra/main.tf":  {Data: []byte(tfSrc)},
		"infra/vars.tf":  {Data: []byte("variable \"extra\" {\n  default = 3\n}\n")},
		"infra/sub/x.tf": {Data: []byte("resource \"a\" \"b\" {}\n")}, // another directory: not listed
	}
	set, err := iac.ReadModule(context.Background(), fsys, "infra")
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Resources) != 1 || set.Resources[0].Span.File != "infra/main.tf" {
		t.Fatalf("resources %+v", set.Resources)
	}
	if got := attr(t, set.Resources[0], "acl"); got.Class != iac.ClassDefaulted {
		t.Errorf("a module read resolves a variable default: acl is %q", got.Class)
	}
	if _, err := iac.ReadModule(context.Background(), fsys, "../infra"); err == nil {
		t.Error("a dir that is not an fs path must be an error")
	}
	if _, err := iac.ReadModule(context.Background(), nil, "."); err == nil {
		t.Error("a nil fsys must be an error")
	}
	if _, err := iac.ReadModule(context.Background(), fsys, "missing"); err == nil {
		t.Error("a missing directory must be an error")
	}
}

// Every slice is non-nil and equal inputs give byte-equal JSON.
func TestResultsAreNonNilAndDeterministic(t *testing.T) {
	for _, c := range []struct{ path, src string }{
		{"a.tf", tfSrc}, {"c.yaml", cfnSrc}, {"Pulumi.yaml", pulumiSrc}, {"empty.tf", ""}, {"j.tf.json", "{}"},
	} {
		set := read(t, c.path, c.src, iac.WithValues())
		b1, _ := json.Marshal(set)
		for _, null := range []string{`"resources":null`, `"dropped":null`, `"attrs":null`, `"refs":null`} {
			if strings.Contains(string(b1), null) {
				t.Errorf("%s: %s in %s", c.path, null, b1)
			}
		}
		for i := 0; i < 5; i++ {
			b2, _ := json.Marshal(read(t, c.path, c.src, iac.WithValues()))
			if !bytes.Equal(b1, b2) {
				t.Fatalf("%s: equal inputs gave different JSON", c.path)
			}
		}
	}
}

func TestReadsAreSafeConcurrently(t *testing.T) {
	want, _ := json.Marshal(read(t, "a.tf", tfSrc, iac.WithValues()))
	done := make(chan []byte, 8)
	for i := 0; i < 8; i++ {
		go func() {
			set, err := iac.ReadFile(context.Background(), "a.tf", []byte(tfSrc), iac.WithValues())
			if err != nil {
				done <- nil
				return
			}
			b, _ := json.Marshal(set)
			done <- b
		}()
	}
	for i := 0; i < 8; i++ {
		if got := <-done; !bytes.Equal(got, want) {
			t.Fatal("concurrent reads differ")
		}
	}
}

// The public constants are literals, held to pkg/lang's and the reader's.
func TestPublicConstantValues(t *testing.T) {
	deadline, langDeadline := iac.DefaultReadDeadline, lang.DefaultReadDeadline
	if deadline != 30*time.Second || deadline != langDeadline {
		t.Errorf("DefaultReadDeadline = %v, want 30s and pkg/lang's %v", deadline, langDeadline)
	}
	limit, innerLimit, langLimit := iac.MaxFileBytes, iacfacts.MaxFileBytes, lang.DefaultMaxFileBytes
	if limit != 2*1024*1024 || limit != innerLimit || limit != langLimit {
		t.Errorf("MaxFileBytes = %d, want 2 MiB, the reader's %d and pkg/lang's %d", limit, innerLimit, langLimit)
	}
	for _, c := range []struct{ got, want string }{
		{string(iac.ClassLiteral), "literal"}, {string(iac.ClassDefaulted), "defaulted"}, {string(iac.ClassReference), "reference"},
		{string(iac.ClassExpression), "expression"}, {string(iac.ClassUnknown), "unknown"},
		{string(iac.MultiSingle), "single"}, {string(iac.MultiCount), "count"}, {string(iac.MultiForEach), "for_each"}, {string(iac.MultiCondition), "condition"},
		{iac.ToolTerraform, "terraform"}, {iac.ToolCloudFormation, "cloudformation"}, {iac.ToolPulumi, "pulumi"},
		{iac.ModeManaged, "managed"}, {iac.ModeData, "data"}, {iac.ModeModule, "module"}, {iac.ModeEphemeral, "ephemeral"},
	} {
		if c.got != c.want {
			t.Errorf("constant = %q, want %q: a change is a breaking change", c.got, c.want)
		}
	}
	// The classes and kinds agree with the reader's.
	for pub, in := range map[string]string{
		string(iac.ClassLiteral): string(iacfacts.ClassLiteral), string(iac.ClassDefaulted): string(iacfacts.ClassDefaulted),
		string(iac.ClassReference): string(iacfacts.ClassReference), string(iac.ClassExpression): string(iacfacts.ClassExpression),
		string(iac.ClassUnknown): string(iacfacts.ClassUnknown),
		string(iac.MultiSingle):  string(iacfacts.MultiSingle), string(iac.MultiCount): string(iacfacts.MultiCount),
		string(iac.MultiForEach): string(iacfacts.MultiForEach), string(iac.MultiCondition): string(iacfacts.MultiCondition),
		iac.ModeManaged: iacfacts.ModeManaged, iac.ModeData: iacfacts.ModeData, iac.ModeModule: iacfacts.ModeModule, iac.ModeEphemeral: iacfacts.ModeEphemeral,
	} {
		if pub != in {
			t.Errorf("public %q disagrees with the reader's %q", pub, in)
		}
	}
}

// No internal type reaches the public one: the JSON shape is the public types' own, and the conversion keeps what
// the reader read.
func TestConversionKeepsTheRead(t *testing.T) {
	src := []byte(cfnSrc)
	in, err := iacfacts.ReadCloudFormationFile("c.yaml", src, iacfacts.WithValues())
	if err != nil {
		t.Fatal(err)
	}
	out, err := iac.ReadFile(context.Background(), "c.yaml", src, iac.WithValues())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Resources) != len(in.Resources) || out.SyntaxErrors != in.SyntaxErrors || len(out.Dropped) != len(in.Dropped) {
		t.Fatalf("shape differs: %+v vs %+v", out, in)
	}
	for i, r := range in.Resources {
		o := out.Resources[i]
		if o.Type != r.Type || o.Name != r.Name || len(o.Attrs) != len(r.Attrs) || o.Span.StartLine != r.Span.StartLine {
			t.Errorf("resource %d differs: %+v vs %+v", i, o, r)
		}
		for j, a := range r.Attrs {
			if o.Attrs[j].Path != a.Path || string(o.Attrs[j].Class) != string(a.Class) || o.Attrs[j].Value != a.Value {
				t.Errorf("attr %d.%d differs: %+v vs %+v", i, j, o.Attrs[j], a)
			}
		}
	}
}
