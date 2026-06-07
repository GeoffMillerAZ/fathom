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
		if n.Kind != core.KindTerraformResource {
			t.Errorf("Kind: got %q, want KindTerraformResource", n.Kind)
		}
	})

	t.Run("data source", func(t *testing.T) {
		_, ok := byName["data.aws_iam_policy_document.assume_role"]
		if !ok {
			t.Fatal("expected symbol data.aws_iam_policy_document.assume_role")
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
