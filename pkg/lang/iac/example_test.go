package iac_test

import (
	"context"
	"fmt"
	"log"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

// ReadFile reads a file's resources. Each attribute has a class, never a value unless WithValues is passed, so a
// policy tool learns that a bucket name is a constant, a default or a reference without holding it.
func ExampleReadFile() {
	src := []byte(`
variable "env" {
  default = "dev"
}
resource "aws_s3_bucket" "logs" {
  bucket = "my-logs"
  acl    = var.env
  versioning {
    enabled = true
  }
}
`)
	set, err := iac.ReadFile(context.Background(), "infra/main.tf", src)
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range set.Resources {
		fmt.Println(r.Tool, r.Type, r.Name)
		for _, a := range r.Attrs {
			fmt.Printf("  %-20s %s %v\n", a.Path, a.Class, a.Value)
		}
	}
	// Output:
	// terraform aws_s3_bucket logs
	//   acl                  reference <nil>
	//   bucket               literal <nil>
	//   versioning[*].enabled literal <nil>
}

// ReadModule resolves variable defaults across a directory: an attribute that reads var.env reads as defaulted,
// and with WithValues its value is the default's.
func ExampleReadModule() {
	fsys := fstest.MapFS{
		"infra/vars.tf": {Data: []byte("variable \"env\" {\n  default = \"dev\"\n}\n")},
		"infra/main.tf": {Data: []byte("resource \"aws_s3_bucket\" \"logs\" {\n  acl = var.env\n}\n")},
	}
	set, err := iac.ReadModule(context.Background(), fsys, "infra", iac.WithValues())
	if err != nil {
		log.Fatal(err)
	}
	a := set.Resources[0].Attrs[0]
	fmt.Println(a.Path, a.Class, a.Value, a.Refs)
	// Output:
	// acl defaulted dev [var.env]
}

// OpenCatalog validates a catalog an operator built, and Contract answers from it: what the provider's schema says
// of a type at an exact version. Comparing a read with a contract is how "this attribute is unset" is learned.
func ExampleOpenCatalog() {
	cat, err := iac.OpenCatalog("testdata/registry.opentofu.org_example_widget.v1.iaccat.zip")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(cat.Source(), cat.Versions())
	ct, err := cat.Contract(cat.Source(), "1.0.0", "widget_thing")
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range ct.Attributes {
		if a.Required || a.Sensitive {
			fmt.Printf("%s required=%v sensitive=%v\n", a.Path, a.Required, a.Sensitive)
		}
	}
	// Output:
	// registry.opentofu.org/example/widget [1.0.0 1.1.0]
	// name required=true sensitive=false
	// rule[*] required=true sensitive=false
	// rule[*].port required=true sensitive=false
	// secret required=false sensitive=true
	// settings[*].mode required=true sensitive=false
}
