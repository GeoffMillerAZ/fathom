//go:build decideexp

package decide_test

// exp_live_test.go — EXP-3 (docs/plans/agentic-decisions.md stage 0): E1's
// harness. It selects Go function spans from scratch ingests' call graphs,
// computes each span's truth (internal/ask, EXP-2), draws E1's seeded
// sample per template, asks one batched predicate request per chunk
// through decide.Open, keeps a running spend tally that stops BEFORE the
// chunk whose estimate would pass the cap, and writes aggregates only —
// never a span's text, path or name — to the output file. Everything it
// fixes is pre-registered in docs/bench/readings/decide/exp-preregistration.md
// (EXP-1). Never in the gate: the build tag keeps it out of every
// `go test ./...`, and FATHOM_DECIDE_EXP=1 must be set as well (the DP-37
// precedent, live_smoke_test.go).
//
//	FATHOM_DECIDE_EXP=1 FATHOM_DECIDE_EXP_OUT=<file.json> \
//	FATHOM_DECIDE_EXP_SUBJECTS='fathom|<root>|<store>;websocket|<root>|<store>;bbolt|<root>|<store>' \
//	FATHOM_DECIDE_EXP_OPENROUTER_KEY_FILE=<absolute path, chmod 600> \
//	  go test -tags decideexp -run TestDecideExpE1 -count=1 -timeout 60m -v ./internal/decide
//
// FATHOM_DECIDE_EXP_DRY=1 is the $0 dry mode: the kit's mock decider
// answers (the baseline's answer at 0.95 or 0.05), decide.Open is never
// called, no key is looked at and nothing is dialed; the spend line reads
// what the run would have cost. FATHOM_DECIDE_EXP_CAP_USD overrides the
// pre-registered $2 cap downward only. FATHOM_DECIDE=off (KS-1) closes the
// run before anything is opened. The key: the adapter reads the file
// FATHOM_DECIDE_EXP_OPENROUTER_KEY_FILE names (api_key_file, exactly as the
// live smoke takes it); this harness never reads or prints it.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/ask"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/pkg/client"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// expMaxConsecutiveFailures stops a run whose provider keeps failing.
const expMaxConsecutiveFailures = 3

type expSubject struct {
	Name, Root, Store string
}

func expSubjects(t *testing.T) []expSubject {
	t.Helper()
	raw := os.Getenv("FATHOM_DECIDE_EXP_SUBJECTS")
	if raw == "" {
		t.Fatal("FATHOM_DECIDE_EXP_SUBJECTS must name name|root|store triples, ';'-separated")
	}
	var out []expSubject
	for _, part := range strings.Split(raw, ";") {
		f := strings.Split(strings.TrimSpace(part), "|")
		if len(f) != 3 || f[0] == "" || f[1] == "" || f[2] == "" {
			t.Fatalf("FATHOM_DECIDE_EXP_SUBJECTS entry %q is not name|root|store", part)
		}
		out = append(out, expSubject{Name: f[0], Root: f[1], Store: f[2]})
	}
	return out
}

