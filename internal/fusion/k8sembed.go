package fusion

// k8sembed.go — documents a Kubernetes object embeds (wave c3, LB-F366;
// docs/design/language-breadth.md §4 "Observability documents"). A
// dashboard or an Alertmanager configuration often ships as a string inside
// a Kubernetes object rather than as a file of its own:
//
//   - a ConfigMap's (or a ConfigMapList item's) data value whose key ends
//     .json and that is a Grafana dashboard (kube-prometheus'
//     grafana-dashboardDefinitions.yaml), whose key is alertmanager.yml or
//     alertmanager.yaml, or whose key ends .yaml or .yml and that is a
//     Grafana provisioning file (grafana-dashboardSources.yaml);
//   - a grafana-operator GrafanaDashboard's spec.json;
//   - a Secret's alertmanager.yml or alertmanager.yaml, or a Grafana
//     provisioning file under a .yaml or .yml key, in stringData or as
//     base64 data (kube-prometheus' alertmanager-secret.yaml and
//     grafana-dashboardDatasources.yaml).
//
// A value is read only when its dialect's content sniff claims it, through
// that dialect's document engine (grafana-dashboard, alertmanager), as if it
// were a file: a block scalar's lines map onto the
// manifest's at its offset, a quoted or base64 value's onto the value's own
// line. Its entities sit beside the Kubernetes object's in the manifest,
// qualified by the key (`alertmanager.yaml/route.routes[0]`), so two
// documents of one manifest never share a name; a document root named by
// the file is named by its key instead, and a dashboard keeps its uid, which
// links name. Only these keys are read: a Secret's other values never are,
// and the alertmanager dialect itself selects no secret-bearing field.
// AlertmanagerConfig custom resources (monitoring.coreos.com/v1alpha1)
// carry a camel-cased schema of their own and are not read
// (LB-Q-alertmanager-rest).

import (
	"bytes"
	"encoding/base64"
	"path"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"gopkg.in/yaml.v3"
)

// embeddedDoc is one document a Kubernetes object embeds.
type embeddedDoc struct {
	dialect string // grafana-dashboard or alertmanager
	key     string // the qualifier: the data key, or the object's name
	object  string // the embedding object's metadata.name
	content []byte
	offset  int  // added to each of content's lines (a block scalar)
	flat    bool // every line is line, the value's own (a quoted or base64 value)
	line    int
}

// k8sEmbedCandidate is the cheap pre-check: an object kind that can embed a
// document, and a key that can hold one.
func k8sEmbedCandidate(content []byte) bool {
	if !bytes.Contains(content, []byte("ConfigMap")) && !bytes.Contains(content, []byte("Secret")) && !bytes.Contains(content, []byte("GrafanaDashboard")) {
		return false
	}
	return bytes.Contains(content, []byte(".json")) || bytes.Contains(content, []byte(".yaml")) || bytes.Contains(content, []byte(".yml")) || bytes.Contains(content, []byte("json:"))
}

// k8sEmbeddedDocs reads the documents a manifest's objects embed, in
// document order.
func k8sEmbeddedDocs(content []byte) []embeddedDoc {
	var out []embeddedDoc
	for _, doc := range yamlDocs(content) {
		embedObject(doc, &out, 0)
	}
	// a key two objects share is qualified by each object's name
	seen := map[string]int{}
	for _, d := range out {
		seen[d.key]++
	}
	for i := range out {
		if seen[out[i].key] > 1 && out[i].object != "" {
			out[i].key = out[i].object + "/" + out[i].key
		}
	}
	return out
}

