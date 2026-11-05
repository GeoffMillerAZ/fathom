package iacfacts

import (
	"strings"
	"testing"
	"testing/fstest"
)

// M11: a source with no host resolves to the registry its lock key names, the address OpenTofu reports. The binding
// said registry.terraform.io for an OpenTofu key, which made the host note claim a difference between two OpenTofu
// addresses and made the choice between two installed catalogs favour the wrong one without a word.
func TestBindingImpliedHostTakesTheLockKeysHost(t *testing.T) {
	const tofu, tf = "registry.opentofu.org/hashicorp/tls", "registry.terraform.io/hashicorp/tls"
	both := []IaCCatalogInfo{fxCat(tf, "3.4.0"), fxCat(tofu, "3.4.0")}
	ref := IaCBlockRef{Dir: ".", Type: "tls_private_key"}
	repo := func(key string) fstest.MapFS {
		return fstest.MapFS{
			"main.tf":    file("terraform {\n  required_providers {\n    tls = { source = \"hashicorp/tls\" }\n  }\n}\nresource \"tls_private_key\" \"k\" {}\n"),
			LockfileName: lock(key, "3.4.0"),
		}
	}
	for _, tc := range []struct{ key, want, wantCat string }{{tofu, tofu, both[1].Name}, {tf, tf, both[0].Name}} {
		b := bindBlock(repo(tc.key), both, ref)
		if b.Status != BindLocked || b.Source != tc.want || b.Catalog != tc.wantCat {
			t.Errorf("lock key %s: %+v", tc.key, b)
		}
		if strings.Contains(strings.Join(b.Disclosures, "|"), "the catalog was built from") {
			t.Errorf("lock key %s: the catalog is for the block's own host and needs no host note: %v", tc.key, b.Disclosures)
		}
	}
	// With only the other registry's catalog the difference is real and is disclosed.
	b := bindBlock(repo(tf), []IaCCatalogInfo{both[1]}, ref)
	if b.Status != BindLocked || b.Source != tf || !strings.Contains(strings.Join(b.Disclosures, "|"), "the catalog was built from "+openTofuRegistryHost) {
		t.Errorf("%+v", b)
	}
	// Roots that name different hosts leave the default: no one host is the block's.
	two := fstest.MapFS{
		"mod/main.tf":       repo(tf)["main.tf"],
		"a/main.tf":         file(`module "m" { source = "../mod" }`),
		"a/" + LockfileName: lock(tf, "3.4.0"),
		"b/main.tf":         file(`module "m" { source = "../mod" }`),
		"b/" + LockfileName: lock(tofu, "3.4.0"),
	}
	b = bindBlock(two, both, IaCBlockRef{Dir: "mod", Type: "tls_private_key"})
	if b.Status != BindLocked || b.Source != tf {
		t.Errorf("%+v", b)
	}
}
