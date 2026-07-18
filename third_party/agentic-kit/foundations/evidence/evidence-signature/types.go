// Package evidencesignature defines per-concern evidence signatures: declarative,
// diffable, testable specifications of what constitutes "enough evidence" for a
// given concern, decided at authoring time.
//
// # Core concept
//
// A ConcernSignature is a first-class artifact owned by the team responsible for
// the concern. It states, in CUE-contracted form, the archetype, scope, predicates,
// and (for absence archetypes) a planted-positive canary fixture. Signatures are
// version-controlled, liveness-tested in CI, and canary-tested in the evidence
// ladder.
//
// # Honest restriction — lexically decidable claims only
//
// Archetypes in this package are lexically decidable claims: a Tier-2 engine can
// evaluate them by searching text/ASTs for patterns. The following claim classes
// are NOT representable as ConcernSignature and will yield unreliable verdicts
// if forced into lexical predicates:
//
//   - Reachability: "function A can be called from function B" requires a full
//     call-graph analyzer (e.g. a code-intelligence index like sextant/fathom).
//   - Cross-service data flows: "service X sends PII to service Y" requires
//     distributed tracing or data-flow analysis.
//   - Semantic equivalence: "this implementation matches this spec" requires
//     a reasoning step beyond pattern matching.
//
// Decompose such obligations into lexical sub-obligations (e.g. "service X
// imports the PII-egress package") or route them to a dedicated structural
// analyzer. Do not encode them as ConcernSignature predicates.
//
// # Maintenance cost
//
// The three mandatory test classes (canary, liveness, manifest) impose ongoing
// maintenance: glob patterns drift as trees evolve; canary fixtures must stay
// planted; coverage manifests must be regenerated after structural changes.
// This cost is budgeted by the spec — it is the operating cost of the
// "earned ABSENT_PROVEN" guarantee and must not be wished away.
//
// # Concurrency
//
// All types are plain value types with no mutable state; safe for concurrent
// reads. Validate and PreconditionSatisfied are pure functions and may be called
// concurrently.
package evidencesignature

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Archetype classifies what a ConcernSignature is asserting.
//
// CUE mirror: #Archetype in schema.cue.
type Archetype string

const (
	// ArchetypeExistence asserts that at least one decisive hit exists.
	ArchetypeExistence Archetype = "existence"

	// ArchetypeAbsence asserts that zero hits exist within a fully-covered scope.
	// Requires a canary fixture (planted positive that Tier-2 must find).
	ArchetypeAbsence Archetype = "absence"

	// ArchetypeThreshold asserts a numeric comparison over the hit count.
	ArchetypeThreshold Archetype = "threshold"

	// ArchetypeContradicts asserts that findings contradict a claim.
	ArchetypeContradicts Archetype = "contradicts"
)

var validArchetypes = map[Archetype]struct{}{
	ArchetypeExistence:   {},
	ArchetypeAbsence:     {},
	ArchetypeThreshold:   {},
	ArchetypeContradicts: {},
}

// Scope declares the search perimeter: either file-system globs (on-disk trees)
// or registered source IDs (document stores). Exactly one must be set.
//
// CUE mirror: #Scope in schema.cue.
type Scope struct {
	// Globs is a list of file-system glob patterns (filepath.Match syntax).
	// Non-empty iff SourceIDs is empty.
	Globs []string `json:"globs,omitempty"`

	// SourceIDs is a list of registered source IDs (from source-registry).
	// Non-empty iff Globs is empty.
	SourceIDs []string `json:"source_ids,omitempty"`
}

// Validate returns an error if the Scope does not satisfy the exactly-one-set rule.
func (s Scope) Validate() error {
	hasGlobs := len(s.Globs) > 0
	hasSources := len(s.SourceIDs) > 0
	if hasGlobs && hasSources {
		return fmt.Errorf("evidencesignature: Scope: globs and source_ids are mutually exclusive; set exactly one")
	}
	if !hasGlobs && !hasSources {
		return fmt.Errorf("evidencesignature: Scope: exactly one of globs or source_ids must be set")
	}
	if hasGlobs {
		for i, g := range s.Globs {
			if g == "" {
				return fmt.Errorf("evidencesignature: Scope.Globs[%d] must not be empty", i)
			}
		}
	}
	if hasSources {
		for i, id := range s.SourceIDs {
			if id == "" {
				return fmt.Errorf("evidencesignature: Scope.SourceIDs[%d] must not be empty", i)
			}
		}
	}
	return nil
}

