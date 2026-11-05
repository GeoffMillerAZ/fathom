package lang_test

import (
	"fmt"
	"github.com/geoffmilleraz/fathom/pkg/lang"
	"runtime"
	"strings"
	"testing"
	"time"
)

// yaml.v3 builds an error string for each pair of equal keys in a mapping it
// decodes into a struct, so 4,000 repeats of one key held 2.2 GB inside 20 KB,
// and it compares every pair of keys, so 150,000 distinct keys took 80 s. The
// extractors that decode into a struct read a mapping that repeats a key, or is
// wide, from the nodes instead.
func TestRepeatedAndWideYAMLMappingsAreLinear(t *testing.T) {
	r := newReader(t)
	const k8sHead = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"
	for _, tc := range []struct {
		name, path, head string
		siblings         []string
	}{
		{"chart", "chart/Chart.yaml", "apiVersion: v2\nname: x\nversion: 1.0.0\n", nil},
		{"kubernetes", "k/d.yaml", k8sHead, nil},
	} {
		t.Run(tc.name+" repeated key", func(t *testing.T) {
			src := tc.head + strings.Repeat("z: 1\n", 4_000)
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			var got lang.Result
			within(t, 60*time.Second, tc.path, func() { got = extract(t, r, tc.path, src, tc.siblings...) })
			runtime.ReadMemStats(&after)
			if grew := (after.TotalAlloc - before.TotalAlloc) >> 20; grew > 256 {
				t.Errorf("4,000 repeats of a key allocated %d MiB", grew)
			}
			if got.Skipped != "" {
				t.Errorf("skipped %q", got.Skipped)
			}
		})
		t.Run(tc.name+" distinct keys", func(t *testing.T) {
			var b strings.Builder
			b.WriteString(tc.head)
			for i := 0; i < 100_000; i++ {
				fmt.Fprintf(&b, "k%d: 1\n", i)
			}
			within(t, 20*time.Second, tc.path+" with 100,000 keys", func() { extract(t, r, tc.path, b.String(), tc.siblings...) })
		})
	}
	// The doc engine builds an ordered map one key at a time.
	for _, tc := range []struct{ path, head string }{
		{"stack.yaml", "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  A:\n    Type: AWS::S3::Bucket\n"},
		{"docker-compose.yml", "services:\n  a:\n    image: x\n"},
		{"data.json", ""},
	} {
		t.Run("doc engine "+tc.path, func(t *testing.T) {
			var b strings.Builder
			if tc.head == "" {
				b.WriteString("{")
				for i := 0; i < 150_000; i++ {
					fmt.Fprintf(&b, "\"k%d\": 1,\n", i)
				}
				b.WriteString("\"z\": 1}\n")
			} else {
				b.WriteString(tc.head)
				for i := 0; i < 150_000; i++ {
					fmt.Fprintf(&b, "x%d: 1\n", i)
				}
			}
			within(t, 10*time.Second, tc.path+" with 150,000 keys", func() { extract(t, r, tc.path, b.String()) })
		})
	}
}
