package extractor

import (
	"runtime/debug"
	"strings"
	"testing"
)

// hclsyntax parses `1+1+1+...` and `1?1:1?1:...` into a chain one level per
// operator, and evaluating one (Expression.Value) recurses once per level, about
// 1.3 KB of Go stack each. A stack that outgrows its limit is a fatal error that
// no recover catches, and a chain of 400,000 operators is a file of 800 KB. The
// extractors read the value of a few attributes (a module's source, a Packer
// build's name and sources, a Nomad service name and image, a bake target's
// inherits, a group's targets and a target's contexts), and must evaluate
// nothing but a literal. The test lowers the stack limit so a chain far shorter
// than a file's limit overflows it: without the guard the test binary dies.
func TestHCLChainsAreNeverEvaluated(t *testing.T) {
	defer debug.SetMaxStack(debug.SetMaxStack(64 << 20))
	const n = 60_000
	pad := strings.Repeat("#\n", 20_000) // a file of short lines is not minified
	plus := strings.Repeat("1+", n) + "1"
	cond := strings.Repeat("1?1:", n) + "1"
	for _, chain := range []struct{ name, expr string }{{"binary operators", plus}, {"conditionals", cond}} {
		for _, tc := range []struct{ site, path, src string }{
			{"module source", "main.tf", "module \"m\" {\n  source = " + chain.expr + "\n}\n"},
			{"packer build name", "a.pkr.hcl", "build {\n  name = " + chain.expr + "\n}\n"},
			{"packer sources", "a.pkr.hcl", "build {\n  sources = [" + chain.expr + "]\n}\n"},
			{"nomad service name", "a.nomad.hcl", "job \"j\" {\n  group \"g\" {\n    service {\n      name = " + chain.expr + "\n    }\n  }\n}\n"},
			{"nomad image", "a.nomad.hcl", "job \"j\" {\n  group \"g\" {\n    task \"t\" {\n      config {\n        image = " + chain.expr + "\n      }\n    }\n  }\n}\n"},
			{"bake inherits", "docker-bake.hcl", "target \"a\" {\n  inherits = [" + chain.expr + "]\n}\n"},
			{"bake group targets", "docker-bake.hcl", "group \"g\" {\n  targets = [" + chain.expr + "]\n}\n"},
			{"bake contexts", "docker-bake.hcl", "target \"a\" {\n  contexts = {\n    x = " + chain.expr + "\n  }\n}\n"},
		} {
			t.Run(chain.name+"/"+tc.site, func(t *testing.T) {
				src := []byte(pad + tc.src)
				var x Extractor = NewHCLExtractor()
				if !strings.HasSuffix(tc.path, ".tf") {
					x, _ = HCLToolDialectExtractor("hcl-tools")
				}
				if _, err := x.Extract("svc", tc.path, src); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// A literal is still evaluated: a module source in quotes, in parentheses, and a
// heredoc.
func TestHCLLiteralValueIsEvaluated(t *testing.T) {
	for name, src := range map[string]string{
		"quoted":      `module "m" { source = "./a" }`,
		"parentheses": `module "m" { source = (("./a")) }`,
	} {
		nodes, err := NewHCLExtractor().Extract("svc", "main.tf", []byte(src))
		if err != nil || len(nodes) == 0 || !strings.Contains(nodes[0].Signature, `source = "./a"`) {
			t.Errorf("%s: %+v %v", name, nodes, err)
		}
	}
}
