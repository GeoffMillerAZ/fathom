package decide

// requestplan.go — I61 (ADR-056, docs/plans/judgment-foundations.md): the
// request plan is the one path every ask's request leaves through. compose
// puts it in front of every provider adapter (Open, Ports.Gated and the
// reopening port all compose), so every ask a configured provider answers
// passes it and no site can step around it; provider none never composes,
// so it never does. The plan keeps only the request fields the asking site
// declares, sends nothing that names a path a [decide] exclude glob
// matches, redacts secret-shaped strings with llmsec's extended catalog and
// counts them on the decision row, and under FATHOM_DECIDE_PLAN_ONLY=1
// prints exactly what would leave and sends nothing.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/llmsec"
	"github.com/geoffmilleraz/fathom/internal/pathglob"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// PlanOnlyEnv is the plan-only switch: with exactly "1" in a process's
// environment when its port opens, every ask writes its request plan, the
// JSON of the request exactly as it would go to the provider adapter, as
// one line to the disclosure writer (stderr by default), sends nothing, and
// answers on its deterministic path with one `plan only` line.
const PlanOnlyEnv = "FATHOM_DECIDE_PLAN_ONLY"

// The miss classes the request plan adds to degradeClass's: the plan let
// nothing leave (an excluded path, a site with no declaration), a template
// names a security property (securitytemplate.go), or plan-only printed it.
const (
	classExcluded = "excluded"
	classSecurity = "security property"
	classPlanOnly = "plan only"
)

// planRefusedError is an ask the request plan sent nothing for. It reaches
// the site wrapped in a *DegradedError (the kit's fallback answers with
// decision.None), so the site takes its deterministic path and the degrade
// line names class.
type planRefusedError struct{ class, reason string }

func (e *planRefusedError) Error() string { return e.reason }

// The request fields a site may declare. ID and Kind always go: they are
// how an answer finds its question.
const (
	fieldState        = "state"        // Request.State: the spans and assembled context
	fieldInstructions = "instructions" // Question.Instructions: the question text
	fieldOptions      = "options"      // Question.Options
	fieldCriteria     = "criteria"     // Question.TrueCriteria and FalseCriteria
	fieldRange        = "range"        // Question.Min and Max
	fieldRubric       = "rubric"       // Question.Rubric
	fieldLevels       = "levels"       // Question.Levels
	fieldImages       = "images"       // Request.Images
)

// requestFields is every field a declaration may name, in plan order.
var requestFields = []string{fieldState, fieldInstructions, fieldOptions, fieldCriteria, fieldRange, fieldRubric, fieldLevels, fieldImages}

// siteDeclaration is what one site sends a provider: its unit (what one
// span is; never a whole file unless the unit is a file, never a
// transcript) and the request fields its questions use.
type siteDeclaration struct {
	Site   string
	Unit   string
	Fields []string
}

// siteDeclarations declares every site in AllSites and nothing else
// (TestEverySiteDeclaresItsRequestFields): a site without one sends
// nothing.
var siteDeclarations = []siteDeclaration{
	{SiteRoute, "the prompt", []string{fieldState, fieldInstructions, fieldOptions}},
	{SiteCompilePrefilter, "guide sections, each with its doc path", []string{fieldState, fieldInstructions}},
	{SiteObligationsRerank, "the edited file's path and hunk, and each rule's text", []string{fieldState, fieldInstructions, fieldCriteria}},
	{SiteCertifySuggest, "each test function's source, with its path and line", []string{fieldState, fieldInstructions, fieldOptions, fieldCriteria}},
	{SiteAssess, "a unit's commit diffs, or each guided file's diff", []string{fieldState, fieldInstructions, fieldOptions, fieldCriteria}},
	{SitePreflight, "a plan row's measured features, never its text", []string{fieldState, fieldInstructions, fieldRubric, fieldLevels}},
}

