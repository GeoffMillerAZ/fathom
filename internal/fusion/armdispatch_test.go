package fusion

// armdispatch_test.go — LB-F146: an ARM template or parameters file is
// claimed by its content (a root `$schema` naming a deployment template or
// parameters file), in ingest and in ReingestFile alike; a linked template
// by relativePath, or by a uri relative to the template's own link, is a
// cross-file edge; a document that only embeds a template (an Azure Policy
// definition), a JSON schema and generic JSON stay what they were.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	armMain = `{
  "$schema": "https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#",
  "contentVersion": "1.0.0.0",
  "parameters": {"location": {"type": "string"}},
  "variables": {"vnetName": "example-vnet"},
  "resources": [
    {"type": "Microsoft.Network/virtualNetworks", "apiVersion": "2023-09-01", "name": "[variables('vnetName')]", "location": "[parameters('location')]"},
    {"type": "Microsoft.Resources/deployments", "apiVersion": "2022-09-01", "name": "storage",
     "dependsOn": ["[resourceId('Microsoft.Network/virtualNetworks', variables('vnetName'))]"],
     "properties": {"mode": "Incremental", "templateLink": {"relativePath": "nested/storage.json"}}},
    {"type": "Microsoft.Resources/deployments", "apiVersion": "2022-09-01", "name": "dns",
     "properties": {"mode": "Incremental", "templateLink": {"uri": "[uri(deployment().properties.templateLink.uri, 'nested/dns.json')]"}}}
  ]
}
`
	armStorage = `{
  // a comment, as ARM allows
  "contentVersion": "1.0.0.0",
  "$schema": "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
  "resources": [{"type": "Microsoft.Storage/storageAccounts", "apiVersion": "2023-01-01", "name": "examplesa",}]
}
`
	armDNS    = "{\"$schema\": \"https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#\", \"contentVersion\": \"1.0.0.0\", \"resources\": []}\n"
	armParams = "{\"$schema\": \"https://schema.management.azure.com/schemas/2019-04-01/deploymentParameters.json#\", \"contentVersion\": \"1.0.0.0\", \"parameters\": {\"location\": {\"value\": \"westus\"}}}\n"
	armSub    = "{\"$schema\": \"https://schema.management.azure.com/schemas/2018-05-01/subscriptionDeploymentTemplate.json#\", \"contentVersion\": \"1.0.0.0\", \"resources\": [{\"type\": \"Microsoft.Resources/resourceGroups\", \"apiVersion\": \"2022-09-01\", \"name\": \"example-rg\", \"location\": \"westus\"}]}\n"
	// An Azure Policy definition embeds a template in deployIfNotExists: it
	// is no template (wave pc's format), and its root has no $schema.
	armPolicy = "{\"properties\": {\"policyRule\": {\"then\": {\"effect\": \"deployIfNotExists\", \"details\": {\"deployment\": {\"properties\": {\"template\": {\"$schema\": \"https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#\", \"contentVersion\": \"1.0.0.0\", \"resources\": []}}}}}}}}\n"
	// The ARM schema itself names deploymentTemplate.json as its id, never
	// as its own $schema.
	armSchema = "{\"$schema\": \"http://json-schema.org/draft-04/schema#\", \"id\": \"https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#\", \"properties\": {\"resources\": {}}}\n"
)

func armRepoCorpus() map[string]string {
	return map[string]string{
		"infra/azuredeploy.json":            armMain,
		"infra/azuredeploy.parameters.json": armParams,
		"infra/nested/storage.json":         armStorage,
		"infra/nested/dns.json":             armDNS,
		"infra/sub.json":                    armSub,
		"policy/deploy-storage.json":        armPolicy,
		"schemas/deploymentTemplate.json":   armSchema,
		"conf/settings.json":                "{\"name\": \"example\", \"resources\": []}\n",
		"package.json":                      "{\"name\": \"example\", \"version\": \"1.0.0\"}\n",
	}
}

func TestAppFrameworkDispatchClaims(t *testing.T) {
	cases := []struct {
		path, content string
		want          ToolKind // "" = not claimed
	}{
		{"infra/azuredeploy.json", armMain, ToolARMTemplate},
		{"infra/nested/storage.json", armStorage, ToolARMTemplate},
		{"infra/azuredeploy.parameters.json", armParams, ToolARMTemplate},
		{"infra/sub.json", armSub, ToolARMTemplate},
		{"policy/deploy-storage.json", armPolicy, ""},
		{"schemas/deploymentTemplate.json", armSchema, ""},
		{"conf/settings.json", "{\"name\": \"example\"}\n", ""},
		{"infra/azuredeploy.yaml", armMain, ""}, // an ARM file is JSON
		{"broken.json", "{\"$schema\": \"https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#\", \"resources\": [\n", ""},
		{"web/Pulumi.dev.yaml", "config:\n  aws:region: us-west-2\n", ToolPulumiYAML},
		{"web/Pulumi.prod.yml", "encryptionsalt: v1:abc\nconfig:\n  web:size: 2\n", ToolPulumiYAML},
		{"web/Pulumi.yaml", "name: web\nruntime: yaml\n", ""}, // a fast-path basename, not this hook's
		{"web/Pulumi.dev.yaml", "- just\n- a list\n", ""},
		{"web/notpulumi.dev.yaml", "config:\n  a: b\n", ""},
	}
	for _, c := range cases {
		_, got, ok := appFrameworkDispatch(c.path, []byte(c.content))
		if !ok {
			got = ""
		}
		if got != c.want {
			t.Errorf("%s: claimed as %q, want %q", c.path, got, c.want)
		}
		wantCD := ""
		if c.want == ToolARMTemplate {
			wantCD = "arm-templates"
		}
		if cd := AppFrameworkContentDialect(c.path, []byte(c.content)); cd != wantCD {
			t.Errorf("%s: AppFrameworkContentDialect = %q, want %q", c.path, cd, wantCD)
		}
	}
}

// TestIngest_ARMRepo: ingest reads every ARM file through the dialect (its
// content, not its extension, decides), keeps generic JSON and package.json
// where they were, and stores the linked-template edges.
func TestIngest_ARMRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, armRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "arm", Path: dir}); err != nil {
		t.Fatal(err)
	}
	got := armObservations(t, st, "arm")
	want := map[string][]string{
		"infra/azuredeploy.json|ARMTemplate|azuredeploy.json":          nil,
		"infra/azuredeploy.json|ARMParameter|location":                 nil,
		"infra/azuredeploy.json|ARMVariable|vnetName":                  nil,
		"infra/azuredeploy.json|ARMResource|[variables('vnetName')]":   {"location", "vnetName"},
		"infra/azuredeploy.json|ARMResource|storage":                   {"[variables('vnetName')]"}, // a dependsOn names a resource, not a variable
		"infra/azuredeploy.json|ARMResource|dns":                       nil,
		"infra/azuredeploy.parameters.json|ARMParameterValue|location": nil,
		"infra/nested/storage.json|ARMTemplate|storage.json":           nil,
		"infra/nested/storage.json|ARMResource|examplesa":              nil,
		"infra/nested/dns.json|ARMTemplate|dns.json":                   nil,
		"infra/sub.json|ARMTemplate|sub.json":                          nil,
		"infra/sub.json|ARMResource|example-rg":                        nil,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ARM observations\n got: %v\nwant: %v", got, want)
	}
	obs, _ := st.QueryObservations(ctx, "arm", "")
	for _, o := range obs {
		if o.SourceRef == "policy/deploy-storage.json" || o.SourceRef == "schemas/deploymentTemplate.json" || o.SourceRef == "conf/settings.json" {
			if o.SourceKind != core.KindJSONKey {
				t.Errorf("%s read as %s; it keeps the JSON extractor", o.SourceRef, o.SourceKind)
			}
		}
	}
	all, _ := st.TypedEdges(ctx, "arm")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	wantEdges := []string{
		"infra/azuredeploy.json|arm::infra/azuredeploy.dns->arm::infra/nested/dns.dns.json",
		"infra/azuredeploy.json|arm::infra/azuredeploy.storage->arm::infra/nested/storage.storage.json",
	}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Errorf("linked-template edges\n got: %q\nwant: %q", edges, wantEdges)
	}
}

// TestReingestFile_ARMRepo: a one-file refresh reads an ARM file as ingest
// does.
func TestReingestFile_ARMRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, armRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "arm", Path: dir}); err != nil {
		t.Fatal(err)
	}
	edits := map[string]string{
		"infra/nested/dns.json":             "{\"$schema\": \"https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#\", \"contentVersion\": \"1.0.0.0\", \"resources\": [{\"type\": \"Microsoft.Network/dnsZones\", \"apiVersion\": \"2018-05-01\", \"name\": \"example.com\"}]}\n",
		"infra/azuredeploy.parameters.json": "{\"$schema\": \"https://schema.management.azure.com/schemas/2019-04-01/deploymentParameters.json#\", \"contentVersion\": \"1.0.0.0\", \"parameters\": {\"location\": {\"value\": \"westus\"}, \"sku\": {\"value\": \"S1\"}}}\n",
	}
	for rel, body := range edits {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "arm", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	got := armObservations(t, st, "arm")
	for _, w := range []string{"infra/nested/dns.json|ARMResource|example.com", "infra/azuredeploy.parameters.json|ARMParameterValue|sku"} {
		if _, ok := got[w]; !ok {
			t.Errorf("after ReingestFile %s is missing", w)
		}
	}
}

// armObservations maps "file|SourceKind|symbol" to its calls for every ARM
// observation.
func armObservations(t *testing.T, st store.Store, svc string) map[string][]string {
	t.Helper()
	obs, err := st.QueryObservations(context.Background(), svc, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, o := range obs {
		switch o.SourceKind {
		case "ARMTemplate", "ARMParameter", "ARMParameterValue", "ARMVariable", "ARMResource", "ARMOutput", "ARMFunction":
			got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o.Calls
		}
	}
	return got
}

// TestIngest_AppFrameworkRepo (LB-F148): serverless.yml, Pulumi.yaml and
// cdk.json read through their dialects by name, a Pulumi stack file by the
// glob, in ingest and in ReingestFile alike; their references are the
// dialects' (a ${self:custom.x}, a Pulumi ${res.prop}, the program a CDK app
// runs).
func TestIngest_AppFrameworkRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"api/serverless.yml":  "service: example-api\nprovider:\n  name: aws\n  stage: dev\ncustom:\n  table: items-${self:provider.stage}\nfunctions:\n  list:\n    handler: src/list.handler\n    environment:\n      TABLE: ${self:custom.table}\n    events:\n      - httpApi: 'GET /items'\n",
		"web/Pulumi.yaml":     "name: example-web\nruntime: yaml\nresources:\n  bucket:\n    type: aws:s3:BucketV2\noutputs:\n  name: ${bucket.bucket}\n",
		"web/Pulumi.dev.yaml": "config:\n  aws:region: us-west-2\n",
		"cdk/cdk.json":        "{\"app\": \"npx ts-node bin/app.ts\", \"context\": {\"@aws-cdk/core:checkSecretUsage\": true}}\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "af", Path: dir}); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"api/serverless.yml|ServerlessService|example-api":           nil,
		"api/serverless.yml|ServerlessProvider|provider":             nil,
		"api/serverless.yml|ServerlessProviderKey|name":              nil,
		"api/serverless.yml|ServerlessProviderKey|stage":             nil,
		"api/serverless.yml|ServerlessCustom|table":                  {"stage"},
		"api/serverless.yml|ServerlessFunction|list":                 {"table"},
		"api/serverless.yml|ServerlessRoute|GET /items":              nil,
		"web/Pulumi.yaml|PulumiProject|example-web":                  nil,
		"web/Pulumi.yaml|PulumiResource|bucket":                      nil,
		"web/Pulumi.yaml|PulumiOutput|name":                          {"bucket"},
		"web/Pulumi.dev.yaml|PulumiConfig|aws:region":                nil,
		"cdk/cdk.json|CDKApp|app":                                    {"bin/app.ts"},
		"cdk/cdk.json|CDKFeatureFlag|@aws-cdk/core:checkSecretUsage": nil,
	}
	check := func(when string) {
		t.Helper()
		obs, err := st.QueryObservations(ctx, "af", "")
		if err != nil {
			t.Fatal(err)
		}
		got := map[string][]string{}
		for _, o := range obs {
			k := string(o.SourceKind)
			if strings.HasPrefix(k, "Serverless") || strings.HasPrefix(k, "Pulumi") || strings.HasPrefix(k, "CDK") {
				got[o.SourceRef+"|"+k+"|"+o.Symbol] = o.Calls
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: observations\n got: %v\nwant: %v", when, got, want)
		}
	}
	check("ingest")
	for _, rel := range []string{"api/serverless.yml", "web/Pulumi.yaml", "web/Pulumi.dev.yaml", "cdk/cdk.json"} {
		if _, err := p.ReingestFile(ctx, st, "af", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	check("ReingestFile")
}
