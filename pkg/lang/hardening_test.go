package lang_test

import (
	"fmt"
	"strings"
	"testing"
)

// F1 (later documents): a YAML file whose second document fails to decode
// keeps the first, and the file must read incomplete. A real typo keeps its
// apiVersion and kind, so the dialect stays the custom resource's and never
// falls back to Kubernetes, whose reader already discloses.
func TestLaterBadDocumentIsIncomplete(t *testing.T) {
	r := newReader(t)
	doc := func(apiVersion, kind, name string, bad bool) string {
		s := fmt.Sprintf("apiVersion: %s\nkind: %s\nmetadata:\n  name: %s\nspec: {}\n", apiVersion, kind, name)
		if bad {
			s += " bad: [\n"
		}
		return s
	}
	for _, tc := range []struct{ dialect, path, apiVersion, kind string }{
		{"kyverno", "policies/p.yaml", "kyverno.io/v1", "ClusterPolicy"},
		{"gatekeeper", "policies/t.yaml", "constraints.gatekeeper.sh/v1", "K8sRequiredLabels"},
		{"argo", "apps/a.yaml", "argoproj.io/v1alpha1", "Application"},
		{"flux", "flux/g.yaml", "source.toolkit.fluxcd.io/v1", "GitRepository"},
		{"crossplane", "xp/p.yaml", "pkg.crossplane.io/v1", "Provider"},
	} {
		t.Run(tc.dialect, func(t *testing.T) {
			good := doc(tc.apiVersion, tc.kind, "a", false) + "---\n" + doc(tc.apiVersion, tc.kind, "b", false) + "---\n" + doc(tc.apiVersion, tc.kind, "c", false)
			bad := doc(tc.apiVersion, tc.kind, "a", false) + "---\n" + doc(tc.apiVersion, tc.kind, "b", true) + "---\n" + doc(tc.apiVersion, tc.kind, "c", false)
			ok := extract(t, r, tc.path, good)
			if ok.Dialect != tc.dialect || ok.Incomplete || len(ok.Notes) != 0 {
				t.Fatalf("control: want %s complete, got dialect %q incomplete=%v notes=%v", tc.dialect, ok.Dialect, ok.Incomplete, ok.Notes)
			}
			got := extract(t, r, tc.path, bad)
			if got.Dialect != tc.dialect {
				t.Fatalf("the typo moved the dialect to %q", got.Dialect)
			}
			if len(got.Symbols) >= len(ok.Symbols) {
				t.Fatalf("test premise: the typo should lose symbols (%d vs %d)", len(got.Symbols), len(ok.Symbols))
			}
			if !got.Incomplete || !hasReason(got.Notes) {
				t.Fatalf("%d of %d symbols read and the file reads complete: incomplete=%v notes=%v", len(got.Symbols), len(ok.Symbols), got.Incomplete, got.Notes)
			}
		})
	}
}

func hasReason(notes []string) bool {
	for _, n := range notes {
		if strings.HasPrefix(n, "incomplete: ") && !strings.Contains(n, "\n") {
			return true
		}
	}
	return false
}

// F1: a document the dialect does not read (a second YAML document in a
// dialect that reads one, a second JSON value) is disclosed, not dropped.
func TestUnreadDocumentsAreIncomplete(t *testing.T) {
	r := newReader(t)
	cfn := "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  A:\n    Type: AWS::S3::Bucket\n"
	cfn2 := "Resources:\n  B:\n    Type: AWS::S3::Bucket\n"
	cfnJSON := `{"AWSTemplateFormatVersion": "2010-09-09", "Resources": {"A": {"Type": "AWS::S3::Bucket"}}}`
	for _, tc := range []struct{ name, path, src, ctl string }{
		{"cfn second yaml document", "stack.yaml", cfn + "---\n" + cfn2, cfn},
		{"cfn second json value", "stack.json", cfnJSON + "\n" + cfnJSON, cfnJSON},
		{"gitlab-ci later document", ".gitlab-ci.yml", "build:\n  script: [a]\n---\ntest:\n  script: [b\n---\ndeploy:\n  script: [c]\n", "build:\n  script: [a]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if ok := extract(t, r, tc.path, tc.ctl); ok.Incomplete || len(ok.Notes) != 0 {
				t.Fatalf("control reads incomplete: %v", ok.Notes)
			}
			got := extract(t, r, tc.path, tc.src)
			if got.Skipped != "" || !got.Incomplete || !hasReason(got.Notes) {
				t.Fatalf("dialect %q skipped %q incomplete=%v notes=%v", got.Dialect, got.Skipped, got.Incomplete, got.Notes)
			}
		})
	}
}

// F8: an alias chain multiplies by its fan-out per level, so a file under 1 KB
// asks for gigabytes. The file is refused whole, in time, as incomplete, and
// real anchors still read.
func TestYAMLAliasBombIsRefused(t *testing.T) {
	r := newReader(t)
	bomb := func(levels int) string {
		var b strings.Builder
		b.WriteString("l0: &l0 [x, x, x, x, x, x, x, x, x, x]\n")
		for i := 1; i <= levels; i++ {
			fmt.Fprintf(&b, "l%d: &l%d [%s]\n", i, i, strings.TrimSuffix(strings.Repeat(fmt.Sprintf("*l%d, ", i-1), 10), ", "))
		}
		return b.String()
	}
	cfnHead := "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  A:\n    Type: AWS::S3::Bucket\n"
	for _, tc := range []struct{ name, path, head string }{
		{"cloudformation", "stack.yaml", cfnHead},
		{"docker-compose", "docker-compose.yml", "services:\n  a:\n    image: x\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.head + bomb(6)
			if len(src) > 1024 {
				t.Fatalf("the bomb is %d bytes", len(src))
			}
			got := extract(t, r, tc.path, src)
			if !got.Incomplete || !hasReason(got.Notes) || len(got.Symbols) != 0 {
				t.Fatalf("the bomb reads: incomplete=%v symbols=%d notes=%v", got.Incomplete, len(got.Symbols), got.Notes)
			}
			if !strings.Contains(strings.Join(got.Notes, " "), "alias") {
				t.Errorf("the reason does not name the aliases: %v", got.Notes)
			}
		})
	}
	t.Run("real anchors still read", func(t *testing.T) {
		src := "services:\n  a: &base\n    image: x\n    environment: &env {A: '1', B: '2'}\n  b:\n    <<: *base\n    environment: *env\n"
		got := extract(t, r, "docker-compose.yml", src)
		if got.Incomplete || len(got.Symbols) < 2 {
			t.Fatalf("anchors refused: incomplete=%v symbols=%d notes=%v", got.Incomplete, len(got.Symbols), got.Notes)
		}
	})
}