// siteDeclarationOf is site's declaration. An ask with no site on its
// context (a CLI verb, which asks one site) is planned under the union of
// every site's fields, still never images; a site with no declaration (an
// opt-in name with no call site) has none.
func siteDeclarationOf(site string) (siteDeclaration, bool) {
	if site == "" {
		var fields []string
		for _, f := range requestFields {
			for _, d := range siteDeclarations {
				if slices.Contains(d.Fields, f) && !slices.Contains(fields, f) {
					fields = append(fields, f)
				}
			}
		}
		return siteDeclaration{Unit: "the asking verb's", Fields: fields}, true
	}
	for _, d := range siteDeclarations {
		if d.Site == site {
			return d, true
		}
	}
	return siteDeclaration{}, false
}

// requestPlan is one ask's request as it leaves.
type requestPlan struct {
	Site       string
	Unit       string
	Request    decision.Request
	Redactions int
}

// buildRequestPlan plans req from site: the declared fields only, nothing
// from an excluded path, secret-shaped strings redacted and counted. An
// error is a *planRefusedError: nothing leaves.
func buildRequestPlan(site string, req decision.Request, exclude []string, red *llmsec.Redactor) (requestPlan, error) {
	decl, ok := siteDeclarationOf(site)
	if !ok {
		return requestPlan{}, &planRefusedError{class: classExcluded, reason: fmt.Sprintf("site %s declares no request fields, so nothing it asks leaves this machine", site)}
	}
	out := keepDeclared(req, decl.Fields)
	if ref, glob, hit := excludedSpan(out, exclude); hit {
		return requestPlan{}, &planRefusedError{class: classExcluded, reason: fmt.Sprintf("a span from %s matches [decide] exclude glob %q, so it never leaves this machine", ref, glob)}
	}
	n := redactRequest(&out, red)
	return requestPlan{Site: site, Unit: decl.Unit, Request: out, Redactions: n}, nil
}

// keepDeclared copies req with only fields' fields. A question field the
// kit adds later is left behind until a site declares it.
func keepDeclared(req decision.Request, fields []string) decision.Request {
	has := func(f string) bool { return slices.Contains(fields, f) }
	var out decision.Request
	if has(fieldState) {
		out.State = req.State
	}
	if has(fieldImages) {
		out.Images = slices.Clone(req.Images)
	}
	out.Questions = make([]decision.Question, len(req.Questions))
	for i, q := range req.Questions {
		k := decision.Question{ID: q.ID, Kind: q.Kind}
		if has(fieldInstructions) {
			k.Instructions = q.Instructions
		}
		if has(fieldOptions) {
			k.Options = slices.Clone(q.Options)
		}
		if has(fieldCriteria) {
			k.TrueCriteria, k.FalseCriteria = q.TrueCriteria, q.FalseCriteria
		}
		if has(fieldRange) {
			k.Min, k.Max = q.Min, q.Max
		}
		if has(fieldRubric) {
			k.Rubric = q.Rubric
		}
		if has(fieldLevels) {
			k.Levels = slices.Clone(q.Levels)
		}
		out.Questions[i] = k
	}
	return out
}

// requestTexts is every string of req that leaves.
func requestTexts(req decision.Request) []string {
	texts := []string{req.State}
	for _, q := range req.Questions {
		texts = append(texts, q.ID, q.Instructions, q.TrueCriteria, q.FalseCriteria, q.Rubric)
		texts = append(texts, q.Levels...)
		for _, o := range q.Options {
			texts = append(texts, o.Name, o.Criteria)
		}
	}
	return texts
}

// excludedSpan finds the first path req names that an exclude glob
// matches. Sites name a span's path in its text (`file: <path>`, `doc:
// <path>`, `(<path>:<line>)`, a diff header), so every path-shaped token is
// a candidate, and so is every trailing part of one: a diff's `a/` prefix
// or an absolute path is no escape, and a glob without a slash matches a
// base name at any depth.
func excludedSpan(req decision.Request, globs []string) (ref, glob string, hit bool) {
	if len(globs) == 0 {
		return "", "", false
	}
	for _, text := range requestTexts(req) {
		for _, tok := range strings.FieldsFunc(text, isPathDelimiter) {
			for _, cand := range pathCandidates(tok) {
				for _, g := range globs {
					if pathglob.Match(g, cand) {
						return cand, g, true
					}
				}
			}
		}
	}
	return "", "", false
}

