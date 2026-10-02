package fusion

// terragrunt_dialect_test.go — LB-F141, LB-F142: the native Terragrunt
// dialect is reached by ingest (a manifest basename, or a .hcl its content
// sniff claims), ReingestFile decides as ingest does, an .hcl no sniff claims
// is never extracted (counted ambiguous), and an ingest stores the dialect's
// cross-file references as doc-xref edges: include to the included file,
// dependency to the dependency's terragrunt.hcl, a local terraform.source to
// every definition of the module directory, an inputs key to the module's
// variable.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// terragruntCorpus is a small live repository: a root.hcl and a shared
// _common/app.hcl (a .hcl the sniff claims), a vpc unit and an app unit that
// includes both, depends on the vpc and calls a local module, the two
// modules, and the .hcl files no sniff claims (a Vault configuration and a
// provider lock file).
func terragruntCorpus() map[string]string {
	return map[string]string{
		"live/root.hcl": `locals {
  region = "us-east-1"
}

remote_state {
  backend = "s3"
  config = {
    key = "${path_relative_to_include()}/tf.tfstate"
  }
}
`,
		"live/_common/app.hcl": `locals {
  base = "example"
}

inputs = {
  name = local.base
}
`,
		"live/prod/vpc/terragrunt.hcl": `include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "../../../modules/vpc"
}

inputs = {
  cidr = "10.0.0.0/16"
}
`,
		"live/prod/app/terragrunt.hcl": `include "root" {
  path = find_in_parent_folders("root.hcl")
}

include "common" {
  path = "${dirname(find_in_parent_folders("root.hcl"))}/_common/app.hcl"
}

dependency "vpc" {
  config_path = "../vpc"
}

terraform {
  source = "../../../modules/app"
}

inputs = {
  vpc_id = dependency.vpc.outputs.id
}
`,
		"modules/vpc/main.tf": "variable \"cidr\" {}\n\noutput \"id\" {\n  value = var.cidr\n}\n",
		"modules/app/main.tf": "variable \"vpc_id\" {}\n\nresource \"example_thing\" \"this\" {\n  vpc = var.vpc_id\n}\n",
		"vault.hcl":           "ui = true\n\nstorage \"raft\" {\n  path = \"/vault/data\"\n}\n",
		".terraform.lock.hcl": "provider \"registry.example.com/example/thing\" {\n  version = \"1.0.0\"\n}\n",
	}
}

func ingestStored(t *testing.T, id string, files map[string]string) (*Pipeline, store.Store, string, Report) {
	t.Helper()
	dir := t.TempDir()
	writeCorpus(t, dir, files)
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: id, Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	return p, st, dir, rep
}

