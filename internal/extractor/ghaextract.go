package extractor

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"gopkg.in/yaml.v3"
)

// GHAExtractor implements Extractor for GitHub Actions workflow YAML files
// (typically found under .github/workflows/).
//
// Output (all with Tier = TierVerified):
//   - KindGHAWorkflow — one per file. Name = workflow `name:` value (or file
//     basename without extension when absent). Signature = short trigger summary
//     like "on: push, pull_request". Line = 1 (or name node line when found).
//   - KindGHAJob — one per key under top-level `jobs:`. Name = job id.
//     Signature = `runs-on` value. Line = job key node line.
//     Calls = `needs` entries (string or list).
//   - KindGHAAction — one per step that has a `uses:` key. Name = uses ref
//     (e.g. "actions/checkout@v4"). Signature = step `name` if present.
//     Line = step node line.
//
// Best-effort: never returns a non-nil error; malformed input is skipped.
type GHAExtractor struct{}

// NewGHAExtractor returns a new GHAExtractor.
func NewGHAExtractor() *GHAExtractor { return &GHAExtractor{} }

// Extract parses src as a GitHub Actions workflow and returns SymbolNodes.
func (g *GHAExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)

	// Parse into a raw yaml.Node to preserve line numbers.
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		// Malformed YAML — return empty result (best-effort).
		return nil, nil
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, nil
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil, nil
	}

	var nodes []core.SymbolNode

	// Walk the top-level mapping key/value pairs.
	workflowName := ""
	workflowLine := 1
	var onSig string
	var jobsNode *yaml.Node

	pairs := mapping.Content
	for i := 0; i+1 < len(pairs); i += 2 {
		keyNode := pairs[i]
		valNode := pairs[i+1]
		switch keyNode.Value {
		case "name":
			workflowName = valNode.Value
			workflowLine = keyNode.Line
		case "on", "true": // yaml.v3 parses bare `on` as boolean true
			onSig = ghaOnSig(valNode)
		case "jobs":
			jobsNode = valNode
		}
	}

	// Workflow name fallback: file basename without extension.
	if workflowName == "" {
		base := filepath.Base(relPath)
		if dot := strings.LastIndex(base, "."); dot >= 0 {
			base = base[:dot]
		}
		workflowName = base
	}

	nodes = append(nodes, core.SymbolNode{
		Service:   service,
		Package:   pkg,
		Name:      workflowName,
		Kind:      core.KindGHAWorkflow,
		FilePath:  relPath,
		Line:      workflowLine,
		Signature: onSig,
		Tier:      core.TierVerified,
	})

	// Extract jobs.
	if jobsNode != nil && jobsNode.Kind == yaml.MappingNode {
		jobPairs := jobsNode.Content
		for i := 0; i+1 < len(jobPairs); i += 2 {
			jobKeyNode := jobPairs[i]
			jobValNode := jobPairs[i+1]
			jobID := jobKeyNode.Value

			runsOn := ""
			var needs []string
			var stepNodes []*yaml.Node

			if jobValNode.Kind == yaml.MappingNode {
				jobBody := jobValNode.Content
				for j := 0; j+1 < len(jobBody); j += 2 {
					k := jobBody[j]
					v := jobBody[j+1]
					switch k.Value {
					case "runs-on":
						runsOn = ghaScalarOrFirst(v)
					case "needs":
						needs = ghaStringOrList(v)
					case "steps":
						if v.Kind == yaml.SequenceNode {
							stepNodes = v.Content
						}
					}
				}
			}

			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      jobID,
				Kind:      core.KindGHAJob,
				FilePath:  relPath,
				Line:      jobKeyNode.Line,
				Signature: runsOn,
				Calls:     needs,
				Tier:      core.TierVerified,
			})

			// Extract action steps.
			for _, stepNode := range stepNodes {
				if stepNode.Kind != yaml.MappingNode {
					continue
				}
				usesVal := ""
				stepName := ""
				stepLine := stepNode.Line
				stepBody := stepNode.Content
				for j := 0; j+1 < len(stepBody); j += 2 {
					sk := stepBody[j]
					sv := stepBody[j+1]
					switch sk.Value {
					case "uses":
						usesVal = sv.Value
					case "name":
						stepName = sv.Value
					}
				}
				if usesVal != "" {
					nodes = append(nodes, core.SymbolNode{
						Service:   service,
						Package:   pkg,
						Name:      usesVal,
						Kind:      core.KindGHAAction,
						FilePath:  relPath,
						Line:      stepLine,
						Signature: stepName,
						Tier:      core.TierVerified,
					})
				}
			}
		}
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

