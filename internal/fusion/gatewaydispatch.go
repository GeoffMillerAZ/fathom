package fusion

// gatewaydispatch.go — ingest's claim of Envoy, Kong and Traefik
// configuration files (LB-F104, language-breadth px wave). Each is a doc-engine
// dialect (internal/extractor/dialects/{envoy,kong,traefik}) whose manifest
// names the conventional basenames and globs, and whose doc spec names the
// content that makes a file one (detect.content_sniff). A file is claimed only
// when BOTH hold: a manifest basename or glob matches, and the content passes
// the sniff. A name alone claims nothing (envoy-deployment.yaml is usually a
// Kubernetes Deployment; a dynamic*.yml is usually something else), and a
// content signal alone claims nothing (no manifest owns the file, so the
// language-breadth harness could not measure it).
//
// resolveExtractor (ingest and ExtractForBench) and ReingestFile both call
// gatewayDispatch before every other signal, so a Kubernetes manifest under
// a matching glob, a compose file that names one of these tools, and every
// file no manifest names read exactly as before.

import (
	"path/filepath"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const (
	// ToolEnvoy: an Envoy bootstrap or xDS resource file (LB-Q-envoy).
	ToolEnvoy ToolKind = "envoy"
	// ToolKong: a Kong declarative (decK, DB-less) file (LB-Q-kong).
	ToolKong ToolKind = "kong"
	// ToolTraefik: a Traefik static or file-provider file, YAML or TOML (LB-Q-traefik).
	ToolTraefik ToolKind = "traefik"
)

// gatewayKinds maps a dialect id to its ToolKind.
var gatewayKinds = map[string]ToolKind{
	"envoy":   ToolEnvoy,
	"kong":    ToolKong,
	"traefik": ToolTraefik,
}

var (
	gatewayOnce sync.Once
	gatewayExts map[string]*extractor.DocExtractor
)

// gatewayExtractors is the three dialects' document engines, built once; a
// dialect that does not load is absent (TestDialectManifests fails the build's
// tests on that, and dispatch then claims nothing for it).
func gatewayExtractors() map[string]*extractor.DocExtractor {
	gatewayOnce.Do(func() {
		gatewayExts = map[string]*extractor.DocExtractor{}
		for id := range gatewayKinds {
			if x, err := extractor.NewDocDialectExtractor(id); err == nil {
				gatewayExts[id] = x
			}
		}
	})
	return gatewayExts
}

// gatewayDispatch returns the dialect's extractor and ToolKind when relPath
// is an Envoy, Kong or Traefik configuration file: a manifest basename or
// glob names it and the dialect's content sniff claims it. A file that opens
// as a Kubernetes object (a top-level apiVersion and kind) is never one.
func gatewayDispatch(relPath string, content []byte) (extractor.Extractor, ToolKind, bool) {
	v := connector.ClassifyDialect(dialectRules(), relPath, content)
	if v.By != connector.ClassifiedByBasename && v.By != connector.ClassifiedByGlob {
		return nil, "", false
	}
	tk, ok := gatewayKinds[v.Dialect]
	if !ok {
		return nil, "", false
	}
	x := gatewayExtractors()[v.Dialect]
	if x == nil || !x.DetectsContent(content) {
		return nil, "", false
	}
	if ext := filepath.Ext(relPath); (ext == ".yaml" || ext == ".yml") && apiVersionTopRe.Match(content) && kindTopRe.Match(content) {
		return nil, "", false
	}
	return x, tk, true
}
