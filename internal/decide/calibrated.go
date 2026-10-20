package decide

// calibrated.go — SR-58 (docs/plans/surface-reckoning.md, F-69;
// docs/design/judgment-foundations.md §8, A8): every decision-port site
// outside certify is a proposal. It may rule, that is serve a provider's
// answer, only when the latest calibration reading `fathom decide
// calibrate` wrote for (site, template version, provider, resolved model)
// clears the site's bar AND carries an A/B in which the provider beat the
// site's deterministic answer on the same calibration set. Otherwise the
// provider is not asked (or, when the resolved model moved since the
// reading, its answer is dropped: a due site is uncalibrated, JF-10), the
// site gets a *DegradedError that is decision.ErrNoProvider, so it takes
// its deterministic path, and one "decide: not calibrated" line is
// disclosed per session and site (I60's shape).
//
// certify_suggest keeps its own rule: its answers are suggestions written
// only to a human-reviewed artifact, never a claim and never the store
// (cmd/fathom/certify_suggest.go, TC-24), and the levels it may offer are
// already restricted by the evidence (JF-9).
//
// The readings are files (no table, cache or background job: R5, the
// table freeze); each ask reads the readings directory, which costs a
// directory read beside a provider round trip it may save.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// NotCalibratedClass is the degrade class of a refused ask: what MissClass
// returns and a decided_by badge names.
const NotCalibratedClass = "not calibrated"

// notCalibratedText starts every refusal's error text, so a
// decisions.jsonl row can be told from a provider miss (askOutcome).
const notCalibratedText = "decide: not calibrated"

// CalibrationReadingSchema names the reading file's shape.
const CalibrationReadingSchema = "fathom.calibration-reading/1"

// calibrationReadingPrefix starts every reading file's name; the dated
// part keeps the files beside docs/bench/readings' own decision-port
// calibration readings and newest-last in a sorted listing.
const calibrationReadingPrefix = "decision-port-calibration-"

// ABReading is the deterministic baseline graded on the same calibration
// set as the provider: N items, Correct of them right.
type ABReading struct {
	N        int      `json:"n"`
	Correct  int      `json:"correct"`
	Accuracy *float64 `json:"accuracy"`
}

// CalibrationReading is one reading file: the kit's decision.Reading of a
// (site, template version, provider, resolved model), when it was taken,
// how many calibration items the provider left unanswered, and the A/B
// against the deterministic answer (nil for provider none, which is that
// answer, or when none was run).
type CalibrationReading struct {
	Schema     string           `json:"schema"`
	Taken      string           `json:"taken"`
	Reading    decision.Reading `json:"reading"`
	Unanswered int              `json:"unanswered"`
	Baseline   *ABReading       `json:"baseline"`
}

// NotCalibratedError is a refused ask's reason.
type NotCalibratedError struct {
	Site   string
	Reason string
}

func (e *NotCalibratedError) Error() string { return notCalibratedText + ": " + e.Reason }

// TemplateVersion is the version of site's question template, the
// reading key's second field. Every site's template is at its first
// version; a site that changes its question text bumps its own case here,
// which makes its readings due.
func TemplateVersion(site string) string {
	_ = site
	return "1"
}

// SiteBar is the bar site's reading must clear to rule: I62's gate bar (a
// Wilson 95% lower bound of at least 0.90) for the sites that prune or
// reorder a default list (the compile pre-filter drops sections, the
// obligations re-rank reorders rules), else ADR-056 R3's hint bar (90 of
// 100 right), the default docs/design/decision-models.md §2 names.
func SiteBar(site string) decision.CalibrationBar {
	switch site {
	case SiteCompilePrefilter, SiteObligationsRerank:
		return decision.GateBar()
	}
	return decision.HintBar()
}

// DeterministicToken is the decided_by tier a refused ask's row records:
// the table for a site whose deterministic answer is a table
// (readout_mode), else the computed answer.
func DeterministicToken(site string) string {
	if site == SiteReadoutMode {
		return DecidedTable
	}
	return DecidedComputed
}

// ReadingsDir is where calibration readings are written and read: the
// per-user fathom directory's calibration/ ("" when it does not resolve).
func ReadingsDir() string { return fathomhome.Path("calibration") }

// CalibrationSetsDir is where `fathom decide calibrate` reads each site's
// calibration set (<site>.jsonl) by default.
func CalibrationSetsDir() string { return fathomhome.Path("calibration", "sets") }

// ReadingFileBase is a reading file's name without its extension, for the
// reading of key taken on date (YYYY-MM-DD).
func ReadingFileBase(date string, key decision.ReadingKey) string {
	parts := []string{calibrationReadingPrefix + date, slug(key.Site), slug(key.Provider)}
	if key.ResolvedModel != "" {
		parts = append(parts, slug(key.ResolvedModel))
	}
	return strings.Join(parts, ".")
}

// slug keeps letters, digits, '-' and '_', so a model id such as
// "vendor/model-1.2" names one file.
func slug(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
}

// WriteReadingJSON writes cr under dir as the reading file the gate reads
// (ReadingFileBase(date, cr.Reading.Key) + ".json", indented, one trailing
// newline) and returns its path.
func WriteReadingJSON(dir, date string, cr CalibrationReading) (string, error) {
	if cr.Schema == "" {
		cr.Schema = CalibrationReadingSchema
	}
	data, err := json.MarshalIndent(cr, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, ReadingFileBase(date, cr.Reading.Key)+".json")
	return path, os.WriteFile(path, append(data, '\n'), 0o644)
}

