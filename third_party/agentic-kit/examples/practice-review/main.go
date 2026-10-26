// Command practice-review checks a bounded synthetic review fixture offline.
// It is safe for concurrent use because each invocation owns its state.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	catalogkit "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/catalog-kit"
)

const maxFile = 1 << 20

var keyRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._/-]{0,127}$`)

type Practice struct {
	StableKey    string   `json:"stable_key"`
	Version      string   `json:"version"`
	Path         string   `json:"path"`
	Title        string   `json:"title"`
	FailureMode  string   `json:"failure_mode"`
	Benefit      string   `json:"benefit"`
	Remedy       string   `json:"remedy"`
	Sources      []string `json:"sources"`
	NegativeCase string   `json:"negative_case"`
	Kind         string   `json:"kind"`
	Definition   string   `json:"definition"`
	MaxSteps     *int     `json:"max_steps,omitempty"`
}
type CatalogInput struct {
	Spec      catalogkit.Spec `json:"spec"`
	Practices []Practice      `json:"practices"`
}
type Pin struct {
	StableKey        string `json:"stable_key"`
	Version          string `json:"version"`
	DefinitionDigest string `json:"definition_digest"`
}
type ExpectedSubject struct {
	ID             string   `json:"id"`
	Revision       string   `json:"revision"`
	AllowedActions []string `json:"allowed_actions"`
	Criteria       []string `json:"criteria"`
}
type Scope struct {
	Version  int               `json:"version"`
	Criteria []Pin             `json:"criteria"`
	Subjects []ExpectedSubject `json:"subjects"`
}
type Evidence struct {
	SubjectID  string    `json:"subject_id"`
	Source     string    `json:"source"`
	Revision   string    `json:"revision"`
	Text       string    `json:"text"`
	Digest     string    `json:"digest"`
	ObservedAt time.Time `json:"observed_at"`
	ValidUntil time.Time `json:"valid_until"`
}
type Work struct {
	Steps *int `json:"steps"`
}
type Check struct {
	StableKey        string    `json:"stable_key"`
	Evidence         Evidence  `json:"evidence"`
	Work             *Work     `json:"work,omitempty"`
	RequestedActions *[]string `json:"requested_actions,omitempty"`
}
type Subject struct {
	ID       string  `json:"id"`
	Revision string  `json:"revision"`
	Checks   []Check `json:"checks"`
}
type Subjects struct {
	Version  int       `json:"version"`
	Subjects []Subject `json:"subjects"`
}
type BindingReceipt struct {
	SubjectID        string `json:"subject_id"`
	StableKey        string `json:"stable_key"`
	Version          string `json:"version"`
	Path             string `json:"path"`
	DefinitionDigest string `json:"definition_digest"`
	NodeDigest       string `json:"node_digest"`
	PreparedDigest   string `json:"prepared_digest"`
}
type SubjectReceipt struct {
	ID         string             `json:"id"`
	Revision   string             `json:"revision"`
	Assessment fitness.Assessment `json:"assessment"`
}
type Receipt struct {
	Scope         string            `json:"scope"`
	Limits        string            `json:"limits"`
	InputDigests  map[string]string `json:"input_digests"`
	CatalogDigest string            `json:"catalog_digest"`
	Bindings      []BindingReceipt  `json:"bindings"`
	CoverageGaps  []string          `json:"coverage_gaps"`
	Subjects      []SubjectReceipt  `json:"subjects"`
	Status        string            `json:"status"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// strictJSON bounds structural recursion and rejects duplicate keys, nulls,
