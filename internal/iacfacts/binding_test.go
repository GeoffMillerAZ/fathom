package iacfacts

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

func fxCat(source string, versions ...string) IaCCatalogInfo {
	v, _ := ParseIaCVersion(versions[0])
	m := IaCCatalogManifest{Format: 1, Kind: KindTerraformProvider, Source: source, Line: fmt.Sprint(v.Major), Versions: versions}
	return IaCCatalogInfo{Name: CatalogFileName(m), Manifest: m}
}

func lock(entries ...string) *fstest.MapFile {
	var b strings.Builder
	for i := 0; i+1 < len(entries); i += 2 {
		fmt.Fprintf(&b, "provider %q {\n  version     = %q\n  constraints = \"~> 5.0\"\n  hashes = [\"h1:abc\"]\n}\n\n", entries[i], entries[i+1])
	}
	return &fstest.MapFile{Data: []byte(b.String())}
}

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

const reusableModule = `
terraform {
  required_providers {
    aws = { source = "hashicorp/aws", version = ">= 5.0, < 7.0" }
  }
}
resource "aws_lb_listener" "main" {}
`

const exampleCall = `module "m" { source = "../.." }`

var awsCatalogs = []IaCCatalogInfo{
	fxCat("registry.opentofu.org/hashicorp/aws", "5.99.0", "5.100.0"),
	fxCat("registry.opentofu.org/hashicorp/aws", "6.0.0", "6.72.0"),
}

func bindBlock(fsys fstest.MapFS, cats []IaCCatalogInfo, ref IaCBlockRef) IaCBinding {
	return ResolveIaCBinding(IaCBindingQuery{Repo: fsys, Catalogs: cats, Block: ref})
}

// The reusable module plus examples layout: the module has no lockfile, each example is a root with one.
func TestBindingReusableModulePlusExamples(t *testing.T) {
	ref := IaCBlockRef{Dir: ".", Type: "aws_lb_listener"}
	repo := func(basic, advanced string) fstest.MapFS {
		m := fstest.MapFS{
			"main.tf":                        file(reusableModule),
			"examples/basic/main.tf":         file(exampleCall),
			"examples/basic/" + LockfileName: lock("registry.terraform.io/hashicorp/aws", basic),
		}
		if advanced != "" {
			m["examples/advanced/main.tf"] = file(exampleCall)
			m["examples/advanced/"+LockfileName] = lock("registry.terraform.io/hashicorp/aws", advanced)
		}
		return m
	}

	t.Run("one root locks it", func(t *testing.T) {
		b := bindBlock(repo("5.100.0", ""), awsCatalogs, ref)
		if b.Status != BindLocked || b.Version != "5.100.0" || b.Source != "registry.terraform.io/hashicorp/aws" || len(b.Roots) != 1 || b.Roots[0].Via != "module call" || b.Roots[0].Dir != "examples/basic" {
			t.Fatalf("%+v", b)
		}
		if b.Catalog != awsCatalogs[0].Name {
			t.Fatalf("catalog %q", b.Catalog)
		}
		if !strings.Contains(strings.Join(b.Disclosures, "|"), "registry.opentofu.org") {
			t.Fatalf("the catalog's registry host differs from the lock's and must be disclosed: %v", b.Disclosures)
		}
	})

	t.Run("two roots that agree still lock it", func(t *testing.T) {
		b := bindBlock(repo("5.100.0", "5.100.0"), awsCatalogs, ref)
		if b.Status != BindLocked || b.Version != "5.100.0" || len(b.Roots) != 2 {
			t.Fatalf("%+v", b)
		}
	})

	t.Run("roots that disagree are ambiguous, with no first-root choice", func(t *testing.T) {
		b := bindBlock(repo("5.100.0", "6.72.0"), awsCatalogs, ref)
		if b.Status != BindAmbiguous || b.Version != "" || b.Catalog != "" || strings.Join(b.Candidates, ",") != "5.100.0,6.72.0" || len(b.Roots) != 2 {
			t.Fatalf("%+v", b)
		}
	})

	t.Run("no lockfile constraint-matches the highest satisfying release", func(t *testing.T) {
		m := fstest.MapFS{"main.tf": file(reusableModule)}
		b := bindBlock(m, awsCatalogs, ref)
		if b.Status != BindConstraintMatched || b.Version != "6.72.0" || b.Constraint != ">= 5.0, < 7.0" || len(b.Roots) != 0 {
			t.Fatalf("%+v", b)
		}
		m["main.tf"] = file(strings.Replace(reusableModule, ">= 5.0, < 7.0", "~> 5.0", 1))
		if b = bindBlock(m, awsCatalogs, ref); b.Version != "5.100.0" || b.Status != BindConstraintMatched {
			t.Fatalf("~> 5.0 should bind 5.100.0: %+v", b)
		}
		if !strings.Contains(strings.Join(b.Disclosures, "|"), "no root has been shown to install") {
			t.Fatalf("a constraint match must not read as locked: %v", b.Disclosures)
		}
	})

	t.Run("a locked version without a catalog is unbound and names the line, never a nearest version", func(t *testing.T) {
		b := bindBlock(repo("5.101.0", ""), awsCatalogs, ref)
		if b.Status != BindUnbound || b.Version != "" || b.Catalog != "" || b.AddLine != "5" || b.Locked != "5.101.0" {
			t.Fatalf("%+v", b)
		}
		if d := strings.Join(b.Disclosures, "|"); !strings.Contains(d, "line 5") && !strings.Contains(d, "the 5 line") || !strings.Contains(d, "no nearest version") {
			t.Fatalf("disclosure %v", b.Disclosures)
		}
		b = bindBlock(repo("4.67.0", ""), awsCatalogs, ref)
		if b.Status != BindUnbound || b.AddLine != "4" || !strings.Contains(strings.Join(b.Disclosures, "|"), "major line 4") {
			t.Fatalf("%+v", b)
		}
	})
}

