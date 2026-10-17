//go:build decideexp

package decide_test

// exp_e2_test.go — E2's harness (EXP-M2; docs/bench/readings/decide/exp-preregistration.md
// section 8): questions with no computable truth, read against labels a
// Claude reviewer wrote before any E2 call (provenance frontier), with the
// owner's spot-check (provenance human) recorded only when one is given.
//
//	FATHOM_DECIDE_EXP=1 FATHOM_DECIDE_EXP_OUT=<scratch>/exp-e2.json \
//	FATHOM_DECIDE_EXP_E2_ITEMS=<scratch>/e2a-items.jsonl:<scratch>/e2b-items.jsonl \
//	FATHOM_DECIDE_EXP_E2_LABELS=<scratch>/e2-labels.jsonl \
//	FATHOM_DECIDE_EXP_CAP_USD=<$2 less the spend E1 recorded> \
//	FATHOM_DECIDE_EXP_OPENROUTER_KEY_FILE=<absolute path, chmod 600> \
//	  go test -tags decideexp -run 'TestDecideExpE2$' -count=1 -timeout 30m -v ./internal/decide
//
// FATHOM_DECIDE_EXP_E2_ITEMS is one JSONL file or several joined by the
// path-list separator. An item's id starts e2a- (the caller question:
// {id, function, file, caller, caller_file, diff, caller_src}, the diff at
// most 80 lines; exp_e2items_test.go builds them) or e2b- (the failure
// question: {id, test, file, output, test_src}, the output at most 60
// lines). FATHOM_DECIDE_EXP_E2_LABELS is JSONL {id, label, provenance,
// note?}: label is "true" or "false" for e2a and one of the five options for
// e2b; provenance is frontier or human; note is the reviewer's and is never
// sent or written. Labels before any call: every input is read and checked
// first, and the run REFUSES (fails before anything is opened, no key looked
// at) unless every item has exactly one frontier label, no label names an
// unknown item and every value is one its question takes.
//
// Fixed here, before any E2 call (K19): the State and both questions are
// the page's words verbatim; e2b's options go in the page's order with the
// page's parenthesis as each option's criteria; each question runs in item
// order, E2a first, one batched request per chunk (E1's bounds: 64
// questions, 96 KiB); the model's e2a answer is true at Noul >= 0.5 and its
// band is Noul (E1's), e2b's band is the chosen option's probability; the
// baseline is the majority frontier label within the question, a tie going
// to the earlier answer (true before false, then the options' order). The
// dry mode (FATHOM_DECIDE_EXP_DRY=1, $0) answers the majority label at 0.95
// through the kit's validating mock. The output holds aggregates only and
// the two input files' sha256: never an item's text, id, path or name.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/ask"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// expE2State is every E2 request's State, verbatim from section 8.
const expE2State = "Each question below is about one Go change or one Go test failure, shown in the question. Answer each independently."

// The two questions' statements, verbatim from section 8.
const (
	expE2aStatement = "This caller depends on the behaviour the diff changes: for some input, what the caller returns or does would differ because of the change."
	expE2bStatement = "What kind of failure is this Go test failure?"
)

// expE2bOptions are F35's five options in the page's order, each with the
// page's parenthesis as its criteria.
var expE2bOptions = []decision.Option{
	{Name: "code_bug", Criteria: "the code under test is wrong and the test is right"},
	{Name: "environment", Criteria: "the machine, a tool, a file, a permission or the network, not the code"},
	{Name: "timing", Criteria: "a deadline, an ordering or a race: the same code can pass on another run"},
	{Name: "outdated_expectation", Criteria: "the code changed on purpose and the test still expects the old behaviour"},
	{Name: "other", Criteria: "none of these"},
}

// expE2aAnswers are the caller question's answers in tie-break order.
var expE2aAnswers = []string{"true", "false"}

// Section 8's bounds on what an item shows.
const (
	expE2aMaxDiffLines   = 80
	expE2bMaxOutputLines = 60
)

// expE2Item is one E2 item: the caller question's fields, then the failure
// question's (file is the changed function's file for e2a, the failing
// test's for e2b). The field order is the JSONL's.
type expE2Item struct {
	ID         string `json:"id"`
	Function   string `json:"function,omitempty"`
	File       string `json:"file,omitempty"`
	Caller     string `json:"caller,omitempty"`
	CallerFile string `json:"caller_file,omitempty"`
	Diff       string `json:"diff,omitempty"`
	CallerSrc  string `json:"caller_src,omitempty"`
	Test       string `json:"test,omitempty"`
	Output     string `json:"output,omitempty"`
	TestSrc    string `json:"test_src,omitempty"`
}

