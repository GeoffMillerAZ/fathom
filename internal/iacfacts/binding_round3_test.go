package iacfacts

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

var tlsLines = []IaCCatalogInfo{
	fxCat("registry.opentofu.org/hashicorp/tls", "3.4.0"),
	fxCat("registry.opentofu.org/hashicorp/tls", "4.0.0", "4.0.6"),
}

const (
	wantRefuse  = "refuse"
	wantUnbound = "unbound"
)

func tlsRequired(entry string) string {
	return "terraform {\n  required_providers {\n    tls = " + entry + "\n  }\n}\n"
}

// bindTLS binds a tls_private_key in the root of a one-file module and returns the version it chose, or wantRefuse
// for an answer refused for an unread input, or wantUnbound for no version.
func bindTLS(file, content string) string {
	b := bindBlock(fstest.MapFS{file: {Data: []byte(content)}}, tlsLines, IaCBlockRef{Dir: ".", Type: "tls_private_key"})
	switch {
	case b.Status == BindUnbound && b.Refusal != nil:
		return wantRefuse
	case b.Status == BindUnbound:
		return wantUnbound
	}
	return b.Version
}

// IAC6, IAC7, N10: every row is what `tofu init` did with the same file against a mirror holding tls 3.4.0, 4.0.0
// and 4.0.6 (OpenTofu 1.13, a filesystem mirror, so no network). An init that failed reads as a refusal, or as no
// version where the constraint admits nothing.
func TestBindingFollowsWhatTofuInitDid(t *testing.T) {
	for _, tc := range []struct {
		name, file, content, want string
	}{
		{"quoted keys", "main.tf", tlsRequired(`{ "source" = "hashicorp/tls", "version" = "4.0.0" }`), "4.0.0"},
		{"quoted version only", "main.tf", tlsRequired(`{ source = "hashicorp/tls", "version" = "~> 3.0" }`), "3.4.0"},
		{"colon keys", "main.tf", tlsRequired(`{ "source" : "hashicorp/tls", version : "4.0.0" }`), "4.0.0"},
		{"parenthesised key", "main.tf", tlsRequired(`{ (source) = "hashicorp/tls", version = "4.0.0" }`), wantRefuse},
		{"upper case key", "main.tf", tlsRequired(`{ SOURCE = "hashicorp/tls", version = "4.0.0" }`), wantRefuse},
		{"quoted upper case key", "main.tf", tlsRequired(`{ source = "hashicorp/tls", "Version" = "4.0.0" }`), wantRefuse},
		{"unknown key", "main.tf", tlsRequired(`{ source = "hashicorp/tls", other = "x" }`), wantRefuse},
		{"configuration_aliases", "main.tf", tlsRequired(`{ source = "hashicorp/tls", version = "4.0.0", configuration_aliases = [tls.a] }`), "4.0.0"},
		{"empty legacy constraint", "main.tf", tlsRequired(`""`), wantRefuse},
		{"empty version", "main.tf", tlsRequired(`{ source = "hashicorp/tls", version = "" }`), wantRefuse},
		{"empty source", "main.tf", tlsRequired(`{ source = "", version = "4.0.0" }`), wantRefuse},
		{"v prefix", "main.tf", tlsRequired(`{ source = "hashicorp/tls", version = "v4.0.6" }`), wantUnbound},
		{"v prefix after an operator", "main.tf", tlsRequired(`{ source = "hashicorp/tls", version = ">= v4.0.6" }`), wantUnbound},
		{"provider block version", "main.tf", "provider \"tls\" {\n  version = \"~> 3.0\"\n}\n", "3.4.0"},
		{"provider block beside an entry with no version", "main.tf", "provider \"tls\" {\n  version = \"~> 3.0\"\n}\n" + tlsRequired(`{ source = "hashicorp/tls" }`), "3.4.0"},
		{"provider block and entry both hold", "main.tf", "provider \"tls\" {\n  version = \">= 3.0, < 4.0\"\n}\n" + tlsRequired(`{ source = "hashicorp/tls", version = "~> 3.4.0" }`), "3.4.0"},
		{"provider block and entry disagree", "main.tf", "provider \"tls\" {\n  version = \"~> 3.0\"\n}\n" + tlsRequired(`{ source = "hashicorp/tls", version = "~> 4.0" }`), wantUnbound},
		{"two provider blocks combine", "main.tf", "provider \"tls\" {\n  version = \"<= 4.0.0\"\n}\nprovider \"tls\" {\n  alias   = \"b\"\n  version = \">= 3.4.1\"\n}\n", "4.0.0"},
		{"provider block version is a variable", "main.tf", "variable \"v\" {\n  default = \"3.4.0\"\n}\nprovider \"tls\" {\n  version = var.v\n}\n", wantRefuse},
		{"provider block version is empty", "main.tf", "provider \"tls\" {\n  version = \"\"\n}\n", wantRefuse},
		{"json provider block", "main.tf.json", `{"provider": {"tls": {"version": "~> 3.0"}}}`, "3.4.0"},
		{"json provider blocks in a list", "main.tf.json", `{"provider": {"tls": [{"version": "~> 3.0"}, {"alias": "b", "version": "<= 3.4.0"}]}}`, "3.4.0"},
		{"json provider list of maps", "main.tf.json", `{"provider": [{"tls": {"version": "~> 3.0"}}]}`, "3.4.0"},
		{"json entry", "main.tf.json", `{"terraform": {"required_providers": {"tls": {"source": "hashicorp/tls", "version": "4.0.0"}}}}`, "4.0.0"},
		{"json upper case key", "main.tf.json", `{"terraform": {"required_providers": {"tls": {"SOURCE": "hashicorp/tls", "version": "4.0.0"}}}}`, wantRefuse},
		{"json unknown key", "main.tf.json", `{"terraform": {"required_providers": {"tls": {"source": "hashicorp/tls", "other": "x"}}}}`, wantRefuse},
		{"json configuration_aliases", "main.tf.json", `{"terraform": {"required_providers": {"tls": {"source": "hashicorp/tls", "version": "4.0.0", "configuration_aliases": ["tls.a"]}}}}`, "4.0.0"},
		{"json empty version", "main.tf.json", `{"terraform": {"required_providers": {"tls": {"source": "hashicorp/tls", "version": ""}}}}`, wantRefuse},
		{"json provider block version is an expression", "main.tf.json", `{"provider": {"tls": {"version": "${var.v}"}}}`, wantRefuse},
		{"json terraform key repeated (tofu: duplicate required providers)", "main.tf.json", `{"terraform": {"required_providers": {"tls": {"source": "hashicorp/tls", "version": "3.4.0"}}}, "terraform": {"required_providers": {"tls": {"source": "hashicorp/tls", "version": "4.0.0"}}}}`, wantRefuse},
		{"json entry repeated (tofu: duplicate attribute)", "main.tf.json", `{"terraform": {"required_providers": {"tls": {"source": "hashicorp/tls", "version": "3.4.0"}, "tls": {"source": "hashicorp/tls", "version": "4.0.0"}}}}`, wantRefuse},
		{"json version repeated (tofu: the last wins, the reader refuses)", "main.tf.json", `{"terraform": {"required_providers": {"tls": {"source": "hashicorp/tls", "version": "3.4.0", "version": "4.0.0"}}}}`, wantRefuse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := bindTLS(tc.file, tc.content); got != tc.want {
				t.Errorf("want %s, got %s", tc.want, got)
			}
		})
	}
}

