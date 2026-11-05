package iacfacts

import (
	"testing"
	"testing/fstest"
)

// MF3: binding answered from the parser's recovery for a module file, and read an entry it could not read as one
// that states nothing.
func TestBindingRefusesWhatItDidNotRead(t *testing.T) {
	ref := IaCBlockRef{Dir: ".", Type: "aws_lb_listener"}
	acme := "terraform {\n  required_providers {\n    aws = { source = \"acme/aws\" }\n  }\n}\nresource \"aws_lb_listener\" \"main\" {}\n"
	bad := map[string]string{
		"error above acme":    ")\n" + acme,
		"unreadable source":   "terraform {\n  required_providers {\n    aws = { source = @ }\n  }\n}\nresource \"aws_lb_listener\" \"main\" {}\n",
		"unterminated string": "terraform {\n  required_providers {\n    aws = { source = \"hashicorp/aws\", version = \"~> 1 }\n  }\n}\nresource \"aws_lb_listener\" \"main\" {}\n",
		"version is a var":    "terraform {\n  required_providers {\n    aws = { source = \"hashicorp/aws\", version = var.v }\n  }\n}\nresource \"aws_lb_listener\" \"main\" {}\n",
		"source is a var":     "terraform {\n  required_providers {\n    aws = { source = var.s }\n  }\n}\nresource \"aws_lb_listener\" \"main\" {}\n",
		"entry is a var":      "terraform {\n  required_providers {\n    aws = var.p\n  }\n}\nresource \"aws_lb_listener\" \"main\" {}\n",
	}
	for name, tf := range bad {
		b := bindBlock(fstest.MapFS{"main.tf": file(tf)}, awsCatalogs, ref)
		if b.Status != BindUnbound {
			t.Errorf("%s: want unbound, got %+v", name, b)
		}
	}
	jsonBad := map[string]string{
		"version is an expression": `{"terraform": {"required_providers": {"aws": {"source": "hashicorp/aws", "version": "${var.v}"}}}}`,
		"entry is a number":        `{"terraform": {"required_providers": {"aws": 5}}}`,
	}
	for name, js := range jsonBad {
		b := bindBlock(fstest.MapFS{"main.tf": file(`resource "aws_lb_listener" "main" {}`), "v.tf.json": file(js)}, awsCatalogs, ref)
		if b.Status != BindUnbound {
			t.Errorf("%s: want unbound, got %+v", name, b)
		}
	}
	// A clean module still binds.
	if b := bindBlock(fstest.MapFS{"main.tf": file(reusableModule)}, awsCatalogs, ref); b.Status != BindConstraintMatched {
		t.Errorf("a clean module must still bind: %+v", b)
	}
}

// MF4: OpenTofu merges a module call in an override file over the base call, so source ./b in an override leads to
// ./b and not ./a. Binding did not read the override.
func TestBindingRefusesAnOverriddenModuleSource(t *testing.T) {
	base := func(over map[string]string) fstest.MapFS {
		m := fstest.MapFS{
			"main.tf":          file(`module "m" { source = "./a" }`),
			"a/main.tf":        file(reusableModule),
			"b/main.tf":        file(reusableModule),
			LockfileName:       lock("registry.terraform.io/hashicorp/aws", "5.100.0"),
			"unrelated/ok.tf":  file(`resource "aws_lb_listener" "x" {}`),
			"unrelated/ok2.tf": file(`# nothing`),
		}
		for k, v := range over {
			m[k] = file(v)
		}
		return m
	}
	cases := map[string]map[string]string{
		"hcl source":      {"main_override.tf": `module "m" { source = "./b" }`},
		"hcl providers":   {"override.tf": `module "m" { providers = { aws = aws.east } }`},
		"json source":     {"main_override.tf.json": `{"module": {"m": {"source": "./b"}}}`},
		"json providers":  {"x_override.tf.json": `{"module": [{"m": [{"providers": {"aws": "aws.east"}}]}]}`},
		"tofu hcl source": {"main_override.tofu": `module "m" { source = "./b" }`},
	}
	for name, over := range cases {
		for _, dir := range []string{"a", "b"} {
			b := bindBlock(base(over), awsCatalogs, IaCBlockRef{Dir: dir, Type: "aws_lb_listener"})
			if b.Status == BindLocked || b.Status == BindConstraintMatched {
				t.Errorf("%s: block in %s must not bind through a module graph it did not read: %+v", name, dir, b)
			}
		}
	}
	// An override that sets only an input does not change the graph.
	b := bindBlock(base(map[string]string{"main_override.tf": `module "m" { x = 1 }`}), awsCatalogs, IaCBlockRef{Dir: "a", Type: "aws_lb_listener"})
	if b.Status != BindLocked {
		t.Errorf("an override of an input leaves the graph alone: %+v", b)
	}
}
