package main

// certify_suggest.go — "fathom certify suggest" (TC-24, design doc §6,
// ADR-043 decision 6): for every Test* symbol with NO certification
// declared at all (certifyingTestSymbolKeys, shared with TC-22's
// orphans), one batched decision-port request per content-sized chunk —
// a choice over the strength ladder and a noul "asserts a real
// behaviour", state = that test's own source capped at 4 KiB. A
// suggestion is NEVER a claim and NEVER written to the store — only to
// workspace/artifacts/certify-suggestions.jsonl, a human-reviewed
// artifact `certify gaps` renders a hint from when present.
//
// Bands (agentic-kit foundations/decision.BandOf, 0.5/0.9 — ADR-042):
//   - PROCEED on BOTH questions (the strength choice's own Confidence in
//     BandProceed, AND the noul's raw truth-probability >= 0.9 — a
//     confident "this does NOT assert a real behaviour" must never count
//     as proceed just because the DIRECTION is confident) -> written to
//     the artifact and printed as "suggested: ...".
//   - otherwise, if either answer clears the CONFIRM floor (choice not
//     BandUnsure, or noul >= 0.5) -> printed as "unsure: ..." only, never
//     persisted.
//   - otherwise (both weak) -> fully silent; "nothing below" per the
//     design doc.
//
// Batching is a small LOCAL equivalent of internal/semantic's
// decisionBatchPlan/decisionBatchCaps (DP-26): that helper is unexported
// and package-private to internal/semantic (see its own doc comment on
// why: "internal/semantic stays free of the decide.Recorder type
// itself"), so it cannot be imported here. The caps this file uses are
// the SAME numbers DP-26 chose for typesafe/any-unrecognized-Decider (64
// questions, 96 KiB of state) — one test contributes exactly 2 questions,
// so the effective per-chunk test cap is 32, whichever cap binds first.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const (
	certifySuggestMaxQuestions   = 64
	certifySuggestMaxStateBytes  = 96 * 1024
	certifySuggestMaxSourceBytes = 4 * 1024
)

// certifyStrengthOptions is the strength ladder (internal/extractor's own
// CertifyStrength constants — this file reuses their string values rather
// than inventing its own) offered to the decision port, each with a
// one-line criterion (design doc §6, ADR-043 decision 6).
var certifyStrengthOptions = []decision.Option{
	{Name: "smoke", Criteria: "runs the code with no real assertion on the result — a crash-only check."},
	{Name: "asserts", Criteria: "asserts a concrete expected outcome for at least one case."},
	{Name: "golden", Criteria: "compares the full output against a stored golden/fixture value."},
	{Name: "property", Criteria: "checks a property holds across many generated or varied inputs."},
	{Name: "conformance", Criteria: "checks a concrete implementation against a shared interface contract."},
	{Name: "mutation-killed", Criteria: "is confirmed, by a mutation report, to kill mutants in the code it covers."},
}

// certifySuggestion is one proceed-band row — the exact JSON shape design
// doc §6 names: {test, file, line, strength, asserts_noul, confidence,
// model, generated_at}. NEVER a claim, NEVER written to the store.
type certifySuggestion struct {
	Test        string  `json:"test"`
	File        string  `json:"file"`
	Line        int     `json:"line"`
	Strength    string  `json:"strength"`
	AssertsNoul float64 `json:"asserts_noul"`
	Confidence  float64 `json:"confidence"`
	Model       string  `json:"model"`
	GeneratedAt string  `json:"generated_at"`
}

func cmdCertifySuggest(args []string, stdout, stderr io.Writer) int {
	loaded, cfgErr := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if cfgErr != nil {
		fmt.Fprintf(stderr, "warn: decision port disabled: loading config: %v\n", cfgErr)
		loaded = config.Loaded{Config: config.Config{Decide: config.DefaultDecide}}
	}
	ports := openDecisionPorts(loaded, "", resolveCallLogPath(), "fathom certify suggest", stderr)
	return runCertifySuggest(args, ports, time.Now, stdout, stderr)
}

