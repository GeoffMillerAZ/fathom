//go:build unix

package iacfacts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"
	"time"
)

// N1: suspect compared every attribute with every reported error: 80,000 attributes and 400,000 errors took 53 s.
func TestSuspectDoesNotScanEveryErrorPerAttribute(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("resource \"t\" \"n\" {\n")
	const attrs = 80_000
	for i := 0; i < attrs; i++ {
		fmt.Fprintf(&sb, "  a%d = 1\n", i)
	}
	sb.WriteString("}\n" + strings.Repeat("\x01", 400_000))
	cpuWithin(t, 10*time.Second, "attributes x errors", func() {
		set, err := ReadTerraformFile("a.tf", []byte(sb.String()))
		if err != nil || set.SyntaxErrors == 0 || len(set.Resources) != 1 || len(set.Resources[0].Attrs) != attrs {
			t.Errorf("%v", err)
		}
	})
}

// N2: an override that names many attributes under one top segment reset the whole segment for each of them.
func TestReadTerraformOverridesUnderOneSegmentAreNotQuadratic(t *testing.T) {
	const n = 40_000
	var base, over strings.Builder
	base.WriteString("resource \"t\" \"n\" {\n  blk {\n")
	over.WriteString("resource \"t\" \"n\" {\n  blk {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&base, "    a%d = %d\n", i, i)
		fmt.Fprintf(&over, "    a%d = %d\n", i, i+1)
	}
	base.WriteString("  }\n}\n")
	over.WriteString("  }\n}\n")
	plain := fstest.MapFS{"main.tf": file(base.String()), "main2.tf": file(over.String())}
	overridden := fstest.MapFS{"main.tf": file(base.String()), "main_override.tf": file(over.String())}
	parse := cpuOf(func() { _, _ = ReadTerraformModule(plain, ".") })
	var set *ResourceSet
	got := cpuOf(func() {
		var err error
		if set, err = ReadTerraformModule(overridden, "."); err != nil {
			t.Fatal(err)
		}
	})
	if limit := 2*parse + time.Second/2; got > limit {
		t.Errorf("reading with overrides spent %v of CPU, plain files %v, limit %v", got, parse, limit)
	}
	for _, a := range set.Resources[0].Attrs {
		if a.Class != ClassUnknown {
			t.Fatalf("%s is %s, want unknown", a.Path, a.Class)
		}
	}
}

// N3: a link in the module directory to a FIFO blocked the read in Open until a writer came.
func TestReadTerraformModuleDoesNotOpenALinkToAFIFO(t *testing.T) {
	root := writeTree(t, map[string]string{"mod/main.tf": "resource \"t\" \"n\" {}\n"})
	fifo := filepath.Join(root, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("no FIFOs here: %v", err)
	}
	symlink(t, root, "../pipe", "mod/link.tf")
	done := make(chan *ResourceSet, 1)
	go func() {
		set, _ := ReadTerraformModule(os.DirFS(root), "mod")
		done <- set
	}()
	select {
	case set := <-done:
		if set == nil || set.SyntaxErrors != 1 || resourceNames(set) != "n" {
			t.Errorf("%+v", set)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the read is blocked opening a FIFO")
	}
}