// IsGlobBased reports whether this scope is defined by file-system globs.
func (s Scope) IsGlobBased() bool { return len(s.Globs) > 0 }

// Predicate is a single proof predicate evidence must satisfy.
//
// CUE mirror: #Predicate in schema.cue.
type Predicate struct {
	// Kind names the predicate type (e.g. "regexp", "contains", "not_contains").
	// Consumer-defined; the signature package imposes no vocabulary constraint.
	Kind string `json:"kind"`
	// Arg is the predicate argument (e.g. the regexp pattern, the substring).
	Arg string `json:"arg"`
}

// Validate returns an error if the predicate is malformed.
func (p Predicate) Validate() error {
	if p.Kind == "" {
		return fmt.Errorf("evidencesignature: Predicate.Kind must not be empty")
	}
	if p.Arg == "" {
		return fmt.Errorf("evidencesignature: Predicate.Arg must not be empty")
	}
	return nil
}

// ThresholdOp is the comparison operator for a Threshold.
type ThresholdOp string

const (
	// ThresholdGTE requires the observed value to be greater than or equal
	// to the threshold.
	ThresholdGTE ThresholdOp = ">="
	// ThresholdLTE requires the observed value to be less than or equal to
	// the threshold.
	ThresholdLTE ThresholdOp = "<="
	// ThresholdEQ requires the observed value to equal the threshold.
	ThresholdEQ ThresholdOp = "=="
)

var validThresholdOps = map[ThresholdOp]struct{}{
	ThresholdGTE: {},
	ThresholdLTE: {},
	ThresholdEQ:  {},
}

// Threshold declares a numeric comparison over the count of findings.
//
// CUE mirror: #Threshold in schema.cue.
type Threshold struct {
	Op    ThresholdOp `json:"op"`
	Value float64     `json:"value"`
}

// Validate returns an error if the threshold is malformed.
func (t Threshold) Validate() error {
	if _, ok := validThresholdOps[t.Op]; !ok {
		return fmt.Errorf("evidencesignature: Threshold.Op %q must be one of >=|<=|==", string(t.Op))
	}
	if t.Value < 0 {
		return fmt.Errorf("evidencesignature: Threshold.Value must be >= 0, got %g", t.Value)
	}
	return nil
}

// Canary is a planted-positive fixture: a file at a known path containing a
// known pattern. Tier-2 engines must find it when searching under the scope.
// Mandatory for absence archetypes — their guarantee rests on Tier-2 being
// able to find what is present, which the canary verifies.
//
// CUE mirror: #Canary in schema.cue.
type Canary struct {
	// Path is the relative path (within the search root) where the canary is planted.
	Path string `json:"path"`
	// Pattern is the text or regexp that Tier-2 must find at that path.
	Pattern string `json:"pattern"`
}

// Validate returns an error if the canary is malformed.
func (c Canary) Validate() error {
	if c.Path == "" {
		return fmt.Errorf("evidencesignature: Canary.Path must not be empty")
	}
	if c.Pattern == "" {
		return fmt.Errorf("evidencesignature: Canary.Pattern must not be empty")
	}
	return nil
}

