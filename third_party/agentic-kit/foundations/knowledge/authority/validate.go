package authority

import "fmt"

// validAuthorityTypes is the canonical set of recognised AuthorityType values.
var validAuthorityTypes = map[AuthorityType]struct{}{
	TypeStatute:        {},
	TypeRegulation:     {},
	TypeCase:           {},
	TypeConstitutional: {},
	TypeContractTerm:   {},
	TypePolicy:         {},
	TypeSecondary:      {},
}

// Validate returns an error if the AuthorityType is not one of the seven
// recognised values.
func (t AuthorityType) Validate() error {
	if _, ok := validAuthorityTypes[t]; !ok {
		return fmt.Errorf("authority: unknown AuthorityType %q; must be one of statute|regulation|case|constitutional|contract_term|policy|secondary", string(t))
	}
	return nil
}

// validWeights is the canonical set of recognised Weight values.
var validWeights = map[Weight]struct{}{
	WeightBinding:          {},
	WeightPersuasive:       {},
	WeightNonAuthoritative: {},
}

// Validate returns an error if the Weight is not one of the three recognised
// values.
func (w Weight) Validate() error {
	if _, ok := validWeights[w]; !ok {
		return fmt.Errorf("authority: unknown Weight %q; must be one of binding|persuasive|non_authoritative", string(w))
	}
	return nil
}

// validStatuses is the canonical set of recognised Status values.
var validStatuses = map[Status]struct{}{
	StatusGoodLaw:    {},
	StatusQuestioned: {},
	StatusSuperseded: {},
	StatusOverruled:  {},
	StatusUnknown:    {},
}

// Validate returns an error if the Status is not one of the five recognised
// values.
func (s Status) Validate() error {
	if _, ok := validStatuses[s]; !ok {
		return fmt.Errorf("authority: unknown Status %q; must be one of good_law|questioned|superseded|overruled|unknown", string(s))
	}
	return nil
}

// validTreatmentKinds is the canonical set of recognised TreatmentKind values.
var validTreatmentKinds = map[TreatmentKind]struct{}{
	KindFollows:       {},
	KindDistinguishes: {},
	KindCriticizes:    {},
	KindOverrules:     {},
	KindReaffirms:     {},
	KindQuestions:     {},
}

// Validate returns an error if the TreatmentKind is not one of the six
// recognised values.
func (k TreatmentKind) Validate() error {
	if _, ok := validTreatmentKinds[k]; !ok {
		return fmt.Errorf("authority: unknown TreatmentKind %q; must be one of follows|distinguishes|criticizes|overrules|reaffirms|questions", string(k))
	}
	return nil
}

// Validate returns an error if the Authority violates any invariant:
//   - ID must be non-empty.
//   - Citation must be non-empty.
//   - Type, Weight, Status must each be a recognised enum value.
//   - When both EffectiveFrom and EffectiveTo are non-zero, EffectiveFrom must
//     be ≤ EffectiveTo (the window must not be inverted).
//   - When Author.Kind is non-empty, Author.Validate() must pass.
func (a Authority) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("authority: Authority.ID must not be empty")
	}
	if a.Citation == "" {
		return fmt.Errorf("authority: Authority.Citation must not be empty")
	}
	if err := a.Type.Validate(); err != nil {
		return err
	}
	if err := a.Weight.Validate(); err != nil {
		return err
	}
	if err := a.Status.Validate(); err != nil {
		return err
	}
	if !a.EffectiveFrom.IsZero() && !a.EffectiveTo.IsZero() {
		if a.EffectiveFrom.After(a.EffectiveTo) {
			return fmt.Errorf("authority: EffectiveFrom %v is after EffectiveTo %v", a.EffectiveFrom, a.EffectiveTo)
		}
	}
	// Validate Author only when Kind is set (zero Author is valid — optional).
	if a.Author.Kind != "" {
		if err := a.Author.Validate(); err != nil {
			return fmt.Errorf("authority: Author invalid: %w", err)
		}
	}
	return nil
}

// Validate returns an error if the Treatment violates any invariant:
//   - FromID must be non-empty.
//   - ToID must be non-empty.
//   - Kind must be a recognised TreatmentKind.
func (t Treatment) Validate() error {
	if t.FromID == "" {
		return fmt.Errorf("authority: Treatment.FromID must not be empty")
	}
	if t.ToID == "" {
		return fmt.Errorf("authority: Treatment.ToID must not be empty")
	}
	return t.Kind.Validate()
}

// Validate returns an error if the Justification violates any invariant:
//   - ConclusionID must be non-empty.
//   - EvidenceRefs must contain at least one entry, each non-empty.
//   - AuthorityRefs must contain at least one entry, each non-empty.
func (j Justification) Validate() error {
	if j.ConclusionID == "" {
		return fmt.Errorf("authority: Justification.ConclusionID must not be empty")
	}
	if len(j.EvidenceRefs) == 0 {
		return fmt.Errorf("authority: Justification.EvidenceRefs must contain at least one evidence reference")
	}
	for i, ref := range j.EvidenceRefs {
		if ref == "" {
			return fmt.Errorf("authority: Justification.EvidenceRefs[%d] must not be empty", i)
		}
	}
	if len(j.AuthorityRefs) == 0 {
		return fmt.Errorf("authority: Justification.AuthorityRefs must contain at least one authority reference")
	}
	for i, ref := range j.AuthorityRefs {
		if ref == "" {
			return fmt.Errorf("authority: Justification.AuthorityRefs[%d] must not be empty", i)
		}
	}
	return nil
}

// Clone returns a deep copy of the Justification. The returned value does not
// share the underlying EvidenceRefs or AuthorityRefs slices with the original —
// mutating one cannot affect the other.
func (j Justification) Clone() Justification {
	out := Justification{ConclusionID: j.ConclusionID}
	if len(j.EvidenceRefs) > 0 {
		out.EvidenceRefs = make([]string, len(j.EvidenceRefs))
		copy(out.EvidenceRefs, j.EvidenceRefs)
	}
	if len(j.AuthorityRefs) > 0 {
		out.AuthorityRefs = make([]string, len(j.AuthorityRefs))
		copy(out.AuthorityRefs, j.AuthorityRefs)
	}
	return out
}
