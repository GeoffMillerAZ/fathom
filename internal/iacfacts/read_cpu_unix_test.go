//go:build unix

package iacfacts

import (
	"fmt"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"
	"time"
)

// processCPU is this process's user and system CPU time so far. These bounds measure CPU and not the wall clock,
// which parallel test runs and gate load stretch (the 5 s wall-clock bound that was here flipped under load).
func processCPU() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

// cpuWithin fails the test when fn spends more than d of CPU, d scaled by raceSlowdown. The quadratic shapes below
// spend minutes.
func cpuWithin(t *testing.T, d time.Duration, name string, fn func()) {
	t.Helper()
	d *= raceSlowdown
	if used := cpuOf(fn); used > d {
		t.Errorf("%s: spent %v of CPU, over %v", name, used, d)
	}
}

// 40,000 resources and 10,000 references took 1.5 s with a linear name lookup. The size is bounded by the file cap.
func TestReadPulumiNameLookupIsConstantTime(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("resources:\n")
	const n = 30000
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "  r%d: {type: a:b:C, properties: {p: \"${r%d.id}\"}}\n", i, n-1-i)
	}
	cpuWithin(t, 5*time.Second, "pulumi lookup", func() {
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(sb.String()))
		if err != nil || len(set.Resources) != n {
			t.Errorf("%v", err)
		}
	})
}

// cpuOf is the CPU time fn spends.
func cpuOf(fn func()) time.Duration {
	before := processCPU()
	fn()
	return processCPU() - before
}

// An override file is matched to its base by index. 40,000 base and 40,000 override attributes took 34 s with a
// scan per override attribute, and as many override resources over as many base resources took 2.9 s. The bound is
// relative: the same two files read as plain files (so nothing is overridden) set the cost of parsing them, and
// matching the override must add little to it, whatever the load or the race detector does to both.
func TestReadTerraformOverridesAreNotQuadratic(t *testing.T) {
	const n = 20_000
	check := func(t *testing.T, base, over string, verify func(*ResourceSet)) {
		t.Helper()
		plain := fstest.MapFS{"main.tf": file(base), "main2.tf": file(over)}
		overridden := fstest.MapFS{"main.tf": file(base), "main_override.tf": file(over)}
		var set *ResourceSet
		parse := cpuOf(func() { _, _ = ReadTerraformModule(plain, ".") })
		got := cpuOf(func() {
			var err error
			if set, err = ReadTerraformModule(overridden, "."); err != nil {
				t.Fatal(err)
			}
		})
		if limit := 2*parse + time.Second/2; got > limit {
			t.Errorf("reading with overrides spent %v of CPU, plain files %v, limit %v", got, parse, limit)
		}
		verify(set)
	}
	t.Run("attributes of one resource", func(t *testing.T) {
		var base, over strings.Builder
		base.WriteString("resource \"t\" \"n\" {\n")
		over.WriteString("resource \"t\" \"n\" {\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&base, "  a%d = %d\n", i, i)
			fmt.Fprintf(&over, "  a%d = %d\n", i, i+1)
		}
		base.WriteString("}\n")
		over.WriteString("  extra = 1\n}\n")
		check(t, base.String(), over.String(), func(set *ResourceSet) {
			if len(set.Resources) != 1 || len(set.Resources[0].Attrs) != n+1 {
				t.Fatalf("%d resources", len(set.Resources))
			}
			for _, a := range set.Resources[0].Attrs {
				if a.Class != ClassUnknown {
					t.Fatalf("%s is %s, want unknown", a.Path, a.Class)
				}
			}
		})
	})
	t.Run("resources", func(t *testing.T) {
		// Empty blocks keep the parse cheap, so the matching is what the bound sees.
		const nres = 35_000
		var base, over strings.Builder
		for i := 0; i < nres; i++ {
			fmt.Fprintf(&base, "resource \"t\" \"r%d\" {}\n", i)
			fmt.Fprintf(&over, "resource \"t\" \"r%d\" {}\n", i)
		}
		check(t, base.String(), over.String(), func(set *ResourceSet) {
			if len(set.Resources) != nres {
				t.Fatalf("%d resources", len(set.Resources))
			}
		})
	})
}