// ConcernSignature is the top-level contract for a single concern. It is the
// authoritative specification of "enough evidence" for that concern, authored
// and version-controlled by the responsible team.
//
// CUE mirror: #ConcernSignature in schema.cue.
type ConcernSignature struct {
	// ID is the unique, stable identifier for this concern. Must be non-empty.
	ID string `json:"id"`

	// Archetype classifies the assertion (existence|absence|threshold|contradicts).
	Archetype Archetype `json:"archetype"`

	// Scope declares the search perimeter (globs XOR source_ids).
	Scope Scope `json:"scope"`

	// Constraints optionally bounds the paths/sources over which search is valid.
	Constraints []string `json:"constraints,omitempty"`

	// Predicates is the ordered proof predicate list. Must be non-empty.
	Predicates []Predicate `json:"predicates"`

	// Threshold is required when Archetype == ArchetypeThreshold.
	Threshold *Threshold `json:"threshold,omitempty"`

	// RetrieverHints are opaque hints for Tier-1/Tier-2 retrieval tuning.
	RetrieverHints []string `json:"retriever_hints,omitempty"`

	// Canary is MANDATORY for absence archetypes.
	// A planted positive that Tier-2 must find — its absence means the search
	// engine would miss real hits, making ABSENT_PROVEN unearnable.
	Canary *Canary `json:"canary,omitempty"`
}

// Validate returns an error if the ConcernSignature is malformed.
//
// Rules:
//   - ID must be non-empty.
//   - Archetype must be one of the four recognised values.
//   - Scope must be valid (exactly one of globs/source_ids).
//   - Predicates must be non-empty; each predicate must be valid.
//   - If Archetype is ArchetypeAbsence, Canary must be set and valid.
//   - If Archetype is ArchetypeThreshold, Threshold must be set and valid.
//   - If Threshold is set, Archetype must be ArchetypeThreshold.
//   - If Canary is set, it must be valid.
func (sig ConcernSignature) Validate() error {
	if sig.ID == "" {
		return fmt.Errorf("evidencesignature: ConcernSignature.ID must not be empty")
	}
	if _, ok := validArchetypes[sig.Archetype]; !ok {
		return fmt.Errorf("evidencesignature: ConcernSignature.Archetype %q must be one of existence|absence|threshold|contradicts", string(sig.Archetype))
	}
	if err := sig.Scope.Validate(); err != nil {
		return err
	}
	if len(sig.Predicates) == 0 {
		return fmt.Errorf("evidencesignature: ConcernSignature %q: Predicates must not be empty", sig.ID)
	}
	for i, p := range sig.Predicates {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("evidencesignature: ConcernSignature %q Predicates[%d]: %w", sig.ID, i, err)
		}
	}
	if sig.Archetype == ArchetypeAbsence && sig.Canary == nil {
		return fmt.Errorf("evidencesignature: ConcernSignature %q: absence archetype requires a canary fixture", sig.ID)
	}
	if sig.Archetype == ArchetypeThreshold && sig.Threshold == nil {
		return fmt.Errorf("evidencesignature: ConcernSignature %q: threshold archetype requires a Threshold", sig.ID)
	}
	if sig.Threshold != nil {
		if err := sig.Threshold.Validate(); err != nil {
			return fmt.Errorf("evidencesignature: ConcernSignature %q Threshold: %w", sig.ID, err)
		}
	}
	if sig.Canary != nil {
		if err := sig.Canary.Validate(); err != nil {
			return fmt.Errorf("evidencesignature: ConcernSignature %q Canary: %w", sig.ID, err)
		}
	}
	return nil
}

// CoverageManifest records the completeness of a Tier-2 search over a scope.
// It is the authoritative artifact that supports an ABSENT_PROVEN verdict:
// every file walked must be accounted for, and any unexplained skip downgrades
// the verdict to ABSENT_UNVERIFIED.
//
// Plan D10: ABSENT_PROVEN requires a CoverageManifest with a complete account
// of every file in the scope at the given tree snapshot.
type CoverageManifest struct {
	// Files is the sorted list of relative file paths that were fully searched.
	Files []string `json:"files"`

	// TreeHash is the content hash (e.g. git tree hash) of the scoped tree at
	// the time of search. Ties the manifest to a specific snapshot.
	TreeHash string `json:"tree_hash"`

	// Tool identifies the search engine that produced this manifest
	// (e.g. "direct-investigator/pure", "direct-investigator/rg").
	Tool string `json:"tool"`

	// Flags records any relevant engine flags (e.g. "--no-ignore").
	Flags []string `json:"flags,omitempty"`

	// Skipped records files that were not fully searched, by category.
	// Any non-zero category that lacks explanation downgrades ABSENT_PROVEN.
	Skipped SkippedFiles `json:"skipped"`
}

