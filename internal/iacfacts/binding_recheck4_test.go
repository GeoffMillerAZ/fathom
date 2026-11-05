package iacfacts

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// IAC17: binding deduped its notes by a linear scan and kept every one: a module of 1 MiB holding tens of thousands of
// bad required_providers entries took 44 s, and the answer carried a disclosure for each.
func TestBindingCapsAndDedupesItsNotes(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("terraform {\n  required_providers {\n")
	for i := 0; i < 60_000; i++ {
		fmt.Fprintf(&sb, "a%d=1\n", i)
	}
	sb.WriteString("  }\n}\nresource \"aws_vpc\" \"v\" {}\n")
	var b IaCBinding
	within(t, 20*time.Second, "notes", func() {
		b = bindBlock(fstest.MapFS{"main.tf": file(sb.String())}, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_vpc"})
	})
	if len(b.Disclosures) > maxBindingNotes+16 {
		t.Fatalf("%d disclosures, want at most %d plus a few", len(b.Disclosures), maxBindingNotes)
	}
	if !strings.Contains(strings.Join(b.Disclosures, "|"), "more disclosures past the first") {
		t.Errorf("what was dropped must be said: %d disclosures", len(b.Disclosures))
	}
	if b.Refusal == nil && b.Status != BindUnbound {
		t.Errorf("bad entries must still refuse: %+v", b.Status)
	}
}

// IAC28: only `terraform` and `provider` repeats refuse a JSON module (and a repeat inside the three binding reads).
func TestBindingJSONRepeatedVariableAndResourceKeysDoNotRefuse(t *testing.T) {
	js := `{"terraform": {"required_providers": {"tls": {"source": "hashicorp/tls", "version": "4.0.0"}}},
 "variable": {"a": {}}, "variable": {"b": {}}, "resource": {}, "resource": {}}`
	if got := bindTLS("main.tf.json", js); got != "4.0.0" {
		t.Fatalf("got %q, want 4.0.0", got)
	}
}

// IAC17: a shared module of dynamic sources reached by sixteen lockfile roots wrote a note per source per root, and
// deduped them by a scan: 3m22s and 484,402 disclosures at 1 MiB.
func TestBindingCapsTheNotesOfASharedModuleReachedByManyRoots(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(reusableModule)
	for i := 0; i < 8000; i++ {
		fmt.Fprintf(&sb, "module \"m%d\" { source = var.s }\n", i)
	}
	repo := fstest.MapFS{"mod/main.tf": file(sb.String())}
	for i := 0; i < 16; i++ {
		repo[fmt.Sprintf("root%02d/main.tf", i)] = file(`module "m" { source = "../mod" }`)
		repo[fmt.Sprintf("root%02d/%s", i, LockfileName)] = lock("registry.terraform.io/hashicorp/aws", "5.100.0")
	}
	var b IaCBinding
	within(t, 30*time.Second, "shared module", func() {
		b = bindBlock(repo, awsCatalogs, IaCBlockRef{Dir: "mod", Type: "aws_lb_listener"})
	})
	if len(b.Disclosures) > maxBindingNotes+16 {
		t.Fatalf("%d disclosures, want at most %d plus a few", len(b.Disclosures), maxBindingNotes)
	}
}
