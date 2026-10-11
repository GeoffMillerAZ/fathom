package receipt

// withheld.go — the withheld state's classifier (EVX-1, ADR-065, ADR-068
// decision 3): a file whose values may be secrets, or one a suppression
// hides, gets no excerpt and no digest. The secret rule moved here from
// internal/mcp/secret_withhold.go (SR-76), which now reads it from this
// package, so the read tools and the anchor core share one rule.

import (
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/scope"
)

// K8sSecretKind names a Kubernetes Secret in a withheld reason: the
// Kubernetes extractor's workload entity carries no kind of its own for it.
const K8sSecretKind = "Kubernetes Secret"

// SecretValuedKind is why n's values may be secrets — its kind, when a
// dialect declares that kind sensitive, or K8sSecretKind for a core
// Kubernetes Secret (a KindK8sWorkload "Secret/[namespace/]name" of
// apiVersion v1) — and "" otherwise.
func SecretValuedKind(n core.SymbolNode) string {
	if extractor.SensitiveKind(string(n.Kind)) {
		return string(n.Kind)
	}
	if n.Kind == core.KindK8sWorkload && strings.HasPrefix(n.Name, "Secret/") && n.Signature == "v1" {
		return K8sSecretKind
	}
	return ""
}

// SecretFiles maps each file of nodes holding a definition whose values may
// be secrets to the first such kind, in node order.
func SecretFiles(nodes []core.SymbolNode) map[string]string {
	var out map[string]string
	for _, n := range nodes {
		k := SecretValuedKind(n)
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

// withheldReason is why rel's bytes may not leave a receipt: the file holds
// a secret-valued definition (secret, SecretFiles' map), or a suppression
// active at now hides it for service. "" when neither holds.
func withheldReason(rel string, secret map[string]string, set *scope.Set, service string, now time.Time) string {
	if k := secret[rel]; k != "" {
		return "a file whose values may be secrets (" + k + "): no excerpt and no digest leave"
	}
	if sup, ok := set.Match(service, rel, now); ok {
		return "suppressed (" + sup.Reason + "): no excerpt and no digest leave"
	}
	return ""
}
