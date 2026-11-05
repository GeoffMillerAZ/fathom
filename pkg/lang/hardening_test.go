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

// F9: nesting inside the size limit must be refused, not recursed into. Go
// reports a stack overflow as a fatal error that recover cannot catch, so a
// failure here ends the test binary. Each reader below died on the input
// shown, measured at 150 KB to 1.8 MB.
func TestDeepNestingIsRefusedNotFatal(t *testing.T) {
	r := newReader(t)
	rep := func(s string, n int) string { return strings.Repeat(s, n) }
	for _, tc := range []struct{ name, path, src, ctl string }{
		{"terraform expression", "infra/main.tf", "a = [\n" + rep("[\n", 400_000), "a = [\n[\n]\n]\n"},
		{"terraform blocks", "infra/main.tf", rep("b {\n", 400_000), "b {\n b {\n }\n}\n"},
		{"tfvars", "infra/prod.tfvars", "a = [\n" + rep("[\n", 900_000), "a = [1]\n"},
		{"terragrunt", "live/terragrunt.hcl", "a = [\n" + rep("[\n", 900_000), "inputs = {\n a = [1]\n}\n"},
		{"packer", "img/web.pkr.hcl", "a = [\n" + rep("[\n", 900_000), "source \"a\" \"b\" {\n x = [1]\n}\n"},
		{"nomad", "jobs/web.nomad.hcl", "job \"j\" {\n" + rep("b {\n", 400_000), "job \"j\" {\n}\n"},
		{"cargo", "crate/Cargo.toml", "a = [\n" + rep("[\n", 1_000_000), "[package]\nname = \"x\"\nversion = \"1.0.0\"\n"},
		{"nginx", "etc/nginx.conf", rep("a {\n", 450_000), "events {}\nhttp {\n  server {\n    listen 80;\n  }\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.src) > 2<<20 {
				t.Fatalf("the input is %d bytes, over the 2 MiB limit", len(tc.src))
			}
			ok := extract(t, r, tc.path, tc.ctl)
			if ok.Skipped != "" || ok.Incomplete || len(ok.Notes) != 0 {
				t.Fatalf("control: skipped %q incomplete=%v notes=%v", ok.Skipped, ok.Incomplete, ok.Notes)
			}
			got := extract(t, r, tc.path, tc.src)
			if got.Skipped != "" {
				t.Fatalf("skipped (%s), so no reader was reached", got.Skipped)
			}
			if !got.Incomplete || !hasReason(got.Notes) || !strings.Contains(strings.Join(got.Notes, " "), "nesting") {
				t.Fatalf("incomplete=%v notes=%v", got.Incomplete, got.Notes)
			}
		})
	}
}

// Brackets in a comment, a string or the middle of an nginx word are not
// nesting, so a file that is long but flat reads whole.
func TestFlatFilesWithBracketsAreNotRefused(t *testing.T) {
	r := newReader(t)
	flat := strings.Repeat("# [[[[[[[[[[\n", 12_000)
	for _, tc := range []struct{ path, src string }{
		{"infra/main.tf", flat + strings.Repeat("// {{{{{\n", 12_000) + "variable \"v\" {\n  default = \"[[[[[\"\n}\n"},
		{"crate/Cargo.toml", flat + "[package]\nname = \"x[[[[\"\nversion = '1'\n" + "tool = '[[[[[[[[[['\n"},
		{"etc/nginx.conf", strings.Repeat("# {{{{{{{{\n", 12_000) + "http {\n  server { return 200 \"{{{{{\"; }\n}\n"},
	} {
		got := extract(t, r, tc.path, tc.src)
		if got.Skipped != "" || got.Incomplete || len(got.Symbols) == 0 {
			t.Errorf("%s: skipped %q incomplete=%v symbols=%d notes=%v", tc.path, got.Skipped, got.Incomplete, len(got.Symbols), got.Notes)
		}
	}
}

// F10: a file the Reader read has a Dialect (types.go: Dialect is set unless
// Skipped), also when its extension is shared by several dialects and it holds
// nothing, or only junk, to emit.
func TestReadFileAlwaysHasADialect(t *testing.T) {
	r := newReader(t)
	for _, path := range []string{"img/web.pkr.hcl", "img/prod.pkrvars.hcl", "jobs/web.nomad.hcl", "build/docker-bake.hcl", "live/terragrunt.hcl"} {
		for name, src := range map[string]string{"empty": "", "comment": "# nothing here\n", "junk": "}}}}}"} {
			t.Run(path+"/"+name, func(t *testing.T) {
				got := extract(t, r, path, src)
				if got.Skipped != "" {
					return // not read, so the contract says nothing
				}
				if got.Dialect == "" || got.Level == "" {
					t.Fatalf("read with dialect %q level %q: %+v", got.Dialect, got.Level, got)
				}
			})
		}
	}
}

// F11: a Go named type that is not a struct is not canonically a struct.
func TestGoTypeCanonicalFollowsTheDeclaration(t *testing.T) {
	r := newReader(t)
	got := extract(t, r, "a/a.go", "package a\n\ntype S struct{ F int }\ntype N int\ntype A = string\ntype Fn func()\ntype I interface{ M() }\ntype G = struct{}\n")
	want := map[string]string{"S": "struct", "N": "type_alias", "A": "type_alias", "Fn": "type_alias", "I": "interface", "G": "type_alias"}
	seen := 0
	for _, s := range got.Symbols {
		if w, ok := want[s.Name]; ok {
			seen++
			if s.Canonical != w {
				t.Errorf("type %s reads canonical %q, want %q", s.Name, s.Canonical, w)
			}
		}
	}
	if seen != len(want) {
		t.Fatalf("read %d of %d types: %+v", seen, len(want), got.Symbols)
	}
}

// F12: Symbol.Package is the extractor's scope, and for Go a method's and a
// field's is its owner type. Reference.From joins on Key, so the shape is
// pinned: a change is a Breaking change.
func TestGoSymbolScopeIsPinned(t *testing.T) {
	r := newReader(t)
	got := extract(t, r, "a/a.go", "package a\n\ntype T struct{ F int }\n\nfunc (t T) M() {}\n\nfunc Fn() { T{}.M() }\n")
	keys := map[string]string{} // name -> key|package
	for _, s := range got.Symbols {
		keys[s.Name] = s.Key + "|" + s.Package
	}
	for name, want := range map[string]string{"Fn": "a.Fn|a", "T": "a.T|a", "T.M": "T.T.M|T", "T.F": "T.T.F|T"} {
		if keys[name] != want {
			t.Errorf("%s reads key|package %q, want %q (all: %v)", name, keys[name], want, keys)
		}
	}
	from := ""
	for _, ref := range got.References {
		if strings.HasSuffix(ref.To, "M") {
			from = ref.From
		}
	}
	if from != "a.Fn" {
		t.Errorf("the call's From is %q, want a.Fn", from)
	}
}