// SkippedFiles categorizes files that were not searched, per D10.
// Every category has a documented reason that makes the skip explainable.
type SkippedFiles struct {
	// Ignored: excluded by .gitignore / .fathomignore / scope constraints.
	Ignored []string `json:"ignored,omitempty"`
	// Binary: detected as binary (non-text) content.
	Binary []string `json:"binary,omitempty"`
	// Oversize: exceeded the configured per-file size limit.
	Oversize []string `json:"oversize,omitempty"`
	// Symlink: symbolic links not followed.
	Symlink []string `json:"symlink,omitempty"`
	// LFSStub: Git LFS pointer files not materialized.
	LFSStub []string `json:"lfs_stub,omitempty"`
	// NonUTF8: files that could not be decoded as UTF-8.
	NonUTF8 []string `json:"non_utf8,omitempty"`
}

// AllSkipped returns all skipped file paths across all categories.
func (s SkippedFiles) AllSkipped() []string {
	var all []string
	all = append(all, s.Ignored...)
	all = append(all, s.Binary...)
	all = append(all, s.Oversize...)
	all = append(all, s.Symlink...)
	all = append(all, s.LFSStub...)
	all = append(all, s.NonUTF8...)
	return all
}

// UnexplainedSkips returns skipped files in categories that are NOT considered
// automatically-explained for ABSENT_PROVEN purposes.
// Currently only Binary, Oversize, LFSStub, and NonUTF8 are unexplained
// (they could contain the pattern in representable form). Ignored and Symlink
// are considered explained — they are explicitly excluded by the scope.
func (s SkippedFiles) UnexplainedSkips() []string {
	var out []string
	out = append(out, s.Binary...)
	out = append(out, s.Oversize...)
	out = append(out, s.LFSStub...)
	out = append(out, s.NonUTF8...)
	return out
}

// Validate returns an error if the manifest is structurally malformed.
func (m CoverageManifest) Validate() error {
	if m.Tool == "" {
		return fmt.Errorf("evidencesignature: CoverageManifest.Tool must not be empty")
	}
	return nil
}

// Complete reports whether the manifest represents a complete, unexplained-skip-free
// search. This is the necessary condition for ABSENT_PROVEN.
func (m CoverageManifest) Complete() bool {
	return len(m.Skipped.UnexplainedSkips()) == 0
}

// Finding is a single search result: a file path and matched line range.
// Decisive signals whether this finding is strong enough to count toward
// PreconditionSatisfied for an existence archetype.
type Finding struct {
	// Path is the relative file path where the match was found.
	Path string `json:"path"`
	// LineStart is the 1-based start line of the match (0 if unknown).
	LineStart int `json:"line_start,omitempty"`
	// LineEnd is the 1-based end line of the match (0 if unknown).
	LineEnd int `json:"line_end,omitempty"`
	// Decisive indicates this finding is a strong match (vs. a weak hint).
	Decisive bool `json:"decisive"`
}

// PreconditionSatisfied evaluates whether the findings and coverage satisfy the
// signature's archetype condition.
//
// Rules by archetype:
//
//   - existence: at least one finding is Decisive. Coverage is not required.
//   - absence: zero findings AND the manifest is Complete (no unexplained skips).
//     The canary fixture is not re-checked here — that is Tier-2's responsibility
//     before it reports findings as definitive. If Tier-2 found the canary and
//     found nothing else, it should call PreconditionSatisfied with an empty findings.
//   - threshold: the count of decisive findings satisfies sig.Threshold.
//   - contradicts: at least one decisive finding. (Same as existence — the
//     semantic distinction is in the concern definition, not here.)
//
// If sig.Validate() fails, PreconditionSatisfied returns false.
func PreconditionSatisfied(sig ConcernSignature, findings []Finding, cov CoverageManifest) bool {
	if err := sig.Validate(); err != nil {
		return false
	}

	switch sig.Archetype {
	case ArchetypeExistence, ArchetypeContradicts:
		for _, f := range findings {
			if f.Decisive {
				return true
			}
		}
		return false

	case ArchetypeAbsence:
		// Absence requires zero findings AND a complete manifest.
		if len(findings) > 0 {
			return false
		}
		return cov.Complete()

	case ArchetypeThreshold:
		if sig.Threshold == nil {
			return false
		}
		count := 0
		for _, f := range findings {
			if f.Decisive {
				count++
			}
		}
		return applyThreshold(sig.Threshold.Op, float64(count), sig.Threshold.Value)
	}

	return false
}

