package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
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
