package iacfacts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// writeTree writes files under a fresh directory and returns it.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func symlink(t *testing.T, root, target, link string) {
	t.Helper()
	if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(link))); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
}

func resourceNames(set *ResourceSet) string {
	var names []string
	for _, r := range set.Resources {
		names = append(names, r.Name)
	}
	return strings.Join(names, ",")
}

// M8: the module file set follows OpenTofu 1.13 as measured with `tofu show -json -config`: a symbolic link to a file
// is read, a dot file is not, and a JSON file is not read by this reader and says so.
func TestReadTerraformModuleFileSetMatchesTofu(t *testing.T) {
	root := writeTree(t, map[string]string{
		"shared.tf":        "resource \"t\" \"fromlink\" {}\nvariable \"x\" {\n  default = \"sym\"\n}\n",
		"mod/main.tf":      "resource \"t\" \"n\" {\n  v = var.x\n  a = 1\n}\n",
		"mod/.hidden.tf":   "resource \"t\" \"hidden\" {}\n",
		"mod/backup.tf~":   "resource \"t\" \"backup\" {}\n",
		"mod/#lock.tf#":    "resource \"t\" \"editor\" {}\n",
		"mod/plain.tf.txt": "resource \"t\" \"txt\" {}\n",
	})
	symlink(t, root, "../shared.tf", "mod/link.tf")
	fsys := os.DirFS(root)

	set, err := ReadTerraformModule(fsys, "mod", WithValues())
	if err != nil {
		t.Fatal(err)
	}
	if got := resourceNames(set); got != "fromlink,n" {
		t.Fatalf("the linked file is read and the dot and backup files are not: %s", got)
	}
	if set.SyntaxErrors != 0 {
		t.Fatalf("syntax errors %d", set.SyntaxErrors)
	}
	for _, r := range set.Resources {
		if r.Name == "n" {
			if a := attrOf(t, r, "v"); a.Class != ClassDefaulted || a.Value != "sym" {
				t.Errorf("var.x must resolve through the linked file: %+v", a)
			}
		}
	}

	// a link that points nowhere, and one that points at a directory, are counted.
	symlink(t, root, "../nope.tf", "mod/gone.tf")
	symlink(t, root, "..", "mod/dir.tf")
	set, err = ReadTerraformModule(fsys, "mod")
	if err != nil || set.SyntaxErrors != 2 || resourceNames(set) != "fromlink,n" {
		t.Fatalf("%v syntax %d resources %s", err, set.SyntaxErrors, resourceNames(set))
	}
}

func TestReadTerraformModuleJSONFiles(t *testing.T) {
	base := "resource \"t\" \"n\" {\n  a = 1\n  b = 2\n  blk {\n    c = 3\n  }\n}\nvariable \"v\" {\n  default = \"d\"\n}\nlocals {\n  l = \"x\"\n}\nresource \"t\" \"m\" {\n  p = var.v\n  q = local.l\n  r = 4\n}\n"
	// a JSON file adds blocks this reader does not see: it counts, and says so.
	fsys := fstest.MapFS{"main.tf": file(base), "extra.tf.json": file(`{"resource": {"t": {"added": {"a": 1}}}}`)}
	set, err := ReadTerraformModule(fsys, ".")
	if err != nil || set.SyntaxErrors != 1 || resourceNames(set) != "n,m" {
		t.Fatalf("%v syntax %d resources %s", err, set.SyntaxErrors, resourceNames(set))
	}
	// .tofu.json shadows .tf.json of the same name.
	fsys = fstest.MapFS{"main.tf": file(base), "x.tf.json": file(`{}`), "x.tofu.json": file(`{}`)}
	if set, err = ReadTerraformModule(fsys, "."); err != nil || set.SyntaxErrors != 1 {
		t.Fatalf("one JSON file is read: %v %d", err, set.SyntaxErrors)
	}

	// an override file in the JSON syntax marks what it names unknown: tofu reports 2 where the base writes 1.
	over := `{
  "resource": {"t": {"n": {"a": 2, "blk": [{"c": 4}], "count": 2, "provider": "aws.east"}, "ghost": {"z": 1}}},
  "variable": {"v": {"default": "other"}},
  "locals": [{"l": "y"}]
}`
	fsys = fstest.MapFS{"main.tf": file(base), "main_override.tf.json": file(over)}
	set, err = ReadTerraformModule(fsys, ".", WithValues())
	if err != nil || set.SyntaxErrors != 0 || resourceNames(set) != "n,m" {
		t.Fatalf("%v syntax %d resources %s", err, set.SyntaxErrors, resourceNames(set))
	}
	n, m := set.Resources[0], set.Resources[1]
	for _, p := range []string{"a", "blk[*].c"} {
		if a := attrOf(t, n, p); a.Class != ClassUnknown || a.Value != nil {
			t.Errorf("n.%s: %+v", p, a)
		}
	}
	if a := attrOf(t, n, "b"); a.Class != ClassLiteral || a.Value != int64(2) {
		t.Errorf("n.b is not named by the override: %+v", a)
	}
	if n.Multiplicity.Kind != MultiCount || n.Multiplicity.Class != ClassUnknown || n.Provider != "aws.east" {
		t.Errorf("n meta: %+v %q", n.Multiplicity, n.Provider)
	}
	for _, p := range []string{"p", "q"} {
		if a := attrOf(t, m, p); a.Class != ClassUnknown || a.Value != nil {
			t.Errorf("m.%s must read unknown through the overridden variable and local: %+v", p, a)
		}
	}
	if a := attrOf(t, m, "r"); a.Value != int64(4) {
		t.Errorf("m.r: %+v", a)
	}
	// an override file that is not JSON counts.
	fsys = fstest.MapFS{"main.tf": file(base), "main_override.tf.json": file(`{`)}
	if set, err = ReadTerraformModule(fsys, "."); err != nil || set.SyntaxErrors != 1 {
		t.Fatalf("%v %d", err, set.SyntaxErrors)
	}
}

