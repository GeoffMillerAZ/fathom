package extractor

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"gopkg.in/yaml.v3"
)

// helmTemplatePlaceholder replaces every {{ ... }} action span (see
// substituteHelmActions) before YAML decode. It must be a bare YAML scalar
// safe in any position an action can appear — a mapping value
// ("image: {{ .Values.image }}"), a fragment of a larger scalar
// ("my-{{ .Values.name }}-svc"), or a whole line by itself
// ("{{- if .Values.x }}"). It deliberately contains no YAML-special
// characters and is unlikely to collide with real chart content.
const helmTemplatePlaceholder = "__FATHOM_HELM_TPL__"

// valuesRefRe finds `.Values.dotted.path` references in raw template text —
// it MUST run before substituteHelmActions, which destroys the actual path
// text inside the {{ }} span.
var valuesRefRe = regexp.MustCompile(`\.Values((?:\.[A-Za-z0-9_]+)+)`)

// HelmTemplateYAMLExtractor implements Extractor for Helm chart templates
// under templates/*.yaml (and *.yml) — dispatched by fusion's pass-3
// classifier when the path is under a templates/ directory with an ancestor
// Chart.yaml (ADR-021 §2 / TI-2).
//
// Grounding: yaml.v3 fails outright on raw Go-template {{ }} syntax, so
// templates/*.yaml — the majority of a real chart's content, the .tpl
// partials are the minority — silently produced zero symbols. This
// extractor replaces every {{ ... }} span with a scalar placeholder before
// decoding, recovering the k8s document shape for the common case (a
// template whose keys are static YAML and whose VALUES are templated).
// Control-flow blocks ({{- if }}/{{- range }}/{{- end }}) that span
// document structure are a documented best-effort limit: the substituted
// line still isn't valid YAML at that position, so a document built mostly
// from conditionals may still fail to decode — the same "skip this
// document" posture k8sextract.go already takes for malformed documents.
//
// Output (Tier = TierVerified except where noted):
//   - KindK8sWorkload / KindK8sContainer — same shape as k8sextract.go's
//     documents. When metadata.name is itself templated (resolves to the
//     placeholder or is empty), Name falls back to the file's stem and
//     Signature gets a trailing " templated:true" marker.  A container
//     image that is entirely a template expression (no literal value) is
//     dropped rather than emitted with the placeholder as its "image".
//   - KindHelmValueRef (Tier = TierPreScan, heuristic regex over raw text) —
//     one per unique `.Values.x.y` reference found anywhere in the file,
//     dotted path with the leading "Values." stripped. Queryable via the
//     `references` tool alongside the corresponding values.yaml declaration
//     (TI-7's values-key ↔ template-usage cross-ref).
//
// Extract never returns a non-nil error.
type HelmTemplateYAMLExtractor struct{}

// NewHelmTemplateYAMLExtractor returns a new HelmTemplateYAMLExtractor.
func NewHelmTemplateYAMLExtractor() *HelmTemplateYAMLExtractor {
	return &HelmTemplateYAMLExtractor{}
}

