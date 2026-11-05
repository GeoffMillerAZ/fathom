package iacfacts

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func tfWith(expr string) []byte {
	return []byte("resource \"t\" \"n\" {\n  x = " + expr + "\n}\n")
}

// A run of prefix operators makes the HCL parser recurse once per operator, and Go's stack overflow is a fatal
// error no recover can catch. 300 KB of `!` or `-` killed the process before parseHCLBounded.
func TestReadTerraformOperatorRunsAreRefused(t *testing.T) {
	const n = 300_000
	shapes := map[string]string{
		"bang":       strings.Repeat("!", n) + "true",
		"minus":      strings.Repeat("-", n) + "1",
		"mixed":      strings.Repeat("!-", n/2) + "1",
		"parens+ops": strings.Repeat("(", 100) + strings.Repeat("!", 100) + "true" + strings.Repeat(")", 100),
	}
	for name, e := range shapes {
		_, err := ReadTerraformFile("a.tf", tfWith(e))
		if !errors.Is(err, ErrLimit) {
			t.Errorf("%s: want ErrLimit, got %v", name, err)
		}
	}
	// Under the bound the run still reads.
	set, err := ReadTerraformFile("a.tf", tfWith(strings.Repeat("!", MaxDepth-1)+"true"))
	if err != nil || len(set.Resources) != 1 {
		t.Fatalf("a run under the bound must read: %v", err)
	}
	// Cap + 1.
	if _, err := ReadTerraformFile("a.tf", tfWith(strings.Repeat("!", MaxDepth+1)+"true")); !errors.Is(err, ErrLimit) {
		t.Fatalf("cap+1: %v", err)
	}
}

// The binding reader parses repository files too, up to 1 MiB each, and had no guard at all.
func TestBindingOperatorRunsAreSkippedAndDisclosed(t *testing.T) {
	for name, run := range map[string]string{"bang": "!", "minus": "-", "paren": "("} {
		pad := strings.Repeat(run, maxBindingFileBytes-64)
		fsys := fstest.MapFS{"main.tf": file(reusableModule), "z.tf": file("x = " + pad + "\n")}
		b := bindBlock(fsys, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"})
		if !strings.Contains(strings.Join(b.Disclosures, "|"), "past the read bound") {
			t.Errorf("%s: %v", name, b.Disclosures)
		}
	}
}
