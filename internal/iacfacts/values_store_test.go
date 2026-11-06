package iacfacts_test

// LL-36: a resource value never reaches the store. ADR-092 decision 7 and docs/design/lang-library.md section 5 say
// the product never opts in to values, and these tests are the instrument:
//
//   - (a) a source scan fails on any use of WithValues, the public door's iac.WithValues or the reader's own
//     iacfacts.WithValues, outside pkg/lang (the door), cmd/fathom-lang (its CLI) and test files, and on any other
//     assignment that turns the reader's value switch on;
//   - (b) a Terraform, CloudFormation and Pulumi fixture that plants a sentinel literal in every place a value can
//     sit is ingested through the product's ingest path into a scratch store, and the sentinel is in no byte the
//     store wrote.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/iacfacts"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// valuesAllowed are the path prefixes that may use WithValues: the public door and the command that prints it.
var valuesAllowed = []string{"pkg/lang/", "cmd/fathom-lang/"}

// valuesUses returns every use of WithValues, and every assignment to the reader's value switch, in the Go sources
// in files (repository-relative path to source) that lies outside valuesAllowed. A test file is never passed. The
// definition itself is not a use.
func valuesUses(t *testing.T, files map[string][]byte) []string {
	t.Helper()
	var out []string
	fset := token.NewFileSet()
	for rel, src := range files {
		f, err := parser.ParseFile(fset, rel, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		allowed := false
		for _, p := range valuesAllowed {
			allowed = allowed || strings.HasPrefix(rel, p)
		}
		inDefinition := func(stack []ast.Node) bool {
			for _, n := range stack {
				if fd, ok := n.(*ast.FuncDecl); ok && fd.Name.Name == "WithValues" && fd.Recv == nil {
					return true
				}
			}
			return false
		}
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			switch x := n.(type) {
			case *ast.Ident:
				// A bare WithValues is a use inside the reader's own package, or a dot import.
				if x.Name == "WithValues" && !allowed {
					if fd, ok := stack[len(stack)-2].(*ast.FuncDecl); ok && fd.Name == x {
						break // the declaration's name
					}
					if _, isSel := stack[len(stack)-2].(*ast.SelectorExpr); isSel {
						break // counted once, at the selector
					}
					out = append(out, fset.Position(x.Pos()).String()+": WithValues")
				}
			case *ast.SelectorExpr:
				if x.Sel.Name == "WithValues" && !allowed {
					out = append(out, fset.Position(x.Pos()).String()+": "+exprName(x.X)+".WithValues")
				}
			case *ast.AssignStmt:
				// The reader's switch is config.values: it is set by WithValues alone.
				for _, l := range x.Lhs {
					if sel, ok := l.(*ast.SelectorExpr); ok && sel.Sel.Name == "values" && !inDefinition(stack) {
						out = append(out, fset.Position(x.Pos()).String()+": assigns a .values field outside WithValues")
					}
				}
			case *ast.KeyValueExpr:
				if k, ok := x.Key.(*ast.Ident); ok && k.Name == "values" && !allowed && strings.HasPrefix(rel, "internal/iacfacts/") && !inDefinition(stack) {
					out = append(out, fset.Position(x.Pos()).String()+": sets values in a literal outside WithValues")
				}
			}
			return true
		})
	}
	sort.Strings(out)
	return out
}

func exprName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return "?"
}

// skipDirs are directories the scan does not enter: build leftovers and testdata. The go command also ignores a
// directory whose name starts with "." or "_", so the scan does.
var skipDirs = map[string]bool{"workspace": true, "ref": true, "tmp": true, "testdata": true, "node_modules": true}

func repoGoSources(t *testing.T) map[string][]byte {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); p != root && (skipDirs[name] || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 500 || files["internal/iacfacts/resource.go"] == nil || files["pkg/lang/iac/read.go"] == nil {
		t.Fatalf("scanned %d source files; the walk did not reach the repository", len(files))
	}
	return files
}

