package iac_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

// The seeds are the shapes the reader tests in internal/iacfacts hold: a Terraform file of every class, a
// CloudFormation template with parameters, conditions and intrinsics, a Pulumi program, the JSON syntax, and input
// that is none of them.
var fuzzSeeds = []string{
	`
variable "x" {
  default = "d"
}
variable "s" {
  default   = "secret"
  sensitive = true
}
resource "aws_s3_bucket" "logs" {
  bucket     = "my-logs"
  a          = var.x
  b          = ["a", var.x]
  c          = "pre-${var.x}"
  d          = true ? "a" : "b"
  e          = upper("x")
  f          = aws_s3_bucket.other.id
  l          = aws_instance.web[*].id
  count      = var.n
  provider   = aws.west
  depends_on = [aws_iam_role.r]
  versioning {
    enabled = true
  }
  dynamic "grant" {
    for_each = var.grants
    content {
      id = grant.value
    }
  }
  lifecycle {
    ignore_changes = [tags]
  }
}
data "aws_caller_identity" "me" {}
module "net" {
  source = "./net"
}
`,
	"resource \"a\" \"b\" {\n  x = \n  y = [\n}\nvariable \"v\" {\n",
	`{"resource":{"aws_s3_bucket":{"b":{"bucket":"x"}}}}`,
	`AWSTemplateFormatVersion: "2010-09-09"
Parameters:
  Env:
    Type: String
    Default: dev
  Key:
    Type: String
    NoEcho: true
    Default: hunter2
Conditions:
  IsProd: !Equals [!Ref Env, prod]
Resources:
  Bucket:
    Type: AWS::S3::Bucket
    Condition: IsProd
    Properties:
      BucketName: !Sub "${Env}-${AWS::Region}-logs"
      Tags:
        - Key: env
          Value: !Ref Env
      Acl: !If [IsProd, private, !Ref "AWS::NoValue"]
      Long:
        Fn::Join: ["", [a, !Ref Env]]
      Exp: 1e6
      Odd: !Frobnicate x
`,
	`{"AWSTemplateFormatVersion":"2010-09-09","Resources":{"B":{"Type":"AWS::S3::Bucket","Properties":{"Name":{"Ref":"P"},"N":1e999}}}}`,
	`name: demo
runtime: yaml
configuration:
  env:
    type: String
    default: dev
resources:
  logs:
    type: aws:s3/bucket:Bucket
    properties:
      bucket: ${env}-logs
      secretish:
        fn::secret: hunter2
      rules:
        - id: one
        - id: ${logs.id}
  ami:
    type: aws:ec2:getAmi
    get:
      id: ami-123
`,
	"a: &x [*x]\nb: *x\n",
	"Resources: *all\n",
	"\x00\xff{{{",
	"",
}

var fuzzNames = []string{"main.tf", "main.tofu", "main.tf.json", "override.tf", "stack.yaml", "stack.json", "stack.template", "Pulumi.yaml", "notes.txt"}

// fuzzDeadline is the deadline every fuzz read is given, and fuzzSlack what a read may take beyond it.
const (
	fuzzDeadline = 5 * time.Second
	fuzzSlack    = 5 * time.Second
)

// FuzzReadFile (LL-37) reads the same bytes under every routable name, with and without values, and fails on a
// panic, an error that is not one the door documents, a read that returns after its deadline, a result that does
// not marshal, or two reads of one input that differ. The seeds run in the normal test run.
func FuzzReadFile(f *testing.F) {
	for _, s := range fuzzSeeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		ctx := context.Background()
		for _, name := range fuzzNames {
			for _, values := range []bool{false, true} {
				opts := []iac.Option{iac.WithDeadline(fuzzDeadline)}
				if values {
					opts = append(opts, iac.WithValues())
				}
				var first []byte
				for round := 0; round < 2; round++ {
					start := time.Now()
					set, err := iac.ReadFile(ctx, name, src, opts...)
					if d := time.Since(start); d > fuzzDeadline+fuzzSlack {
						t.Fatalf("%s: a read took %v, past its %v deadline", name, d, fuzzDeadline)
					}
					if err != nil {
						checkReadError(t, name, err)
						break
					}
					if set == nil || set.Resources == nil || set.Dropped == nil {
						t.Fatalf("%s: a nil set or nil slice without an error", name)
					}
					b, err := json.Marshal(set)
					if err != nil {
						t.Fatalf("%s: the result does not marshal: %v", name, err)
					}
					if round == 0 {
						first = b
					} else if !bytes.Equal(first, b) {
						t.Fatalf("%s: two reads of one input differ", name)
					}
				}
			}
		}
		// The same bytes as the only file of a module.
		fsys := fstest.MapFS{"m/main.tf": {Data: src}, "m/vars.tf": {Data: src}}
		set, err := iac.ReadModule(ctx, fsys, "m", iac.WithDeadline(fuzzDeadline), iac.WithValues())
		if err != nil {
			checkReadError(t, "module", err)
			return
		}
		if _, err := json.Marshal(set); err != nil {
			t.Fatalf("module: the result does not marshal: %v", err)
		}
	})
}

// checkReadError fails on an error the door does not document: a limit, a deadline, the busy refusal or a file no
// reader reads. Anything else (a panic recovered into a vague error, a parser's message) is a bug.
func checkReadError(t *testing.T, name string, err error) {
	t.Helper()
	var nr *iac.NoReaderError
	switch {
	case errors.Is(err, iac.ErrLimit), errors.Is(err, iac.ErrDeadline), errors.Is(err, iac.ErrBusy), errors.As(err, &nr):
	default:
		t.Fatalf("%s: an error the door does not document: %v", name, err)
	}
}

// FuzzOpenCatalog (the design's section 8 row 5) writes the bytes as a catalog file, opens it and asks every
// version for a few types. It fails on a panic, or an error that is neither a *CatalogError nor an I/O error.
func FuzzOpenCatalog(f *testing.F) {
	good, err := os.ReadFile(fixtureCatalog)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(good)
	f.Add(good[:len(good)/2])
	f.Add(good[:22])
	f.Add([]byte("PK\x05\x06" + string(make([]byte, 18))))
	f.Add([]byte("not a zip"))
	f.Add([]byte{})
	dir := f.TempDir()
	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(dir, "c.zip")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		c, err := iac.OpenCatalog(path)
		if err != nil {
			return
		}
		for _, v := range c.Versions() {
			for _, typ := range []string{"widget_thing", "aws_s3_bucket", ""} {
				ct, err := c.Contract(c.Source(), v, typ)
				if err != nil {
					var ce *iac.CatalogError
					if !errors.As(err, &ce) && !errors.Is(err, os.ErrNotExist) && typ != "" {
						t.Fatalf("Contract: an error that is not a *CatalogError: %v", err)
					}
					continue
				}
				if _, err := json.Marshal(ct); err != nil {
					t.Fatalf("the contract does not marshal: %v", err)
				}
			}
		}
	})
}
