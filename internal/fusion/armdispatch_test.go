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
	"github.com/geoffmilleraz/fathom/internal/extractor"
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
	policyDefs := 0
	for _, o := range obs {
		switch o.SourceRef {
		case "schemas/deploymentTemplate.json", "conf/settings.json":
			if o.SourceKind != core.KindJSONKey {
				t.Errorf("%s read as %s; it keeps the JSON extractor", o.SourceRef, o.SourceKind)
			}
		case "policy/deploy-storage.json":
			// An Azure Policy definition embedding a template is never ARM's: wave pc's
			// azure-policy dialect (cloudPolicyDispatch, before this hook) reads it.
			if strings.HasPrefix(string(o.SourceKind), "ARM") {
				t.Errorf("%s read as %s; an Azure Policy definition is not an ARM template", o.SourceRef, o.SourceKind)
			}
			if o.SourceKind == "AzurePolicyDefinition" {
				policyDefs++
			}
		}
	}
	if policyDefs != 1 {
		t.Errorf("policy/deploy-storage.json: %d AzurePolicyDefinition observations, want 1 (the azure-policy dialect)", policyDefs)
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

// TestIngest_BicepRepo (LB-F147): a .bicep file reads through the bicep tags
// dialect (its grammar pinned in the lock), and the file resolver draws a
// module's file edge (through the module's own binding of its path) and a
// compile-time import's (through the imported name a declaration uses); a
// registry module resolves to no file.
func TestIngest_BicepRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"infra/main.bicep": "import { skuName as sku } from './types.bicep'\n\nparam location string = resourceGroup().location\nparam tier sku\n\n" +
			"module storage './modules/storage.bicep' = {\n  name: 'storage'\n  params: {\n    location: location\n    tier: tier\n  }\n}\n\n" +
			"module dns 'br/public:avm/res/network/dns-zone:0.5.0' = {\n  name: 'dns'\n}\n\noutput accountName string = storage.outputs.accountName\n",
		"infra/modules/storage.bicep": "param location string\nparam tier string\n\nresource account 'Microsoft.Storage/storageAccounts@2023-01-01' = {\n  name: 'examplesa'\n  location: location\n  sku: {\n    name: tier\n  }\n  kind: 'StorageV2'\n}\n\noutput accountName string = account.name\n",
		"infra/types.bicep":           "@export()\ntype skuName = 'Standard_LRS' | 'Premium_LRS'\n",
		"infra/main.bicepparam":       "using './main.bicep'\n\nparam location = 'westus'\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "bc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "bc", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, o := range obs {
		if strings.HasPrefix(string(o.SourceKind), "Bicep") {
			got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o.Calls
		}
	}
	want := map[string][]string{
		"infra/main.bicep|BicepParam|location":                nil,
		"infra/main.bicep|BicepParam|tier":                    {"sku"},
		"infra/main.bicep|BicepModule|storage":                {"./modules/storage.bicep.location", "./modules/storage.bicep.tier", "location", "tier"},
		"infra/main.bicep|BicepModule|dns":                    nil,
		"infra/main.bicep|BicepOutput|accountName":            {"storage"},
		"infra/modules/storage.bicep|BicepParam|location":     nil,
		"infra/modules/storage.bicep|BicepParam|tier":         nil,
		"infra/modules/storage.bicep|BicepResource|account":   {"location", "tier"},
		"infra/modules/storage.bicep|BicepOutput|accountName": {"account"},
		"infra/types.bicep|BicepType|skuName":                 nil,
		"infra/main.bicepparam|BicepParam|location":           nil,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bicep observations\n got: %v\nwant: %v", got, want)
	}
	all, _ := st.TypedEdges(ctx, "bc")
	files := map[string]bool{}
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			files[e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol] = true
		}
	}
	for _, w := range []string{
		"infra/main.bicep|bc::infra/main.storage->bc::infra/modules/storage.location",
		"infra/main.bicep|bc::infra/main.tier->bc::infra/types.skuName",
	} {
		if !files[w] {
			t.Errorf("missing file-resolved edge %s; have %v", w, files)
		}
	}
}

// TestResolveFileEdges_BicepModules (LB-F147): the file resolver binds a
// module's params keys, stored qualified by the module's path, to the target
// file's parameters, and a compile-time import's alias to the exported type;
// a registry module (br/...) is external and draws no cross-file edge.
func TestResolveFileEdges_BicepModules(t *testing.T) {
	dir := t.TempDir()
	corpus := map[string]string{
		"infra/main.bicep": "import { skuName as sku } from './types.bicep'\n\nparam location string = resourceGroup().location\nparam tier sku\n\n" +
			"module storage './modules/storage.bicep' = {\n  name: 'storage'\n  params: {\n    location: location\n    tier: tier\n  }\n}\n\n" +
			"module dns 'br/public:avm/res/network/dns-zone:0.5.0' = {\n  name: 'dns'\n  params: {\n    name: 'example.com'\n  }\n}\n",
		"infra/modules/storage.bicep": "param location string\nparam tier string\n\noutput accountName string = '${location}-${tier}'\n",
		"infra/types.bicep":           "@export()\ntype skuName = 'Standard_LRS' | 'Premium_LRS'\n",
	}
	writeCorpus(t, dir, corpus)
	e, err := extractor.NewTSExtractor("bicep")
	if err != nil {
		t.Fatal(err)
	}
	var files []core.SourceFile
	var nodes []core.SymbolNode
	for _, rel := range []string{"infra/main.bicep", "infra/modules/storage.bicep", "infra/types.bicep"} {
		src := []byte(corpus[rel])
		files = append(files, core.SourceFile{Path: rel, Content: src})
		ns, err := e.Extract("bc", rel, src)
		if err != nil {
			t.Fatalf("extract %s: %v", rel, err)
		}
		nodes = append(nodes, ns...)
	}
	edges, report := ResolveFileEdges(context.Background(), "bicep", dir, files, nodes)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	fileOf := map[string]string{}
	for _, n := range nodes {
		fileOf[n.Key()] = n.FilePath
	}
	var got []string // the cross-file edges; a file's own uses bind in it too
	for _, ed := range edges {
		if ed.Kind != core.EdgeFileResolved {
			t.Errorf("edge %+v: kind %q, want file-resolved", ed, ed.Kind)
		}
		if fileOf[ed.FromSymbol] != fileOf[ed.ToSymbol] {
			got = append(got, ed.FromSymbol+"->"+ed.ToSymbol)
		}
	}
	sort.Strings(got)
	want := []string{
		"bc::infra/main.storage->bc::infra/modules/storage.location",
		"bc::infra/main.storage->bc::infra/modules/storage.tier",
		"bc::infra/main.tier->bc::infra/types.skuName",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("file-resolved edges\n got: %v\nwant: %v", got, want)
	}
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
