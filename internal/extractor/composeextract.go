package extractor

import (
	"bytes"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"gopkg.in/yaml.v3"
)

// ComposeExtractor implements Extractor for docker-compose / Compose files
// (docker-compose.yml, docker-compose.yaml, compose.yml, compose.yaml).
//
// It uses yaml.Node to obtain accurate line numbers from the parsed YAML.
//
// Output:
//   - KindComposeService — one per entry under the top-level `services:` key.
//     Name = service name (the YAML map key).
//     Signature = the `image:` value, or "build: <context>" when there is no
//     image field (falls back to "build: ." when the build key has no context).
//     Line = the 1-based line number of the service key in the YAML document.
//     Calls = the service's depends_on list (service→service edges). depends_on
//     may be a YAML sequence (list) or a mapping (map keys are the names).
//     Tier = core.TierVerified (real YAML parse).
//
// Extract never returns a non-nil error.
type ComposeExtractor struct{}

// NewComposeExtractor returns a new ComposeExtractor.
func NewComposeExtractor() *ComposeExtractor { return &ComposeExtractor{} }

// Extract parses src as a Compose YAML file and returns one SymbolNode per
// service defined under the top-level `services:` key.
func (c *ComposeExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)

	// Parse into a yaml.Node so we have full control over line numbers and can
	// handle both the typed and positional parts of the document.
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(src)).Decode(&doc); err != nil {
		return nil, nil // malformed — best-effort, no error
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}

	// The document root is a mapping node.
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil
	}

	// Find the `services:` key.
	var servicesNode *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "services" {
			servicesNode = root.Content[i+1]
			break
		}
	}
	if servicesNode == nil || servicesNode.Kind != yaml.MappingNode {
		return nil, nil
	}

	var nodes []core.SymbolNode

	// Iterate over service key/value pairs.
	for i := 0; i+1 < len(servicesNode.Content); i += 2 {
		keyNode := servicesNode.Content[i]
		valNode := servicesNode.Content[i+1]

		svcName := keyNode.Value
		if svcName == "" {
			continue
		}
		line := keyNode.Line // 1-based line of the service name key

		// Decode the service value into a typed struct (best-effort).
		var svcDef composeServiceDef
		if valNode != nil && valNode.Kind == yaml.MappingNode {
			_ = valNode.Decode(&svcDef)
			// When `build:` is a scalar string, the struct decoder ignores it
			// (it expects a mapping). Extract it separately via the raw node.
			composeExtractBuildContext(valNode, &svcDef)
		}

		// Build the signature: prefer image, fall back to build context.
		sig := ""
		switch {
		case svcDef.Image != "":
			sig = svcDef.Image
		case svcDef.Build.Context != "":
			sig = "build: " + svcDef.Build.Context
		case svcDef.BuildStr != "":
			sig = "build: " + svcDef.BuildStr
		default:
			sig = "build: ."
		}

		// Resolve depends_on: may be a YAML sequence or mapping.
		// We use the raw yaml.Node to handle both forms reliably.
		calls := composeDependsOn(valNode)

		nodes = append(nodes, core.SymbolNode{
			Service:   service,
			Package:   pkg,
			Name:      svcName,
			Kind:      core.KindComposeService,
			FilePath:  relPath,
			Line:      line,
			Signature: sig,
			Calls:     calls,
			Tier:      core.TierVerified,
		})
	}

	// Deterministic output: sort by Line then Name.
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, nil
}

// composeServiceDef is the typed subset of a Compose service definition we need.
type composeServiceDef struct {
	Image     string       `yaml:"image"`
	BuildStr  string       `yaml:"-"` // populated below via raw node when build is a scalar
	Build     composeBuild `yaml:"build"`
	DependsOn interface{}  `yaml:"depends_on"` // list or map — handled via raw node
}

// composeBuild covers the `build:` mapping form.
type composeBuild struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

// composeDependsOn extracts the depends_on service names from a service
// value node. depends_on may be:
//   - a YAML sequence: ["db", "redis"]
//   - a YAML mapping:  {db: {condition: service_healthy}, redis: {}}
//
// Returns the sorted list of service names found (nil when absent).
func composeDependsOn(serviceValNode *yaml.Node) []string {
	if serviceValNode == nil || serviceValNode.Kind != yaml.MappingNode {
		return nil
	}

	// Find the depends_on key in the service mapping.
	var depNode *yaml.Node
	for i := 0; i+1 < len(serviceValNode.Content); i += 2 {
		if serviceValNode.Content[i].Value == "depends_on" {
			depNode = serviceValNode.Content[i+1]
			break
		}
	}
	if depNode == nil {
		return nil
	}

	var names []string
	switch depNode.Kind {
	case yaml.SequenceNode:
		// List form: [svc1, svc2, ...]
		for _, item := range depNode.Content {
			if v := strings.TrimSpace(item.Value); v != "" {
				names = append(names, v)
			}
		}
	case yaml.MappingNode:
		// Map form: {svc1: {condition: ...}, svc2: {}, ...}
		for j := 0; j+1 < len(depNode.Content); j += 2 {
			if v := strings.TrimSpace(depNode.Content[j].Value); v != "" {
				names = append(names, v)
			}
		}
	}

	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	return names
}

// composeExtractBuildContext extracts the build context from the raw YAML node
// when the `build:` key is a scalar (string) rather than a mapping.
// This is called after struct decode; the struct's BuildStr field is set here.
func composeExtractBuildContext(serviceValNode *yaml.Node, def *composeServiceDef) {
	if serviceValNode == nil || serviceValNode.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(serviceValNode.Content); i += 2 {
		if serviceValNode.Content[i].Value == "build" {
			val := serviceValNode.Content[i+1]
			if val.Kind == yaml.ScalarNode {
				def.BuildStr = val.Value
			}
			return
		}
	}
}

// ensure ComposeExtractor satisfies the Extractor interface at compile time.
var _ Extractor = (*ComposeExtractor)(nil)