// unknown fields, malformed UTF-8 and trailing values before typed decoding.
func strictJSON(data []byte, out any) error {
	if !utf8.Valid(data) {
		return errors.New("invalid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if err := scan(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	return nil
}
func scan(d *json.Decoder, depth int) error {
	if depth > 24 {
		return errors.New("JSON nesting exceeds 24")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	if t == nil {
		return errors.New("null is unsupported")
	}
	delim, ok := t.(json.Delim)
	if !ok {
		if s, ok := t.(string); ok && strings.ContainsRune(s, utf8.RuneError) {
			return errors.New("invalid Unicode escape")
		}
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok || seen[s] || strings.ContainsRune(s, utf8.RuneError) {
				return fmt.Errorf("duplicate or invalid key %q", key)
			}
			seen[s] = true
			if err := scan(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := scan(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected delimiter")
	}
	_, err = d.Token()
	return err
}
func read(path string, out any) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxFile {
		return "", errors.New("input exceeds 1 MiB")
	}
	if err = strictJSON(b, out); err != nil {
		return "", err
	}
	return digest(b), nil
}
func nonempty(s string) bool { return strings.TrimSpace(s) != "" && len(s) <= 2048 }
func keyOK(s string) bool    { return keyRE.MatchString(s) }
func unique(xs []string) bool {
	m := map[string]bool{}
	for _, x := range xs {
		if !nonempty(x) || m[x] {
			return false
		}
		m[x] = true
	}
	return true
}
func timeUTC(t time.Time) bool {
	return !t.IsZero() && t.Location() == time.UTC && t.Nanosecond() == 0 && t.Year() >= 1 && t.Year() <= 9999
}

func canonicalNodes(nodes []catalog.AuthoredNode) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Slug < nodes[j].Slug })
	for i := range nodes {
		canonicalNodes(nodes[i].Children)
	}
}

// semanticDefinition binds every practice field that can convey meaning or
// drive a check. Path is location only; Sources are a set.
func semanticDefinition(p Practice) ([]byte, error) {
	sources := append([]string(nil), p.Sources...)
	sort.Strings(sources)
	maxSteps := 0
	if p.MaxSteps != nil {
		maxSteps = *p.MaxSteps
	}
	return json.Marshal(struct {
		StableKey    string   `json:"stable_key"`
		Version      string   `json:"version"`
		Title        string   `json:"title"`
		FailureMode  string   `json:"failure_mode"`
		Benefit      string   `json:"benefit"`
		Remedy       string   `json:"remedy"`
		Sources      []string `json:"sources"`
		NegativeCase string   `json:"negative_case"`
		Kind         string   `json:"kind"`
		Definition   string   `json:"definition"`
		MaxSteps     int      `json:"max_steps"`
	}{p.StableKey, p.Version, p.Title, p.FailureMode, p.Benefit, p.Remedy, sources, p.NegativeCase, p.Kind, p.Definition, maxSteps})
}

func canonicalSubject(s Subject) ([]byte, error) {
	copy := s
	copy.Checks = append([]Check(nil), s.Checks...)
	sort.Slice(copy.Checks, func(i, j int) bool { return copy.Checks[i].StableKey < copy.Checks[j].StableKey })
	for i := range copy.Checks {
		if copy.Checks[i].RequestedActions != nil {
			a := append([]string(nil), (*copy.Checks[i].RequestedActions)...)
			sort.Strings(a)
			copy.Checks[i].RequestedActions = &a
		}
	}
	return json.Marshal(copy)
}

func checkPacket(id, revision string, allowed []string, ch Check) (string, error) {
	a := append([]string(nil), allowed...)
	sort.Strings(a)
	copy := ch
	if ch.RequestedActions != nil {
		actions := append([]string(nil), (*ch.RequestedActions)...)
		sort.Strings(actions)
		copy.RequestedActions = &actions
	}
	b, err := json.Marshal(struct {
		SubjectID      string   `json:"subject_id"`
		Revision       string   `json:"revision"`
		AllowedActions []string `json:"allowed_actions"`
		Check          Check    `json:"check"`
	}{id, revision, a, copy})
	return string(b), err
}

func evaluate(c CatalogInput, scope Scope, subjects Subjects, now time.Time, hashes map[string]string) (Receipt, int, error) {
	r := Receipt{Scope: "synthetic fixture/configuration evidence only", Limits: "No source authenticity, actual effect authorization, Review Kit exchange, or production readiness claim", InputDigests: hashes, Bindings: []BindingReceipt{}, CoverageGaps: []string{}, Subjects: []SubjectReceipt{}, Status: "unknown"}
	if scope.Version != 1 || subjects.Version != 1 || len(scope.Subjects) == 0 || len(scope.Subjects) > 32 || len(subjects.Subjects) == 0 || len(subjects.Subjects) > 32 || len(scope.Criteria) == 0 || len(scope.Criteria) > 32 || len(c.Practices) == 0 || len(c.Practices) > 32 {
		return r, 2, errors.New("version or collection limit invalid")
	}
	if !timeUTC(now) {
		return r, 2, errors.New("evaluation time must be RFC3339 UTC")
	}
	definitions := map[string]Practice{}
	for _, p := range c.Practices {
		if !keyOK(p.StableKey) || !nonempty(p.Version) || !nonempty(p.Path) || !nonempty(p.Title) || !nonempty(p.FailureMode) || !nonempty(p.Benefit) || !nonempty(p.Remedy) || !nonempty(p.NegativeCase) || !nonempty(p.Definition) || len(p.Sources) == 0 || !unique(p.Sources) || definitions[p.StableKey].StableKey != "" {
			return r, 2, errors.New("invalid or duplicate practice")
		}
		switch p.Kind {
		case "evidence-integrity", "effect-scope":
			if p.MaxSteps != nil {
				return r, 2, errors.New("max_steps belongs only to bounded-work")
			}
		case "bounded-work":
			if p.MaxSteps == nil || *p.MaxSteps <= 0 || *p.MaxSteps > 1_000_000 {
				return r, 2, errors.New("bounded-work requires trusted positive max_steps")
			}
		default:
			return r, 2, fmt.Errorf("unsupported check kind %q", p.Kind)
		}
		definitions[p.StableKey] = p
	}
	pins := map[string]Pin{}
	for _, p := range scope.Criteria {
		if !keyOK(p.StableKey) || !nonempty(p.Version) || len(p.DefinitionDigest) != 64 || pins[p.StableKey].StableKey != "" {
			return r, 2, errors.New("invalid or duplicate criterion pin")
		}
		pins[p.StableKey] = p
	}
	if len(definitions) != len(pins) {
		r.CoverageGaps = append(r.CoverageGaps, "definition set differs from independent scope")
	}
	specBytes, err := json.Marshal(c.Spec)
	if err != nil {
		return r, 2, err
	}
	var spec catalogkit.Spec
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		return r, 2, err
	}
	for i := range spec.Layers {
		canonicalNodes(spec.Layers[i].Nodes)
	}
	bundle, err := catalogkit.Resolve(spec, catalogkit.Policy{})
	if err != nil {
		return r, 2, err
	}
	r.CatalogDigest = bundle.Digest
	semantic := map[string][]byte{}
	for key, pin := range pins {
		p, ok := definitions[key]
		if !ok {
			r.CoverageGaps = append(r.CoverageGaps, "missing definition "+key)
			continue
		}
		definition, err := semanticDefinition(p)
		if err != nil {
			return r, 2, err
		}
		if p.Version != pin.Version || digest(definition) != pin.DefinitionDigest {
			r.CoverageGaps = append(r.CoverageGaps, "changed definition/version "+key)
			continue
		}
		n, err := catalogkit.LookupStableKey(bundle, key)
		if err != nil {
			return r, 2, err
		}
		if n.ID() != p.Path || n.Hidden {
			return r, 2, fmt.Errorf("path mismatch or hidden criterion %s: catalog=%s practice=%s hidden=%t", key, n.ID(), p.Path, n.Hidden)
		}
		semantic[key] = definition
	}
	for key := range definitions {
		if _, ok := pins[key]; !ok {
			r.CoverageGaps = append(r.CoverageGaps, "foreign definition "+key)
		}
	}
	expected := map[string]ExpectedSubject{}
	for _, s := range scope.Subjects {
		if !keyOK(s.ID) || !nonempty(s.Revision) || len(s.Criteria) == 0 || len(s.Criteria) > 32 || s.AllowedActions == nil || !unique(s.Criteria) || !unique(s.AllowedActions) || expected[s.ID].ID != "" {
			return r, 2, errors.New("invalid or duplicate expected subject")
		}
		for _, key := range s.Criteria {
			if !keyOK(key) {
				return r, 2, errors.New("invalid expected criterion key")
			}
			if _, ok := pins[key]; !ok {
				return r, 2, errors.New("expected subject references foreign criterion")
			}
		}
		expected[s.ID] = s
	}
	actual := map[string]Subject{}
	for _, s := range subjects.Subjects {
		if !keyOK(s.ID) || !nonempty(s.Revision) || s.Checks == nil || len(s.Checks) > 32 || actual[s.ID].ID != "" {
			return r, 2, errors.New("invalid or duplicate subject")
		}
		if _, ok := expected[s.ID]; !ok {
			return r, 2, errors.New("foreign subject")
		}
		actual[s.ID] = s
	}
	failed := false
	for id, exp := range expected {
		s, ok := actual[id]
		if !ok {
			r.CoverageGaps = append(r.CoverageGaps, "missing subject "+id)
			continue
		}
		if s.Revision != exp.Revision {
			r.CoverageGaps = append(r.CoverageGaps, "revision mismatch "+id)
			continue
		}
		checks := map[string]Check{}
		for _, ch := range s.Checks {
			if !keyOK(ch.StableKey) || checks[ch.StableKey].StableKey != "" {
				return r, 2, errors.New("invalid or duplicate check")
			}
			if _, ok := pins[ch.StableKey]; !ok {
				return r, 2, errors.New("foreign criterion check")
			}
			checks[ch.StableKey] = ch
		}
		profile := fitness.Profile{Version: fitness.ProfileVersion, ID: "practice-review", Domain: "synthetic-review", Criteria: []fitness.Criterion{}}
		obs := []fitness.Observation{}
		canonSubject, err := canonicalSubject(s)
		if err != nil {
			return r, 2, err
		}
		subjectDigest := digest(append([]byte("practice-subject/v1\x00"), canonSubject...))
		for _, key := range exp.Criteria {
			pin := pins[key]
			profile.Criteria = append(profile.Criteria, fitness.Criterion{ID: key, DefinitionDigest: pin.DefinitionDigest, Kind: fitness.CriterionRequired, AllowedMethods: []fitness.EvidenceMethod{fitness.EvidenceMethodDeterministic}})
			ch, ok := checks[key]
			if !ok {
				r.CoverageGaps = append(r.CoverageGaps, "missing criterion "+id+"/"+key)
				continue
			}
			p, ok := definitions[key]
			if !ok || p.Version != pin.Version || semantic[key] == nil {
				continue
			}
			if p.Kind == "bounded-work" {
				if ch.Work == nil || ch.Work.Steps == nil || *ch.Work.Steps < 0 || ch.RequestedActions != nil {
					return r, 2, errors.New("bounded-work requires work steps only")
				}
			} else if ch.Work != nil {
				return r, 2, errors.New("work payload belongs only to bounded-work")
			}
			if p.Kind == "effect-scope" {
				if ch.RequestedActions == nil || !unique(*ch.RequestedActions) || len(*ch.RequestedActions) == 0 {
					return r, 2, errors.New("effect-scope requires unique requested actions")
				}
			} else if ch.RequestedActions != nil {
				return r, 2, errors.New("requested actions belong only to effect-scope")
			}
			ev := ch.Evidence
			if !keyOK(ev.SubjectID) || !nonempty(ev.Source) || !nonempty(ev.Revision) || !nonempty(ev.Text) || !timeUTC(ev.ObservedAt) || !timeUTC(ev.ValidUntil) || ev.ValidUntil.Before(ev.ObservedAt) || len(ev.Digest) != 64 {
				return r, 2, errors.New("invalid evidence envelope")
			}
			packet, err := checkPacket(id, s.Revision, exp.AllowedActions, ch)
			if err != nil {
				return r, 2, err
			}
			n, err := catalogkit.LookupStableKey(bundle, key)
			if err != nil {
				return r, 2, err
			}
			binding := catalogkit.Binding{NodeID: n.ID(), NodeDigest: catalogkit.NodeDigest(n), DefinitionDigest: pin.DefinitionDigest, Version: p.Version, Question: decision.Question{ID: key, Kind: decision.KindChoice, Instructions: "Evaluate deterministic practice property", Options: []decision.Option{{Name: "pass", Criteria: "Property holds"}, {Name: "unknown", Criteria: "Evidence cannot establish property"}}}, RequiredEvidence: []string{"subject-check"}}
			prepared, err := catalogkit.PrepareDecision(bundle, binding, semantic[key], []catalogkit.Evidence{{ID: "subject-check", Source: ev.Source, Revision: ev.Revision, Text: packet, Digest: digest([]byte(packet))}}, 64<<10)
			if err != nil {
				return r, 2, err
			}
			r.Bindings = append(r.Bindings, BindingReceipt{id, key, p.Version, n.ID(), pin.DefinitionDigest, catalogkit.NodeDigest(n), prepared.Digest})
			outcome := fitness.FindingPass
			reason := "deterministic property holds"
			if digest([]byte(ev.Text)) != ev.Digest || ev.Revision != s.Revision || ev.SubjectID != id {
				outcome = fitness.FindingFail
				reason = "evidence digest, subject, or revision mismatch"
			}
			switch p.Kind {
			case "bounded-work":
				if *ch.Work.Steps > *p.MaxSteps {
					outcome = fitness.FindingFail
					reason = "work exceeds declared positive limit"
				}
			case "evidence-integrity":
				if ev.Source == "" || ev.Revision == "" || ev.Digest == "" {
					outcome = fitness.FindingFail
					reason = "evidence identity incomplete"
				}
			case "effect-scope":
				allowed := map[string]bool{}
				for _, a := range exp.AllowedActions {
					allowed[a] = true
				}
				for _, a := range *ch.RequestedActions {
					if !allowed[a] {
						outcome = fitness.FindingFail
						reason = "requested effect outside independent allowed scope"
					}
				}
			}
			if !ev.ValidUntil.After(now) || ev.ObservedAt.After(now) {
				outcome = fitness.FindingUnknown
				reason = "evidence expired or from future"
			}
			obs = append(obs, fitness.Observation{CriterionID: key, DefinitionDigest: pin.DefinitionDigest, SubjectDigest: subjectDigest, Outcome: outcome, Method: fitness.EvidenceMethodDeterministic, Reason: reason, ArtifactDigests: []string{ev.Digest}, ObservedAt: ev.ObservedAt, ValidUntil: ev.ValidUntil})
		}
		for key := range checks {
			found := false
			for _, k := range exp.Criteria {
				if key == k {
					found = true
				}
			}
			if !found {
				return r, 2, fmt.Errorf("foreign subject criterion %s/%s", id, key)
			}
		}
		a, err := fitness.AssessProfile(profile, subjectDigest, obs, now)
		if err != nil {
			return r, 2, err
		}
		a.SoftScore = nil
		if a.RequiredStatus == fitness.RequiredFail {
			failed = true
		}
		if a.RequiredStatus == fitness.RequiredUnknown {
			r.CoverageGaps = append(r.CoverageGaps, "unknown assessment "+id)
		}
		r.Subjects = append(r.Subjects, SubjectReceipt{id, s.Revision, a})
	}
	sort.Slice(r.Bindings, func(i, j int) bool {
		if r.Bindings[i].SubjectID == r.Bindings[j].SubjectID {
			return r.Bindings[i].StableKey < r.Bindings[j].StableKey
		}
		return r.Bindings[i].SubjectID < r.Bindings[j].SubjectID
	})
	sort.Slice(r.Subjects, func(i, j int) bool { return r.Subjects[i].ID < r.Subjects[j].ID })
	sort.Strings(r.CoverageGaps)
	if len(r.CoverageGaps) > 0 {
		r.Status = "unknown"
		return r, 2, nil
	}
	if failed {
		r.Status = "fail"
		return r, 1, nil
	}
	r.Status = "pass"
	return r, 0, nil
}

