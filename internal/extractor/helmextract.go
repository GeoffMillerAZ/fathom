package extractor

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
	"text/template/parse"

	"github.com/geoffmilleraz/fathom/internal/core"
	"gopkg.in/yaml.v3"
)

// ─── HelmChartExtractor ──────────────────────────────────────────────────────

// HelmChartExtractor implements Extractor for Helm Chart.yaml files.
//
// Output:
//   - KindHelmChart      — one node; Name = chart name, Signature = version.
//   - KindHelmDependency — one per entry in `dependencies:`; Name = dep name,
//     Signature = "<version> <repository>".
//
// Tier is TierVerified (real YAML parse). Line numbers are taken from the
// yaml.Node document line where available.
type HelmChartExtractor struct{}

// NewHelmChartExtractor returns a new HelmChartExtractor.
func NewHelmChartExtractor() *HelmChartExtractor { return &HelmChartExtractor{} }

// Extract parses a Helm Chart.yaml and returns SymbolNodes.
// It never returns a non-nil error.
func (h *HelmChartExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)

	// Parse into a yaml.Node so we can obtain line numbers.
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(src)).Decode(&doc); err != nil || len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0] // mapping node

	// Extract typed values.
	type helmChart struct {
		Name         string `yaml:"name"`
		Version      string `yaml:"version"`
		Dependencies []struct {
			Name       string `yaml:"name"`
			Version    string `yaml:"version"`
			Repository string `yaml:"repository"`
		} `yaml:"dependencies"`
	}
	var chart helmChart
	if err := root.Decode(&chart); err != nil {
		return nil, nil
	}
	if chart.Name == "" {
		return nil, nil
	}

	// Line of the document root (fallback to 1).
	docLine := root.Line
	if docLine == 0 {
		docLine = 1
	}

	var nodes []core.SymbolNode

	// Chart node.
	nodes = append(nodes, core.SymbolNode{
		Service:   service,
		Package:   pkg,
		Name:      chart.Name,
		Kind:      core.KindHelmChart,
		FilePath:  relPath,
		Line:      docLine,
		Signature: chart.Version,
		Tier:      core.TierVerified,
	})

	// Find the line of the `dependencies:` key so we can give each dep a
	// better line number. Walk the root mapping's key-value pairs.
	depsLine := 0
	for i := 0; i+1 < len(root.Content); i += 2 {
		kn := root.Content[i]
		vn := root.Content[i+1]
		if kn.Value == "dependencies" {
			depsLine = kn.Line
			// Try to get per-dep lines from the sequence.
			if vn.Kind == yaml.SequenceNode {
				for di, depNode := range vn.Content {
					if di < len(chart.Dependencies) {
						dep := chart.Dependencies[di]
						if dep.Name == "" {
							continue
						}
						sig := dep.Version
						if dep.Repository != "" {
							sig = dep.Version + " " + dep.Repository
						}
						line := depNode.Line
						if line == 0 {
							line = depsLine
						}
						nodes = append(nodes, core.SymbolNode{
							Service:   service,
							Package:   pkg,
							Name:      dep.Name,
							Kind:      core.KindHelmDependency,
							FilePath:  relPath,
							Line:      line,
							Signature: sig,
							Tier:      core.TierVerified,
						})
					}
				}
			}
			break
		}
	}

	// Fallback: if we didn't emit deps via the node walk (e.g. deps key not found
	// in content), emit them at depsLine or docLine.
	if depsLine == 0 {
		for _, dep := range chart.Dependencies {
			if dep.Name == "" {
				continue
			}
			sig := dep.Version
			if dep.Repository != "" {
				sig = dep.Version + " " + dep.Repository
			}
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      dep.Name,
				Kind:      core.KindHelmDependency,
				FilePath:  relPath,
				Line:      docLine,
				Signature: sig,
				Tier:      core.TierVerified,
			})
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// ─── HelmValuesExtractor ─────────────────────────────────────────────────────

// HelmValuesExtractor implements Extractor for Helm values.yaml files.
//
// Output:
//   - KindHelmValue — one per top-level key in the YAML mapping; Name = key,
//     Line = key node line number.
//
// Only top-level keys are emitted (no recursion). Tier = TierVerified.
type HelmValuesExtractor struct{}