// M8 in binding: the same file set, and the JSON syntax is read.
func TestBindingReadsTheModuleFileSetOfTofu(t *testing.T) {
	cats := []IaCCatalogInfo{fxCat("registry.opentofu.org/hashicorp/tls", "4.0.0", "4.2.1"), fxCat("registry.opentofu.org/acme/tls", "1.0.0", "1.5.0")}
	ref := IaCBlockRef{Dir: "mod", Type: "tls_private_key"}
	const decl = "terraform {\n  required_providers {\n    tls = { source = \"acme/tls\" }\n  }\n}\n"
	const use = `resource "tls_private_key" "k" {}`

	root := writeTree(t, map[string]string{"shared/versions.tf": decl, "mod/main.tf": use})
	symlink(t, root, "../shared/versions.tf", "mod/versions.tf")
	b := ResolveIaCBinding(IaCBindingQuery{Repo: os.DirFS(root), Catalogs: cats, Block: ref})
	if b.Source != "registry.terraform.io/acme/tls" || b.Status != BindConstraintMatched || b.Version != "1.5.0" {
		t.Fatalf("a linked versions.tf declares acme/tls: %+v", b)
	}

	for name, files := range map[string]fstest.MapFS{
		"hidden":    {"mod/main.tf": file(use), "mod/.versions.tf": file(decl)},
		"json":      {"mod/main.tf": file(use), "mod/versions.tf.json": file(`{"terraform": {"required_providers": {"tls": {"source": "acme/tls", "version": ">= 1.0"}}}}`)},
		"json list": {"mod/main.tf": file(use), "mod/versions.tf.json": file(`{"terraform": [{"required_providers": [{"tls": {"source": "acme/tls"}}]}]}`)},
	} {
		b := bindBlock(files, cats, ref)
		wantAcme := name != "hidden"
		if got := strings.HasSuffix(b.Source, "/acme/tls"); got != wantAcme || b.Refusal != nil {
			t.Errorf("%s: %+v", name, b)
		}
	}
	// a JSON module call is walked like an HCL one.
	files := fstest.MapFS{
		"root/main.tf.json":    file(`{"module": {"m": {"source": "../mod"}}}`),
		"root/" + LockfileName: lock("registry.terraform.io/hashicorp/tls", "4.2.1"),
		"mod/main.tf":          file(use),
	}
	if b := bindBlock(files, cats, ref); b.Status != BindLocked || b.Version != "4.2.1" || len(b.Roots) != 1 || b.Roots[0].Via != "module call" {
		t.Errorf("%+v", b)
	}
}