func isPathDelimiter(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune("()[]{}<>\"'`,;|=", r)
}

// pathCandidates is tok cleaned of a line suffix (`:12`, `:3-9`), an
// anchor (`#L4`) and sentence punctuation, then every trailing part of it
// after a slash.
func pathCandidates(tok string) []string {
	if i := strings.IndexByte(tok, '#'); i >= 0 {
		tok = tok[:i]
	}
	for i := 0; i+1 < len(tok); i++ {
		if tok[i] == ':' && tok[i+1] >= '0' && tok[i+1] <= '9' {
			tok = tok[:i]
			break
		}
	}
	tok = strings.TrimRight(strings.TrimPrefix(tok, "./"), ".:!?")
	if tok == "" {
		return nil
	}
	cands := []string{tok}
	for i := 0; i+1 < len(tok); i++ {
		if tok[i] == '/' {
			cands = append(cands, tok[i+1:])
		}
	}
	return cands
}

// redactRequest redacts every free-text field of req in place with red and
// returns how many spans it replaced. Answer vocabulary (ids, option
// names, levels) is left as it is, since an answer names it.
func redactRequest(req *decision.Request, red *llmsec.Redactor) int {
	n := 0
	scrub := func(s string) string {
		out, k := red.RedactCount(s)
		n += k
		return out
	}
	req.State = scrub(req.State)
	for i := range req.Questions {
		q := &req.Questions[i]
		q.Instructions, q.Rubric = scrub(q.Instructions), scrub(q.Rubric)
		q.TrueCriteria, q.FalseCriteria = scrub(q.TrueCriteria), scrub(q.FalseCriteria)
		for j := range q.Options {
			q.Options[j].Criteria = scrub(q.Options[j].Criteria)
		}
	}
	return n
}

// planBytes is exactly what plan sends the provider adapter: the planned
// request's JSON and a newline. A dry run (plan-only now, `fathom ask
// --dry-run` later) prints these bytes.
func planBytes(plan requestPlan) ([]byte, error) {
	b, err := json.Marshal(plan.Request)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// requestPlanner is the request plan's place in the wrapper chain (guard).
type requestPlanner struct {
	exclude []string
	red     *llmsec.Redactor
	rec     *Recorder

	mu sync.Mutex
	// planOnly receives each plan under PlanOnlyEnv; nil sends.
	planOnly io.Writer
}

func newRequestPlanner(cfg config.Decide, rec *Recorder, planOnly io.Writer) *requestPlanner {
	return &requestPlanner{exclude: slices.Clone(cfg.Exclude), red: llmsec.NewRedactor(llmsec.ExtendedPosture()), rec: rec, planOnly: planOnly}
}

// guard is inner asked through the plan.
func (p *requestPlanner) guard(inner decision.Decider) decision.Decider {
	return planned{inner: inner, p: p}
}

type planned struct {
	inner decision.Decider
	p     *requestPlanner
}

func (d planned) Name() string { return d.inner.Name() }

// Capabilities declares what the wrapped provider declares.
func (d planned) Capabilities() decision.Capabilities { return decision.CapabilitiesOf(d.inner) }

func (d planned) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	site := ScopeFrom(ctx).Site
	if err := refuseSecurityTemplate(site, req); err != nil {
		return decision.Response{Provider: d.Name()}, err
	}
	plan, err := buildRequestPlan(site, req, d.p.exclude, d.p.red)
	if err != nil {
		return decision.Response{Provider: d.Name()}, err
	}
	if d.p.planOnly != nil {
		b, err := planBytes(plan)
		if err != nil {
			return decision.Response{Provider: d.Name()}, err
		}
		d.p.mu.Lock()
		_, _ = d.p.planOnly.Write(b)
		d.p.mu.Unlock()
		return decision.Response{Provider: d.Name()}, &planRefusedError{class: classPlanOnly, reason: PlanOnlyEnv + "=1: the request plan was printed and nothing was sent"}
	}
	resp, err := d.inner.Decide(ctx, plan.Request)
	if err == nil {
		d.p.rec.noteRedactions(plan.Request.Questions, plan.Redactions)
	}
	return resp, err
}
