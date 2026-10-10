package mcp

// secret_withhold.go — SR-76 (wave h3): no stored secret through a read
// tool. LB-F367 withheld symbol_context's snippet for a definition of a kind
// a dialect declares sensitive (extractor.SensitiveKind: an Alertmanager
// configuration's and a Grafana provisioning file's entities). A file whose
// values may be secrets holds more than those definitions: a Kubernetes
// Secret embedding an Alertmanager configuration also holds the Secret's own
// definition and its other keys, and the references tool's occurrence rows
// serve a line of any file the query word appears in. The rule here is the
// file's: every definition in such a file is answered without its source,
// and an occurrence in it without its line text, by the same withheld-reason
// shape symbol_context's snippet_withheld set. read_source and search's
// literal grep mode stay raw-bytes readers by design (docs/mcp-tools.md).

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// k8sSecretKind names a Kubernetes Secret in a withheld reason: the
// Kubernetes extractor's workload entity carries no kind of its own for it.
const k8sSecretKind = "Kubernetes Secret"

// secretValuedKind is why n's values may be secrets — its kind, when a
// dialect declares that kind sensitive, or k8sSecretKind for a core
// Kubernetes Secret (a KindK8sWorkload "Secret/[namespace/]name" of
// apiVersion v1) — and "" otherwise.
func secretValuedKind(n core.SymbolNode) string {
	if extractor.SensitiveKind(string(n.Kind)) {
		return string(n.Kind)
	}
	if n.Kind == core.KindK8sWorkload && strings.HasPrefix(n.Name, "Secret/") && n.Signature == "v1" {
		return k8sSecretKind
	}
	return ""
}

// secretFiles maps each file of nodes holding a definition whose values may
// be secrets to the first such kind, in node order.
func secretFiles(nodes []core.SymbolNode) map[string]string {
	var out map[string]string
	for _, n := range nodes {
		k := secretValuedKind(n)
		if k == "" || n.FilePath == "" {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		if _, ok := out[n.FilePath]; !ok {
			out[n.FilePath] = k
		}
	}
	return out
}

// snippetWithheld is why def's source is not served: def's own values, or
// those of another definition in its file (nodes is the service's symbol
// set), may be secrets, so a window of that file could serve them. "" when
// neither holds.
func snippetWithheld(def core.SymbolNode, nodes []core.SymbolNode) string {
	k := secretValuedKind(def)
	for i := 0; k == "" && def.FilePath != "" && i < len(nodes); i++ {
		if nodes[i].FilePath == def.FilePath {
			k = secretValuedKind(nodes[i])
		}
	}
	if k == "" {
		return ""
	}
	return "the definition sits in a file whose values may be secrets (" + k + "): its source is not served"
}

// occurrenceTextWithheld is the references answer's reason for occurrence
// rows served without their line text, naming the kinds that made each file
// secret-valued (sorted); "" for none.
func occurrenceTextWithheld(kinds map[string]bool) string {
	if len(kinds) == 0 {
		return ""
	}
	ks := make([]string, 0, len(kinds))
	for k := range kinds {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return "occurrences in a file whose values may be secrets (" + strings.Join(ks, ", ") + ") carry no line text: their source is not served"
}
