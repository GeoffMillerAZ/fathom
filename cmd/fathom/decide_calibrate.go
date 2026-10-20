package main

// decide_calibrate.go — `fathom decide calibrate --provider X`
// (docs/design/judgment-foundations.md §8 and owner rule 2): every
// registered decision-port site is re-graded against its calibration set,
// and one calibration reading per site is written, so adopting a better
// model is a measurement, not a redesign. A reading is the kit's
// decision.Reading of (site, template version, provider, resolved model):
// n, accuracy by confidence band, Wilson bounds, Brier score, AUROC and
// the label provenance mix, beside an A/B of the site's deterministic
// answer graded on the same set. It is written as a pair in the shape
// docs/bench/readings use: a JSON reading the decision port reads before a
// site may rule (internal/decide calibrated.go) and a Markdown reading
// with a plain verdict and the confidence-bucket table `fathom decide log`
// parses. Files only: no table, cache or background job.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// decideCalibratePorts opens the port calibrate grades through: the same
// fail-open glue every site uses, so a provider that cannot open prints the
// one disclosed line and comes back as provider none.
var decideCalibratePorts = func(loaded config.Loaded, stderr io.Writer) *decide.Ports {
	return openDecisionPorts(loaded, "", resolveCallLogPath(), "fathom decide calibrate", stderr)
}

// decideCalibrateNow is calibrate's clock: a reading's Taken and its date.
var decideCalibrateNow = time.Now

// calibrationItem is one line of a site's calibration set
// (<sets>/<site>.jsonl): the state and question the site would ask, the
// adjudicated answer, the site's deterministic answer to the same item and
// where the adjudication came from.
type calibrationItem struct {
	ID            string   `json:"id"`
	State         string   `json:"state"`
	Kind          string   `json:"kind"`
	Instructions  string   `json:"instructions"`
	Options       []string `json:"options,omitempty"`
	TrueCriteria  string   `json:"true_criteria,omitempty"`
	FalseCriteria string   `json:"false_criteria,omitempty"`
	Levels        []string `json:"levels,omitempty"`
	Rubric        string   `json:"rubric,omitempty"`
	Min           float64  `json:"min,omitempty"`
	Max           float64  `json:"max,omitempty"`
	// Expected is the right answer: an option name (choice), "true" or
	// "false" (predicate), a number (score).
	Expected string `json:"expected"`
	// Baseline is the site's deterministic answer, in Expected's form; ""
	// when the deterministic path answers nothing.
	Baseline string `json:"baseline"`
	// Provenance is who adjudicated Expected: human, frontier or computed.
	Provenance string `json:"provenance"`
}

// question is the item as the site asks it.
func (it calibrationItem) question() decision.Question {
	q := decision.Question{ID: it.ID, Kind: decision.Kind(it.Kind), Instructions: it.Instructions,
		TrueCriteria: it.TrueCriteria, FalseCriteria: it.FalseCriteria, Levels: it.Levels, Rubric: it.Rubric, Min: it.Min, Max: it.Max}
	for _, o := range it.Options {
		q.Options = append(q.Options, decision.Option{Name: o})
	}
	return q
}

// answerText renders a in an item's Expected form.
func answerText(kind string, a decision.Answer) string {
	switch decision.Kind(kind) {
	case decision.KindChoice:
		return a.Choice
	case decision.KindScore:
		return strconv.FormatFloat(a.Score, 'f', -1, 64)
	}
	if a.Noul >= 0.5 {
		return "true"
	}
	return "false"
}

func sameAnswer(a, b string) bool {
	a, b = strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	return a != "" && a == b
}

