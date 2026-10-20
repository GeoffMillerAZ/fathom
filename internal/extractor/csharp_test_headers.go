package extractor

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const csharpTestSourceLimit = 16 << 20
const csharpTestHeaderLimit = 64 << 10

// CSharpMethodHeaders reads complete, bounded method headers for an explicit
// source-bound test-policy check. Normal indexed/display signatures remain
// capped by the dialect. These are syntax facts, not resolved NUnit attributes.
// A source over 16MiB returns an error. Normalized UTF-8 headers over 64KiB or
// clipped headers are omitted, never treated as complete policy.
func CSharpMethodHeaders(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	if len(src) > csharpTestSourceLimit {
		return nil, fmt.Errorf("C# test-policy source exceeds 16MiB")
	}
	registry, err := tsRegistry()
	if err != nil {
		return nil, err
	}
	base := registry["csharp"]
	if base == nil {
		return nil, fmt.Errorf("C# test-policy dialect unavailable")
	}
	// Clone the per-extractor rule values. Shared registry/query/parser policy
	// must not acquire a larger display limit from this read-side operation.
	cloneVariant := func(v tsVariant) tsVariant {
		rules := *v.rules
		rules.sigMax = csharpTestHeaderLimit + 1
		v.rules, v.leaseOwner = &rules, ""
		return v
	}
	spec := *base
	spec.defaultVariant = cloneVariant(base.defaultVariant)
	spec.variantForExt = make(map[string]tsVariant, len(base.variantForExt))
	for ext, variant := range base.variantForExt {
		spec.variantForExt[ext] = cloneVariant(variant)
	}
	e := newTSExtractorFromSpec(&spec)
	defer e.Close()
	nodes, err := e.Extract(service, relPath, src)
	if err != nil {
		return nil, err
	}
	out := make([]core.SymbolNode, 0)
	for _, n := range nodes {
		if n.Kind == core.KindCSharpMethod && len(n.Signature) <= csharpTestHeaderLimit && !strings.HasSuffix(n.Signature, "…") {
			out = append(out, n)
		}
	}
	return out, nil
}