// NewHelmValuesExtractor returns a new HelmValuesExtractor.
func NewHelmValuesExtractor() *HelmValuesExtractor { return &HelmValuesExtractor{} }

// Extract parses a Helm values.yaml and returns SymbolNodes for each top-level key.
// It never returns a non-nil error.
func (h *HelmValuesExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)

	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(src)).Decode(&doc); err != nil || len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil
	}

	var nodes []core.SymbolNode
	// Mapping nodes store key-value pairs as consecutive Content entries.
	for i := 0; i+1 < len(root.Content); i += 2 {
		kn := root.Content[i]
		if kn.Value == "" {
			continue
		}
		line := kn.Line
		if line == 0 {
			line = 1
		}
		nodes = append(nodes, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     kn.Value,
			Kind:     core.KindHelmValue,
			FilePath: relPath,
			Line:     line,
			Tier:     core.TierVerified,
		})
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// ─── HelmTemplateExtractor ───────────────────────────────────────────────────

// HelmTemplateExtractor implements Extractor for Helm .tpl template files.
//
// Strategy:
//  1. Try text/template/parse.Parse with a comprehensive Helm+sprig funcs map.
//     Named templates from the parse tree → KindHelmTemplate, TierVerified.
//  2. On parse failure, fall back to regex `\{\{-?\s*define\s+"([^"]+)"`.
//     Named templates from regex → KindHelmTemplate, TierPreScan.
//
// Never returns a non-nil error.
type HelmTemplateExtractor struct{}

// NewHelmTemplateExtractor returns a new HelmTemplateExtractor.
func NewHelmTemplateExtractor() *HelmTemplateExtractor { return &HelmTemplateExtractor{} }

// helmFuncs is a map of common Helm + sprig function names to dummy functions.
// text/template/parse only needs the names to exist (not callable values) so
// all entries use the same dummy closure.
var helmFuncs = func() map[string]any {
	dummy := func() string { return "" }
	names := []string{
		"include", "required", "tpl", "toYaml", "toJson", "fromYaml",
		"nindent", "indent", "trim", "trimSuffix", "trimPrefix",
		"default", "quote", "squote", "b64enc", "b64dec",
		"upper", "lower", "title", "replace", "printf",
		"hasPrefix", "hasSuffix", "contains", "split",
		"list", "dict", "get", "kindIs", "typeIs", "empty",
		"ternary", "coalesce", "semverCompare", "sha256sum",
		"lookup", "regexReplaceAll", "trunc", "repeat",
		"len", "eq", "ne", "and", "or", "not",
	}
	m := make(map[string]any, len(names))
	for _, n := range names {
		m[n] = dummy
	}
	return m
}()

// helmDefineRe matches {{- define "..." -}} and {{ define "..." }} patterns.
var helmDefineRe = regexp.MustCompile(`\{\{-?\s*define\s+"([^"]+)"`)

// Extract parses a Helm .tpl file and returns SymbolNodes for each named template.
// It never returns a non-nil error.
func (h *HelmTemplateExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)

	// Try real parse first. parse.Parse is the package-level function returning
	// map[string]*parse.Tree keyed by template name.
	trees, err := parse.Parse(relPath, string(src), "{{", "}}", helmFuncs)
	if err == nil {
		var nodes []core.SymbolNode
		for name := range trees {
			// Skip the root template (same name as relPath, or empty string).
			if name == relPath || name == "" {
				continue
			}
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindHelmTemplate,
				FilePath: relPath,
				Line:     1, // parse.Tree doesn't expose line of define; use 1.
				Tier:     core.TierVerified,
			})
		}
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].Line != nodes[j].Line {
				return nodes[i].Line < nodes[j].Line
			}
			return nodes[i].Name < nodes[j].Name
		})
		return nodes, nil
	}

	// Fallback: regex over raw bytes.
	var nodes []core.SymbolNode
	lines := bytes.Split(src, []byte("\n"))
	for lineIdx, line := range lines {
		matches := helmDefineRe.FindAllSubmatch(line, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			name := strings.TrimSpace(string(m[1]))
			if name == "" {
				continue
			}
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindHelmTemplate,
				FilePath: relPath,
				Line:     lineIdx + 1, // 1-based
				Tier:     core.TierPreScan,
			})
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}