// applyThreshold evaluates count op threshold.
func applyThreshold(op ThresholdOp, count, threshold float64) bool {
	switch op {
	case ThresholdGTE:
		return count >= threshold
	case ThresholdLTE:
		return count <= threshold
	case ThresholdEQ:
		return count == threshold
	}
	return false
}

// LivenessCheck verifies that every glob in every signature matches at least
// one file in fsys, or is annotated expected_empty (by having the glob appear
// in the signature's Constraints slice with the prefix "expected_empty:").
//
// This is the mandatory CI liveness test: call it against the live tree to
// catch stale globs before they silently exclude relevant files from search.
//
// Non-glob scopes (SourceIDs) are not checked by this helper — they require
// a source-registry query, which is out of scope for a stdlib-only function.
//
// Returns a sorted list of glob patterns that matched no files and are not
// annotated expected_empty.
func LivenessCheck(fsys fs.FS, sigs []ConcernSignature) []string {
	expectedEmpty := make(map[string]struct{})
	for _, sig := range sigs {
		for _, c := range sig.Constraints {
			if strings.HasPrefix(c, "expected_empty:") {
				pat := strings.TrimPrefix(c, "expected_empty:")
				expectedEmpty[pat] = struct{}{}
			}
		}
	}

	var dead []string
	seen := make(map[string]struct{})
	for _, sig := range sigs {
		if !sig.Scope.IsGlobBased() {
			continue
		}
		for _, glob := range sig.Scope.Globs {
			if _, already := seen[glob]; already {
				continue
			}
			seen[glob] = struct{}{}
			if _, ok := expectedEmpty[glob]; ok {
				continue
			}
			matched, _ := fs.Glob(fsys, glob)
			if len(matched) == 0 {
				// Also try filepath.Match-style walk for recursive patterns.
				if !globMatchesAny(fsys, glob) {
					dead = append(dead, glob)
				}
			}
		}
	}
	sort.Strings(dead)
	return dead
}

// globMatchesAny walks fsys and returns true if any file path matches glob
// using filepath.Match. This handles patterns like "**/*.go" where fs.Glob
// may not apply full double-star semantics.
func globMatchesAny(fsys fs.FS, glob string) bool {
	found := false
	_ = fs.WalkDir(fsys, ".", func(path string, _ fs.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		matched, _ := filepath.Match(glob, path)
		if matched {
			found = true
		}
		// Also try matching just the base name for simple patterns.
		if !matched {
			matched, _ = regexp.MatchString(globToRegexp(glob), path)
			if matched {
				found = true
			}
		}
		return nil
	})
	return found
}

// globToRegexp converts a simple glob (with * and **) to a regexp.
func globToRegexp(glob string) string {
	var sb strings.Builder
	sb.WriteString("^")
	i := 0
	for i < len(glob) {
		if i+1 < len(glob) && glob[i] == '*' && glob[i+1] == '*' {
			sb.WriteString(".*")
			i += 2
			if i < len(glob) && glob[i] == '/' {
				i++
			}
			continue
		}
		switch glob[i] {
		case '*':
			sb.WriteString("[^/]*")
		case '?':
			sb.WriteString("[^/]")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|':
			sb.WriteByte('\\')
			sb.WriteByte(glob[i])
		default:
			sb.WriteByte(glob[i])
		}
		i++
	}
	sb.WriteString("$")
	return sb.String()
}
