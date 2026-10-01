package fusion

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// terraformCorpus is a stack module calling a local child module and a
// registry one (LB-Q-terraform's L3): stack/main.tf refers to variables in
// stack/variables.tf, passes the child two inputs and reads a child output
// through the module block it declares; stack/outputs.tf refers to a
// resource and to the module call in main.tf; the child's own files refer
// to each other.
func terraformCorpus() map[string]string {
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

// terraformXrefWant is every cross-file reference terraformCorpus resolves
// to: within a module directory by address, a module input to the child's
// variable, a module output to the child's output. Same-file references
// (aws_subnet.a to aws_vpc.this) are Calls, never here; the registry module
// resolves nothing.
var terraformXrefWant = []string{
	"modules/net/main.tf:aws_internet_gateway.gw -references-> modules/net/variables.tf:var.name",
	"modules/net/main.tf:aws_internet_gateway.gw -references-> modules/net/variables.tf:var.vpc_id",
	"modules/net/outputs.tf:output.gateway_id -references-> modules/net/main.tf:aws_internet_gateway.gw",
	"stack/main.tf:aws_route.r -module-output-> modules/net/outputs.tf:output.gateway_id",
	"stack/main.tf:aws_vpc.this -references-> stack/variables.tf:var.cidr",
	"stack/main.tf:local.tags -references-> stack/variables.tf:var.name",
	"stack/main.tf:local.tags -references-> stack/variables.tf:var.tags",
	"stack/main.tf:module.net -module-input-> modules/net/variables.tf:var.name",
	"stack/main.tf:module.net -module-input-> modules/net/variables.tf:var.vpc_id",
	"stack/main.tf:module.net -references-> stack/variables.tf:var.create",
	"stack/main.tf:module.net -references-> stack/variables.tf:var.name",
	"stack/main.tf:module.remote -references-> stack/variables.tf:var.name",
	"stack/outputs.tf:output.gateway_id -references-> stack/main.tf:module.net",
	"stack/outputs.tf:output.vpc_id -references-> stack/main.tf:aws_vpc.this",
}

// TestResolveDocXrefs_Terraform (LB-Q-terraform L3): the terraform
// dialect's label-selector references resolve through ResolveDocXrefs'
// existing join, one module directory per scope.
func TestResolveDocXrefs_Terraform(t *testing.T) {
	x := extractor.NewHCLExtractor()
	corpus := terraformCorpus()
	var paths []string
	for p := range corpus {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var files []DocFile
	for _, rel := range paths {
		r, err := x.ExtractDoc("svc", rel, []byte(corpus[rel]))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, DocFile{Path: rel, Result: r})
	}
	got := xrefBrief(ResolveDocXrefs(files))
	sort.Strings(got)
	if !reflect.DeepEqual(got, terraformXrefWant) {
		t.Fatalf("got  %q\nwant %q", got, terraformXrefWant)
	}
}

// TestIngest_TerraformDocXrefs (LB-F84): an ingest reads every .tf file
// through the doc-xref pass (isDocCandidate) and stores the resolved
// references as file-tier typed edges.
func TestIngest_TerraformDocXrefs(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, terraformCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "tf", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	all, err := st.TypedEdges(ctx, "tf")
	if err != nil {
		t.Fatal(err)
	}
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	// One edge per (from, to) symbol pair: module.net's two var.name
	// references (its own variable and the child's) are distinct symbols.
	want := make([]string, 0, len(terraformXrefWant))
	for _, w := range terraformXrefWant {
		from, to, _ := strings.Cut(w, " -")
		_, to, _ = strings.Cut(to, "-> ")
		ff, fs, _ := strings.Cut(from, ":")
		tf, ts, _ := strings.Cut(to, ":")
		want = append(want, ff+"|"+tfKey(ff, fs)+"->"+tfKey(tf, ts))
	}
	sort.Strings(want)
	if !reflect.DeepEqual(edges, want) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", edges, want)
	}
	if rep.DocXrefEdges != len(want) {
		t.Errorf("DocXrefEdges = %d, want %d", rep.DocXrefEdges, len(want))
	}
}

// tfKey is the symbol key ingest stores for a terraform definition (its
// service, its file's package and its name).
func tfKey(file, name string) string {
	n, _ := extractor.NewHCLExtractor().Extract("tf", file, []byte(`variable "x" {}`))
	return core.SymbolNode{Service: "tf", FilePath: file, Package: n[0].Package, Name: name}.Key()
}