// question is "e2a" or "e2b" by the id's prefix, "" for neither.
func (it expE2Item) question() string {
	switch {
	case strings.HasPrefix(it.ID, "e2a-"):
		return "e2a"
	case strings.HasPrefix(it.ID, "e2b-"):
		return "e2b"
	}
	return ""
}

// expE2Label is one reviewer label.
type expE2Label struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Provenance string `json:"provenance"`
	Note       string `json:"note,omitempty"`
}

func expLineCount(s string) int { return strings.Count(strings.TrimSuffix(s, "\n"), "\n") + 1 }

// check is the item's own refusal: a missing field or a text past the
// page's bound.
func (it expE2Item) check() error {
	missing := func(fields map[string]string) error {
		var names []string
		for k, v := range fields {
			if strings.TrimSpace(v) == "" {
				names = append(names, k)
			}
		}
		sort.Strings(names)
		if len(names) > 0 {
			return fmt.Errorf("item %s: missing %s", it.ID, strings.Join(names, ", "))
		}
		return nil
	}
	switch it.question() {
	case "e2a":
		if err := missing(map[string]string{"function": it.Function, "file": it.File, "caller": it.Caller, "caller_file": it.CallerFile, "diff": it.Diff, "caller_src": it.CallerSrc}); err != nil {
			return err
		}
		if it.Test != "" || it.Output != "" || it.TestSrc != "" {
			return fmt.Errorf("item %s: an e2a item carries no test, output or test_src", it.ID)
		}
		if n := expLineCount(it.Diff); n > expE2aMaxDiffLines {
			return fmt.Errorf("item %s: diff of %d lines, at most %d", it.ID, n, expE2aMaxDiffLines)
		}
	case "e2b":
		if err := missing(map[string]string{"test": it.Test, "file": it.File, "output": it.Output, "test_src": it.TestSrc}); err != nil {
			return err
		}
		if it.Function != "" || it.Caller != "" || it.CallerFile != "" || it.Diff != "" || it.CallerSrc != "" {
			return fmt.Errorf("item %s: an e2b item carries no function, caller, caller_file, diff or caller_src", it.ID)
		}
		if n := expLineCount(it.Output); n > expE2bMaxOutputLines {
			return fmt.Errorf("item %s: output of %d lines, at most %d", it.ID, n, expE2bMaxOutputLines)
		}
	default:
		return fmt.Errorf("item %q: an id starts e2a- or e2b-", it.ID)
	}
	return nil
}

// expE2Answers is the answers question q takes, in tie-break order.
func expE2Answers(q string) []string {
	if q == "e2a" {
		return expE2aAnswers
	}
	out := make([]string, len(expE2bOptions))
	for i, o := range expE2bOptions {
		out[i] = o.Name
	}
	return out
}

// expE2Text is the Instructions sent for it.
func expE2Text(it expE2Item) string {
	var b strings.Builder
	if it.question() == "e2a" {
		b.WriteString(expE2aStatement)
		fmt.Fprintf(&b, "\n\nChanged function %s in %s, the diff of the last commit that changed it:\n```diff\n%s\n```", it.Function, it.File, strings.TrimSuffix(it.Diff, "\n"))
		fmt.Fprintf(&b, "\n\nCaller %s in %s, as it stands now:\n```go\n%s\n```", it.Caller, it.CallerFile, strings.TrimSuffix(it.CallerSrc, "\n"))
		return b.String()
	}
	b.WriteString(expE2bStatement)
	fmt.Fprintf(&b, "\n\nTest %s in %s, its output:\n```text\n%s\n```", it.Test, it.File, strings.TrimSuffix(it.Output, "\n"))
	fmt.Fprintf(&b, "\n\nThe test function:\n```go\n%s\n```", strings.TrimSuffix(it.TestSrc, "\n"))
	return b.String()
}

// expReadJSONL decodes every non-blank line of path into a fresh T, unknown
// fields refused, and returns the file's sha256.
func expReadJSONL[T any](path string) ([]T, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(data)
	var out []T
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		var v T
		if err := dec.Decode(&v); err != nil {
			return nil, "", fmt.Errorf("%s line %d: %w", filepath.Base(path), n, err)
		}
		out = append(out, v)
	}
	if err := sc.Err(); err != nil {
		return nil, "", err
	}
	return out, hex.EncodeToString(sum[:]), nil
}

