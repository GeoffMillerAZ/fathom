package main

import (
	"reflect"
	"sort"
	"testing"
)

func TestParseNode(t *testing.T) {
	for in, want := range map[string]gnode{
		`[root] aws_vpc.this (expand)`:                          {addr: "aws_vpc.this", note: "expand"},
		`[root] var.name`:                                       {addr: "var.name"},
		`[root] module.a.module.b (close)`:                      {key: "a", call: "b", note: "close"},
		`[root] module.a.var.x (expand, input)`:                 {key: "a", addr: "var.x", note: "expand, input"},
		`[root] module.a.module.b.data.aws_ami.x (expand)`:      {key: "a.b", addr: "data.aws_ami.x", note: "expand"},
		`[root] module.vpc-endpoints.output.endpoints (expand)`: {key: "vpc-endpoints", addr: "output.endpoints", note: "expand"},
	} {
		got, ok := parseNode(in)
		if !ok || got != want {
			t.Errorf("parseNode(%q) = %+v, %v; want %+v", in, got, ok, want)
		}
	}
	for _, in := range []string{`[root] root`, `[root] provider["registry.opentofu.org/hashicorp/aws"] (close)`, `[root] meta.count-boundary (EachMode fixup)`} {
		if _, ok := parseNode(in); ok {
			t.Errorf("parseNode(%q) accepted a structural node", in)
		}
	}
}

func TestTarget(t *testing.T) {
	type res struct {
		name, call, output string
		ok                 bool
	}
	for in, want := range map[string]res{
		`var.tags`:                          {"var.tags", "", "", true},
		`var.opts["mode"]`:                  {"var.opts", "", "", true},
		`data.aws_iam_policy_document.x[0]`: {"data.aws_iam_policy_document.x", "", "", true},
		`module.vpc.vpc_id`:                 {"module.vpc", "vpc", "vpc_id", true},
		`module.vpc[0].vpc_id`:              {"module.vpc", "vpc", "vpc_id", true},
		`module.vpc`:                        {"module.vpc", "", "", true},
		`aws_subnet.private[0].id`:          {"aws_subnet.private", "", "", true},
		`count.index`:                       {},
		`each.value`:                        {},
		`path.module`:                       {},
	} {
		n, c, o, ok := target(in)
		if got := (res{n, c, o, ok}); got != want {
			t.Errorf("target(%q) = %+v, want %+v", in, got, want)
		}
	}
}

func TestScanFile(t *testing.T) {
	src := `variable "a" {}

variable "b" {
  type = string
}

resource "aws_iam_policy" "p" {
  policy = <<EOF
{
  "x": "y"
}
EOF
}

locals {
  one = 1
  two = merge(
    var.a,
    # trailing comment
    {},
  )

  # about three
  three = <<-EOT
    text
  EOT
}

data "aws_ami" "u" {
  most_recent = true
}

module "m" {
  source = "./m"
}
`
	got := scanFile(src)
	want := map[string]located{
		"var.a":            {line: 1, end: 1},
		"var.b":            {line: 3, end: 5},
		"aws_iam_policy.p": {line: 7, end: 13},
		"local.one":        {line: 16, end: 16},
		"local.two":        {line: 17, end: 21},
		"local.three":      {line: 24, end: 26},
		"data.aws_ami.u":   {line: 29, end: 31},
		"module.m":         {line: 33, end: 35},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scanFile\n got %+v\nwant %+v", got, want)
	}
}

// TestAddGraph: a module call's argument node refers for the call, a
// reference to a called module's output names the call and that output, a
// reference to a call's own node (depends_on) names the call, and the
// graph's structural edges (a variable's reference node to its input node, a
// node to its own module's expand node, a provider) are none.
func TestAddGraph(t *testing.T) {
	dot := `digraph {
		"[root] local.name (expand)" -> "[root] var.prefix (expand, reference)"
		"[root] var.prefix (expand, reference)" -> "[root] var.prefix"
		"[root] module.net.var.vpc_id (expand, input)" -> "[root] aws_vpc.this (expand)"
		"[root] module.net.var.vpc_id (expand, input)" -> "[root] module.net (expand)"
		"[root] module.net.var.vpc_id (expand, reference)" -> "[root] module.net.var.vpc_id (expand, input)"
		"[root] module.net.aws_internet_gateway.gw (expand)" -> "[root] module.net.var.vpc_id (expand, reference)"
		"[root] module.net.aws_internet_gateway.gw (expand)" -> "[root] module.net (expand)"
		"[root] aws_vpc.this (expand)" -> "[root] provider[\"registry.opentofu.org/hashicorp/aws\"]"
		"[root] output.gw (expand)" -> "[root] module.net.output.gateway_id (expand)"
		"[root] aws_route.r (expand)" -> "[root] module.net (close)"
	}`
	c := &collector{defs: map[entity]string{}, refs: map[[2]entity]bool{}, inputs: map[[2]entity]bool{}, dirs: map[string]bool{}}
	if err := c.addGraph([]byte(dot), map[string]string{"": "stack", "net": "modules/net"}, map[[2]entity]bool{}); err != nil {
		t.Fatal(err)
	}
	var got []string
	for p := range c.refs {
		got = append(got, p[0].dir+":"+p[0].name+" -> "+p[1].dir+":"+p[1].name)
	}
	sort.Strings(got)
	want := []string{
		"modules/net:aws_internet_gateway.gw -> modules/net:var.vpc_id",
		"stack:aws_route.r -> stack:module.net",
		"stack:local.name -> stack:var.prefix",
		"stack:module.net -> stack:aws_vpc.this",
		"stack:output.gw -> modules/net:output.gateway_id",
		"stack:output.gw -> stack:module.net",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("refs\n got %q\nwant %q", got, want)
	}
	if c.defs[entity{"stack", "local.name"}] != "variable" {
		t.Errorf("local.name is not a definition: %v", c.defs)
	}
}