func TestBindingNoCatalogForTheConstraint(t *testing.T) {
	m := fstest.MapFS{"main.tf": file(strings.Replace(reusableModule, ">= 5.0, < 7.0", "~> 4.0", 1))}
	b := bindBlock(m, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_instance"})
	if b.Status != BindUnbound || b.AddLine != "4" || b.Version != "" {
		t.Fatalf("%+v", b)
	}
	b = bindBlock(m, nil, IaCBlockRef{Dir: ".", Type: "aws_instance"})
	if b.Status != BindUnbound {
		t.Fatalf("%+v", b)
	}
}

func TestBindingAliasesAndLocalNames(t *testing.T) {
	m := fstest.MapFS{
		"main.tf": file(`
terraform {
  required_providers {
    google      = { source = "hashicorp/google", version = "~> 6.0" }
    google-beta = { source = "hashicorp/google-beta", version = "~> 6.0" }
  }
}
provider "google-beta" { alias = "eu" }
resource "google_compute_instance" "a" {}
resource "google_compute_instance" "b" { provider = google-beta.eu }
`),
	}
	cats := []IaCCatalogInfo{
		fxCat("registry.terraform.io/hashicorp/google", "6.1.0", "6.50.0"),
		fxCat("registry.terraform.io/hashicorp/google-beta", "6.2.0"),
	}
	plain := bindBlock(m, cats, IaCBlockRef{Dir: ".", Type: "google_compute_instance"})
	if plain.Source != "registry.terraform.io/hashicorp/google" || plain.Version != "6.50.0" || plain.LocalName != "google" {
		t.Fatalf("%+v", plain)
	}
	beta := bindBlock(m, cats, IaCBlockRef{Dir: ".", Type: "google_compute_instance", Provider: "google-beta.eu"})
	if beta.Source != "registry.terraform.io/hashicorp/google-beta" || beta.Version != "6.2.0" || beta.LocalName != "google-beta" || beta.Alias != "eu" {
		t.Fatalf("the provider meta-argument names the provider, not the type prefix: %+v", beta)
	}
}

func TestBindingImpliedProviderIsDisclosed(t *testing.T) {
	m := fstest.MapFS{
		"main.tf":                    file(`resource "aws_s3_bucket" "b" {}`),
		"examples/x/main.tf":         file(`module "m" { source = "../.." }`),
		"examples/x/" + LockfileName: lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
	}
	b := bindBlock(m, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_s3_bucket"})
	if b.Status != BindLocked || b.Source != "registry.terraform.io/hashicorp/aws" {
		t.Fatalf("%+v", b)
	}
	if !strings.Contains(strings.Join(b.Disclosures, "|"), "implies") {
		t.Fatalf("Terraform's implied provider must be disclosed: %v", b.Disclosures)
	}
}

func TestBindingModuleProviderMappings(t *testing.T) {
	childDecl := `
terraform {
  required_providers {
    aws = { source = "hashicorp/aws" }
  }
}
resource "aws_vpc" "v" {}
`
	repo := func(parentDecl string) fstest.MapFS {
		return fstest.MapFS{
			"main.tf": file(parentDecl + `
module "net" {
  source    = "./modules/net"
  providers = { aws = amazon, aws.east = amazon.us }
}`),
			"modules/net/main.tf": file(childDecl),
			LockfileName:          lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
		}
	}
	ref := IaCBlockRef{Dir: "modules/net", Type: "aws_vpc"}
	ok := bindBlock(repo(amazonDecl("hashicorp/aws")), awsCatalogs, ref)
	if ok.Status != BindLocked || ok.Version != "5.100.0" || len(ok.Roots) != 1 || ok.Roots[0].Dir != "." {
		t.Fatalf("a mapped child resolves through the root lockfile: %+v", ok)
	}
	aliased := bindBlock(repo(amazonDecl("hashicorp/aws")), awsCatalogs, IaCBlockRef{Dir: "modules/net", Type: "aws_vpc", Provider: "aws.east"})
	if aliased.Status != BindLocked {
		t.Fatalf("%+v", aliased)
	}
	bad := bindBlock(repo(amazonDecl("acme/aws")), awsCatalogs, ref)
	if bad.Status != BindAmbiguous || !strings.Contains(strings.Join(bad.Disclosures, "|"), "Terraform rejects") {
		t.Fatalf("a mapping to a different provider has no honest binding: %+v", bad)
	}
}

func TestBindingNearestLockfileFallback(t *testing.T) {
	m := fstest.MapFS{
		LockfileName:       lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
		"stacks/a/main.tf": file(reusableModule),
	}
	b := bindBlock(m, awsCatalogs, IaCBlockRef{Dir: "stacks/a", Type: "aws_instance"})
	if b.Status != BindLocked || b.Version != "5.100.0" || b.Roots[0].Via != "nearest lockfile" {
		t.Fatalf("%+v", b)
	}
}

func TestBindingBoundsAreDisclosed(t *testing.T) {
	m := fstest.MapFS{"main.tf": file(reusableModule)}
	for i := 0; i < MaxBindingLockfiles+2; i++ {
		m[fmt.Sprintf("roots/r%02d/%s", i, LockfileName)] = lock("registry.terraform.io/hashicorp/aws", "5.100.0")
	}
	b := bindBlock(m, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_instance"})
	if b.Status != BindUnbound || b.Refusal == nil || b.Refusal.Code != RefuseBindingBound || b.Version != "" || b.Catalog != "" {
		t.Fatalf("more lockfiles than one answer reads must refuse, not answer: %+v", b)
	}
	if !strings.Contains(b.Refusal.Reason, "18 lockfiles") {
		t.Fatalf("the refusal must say why: %v", b.Refusal)
	}
	big := fstest.MapFS{"main.tf": &fstest.MapFile{Data: []byte(strings.Repeat("# pad\n", maxBindingFileBytes/6+10))}}
	b = bindBlock(big, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_instance"})
	if !strings.Contains(strings.Join(b.Disclosures, "|"), "read bound") {
		t.Fatalf("%v", b.Disclosures)
	}
}

// M13: a bound that leaves another root able to reach the block refuses the answer, whichever bound it is.
func TestBindingWalkStopRefusesTheAnswer(t *testing.T) {
	var calls strings.Builder
	for i := 0; i < maxBindingModules+6; i++ {
		fmt.Fprintf(&calls, "module \"m%d\" { source = \"./m%d\" }\n", i, i)
	}
	m := fstest.MapFS{
		"root/main.tf":         file(calls.String()),
		"root/" + LockfileName: lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
		"elsewhere/main.tf":    file(reusableModule),
		"root/m3/main.tf":      file(reusableModule),
	}
	b := bindBlock(m, awsCatalogs, IaCBlockRef{Dir: "elsewhere", Type: "aws_lb_listener"})
	if b.Status != BindUnbound || b.Refusal == nil || b.Refusal.Code != RefuseBindingBound || b.Version != "" || !strings.Contains(b.Refusal.Reason, "root") {
		t.Fatalf("a stopped module walk must refuse, not fall back to a constraint match: %+v", b)
	}
	// a block the walk did reach before it stopped is still answered.
	got := bindBlock(m, awsCatalogs, IaCBlockRef{Dir: "root/m3", Type: "aws_lb_listener"})
	if got.Status != BindLocked || got.Version != "5.100.0" {
		t.Fatalf("%+v", got)
	}

	// the depth bound refuses the same way.
	deep := fstest.MapFS{
		"root/main.tf":         file(`module "d0" { source = "./d0" }`),
		"root/" + LockfileName: lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
		"elsewhere/main.tf":    file(reusableModule),
	}
	for i := 0; i < maxBindingModuleDepth+2; i++ {
		deep[fmt.Sprintf("root/d%d/main.tf", i)] = file(fmt.Sprintf(`module "n" { source = "../d%d" }`, i+1))
	}
	b = bindBlock(deep, awsCatalogs, IaCBlockRef{Dir: "elsewhere", Type: "aws_lb_listener"})
	if b.Status != BindUnbound || b.Refusal == nil || b.Refusal.Code != RefuseBindingBound {
		t.Fatalf("a depth stop must refuse: %+v", b)
	}
}

func TestBindingBuiltinTerraformProvider(t *testing.T) {
	b := bindBlock(fstest.MapFS{}, awsCatalogs, IaCBlockRef{Dir: ".", Type: "terraform_remote_state"})
	if b.Status != BindUnbound || len(b.Disclosures) == 0 {
		t.Fatalf("%+v", b)
	}
}

// TestBindingReadsInstalledCatalogs binds against catalogs that ListIaCCatalogs read off disk.
func TestBindingReadsInstalledCatalogs(t *testing.T) {
	dir := t.TempDir()
	p := twoVersionCatalog(t)
	if _, err := AddIaCCatalog(dir, p); err != nil {
		t.Fatal(err)
	}
	cats, bad := ListIaCCatalogs(dir, false)
	if len(bad) != 0 || len(cats) != 1 {
		t.Fatalf("%v %v", cats, bad)
	}
	m := fstest.MapFS{"main.tf": file("terraform {\n  required_providers {\n    widget = { source = \"registry.opentofu.org/example/widget\", version = \"~> 1.0\" }\n  }\n}\n")}
	b := bindBlock(m, cats, IaCBlockRef{Dir: ".", Type: "widget_thing"})
	if b.Status != BindConstraintMatched || b.Version != "1.1.0" || b.Catalog != cats[0].Name {
		t.Fatalf("%+v", b)
	}
}

func TestIaCConstraints(t *testing.T) {
	for _, tc := range []struct {
		c    string
		v    string
		want bool
	}{
		{"", "5.0.0", true},
		{">= 5.0, < 6.0", "5.100.0", true},
		{">= 5.0, < 6.0", "6.0.0", false},
		{"~> 5.1", "5.1.0", true},
		{"~> 5.1", "5.99.0", true},
		{"~> 5.1", "6.0.0", false},
		{"~> 5.1", "5.0.9", false},
		{"~> 5.1.2", "5.1.9", true},
		{"~> 5.1.2", "5.2.0", false},
		// OpenTofu's Ruby-style rule: one or two written parts hold the major (ruby_style.go; `tofu init` with
		// `~> 3` printed `matching "~> 3.0"` and installed 3.4.0 while 4.x existed).
		{"~> 5", "9.0.0", false},
		{"~> 5", "6.0.0", false},
		{"~> 5", "5.0.0", true},
		{"~> 5", "5.99.1", true},
		{"~> 3", "3.4.0", true},
		{"~> 3", "4.0.0", false},
		{"~> 5.0", "5.7.0", true},
		{"~> 5.0", "6.0.0", false},
		{"= 5.1.0", "5.1.0", true},
		{"5.1.0", "5.1.1", false},
		{"!= 5.1.0", "5.1.0", false},
		{"> 5.1.0, <= 5.3.0", "5.3.0", true},
		{">= 5.0", "6.0.0-rc1", false},
		{">= 6.0.0-rc1", "6.0.0-rc2", true},
	} {
		c, err := ParseIaCConstraint(tc.c)
		if err != nil {
			t.Fatalf("%q: %v", tc.c, err)
		}
		v, _ := ParseIaCVersion(tc.v)
		if got := c.Matches(v); got != tc.want {
			t.Errorf("%q matches %s = %v, want %v", tc.c, tc.v, got, tc.want)
		}
	}
	one, _ := ParseIaCConstraint("~> 5")
	two, _ := ParseIaCConstraint("~> 5.0")
	for _, v := range []string{"4.9.0", "5.0.0", "5.8.1", "6.0.0", "7.1.0"} {
		pv, _ := ParseIaCVersion(v)
		if one.Matches(pv) != two.Matches(pv) {
			t.Errorf("`~> 5` and `~> 5.0` must agree at %s", v)
		}
	}
	if _, err := ParseIaCConstraint(">= banana"); err == nil {
		t.Fatal("a malformed constraint must not parse")
	}
	a, _ := ParseIaCVersion("5.9.0")
	b, _ := ParseIaCVersion("5.10.0")
	if CompareIaCVersions(a, b) >= 0 {
		t.Fatal("5.9.0 sorts before 5.10.0")
	}
}

func amazonDecl(src string) string {
	return "terraform {\n  required_providers {\n    amazon = { source = \"" + src + "\" }\n  }\n}\n"
}

// tlsLockfile is the .terraform.lock.hcl `tofu init` 1.13.0 wrote for `tls = { source = "hashicorp/tls", version =
// "~> 3" }` (hashes trimmed). OpenTofu keys the entry on registry.opentofu.org although the source says no host.
const tlsLockfile = `# This file is maintained automatically by "tofu init".
# Manual edits may be lost in future updates.

provider "registry.opentofu.org/hashicorp/tls" {
  version     = "3.4.0"
  constraints = "~> 3.0"
  hashes = [
    "h1:/inWZqMupribkExU5ojt2pb91yTdcRjNdecxd8JaljQ=",
    "zh:065536e53db5b165a33d5f08dd2c8d95c26fafd07482e686fbf2b4a1b576bb71",
  ]
}
`

var tlsCatalogs = []IaCCatalogInfo{
	fxCat("registry.opentofu.org/hashicorp/tls", "3.1.0", "3.4.0"),
	fxCat("registry.opentofu.org/hashicorp/tls", "4.0.0", "4.2.1"),
}

// M11: a real OpenTofu lockfile binds the version it pins, not a constraint match on a later major line.
func TestBindingReadsARealOpenTofuLockfile(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"shorthand", `"hashicorp/tls"`},
		{"upper case", `"HashiCorp/TLS"`},
		{"bare name", `"tls"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fstest.MapFS{
				"main.tf":    file("terraform {\n  required_providers {\n    tls = { source = " + tc.src + ", version = \">= 3.0\" }\n  }\n}\nresource \"tls_private_key\" \"k\" {}\n"),
				LockfileName: file(tlsLockfile),
			}
			b := bindBlock(m, tlsCatalogs, IaCBlockRef{Dir: ".", Type: "tls_private_key"})
			if b.Status != BindLocked || b.Version != "3.4.0" || b.Catalog != tlsCatalogs[0].Name {
				t.Fatalf("the lockfile pins 3.4.0 and must win over the highest release: %+v", b)
			}
			if b.Source != "registry.terraform.io/hashicorp/tls" || len(b.Roots) != 1 || b.Roots[0].Version != "3.4.0" {
				t.Fatalf("%+v", b)
			}
			if strings.Contains(strings.Join(b.Disclosures, "|"), "no lockfile reaches") {
				t.Fatalf("a lockfile sits in the same directory: %v", b.Disclosures)
			}
		})
	}
	// an explicit host stays exact: a source that names registry.terraform.io does not read an opentofu.org key.
	m := fstest.MapFS{
		"main.tf":    file("terraform {\n  required_providers {\n    tls = { source = \"registry.terraform.io/hashicorp/tls\" }\n  }\n}\nresource \"tls_private_key\" \"k\" {}\n"),
		LockfileName: file(tlsLockfile),
	}
	b := bindBlock(m, tlsCatalogs, IaCBlockRef{Dir: ".", Type: "tls_private_key"})
	if b.Status != BindConstraintMatched || b.Version != "4.2.1" {
		t.Fatalf("an explicit host matches its own lock key only: %+v", b)
	}
	// both default-registry keys pinning different versions is ambiguous, not a pick.
	both := lock("registry.terraform.io/hashicorp/tls", "4.0.0", "registry.opentofu.org/hashicorp/tls", "3.4.0")
	b = bindBlock(fstest.MapFS{"main.tf": file(`resource "tls_private_key" "k" {}`), LockfileName: both}, tlsCatalogs, IaCBlockRef{Dir: ".", Type: "tls_private_key"})
	if b.Status != BindAmbiguous || b.Version != "" {
		t.Fatalf("%+v", b)
	}
}

// M15: a host other than the two default registries reads unbound against a public catalog.
func TestBindingPrivateRegistryHostNeedsItsOwnCatalog(t *testing.T) {
	src := "registry.corp.example/hashicorp/aws"
	m := fstest.MapFS{
		"main.tf":    file("terraform {\n  required_providers {\n    aws = { source = \"" + src + "\" }\n  }\n}\nresource \"aws_vpc\" \"v\" {}\n"),
		LockfileName: lock(src, "5.100.0"),
	}
	b := bindBlock(m, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_vpc"})
	if b.Status != BindUnbound || b.Version != "" || b.Catalog != "" || b.Locked != "5.100.0" {
		t.Fatalf("a private host must not bind a public catalog: %+v", b)
	}
	if d := strings.Join(b.Disclosures, "|"); !strings.Contains(d, "registry.corp.example") || strings.Contains(d, "IF-P2") {
		t.Fatalf("%v", b.Disclosures)
	}
	own := append([]IaCCatalogInfo{fxCat(src, "5.99.0", "5.100.0")}, awsCatalogs...)
	if b = bindBlock(m, own, IaCBlockRef{Dir: ".", Type: "aws_vpc"}); b.Status != BindLocked || b.Catalog != own[0].Name {
		t.Fatalf("a catalog for exactly that host binds: %+v", b)
	}
	// the same holds without a lockfile.
	m2 := fstest.MapFS{"main.tf": m["main.tf"]}
	if b = bindBlock(m2, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_vpc"}); b.Status != BindUnbound || !strings.Contains(strings.Join(b.Disclosures, "|"), "registry.corp.example") {
		t.Fatalf("%+v", b)
	}
}

// M16: binding reads .tofu files with the reader's shadowing.
func TestBindingReadsTofuFilesWithTheReadersShadowing(t *testing.T) {
	m := fstest.MapFS{
		"versions.tf":        file("terraform {\n  required_providers {\n    aws = { source = \"hashicorp/aws\" }\n    zed = { source = \"acme/zed\" }\n  }\n}\n"),
		"versions.tofu":      file("terraform {\n  required_providers {\n    aws = { source = \"acme/aws\" }\n  }\n}\n"),
		"main.tf":            file(`resource "aws_vpc" "v" {}`),
		LockfileName:         lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
		"extra.tofu":         file(`module "x" { source = "./x" }`),
		"x/main.tf":          file(`resource "aws_vpc" "v" {}`),
		"x/main_override.tf": file("terraform {\n  required_providers {\n    aws = { source = \"hashicorp/aws\", version = \"~> 5.0\" }\n  }\n}\n"),
	}
	b := bindBlock(m, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_vpc"})
	if b.Source != "registry.terraform.io/acme/aws" || b.Status != BindUnbound {
		t.Fatalf("versions.tofu shadows versions.tf, so the source is acme/aws: %+v", b)
	}
	if z := bindBlock(m, awsCatalogs, IaCBlockRef{Dir: ".", Type: "zed_thing"}); z.Source != "registry.terraform.io/hashicorp/zed" {
		t.Fatalf("a shadowed versions.tf declares nothing: %+v", z)
	}
	only := fstest.MapFS{"versions.tf": m["versions.tf"], "main.tf": m["main.tf"], LockfileName: m[LockfileName]}
	if b = bindBlock(only, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_vpc"}); b.Status != BindLocked {
		t.Fatalf("%+v", b)
	}
	// a module call written in a .tofu file is followed, and an override file amends required_providers.
	if b = bindBlock(m, awsCatalogs, IaCBlockRef{Dir: "x", Type: "aws_vpc"}); len(b.Roots) != 1 || b.Roots[0].Via != "module call" || b.Constraint != "~> 5.0" {
		t.Fatalf("%+v", b)
	}
}

// M17: a constraint that cannot be read is unbound with the reason, never "no constraint".
func TestBindingUnreadableConstraintIsUnbound(t *testing.T) {
	m := fstest.MapFS{"main.tf": file(strings.Replace(reusableModule, ">= 5.0, < 7.0", ">= banana", 1))}
	b := bindBlock(m, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"})
	if b.Status != BindUnbound || b.Version != "" || b.Catalog != "" || !strings.Contains(strings.Join(b.Disclosures, "|"), "unreadable") || !strings.Contains(strings.Join(b.Disclosures, "|"), "banana") {
		t.Fatalf("%+v", b)
	}
	// a lockfile pin still binds, and the unreadable constraint is disclosed.
	m[LockfileName] = lock("registry.terraform.io/hashicorp/aws", "5.100.0")
	if b = bindBlock(m, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"}); b.Status != BindLocked || !strings.Contains(strings.Join(b.Disclosures, "|"), "unreadable") {
		t.Fatalf("%+v", b)
	}
}

// N8: a second module call with a mismatched mapping, a root with no entry, and host preference at a tie.
func TestBindingEdgeCases(t *testing.T) {
	childDecl := "terraform {\n  required_providers {\n    aws = { source = \"hashicorp/aws\" }\n  }\n}\nresource \"aws_vpc\" \"v\" {}\n"
	m := fstest.MapFS{
		"main.tf": file(amazonDecl("hashicorp/aws") + `
module "good" {
  source    = "./modules/net"
  providers = { aws = amazon }
}
module "bad" {
  source    = "./modules/net"
  providers = { aws = other }
}`),
		"modules/net/main.tf": file(childDecl),
		LockfileName:          lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
	}
	m["main.tf"] = file(string(m["main.tf"].Data) + "\nterraform {\n  required_providers {\n    other = { source = \"acme/aws\" }\n  }\n}\n")
	b := bindBlock(m, awsCatalogs, IaCBlockRef{Dir: "modules/net", Type: "aws_vpc"})
	if b.Status != BindAmbiguous || !strings.Contains(strings.Join(b.Disclosures, "|"), "Terraform rejects") {
		t.Fatalf("every call into the module is checked, not the first: %+v", b)
	}

	// one reaching root pins, another has no entry for the provider: not locked.
	two := fstest.MapFS{
		"main.tf":                    file(reusableModule),
		"examples/a/main.tf":         file(exampleCall),
		"examples/a/" + LockfileName: lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
		"examples/b/main.tf":         file(exampleCall),
		"examples/b/" + LockfileName: lock("registry.terraform.io/hashicorp/other", "1.0.0"),
	}
	b = bindBlock(two, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"})
	if b.Status != BindAmbiguous || b.Version != "" || strings.Join(b.Candidates, ",") != "5.100.0" || !strings.Contains(strings.Join(b.Disclosures, "|"), "no entry in examples/b") {
		t.Fatalf("a root without the provider breaks 'every root pins one version': %+v", b)
	}
	// every reaching root lacks an entry: the reason does not claim no lockfile reaches.
	none := fstest.MapFS{"main.tf": two["main.tf"], "examples/b/main.tf": two["examples/b/main.tf"], "examples/b/" + LockfileName: two["examples/b/"+LockfileName]}
	b = bindBlock(none, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"})
	if b.Status != BindConstraintMatched || strings.Contains(strings.Join(b.Disclosures, "|"), "no lockfile reaches") || !strings.Contains(strings.Join(b.Disclosures, "|"), "none pins") {
		t.Fatalf("%+v", b)
	}

	// equal versions in two catalogs: the one built from the block's own host wins, whatever the name order.
	cats := []IaCCatalogInfo{
		fxCat("registry.opentofu.org/hashicorp/aws", "5.100.0"),
		fxCat("registry.terraform.io/hashicorp/aws", "5.100.0"),
	}
	cats[0].Name, cats[1].Name = "a-opentofu", "b-terraform"
	b = bindBlock(fstest.MapFS{"main.tf": file(reusableModule)}, cats, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"})
	if b.Status != BindConstraintMatched || b.Catalog != "b-terraform" {
		t.Fatalf("%+v", b)
	}
}
