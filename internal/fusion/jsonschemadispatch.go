package fusion

// jsonschemadispatch.go — LB-Q-json-schema: ingest's claim of a JSON Schema
// document no basename names, by content or by the *.schema.json naming
// convention. A .json file is one when it decodes to a root mapping AND either
//
//   - its root `$schema` names a json-schema.org meta-schema (the json-schema
//     doc spec's detect.content_sniff claims the file; the root entity's
//     signature is that `$schema` value, so a nested `$schema` never claims), or
//   - its base name ends in `.schema.json` (the convention tooling uses for a
//     schema that omits `$schema`).
//
// A document an earlier verdict claimed (CloudFormation, OpenAPI, Grafana, ARM)
// and a file a basename claims (package.json, cdk.json) never reach this hook:
// resolveExtractor and ReingestFile ask it after appFrameworkDispatch, the last
// content claim, so every earlier verdict stands. A package.json or tsconfig.json
// that points `$schema` at a schemastore.org URL is an instance of a schema, not
// one, and stays JSON.

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// ToolJSONSchema: a JSON Schema document.
const ToolJSONSchema ToolKind = "json-schema"

var (
	jsonSchemaOnce sync.Once
	jsonSchemaExt  *extractor.DocExtractor

	// jsonSchemaMetaRe is a root `$schema` that names a json-schema.org
	// meta-schema (draft-04 through 2020-12, with or without a trailing #).
	jsonSchemaMetaRe = regexp.MustCompile(`^https?://json-schema\.org/`)
)

// jsonSchemaExtractor is the json-schema dialect's document engine, built
// once; nil when the manifest does not load (TestDialectManifests fails on
// that, and dispatch then claims nothing).
func jsonSchemaExtractor() *extractor.DocExtractor {
	jsonSchemaOnce.Do(func() {
		if x, err := extractor.NewDocDialectExtractor("json-schema"); err == nil {
			jsonSchemaExt = x
		}
	})
	return jsonSchemaExt
}

// jsonSchemaJSON reports whether a .json file is a JSON Schema document (see
// the file comment). A file the sniff does not claim and whose name is not
// *.schema.json is never decoded.
func jsonSchemaJSON(relPath string, content []byte) bool {
	if filepath.Ext(relPath) != ".json" {
		return false
	}
	x := jsonSchemaExtractor()
	if x == nil {
		return false
	}
	byName := strings.HasSuffix(strings.ToLower(path.Base(filepath.ToSlash(relPath))), ".schema.json")
	if !byName && !x.DetectsContent(content) {
		return false
	}
	res, err := x.ExtractDoc("", path.Base(filepath.ToSlash(relPath)), content)
	if err != nil {
		return false
	}
	for _, n := range res.Nodes {
		if n.Kind == "JSONSchemaDocument" {
			return byName || jsonSchemaMetaRe.MatchString(n.Signature)
		}
	}
	return false
}

// jsonSchemaDispatch returns the dialect's extractor and ToolKind when
// relPath is a JSON Schema document.
func jsonSchemaDispatch(relPath string, content []byte) (extractor.Extractor, ToolKind, bool) {
	if jsonSchemaJSON(relPath, content) {
		return jsonSchemaExtractor(), ToolJSONSchema, true
	}
	return nil, "", false
}

// JSONSchemaContentDialect names "json-schema" for a file ingest's dispatch
// reads as a JSON Schema document, "" otherwise. The language-breadth harness
// measures the dialect on exactly these files (indexbench lang-oracle), beside
// Pipeline.ContentDialect and AppFrameworkContentDialect.
func JSONSchemaContentDialect(relPath string, content []byte) string {
	if jsonSchemaJSON(relPath, content) {
		return string(ToolJSONSchema)
	}
	return ""
}
