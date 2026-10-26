package governedprosekit

import (
	"fmt"
	"sort"
	"strings"
)

const sessionDigestVersion = "session-prose/v1"

func sessErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrSession, fmt.Sprintf(format, args...))
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

func strs(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func dedupSorted(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func validGoal(g Goal, maxOrigin ...string) error {
	if blank(g.ID) || blank(g.Statement) || blank(g.Scope) || blank(g.Method) || blank(g.Acceptance) {
		return sessErr("goal %q requires id, statement, scope, method and acceptance", g.ID)
	}
	if g.Class != GoalRequired && g.Class != GoalAdvisory {
		return sessErr("goal %q class must be required or advisory", g.ID)
	}
	ok := false
	for _, o := range maxOrigin {
		ok = ok || g.Origin == o
	}
	if !ok {
		return sessErr("goal %q has origin %q not permitted here", g.ID, g.Origin)
	}
	return nil
}

func profileDigest(p DocumentTypeProfile) (string, error) {
	p.Digest = ""
	return digestJSON(p)
}

// NewDocumentTypeProfile validates a profile, deep-copies it and seals its digest.
// It does not resolve catalogs: a Kind "catalog" origin must carry the stable key,
// complete definition digest and layer evidence the caller resolved. Profiles need
// at least one section, one required section and one required goal, including
// freeform profiles, so coverage cannot be defined by what a generator produced.
func NewDocumentTypeProfile(in DocumentTypeProfile) (DocumentTypeProfile, error) {
	p, err := cloneValue(in)
	if err != nil {
		return DocumentTypeProfile{}, sessErr("profile encoding: %v", err)
	}
	p.Origin.LayerEvidence, p.Sections, p.Goals = strs(p.Origin.LayerEvidence), append([]SectionSpec{}, p.Sections...), append([]Goal{}, p.Goals...)
	if blank(p.Key) || blank(p.Version) {
		return DocumentTypeProfile{}, sessErr("profile key and version required")
	}
	switch p.Origin.Kind {
	case "supplied":
		if p.Origin.CatalogKey != "" || p.Origin.DefinitionDigest != "" || len(p.Origin.LayerEvidence) != 0 {
			return DocumentTypeProfile{}, sessErr("supplied profile cannot claim catalog provenance")
		}
	case "catalog":
		if blank(p.Origin.CatalogKey) || !validDigest(p.Origin.DefinitionDigest) || len(p.Origin.LayerEvidence) == 0 {
			return DocumentTypeProfile{}, sessErr("catalog profile requires stable key, definition digest and layer evidence")
		}
		for _, e := range p.Origin.LayerEvidence {
			if blank(e) {
				return DocumentTypeProfile{}, sessErr("blank layer evidence")
			}
		}
	default:
		return DocumentTypeProfile{}, sessErr("profile origin kind must be supplied or catalog")
	}
	if blank(p.Defaults.Audience) || blank(p.Defaults.Voice) || blank(p.Defaults.TechnicalLevel) {
		return DocumentTypeProfile{}, sessErr("profile defaults require audience, voice and technical level")
	}
	sections, required := map[string]bool{}, false
	for i := range p.Sections {
		s := &p.Sections[i]
		s.FactFamilies = strs(s.FactFamilies)
		if blank(s.ID) || blank(s.Title) || sections[s.ID] {
			return DocumentTypeProfile{}, sessErr("section ids must be unique and titled")
		}
		sections[s.ID], required = true, required || s.Required
	}
	if len(sections) == 0 || !required {
		return DocumentTypeProfile{}, sessErr("profile requires a required section")
	}
	goals, hasRequired := map[string]bool{}, false
	for _, g := range p.Goals {
		if err := validGoal(g, OriginProfile); err != nil {
			return DocumentTypeProfile{}, err
		}
		if goals[g.ID] || (g.Scope != ScopeDocument && !sections[g.Scope]) {
			return DocumentTypeProfile{}, sessErr("goal %q duplicate or scoped to unknown section", g.ID)
		}
		goals[g.ID], hasRequired = true, hasRequired || g.Class == GoalRequired
	}
	if !hasRequired || len(p.Goals) > DefaultDocumentLimits().MaxGoals {
		return DocumentTypeProfile{}, sessErr("profile requires bounded goals including a required goal")
	}
	p.DigestVersion = sessionDigestVersion
	if p.Digest, err = profileDigest(p); err != nil {
		return DocumentTypeProfile{}, err
	}
	return p, nil
}

// VerifyProfile recomputes the profile digest and refuses any difference.
func VerifyProfile(p DocumentTypeProfile) error {
	d, err := profileDigest(p)
	if err != nil || d != p.Digest || p.DigestVersion != sessionDigestVersion {
		return sessErr("profile digest mismatch")
	}
	return nil
}

func sessionDigest(s DocumentSession) (string, error) {
	s.Digest = ""
	return digestJSON(s)
}

// VerifySession recomputes the session digest and refuses any difference.
func VerifySession(s DocumentSession) error {
	d, err := sessionDigest(s)
	if err != nil || d != s.Digest {
		return sessErr("session digest mismatch")
	}
	return nil
}

// NewDocumentSession creates an immutable session revision. Profile goals are hard
// floors: a session may add goals (origin session or task), or tighten an advisory
// profile goal to required, but never remove, downgrade or redefine one. Audience,
// voice and level default from the profile with provenance recorded in
// DefaultsFrom. Expected goal/section IDs are caller-owned: each must resolve to an
// effective required goal / known section, and profile required goals and sections
// are always added to them. With parent non-nil, the result is the next revision of
// the same session ID, so any goal/profile change yields a new digest even when the
// output would be byte-identical.
func NewDocumentSession(profile DocumentTypeProfile, in SessionInput, parent *DocumentSession) (DocumentSession, error) {
	if err := VerifyProfile(profile); err != nil {
		return DocumentSession{}, err
	}
	owned, err := cloneValue(in)
	if err != nil {
		return DocumentSession{}, sessErr("session encoding: %v", err)
	}
	if blank(owned.ID) || blank(owned.Purpose) {
		return DocumentSession{}, sessErr("session id and purpose required")
	}
	s := DocumentSession{
		ID: owned.ID, Revision: 1, Purpose: owned.Purpose, Intent: owned.Intent, DefaultsFrom: map[string]string{},
		Profile: ProfileRef{Key: profile.Key, Version: profile.Version, Digest: profile.Digest}, Constraints: strs(owned.Constraints),
	}
	if parent != nil {
		if err := VerifySession(*parent); err != nil || parent.ID != s.ID {
			return DocumentSession{}, sessErr("parent session invalid or different ID")
		}
		s.Revision, s.Parent = parent.Revision+1, parent.Digest
	}
	pick := func(field, supplied, def string) string {
		if !blank(supplied) {
			s.DefaultsFrom[field] = "session"
			return supplied
		}
		s.DefaultsFrom[field] = "profile:" + profile.Key
		return def
	}
	s.Presentation = PresentationDefaults{
		Audience:       pick("audience", owned.Supplied.Audience, profile.Defaults.Audience),
		Voice:          pick("voice", owned.Supplied.Voice, profile.Defaults.Voice),
		TechnicalLevel: pick("technical_level", owned.Supplied.TechnicalLevel, profile.Defaults.TechnicalLevel),
	}
	sections := map[string]bool{}
	for _, sec := range profile.Sections {
		sections[sec.ID] = true
	}
	for _, id := range owned.ExpectedSectionIDs {
		if blank(id) || (!sections[id] && !profile.PermitExtra) {
			return DocumentSession{}, sessErr("expected section %q is not in the profile and extras are not permitted", id)
		}
		sections[id] = true
	}
	merged := map[string]Goal{}
	for _, g := range profile.Goals {
		merged[g.ID] = g
	}
	for _, g := range owned.Goals {
		if err := validGoal(g, OriginSession, OriginTask); err != nil {
			return DocumentSession{}, err
		}
		if g.Scope != ScopeDocument && !sections[g.Scope] {
			return DocumentSession{}, sessErr("goal %q scoped to unknown section %q", g.ID, g.Scope)
		}
		if base, exists := merged[g.ID]; exists {
			if base.Origin != OriginProfile || base.Class != GoalAdvisory || g.Class != GoalRequired || base.Scope != g.Scope || base.Statement != g.Statement {
				return DocumentSession{}, sessErr("goal %q cannot redefine, remove or downgrade a profile goal", g.ID)
			}
		}
		merged[g.ID] = g
	}
	if len(merged) > DefaultDocumentLimits().MaxGoals {
		return DocumentSession{}, sessErr("too many goals")
	}
	for _, g := range merged {
		s.Goals = append(s.Goals, g)
	}
	sort.Slice(s.Goals, func(i, j int) bool { return s.Goals[i].ID < s.Goals[j].ID })
	expectedGoals := append([]string{}, owned.ExpectedGoalIDs...)
	for _, g := range s.Goals {
		if g.Class == GoalRequired && g.Origin == OriginProfile {
			expectedGoals = append(expectedGoals, g.ID)
		}
	}
	for _, id := range expectedGoals {
		if g, ok := merged[id]; !ok || g.Class != GoalRequired {
			return DocumentSession{}, sessErr("expected goal %q is missing or not required", id)
		}
	}
	expectedSections := append([]string{}, owned.ExpectedSectionIDs...)
	for _, sec := range profile.Sections {
		if sec.Required {
			expectedSections = append(expectedSections, sec.ID)
		}
	}
	s.ExpectedGoalIDs, s.ExpectedSectionIDs = dedupSorted(expectedGoals), dedupSorted(expectedSections)
	if s.Digest, err = sessionDigest(s); err != nil {
		return DocumentSession{}, err
	}
	return s, nil
}