// LatestReading is the newest reading under dir for site and provider (by
// Taken, then file name), nil when there is none. A file that is not a
// reading is skipped; one that cannot be read is an error.
func LatestReading(dir, site, provider string) (*CalibrationReading, error) {
	if dir == "" {
		return nil, nil
	}
	paths, err := filepath.Glob(filepath.Join(dir, calibrationReadingPrefix+"*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var best *CalibrationReading
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var cr CalibrationReading
		if json.Unmarshal(data, &cr) != nil || cr.Schema != CalibrationReadingSchema {
			continue
		}
		if cr.Reading.Key.Site != site || cr.Reading.Key.Provider != provider {
			continue
		}
		if best == nil || cr.Taken >= best.Taken {
			c := cr
			best = &c
		}
	}
	return best, nil
}

// RulingVerdict is whether the site cr is a reading of may rule under it,
// as the gate would judge an ask whose key is cr's own, and why not.
func RulingVerdict(cr CalibrationReading) (bool, string) {
	return requireCalibratedSite(cr.Reading.Key.Site, cr.Reading.Key, &cr)
}

// requireCalibratedSite is SR-58's rule: site may rule for an ask keyed
// current only when latest (its newest reading for the provider) speaks
// for current, its proceed band clears SiteBar(site), and its A/B shows the
// provider right on more of the calibration set than the deterministic
// answer. certify_suggest is exempt. reason names the first unmet
// condition.
func requireCalibratedSite(site string, current decision.ReadingKey, latest *CalibrationReading) (ok bool, reason string) {
	if site == SiteCertifySuggest {
		return true, ""
	}
	name := site
	if name == "" {
		name = "(unnamed)"
	}
	if latest == nil {
		return false, fmt.Sprintf("no calibration reading for site %s on provider %s", name, current.Provider)
	}
	r := latest.Reading
	bc := decision.CalibratedBand(decision.ProceedAtOrAbove, r, current, SiteBar(site))
	switch {
	case bc.Status == decision.CalibrationRecalibrationDue:
		return false, fmt.Sprintf("recalibration due: the latest reading is of %s@%s on %s/%s, the ask is %s@%s on %s/%s",
			r.Key.Site, r.Key.TemplateVersion, r.Key.Provider, r.Key.ResolvedModel,
			current.Site, current.TemplateVersion, current.Provider, current.ResolvedModel)
	case !bc.Calibrated():
		return false, fmt.Sprintf("the latest reading's proceed band is %s at the site's bar", bc.Status)
	}
	ab := latest.Baseline
	if ab == nil || ab.N <= 0 {
		return false, "no A/B against the deterministic answer"
	}
	set := r.N + latest.Unanswered
	if set <= 0 || r.Correct*ab.N <= ab.Correct*set {
		return false, fmt.Sprintf("the A/B does not beat the deterministic answer (%d/%d against %d/%d)", r.Correct, set, ab.Correct, ab.N)
	}
	return true, ""
}

// proposing is the gate in front of a composed port (Open, Gated, a
// reopened port): what every site asks. An ask whose context names no site
// is not a site's ask (a port built in code, a test of the provider path)
// and passes through: every product site names itself (AtSite, WithSite;
// the compile pre-filter and the obligations re-rank in internal/semantic),
// and a site that did not could not be calibrated either.
type proposing struct {
	inner decision.Decider
	obs   *degrader
	dir   string
}

// propose puts d behind the calibration gate, its readings under dir and
// its line through obs.
func propose(d decision.Decider, obs *degrader, dir string) decision.Decider {
	return proposing{inner: d, obs: obs, dir: dir}
}

func (p proposing) Name() string { return p.inner.Name() }

// Capabilities declares what the gated port declares.
func (p proposing) Capabilities() decision.Capabilities { return decision.CapabilitiesOf(p.inner) }

func (p proposing) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	scope := ScopeFrom(ctx)
	if scope.Site == "" || scope.Site == SiteCertifySuggest {
		return p.inner.Decide(ctx, req)
	}
	provider := p.inner.Name()
	cur := decision.ReadingKey{Site: scope.Site, TemplateVersion: TemplateVersion(scope.Site), Provider: provider}
	latest, err := LatestReading(p.dir, scope.Site, provider)
	if err != nil {
		return p.refuse(scope, provider, "the calibration readings could not be read: "+oneLine(err.Error()))
	}
	if latest != nil {
		cur.ResolvedModel = latest.Reading.Key.ResolvedModel
	}
	if ok, why := requireCalibratedSite(scope.Site, cur, latest); !ok {
		return p.refuse(scope, provider, why)
	}
	resp, err := p.inner.Decide(ctx, req)
	if err != nil {
		return resp, err
	}
	cur.ResolvedModel = resp.Model
	if ok, why := requireCalibratedSite(scope.Site, cur, latest); !ok {
		return p.refuse(scope, provider, why)
	}
	return resp, nil
}

// refuse discloses the refusal once per scope and returns the
// deterministic path's signal.
func (p proposing) refuse(scope Scope, provider, why string) (decision.Response, error) {
	if p.obs != nil && p.obs.w != nil {
		p.obs.mu.Lock()
		key := degradeKey{scope: scope, class: NotCalibratedClass}
		if !p.obs.disclosed[key] {
			if p.obs.disclosed == nil {
				p.obs.disclosed = map[degradeKey]bool{}
			}
			p.obs.disclosed[key] = true
			who := ""
			if s := scope.String(); s != "" {
				who = " (" + s + ")"
			}
			fmt.Fprintf(p.obs.w, "%s%s: %s; the site answered on its deterministic path\n", notCalibratedText, who, why)
		}
		p.obs.mu.Unlock()
	}
	return decision.Response{Provider: provider}, &DegradedError{Provider: provider, Err: &NotCalibratedError{Site: scope.Site, Reason: why}}
}

// isNotCalibrated reports whether err is a refused ask's.
func isNotCalibrated(err error) bool {
	var nce *NotCalibratedError
	return errors.As(err, &nce)
}