// embedObject reads one object (a List's items, recursively once).
func embedObject(n *yaml.Node, out *[]embeddedDoc, depth int) {
	if n == nil || n.Kind != yaml.MappingNode || depth > 1 {
		return
	}
	kind := yscalar(ymap(n, "kind"))
	if strings.HasSuffix(kind, "List") {
		if items := ymap(n, "items"); items != nil && items.Kind == yaml.SequenceNode {
			for _, it := range items.Content {
				embedObject(it, out, depth+1)
			}
		}
		return
	}
	isAMKey := func(k string) bool { return k == "alertmanager.yaml" || k == "alertmanager.yml" }
	isYAMLKey := func(k string) bool { return strings.HasSuffix(k, ".yaml") || strings.HasSuffix(k, ".yml") }
	object := yscalar(ymap(n, "metadata", "name"))
	switch {
	case kind == "ConfigMap":
		data := ymap(n, "data")
		if data == nil || data.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(data.Content); i += 2 {
			k, v := data.Content[i].Value, data.Content[i+1]
			switch {
			case strings.HasSuffix(k, ".json"):
				addEmbedded(out, "grafana-dashboard", k, object, v, false)
			case isAMKey(k):
				addEmbedded(out, "alertmanager", k, object, v, false)
			case isYAMLKey(k):
				addEmbedded(out, "grafana-dashboard", k, object, v, false) // a Grafana provisioning file
			}
		}
	case kind == "Secret":
		for _, field := range []string{"stringData", "data"} {
			data := ymap(n, field)
			if data == nil || data.Kind != yaml.MappingNode {
				continue
			}
			for i := 0; i+1 < len(data.Content); i += 2 {
				switch k := data.Content[i].Value; {
				case isAMKey(k):
					addEmbedded(out, "alertmanager", k, object, data.Content[i+1], field == "data")
				case isYAMLKey(k):
					addEmbedded(out, "grafana-dashboard", k, object, data.Content[i+1], field == "data") // a Grafana provisioning file
				}
			}
		}
	case kind == "GrafanaDashboard" && strings.HasPrefix(yscalar(ymap(n, "apiVersion")), "grafana.integreatly.org/"):
		name := object
		if name == "" {
			name = "spec.json"
		}
		addEmbedded(out, "grafana-dashboard", name, object, ymap(n, "spec", "json"), false)
	}
}

// addEmbedded appends a value that is its dialect's document.
func addEmbedded(out *[]embeddedDoc, dialect, key, object string, v *yaml.Node, b64 bool) {
	if v == nil || v.Kind != yaml.ScalarNode || v.Value == "" {
		return
	}
	content := []byte(v.Value)
	flat := b64 || (v.Style != yaml.LiteralStyle && v.Style != yaml.FoldedStyle)
	if b64 {
		dec, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(v.Value), ""))
		if err != nil {
			return
		}
		content = dec
	}
	x := lateYAMLLookup(dialect)
	if x == nil || !x.DetectsContent(content) {
		return
	}
	*out = append(*out, embeddedDoc{dialect: dialect, key: key, object: object, content: content, offset: v.Line, flat: flat, line: v.Line})
}

// k8sEmbeddedResult is the merged extraction of every document a manifest
// embeds, each qualified by its key and placed at its lines; ok is false
// when none reads to an entity.
func k8sEmbeddedResult(service, relPath string, content []byte) (extractor.DocResult, bool) {
	var all extractor.DocResult
	for _, d := range k8sEmbeddedDocs(content) {
		x := lateYAMLLookup(d.dialect)
		if x == nil {
			continue
		}
		res, err := x.ExtractDoc(service, relPath, d.content)
		if err != nil || len(res.Roots) == 0 {
			continue
		}
		res = placeEmbedded(res, relPath, d)
		all.Nodes = append(all.Nodes, res.Nodes...)
		all.CrossFile = append(all.CrossFile, res.CrossFile...)
		all.Labels = append(all.Labels, res.Labels...)
		all.Roots = append(all.Roots, res.Roots...)
		all.Unresolved = append(all.Unresolved, res.Unresolved...)
		all.Matchers = append(all.Matchers, res.Matchers...)
		all.Facts = append(all.Facts, res.Facts...)
	}
	return all, len(all.Nodes) > 0
}