// runCertifySuggest is cmdCertifySuggest's testable core: ports is
// injected (a real config-loaded *decide.Ports from the CLI entry point, a
// hand-built one over decision.MockDecider or decision.None{} in tests)
// and now is the injected clock every suggestion's generated_at uses.
func runCertifySuggest(args []string, ports *decide.Ports, now func() time.Time, stdout, stderr io.Writer) int {
	fs := newFlagSet("certify suggest", stderr)
	service := fs.String("service", "", "service ID (default: resolved from the repo directory name)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	out := fs.String("out", workspacePath("artifacts/certify-suggestions.jsonl"), "suggestions artifact path (regenerated fresh each run, like certify manifest — never appended)")
	limit := fs.Int("limit", 0, "cap the number of unregistered Test* symbols considered (0 = no cap)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if ports == nil || ports.Decider == nil || ports.Decider.Name() == "none" {
		fmt.Fprintln(stdout, "certify suggest: a decision provider is required (set [decide] provider = llm|typesafe in .fathom/config.toml)")
		return 0
	}
	// ADR-044 decision 8 (I14): a second, independent gate a real provider
	// must also clear — "certify_suggest" absent from [decide] sites makes
	// NO call at all, same "print and return 0" shape as the provider-
	// required gate above, naming why instead of what is missing.
	if !decide.SiteAllowed(ports.Cfg, decide.SiteCertifySuggest) {
		fmt.Fprintf(stdout, "certify suggest: site %q not allowed by [decide] sites (no call made)\n", decide.SiteCertifySuggest)
		return 0
	}

	svc := *service
	if svc == "" {
		svc = defaultTestsCoverageService()
		if svc == "" {
			fmt.Fprintln(stderr, "error: certify suggest requires --service (could not guess one from the current directory)")
			return 1
		}
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	ctx := context.Background()

	nodes, err := c.ListAllSymbols(ctx, svc)
	if err != nil {
		fmt.Fprintf(stderr, "error: list symbols for service %q: %v\n", svc, err)
		return 1
	}
	claims, err := c.ClaimsForService(ctx, svc, certify.Layer)
	if err != nil {
		fmt.Fprintf(stderr, "error: certifications layer claims for %q: %v\n", svc, err)
		return 1
	}
	certifying := certifyingTestSymbolKeys(claims)

	corpusRoot := ""
	if meta, found, merr := st.GetServiceMeta(ctx, svc); merr == nil && found {
		corpusRoot = meta.Path
	}

	var candidates []core.SymbolNode
	for _, n := range nodes {
		if n.Kind == core.KindGoFunc && strings.HasPrefix(n.Name, "Test") && !certifying[n.Key()] {
			candidates = append(candidates, n)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].FilePath != candidates[j].FilePath {
			return candidates[i].FilePath < candidates[j].FilePath
		}
		return candidates[i].Line < candidates[j].Line
	})
	if *limit > 0 && len(candidates) > *limit {
		candidates = candidates[:*limit]
	}

	if len(candidates) == 0 {
		fmt.Fprintln(stdout, "certify suggest: no unregistered Test* symbols found")
		return 0
	}

	items := make([]certifySuggestItem, len(candidates))
	for i, n := range candidates {
		items[i] = certifySuggestItem{Node: n, Source: readCertifySuggestSource(corpusRoot, n)}
	}
	chunks := certifySuggestChunks(items)

	var written []certifySuggestion
	var unsure, silent int
	for _, chunk := range chunks {
		req := buildCertifySuggestRequest(chunk)
		callStart := time.Now()
		resp, derr := ports.Decider.Decide(ctx, req)
		if derr != nil {
			ports.Recorder.RecordError("certify_suggest", ports.Decider.Name(), "", len(req.Questions), time.Since(callStart), derr)
			var degraded *decide.DegradedError
			if errors.As(derr, &degraded) {
				// JF-6: the port wrote the one disclosed line; the
				// deterministic path asks no further chunk.
				break
			}
			fmt.Fprintf(stderr, "warn: certify suggest: decide call failed for a %d-test chunk: %v\n", len(chunk), derr)
			continue
		}

		writtenNames := map[string]bool{}
		for _, it := range chunk {
			n := it.Node
			strengthAns, hasStrength := resp.Answers[n.Name+":strength"]
			assertsAns, hasAsserts := resp.Answers[n.Name+":asserts"]
			if !hasStrength || !hasAsserts {
				continue // no answer for one of the pair: the port left it invalid, nothing to render
			}
			strengthProceed := decision.BandOf(strengthAns.Confidence) == decision.BandProceed
			assertsProceed := assertsAns.Noul >= decision.ProceedAtOrAbove
			if strengthProceed && assertsProceed {
				written = append(written, certifySuggestion{
					Test: n.Name, File: n.FilePath, Line: n.Line,
					Strength: strengthAns.Choice, AssertsNoul: assertsAns.Noul,
					Confidence: strengthAns.Confidence, Model: resp.Model,
					GeneratedAt: now().UTC().Format(time.RFC3339),
				})
				writtenNames[n.Name] = true
				continue
			}
			strengthConfirm := decision.BandOf(strengthAns.Confidence) != decision.BandUnsure
			assertsConfirm := assertsAns.Noul >= decision.UnsureBelow
			if strengthConfirm || assertsConfirm {
				unsure++
				fmt.Fprintf(stdout, "unsure: %s (%s:%d)\n", n.Name, n.FilePath, n.Line)
				continue
			}
			silent++
		}
		ports.Recorder.Record("certify_suggest", resp, func(a decision.Answer) bool {
			name, _, ok := strings.Cut(a.ID, ":")
			return ok && writtenNames[name]
		})
	}

	sort.Slice(written, func(i, j int) bool { return written[i].Test < written[j].Test })
	if werr := writeCertifySuggestions(*out, written); werr != nil {
		fmt.Fprintf(stderr, "error: writing suggestions to %q: %v\n", *out, werr)
		return 1
	}
	for _, s := range written {
		fmt.Fprintf(stdout, "suggested: %s may certify something (strength: %s, confidence %.2f, asserts %.2f)\n", s.Test, s.Strength, s.Confidence, s.AssertsNoul)
	}
	fmt.Fprintf(stdout, "certify suggest: %d candidate(s), %d suggestion(s), %d unsure, %d silent\n", len(candidates), len(written), unsure, silent)
	return 0
}

// certifySuggestItem pairs one candidate Test* symbol with its own
// (already 4 KiB-capped) source text, read once and reused for both
// chunk planning and request building.
type certifySuggestItem struct {
	Node   core.SymbolNode
	Source string
}

// readCertifySuggestSource reads n's own [Line, EndLine] source lines
// from corpusRoot (best effort: an unreadable file, or a symbol with no
// usable line range, yields "" rather than failing the whole run — a test
// with no readable source still gets asked about, just with empty
// state for its own portion), capped at certifySuggestMaxSourceBytes.
func readCertifySuggestSource(corpusRoot string, n core.SymbolNode) string {
	path := n.FilePath
	if corpusRoot != "" && !filepath.IsAbs(path) {
		path = filepath.Join(corpusRoot, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	start := n.Line - 1
	if start < 0 {
		start = 0
	}
	end := n.EndLine
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start >= end {
		return ""
	}
	body := strings.Join(lines[start:end], "\n")
	if len(body) > certifySuggestMaxSourceBytes {
		body = body[:certifySuggestMaxSourceBytes] + "\n... [truncated]"
	}
	return body
}

// certifySuggestChunks packs items into requests bounded by
// certifySuggestMaxQuestions (2 per test) and certifySuggestMaxStateBytes
// — see this file's own doc comment on why this is a small local
// equivalent of internal/semantic's decisionBatchPlan rather than a
// direct reuse. Greedy, order-preserving: items arrive pre-sorted
// (file, line), so chunk order and within-chunk order are both
// deterministic.
func certifySuggestChunks(items []certifySuggestItem) [][]certifySuggestItem {
	var chunks [][]certifySuggestItem
	var cur []certifySuggestItem
	curBytes := 0
	for _, it := range items {
		b := len(it.Source)
		exceedsCount := (len(cur)+1)*2 > certifySuggestMaxQuestions
		exceedsBytes := curBytes+b > certifySuggestMaxStateBytes
		if len(cur) > 0 && (exceedsCount || exceedsBytes) {
			chunks = append(chunks, cur)
			cur = nil
			curBytes = 0
		}
		cur = append(cur, it)
		curBytes += b
	}
	if len(cur) > 0 {
		chunks = append(chunks, cur)
	}
	return chunks
}

// buildCertifySuggestRequest renders one chunk's items into a single
// decision.Request: State is every item's labeled source concatenated (so
// the two Questions per test — carrying only the test's bare name in
// their own Instructions/ID — can be answered against the right portion
// of a shared blob, the same "one State, many Questions" shape
// PrefilterSections' own chunked calls use).
func buildCertifySuggestRequest(items []certifySuggestItem) decision.Request {
	var state strings.Builder
	questions := make([]decision.Question, 0, len(items)*2)
	for _, it := range items {
		n := it.Node
		fmt.Fprintf(&state, "=== %s (%s:%d) ===\n%s\n\n", n.Name, n.FilePath, n.Line, it.Source)
		questions = append(questions,
			decision.Question{
				ID:           n.Name + ":strength",
				Kind:         decision.KindChoice,
				Instructions: fmt.Sprintf("For the test function %s (shown above in State), which strength would it provide if linked to a certification target? Pick exactly one.", n.Name),
				Options:      certifyStrengthOptions,
			},
			decision.Question{
				ID:            n.Name + ":asserts",
				Kind:          decision.KindPredicate,
				Instructions:  fmt.Sprintf("%s (shown above in State) asserts a real behaviour or outcome, rather than merely executing code with no meaningful check.", n.Name),
				TrueCriteria:  "the test fails a real assertion (an expected value, error, or state check) when the behaviour is wrong.",
				FalseCriteria: "the test only calls code and checks nothing, or only checks that nothing panicked.",
			},
		)
	}
	return decision.Request{State: state.String(), Questions: questions}
}

// writeCertifySuggestions REPLACES path with suggestions (one JSON object
// per line, sorted by test name for determinism) — regenerated fresh each
// run, like certify manifest, never appended: a suggestion for a test
// that has since been declared (and so is no longer a candidate) simply
// will not reappear the next time this runs. An empty suggestions still
// writes an empty file, distinguishing "ran, found nothing to suggest"
// from "never ran" for loadCertifySuggestions' own caller (certify gaps).
func writeCertifySuggestions(path string, suggestions []certifySuggestion) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", filepath.Dir(path), err)
	}
	var b strings.Builder
	for _, s := range suggestions {
		line, err := json.Marshal(s)
		if err != nil {
			return fmt.Errorf("marshal suggestion for %q: %w", s.Test, err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

// loadCertifySuggestions reads path as newline-delimited certifySuggestion
// JSON — best effort, mirroring loadTestSelectReplay's own discipline: a
// missing file returns nil (never an error — certify suggest may simply
// never have run), a malformed line is skipped rather than aborting the
// whole read.
func loadCertifySuggestions(path string) []certifySuggestion {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []certifySuggestion
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var s certifySuggestion
		if json.Unmarshal([]byte(line), &s) == nil {
			out = append(out, s)
		}
	}
	return out
}
