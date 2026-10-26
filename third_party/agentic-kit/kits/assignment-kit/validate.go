package assignmentkit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrInvalid marks an invalid assignment or evaluation envelope.
var ErrInvalid = errors.New("assignmentkit: invalid input")
var idRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._/-]{0,127}$`)
var digestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalid}, a...)...)
}
func id(s string) bool     { return len(s) <= 128 && idRE.MatchString(s) }
func digest(s string) bool { return len(s) == 64 && digestRE.MatchString(s) }
func textValid(s string) bool {
	return s != "" && len(s) <= 2048 && utf8.ValidString(s) && strings.TrimSpace(s) != ""
}
func validTime(t time.Time) bool {
	return !t.IsZero() && t.Location() == time.UTC && t == t.Round(0) && t.Year() >= 1 && t.Year() <= 9999
}
func hash(domain string, v any) (string, error) {
	b, e := json.Marshal(struct {
		Domain string `json:"domain"`
		Value  any    `json:"value"`
	}{domain, v})
	if e != nil {
		return "", invalid("canonical JSON: %v", e)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
func uniqueRefs(xs []Reference) bool {
	seen := map[string]bool{}
	for _, r := range xs {
		if !id(r.ID) || !digest(r.RevisionDigest) || seen[r.ID] {
			return false
		}
		seen[r.ID] = true
	}
	return true
}
func uniqueIDs(xs []string) bool {
	seen := map[string]bool{}
	for _, x := range xs {
		if !id(x) || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}

// Validate checks the complete bounded assignment before fingerprinting or evaluation.
func Validate(a Assignment) error {
	if a.Version != 1 || !id(a.ID) || !textValid(a.Title) || !digest(a.PolicyDigest) || !digest(a.SubjectDigest) || !validTime(a.Deadline) {
		return invalid("invalid assignment header")
	}
	if len(a.IntentRefs) > 16 || !uniqueRefs(a.IntentRefs) {
		return invalid("invalid intent references")
	}
	if len(a.Goals) == 0 || len(a.Goals) > 64 || len(a.Concerns) > 64 || len(a.Budgets) == 0 || len(a.Budgets) > 16 || len(a.Rules) > 32 {
		return invalid("assignment bounds")
	}
	goals := map[string]Goal{}
	required := 0
	criteria := 0
	for _, g := range a.Goals {
		if !id(g.ID) || !textValid(g.Title) || (g.Kind != "required" && g.Kind != "stretch") || goals[g.ID].ID != "" || len(g.DependsOn) > 64 || !uniqueIDs(g.DependsOn) {
			return invalid("invalid goal %q", g.ID)
		}
		if e := g.Profile.Validate(); e != nil {
			return invalid("goal %q profile: %v", g.ID, e)
		}
		criteria += len(g.Profile.Criteria)
		if g.Kind == "required" {
			required++
		}
		by := map[string]fitness.Criterion{}
		for _, c := range g.Profile.Criteria {
			by[c.ID] = c
		}
		seen := map[string]bool{}
		for _, b := range g.Confidence {
			c, ok := by[b.CriterionID]
			if !ok || seen[b.CriterionID] || math.IsNaN(b.Minimum) || math.IsInf(b.Minimum, 0) || b.Minimum <= 0 || b.Minimum > 1 || !digest(b.BasisDigest) || !digest(b.SiteDigest) {
				return invalid("invalid confidence bar")
			}
			found := false
			for _, m := range c.AllowedMethods {
				if m == fitness.EvidenceMethodHeuristic {
					found = true
				}
			}
			if !found {
				return invalid("confidence bar requires heuristic method")
			}
			seen[b.CriterionID] = true
		}
		goals[g.ID] = g
	}
	if required == 0 || criteria > 256 {
		return invalid("required goal or criterion bound")
	}
	state := map[string]int{}
	var visit func(string) error
	visit = func(gid string) error {
		if state[gid] == 1 {
			return invalid("goal cycle")
		}
		if state[gid] == 2 {
			return nil
		}
		state[gid] = 1
		g := goals[gid]
		for _, d := range g.DependsOn {
			dep, ok := goals[d]
			if !ok || g.Kind == "required" && dep.Kind == "stretch" {
				return invalid("invalid dependency %q", d)
			}
			if e := visit(d); e != nil {
				return e
			}
		}
		state[gid] = 2
		return nil
	}
	for gid := range goals {
		if e := visit(gid); e != nil {
			return e
		}
	}
	seen := map[string]bool{}
	for _, c := range a.Concerns {
		if !id(c.ID) || seen[c.ID] || !textValid(c.Description) || (c.Severity != "blocking" && c.Severity != "advisory") {
			return invalid("invalid concern")
		}
		seen[c.ID] = true
	}
	seen = map[string]bool{}
	for _, b := range a.Budgets {
		if !id(b.ID) || seen[b.ID] || !id(b.Unit) || b.Limit <= 0 {
			return invalid("invalid budget")
		}
		seen[b.ID] = true
	}
	seen = map[string]bool{}
	for _, r := range a.Rules {
		if !id(r.ID) || seen[r.ID] || r.Threshold <= 0 || !id(r.Route) || !textValid(r.Reason) || (r.Counter != "attempts" && r.Counter != "no_progress" && r.Counter != "ambiguities" && r.Counter != "conflicts") || (r.Action != "escalate" && r.Action != "pivot" && r.Action != "stop") {
			return invalid("invalid rule")
		}
		seen[r.ID] = true
	}
	return nil
}
func canonicalAssignment(a Assignment) Assignment {
	a.IntentRefs = append([]Reference{}, a.IntentRefs...)
	sort.Slice(a.IntentRefs, func(i, j int) bool { return a.IntentRefs[i].ID < a.IntentRefs[j].ID })
	a.Goals = append([]Goal{}, a.Goals...)
	for i := range a.Goals {
		g := &a.Goals[i]
		g.DependsOn = append([]string{}, g.DependsOn...)
		sort.Strings(g.DependsOn)
		g.Confidence = append([]ConfidenceBar{}, g.Confidence...)
		sort.Slice(g.Confidence, func(i, j int) bool { return g.Confidence[i].CriterionID < g.Confidence[j].CriterionID })
		g.Profile.Criteria = append([]fitness.Criterion{}, g.Profile.Criteria...)
		for j := range g.Profile.Criteria {
			g.Profile.Criteria[j].AllowedMethods = append([]fitness.EvidenceMethod{}, g.Profile.Criteria[j].AllowedMethods...)
			sort.Slice(g.Profile.Criteria[j].AllowedMethods, func(x, y int) bool {
				return g.Profile.Criteria[j].AllowedMethods[x] < g.Profile.Criteria[j].AllowedMethods[y]
			})
		}
		sort.Slice(g.Profile.Criteria, func(i, j int) bool { return g.Profile.Criteria[i].ID < g.Profile.Criteria[j].ID })
	}
	sort.Slice(a.Goals, func(i, j int) bool { return a.Goals[i].ID < a.Goals[j].ID })
	a.Concerns = append([]Concern{}, a.Concerns...)
	sort.Slice(a.Concerns, func(i, j int) bool { return a.Concerns[i].ID < a.Concerns[j].ID })
	a.Budgets = append([]Budget{}, a.Budgets...)
	sort.Slice(a.Budgets, func(i, j int) bool { return a.Budgets[i].ID < a.Budgets[j].ID })
	a.Rules = append([]Rule{}, a.Rules...)
	sort.Slice(a.Rules, func(i, j int) bool { return a.Rules[i].ID < a.Rules[j].ID })
	return a
}

// Fingerprint returns the canonical assignment SHA-256 identity.
func Fingerprint(a Assignment) (string, error) {
	if e := Validate(a); e != nil {
		return "", e
	}
	return hash("agentic-kit/assignment/v1", canonicalAssignment(a))
}

// ObservationFingerprint binds a validated observation's complete canonical content.
func ObservationFingerprint(o fitness.Observation) (string, error) {
	if e := o.Validate(); e != nil {
		return "", invalid("observation: %v", e)
	}
	o.ArtifactDigests = append([]string{}, o.ArtifactDigests...)
	sort.Strings(o.ArtifactDigests)
	return hash("agentic-kit/assignment-observation/v1", o)
}
