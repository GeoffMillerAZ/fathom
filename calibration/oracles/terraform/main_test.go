package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
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

func TestCheckCompleteness(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		roots, failed, missing int
		wantError              bool
	}{
		{"complete nested modules", 19, 0, 0, false},
		{"no roots", 0, 0, 0, true},
		{"one nested root failed", 19, 1, 0, true},
		{"nested definition unlocated", 19, 0, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkCompleteness(tc.roots, tc.failed, tc.missing)
			if (err != nil) != tc.wantError {
				t.Fatalf("checkCompleteness(%d, %d, %d) error = %v, wantError %t", tc.roots, tc.failed, tc.missing, err, tc.wantError)
			}
		})
	}
}

func TestModuleDirsNestedAndExternal(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "src")
	wd := filepath.Join(src, "examples", "demo")
	manifest := filepath.Join(base, "modules.json")
	if err := os.MkdirAll(wd, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"Modules":[{"Key":"","Dir":"."},{"Key":"net","Dir":"../../modules/net"},{"Key":"net.child","Dir":"../../modules/net/child"},{"Key":"remote","Dir":"../../../external/remote"}]}`
	if err := os.WriteFile(manifest, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := moduleDirs(manifest, src, wd)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"": "examples/demo", "net": "modules/net", "net.child": "modules/net/child", "remote": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nested module paths = %#v, want %#v", got, want)
	}
}

func TestTofuEnvIsolated(t *testing.T) {
	for name, value := range map[string]string{
		"HOME": "/personal-home", "AWS_ACCESS_KEY_ID": "secret-aws", "AWS_PROFILE": "personal",
		"ARM_CLIENT_SECRET": "secret-azure", "GOOGLE_APPLICATION_CREDENTIALS": "/personal-key.json",
		"TF_TOKEN_registry_opentofu_org": "secret-token", "HTTPS_PROXY": "http://live-proxy:8080",
		"PATH": "/personal-bin", "CUSTOM_SECRET": "secret-custom",
	} {
		t.Setenv(name, value)
	}
	scratch := t.TempDir()
	env := tofuEnv(filepath.Join(scratch, "tofurc"), filepath.Join(scratch, "data"))
	values := map[string]string{}
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("invalid environment entry: %q", entry)
		}
		if _, exists := values[name]; exists {
			t.Fatalf("duplicate environment key: %s", name)
		}
		values[name] = value
	}
	for _, name := range []string{"AWS_ACCESS_KEY_ID", "AWS_PROFILE", "ARM_CLIENT_SECRET",
		"GOOGLE_APPLICATION_CREDENTIALS", "TF_TOKEN_registry_opentofu_org", "CUSTOM_SECRET"} {
		if _, exists := values[name]; exists {
			t.Fatalf("inherited %s", name)
		}
	}
	if values["HOME"] != filepath.Join(scratch, "home") || values["TMPDIR"] != filepath.Join(scratch, "tmp") {
		t.Fatalf("HOME/TMPDIR are not scratch-owned: %v", values)
	}
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		if values[name] != "http://localhost:9" {
			t.Fatalf("%s = %q, want dead proxy", name, values[name])
		}
	}
	if values["PATH"] != "/usr/bin:/bin:/usr/sbin:/sbin" || values["TF_INPUT"] != "0" {
		t.Fatalf("unsafe tool environment: %v", values)
	}
}

func TestRunOnceReceivesOnlyIsolatedEnv(t *testing.T) {
	t.Setenv("AWS_SECRET_ACCESS_KEY", "canary-secret")
	dir := t.TempDir()
	env := tofuEnv(filepath.Join(dir, "tofurc"), filepath.Join(dir, "data"))
	stdout, err := runOnce(context.Background(), "/usr/bin/env", dir, env)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stdout), "AWS_SECRET_ACCESS_KEY") || strings.Contains(string(stdout), "canary-secret") {
		t.Fatal("subprocess inherited cloud credential")
	}
	if !strings.Contains(string(stdout), "HOME="+filepath.Join(dir, "home")) {
		t.Fatal("subprocess lacked scratch-owned HOME")
	}
}
