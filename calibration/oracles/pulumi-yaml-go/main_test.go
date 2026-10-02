package main

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func subject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const program = `name: site
runtime: yaml
description: a static site
config:
  siteDir:
    type: string
    default: www
variables:
  zone:
    fn::invoke:
      function: aws:route53:getZone
      arguments:
        name: example.org
resources:
  bucket:
    type: aws:s3:Bucket
    properties:
      website:
        indexDocument: index.html
  policy:
    type: aws:s3:BucketPolicy
    properties:
      bucket: ${bucket.id}
      note: "$${not.a.ref} in ${siteDir}"
    options:
      dependsOn:
        - ${bucket}
  notAResource:
    properties: {}
outputs:
  url: ${bucket.websiteEndpoint}
  zoneId: ${zone.id}
  stack: ${pulumi.stack}
`

// TestOracleEntities: Pulumi's own loaders decide every entity — the CLI's
// project loader (name, runtime, config) and pulumi-yaml's template parse
// (variables, resources with a type, outputs) — and pulumi-yaml's
// interpolation grammar the references; an independent YAML reader gives
// lines. A template is instantiated the way pulumi new does (${PROJECT}
// and ${DESCRIPTION} replaced) and its name mapped back to the name as
// written. A project file the project loader refuses yields nothing; one
// without a runtime is a project (its config keys) but no package, the
// dialect's rule read off the typed project.
func TestOracleEntities(t *testing.T) {
	dir := subject(t, map[string]string{
		"site/Pulumi.yaml": program,
		"node/Pulumi.yaml": "name: node-app\nruntime: nodejs\nconfig:\n  aws:region: us-west-2\n",
		"bare/Pulumi.yaml": "name: no-runtime\nconfig:\n  x: 1\n",
		"tmpl/Pulumi.yaml": "name: ${PROJECT}\ndescription: ${DESCRIPTION}\nruntime: yaml\nresources:\n  b:\n    type: aws:s3:Bucket\n",
		"bad/Pulumi.yaml":  "name: bad\nruntime: yaml\nconfig:\n  x:\n    type: string\n    default: 1\n",
	})
	o, err := run(dir)
	if err != nil {
		t.Fatal(err)
	}
	var defs []string
	for _, d := range o.Definitions {
		defs = append(defs, strings.Join([]string{d.Path, d.Name, d.Kind}, "|")+"@"+strconv.Itoa(d.Line))
	}
	sort.Strings(defs)
	want := []string{
		"bare/Pulumi.yaml|x|variable_decl@3",
		"node/Pulumi.yaml|aws:region|variable_decl@4",
		"node/Pulumi.yaml|node-app|package@1",
		"site/Pulumi.yaml|bucket|resource@15",
		"site/Pulumi.yaml|policy|resource@20",
		"site/Pulumi.yaml|siteDir|variable_decl@5",
		"site/Pulumi.yaml|site|package@1",
		"site/Pulumi.yaml|stack|output@33",
		"site/Pulumi.yaml|url|output@31",
		"site/Pulumi.yaml|zoneId|output@32",
		"site/Pulumi.yaml|zone|variable@9",
		"tmpl/Pulumi.yaml|${PROJECT}|package@1",
		"tmpl/Pulumi.yaml|b|resource@5",
	}
	if strings.Join(defs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("defs:\n%s\nwant:\n%s", strings.Join(defs, "\n"), strings.Join(want, "\n"))
	}
	var refs []string
	for _, r := range o.References {
		refs = append(refs, r.Path+"|"+r.From+"->"+r.To)
	}
	sort.Strings(refs)
	wantRefs := []string{
		"site/Pulumi.yaml|policy->bucket",
		"site/Pulumi.yaml|policy->siteDir",
		"site/Pulumi.yaml|url->bucket",
		"site/Pulumi.yaml|zoneId->zone",
	}
	if strings.Join(refs, "\n") != strings.Join(wantRefs, "\n") {
		t.Fatalf("refs:\n%s\nwant:\n%s", strings.Join(refs, "\n"), strings.Join(wantRefs, "\n"))
	}
	if o.stats.refused != 1 {
		t.Fatalf("refused = %d, want 1 (the project whose config default fails its type)", o.stats.refused)
	}
	if o.stats.lineMismatch != 0 {
		t.Fatalf("line mismatches against pulumi-yaml's own ranges: %d", o.stats.lineMismatch)
	}
}