// expE2Inputs is E2's checked input: the items in order and, per item id,
// its frontier label and (when the owner gave one) its human label.
type expE2Inputs struct {
	Items    []expE2Item
	Frontier map[string]string
	Human    map[string]string
	Hashes   []string // sha256 of each items file, then of the labels file
}

// expE2Load reads and checks E2's inputs; any error is a refusal, and it
// runs before anything is opened.
func expE2Load(itemPaths []string, labelPath string) (expE2Inputs, error) {
	var in expE2Inputs
	byID := map[string]expE2Item{}
	for _, p := range itemPaths {
		items, sum, err := expReadJSONL[expE2Item](p)
		if err != nil {
			return in, err
		}
		in.Hashes = append(in.Hashes, sum)
		for _, it := range items {
			if err := it.check(); err != nil {
				return in, err
			}
			if _, dup := byID[it.ID]; dup {
				return in, fmt.Errorf("item %s appears twice", it.ID)
			}
			byID[it.ID] = it
			in.Items = append(in.Items, it)
		}
	}
	if len(in.Items) == 0 {
		return in, fmt.Errorf("no items in %v", itemPaths)
	}
	labels, sum, err := expReadJSONL[expE2Label](labelPath)
	if err != nil {
		return in, err
	}
	in.Hashes = append(in.Hashes, sum)
	in.Frontier, in.Human = map[string]string{}, map[string]string{}
	for _, l := range labels {
		it, ok := byID[l.ID]
		if !ok {
			return in, fmt.Errorf("label for %q names no item", l.ID)
		}
		valid := false
		for _, a := range expE2Answers(it.question()) {
			valid = valid || a == l.Label
		}
		if !valid {
			return in, fmt.Errorf("label for %s is %q; %s takes %s", l.ID, l.Label, it.question(), strings.Join(expE2Answers(it.question()), ", "))
		}
		var into map[string]string
		switch l.Provenance {
		case "frontier":
			into = in.Frontier
		case "human":
			into = in.Human
		default:
			return in, fmt.Errorf("label for %s has provenance %q; want frontier or human", l.ID, l.Provenance)
		}
		if _, dup := into[l.ID]; dup {
			return in, fmt.Errorf("two %s labels for %s", l.Provenance, l.ID)
		}
		into[l.ID] = l.Label
	}
	var unlabelled []string
	for _, it := range in.Items {
		if _, ok := in.Frontier[it.ID]; !ok {
			unlabelled = append(unlabelled, it.ID)
		}
	}
	if len(unlabelled) > 0 {
		return in, fmt.Errorf("%d of %d items have no frontier label (first: %s); labels come before any call", len(unlabelled), len(in.Items), unlabelled[0])
	}
	return in, nil
}

// expE2Human is the owner's spot-check, when given: how often the frontier
// label and the model's answer agree with it.
type expE2Human struct {
	N                 int     `json:"n"`
	FrontierAgreement float64 `json:"frontier_agreement_with_human"`
	ModelAnswered     int     `json:"model_answered"`
	ModelAgreement    float64 `json:"model_agreement_with_human"`
}

// expE2QuestionReading is one question's aggregate: counts, the agreement
// reading against the frontier labels, latency and spend. No item's text,
// id, path or name.
type expE2QuestionReading struct {
	Question       string               `json:"question"`
	Kind           decision.Kind        `json:"kind"`
	Items          int                  `json:"items"`
	FrontierLabels map[string]int       `json:"frontier_labels"`
	Majority       string               `json:"majority_label_baseline"`
	Answered       int                  `json:"answered"`
	Invalid        int                  `json:"invalid"`
	Declined       int                  `json:"declined"`
	Failed         int                  `json:"failed"`
	Unsent         int                  `json:"unsent"`
	Chunks         int                  `json:"chunks_sent"`
	ModelAnswers   map[string]int       `json:"model_answers"`
	Reading        ask.AgreementReading `json:"agreement_with_frontier_labels"`
	AUROC          float64              `json:"auroc_against_frontier_labels,omitempty"`
	AUROCDefined   bool                 `json:"auroc_defined"`
	Human          *expE2Human          `json:"human_spot_check,omitempty"`
	LatencyP50MS   float64              `json:"latency_p50_ms"`
	LatencyP95MS   float64              `json:"latency_p95_ms"`
	USD            float64              `json:"usd"`
	USDPer100Items float64              `json:"usd_per_100_items"`
}