func TestBindingDisclosesAProviderBlockConstraint(t *testing.T) {
	b := bindBlock(fstest.MapFS{"main.tf": file("provider \"tls\" {\n  version = \"~> 3.0\"\n}\n")}, tlsLines, IaCBlockRef{Dir: ".", Type: "tls_private_key"})
	if b.Status != BindConstraintMatched || b.Version != "3.4.0" || b.Constraint != "~> 3.0" {
		t.Fatalf("%+v", b)
	}
	if !strings.Contains(strings.Join(b.Disclosures, "|"), "deprecates but honours") {
		t.Fatalf("the provider block constraint must be disclosed: %v", b.Disclosures)
	}
}

// denyFS refuses to list the named directories, as a wedged TCC verdict or a mode bit does.
type denyFS struct {
	fs.FS
	deny map[string]bool
}

func (d denyFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if d.deny[name] {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return fs.ReadDir(d.FS, name)
}

// IAC8: a directory the lockfile search could not list may hold the lockfile that reaches the module.
func TestBindingRefusesWhenTheLockfileSearchCannotListADirectory(t *testing.T) {
	repo := fstest.MapFS{
		"root/main.tf":         file(`module "m" { source = "../mod" }`),
		"root/" + LockfileName: lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
		"mod/main.tf":          file(reusableModule),
	}
	ref := IaCBlockRef{Dir: "mod", Type: "aws_lb_listener"}
	if b := bindBlock(repo, awsCatalogs, ref); b.Status != BindLocked || b.Version != "5.100.0" {
		t.Fatalf("the clean read must lock: %+v", b)
	}
	for _, deny := range []string{"root", "."} {
		b := ResolveIaCBinding(IaCBindingQuery{Repo: denyFS{FS: repo, deny: map[string]bool{deny: true}}, Catalogs: awsCatalogs, Block: ref})
		if b.Status != BindUnbound || b.Refusal == nil || b.Version != "" || !strings.Contains(b.Refusal.Reason, "could not be read during the lockfile search") {
			t.Errorf("deny %q: an unlistable directory must refuse, not read constraint-matched: %+v", deny, b)
		}
	}
}

// N4: OpenTofu reads a repeated top-level `module` key as two blocks.
func TestBindingReadsARepeatedJSONModuleKey(t *testing.T) {
	repo := fstest.MapFS{
		"main.tf.json": file(`{"module":{"a":{"source":"./a"}},"module":{"b":{"source":"./b"}}}`),
		"a/main.tf":    file(reusableModule),
		"b/main.tf":    file(reusableModule),
		LockfileName:   lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
	}
	for _, dir := range []string{"a", "b"} {
		b := bindBlock(repo, awsCatalogs, IaCBlockRef{Dir: dir, Type: "aws_lb_listener"})
		if b.Status != BindLocked || b.Version != "5.100.0" || len(b.Roots) != 1 || b.Roots[0].Via != "module call" {
			t.Errorf("%s: both module keys are calls: %+v", dir, b)
		}
	}
}

// N7: a lockfile pins the module graph of its own directory. One that no call reaches pins nothing for a module.
func TestBindingAnUnreachedLockfileIsNoPin(t *testing.T) {
	repo := fstest.MapFS{
		LockfileName:       lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
		"stacks/a/main.tf": file(reusableModule),
	}
	b := bindBlock(repo, awsCatalogs, IaCBlockRef{Dir: "stacks/a", Type: "aws_instance"})
	if b.Status != BindConstraintMatched || b.Version != "6.72.0" || len(b.Roots) != 0 {
		t.Fatalf("a module nothing calls is not locked by a lockfile that does not reach it: %+v", b)
	}
	joined := strings.Join(b.Disclosures, "|")
	if !strings.Contains(joined, "is not reached by any module call") || strings.Contains(joined, "roots reach") || strings.Contains(joined, "root reaches") {
		t.Fatalf("the disclosure must say the lockfile does not reach the module: %v", b.Disclosures)
	}
}

func TestBindingCountsTheRootsThatReachAModuleWithoutAPin(t *testing.T) {
	repo := fstest.MapFS{
		"main.tf":    file(`module "m" { source = "./m" }`),
		"m/main.tf":  file(reusableModule),
		LockfileName: lock("registry.terraform.io/hashicorp/random", "3.6.0"),
	}
	b := bindBlock(repo, awsCatalogs, IaCBlockRef{Dir: "m", Type: "aws_instance"})
	joined := strings.Join(b.Disclosures, "|")
	if b.Status != BindConstraintMatched || !strings.Contains(joined, "1 lockfile root reaches this module but it does not pin") || strings.Contains(joined, "1 lockfile roots") {
		t.Fatalf("%v", b)
	}
}

func TestBindingDisclosesAPinOutsideTheConstraint(t *testing.T) {
	mod := "terraform {\n  required_providers {\n    aws = { source = \"hashicorp/aws\", version = \"~> 6.0\" }\n  }\n}\n"
	repo := fstest.MapFS{"main.tf": file(mod), LockfileName: lock("registry.terraform.io/hashicorp/aws", "5.100.0")}
	b := bindBlock(repo, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_instance"})
	if b.Status != BindLocked || b.Version != "5.100.0" {
		t.Fatalf("%+v", b)
	}
	if !strings.Contains(strings.Join(b.Disclosures, "|"), "would stop at init") {
		t.Fatalf("a pin the constraint does not admit must be disclosed: %v", b.Disclosures)
	}
	ok := fstest.MapFS{"main.tf": file(reusableModule), LockfileName: lock("registry.terraform.io/hashicorp/aws", "5.100.0")}
	if b := bindBlock(ok, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_instance"}); strings.Contains(strings.Join(b.Disclosures, "|"), "would stop") {
		t.Fatalf("a pin inside the constraint needs no warning: %v", b.Disclosures)
	}
}

// N8: a lock entry the reader cannot read is unread, and never "none pins".
func TestBindingRefusesAnUnreadableLockEntry(t *testing.T) {
	for name, lockSrc := range map[string]string{
		"version is a variable": "provider \"registry.terraform.io/hashicorp/aws\" {\n  version = var.x\n}\n",
		"two labels":            "provider \"registry.terraform.io\" \"hashicorp/aws\" {\n  version = \"5.100.0\"\n}\n",
		"no version":            "provider \"registry.terraform.io/hashicorp/aws\" {\n  constraints = \"~> 5.0\"\n}\n",
	} {
		repo := fstest.MapFS{"main.tf": file(reusableModule), LockfileName: file(lockSrc)}
		b := bindBlock(repo, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_instance"})
		if b.Status != BindUnbound || b.Refusal == nil || b.Version != "" {
			t.Errorf("%s: want a refusal, got %+v", name, b)
		}
	}
}

// N10: OpenTofu never selects a pre-release for a range or for no constraint. Only an `=` that names it does.
func TestIaCConstraintPreReleases(t *testing.T) {
	pre, _ := ParseIaCVersion("6.1.0-beta1")
	stable, _ := ParseIaCVersion("6.0.0")
	for _, tc := range []struct {
		constraint string
		pre, stab  bool
	}{
		{"", false, true},
		{">= 6.0.0", false, true},
		{">= 6.1.0-beta1", false, false},
		{"> 6.0.0", false, false},
		{"!= 6.0.0", false, false},
		{"< 7.0.0", false, true},
		{"<= 6.1.0-beta1", false, true},
		{"~> 6.1.0-beta1", false, false},
		{">= 6.1.0-beta1, < 7.0.0", false, false},
		{"6.1.0-beta1", true, false},
		{"= 6.1.0-beta1, >= 6.0.0", true, false},
		{"= 6.1.0-beta1, != 6.0.0", true, false},
		{"= 6.1.0-beta1, < 7.0.0", true, false},
	} {
		c, err := ParseIaCConstraint(tc.constraint)
		if err != nil {
			t.Fatalf("%q: %v", tc.constraint, err)
		}
		if got := c.Matches(pre); got != tc.pre {
			t.Errorf("%q over 6.1.0-beta1: tofu init says %v, got %v", tc.constraint, tc.pre, got)
		}
		if got := c.Matches(stable); got != tc.stab {
			t.Errorf("%q over 6.0.0: tofu init says %v, got %v", tc.constraint, tc.stab, got)
		}
	}
	for _, bad := range []string{"v4.0.6", ">= v4.0.6", "= v1.0.0"} {
		if _, err := ParseIaCConstraint(bad); err == nil {
			t.Errorf("%q: tofu says Invalid version constraint", bad)
		}
	}
	// no constraint at all binds the highest stable release of an imported catalog, never a pre-release above it.
	cat := []IaCCatalogInfo{fxCat("registry.opentofu.org/hashicorp/tls", "6.0.0", "6.1.0-beta1")}
	b := bindBlock(fstest.MapFS{"main.tf": file(`resource "tls_private_key" "k" {}`)}, cat, IaCBlockRef{Dir: ".", Type: "tls_private_key"})
	if b.Status != BindConstraintMatched || b.Version != "6.0.0" {
		t.Fatalf("an empty constraint must not select 6.1.0-beta1: %+v", b)
	}
}