// R6 (design section 5 and 9): an input binding needed and did not read refuses the answer. Each of these gave a
// definite `locked` or `constraint-matched` answer with only a note, on the provider the unread file contradicts.
func TestBindingRefusesWhenAnInputWasNotRead(t *testing.T) {
	cats := append([]IaCCatalogInfo{fxCat("registry.opentofu.org/hashicorp/tls", "4.0.0", "4.2.1")}, fxCat("registry.opentofu.org/acme/tls", "1.0.0", "1.5.0"))
	ref := IaCBlockRef{Dir: ".", Type: "tls_private_key"}
	const decl = "terraform {\n  required_providers {\n    tls = { source = \"acme/tls\" }\n  }\n}\n"
	const use = `resource "tls_private_key" "k" {}` + "\n"
	pin := lock("registry.terraform.io/hashicorp/tls", "4.0.0")

	refused := func(t *testing.T, name string, files fstest.MapFS, block IaCBlockRef, want string) {
		t.Helper()
		b := bindBlock(files, cats, block)
		if b.Status != BindUnbound || b.Refusal == nil || b.Refusal.Code != RefuseBindingBound || b.Version != "" || b.Catalog != "" {
			t.Errorf("%s: want a refusal, got %+v", name, b)
			return
		}
		if !strings.Contains(b.Refusal.Reason, want) {
			t.Errorf("%s: reason %q lacks %q", name, b.Refusal.Reason, want)
		}
	}

	// a declaration file over the byte bound.
	big := decl + "# " + strings.Repeat("x", maxBindingFileBytes) + "\n"
	refused(t, "oversize", fstest.MapFS{"main.tf": file(use), "versions.tf": file(big), LockfileName: pin}, ref, "read bound")

	// the 257th file in a directory.
	many := fstest.MapFS{"main.tf": file(use), LockfileName: pin}
	for i := 0; i <= maxBindingTFFiles; i++ {
		many[fmt.Sprintf("n%03d.tf", i)] = file("# empty\n")
	}
	many["zz_versions.tf"] = file(decl)
	refused(t, "257th file", many, ref, "more than")

	// a file past the nesting bound.
	refused(t, "nesting", fstest.MapFS{"main.tf": file(use), "z.tf": file("x = " + strings.Repeat("(", maxBindingFileBytes-64) + "\n"), LockfileName: pin}, ref, "read bound")

	// a lockfile with merge conflict markers holds a pin the parse cannot be trusted to show.
	conflict := "provider \"registry.terraform.io/hashicorp/tls\" {\n  version = \"4.0.0\"\n}\n<<<<<<< HEAD\nprovider \"registry.terraform.io/hashicorp/other\" {\n  version = \"1.0.0\"\n}\n=======\n>>>>>>> branch\n"
	refused(t, "conflict markers", fstest.MapFS{"main.tf": file(use), LockfileName: file(conflict)}, ref, "does not parse")

	// a module call whose source OpenTofu resolves and this reader does not may lead to the block's module.
	for name, call := range map[string]string{
		"variable":      `module "m" { source = var.src }`,
		"interpolation": `module "m" { source = "./${var.dir}" }`,
	} {
		refused(t, name, fstest.MapFS{
			"root/main.tf":         file(call),
			"root/" + LockfileName: pin,
			"mod/main.tf":          file(use),
		}, IaCBlockRef{Dir: "mod", Type: "tls_private_key"}, "root")
	}
	refused(t, "json source", fstest.MapFS{
		"root/main.tf.json":    file(`{"module": {"m": {"source": "${var.src}"}}}`),
		"root/" + LockfileName: pin,
		"mod/main.tf":          file(use),
	}, IaCBlockRef{Dir: "mod", Type: "tls_private_key"}, "root")

	// a JSON file that is not JSON, and a link that points nowhere.
	refused(t, "bad json", fstest.MapFS{"main.tf": file(use), "v.tf.json": file("{"), LockfileName: pin}, ref, "not valid JSON")

	// control: the same repositories without the fault answer.
	ok := bindBlock(fstest.MapFS{"main.tf": file(use), "versions.tf": file(decl), LockfileName: lock("registry.terraform.io/acme/tls", "1.0.0")}, cats, ref)
	if ok.Status != BindLocked || ok.Version != "1.0.0" || ok.Refusal != nil {
		t.Errorf("control: %+v", ok)
	}
	// a literal source that is not a local path is not a fault, and neither is a directory that does not exist.
	ok = bindBlock(fstest.MapFS{
		"root/main.tf":         file("module \"r\" { source = \"hashicorp/consul/aws\" }\nmodule \"g\" { source = \"./gone\" }\nmodule \"m\" { source = \"../mod\" }\n"),
		"root/" + LockfileName: pin,
		"mod/main.tf":          file(use),
	}, cats, IaCBlockRef{Dir: "mod", Type: "tls_private_key"})
	if ok.Status != BindLocked || ok.Refusal != nil {
		t.Errorf("literal remote source: %+v", ok)
	}
}
