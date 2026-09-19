package governedprosekit

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf8"
)

func digestJSON(v any) (string, error) {
	if !sourcedUTF8Valid(v) {
		return "", fmt.Errorf("%w: invalid UTF-8 input", ErrArtifact)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return ContentDigest(string(b)), nil
}

func validDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32 && s == strings.ToLower(s)
}

func validRef(r SourceRef) bool {
	return strings.TrimSpace(r.ID) != "" && strings.TrimSpace(r.Version) != "" && validDigest(r.Digest)
}

func cloneValue[T any](v T) (T, error) {
	var out T
	if !sourcedUTF8Valid(v) {
		return out, fmt.Errorf("%w: invalid UTF-8 input", ErrArtifact)
	}
	b, err := json.Marshal(v)
	if err == nil {
		err = json.Unmarshal(b, &out)
	}
	return out, err
}

func (p *SourcedPipeline) resolveFacts(ctx context.Context, req SourcedRequest) (map[string]Fact, map[SourceRef]SourceSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(req.Principal) == "" || strings.TrimSpace(req.Topic) == "" || len(req.Facts) == 0 || len(req.Facts) > p.config.Limits.MaxFacts {
		return nil, nil, fmt.Errorf("%w: principal, topic and bounded facts required", ErrEvidence)
	}
	b, err := json.Marshal(req)
	if err != nil || len(b) > p.config.Limits.MaxBytes {
		return nil, nil, fmt.Errorf("%w: input cannot be encoded within limit", ErrEvidence)
	}
	facts := make(map[string]Fact, len(req.Facts))
	sources := make(map[SourceRef]SourceSnapshot)
	sourceBytes := 0
	for _, f := range req.Facts {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		s := f.Span
		if _, exists := facts[s.ID]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate fact ID", ErrEvidence)
		}
		if s.Validate() != nil || math.IsNaN(s.Confidence) || s.Grade == nil || math.IsNaN(s.Grade.Confidence) || s.Author.Validate() != nil || s.OccurredAt.IsZero() || s.ObservedAt.Before(s.OccurredAt) {
			return nil, nil, fmt.Errorf("%w: fact provenance invalid", ErrEvidence)
		}
		if f.Derivation != nil {
			if f.Source != (SourceRef{}) || !s.Locator.IsZero() || s.RecordID != "" || s.RecordHash != "" || s.SourceID != "" || f.Historical {
				return nil, nil, fmt.Errorf("%w: derived facts inherit source identity through inputs", ErrEvidence)
			}
			if err := checkDerivation(f, facts); err != nil {
				return nil, nil, err
			}
			facts[s.ID] = f
			continue
		}
		if s.ValidateAsEvidence() != nil || !validRef(f.Source) || s.SourceID != f.Source.ID || s.RecordID != f.Source.Version || s.RecordHash != f.Source.Digest || s.Locator.Kind != "byte-range" {
			return nil, nil, fmt.Errorf("%w: exact versioned byte-range evidence required", ErrEvidence)
		}
		snapshot, found := sources[f.Source]
		if !found {
			snapshot, err = p.sources.Resolve(ctx, req.Principal, f.Source)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: resolve: %w", ErrEvidence, err)
			}
			now := p.now()
			if snapshot.Ref != f.Source || ContentDigest(snapshot.Content) != f.Source.Digest || snapshot.Revoked || !snapshot.RetainUntil.After(now) || snapshot.AcquiredAt.IsZero() || snapshot.AcquiredAt.After(now) || strings.TrimSpace(snapshot.AcquiredBy) == "" || (snapshot.Access != "public" && snapshot.Access != "restricted") || !strings.HasPrefix(snapshot.MediaType, "text/") || !sourcedUTF8Valid(snapshot) || len(snapshot.Content) > p.config.Limits.MaxBytes {
				return nil, nil, fmt.Errorf("%w: source integrity/access/retention invalid", ErrEvidence)
			}
			sourceBytes += len(snapshot.Content)
			if sourceBytes > p.config.Limits.MaxBytes {
				return nil, nil, fmt.Errorf("%w: aggregate source byte limit", ErrEvidence)
			}
			sources[f.Source] = snapshot
		}
		if !f.Historical {
			current, err := p.sources.Current(ctx, req.Principal, f.Source.ID)
			if err != nil || current != f.Source {
				return nil, nil, fmt.Errorf("%w: source is not current", ErrEvidence)
			}
		}
		start, end := s.Locator.Start, s.Locator.End
		if start < 0 || end < start || end >= len(snapshot.Content) || !utf8.ValidString(snapshot.Content[:start]) || !utf8.ValidString(snapshot.Content[start:end+1]) || snapshot.Content[start:end+1] != s.Value {
			return nil, nil, fmt.Errorf("%w: excerpt differs from exact source bytes", ErrEvidence)
		}
		facts[s.ID] = f
	}
	return facts, sources, ctx.Err()
}

