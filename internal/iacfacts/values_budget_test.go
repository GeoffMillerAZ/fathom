package iacfacts

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

// IAC3: MaxTotalValueBytes was charged only when a value was emitted, but a memo holds every value it builds. An
// attribute that forces the memo and keeps no value (an expression, a reference beside it) escaped the budget, and
// the heap grew linearly with no refusal: 346 MB from a 918 KB file at n=400, about 25 GB at the file cap.
func memoPulumi(n int, big string) string {
	var sb strings.Builder
	sb.WriteString("variables:\n  v0: " + big + "\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "  v%d: \"y${v%d}\"\n", i, i-1)
	}
	sb.WriteString("resources:\n  z:\n    type: a:b:C\n  r:\n    type: a:b:C\n    properties:\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "      a%d: \"${v%d}${z.id}\"\n", i, i)
	}
	return sb.String()
}

func memoTerraform(n int, big string) string {
	var sb strings.Builder
	sb.WriteString("locals {\n  l0 = \"" + big + "\"\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "  l%d = \"y${local.l%d}\"\n", i, i-1)
	}
	sb.WriteString("}\nresource \"t\" \"n\" {\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "  a%d = [local.l%d, z.y]\n", i, i)
	}
	sb.WriteString("}\n")
	return sb.String()
}

func memoCloudFormation(n int, big string) string {
	var sb strings.Builder
	sb.WriteString("Parameters:\n  P:\n    Type: String\n    Default: " + big + "\nResources:\n  B:\n    Type: T\n    Properties:\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "      a%d: {\"Fn::Base64\": !Sub \"y${P}\"}\n", i)
	}
	return sb.String()
}

func TestReadMemoizedValuesAreChargedWhenBuilt(t *testing.T) {
	big := strings.Repeat("x", 900_000)
	readers := map[string]func(n int) error{
		"pulumi": func(n int) error {
			_, err := ReadPulumiYAMLFile("P.yaml", []byte(memoPulumi(n, big)), WithValues())
			return err
		},
		"terraform": func(n int) error {
			_, err := ReadTerraformModule(fstest.MapFS{"m.tf": file(memoTerraform(n, big))}, ".", WithValues())
			return err
		},
		"cloudformation": func(n int) error {
			_, err := ReadCloudFormationFile("t.yaml", []byte(memoCloudFormation(n, big)), WithValues())
			return err
		},
	}
	for name, read := range readers {
		// A few memoized values are far under the budget and read.
		if err := read(4); err != nil {
			t.Errorf("%s: a small memo must read: %v", name, err)
		}
		var err error
		got := allocated(func() { err = read(400) })
		if !errors.Is(err, ErrLimit) {
			t.Errorf("%s: 400 memoized values of 900 KB must be refused, got %v", name, err)
		}
		if got > 128<<20 {
			t.Errorf("%s: allocated %d MB before refusing", name, got>>20)
		}
	}
}
