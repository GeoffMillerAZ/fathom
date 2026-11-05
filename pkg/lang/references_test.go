package lang_test

import (
	"context"
	"os"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// F5: a Reference names its target as the dialect writes it (To), joins it to
// a symbol of the same file when exactly one matches (ToKey), carries the edge
// the dialect declared and the line it is written on.

func refTo(t *testing.T, res lang.Result, to, edge string) lang.Reference {
	t.Helper()
	for _, r := range res.References {
		if r.To == to && r.Edge == edge {
			return r
		}
	}
	t.Fatalf("no %q reference to %q in %+v", edge, to, res.References)
	return lang.Reference{}
}

func TestReferenceToKeyAndEdgeByDialect(t *testing.T) {
	r := newReader(t)

	// Go: a call names package.Func, which is the callee's Key. Calls carry no line.
	g := extract(t, r, "a/a.go", "package a\n\nfunc F() { G() }\n\nfunc G() {}\n")
	if ref := refTo(t, g, "a.G", "call"); ref.From != "a.F" || ref.ToKey != "a.G" || ref.Line != 0 {
		t.Errorf("go call: %+v", ref)
	}

	// Python: a call names the function as written, and ToKey joins it to the Key.
	py := extract(t, r, "app.py", "def f():\n    g()\n\ndef g():\n    pass\n")
	if ref := refTo(t, py, "g", "call"); ref.From != "app.f" || ref.ToKey != "app.g" {
		t.Errorf("python call: %+v", ref)
	}

	// Terraform: a variable reference is not a call, and has its line.
	tf := extract(t, r, "main.tf", tfSrc)
	if ref := refTo(t, tf, "var.name", "references"); ref.ToKey != "main.var.name" || ref.Line != 2 {
		t.Errorf("terraform reference: %+v", ref)
	}

	// CloudFormation declares one edge per intrinsic, each with its line.
	cfn := extract(t, r, "cfn.yaml", "AWSTemplateFormatVersion: '2010-09-09'\nParameters:\n  P:\n    Type: String\nConditions:\n  C: !Equals [!Ref P, x]\nResources:\n  A:\n    Type: AWS::S3::Bucket\n    Condition: C\n  B:\n    Type: AWS::S3::Bucket\n    DependsOn: A\n    Properties:\n      BucketName: !Ref A\n      Tags: !GetAtt A.Arn\n")
	for edge, want := range map[string]struct {
		to, toKey string
		line      int
	}{
		"condition":  {"C", "cfn.C", 10},
		"depends_on": {"A", "cfn.A", 13},
		"ref":        {"A", "cfn.A", 15},
		"getatt":     {"A", "cfn.A", 16},
	} {
		if ref := refTo(t, cfn, want.to, edge); ref.ToKey != want.toKey || ref.Line != want.line {
			t.Errorf("cloudformation %s: %+v, want ToKey %q line %d", edge, ref, want.toKey, want.line)
		}
	}
	for _, ref := range cfn.References {
		if ref.Edge == "call" {
			t.Errorf("a CloudFormation reference reads as a call: %+v", ref)
		}
	}
}

// A document dialect whose extractor records no relation reads its same-file
// references as "reference", never as calls.
func TestDocumentDialectWithoutADeclaredEdgeReadsAsReference(t *testing.T) {
	src, err := os.ReadFile(dialectsDir + "/caddyfile/testdata/site.caddyfile")
	if err != nil {
		t.Fatal(err)
	}
	res := extract(t, newReader(t), "site.caddyfile", string(src))
	same := 0
	for _, ref := range res.References {
		if ref.Strategy != "" {
			continue // a cross-file reference carries the edge the dialect declared
		}
		same++
		if ref.Edge != "reference" {
			t.Errorf("a same-file Caddyfile reference reads as %q: %+v", ref.Edge, ref)
		}
	}
	if res.Dialect != "caddyfile" || same == 0 {
		t.Fatalf("setup: dialect %q with %d same-file references", res.Dialect, same)
	}
}

func TestPackReferenceKeepsItsDeclaredEdge(t *testing.T) {
	r := newReader(t, lang.WithDialects(sampleFS(t)))
	// widget-spec declares "uses". Battery is no symbol of the file, so ToKey stays empty.
	res := extract(t, r, "ui/clock.widget.json", `{"name":"Clock","kind":"timer","uses":["Battery"]}`)
	if ref := refTo(t, res, "Battery", "uses"); ref.ToKey != "" || ref.From != "clock.widget.Clock" {
		t.Errorf("pack reference: %+v", ref)
	}
}

func TestGoExtractorReferenceGetsAToKey(t *testing.T) {
	r := newReader(t, lang.WithExtractor(rowMatch("row-spec"), &rowExtractor{}))
	res := extract(t, r, "a/b.row", "def Main\ndef Other\nuses Other\nuses Missing\n")
	if ref := refTo(t, res, "Other", "uses"); ref.ToKey != "app.Other" || ref.Line != 3 {
		t.Errorf("uses Other: %+v", ref)
	}
	if ref := refTo(t, res, "Missing", "uses"); ref.ToKey != "" {
		t.Errorf("uses Missing: %+v", ref)
	}
	// Two symbols with one name make the target ambiguous: no ToKey.
	dup := extract(t, r, "a/c.row", "def Main\ndef Other\ndef Other\nuses Other\n")
	if ref := refTo(t, dup, "Other", "uses"); ref.ToKey != "" {
		t.Errorf("an ambiguous target got a ToKey: %+v", ref)
	}
}

// callEngines and callReaders are the readers whose Calls are calls: the
// tree-sitter engines, and the native Go, Rego, templ and notebook readers.
// Every other reader's same-file edge reads "reference" (or the edge its
// dialect declares), so a CMake link or a Markdown link is not a call.
var (
	callEngines = map[string]bool{"tags": true, "tags+doc": true}
	callReaders = map[string]bool{"go": true, "rego": true, "templ": true, "jupyter": true}
)

func TestCallEdgesComeFromCallEngines(t *testing.T) {
	r := newReader(t)
	engine := map[string]string{}
	for _, d := range r.Dialects() {
		engine[d.ID] = d.Engine
	}
	calls := map[string]bool{}
	scan := func(path string, src []byte) {
		got, err := r.Extract(context.Background(), lang.File{Path: path, Src: src})
		if err != nil {
			t.Fatal(err)
		}
		for _, ref := range got.References {
			if ref.Edge != "call" {
				continue
			}
			calls[got.Dialect] = true
			if !callEngines[engine[got.Dialect]] && !callReaders[got.Dialect] {
				t.Errorf("%s (dialect %s, engine %q): reference %+v reads as a call", path, got.Dialect, engine[got.Dialect], ref)
			}
		}
	}
	for _, f := range dialectFixtures(t) {
		scan(f.rel, f.src)
	}
	for _, s := range ownSamples {
		scan(s.path, []byte(s.src))
	}
	scan("a.rego", []byte("package a\n\nf(x) = y if { y := g(x) }\ng(x) = x\n"))
	scan("a.templ", []byte("package a\n\ntempl A() {\n\t@B()\n}\n\ntempl B() {\n\t<p>x</p>\n}\n"))
	for _, want := range []string{"go", "python"} {
		if !calls[want] {
			t.Errorf("%s reads no call edge, so the list above was checked against nothing for it", want)
		}
	}
	// CMake links and the Kubernetes embed wrapper's entities are references.
	cm := extract(t, r, "CMakeLists.txt", "add_library(a a.c)\nadd_executable(e e.c)\ntarget_link_libraries(e PRIVATE a)\n")
	if len(cm.References) == 0 {
		t.Fatalf("setup: cmake reads no link: %+v", cm)
	}
	for _, ref := range cm.References {
		if ref.Edge != "reference" {
			t.Errorf("cmake link reads edge %q, want reference: %+v", ref.Edge, ref)
		}
	}
}