// readCalibrationSet reads one site's set, sorted by id; a missing file is
// fs.ErrNotExist.
func readCalibrationSet(path string) ([]calibrationItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var items []calibrationItem
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8<<20)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var it calibrationItem
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, n, err)
		}
		switch {
		case it.ID == "" || seen[it.ID]:
			return nil, fmt.Errorf("%s line %d: an item needs an id of its own", path, n)
		case it.Kind != string(decision.KindChoice) && it.Kind != string(decision.KindPredicate) && it.Kind != string(decision.KindScore):
			return nil, fmt.Errorf("%s line %d: kind %q is not choice, predicate or score", path, n, it.Kind)
		case strings.TrimSpace(it.Expected) == "":
			return nil, fmt.Errorf("%s line %d: no expected answer", path, n)
		}
		switch decision.Provenance(it.Provenance) {
		case decision.ProvenanceHuman, decision.ProvenanceFrontier, decision.ProvenanceComputed:
		default:
			return nil, fmt.Errorf("%s line %d: provenance %q is not human, frontier or computed", path, n, it.Provenance)
		}
		seen[it.ID] = true
		items = append(items, it)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

// gradeSite grades items at site. Provider none grades the deterministic
// answer itself (confidence 1, no A/B); any other provider is asked each
// item through d from site's scope, and the deterministic answer is graded
// on the same items as its A/B. An item the provider leaves unanswered is
// counted, not graded. A resolved model that moves mid-run is an error: a
// reading speaks for one model.
func gradeSite(ctx context.Context, d decision.Decider, provider, site string, items []calibrationItem) (decide.CalibrationReading, error) {
	key := decision.ReadingKey{Site: site, TemplateVersion: decide.TemplateVersion(site), Provider: provider}
	baseline := &decide.ABReading{N: len(items)}
	var baseSamples, samples []decision.LabeledSample
	for _, it := range items {
		right := sameAnswer(it.Baseline, it.Expected)
		if right {
			baseline.Correct++
		}
		baseSamples = append(baseSamples, decision.LabeledSample{Confidence: 1, Correct: right, Provenance: decision.Provenance(it.Provenance)})
	}
	if provider == "none" {
		r, err := decision.NewReading(key, baseSamples)
		return decide.CalibrationReading{Reading: r}, err
	}
	unanswered := 0
	for _, it := range items {
		resp, err := d.Decide(decide.WithSite(ctx, site), decision.Request{State: it.State, Questions: []decision.Question{it.question()}})
		a, ok := resp.Answers[it.ID]
		if err != nil || !ok {
			unanswered++
			continue
		}
		switch {
		case key.ResolvedModel == "":
			key.ResolvedModel = resp.Model
		case resp.Model != key.ResolvedModel:
			return decide.CalibrationReading{}, fmt.Errorf("the resolved model moved during the run (%s, then %s); run it again", key.ResolvedModel, resp.Model)
		}
		samples = append(samples, decision.LabeledSample{Confidence: a.Confidence, Correct: sameAnswer(answerText(it.Kind, a), it.Expected), Provenance: decision.Provenance(it.Provenance)})
	}
	r, err := decision.NewReading(key, samples)
	return decide.CalibrationReading{Reading: r, Unanswered: unanswered, Baseline: baseline}, err
}

// writeCalibrationReading writes cr under dir, taken at now: the JSON
// reading the decision port reads and its Markdown reading (a plain
// verdict, the confidence-bucket table, the scores, the label provenance
// and the A/B), both named for the date and the key. Byte-identical for
// the same reading and clock.
func writeCalibrationReading(dir string, cr decide.CalibrationReading, now func() time.Time) (mdPath, jsonPath string, err error) {
	t := now().UTC()
	date := t.Format("2006-01-02")
	cr.Schema = decide.CalibrationReadingSchema
	cr.Taken = t.Format(time.RFC3339)
	if cr.Baseline != nil {
		ab := *cr.Baseline
		ab.Accuracy = nil
		if ab.N > 0 {
			acc := float64(ab.Correct) / float64(ab.N)
			ab.Accuracy = &acc
		}
		cr.Baseline = &ab
	}
	jsonPath, err = decide.WriteReadingJSON(dir, date, cr)
	if err != nil {
		return "", "", err
	}
	mdPath = filepath.Join(dir, decide.ReadingFileBase(date, cr.Reading.Key)+".md")
	return mdPath, jsonPath, os.WriteFile(mdPath, []byte(renderCalibrationReading(cr, date)), 0o644)
}