var decimalRational = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]+)?|[0-9]+/[1-9][0-9]*)$`)

func boundedRational(s string) (*big.Rat, bool) {
	// big.Rat also accepts exponents and base prefixes; a short exponent can
	// allocate enormous integers. v1 deliberately admits bounded base-10 only.
	if len(s) > 128 || !decimalRational.MatchString(s) {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

func checkDerivation(f Fact, facts map[string]Fact) error {
	d := f.Derivation
	if d.Version != "arithmetic/v1" || len(d.Inputs) < 2 || len(d.Inputs) > 32 || (d.Operation != "sum" && d.Operation != "difference" && d.Operation != "ratio") || (d.Operation != "sum" && len(d.Inputs) != 2) {
		return fmt.Errorf("%w: unknown or unbounded derivation", ErrEvidence)
	}
	seen := make(map[string]bool)
	var result *big.Rat
	var unit string
	for i, id := range d.Inputs {
		input, ok := facts[id]
		if !ok || seen[id] || len(input.Span.Value) > 128 {
			return fmt.Errorf("%w: derivation inputs must be unique prior facts", ErrEvidence)
		}
		seen[id] = true
		n, ok := boundedRational(input.Span.Value)
		if !ok {
			return fmt.Errorf("%w: arithmetic requires exact numeric facts", ErrEvidence)
		}
		grade, inputGrade := f.Span.EffectiveGrade(), input.Span.EffectiveGrade()
		if grade.Tier.StrongerThan(inputGrade.Tier) || grade.Confidence > inputGrade.Confidence || f.Span.ObservedAt.Before(input.Span.ObservedAt) {
			return fmt.Errorf("%w: derivation exceeds input grade or predates observation", ErrEvidence)
		}
		if i == 0 {
			result, unit = n, input.Unit
			continue
		}
		if input.Unit != unit {
			return fmt.Errorf("%w: incompatible units", ErrEvidence)
		}
		switch d.Operation {
		case "sum":
			result.Add(result, n)
		case "difference":
			result.Sub(result, n)
		case "ratio":
			if n.Sign() == 0 {
				return fmt.Errorf("%w: division by zero", ErrEvidence)
			}
			result.Quo(result, n)
		}
	}
	if d.Operation == "ratio" {
		unit = "ratio"
	}
	if len(f.Span.Value) > 128 {
		return fmt.Errorf("%w: derived value too large", ErrEvidence)
	}
	want, ok := boundedRational(f.Span.Value)
	if !ok || want.Cmp(result) != 0 || f.Unit != unit {
		return fmt.Errorf("%w: derived value or units differ", ErrEvidence)
	}
	return nil
}

// ExactSupportReviewer accepts only literal, single-fact assertions with no
// unresolved current contradiction. This is a deliberately conservative baseline,
// not a natural-language entailment judge. Historical assertions need a richer
// reviewer. NonFactual contains caller-reviewed exact headings, never model rules.
// Concurrency: safe if callers do not mutate NonFactual during use.
type ExactSupportReviewer struct {
	NonFactual []string
}

// Review checks a literal assertion or an explicitly pre-reviewed exact heading.
func (r ExactSupportReviewer) Review(ctx context.Context, in SupportReview) (SupportDecision, error) {
	out := SupportDecision{Status: "unsupported", Reviewer: "exact-support", Method: "exact-quote/v1", Reason: "no exact uncontradicted support"}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if in.Claim == nil {
		for _, text := range r.NonFactual {
			if in.Unit.Text == text && in.Unit.NonFactualReason != "" {
				out.Status, out.Reason = "supported", "exact caller-reviewed non-factual text"
			}
		}
		return out, nil
	}
	if len(in.Claim.FactIDs) != 1 || len(in.Claim.Contradictions) != 0 {
		return out, nil
	}
	facts := make(map[string]Fact, len(in.Facts))
	for _, fact := range in.Facts {
		facts[fact.Span.ID] = fact
	}
	if historicalDependency(in.Claim.FactIDs[0], facts, make(map[string]uint8)) {
		return out, nil
	}
	for _, fact := range in.Facts {
		if fact.Span.ID != in.Claim.FactIDs[0] || fact.Span.Value != in.Claim.Assertion || fact.Historical {
			continue
		}
		for _, other := range in.Facts {
			if other.Span.ID != fact.Span.ID && !other.Historical && other.Span.EntityID == fact.Span.EntityID && other.Span.Field == fact.Span.Field && other.Span.Value != fact.Span.Value {
				out.Status = "inconclusive"
				return out, nil
			}
		}
		out.Status, out.Reason = "supported", "exact checked fact quotation; not source truth"
		return out, nil
	}
	return out, nil
}

func historicalDependency(id string, facts map[string]Fact, state map[string]uint8) bool {
	if state[id] != 0 {
		return state[id] != 2 // visiting/cyclic or known historical; 2 = checked current
	}
	fact, ok := facts[id]
	if !ok || fact.Historical {
		state[id] = 3
		return true
	}
	state[id] = 1
	if fact.Derivation != nil {
		for _, input := range fact.Derivation.Inputs {
			if historicalDependency(input, facts, state) {
				state[id] = 3
				return true
			}
		}
	}
	state[id] = 2
	return false
}
