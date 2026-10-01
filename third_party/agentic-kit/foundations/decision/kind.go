package decision

import "fmt"

// Kind selects which shape of fast-decision question a Question asks. The
// three kinds are generic question shapes, not any provider's vocabulary:
// an adapter maps each onto its own wire (see the adapter packages
// foundations/decision/llm and foundations/decision/typesafe).
//
// Decode a Kind from text (JSON, configuration, a stored row) with
// ParseKind or through UnmarshalText, which also accept the alias "noul"
// for KindPredicate.
//
// CUE mirror: #Kind in schema.cue.
type Kind string

// The three question kinds.
const (
	// KindChoice asks the caller to pick among named Options and reports a
	// probability distribution over them.
	KindChoice Kind = "choice"
	// KindScore asks for a value on a continuous [Min, Max] range, or a
	// position among ordered Levels.
	KindScore Kind = "score"
	// KindPredicate asks a yes/no question and reports the probability the
	// statement is true. Its decode alias is "noul" (see ParseKind).
	KindPredicate Kind = "predicate"
)

// kindNoulAlias is the decode alias for KindPredicate: the vendor term a
// consumer's configuration, stored rows or a Kind built without ParseKind
// may still carry.
const kindNoulAlias Kind = "noul"

// ParseKind returns the Kind named by s: "choice", "score" or "predicate",
// or the alias "noul", which decodes to KindPredicate. Any other value,
// including the empty string, is an error that names s.
func ParseKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case KindChoice, KindScore, KindPredicate:
		return k, nil
	case kindNoulAlias:
		return KindPredicate, nil
	default:
		return "", fmt.Errorf("decision: unknown Kind %q (want choice, score or predicate)", s)
	}
}

// UnmarshalText decodes text through ParseKind, so JSON, configuration and
// stored rows that say "noul" decode to KindPredicate. Empty text decodes
// to the zero Kind, so an unset field survives a round trip; Validate still
// rejects a Question with no Kind.
func (k *Kind) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*k = ""
		return nil
	}
	parsed, err := ParseKind(string(text))
	if err != nil {
		return err
	}
	*k = parsed
	return nil
}

// canonicalKind returns the canonical Kind for k: the alias "noul" becomes
// KindPredicate and every other value is returned unchanged. Validate and the
// core switch on it, and CanonicalKind exports it for the adapters, so a Kind
// built without ParseKind is still answered as its canonical kind.
func canonicalKind(k Kind) Kind {
	if k == kindNoulAlias {
		return KindPredicate
	}
	return k
}
