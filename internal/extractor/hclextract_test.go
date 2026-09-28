package extractor_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const hclFixture = `# Terraform config

resource "aws_s3_bucket" "assets" {
  bucket = "my-assets"
}

resource "aws_lambda_function" "api" {
  function_name = "api-handler"
}

module "vpc" {
  source = "./modules/vpc"
}

data "aws_iam_policy_document" "assume_role" {
  statement {
    actions = ["sts:AssumeRole"]
  }
}
`

const hclExtendedFixture = `# Extended Terraform fixture

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "env" {
  type = string
}

output "bucket_arn" {
  value = "arn:aws:s3:::example"
}

provider "aws" {
  region = var.region
}

locals {
  prefix = "app"
  suffix = "prod"
}
`

func TestHCLExtractor_Basic(t *testing.T) {
	x := extractor.NewHCLExtractor()
	nodes, err := x.Extract("svc", "main.tf", []byte(hclFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("resource aws_s3_bucket.assets", func(t *testing.T) {
		n, ok := byName["aws_s3_bucket.assets"]
		if !ok {
			t.Fatal("expected symbol aws_s3_bucket.assets")
		}
		if n.Kind != core.KindTerraformResource {
			t.Errorf("Kind: got %q, want KindTerraformResource", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
		// The block body spans multiple lines: EndLine must be the closing
		// brace's line, strictly after Line.
		if n.EndLine <= n.Line {
			t.Errorf("EndLine: got %d, want > Line (%d)", n.EndLine, n.Line)
		}
	})

	t.Run("resource aws_lambda_function.api", func(t *testing.T) {
		_, ok := byName["aws_lambda_function.api"]
		if !ok {
			t.Fatal("expected symbol aws_lambda_function.api")
		}
	})

	t.Run("module vpc", func(t *testing.T) {
		n, ok := byName["module.vpc"]
		if !ok {
			t.Fatal("expected symbol module.vpc")
		}
		if n.Kind != core.KindTerraformModule {
			t.Errorf("Kind: got %q, want KindTerraformModule", n.Kind)
		}
	})

	t.Run("data source", func(t *testing.T) {
		n, ok := byName["data.aws_iam_policy_document.assume_role"]
		if !ok {
			t.Fatal("expected symbol data.aws_iam_policy_document.assume_role")
		}
		if n.Kind != core.KindTerraformData {
			t.Errorf("Kind: got %q, want KindTerraformData", n.Kind)
		}
	})
}

func TestHCLExtractor_Extended(t *testing.T) {
	x := extractor.NewHCLExtractor()
	nodes, err := x.Extract("svc", "vars.tf", []byte(hclExtendedFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("variable region", func(t *testing.T) {
		n, ok := byName["var.region"]
		if !ok {
			t.Fatal("expected symbol var.region")
		}
		if n.Kind != core.KindTerraformVariable {
			t.Errorf("Kind: got %q, want KindTerraformVariable", n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
	})

	t.Run("variable env", func(t *testing.T) {
		_, ok := byName["var.env"]
		if !ok {
			t.Fatal("expected symbol var.env")
		}
	})

	t.Run("output bucket_arn", func(t *testing.T) {
		n, ok := byName["output.bucket_arn"]
		if !ok {
			t.Fatal("expected symbol output.bucket_arn")
		}
		if n.Kind != core.KindTerraformOutput {
			t.Errorf("Kind: got %q, want KindTerraformOutput", n.Kind)
		}
	})

	t.Run("provider aws", func(t *testing.T) {
		n, ok := byName["provider.aws"]
		if !ok {
			t.Fatal("expected symbol provider.aws")
		}
		if n.Kind != core.KindTerraformProvider {
			t.Errorf("Kind: got %q, want KindTerraformProvider", n.Kind)
		}
	})

	t.Run("local prefix", func(t *testing.T) {
		n, ok := byName["local.prefix"]
		if !ok {
			t.Fatal("expected symbol local.prefix")
		}
		if n.Kind != core.KindTerraformLocal {
			t.Errorf("Kind: got %q, want KindTerraformLocal", n.Kind)
		}
		// Single-line attribute: EndLine equals Line.
		if n.EndLine != n.Line {
			t.Errorf("EndLine: got %d, want == Line (%d)", n.EndLine, n.Line)
		}
	})

	t.Run("local suffix", func(t *testing.T) {
		_, ok := byName["local.suffix"]
		if !ok {
			t.Fatal("expected symbol local.suffix")
		}
	})
}

func TestHCLExtractor_Determinism(t *testing.T) {
	x := extractor.NewHCLExtractor()
	n1, err := x.Extract("svc", "main.tf", []byte(hclFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "main.tf", []byte(hclFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name {
			t.Errorf("non-deterministic at [%d]", i)
		}
	}
}

func TestHCLExtractor_EmptyInput(t *testing.T) {
	x := extractor.NewHCLExtractor()
	nodes, err := x.Extract("svc", "empty.tf", []byte(""))
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty input, got %d", len(nodes))
	}
}

// hclRefsFixture exercises resource reference edges (design §13, terraform
// queue item notes): a resource referencing another resource, a var, a
// local and a data source; an output referencing a resource; a module
// with a local-path source; depends_on as an explicit reference.
const hclRefsFixture = `variable "cidr" {
  type = string
}

locals {
  name = "demo-${var.cidr}"
}

data "aws_ami" "ubuntu" {
  most_recent = true
}

resource "aws_vpc" "main" {
  cidr_block = var.cidr
}

resource "aws_subnet" "public" {
  vpc_id     = aws_vpc.main.id
  ami        = data.aws_ami.ubuntu.id
  depends_on = [aws_vpc.main]
}

output "vpc_id" {
  value = aws_vpc.main.id
}

module "vpc" {
  source = "./modules/vpc"
}
`

func TestHCLExtractor_ReferenceEdges(t *testing.T) {
	x := extractor.NewHCLExtractor()
	nodes, err := x.Extract("svc", "main.tf", []byte(hclRefsFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	hasCall := func(t *testing.T, calls []string, want string) {
		t.Helper()
		for _, c := range calls {
			if c == want {
				return
			}
		}
		t.Errorf("Calls %v does not contain %q", calls, want)
	}

	t.Run("resource references var and resource", func(t *testing.T) {
		n, ok := byName["aws_vpc.main"]
		if !ok {
			t.Fatal("expected symbol aws_vpc.main")
		}
		hasCall(t, n.Calls, "var.cidr")
	})

	t.Run("resource references resource, data and depends_on", func(t *testing.T) {
		n, ok := byName["aws_subnet.public"]
		if !ok {
			t.Fatal("expected symbol aws_subnet.public")
		}
		hasCall(t, n.Calls, "aws_vpc.main")
		hasCall(t, n.Calls, "data.aws_ami.ubuntu")
	})

	t.Run("output references resource", func(t *testing.T) {
		n, ok := byName["output.vpc_id"]
		if !ok {
			t.Fatal("expected symbol output.vpc_id")
		}
		hasCall(t, n.Calls, "aws_vpc.main")
	})

	t.Run("local references var", func(t *testing.T) {
		n, ok := byName["local.name"]
		if !ok {
			t.Fatal("expected symbol local.name")
		}
		hasCall(t, n.Calls, "var.cidr")
	})

	t.Run("module records local-path source", func(t *testing.T) {
		n, ok := byName["module.vpc"]
		if !ok {
			t.Fatal("expected symbol module.vpc")
		}
		if !strings.Contains(n.Signature, "./modules/vpc") {
			t.Errorf("Signature %q does not record the module source", n.Signature)
		}
	})
}

const hclRegistryModuleFixture = `module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"
}
`

func TestHCLExtractor_ModuleRegistrySource(t *testing.T) {
	x := extractor.NewHCLExtractor()
	nodes, err := x.Extract("svc", "main.tf", []byte(hclRegistryModuleFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}
	n, ok := byName["module.vpc"]
	if !ok {
		t.Fatal("expected symbol module.vpc")
	}
	if !strings.Contains(n.Signature, "terraform-aws-modules/vpc/aws") {
		t.Errorf("Signature %q does not record the registry source", n.Signature)
	}
}

const tfvarsFixture = `region = "us-east-1"
instance_count = 3
tags = {
  env = "prod"
}
`

func TestHCLExtractor_TFVars(t *testing.T) {
	x := extractor.NewHCLExtractor()
	nodes, err := x.Extract("svc", "terraform.tfvars", []byte(tfvarsFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}
	for _, want := range []string{"region", "instance_count", "tags"} {
		n, ok := byName[want]
		if !ok {
			t.Fatalf("expected tfvars key %q", want)
		}
		if n.Kind != core.KindTerraformVarValue {
			t.Errorf("%s: Kind = %q, want KindTerraformVarValue", want, n.Kind)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("%s: Tier = %q, want TierVerified", want, n.Tier)
		}
	}
}

func TestHCLExtractor_TFVarsDeterminism(t *testing.T) {
	x := extractor.NewHCLExtractor()
	n1, err := x.Extract("svc", "terraform.tfvars", []byte(tfvarsFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	n2, err := x.Extract("svc", "terraform.tfvars", []byte(tfvarsFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name {
			t.Errorf("non-deterministic at [%d]: %q vs %q", i, n1[i].Name, n2[i].Name)
		}
	}
}