// expCallGraphSpans is every Go function or method of service that has a
// call edge in or out: the store's typed edges when it has any, else the
// syntax call graph (cmd/indexbench's fathomEdgesForOracle rule).
func expCallGraphSpans(ctx context.Context, sub expSubject) ([]ask.Span, error) {
	c, err := client.NewConstantClient(client.ModeLocal, sub.Store)
	if err != nil {
		return nil, fmt.Errorf("open store %s: %w", sub.Name, err)
	}
	defer func() { _ = c.Close() }()
	nodes, err := c.ListAllSymbols(ctx, sub.Name)
	if err != nil {
		return nil, fmt.Errorf("symbols %s: %w", sub.Name, err)
	}
	edges, err := c.TypedEdges(ctx, sub.Name)
	if err != nil {
		return nil, fmt.Errorf("typed edges %s: %w", sub.Name, err)
	}
	inGraph := map[string]bool{}
	if len(edges) > 0 {
		for _, e := range edges {
			inGraph[e.FromSymbol], inGraph[e.ToSymbol] = true, true
		}
	} else {
		g := callgraph.Build(nodes)
		for _, n := range nodes {
			for _, ci := range callgraph.CalleesDetail(g, n.Key()) {
				inGraph[n.Key()], inGraph[ci.Key] = true, true
			}
		}
	}
	var spans []ask.Span
	for _, n := range nodes {
		if (n.Kind != core.KindGoFunc && n.Kind != core.KindGoMethod) || !strings.HasSuffix(n.FilePath, ".go") || !inGraph[n.Key()] {
			continue
		}
		spans = append(spans, ask.Span{Repo: sub.Name, File: n.FilePath, Name: n.Name, Start: n.Line, End: n.EndLine})
	}
	sort.Slice(spans, func(i, j int) bool {
		a, b := spans[i], spans[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		return a.Name < b.Name
	})
	return spans, nil
}

// openExpDecider is the harness's one call to decide.Open, kept in one
// helper so the kill-switch work (KS-1) adapts it in one place: provider
// typesafe over OpenRouter with the key file path only, as the live smoke
// takes it. killed is true when Open answered with decision.None.
func openExpDecider(keyFile string, disclose *bytes.Buffer) (d decision.Decider, killed bool, err error) {
	ports, err := decide.Open(config.Decide{Provider: "typesafe", Transport: "openrouter", APIKeyFile: keyFile}, nil, "", nil, decide.WithDisclose(disclose))
	if err != nil {
		return nil, false, err
	}
	if _, none := ports.Decider.(decision.None); none {
		return nil, true, nil
	}
	return ports.Decider, false, nil
}

// expTemplateReading is one template's aggregate: counts, the reading,
// latency and spend. It carries no span text, path or name.
type expTemplateReading struct {
	ID              string         `json:"template"`
	Pool            map[string]int `json:"pool"`
	ExcludedTooLong int            `json:"excluded_too_long"`
	Sampled         map[string]int `json:"sampled"`
	TruthTrue       int            `json:"truth_true"`
	Answered        int            `json:"answered"`
	Invalid         int            `json:"invalid"`
	Declined        int            `json:"declined"`
	Failed          int            `json:"failed"`
	Unsent          int            `json:"unsent"`
	Chunks          int            `json:"chunks_sent"`
	Reading         ask.Reading    `json:"reading"`
	LatencyP50MS    float64        `json:"latency_p50_ms"`
	LatencyP95MS    float64        `json:"latency_p95_ms"`
	USD             float64        `json:"usd"`
	USDPer100Spans  float64        `json:"usd_per_100_spans"`
}

type expSubjectReading struct {
	Name       string         `json:"name"`
	Candidates int            `json:"call_graph_spans"`
	Computed   int            `json:"computed"`
	Excluded   map[string]int `json:"excluded"`
	Applicable map[string]int `json:"applicable"`
}

type expReading struct {
	Experiment      string               `json:"experiment"`
	Preregistration string               `json:"preregistration"`
	Dry             bool                 `json:"dry"`
	Decider         string               `json:"decider"`
	Killed          bool                 `json:"killed,omitempty"`
	Seed            uint64               `json:"seed"`
	Quotas          []ask.Quota          `json:"quotas"`
	Budget          ask.Budget           `json:"budget"`
	SpendUSD        float64              `json:"spend_usd"`
	StopReason      string               `json:"stop_reason,omitempty"`
	Subjects        []expSubjectReading  `json:"subjects"`
	Templates       []expTemplateReading `json:"templates"`
	WallSeconds     float64              `json:"wall_seconds"`
}

func expCap(t *testing.T) float64 {
	t.Helper()
	raw := os.Getenv("FATHOM_DECIDE_EXP_CAP_USD")
	if raw == "" {
		return ask.DefaultCapUSD
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 || v > ask.DefaultCapUSD {
		t.Fatalf("FATHOM_DECIDE_EXP_CAP_USD=%q: want a number from 0 to the pre-registered %v", raw, ask.DefaultCapUSD)
	}
	return v
}

func expWrite(t *testing.T, out string, r expReading) {
	t.Helper()
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

// expMockAnswers scripts the dry run's answer to one chunk: the baseline's
// answer at 0.95 when true, 0.05 when false.
func expMockAnswers(qs []decision.Question, baseline []bool) decision.Response {
	answers := make(map[string]decision.Answer, len(qs))
	for i, q := range qs {
		p := 0.05
		if baseline[i] {
			p = 0.95
		}
		answers[q.ID] = decision.Answer{ID: q.ID, Kind: decision.KindPredicate, Noul: p, Confidence: max(p, 1-p)}
	}
	return decision.Response{Provider: "mock", Model: "mock", Answers: answers, Latency: time.Millisecond}
}

func TestDecideExpE1(t *testing.T) {
	if os.Getenv("FATHOM_DECIDE_EXP") != "1" {
		t.Skip("E1 harness: set FATHOM_DECIDE_EXP=1 (stage 0, at most $2; FATHOM_DECIDE_EXP_DRY=1 for the $0 dry run)")
	}
	out := os.Getenv("FATHOM_DECIDE_EXP_OUT")
	if out == "" {
		t.Fatal("FATHOM_DECIDE_EXP_OUT must name the JSON file the aggregates go to")
	}
	dry := os.Getenv("FATHOM_DECIDE_EXP_DRY") == "1"
	start := time.Now()
	reading := expReading{
		Experiment:      "E1 (EXP-3 harness)",
		Preregistration: "docs/bench/readings/decide/exp-preregistration.md",
		Dry:             dry,
		Seed:            ask.SampleSeed,
		Quotas:          ask.E1Quotas,
		Budget:          ask.Budget{CapUSD: expCap(t), Price: ask.ListedPrice},
	}
	if strings.EqualFold(os.Getenv("FATHOM_DECIDE"), "off") {
		reading.Killed, reading.StopReason, reading.Decider = true, "FATHOM_DECIDE=off", "none"
		expWrite(t, out, reading)
		t.Skip("FATHOM_DECIDE=off: the port is closed; nothing was opened or asked")
	}
	subjects := expSubjects(t)

	var decider decision.Decider
	var mock *decision.MockDecider
	var disclose bytes.Buffer
	if dry {
		mock = decision.NewMockDecider("mock")
		mock.SetValidate(true) // refuse what every shipped adapter refuses
		decider = mock
	} else {
		d, killed, err := openExpDecider(os.Getenv("FATHOM_DECIDE_EXP_OPENROUTER_KEY_FILE"), &disclose)
		if err != nil {
			t.Fatalf("decide.Open: %v", err)
		}
		if killed {
			reading.Killed, reading.StopReason, reading.Decider = true, "decide.Open answered decision.None (a kill holds)", "none"
			expWrite(t, out, reading)
			t.Skip("the port answered decision.None: nothing asked")
		}
		decider = d
	}
	reading.Decider = decider.Name()

	ctx := context.Background()
	var computed []ask.Computed
	for _, sub := range subjects {
		spans, err := expCallGraphSpans(ctx, sub)
		if err != nil {
			t.Fatal(err)
		}
		cs, err := ask.Compute(ctx, sub.Root, spans, nil)
		if err != nil {
			t.Fatal(err)
		}
		sr := expSubjectReading{Name: sub.Name, Candidates: len(spans), Excluded: map[string]int{}, Applicable: map[string]int{}}
		for _, c := range cs {
			if c.Excluded != "" {
				sr.Excluded[c.Excluded]++
				continue
			}
			sr.Computed++
			for _, tmpl := range ask.Templates {
				if applies, _ := tmpl.Truth(c.Facts); applies {
					sr.Applicable[tmpl.ID]++
				}
			}
		}
		reading.Subjects = append(reading.Subjects, sr)
		computed = append(computed, cs...)
		t.Logf("%s: %d call-graph spans, %d computed, excluded %v", sub.Name, len(spans), sr.Computed, sr.Excluded)
	}

	failures := 0
	for _, tmpl := range ask.Templates {
		pool, tooLong := ask.Pool(tmpl, computed)
		sample := ask.Sample(tmpl, pool, ask.E1Quotas, ask.SampleSeed)
		tr := expTemplateReading{ID: tmpl.ID, Pool: map[string]int{}, Sampled: map[string]int{}, ExcludedTooLong: tooLong}
		for _, c := range pool {
			tr.Pool[c.Span.Repo]++
		}
		texts := make([]string, len(sample))
		sizes := make([]int, len(sample))
		truth := make([]bool, len(sample))
		base := make([]bool, len(sample))
		for i, c := range sample {
			tr.Sampled[c.Span.Repo]++
			texts[i] = ask.QuestionText(tmpl, c)
			sizes[i] = len(texts[i])
			_, truth[i] = tmpl.Truth(c.Facts)
			base[i] = tmpl.BaselineAnswer(c.Source)
			if truth[i] {
				tr.TruthTrue++
			}
		}
		var items []ask.Item
		var latencies []float64
		for _, chunk := range ask.Chunks(sizes, len(ask.RequestState), ask.ChunkQuestions, ask.ChunkBytes) {
			qs := make([]decision.Question, len(chunk))
			bl := make([]bool, len(chunk))
			bytesIn := len(ask.RequestState)
			for j, i := range chunk {
				qs[j] = decision.Question{ID: fmt.Sprintf("q%03d", j), Kind: decision.KindPredicate, Instructions: texts[i]}
				bl[j] = base[i]
				bytesIn += sizes[i]
			}
			estIn, estOut := ask.EstimateTokens(bytesIn), ask.OutputTokensPerQuestion*len(chunk)
			if failures >= expMaxConsecutiveFailures {
				reading.StopReason = fmt.Sprintf("%d consecutive failed requests", failures)
				tr.Unsent += len(chunk)
				continue
			}
			if !reading.Budget.Admit(estIn, estOut) {
				if reading.StopReason == "" {
					reading.StopReason = fmt.Sprintf("cap: the next chunk's estimate would pass $%g", reading.Budget.CapUSD)
				}
				tr.Unsent += len(chunk)
				continue
			}
			if mock != nil {
				mock.AddEntry(decision.MockEntry{Response: expMockAnswers(qs, bl)})
			}
			callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
			sent := time.Now()
			resp, err := decider.Decide(callCtx, decision.Request{State: ask.RequestState, Questions: qs})
			wall := time.Since(sent)
			cancel()
			tr.Chunks++
			tr.USD += reading.Budget.Charge(resp.Usage.InputTokens, resp.Usage.OutputTokens, estIn, estOut)
			if err != nil {
				failures++
				tr.Failed += len(chunk)
				t.Logf("%s chunk of %d: %v", tmpl.ID, len(chunk), err)
				continue
			}
			failures = 0
			lat := resp.Latency
			if lat <= 0 {
				lat = wall
			}
			latencies = append(latencies, float64(lat.Milliseconds()))
			for j, i := range chunk {
				id := qs[j].ID
				if _, bad := resp.Invalid[id]; bad {
					tr.Invalid++
					continue
				}
				if _, no := resp.Declined[id]; no {
					tr.Declined++
					continue
				}
				a, ok := resp.Answers[id]
				if !ok {
					tr.Invalid++
					continue
				}
				tr.Answered++
				items = append(items, ask.Item{P: a.Noul, Truth: truth[i], Baseline: base[i]})
			}
		}
		tr.Reading = ask.Read(items)
		tr.LatencyP50MS, tr.LatencyP95MS = ask.Percentile(latencies, 0.5), ask.Percentile(latencies, 0.95)
		if n := len(sample) - tr.Unsent; n > 0 {
			tr.USDPer100Spans = tr.USD / float64(n) * 100
		}
		reading.Templates = append(reading.Templates, tr)
		t.Logf("%s: pool %v, sampled %v, answered %d, qualifies %v (%s)", tmpl.ID, tr.Pool, tr.Sampled, tr.Answered, tr.Reading.Qualifies, tr.Reading.QualifyingBand)
	}
	reading.SpendUSD = reading.Budget.SpentUSD
	if dry {
		reading.SpendUSD = 0
	}
	reading.WallSeconds = time.Since(start).Seconds()
	expWrite(t, out, reading)
	t.Logf("spend $%.4f (budget tally $%.4f of $%g, %d calls: %d reported, %d estimated); stop: %q; written to %s",
		reading.SpendUSD, reading.Budget.SpentUSD, reading.Budget.CapUSD, reading.Budget.Calls, reading.Budget.Reported, reading.Budget.Estimated, reading.StopReason, out)
}
