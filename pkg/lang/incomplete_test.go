package lang_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// F1: a file an engine could not fully read sets Result.Incomplete and says
// why in Notes, in one line and with no stack. Each engine that swallowed a
// read failure is held here, with a clean control that must not set the flag.
var incompleteCases = []struct {
	name, engine, path, broken, valid string
}{
	{"hcl partial parse", "hclextract", "infra/bad.tf", "resource \"a\" \"b\" {\n  x = \n}\nvariable \"v\" {}\n", tfSrc},
	{"hcl no parse", "hclextract", "infra/bad2.tf", "}}}}}", tfSrc},
	{"hcl tfvars", "hclextract", "infra/prod.tfvars", "a = \n", "a = 1\n"},
	{"cloudformation yaml", "docengine", "stack.yaml", "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  B:\n    Type: AWS::S3::Bucket\n   bad: [\n",
		"AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  B:\n    Type: AWS::S3::Bucket\n"},
	{"docker compose", "docengine", "docker-compose.yml", "services:\n  a:\n    image: x\n   bad: [\n", "services:\n  a:\n    image: x\n"},
	{"kubernetes", "k8sextract", "k/deploy.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\n bad: [\n",
		"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\n"},
	{"github actions", "docengine", ".github/workflows/ci.yml", "name: ci\non: push\njobs:\n  a:\n    runs-on: x\n   bad: [\n",
		"name: ci\non: push\njobs:\n  a:\n    runs-on: x\n    steps:\n      - run: echo\n"},
	{"helm chart", "helmextract", "chart/Chart.yaml", "name: [\n", "name: web\nversion: 1.0.0\n"},
	{"helm chart field type", "helmextract", "chart/Chart.yaml", "name: [a]\nversion: 1.0.0\n", "name: web\nversion: 1.0.0\n"},
	{"helm values", "helmextract", "chart/values.yaml", "a: [\n", "a: 1\n"},
	{"terragrunt", "terragruntdialect", "live/terragrunt.hcl", "terraform {\n  source = \n}\n", "terraform {\n  source = \"./mod\"\n}\n"},
	{"packer", "hcltoolsdialect", "img/web.pkr.hcl", "source \"a\" \"b\" {\n  x = \n}\n", "source \"a\" \"b\" {\n  x = 1\n}\n"},
	{"json", "jsonextract", "data.json", `{"a": `, `{"a": 1}`},
	{"package.json", "docengine", "package.json", `{"name": "x", `, `{"name": "x", "version": "1.0.0"}`},
	{"toml", "tomlextract", "data.toml", "a = \n[b\n", "a = 1\n"},
	{"go", "goextract", "a/a.go", "package a\nfunc F( {\n", "package a\n\nfunc F() {}\n"},
}

func TestBrokenInputIsIncomplete(t *testing.T) {
	r := newReader(t)
	for _, tc := range incompleteCases {
		t.Run(tc.name, func(t *testing.T) {
			got := extract(t, r, tc.path, tc.broken)
			if got.Skipped != "" {
				t.Fatalf("%s was skipped (%s), so no engine read it: %+v", tc.path, got.Skipped, got)
			}
			if !got.Incomplete {
				t.Fatalf("%s (%s): a broken file reads as complete: dialect %q, %d symbols, notes %v", tc.engine, tc.path, got.Dialect, len(got.Symbols), got.Notes)
			}
			var reason string
			for _, n := range got.Notes {
				if strings.HasPrefix(n, "incomplete: ") || strings.HasPrefix(n, "extractor error: ") {
					reason = n
				}
			}
			if reason == "" {
				t.Fatalf("Incomplete is true and Notes gives no reason: %v", got.Notes)
			}
			if strings.Contains(reason, "\n") || len(reason) > 220 || strings.Contains(reason, "goroutine") || strings.Contains(reason, "wasm") {
				t.Errorf("the reason is not one short line: %q", reason)
			}
			ok := extract(t, r, tc.path, tc.valid)
			if ok.Incomplete || len(ok.Notes) != 0 {
				t.Errorf("a valid %s file reads incomplete: %v", tc.path, ok.Notes)
			}
		})
	}
}

// A valid document that no extractor model reads, and an empty file, are not
// incomplete: the flag is for a file the engine could not read, not one it
// read and found empty.
func TestWellFormedFilesAreComplete(t *testing.T) {
	r := newReader(t)
	for path, src := range map[string]string{
		"list.json":   `[1, 2, 3]`,
		"empty.json":  ``,
		"empty.toml":  ``,
		"blank.yaml":  "\n",
		"scalar.json": `"x"`,
	} {
		if got := extract(t, r, path, src); got.Incomplete {
			t.Errorf("%s reads incomplete: %v", path, got.Notes)
		}
	}
}

func TestGoExtractorErrorIsIncomplete(t *testing.T) {
	r := newReader(t, lang.WithExtractor(rowMatch("row-spec"), failingExtractor{}))
	got := extract(t, r, "a/b.row", "def Main\n")
	if !got.Incomplete || len(got.Notes) == 0 {
		t.Fatalf("an extractor that returned an error reads complete: %+v", got)
	}
	if ok := extract(t, newReader(t, lang.WithExtractor(rowMatch("row-spec"), &rowExtractor{})), "a/b.row", "def Main\n"); ok.Incomplete {
		t.Fatalf("a clean read is incomplete: %v", ok.Notes)
	}
}

type failingExtractor struct{}

func (failingExtractor) Extract(context.Context, lang.File) (lang.Extracted, error) {
	return lang.Extracted{}, errors.New("cannot read")
}
