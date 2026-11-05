//go:build unix

package iacfacts

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// IAC36: yaml.v3 nodes keep a repeated key, so one entry holds `secret: false` 70,000 times, and every ${name} of it
// asked isSecretFlag about the whole list: keys times interpolations, 1m52s at 2 MB. The answer is once per entry.
func TestPulumiSecretFlagIsAskedOncePerEntry(t *testing.T) {
	program := func(keys int) []byte {
		var sb strings.Builder
		sb.WriteString("configuration:\n  pw:\n    default: x\n")
		sb.WriteString(strings.Repeat("    secret: false\n", keys))
		sb.WriteString("resources:\n  r:\n    type: a:b:C\n    properties:\n")
		for i := 0; i < 50_000; i++ {
			fmt.Fprintf(&sb, "      p%d: ${pw}\n", i)
		}
		return []byte(sb.String())
	}
	read := func(src []byte) time.Duration {
		before := processCPU()
		if _, err := ReadPulumiYAMLFile("Pulumi.yaml", src, WithValues()); err != nil {
			t.Errorf("%v", err)
		}
		return processCPU() - before
	}
	// The same interpolations over one key are the cost of reading them. 40,000 keys must not multiply it: a scan
	// of the keys for each interpolation is two billion flag checks.
	control := read(program(1))
	if used := read(program(40_000)); used > 4*control+time.Second {
		t.Errorf("40,000 secret keys read by 50,000 interpolations spent %v of CPU, against %v for one key", used, control)
	}
}