// f4 renders a statistic, or why it is not stated.
func f4(v *float64, absent string) string {
	if v == nil {
		return absent
	}
	return fmt.Sprintf("%.4f", *v)
}

// renderCalibrationReading is cr's Markdown reading.
func renderCalibrationReading(cr decide.CalibrationReading, date string) string {
	r := cr.Reading
	k := r.Key
	who := k.Provider
	if k.ResolvedModel != "" {
		who += "/" + k.ResolvedModel
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Calibration reading: %s on %s (%s)\n\n## Verdict, plainly\n\n", k.Site, who, date)

	var rule string
	switch ok, why := decide.RulingVerdict(cr); {
	case k.Site == decide.SiteCertifySuggest:
		rule = "certify_suggest keeps its own rule, so this reading informs it and gates nothing"
	case k.Provider == "none":
		rule = "this is the deterministic answer's own reading, the baseline a provider's A/B is measured against"
	case ok:
		rule = "the site may rule"
	default:
		rule = "the site may not rule: " + why
	}
	if r.N == 0 {
		fmt.Fprintf(&b, "**%s on %s: no labeled samples, so no accuracy is stated; %s.**\n\n", k.Site, who, rule)
	} else {
		fmt.Fprintf(&b, "**%s on %s: %d of %d right, accuracy %s (Wilson 95%% %s to %s); %s.**\n\n",
			k.Site, who, r.Correct, r.N, f4(r.Accuracy, "none"), f4(r.AccuracyLo, "none"), f4(r.AccuracyHi, "none"), rule)
	}
	bar := decide.SiteBar(k.Site)
	barText := fmt.Sprintf("at least %d samples and %.2f right in the proceed band", bar.MinN, bar.MinAccuracy)
	if bar.MinWilsonLo > 0 {
		barText = fmt.Sprintf("a Wilson 95%% lower bound of at least %.2f in the proceed band", bar.MinWilsonLo)
	}
	fmt.Fprintf(&b, "Taken %s by `fathom decide calibrate`, template version %s, %d calibration items, %d unanswered. "+
		"A site outside certify rules only when its latest reading clears its bar (%s) and the provider beats the deterministic answer on the same set.\n\n",
		cr.Taken, k.TemplateVersion, r.N+cr.Unanswered, cr.Unanswered, barText)

	b.WriteString("## Accuracy by confidence bucket\n\n| Bucket | Hits/N | Fraction | Wilson 95% |\n|---|---|---|---|\n")
	for _, band := range r.ByBand {
		if band.N == 0 {
			fmt.Fprintf(&b, "| %s | 0/0 | none | none |\n", band.Band)
			continue
		}
		frac := float64(band.Correct) / float64(band.N)
		fmt.Fprintf(&b, "| %s | %d/%d | %.4f | %s to %s |\n", band.Band, band.Correct, band.N, frac, f4(band.WilsonLo, "none"), f4(band.WilsonHi, "none"))
	}

	monotonic := "not stated (fewer than two bands hold samples)"
	if r.Monotonic != nil {
		monotonic = map[bool]string{true: "yes", false: "no: no confidence threshold can repair the site"}[*r.Monotonic]
	}
	fmt.Fprintf(&b, "\n## Scores\n\n- Brier score: %s (the mean squared gap between confidence and being right; lower is better)\n"+
		"- AUROC: %s (how well confidence separates right answers from wrong ones; 0.5 is chance)\n- Monotonic across bands: %s\n",
		f4(r.Brier, "not stated (no labeled samples)"), f4(r.AUROC, "not stated (right or wrong answers are missing)"), monotonic)

	fmt.Fprintf(&b, "\n## Label provenance\n\n- human: %d\n- frontier: %d\n- computed: %d\n", r.Labels.Human, r.Labels.Frontier, r.Labels.Computed)

	b.WriteString("\n## A/B against the deterministic answer\n\n")
	switch ab := cr.Baseline; {
	case k.Provider == "none":
		b.WriteString("Not run: provider none is the deterministic answer.\n")
	case ab == nil || ab.N == 0:
		b.WriteString("Not run.\n")
	default:
		set := r.N + cr.Unanswered
		beats := "the provider does not beat the deterministic answer"
		if set > 0 && r.Correct*ab.N > ab.Correct*set {
			beats = "the provider beats the deterministic answer"
		}
		fmt.Fprintf(&b, "- provider: %d of %d right\n- deterministic answer: %d of %d right\n- %s\n", r.Correct, set, ab.Correct, ab.N, beats)
	}
	return b.String()
}

// cmdDecideCalibrate is `fathom decide calibrate --provider X`.
func cmdDecideCalibrate(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("decide calibrate", stderr)
	provider := flags.String("provider", "", "the provider to grade: none (the deterministic answer, the baseline), llm, typesafe or openai")
	sets := flags.String("sets", "", "directory holding one <site>.jsonl calibration set per site (default: calibration/sets under the per-user fathom directory)")
	out := flags.String("out", "", "directory the readings are written to (default: calibration/ under the per-user fathom directory, where the decision port reads them)")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if *sets == "" {
		*sets = decide.CalibrationSetsDir()
	}
	if *out == "" {
		*out = decide.ReadingsDir()
	}
	if *provider == "" || flags.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: fathom decide calibrate --provider none|llm|typesafe|openai [--sets DIR] [--out DIR]")
		return 1
	}
	if *out == "" {
		fmt.Fprintln(stderr, "decide calibrate: no readings directory: set FATHOM_HOME or pass --out")
		return 1
	}
	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil {
		fmt.Fprintf(stderr, "decide calibrate: loading config: %v\n", err)
		return 1
	}
	dc := &loaded.Config.Decide
	dc.Provider = *provider
	if *provider != "none" {
		// The command line is the user's own consent for this run.
		dc.UserProvider, dc.ProviderSource = *provider, config.SourceUser
	}
	ports := decideCalibratePorts(loaded, stderr)
	grader := ports.Grader()
	if *provider != "none" && (grader == nil || grader.Name() != *provider || ports.Gate.Closed) {
		fmt.Fprintf(stderr, "decide calibrate: provider %s did not open (the line above says why); refused, no reading written\n", *provider)
		return 1
	}

	sites := append([]string(nil), decide.AllSites...)
	sort.Strings(sites)
	failed := 0
	for _, site := range sites {
		path := filepath.Join(*sets, site+".jsonl")
		items, err := readCalibrationSet(path)
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(stdout, "%s: no calibration set (%s); no reading\n", site, path)
			continue
		}
		if err == nil {
			var cr decide.CalibrationReading
			if cr, err = gradeSite(context.Background(), grader, *provider, site, items); err == nil {
				var mdPath string
				if mdPath, _, err = writeCalibrationReading(*out, cr, decideCalibrateNow); err == nil {
					verdict := "may not rule"
					if ok, _ := decide.RulingVerdict(cr); ok {
						verdict = "may rule"
					}
					if *provider == "none" {
						verdict = "baseline"
					}
					fmt.Fprintf(stdout, "%s: %d of %d right, %d unanswered; %s; %s\n", site, cr.Reading.Correct, cr.Reading.N, cr.Unanswered, verdict, mdPath)
					continue
				}
			}
		}
		failed++
		fmt.Fprintf(stderr, "decide calibrate: %s: %v\n", site, err)
	}
	if failed > 0 {
		return 1
	}
	return 0
}
