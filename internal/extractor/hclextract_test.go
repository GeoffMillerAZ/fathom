package extractor_test

import (
	"fmt"
	"reflect"
	"sort"
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

// hclScopedNamesFixture holds names that are not references to another
// block: a dynamic block's iterator (its label, or its `iterator`), a for
// expression's key and value, and the provider meta-arguments (`provider`
// on a resource, `providers` on a module call), which name a provider
// configuration. OpenTofu's own configuration JSON (`tofu show -json
// -config`) and plan graph list none of them as a reference (LB-Q-terraform).
const hclScopedNamesFixture = `resource "aws_security_group" "sg" {
  provider = aws.remote
  vpc_id   = var.vpc_id

  dynamic "ingress" {
    for_each = var.rules
    content {
      from_port = ingress.value.from
      to_port   = ingress.value.to
    }
  }

  dynamic "egress" {
    for_each = var.egress
    iterator = rule
    content {
      cidr_blocks = rule.value.cidrs
    }
  }
}

locals {
  names = { for k, v in var.subnets : k => v.name }
  arns  = [for s in aws_subnet.this : s.arn]
}

output "ids" {
  value = { for k, sg in aws_security_group.sg : k => sg.id }
}

module "child" {
  source    = "./child"
  providers = { aws = aws.remote }
  ids       = [for x in var.list : x.id]
}
`

func TestHCLExtractor_ScopedNamesAreNotReferences(t *testing.T) {
	nodes, err := extractor.NewHCLExtractor().Extract("svc", "main.tf", []byte(hclScopedNamesFixture))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, n := range nodes {
		got[n.Name] = strings.Join(n.Calls, ",")
	}
	want := map[string]string{
		"aws_security_group.sg": "var.egress,var.rules,var.vpc_id",
		"local.names":           "var.subnets",
		"local.arns":            "aws_subnet.this",
		"output.ids":            "aws_security_group.sg",
		"module.child":          "var.list",
	}
	for name, calls := range want {
		if got[name] != calls {
			t.Errorf("%s Calls = %q, want %q", name, got[name], calls)
		}
	}
}

// hclDocCorpus is a stack module that calls a local child module and a
// registry one. stack/main.tf refers to its own variables (another file),
// to its own resources (the same file), to a child module's output through a
// module block it declares, and passes the child two inputs;
// stack/outputs.tf refers to a resource and to the module call in main.tf.
func hclDocCorpus() map[string]string {
	return map[string]string{
		"stack/main.tf": `locals {
  tags = merge(var.tags, { Name = var.name })
}

resource "aws_vpc" "this" {
  cidr_block = var.cidr
  tags       = local.tags
}

resource "aws_subnet" "a" {
  vpc_id = aws_vpc.this.id
}

module "net" {
  source = "../modules/net"
  count  = var.create ? 1 : 0

  vpc_id = aws_vpc.this.id
  name   = var.name
}

module "remote" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "6.0.0"
  name    = var.name
}

resource "aws_route" "r" {
  gateway_id = module.net[0].gateway_id
}
`,
		"stack/variables.tf": `variable "cidr" {}
variable "name" {}
variable "tags" {}
variable "create" {}
`,
		"stack/outputs.tf": `output "vpc_id" {
  value = aws_vpc.this.id
}

output "gateway_id" {
  value = module.net[0].gateway_id
}
`,
		"modules/net/variables.tf": `variable "vpc_id" {}
variable "name" {}
`,
		"modules/net/main.tf": `resource "aws_internet_gateway" "gw" {
  vpc_id = var.vpc_id
  tags   = { Name = var.name }
}
`,
		"modules/net/outputs.tf": `output "gateway_id" {
  value = aws_internet_gateway.gw.id
}
`,
	}
}

// TestHCLExtractor_ExtractDoc (LB-F84): ExtractDoc returns Extract's nodes,
// a "terraform.address" label for every definition scoped to its module
// directory, and a label-selector reference for each reference whose target
// is not defined in the same file: a within-module one, a module input (the
// child's variable) and a module output reached through a module block the
// file declares. A same-file reference stays in Calls only; a registry
// module resolves to nothing.
func TestHCLExtractor_ExtractDoc(t *testing.T) {
	x := extractor.NewHCLExtractor()
	src := []byte(hclDocCorpus()["stack/main.tf"])
	res, err := x.ExtractDoc("svc", "stack/main.tf", src)
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := x.Extract("svc", "stack/main.tf", src)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Nodes, nodes) {
		t.Fatalf("ExtractDoc nodes differ from Extract's:\n%+v\n%+v", res.Nodes, nodes)
	}
	calls := map[string]string{}
	for _, n := range res.Nodes {
		calls[n.Name] = strings.Join(n.Calls, ",")
	}
	for name, want := range map[string]string{
		"local.tags":    "var.name,var.tags",
		"aws_vpc.this":  "local.tags,var.cidr",
		"aws_subnet.a":  "aws_vpc.this",
		"module.net":    "aws_vpc.this,var.create,var.name",
		"module.remote": "var.name",
		"aws_route.r":   "module.net",
	} {
		if calls[name] != want {
			t.Errorf("%s Calls = %q, want %q", name, calls[name], want)
		}
	}

	var labels []string
	for _, l := range res.Labels {
		labels = append(labels, fmt.Sprintf("%s %s %v", l.Entity, l.Scope, l.Labels))
	}
	sort.Strings(labels)
	wantLabels := []string{
		"aws_route.r terraform:stack map[terraform.address:aws_route.r]",
		"aws_subnet.a terraform:stack map[terraform.address:aws_subnet.a]",
		"aws_vpc.this terraform:stack map[terraform.address:aws_vpc.this]",
		"local.tags terraform:stack map[terraform.address:local.tags]",
		"module.net terraform:stack map[terraform.address:module.net]",
		"module.remote terraform:stack map[terraform.address:module.remote]",
	}
	if !reflect.DeepEqual(labels, wantLabels) {
		t.Errorf("labels\n got %q\nwant %q", labels, wantLabels)
	}

	var cross []string
	for _, c := range res.CrossFile {
		if c.Strategy != "label-selector" || c.Target != "terraform.address="+c.Labels["terraform.address"] {
			t.Errorf("%+v: want a label-selector on terraform.address", c)
		}
		cross = append(cross, fmt.Sprintf("%s -%s-> %s %s", c.From, c.Edge, c.Scope, c.Labels["terraform.address"]))
	}
	sort.Strings(cross)
	wantCross := []string{
		"aws_route.r -module-output-> terraform:modules/net output.gateway_id",
		"aws_vpc.this -references-> terraform:stack var.cidr",
		"local.tags -references-> terraform:stack var.name",
		"local.tags -references-> terraform:stack var.tags",
		"module.net -module-input-> terraform:modules/net var.name",
		"module.net -module-input-> terraform:modules/net var.vpc_id",
		"module.net -references-> terraform:stack var.create",
		"module.net -references-> terraform:stack var.name",
		"module.remote -references-> terraform:stack var.name",
	}
	if !reflect.DeepEqual(cross, wantCross) {
		t.Errorf("cross-file\n got %q\nwant %q", cross, wantCross)
	}

	// outputs.tf names the module call, declared in another file: the call
	// is the reference (its source is not this file's to know).
	res, err = x.ExtractDoc("svc", "stack/outputs.tf", []byte(hclDocCorpus()["stack/outputs.tf"]))
	if err != nil {
		t.Fatal(err)
	}
	cross = nil
	for _, c := range res.CrossFile {
		cross = append(cross, fmt.Sprintf("%s -%s-> %s %s", c.From, c.Edge, c.Scope, c.Labels["terraform.address"]))
	}
	sort.Strings(cross)
	wantCross = []string{
		"output.gateway_id -references-> terraform:stack module.net",
		"output.vpc_id -references-> terraform:stack aws_vpc.this",
	}
	if !reflect.DeepEqual(cross, wantCross) {
		t.Errorf("outputs.tf cross-file\n got %q\nwant %q", cross, wantCross)
	}
}

// TestNativeDialectExtractor_Terraform: the harness measures the terraform
// dialect through its own extractor, a DocResulter, so its cross-file
// references reach the same resolver ingest stores edges from.
func TestNativeDialectExtractor_Terraform(t *testing.T) {
	e, ok := extractor.NativeDialectExtractor("terraform")
	if !ok {
		t.Fatal("NativeDialectExtractor(terraform): not found")
	}
	if _, isDoc := e.(extractor.DocResulter); !isDoc {
		t.Fatalf("%T is not a DocResulter", e)
	}
	if _, isHCL := e.(*extractor.HCLExtractor); !isHCL {
		t.Fatalf("%T, want *HCLExtractor", e)
	}
}