// storedEdges is the doc-xref edges an ingest stored, "file|from->to".
func storedEdges(t *testing.T, st store.Store, id string) []string {
	t.Helper()
	all, err := st.TypedEdges(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			got = append(got, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(got)
	return got
}

// TestIngest_TerragruntDialect: ingest extracts a terragrunt.hcl and a
// root.hcl by basename and an included .hcl by its sniff, never a Vault
// configuration or a lock file (counted ambiguous), and stores the
// include, dependency, module-source and module-input edges.
func TestIngest_TerragruntDialect(t *testing.T) {
	p, st, dir, rep := ingestStored(t, "tg", terragruntCorpus())
	ctx := context.Background()
	if rep.AmbiguousUnclassified != 2 {
		t.Errorf("AmbiguousUnclassified = %d, want 2 (vault.hcl, .terraform.lock.hcl)", rep.AmbiguousUnclassified)
	}
	obs, err := st.QueryObservations(ctx, "tg", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		if o.SourceRef == "vault.hcl" || o.SourceRef == ".terraform.lock.hcl" {
			t.Errorf("%s|%s: an unclaimed .hcl was extracted as %s", o.SourceRef, o.Symbol, o.SourceKind)
		}
	}
	for key, kind := range map[string]string{
		"live/root.hcl|root.hcl":                      "TerragruntConfig",
		"live/root.hcl|remote_state":                  "TerragruntRemoteState",
		"live/_common/app.hcl|app.hcl":                "TerragruntConfig",
		"live/_common/app.hcl|input.name":             "TerragruntInput",
		"live/prod/app/terragrunt.hcl|include.common": "TerragruntInclude",
		"live/prod/app/terragrunt.hcl|dependency.vpc": "TerragruntDependency",
		"live/prod/app/terragrunt.hcl|terraform":      "TerragruntTerraform",
		"live/prod/vpc/terragrunt.hcl|input.cidr":     "TerragruntInput",
	} {
		if got[key] != kind {
			t.Errorf("%s = %q, want %q", key, got[key], kind)
		}
	}
	if rep.DocXrefEdges == 0 {
		t.Fatal("no doc-xref edge stored")
	}
	edges := storedEdges(t, st, "tg")
	// a symbol key is its package (the path without its extension) and name
	for _, want := range []string{
		"live/prod/app/terragrunt.hcl|tg::live/prod/app/terragrunt.include.root->tg::live/root.root.hcl",
		"live/prod/app/terragrunt.hcl|tg::live/prod/app/terragrunt.include.common->tg::live/_common/app.app.hcl",
		"live/prod/app/terragrunt.hcl|tg::live/prod/app/terragrunt.dependency.vpc->tg::live/prod/vpc/terragrunt.terragrunt.hcl",
		"live/prod/app/terragrunt.hcl|tg::live/prod/app/terragrunt.terraform->tg::modules/app/main.example_thing.this",
		"live/prod/app/terragrunt.hcl|tg::live/prod/app/terragrunt.terraform->tg::modules/app/main.var.vpc_id",
		"live/prod/app/terragrunt.hcl|tg::live/prod/app/terragrunt.input.vpc_id->tg::modules/app/main.var.vpc_id",
		"live/prod/vpc/terragrunt.hcl|tg::live/prod/vpc/terragrunt.include.root->tg::live/root.root.hcl",
		"live/prod/vpc/terragrunt.hcl|tg::live/prod/vpc/terragrunt.terraform->tg::modules/vpc/main.output.id",
		"live/prod/vpc/terragrunt.hcl|tg::live/prod/vpc/terragrunt.input.cidr->tg::modules/vpc/main.var.cidr",
	} {
		if !containsString(edges, want) {
			t.Errorf("stored edges lack %q (%d stored)", want, len(edges))
		}
	}

	// ReingestFile decides as ingest does: an edit to a basename file and to
	// the sniffed one is read through the terragrunt dialect.
	for rel, body := range map[string]string{
		"live/prod/app/terragrunt.hcl": "dependency \"queue\" {\n  config_path = \"../queue\"\n}\n",
		"live/_common/app.hcl":         "locals {\n  edited = true\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "tg", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	obs, err = st.QueryObservations(ctx, "tg", "")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, o := range obs {
		seen[o.SourceRef+"|"+o.Symbol] = true
	}
	for _, key := range []string{"live/prod/app/terragrunt.hcl|dependency.queue", "live/_common/app.hcl|local.edited"} {
		if !seen[key] {
			t.Errorf("after ReingestFile: %s missing", key)
		}
	}
	if seen["live/prod/app/terragrunt.hcl|include.common"] || seen["live/_common/app.hcl|input.name"] {
		t.Error("after ReingestFile: an edited file's old entities survived")
	}
}

// TestTerragruntDialectClassification: the manifest's match blocks classify
// each .hcl as the dispatch expects, and the sniff reads only Terragrunt's
// top-level blocks.
func TestTerragruntDialectClassification(t *testing.T) {
	rules := dialectRules()
	src := terragruntCorpus()
	for path, want := range map[string]struct{ dialect, by string }{
		"live/prod/app/terragrunt.hcl": {"terragrunt", connector.ClassifiedByBasename},
		"live/root.hcl":                {"terragrunt", connector.ClassifiedByBasename},
		"live/_common/app.hcl":         {"terragrunt", connector.ClassifiedBySniff},
		"vault.hcl":                    {"", connector.ClassifiedAmbiguous},
		".terraform.lock.hcl":          {"", connector.ClassifiedAmbiguous},
	} {
		v := connector.ClassifyDialect(rules, path, []byte(src[path]))
		if v.Dialect != want.dialect || v.By != want.by {
			t.Errorf("ClassifyDialect(%q) = %q by %q, want %q by %q", path, v.Dialect, v.By, want.dialect, want.by)
		}
	}
	for path, want := range map[string]string{"a/terragrunt.hcl": "terragrunt", "root.hcl": "terragrunt"} {
		if got := connector.LanguageForPath(path); got != want {
			t.Errorf("LanguageForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestResolveDocXrefs_TerragruntNearestAncestor: find_in_parent_folders'
// candidates resolve to the files of the repository, one per ancestor that
// has the file; an include of a file the repository lacks resolves to nothing.
func TestResolveDocXrefs_TerragruntNearestAncestor(t *testing.T) {
	src := map[string]string{
		"root.hcl":           "locals {}\n",
		"a/root.hcl":         "locals {}\n",
		"a/b/terragrunt.hcl": "include \"r\" {\n  path = find_in_parent_folders(\"root.hcl\")\n}\n",
		"x/y/terragrunt.hcl": "include \"r\" {\n  path = find_in_parent_folders(\"missing.hcl\")\n}\n",
		"x/z/terragrunt.hcl": "include \"r\" {\n  path = find_in_parent_folders(\"root.hcl\")\n}\n",
	}
	var paths []string
	for p := range src {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	x := extractor.NewTerragruntExtractor()
	var files []DocFile
	for _, rel := range paths {
		r, err := x.ExtractDoc("svc", rel, []byte(src[rel]))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, DocFile{Path: rel, Result: r})
	}
	got := xrefBrief(ResolveDocXrefs(files))
	want := []string{
		"a/b/terragrunt.hcl:include.r -includes-> a/root.hcl:root.hcl",
		"a/b/terragrunt.hcl:include.r -includes-> root.hcl:root.hcl",
		"x/z/terragrunt.hcl:include.r -includes-> root.hcl:root.hcl",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}
