package iac_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

// ImportCatalog builds a catalog from a `tofu providers schema -json` export you made, with no fathom binary. The
// catalog carries attributes, required, sensitive and deprecated flags and no replacement facts, and its manifest
// says so. A CloudFormation registry schema zip builds a region's catalog through ImportRequest.SchemaZip and Region.
func ExampleImportCatalog() {
	dir, err := os.MkdirTemp("", "iac-import")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	export := filepath.Join(dir, "export.json")
	err = os.WriteFile(export, []byte(`{"format_version":"1.0","provider_schemas":{"registry.opentofu.org/example/widget":{
  "provider":{"version":0,"block":{"attributes":{}}},
  "resource_schemas":{"widget_thing":{"version":1,"block":{"attributes":{
    "name":{"type":"string","required":true},
    "size":{"type":"number","optional":true}}}}},
  "data_source_schemas":{}}}}`), 0o644)
	if err != nil {
		log.Fatal(err)
	}
	res, err := iac.ImportCatalog(iac.ImportRequest{
		Dir:     filepath.Join(dir, "catalogs"),
		Exports: []iac.Export{{Version: "1.0.0", Path: export}},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Source, res.Versions, res.ReplacementFacts)

	cat, err := iac.OpenCatalog(res.Path)
	if err != nil {
		log.Fatal(err)
	}
	ct, err := cat.Contract(cat.Source(), "1.0.0", "widget_thing")
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range ct.Attributes {
		fmt.Printf("%s required=%v\n", a.Path, a.Required)
	}
	// Output:
	// registry.opentofu.org/example/widget [1.0.0] absent
	// name required=true
	// size required=false
}