// Extract parses src as a (templated) Helm k8s manifest and returns
// SymbolNodes for its documents plus its .Values references.
func (h *HelmTemplateYAMLExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)
	var nodes []core.SymbolNode

	nodes = append(nodes, helmValuesRefNodes(service, pkg, relPath, src)...)

	substituted := substituteHelmActions(src)
	// pkgFromPath returns the file's basename with its extension stripped —
	// exactly the "file stem" the design calls for as the templated-name
	// fallback, so it is reused here rather than re-deriving it.
	stem := pkg

	for _, chunk := range k8sSplitDocs(substituted) {
		dec := yaml.NewDecoder(bytes.NewReader([]byte(chunk.text)))
		var rawNode yaml.Node
		if err := dec.Decode(&rawNode); err != nil || len(rawNode.Content) == 0 {
			continue
		}
		docLine := chunk.lineStart + rawNode.Line - 1

		var doc k8sDoc
		if err := rawNode.Decode(&doc); err != nil {
			continue
		}
		if doc.Kind == "" {
			continue
		}

		name := doc.Metadata.Name
		sig := doc.APIVersion
		if name == "" || strings.Contains(name, helmTemplatePlaceholder) {
			name = stem
			if sig != "" {
				sig += " "
			}
			sig += "templated:true"
		}
		symName := doc.Kind + "/" + name
		if doc.Metadata.Namespace != "" && !strings.Contains(doc.Metadata.Namespace, helmTemplatePlaceholder) {
			symName = doc.Kind + "/" + doc.Metadata.Namespace + "/" + name
		}

		nodes = append(nodes, core.SymbolNode{
			Service: service, Package: pkg, Name: symName,
			Kind: core.KindK8sWorkload, FilePath: relPath, Line: docLine,
			Signature: sig, Tier: core.TierVerified,
		})

		var containers []k8sContainer
		containers = append(containers, doc.Spec.Containers...)
		containers = append(containers, doc.Spec.InitContainers...)
		containers = append(containers, doc.Spec.Template.Spec.Containers...)
		containers = append(containers, doc.Spec.Template.Spec.InitContainers...)
		for _, c := range containers {
			if c.Image == "" || isFullyTemplated(c.Image) {
				// A fully-templated image reference carries no literal
				// value worth recording. This is NOT just the single-action
				// case (`image: {{ .Values.image }}` → exactly the
				// placeholder) — a repo+tag built from TWO separate actions
				// joined by punctuation (`"{{ .Values.image.repository }}:
				// {{ .Values.image.tag }}"` → "PLACEHOLDER:PLACEHOLDER")
				// also carries zero literal content once both placeholders
				// are stripped, so isFullyTemplated checks that too. A
				// partially templated reference ("myapp:{{ .Chart.
				// AppVersion }}" → "myapp:PLACEHOLDER") still carries a real
				// repository name and is kept as-is.
				continue
			}
			nodes = append(nodes, core.SymbolNode{
				Service: service, Package: pkg, Name: c.Image,
				Kind: core.KindK8sContainer, FilePath: relPath, Line: docLine,
				Signature: c.Name, Tier: core.TierVerified,
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

// helmValuesRefNodes scans raw (pre-substitution) template text for
// `.Values.x.y` references and returns one KindHelmValueRef node per unique
// dotted path, in first-seen order (stable — the caller sorts the final
// slice by line anyway).
func helmValuesRefNodes(service, pkg, relPath string, src []byte) []core.SymbolNode {
	matches := valuesRefRe.FindAllSubmatchIndex(src, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(matches))
	var nodes []core.SymbolNode
	for _, m := range matches {
		dotted := strings.TrimPrefix(string(src[m[2]:m[3]]), ".")
		if dotted == "" || seen[dotted] {
			continue
		}
		seen[dotted] = true
		line := 1 + bytes.Count(src[:m[0]], []byte("\n"))
		nodes = append(nodes, core.SymbolNode{
			Service: service, Package: pkg, Name: dotted,
			Kind: core.KindHelmValueRef, FilePath: relPath, Line: line,
			Tier: core.TierPreScan,
		})
	}
	return nodes
}

// substituteHelmActions replaces every {{ ... }} action span so the
// surrounding YAML decodes as ordinary scalars. Two substitution modes
// (ADR-021 §2, critic F2):
//
//   - An action EMBEDDED in a line that also carries literal content
//     ("image: {{ .Values.image }}") becomes helmTemplatePlaceholder — the
//     line keeps its YAML shape with a scalar in the action's place.
//   - A line whose non-whitespace content consists ENTIRELY of action spans
//     ("{{- if .Values.ingress.enabled -}}", "  {{- end }}",
//     "{{- if .x }}{{- end }}") is control flow, not data: its spans are
//     DROPPED (replaced with nothing), leaving a blank line. Substituting a
//     placeholder there produced a bare scalar above/inside the document
//     mapping, which broke the whole decode — `helm create` wraps half its
//     templates (ingress/serviceaccount/hpa) in exactly such a guard, so
//     the old behavior zeroed out half of every standard chart.
//
// Spans are found via a depth counter over the two-character "{{"/"}}"
// delimiter PAIRS, not individual '{'/'}' runes: Sprig/Helm expressions
// occasionally embed a literal brace inside a string argument (e.g.
// `dict "a" 1`), and counting single braces would mis-close on those.
// Counting only delimiter pairs is conservative — it can only ever merge a
// genuinely nested "{{ ... {{ ... }} ... }}" occurrence (rare, e.g.
// `{{ tpl "{{ .Values.x }}" . }}`) into a single placeholder, and never
// mis-closes on a lone brace from an unrelated expression.
func substituteHelmActions(src []byte) []byte {
	type span struct{ start, end int }
	var spans []span
	i := 0
	for i < len(src) {
		if i+1 < len(src) && src[i] == '{' && src[i+1] == '{' {
			depth := 1
			j := i + 2
			for j < len(src) && depth > 0 {
				switch {
				case j+1 < len(src) && src[j] == '{' && src[j+1] == '{':
					depth++
					j += 2
				case j+1 < len(src) && src[j] == '}' && src[j+1] == '}':
					depth--
					j += 2
				default:
					j++
				}
			}
			spans = append(spans, span{start: i, end: j})
			i = j
			continue
		}
		i++
	}
	if len(spans) == 0 {
		return src
	}

	// Whole-line detection. Spans are sorted and non-overlapping, so the
	// spans sharing span k's line region are contiguous around k — walk
	// backwards/forwards from k rather than rescanning the whole list.
	drop := make([]bool, len(spans))
	decided := make([]bool, len(spans))
	for k := range spans {
		if decided[k] {
			continue
		}
		// The line region: start of the span's first line to end of its
		// last line (a multi-line span's region covers all its lines).
		ls := bytes.LastIndexByte(src[:spans[k].start], '\n') + 1
		le := len(src)
		if idx := bytes.IndexByte(src[spans[k].end:], '\n'); idx >= 0 {
			le = spans[k].end + idx
		}
		first := k
		for first > 0 && spans[first-1].end > ls {
			first--
		}
		last := k
		for last+1 < len(spans) && spans[last+1].start < le {
			last++
		}
		wholeLine := true
		pos := ls
		for m := first; m <= last; m++ {
			for ; pos < spans[m].start; pos++ {
				if !isHelmSpace(src[pos]) {
					wholeLine = false
				}
			}
			if spans[m].end > pos {
				pos = spans[m].end
			}
		}
		for ; pos < le; pos++ {
			if !isHelmSpace(src[pos]) {
				wholeLine = false
			}
		}
		for m := first; m <= last; m++ {
			decided[m] = true
			drop[m] = wholeLine
		}
	}

	var out bytes.Buffer
	prev := 0
	for k, s := range spans {
		out.Write(src[prev:s.start])
		if !drop[k] {
			out.WriteString(helmTemplatePlaceholder)
		}
		prev = s.end
	}
	out.Write(src[prev:])
	return out.Bytes()
}

// isHelmSpace reports whether b is intra-line whitespace (space, tab, CR) —
// the byte class ignored when deciding whether a line consists solely of
// template actions.
func isHelmSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' }

// isFullyTemplated reports whether a substituted image string carries zero
// literal content once every placeholder occurrence is removed — true for
// both a single fully-templated action ("PLACEHOLDER") and multiple actions
// joined by nothing but punctuation ("PLACEHOLDER:PLACEHOLDER",
// "PLACEHOLDER@PLACEHOLDER"), which are just as useless as a single action:
// neither carries a real repository name worth cross-referencing.
func isFullyTemplated(image string) bool {
	stripped := strings.ReplaceAll(image, helmTemplatePlaceholder, "")
	stripped = strings.Trim(stripped, "/:@")
	return stripped == ""
}

// ensure HelmTemplateYAMLExtractor satisfies the Extractor interface at
// compile time.
var _ Extractor = (*HelmTemplateYAMLExtractor)(nil)