// (a) The product never opts in to values.
func TestResourceValuesNeverReachTheStore(t *testing.T) {
	t.Run("no product call of WithValues", func(t *testing.T) {
		files := repoGoSources(t)
		// The scan can see the door's own calls, or it sees nothing and proves nothing.
		if files["pkg/lang/iac/read.go"] == nil || !strings.Contains(string(files["pkg/lang/iac/read.go"]), "iacfacts.WithValues()") {
			t.Fatal("the door no longer calls iacfacts.WithValues; update this scan")
		}
		if got := valuesUses(t, map[string][]byte{"pkg/lang/iac/read.go": files["pkg/lang/iac/read.go"]}); len(got) != 0 {
			t.Errorf("the door's own call is allowed, got %v", got)
		}
		for _, p := range valuesUses(t, files) {
			t.Errorf("a use of WithValues outside pkg/lang, cmd/fathom-lang and tests: %s", p)
		}
	})

	t.Run("the scan fails on a call outside the door", func(t *testing.T) {
		for name, c := range map[string]struct{ path, src string }{
			"reader option":           {"internal/fusion/x.go", "package fusion\nimport \"github.com/geoffmilleraz/fathom/internal/iacfacts\"\nvar _ = iacfacts.WithValues()\n"},
			"door option":             {"internal/mcp/x.go", "package mcp\nimport \"github.com/geoffmilleraz/fathom/pkg/lang/iac\"\nfunc f() { _ = iac.WithValues() }\n"},
			"function value":          {"cmd/fathom/x.go", "package main\nimport \"x/iac\"\nvar opt = iac.WithValues\n"},
			"inside reader":           {"internal/iacfacts/x.go", "package iacfacts\nfunc f() { _ = newConfig([]Option{WithValues()}) }\n"},
			"switch set":              {"internal/iacfacts/x.go", "package iacfacts\nfunc f() { var c config; c.values = true }\n"},
			"literal":                 {"internal/iacfacts/x.go", "package iacfacts\nvar c = config{values: true}\n"},
			"pkg/lang sibling prefix": {"pkg/langx/x.go", "package langx\nimport \"x/iac\"\nvar _ = iac.WithValues()\n"},
		} {
			if got := valuesUses(t, map[string][]byte{c.path: []byte(c.src)}); len(got) == 0 {
				t.Errorf("%s was not caught", name)
			}
		}
		for name, c := range map[string]struct{ path, src string }{
			"the door":       {"pkg/lang/iac/x.go", "package iac\nvar _ = iacfacts.WithValues()\n"},
			"the CLI":        {"cmd/fathom-lang/x.go", "package main\nvar _ = iac.WithValues()\n"},
			"the definition": {"internal/iacfacts/resource.go", "package iacfacts\nfunc WithValues() Option { return func(c *config) { c.values = true } }\n"},
		} {
			if got := valuesUses(t, map[string][]byte{c.path: []byte(c.src)}); len(got) != 0 {
				t.Errorf("%s is allowed, got %v", name, got)
			}
		}
	})

	// (b) A sentinel planted where a value can sit is in no byte the store wrote.
	t.Run("a planted sentinel is in no store row", func(t *testing.T) {
		sentinels := []string{
			"SENTINEL-TF-ATTR-7f3a", "SENTINEL-TF-DEFAULT-7f3a", "SENTINEL-TF-SECRET-7f3a", "SENTINEL-TF-LOCAL-7f3a", "SENTINEL-TF-NESTED-7f3a",
			"SENTINEL-CFN-PROP-7f3a", "SENTINEL-CFN-PARAM-7f3a", "SENTINEL-CFN-NOECHO-7f3a",
			"SENTINEL-PULUMI-PROP-7f3a", "SENTINEL-PULUMI-SECRET-7f3a",
		}
		repo := map[string]string{
			"infra/main.tf": `
variable "region" {
  default = "SENTINEL-TF-DEFAULT-7f3a"
}
variable "db_password" {
  default   = "SENTINEL-TF-SECRET-7f3a"
  sensitive = true
}
locals {
  label = "SENTINEL-TF-LOCAL-7f3a"
}
resource "aws_db_instance" "planted" {
  identifier = "SENTINEL-TF-ATTR-7f3a"
  region     = var.region
  password   = var.db_password
  note       = local.label
  tags       = { Owner = "SENTINEL-TF-NESTED-7f3a" }
  settings {
    key = "SENTINEL-TF-NESTED-7f3a"
  }
}
`,
			"stack/cfn.yaml": `AWSTemplateFormatVersion: "2010-09-09"
Parameters:
  Env:
    Type: String
    Default: SENTINEL-CFN-PARAM-7f3a
  DbPassword:
    Type: String
    NoEcho: true
    Default: SENTINEL-CFN-NOECHO-7f3a
Resources:
  PlantedBucket:
    Type: AWS::S3::Bucket
    Properties:
      BucketName: SENTINEL-CFN-PROP-7f3a
      Tag: !Ref Env
      Password: !Ref DbPassword
`,
			"app/Pulumi.yaml": `name: planted
runtime: yaml
resources:
  planted-logs:
    type: aws:s3/bucket:Bucket
    properties:
      bucket: SENTINEL-PULUMI-PROP-7f3a
      token:
        fn::secret: SENTINEL-PULUMI-SECRET-7f3a
`,
		}
		src := t.TempDir()
		for rel, body := range repo {
			p := filepath.Join(src, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		// The fixture really plants values: the reader, asked for them, returns every non-secret sentinel, and it
		// withholds the ones its source marks secret. Without this the empty store would prove nothing.
		read := func(rel string, f func(string, []byte, ...iacfacts.Option) (*iacfacts.ResourceSet, error)) string {
			set, err := f(rel, []byte(repo[rel]), iacfacts.WithValues())
			if err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
			var b strings.Builder
			for _, r := range set.Resources {
				for _, a := range r.Attrs {
					if s, ok := a.Value.(string); ok {
						b.WriteString(s + "\n")
					}
				}
			}
			return b.String()
		}
		tfVals := read("infra/main.tf", iacfacts.ReadTerraformFile)
		for _, want := range []string{"SENTINEL-TF-ATTR-7f3a", "SENTINEL-TF-NESTED-7f3a"} {
			if !strings.Contains(tfVals, want) {
				t.Errorf("the Terraform fixture does not plant %s as a value the reader returns:\n%s", want, tfVals)
			}
		}
		if v := read("stack/cfn.yaml", iacfacts.ReadCloudFormationFile); !strings.Contains(v, "SENTINEL-CFN-PROP-7f3a") {
			t.Errorf("the CloudFormation fixture plants no readable value:\n%s", v)
		}
		if v := read("app/Pulumi.yaml", iacfacts.ReadPulumiYAMLFile); !strings.Contains(v, "SENTINEL-PULUMI-PROP-7f3a") {
			t.Errorf("the Pulumi fixture plants no readable value:\n%s", v)
		}

		// Ingest through the product's path into a scratch store on disk.
		dbDir := t.TempDir()
		dbPath := filepath.Join(dbDir, "scratch.db")
		st, err := store.NewSQLiteStore(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
		if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "planted", Path: src}); err != nil {
			p.Close()
			st.Close()
			t.Fatal(err)
		}
		p.Close()
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}

		// Every byte the store wrote: the database and any journal beside it.
		var stored []byte
		entries, err := os.ReadDir(dbDir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			b, err := os.ReadFile(filepath.Join(dbDir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			stored = append(stored, b...)
		}
		// The ingest stored what it should (names, never values), or an empty store proves nothing.
		for _, name := range []string{"aws_db_instance", "PlantedBucket", "planted-logs"} {
			if !strings.Contains(string(stored), name) {
				t.Errorf("the store holds no %q, so the ingest did not read the fixture", name)
			}
		}
		for _, s := range sentinels {
			if strings.Contains(string(stored), s) {
				t.Errorf("the sentinel %s is in the store: a resource value reached a row", s)
			}
		}
	})
}