// ghaOnSig builds a human-readable trigger summary from the `on:` node.
// The `on` key in YAML is a reserved boolean word; yaml.v3 may decode it as
// the key "true". The value can be:
//   - scalar string:  "push"
//   - sequence:       [push, pull_request]
//   - mapping:        {push: {branches: [main]}, pull_request: {}}
func ghaOnSig(n *yaml.Node) string {
	switch n.Kind {
	case yaml.ScalarNode:
		return fmt.Sprintf("on: %s", n.Value)
	case yaml.SequenceNode:
		parts := make([]string, 0, len(n.Content))
		for _, c := range n.Content {
			if c.Value != "" {
				parts = append(parts, c.Value)
			}
		}
		return fmt.Sprintf("on: %s", strings.Join(parts, ", "))
	case yaml.MappingNode:
		parts := make([]string, 0, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			parts = append(parts, n.Content[i].Value)
		}
		return fmt.Sprintf("on: %s", strings.Join(parts, ", "))
	}
	return ""
}

// ghaStringOrList extracts a string or list of strings from a yaml.Node.
// Used to parse the `needs:` field which can be a bare string or a sequence.
func ghaStringOrList(n *yaml.Node) []string {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Value != "" {
			return []string{n.Value}
		}
	case yaml.SequenceNode:
		var out []string
		for _, c := range n.Content {
			if c.Value != "" {
				out = append(out, c.Value)
			}
		}
		return out
	}
	return nil
}

// ghaScalarOrFirst returns the scalar value of a node, or the first sequence
// element if the node is a sequence (e.g. runs-on: [ubuntu-latest]).
func ghaScalarOrFirst(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind == yaml.ScalarNode {
		return n.Value
	}
	if n.Kind == yaml.SequenceNode && len(n.Content) > 0 {
		return n.Content[0].Value
	}
	return ""
}

// GHAPinSHA and GHAPinRef are the source kinds of a remote action pin node
// (LB-F311): GHAPinSHA when the ref after the @ is a full commit SHA (40
// hex), GHAPinRef when it is a tag or a branch (the two read alike).
const (
	GHAPinSHA core.SourceKind = "GHAPinSHA"
	GHAPinRef core.SourceKind = "GHAPinRef"
)

// ghaPinPrefix names a pin node: "pin:" and the uses: value as written.
const ghaPinPrefix = "pin:"

// GHAPinKind classifies a step's uses: value. ok is false for anything but a
// remote action reference `owner/repo[/path]@ref`: a local `./path`, a
// `docker://` image, or a value with no ref.
func GHAPinKind(uses string) (kind core.SourceKind, ok bool) {
	if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "../") || strings.HasPrefix(uses, "docker://") || strings.Contains(uses, "${{") {
		return "", false
	}
	repo, ref, found := strings.Cut(uses, "@")
	if !found || ref == "" || !strings.Contains(repo, "/") {
		return "", false
	}
	if len(ref) == 40 && isLowerHex(ref) {
		return GHAPinSHA, true
	}
	return GHAPinRef, true
}

func isLowerHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// GHADialectExtractor is the github-actions document dialect plus a pin node
// for each remote action a step uses, classified by GHAPinKind: the doc engine
// has no regular expression in an entity's selection, so the classification
// is made here, over the dialect's own GHAAction entities. The language-
// breadth harness measures the dialect without the pins (they are not an
// oracle's entity).
type GHADialectExtractor struct{ inner *DocExtractor }

// NewGHADialectExtractor wraps the github-actions dialect's document engine.
func NewGHADialectExtractor(inner *DocExtractor) *GHADialectExtractor {
	return &GHADialectExtractor{inner: inner}
}

// Extract implements Extractor.
func (g *GHADialectExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := g.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// ExtractDoc implements DocResulter.
func (g *GHADialectExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	r, err := g.inner.ExtractDoc(service, relPath, src)
	if err != nil {
		return r, err
	}
	seen := map[string]bool{}
	var pins []core.SymbolNode
	for _, n := range r.Nodes {
		if n.Kind != core.KindGHAAction {
			continue
		}
		kind, ok := GHAPinKind(n.Name)
		if !ok {
			continue
		}
		name := ghaPinPrefix + n.Name
		if key := n.Package + "\x00" + name; !seen[key] {
			seen[key] = true
			repo, _, _ := strings.Cut(n.Name, "@")
			pins = append(pins, core.SymbolNode{
				Service: n.Service, Package: n.Package, Name: name, Kind: kind, FilePath: n.FilePath,
				Line: n.Line, EndLine: n.EndLine, Signature: repo, Tier: n.Tier,
			})
		}
	}
	r.Nodes = append(r.Nodes, pins...)
	sort.SliceStable(r.Nodes, func(i, j int) bool { return r.Nodes[i].Line < r.Nodes[j].Line })
	return r, nil
}