// placeEmbedded qualifies an embedded document's entities by its key and
// moves its lines onto the manifest's.
func placeEmbedded(res extractor.DocResult, relPath string, d embeddedDoc) extractor.DocResult {
	base := path.Base(relPath)
	isRoot := map[string]bool{}
	for _, r := range res.Roots {
		isRoot[r] = true
	}
	q := func(name string) string {
		switch {
		case name == "":
			return ""
		case name == base:
			return d.key // the document root, named by its file: its key now
		case isRoot[name]:
			return name // a dashboard's uid, which links name
		}
		return d.key + "/" + name
	}
	at := func(l int) int {
		switch {
		case l <= 0:
			return l
		case d.flat:
			return d.line
		}
		return l + d.offset
	}
	out := extractor.DocResult{}
	for _, n := range res.Nodes {
		n.Name, n.Line, n.EndLine = q(n.Name), at(n.Line), at(n.EndLine)
		if len(n.Calls) > 0 {
			calls := make([]string, len(n.Calls))
			for i, c := range n.Calls {
				calls[i] = q(c)
			}
			n.Calls = calls
		}
		out.Nodes = append(out.Nodes, n)
	}
	for _, r := range res.Roots {
		out.Roots = append(out.Roots, q(r))
	}
	for _, c := range res.CrossFile {
		c.From, c.Line = q(c.From), at(c.Line)
		out.CrossFile = append(out.CrossFile, c)
	}
	for _, u := range res.Unresolved {
		u.From, u.Line = q(u.From), at(u.Line)
		out.Unresolved = append(out.Unresolved, u)
	}
	for _, l := range res.Labels {
		l.Entity, l.Line = q(l.Entity), at(l.Line)
		out.Labels = append(out.Labels, l)
	}
	for _, m := range res.Matchers {
		m.Entity, m.Line = q(m.Entity), at(m.Line)
		out.Matchers = append(out.Matchers, m)
	}
	for _, f := range res.Facts {
		f.Entity, f.Line = q(f.Entity), at(f.Line)
		out.Facts = append(out.Facts, f)
	}
	return out
}

// k8sEmbedExtractor is a Kubernetes manifest's extractor plus the documents
// its objects embed. It is deliberately no DocResulter: the route and
// policy passes read the manifest as the Kubernetes object it is, and the
// doc-xref pass's hook (alertroute.go) reads the embedded documents.
type k8sEmbedExtractor struct {
	base extractor.Extractor
}

// Extract implements extractor.Extractor: the Kubernetes nodes, then the
// embedded documents' entities.
func (x k8sEmbedExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	nodes, err := x.base.Extract(service, relPath, src)
	if err != nil {
		return nodes, err
	}
	if res, ok := k8sEmbeddedResult(service, relPath, src); ok {
		nodes = append(nodes, res.Nodes...)
	}
	return nodes, nil
}

// k8sEmbedDispatch claims a Kubernetes manifest that embeds a document:
// the Kubernetes extractor wrapped with the embedded documents' reader, its
// tool kind unchanged. ingest's resolveExtractor and ReingestFile ask it.
func (p *Pipeline) k8sEmbedDispatch(relPath string, content []byte, idx *siblingIndex) (extractor.Extractor, ToolKind, bool) {
	if ext := strings.ToLower(path.Ext(relPath)); ext != ".yaml" && ext != ".yml" {
		return nil, "", false
	}
	if strings.Contains(relPath, ".github/workflows/") || !k8sEmbedCandidate(content) {
		return nil, "", false
	}
	if _, named := p.exts[path.Base(relPath)]; named {
		return nil, "", false
	}
	kind := Classify(relPath, content, idx)
	if kind != ToolK8sManifest && kind != ToolK8sCRD {
		return nil, "", false
	}
	base, ok := p.toolExts[kind]
	if !ok || len(k8sEmbeddedDocs(content)) == 0 {
		return nil, "", false
	}
	return k8sEmbedExtractor{base: base}, kind, true
}