func run(args []string, w io.Writer) int {
	f := flag.NewFlagSet("practice-review", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	catalogPath := f.String("catalog", "", "catalog JSON")
	scopePath := f.String("scope", "", "independent scope JSON")
	subjectsPath := f.String("subjects", "", "subjects JSON")
	at := f.String("at", "", "RFC3339 UTC evaluation time")
	format := f.String("format", "text", "text or json")
	if err := f.Parse(args); err != nil || f.NArg() != 0 || *catalogPath == "" || *scopePath == "" || *subjectsPath == "" || *at == "" || (*format != "text" && *format != "json") {
		fmt.Fprintln(w, "invalid arguments: require --catalog --scope --subjects --at and optional --format text|json")
		return 2
	}
	now, err := time.Parse(time.RFC3339, *at)
	if err != nil || now.Location() != time.UTC || now.Format(time.RFC3339) != *at {
		fmt.Fprintln(w, "invalid --at: require RFC3339 UTC ending Z")
		return 2
	}
	var c CatalogInput
	var scope Scope
	var subjects Subjects
	hashes := map[string]string{}
	for _, input := range []struct {
		name, path string
		out        any
	}{{"catalog", *catalogPath, &c}, {"scope", *scopePath, &scope}, {"subjects", *subjectsPath, &subjects}} {
		h, e := read(input.path, input.out)
		if e != nil {
			fmt.Fprintf(w, "invalid %s: %v\n", input.name, e)
			return 2
		}
		hashes[input.name] = h
	}
	r, code, err := evaluate(c, scope, subjects, now, hashes)
	if err != nil {
		fmt.Fprintf(w, "invalid input: %v\n", err)
		return 2
	}
	if *format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err = enc.Encode(r); err != nil {
			return 2
		}
	} else {
		if _, err = fmt.Fprintf(w, "practice review: %s (%s)\n%s\n", r.Status, r.Scope, r.Limits); err != nil {
			return 2
		}
		for _, gap := range r.CoverageGaps {
			if _, err = fmt.Fprintf(w, "gap: %s\n", gap); err != nil {
				return 2
			}
		}
		for _, s := range r.Subjects {
			if _, err = fmt.Fprintf(w, "subject %s: %s\n", s.ID, s.Assessment.RequiredStatus); err != nil {
				return 2
			}
		}
	}
	return code
}
func main() { os.Exit(run(os.Args[1:], os.Stdout)) }
