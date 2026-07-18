package llmegress

import (
	"context"
	"fmt"

	prellmdlpgate "github.com/geoffmilleraz/agentic-kit/foundations/security/pre-llm-dlp-gate"
)

// GateFromDLP returns a Transformer that redacts outbound text through g's
// compiled RedactPolicy (prellmdlpgate.Gate.Scan): every rule match is
// replaced with its configured label, and the redacted text is what leaves
// the process. Findings are not surfaced here — egress wants the rewritten
// text; callers who need findings should Scan separately.
//
// Honesty contract, inherited from pre-llm-dlp-gate: this is best-effort
// secret-SHAPE redaction, not DLP-as-guarantee. Encoded secrets, secrets
// split across fields, and novel formats pass straight through.
//
// Fail closed: a nil g yields a Transformer that errors on every call (and
// therefore aborts every gated call) rather than one that silently passes
// text through.
//
// The returned Transformer is safe for concurrent use (Gate is immutable
// after construction). It ignores ctx: scanning is synchronous and
// in-process.
func GateFromDLP(g *prellmdlpgate.Gate) Transformer {
	return TransformerFunc(func(_ context.Context, text string) (string, error) {
		if g == nil {
			return "", fmt.Errorf("llmegress: GateFromDLP: nil gate (failing closed)")
		}
		out, _ := g.Scan(text)
		return out, nil
	})
}