type expE2Reading struct {
	Experiment      string                 `json:"experiment"`
	Preregistration string                 `json:"preregistration"`
	Dry             bool                   `json:"dry"`
	Decider         string                 `json:"decider"`
	Killed          bool                   `json:"killed,omitempty"`
	InputSHA256     []string               `json:"input_sha256"`
	Budget          ask.Budget             `json:"budget"`
	SpendUSD        float64                `json:"spend_usd"`
	StopReason      string                 `json:"stop_reason,omitempty"`
	Questions       []expE2QuestionReading `json:"questions"`
	WallSeconds     float64                `json:"wall_seconds"`
}

func expWriteJSON(t *testing.T, out string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

// expE2Judged is one answered item: the model's answer and its band
// probability.
type expE2Judged struct {
	answer string
	p      float64
}

// expE2Answer reads q's answer for item it out of resp: answered, or the
// miss it counts as (invalid, declined).
func expE2Answer(resp decision.Response, id string, choice bool) (expE2Judged, string) {
	if _, bad := resp.Invalid[id]; bad {
		return expE2Judged{}, "invalid"
	}
	if _, no := resp.Declined[id]; no {
		return expE2Judged{}, "declined"
	}
	a, ok := resp.Answers[id]
	if !ok {
		return expE2Judged{}, "invalid"
	}
	if !choice {
		ans := "false"
		if a.Noul >= 0.5 {
			ans = "true"
		}
		return expE2Judged{answer: ans, p: a.Noul}, ""
	}
	p, ok := a.Probabilities[a.Choice]
	if !ok {
		return expE2Judged{}, "invalid"
	}
	return expE2Judged{answer: a.Choice, p: p}, ""
}

// expE2Run asks q's items (in order) and reads them.
func expE2Run(ctx context.Context, t *testing.T, a *expAsker, q string, items []expE2Item, in expE2Inputs) expE2QuestionReading {
	choice := q == "e2b"
	r := expE2QuestionReading{Question: q, Kind: decision.KindPredicate, Items: len(items), FrontierLabels: map[string]int{}, ModelAnswers: map[string]int{}}
	if choice {
		r.Kind = decision.KindChoice
	}
	labels := make([]string, len(items))
	for i, it := range items {
		labels[i] = in.Frontier[it.ID]
		r.FrontierLabels[labels[i]]++
	}
	r.Majority = ask.Majority(labels, expE2Answers(q))
	texts := make([]string, len(items))
	sizes := make([]int, len(items))
	for i, it := range items {
		texts[i] = expE2Text(it)
		sizes[i] = len(texts[i])
		if choice {
			sizes[i] += expRequestBytes("", []decision.Question{{Options: expE2bOptions}})
		}
	}
	judged := map[string]expE2Judged{}
	var latencies []float64
	for _, chunk := range ask.Chunks(sizes, len(expE2State), ask.ChunkQuestions, ask.ChunkBytes) {
		qs := make([]decision.Question, len(chunk))
		for j, i := range chunk {
			qs[j] = decision.Question{ID: fmt.Sprintf("q%03d", j), Kind: r.Kind, Instructions: texts[i]}
			if choice {
				qs[j].Options = expE2bOptions
			}
		}
		dry := func() decision.Response {
			answers := make(map[string]decision.Answer, len(qs))
			for _, qq := range qs {
				if choice {
					answers[qq.ID] = expMockChoice(qq.ID, expE2bOptions, r.Majority)
				} else {
					p := 0.05
					if r.Majority == "true" {
						p = 0.95
					}
					answers[qq.ID] = expMockPredicate(qq.ID, p)
				}
			}
			return decision.Response{Provider: "mock", Model: "mock", Answers: answers, Latency: time.Millisecond}
		}
		sent := a.send(ctx, expE2State, qs, dry)
		if !sent.Sent {
			r.Unsent += len(chunk)
			continue
		}
		r.Chunks++
		r.USD += sent.USD
		if sent.Err != nil {
			r.Failed += len(chunk)
			t.Logf("%s request of %d: %v", q, len(chunk), sent.Err)
			continue
		}
		latencies = append(latencies, float64(sent.Latency.Milliseconds()))
		for j, i := range chunk {
			got, miss := expE2Answer(sent.Resp, qs[j].ID, choice)
			switch miss {
			case "invalid":
				r.Invalid++
			case "declined":
				r.Declined++
			default:
				r.Answered++
				r.ModelAnswers[got.answer]++
				judged[items[i].ID] = got
			}
		}
	}
	var agree []ask.AgreementItem
	var scored []ask.Item
	var human expE2Human
	for i, it := range items {
		if h, ok := in.Human[it.ID]; ok {
			human.N++
			if h == labels[i] {
				human.FrontierAgreement++
			}
		}
		got, ok := judged[it.ID]
		if !ok {
			continue
		}
		agree = append(agree, ask.AgreementItem{P: got.p, Agrees: got.answer == labels[i], BaselineAgrees: r.Majority == labels[i]})
		if !choice {
			scored = append(scored, ask.Item{P: got.p, Truth: labels[i] == "true", Baseline: r.Majority == "true"})
		}
		if h, ok := in.Human[it.ID]; ok {
			human.ModelAnswered++
			if got.answer == h {
				human.ModelAgreement++
			}
		}
	}
	r.Reading = ask.ReadAgreement(agree)
	if !choice {
		r.AUROC, r.AUROCDefined = ask.AUROC(scored)
	}
	if human.N > 0 {
		human.FrontierAgreement /= float64(human.N)
		if human.ModelAnswered > 0 {
			human.ModelAgreement /= float64(human.ModelAnswered)
		}
		r.Human = &human
	}
	r.LatencyP50MS, r.LatencyP95MS = ask.Percentile(latencies, 0.5), ask.Percentile(latencies, 0.95)
	if n := len(items) - r.Unsent; n > 0 {
		r.USDPer100Items = r.USD / float64(n) * 100
	}
	return r
}

func TestDecideExpE2(t *testing.T) {
	if os.Getenv("FATHOM_DECIDE_EXP") != "1" {
		t.Skip("E2 harness: set FATHOM_DECIDE_EXP=1 (stage 0, the cap less E1's spend; FATHOM_DECIDE_EXP_DRY=1 for the $0 dry run)")
	}
	out := os.Getenv("FATHOM_DECIDE_EXP_OUT")
	if out == "" {
		t.Fatal("FATHOM_DECIDE_EXP_OUT must name the JSON file the aggregates go to")
	}
	itemPaths := filepath.SplitList(os.Getenv("FATHOM_DECIDE_EXP_E2_ITEMS"))
	labelPath := os.Getenv("FATHOM_DECIDE_EXP_E2_LABELS")
	if len(itemPaths) == 0 || labelPath == "" {
		t.Fatal("FATHOM_DECIDE_EXP_E2_ITEMS (JSONL, path-list separated) and FATHOM_DECIDE_EXP_E2_LABELS (JSONL) are both required")
	}
	// Labels before any call: nothing is opened, and no key is looked at,
	// until every item has its frontier label.
	in, err := expE2Load(itemPaths, labelPath)
	if err != nil {
		t.Fatalf("refused before any call: %v", err)
	}
	dry := os.Getenv("FATHOM_DECIDE_EXP_DRY") == "1"
	start := time.Now()
	reading := expE2Reading{
		Experiment:      "E2 (EXP-M2)",
		Preregistration: "docs/bench/readings/decide/exp-preregistration.md",
		Dry:             dry,
		InputSHA256:     in.Hashes,
	}
	if reason := expKillReason(); reason != "" {
		reading.Killed, reading.StopReason, reading.Decider = true, reason, "none"
		expWriteJSON(t, out, reading)
		t.Skip("FATHOM_DECIDE=off: the port is closed; nothing was opened or asked")
	}
	a := newExpAsker(t, dry)
	reading.Budget = a.budget
	if a.killed != "" {
		reading.Killed, reading.StopReason, reading.Decider = true, a.killed, "none"
		expWriteJSON(t, out, reading)
		t.Skip("the port answered decision.None: nothing asked")
	}
	reading.Decider = a.name()
	ctx := context.Background()
	for _, q := range []string{"e2a", "e2b"} {
		var items []expE2Item
		for _, it := range in.Items {
			if it.question() == q {
				items = append(items, it)
			}
		}
		if len(items) == 0 {
			continue
		}
		qr := expE2Run(ctx, t, a, q, items, in)
		reading.Questions = append(reading.Questions, qr)
		t.Logf("%s: %d items, answered %d, agreement %.3f vs majority %q %.3f, qualifies %v (%s)",
			q, qr.Items, qr.Answered, qr.Reading.Agreement, qr.Majority, qr.Reading.BaselineAgreement, qr.Reading.Qualifies, qr.Reading.QualifyingBand)
	}
	reading.Budget, reading.StopReason, reading.SpendUSD = a.budget, a.stop, a.spend()
	reading.WallSeconds = time.Since(start).Seconds()
	expWriteJSON(t, out, reading)
	t.Logf("spend $%.4f (budget tally $%.4f of $%g, %d calls: %d reported, %d estimated); stop: %q; written to %s",
		reading.SpendUSD, a.budget.SpentUSD, a.budget.CapUSD, a.budget.Calls, a.budget.Reported, a.budget.Estimated, reading.StopReason, out)
}
