package main

// localizereplay.go — TL-7, the TL-1 instrument (docs/plans/telemetry-links.md,
// ADR-062): `indexbench localize-replay` runs the fault-injected fixture apps
// under calibration/incidents/<lang>/ natively, reads each case's truth from
// the runtime's own structured trace (tier A: the innermost frame inside the
// app, recorded by the app's capture runner), scratch-ingests each app ONE at
// a time, and asks the MCP localize tool, as an agent would, for every case
// in every payload shape the runner captured (the raw trace, Bugsnag, OTLP,
// Lambda, New Relic). A case is correct when localize's top candidate is the
// truth's file and its symbol is the injected function. It spends no tokens
// and talks to no daemon.
//
// Usage:
//
//	indexbench localize-replay -work <scratch dir> [-corpus calibration/incidents]
//	    [-langs java,node,php,python] [-arm after] [-baseline <arm.json>] [-out <file>]
//	    [-java <bin>] [-node <bin>] [-php <bin>] [-python <bin>]
//
// The baseline arm is the same harness built against the localize of the
// commit before TL-1; -baseline embeds that arm's file beside this one.
//
// -logs is TL-4's log-line shape (F67): each app's log capture runner
// (incident.json's log_args, its log_env) logs one line per case through the
// real logging libraries, which calibration/incidents/install-log-deps.sh
// installs under -deps; the truth is the call site the runtime reports, the
// rendered line is pasted as localize's `error`, and a case is correct when
// the top candidate is the logger call's enclosing symbol. Every case logs a
// value carrying replayLogSentinel; an answer that matched a template and
// carries it counts as an echo (the reading's echoes, which must be 0).

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// incidentManifest is calibration/incidents/<lang>/incident.json.
type incidentManifest struct {
	Service string   `json:"service"`
	App     string   `json:"app"`
	Runtime string   `json:"runtime"`
	Args    []string `json:"args"`
	About   string   `json:"about,omitempty"`
	// LogArgs and LogEnv run the log capture runner (-logs); "{deps}" in
	// either is the language's directory under -deps.
	LogArgs []string          `json:"log_args,omitempty"`
	LogEnv  map[string]string `json:"log_env,omitempty"`
}

// incidentCapture is one line of a capture runner's output.
type incidentCapture struct {
	Case     string            `json:"case"`
	Lang     string            `json:"lang"`
	API      string            `json:"api,omitempty"`
	Truth    incidentTruth     `json:"truth"`
	Payloads map[string]string `json:"payloads"`
}

// replayLogShape is the log capture runners' one shape: the rendered line,
// pasted as localize's `error` (every other shape is a `stack_trace`).
const replayLogShape = "log"

// replayLogSentinel is in a placeholder value of every logged case.
const replayLogSentinel = "SNTL"

type incidentTruth struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Func string `json:"func"`
	Name string `json:"name"`
}

// replayCase is one (case, shape) answer. In the reading it is one row,
// [case, shape, top_file, top_symbol, rank] (replayMissColumns).
type replayCase struct {
	Case      string
	Shape     string
	TopFile   string
	TopSymbol string
	Correct   bool
	// Rank is the 1-based position of the first correct candidate, 0 when
	// localize did not list it.
	Rank int
}

// replayMissColumns names a miss row's columns.
var replayMissColumns = []string{"case", "shape", "top_file", "top_symbol", "rank"}

func (c replayCase) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{c.Case, c.Shape, c.TopFile, c.TopSymbol, c.Rank})
}

func (c *replayCase) UnmarshalJSON(b []byte) error {
	var row []any
	if err := json.Unmarshal(b, &row); err != nil {
		return err
	}
	if len(row) != len(replayMissColumns) {
		return fmt.Errorf("miss row has %d columns, want %d", len(row), len(replayMissColumns))
	}
	c.Case, _ = row[0].(string)
	c.Shape, _ = row[1].(string)
	c.TopFile, _ = row[2].(string)
	c.TopSymbol, _ = row[3].(string)
	if r, ok := row[4].(float64); ok {
		c.Rank = int(r)
	}
	return nil
}

// replayShape is one shape's tally.
type replayShape struct {
	N       int     `json:"n"`
	Correct int     `json:"correct"`
	Top1    float64 `json:"top1"`
}

// replayLang is one language's reading.
type replayLang struct {
	Service string                 `json:"service"`
	Cases   int                    `json:"cases"`
	Answers int                    `json:"answers"`
	Correct int                    `json:"correct"`
	Top1    float64                `json:"top1"`
	ByShape map[string]replayShape `json:"by_shape"`
	Truths  []incidentTruthRow     `json:"truths"`
	// Misses lists every answer whose top candidate was not the truth.
	Misses   []replayCase `json:"misses"`
	Symbols  int          `json:"symbols"`
	Ingested string       `json:"ingest_wall"`
	// ByAPI tallies log-line answers by the logger API that wrote the line
	// (-logs); Echoes counts answers that matched a template and still
	// carry the sentinel (it gates the verdict); SentinelAnswers counts every
	// answer carrying it, matched or not (context: a quoted value grepped
	// as a literal, before TL-4).
	ByAPI           map[string]replayShape `json:"by_api,omitempty"`
	Echoes          *int                   `json:"echoes,omitempty"`
	SentinelAnswers *int                   `json:"sentinel_answers,omitempty"`
}

type incidentTruthRow struct {
	Case string `json:"case"`
	incidentTruth
}

// replayArm is one arm of the reading.
type replayArm struct {
	Arm     string                 `json:"arm"`
	Commit  string                 `json:"commit,omitempty"`
	Answers int                    `json:"answers"`
	Correct int                    `json:"correct"`
	Top1    float64                `json:"top1"`
	ByShape map[string]replayShape `json:"by_shape"`
	ByAPI   map[string]replayShape `json:"by_api,omitempty"`
	Langs   map[string]replayLang  `json:"langs"`
	Echoes  *int                   `json:"echoes,omitempty"`
	// SentinelAnswers is the languages' sum (-logs).
	SentinelAnswers *int `json:"sentinel_answers,omitempty"`
}

// replayReading is docs/bench/readings/telemetry/tl1-replay.json.
type replayReading struct {
	Reading    string `json:"reading"`
	Row        string `json:"row"`
	Acceptance string `json:"acceptance"`
	Truth      string `json:"truth"`
	// Verdict is the last arm's against the acceptance: PASS when every
	// language and the whole replay reach replayBar, BELOW otherwise (a
	// written ceiling is then owed).
	Verdict     string      `json:"verdict"`
	MissColumns []string    `json:"miss_columns"`
	Arms        []replayArm `json:"arms"`
}

// replayBar is TL-7's acceptance: top-1 per language and overall.
const replayBar = 0.90

// replayVerdict reads an arm against replayBar; an echo fails it outright.
func replayVerdict(a replayArm) string {
	if a.Answers == 0 || a.Top1 < replayBar || a.Echoes != nil && *a.Echoes > 0 {
		return "BELOW"
	}
	for _, l := range a.Langs {
		if l.Top1 < replayBar {
			return "BELOW"
		}
	}
	return "PASS"
}

func cmdLocalizeReplay(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("localize-replay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	corpus := fs.String("corpus", "calibration/incidents", "the fixture apps, one directory per language")
	work := fs.String("work", "", "scratch directory for captures and the probe stores (required; never the repository)")
	langs := fs.String("langs", "java,node,php,python", "languages to replay, comma-separated")
	arm := fs.String("arm", "after", "this arm's label")
	commit := fs.String("commit", "", "the commit this arm's localize was built from")
	baseline := fs.String("baseline", "", "an earlier arm's JSON to embed before this one")
	out := fs.String("out", "", "write the reading here (default stdout)")
	bins := map[string]*string{
		"java":   fs.String("java", "java", "the Java launcher"),
		"node":   fs.String("node", "node", "the Node.js binary"),
		"php":    fs.String("php", "php", "the PHP CLI"),
		"python": fs.String("python", "python3", "the Python 3 interpreter"),
	}
	explain := fs.String("explain", "", "case/shape whose full localize answer is written to stderr")
	logs := fs.Bool("logs", false, "TL-4: run each app's log capture runner and replay its log lines through localize's error input")
	deps := fs.String("deps", "", "the logging libraries install-log-deps.sh put here, one directory per language (with -logs)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	replayExplain = *explain
	if *work == "" {
		fmt.Fprintln(stderr, "localize-replay: -work is required")
		return 2
	}
	if *logs && *deps == "" {
		fmt.Fprintln(stderr, "localize-replay: -logs needs -deps")
		return 2
	}
	mode := replayMode{logs: *logs}
	if *logs {
		abs, err := filepath.Abs(*deps)
		if err != nil {
			fmt.Fprintf(stderr, "localize-replay: %v\n", err)
			return 1
		}
		mode.deps = abs
	}
	if err := os.MkdirAll(*work, 0o755); err != nil {
		fmt.Fprintf(stderr, "localize-replay: %v\n", err)
		return 1
	}
	ctx := context.Background()
	a := replayArm{Arm: *arm, Commit: *commit, Langs: map[string]replayLang{}, ByShape: map[string]replayShape{}}
	for _, lang := range strings.Split(*langs, ",") {
		lang = strings.TrimSpace(lang)
		if lang == "" {
			continue
		}
		res, err := replayLanguage(ctx, *corpus, *work, lang, bins, mode, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "localize-replay: %s: %v\n", lang, err)
			return 1
		}
		a.Langs[lang] = res
		a.Answers += res.Answers
		a.Correct += res.Correct
		for s, t := range res.ByShape {
			acc := a.ByShape[s]
			acc.N += t.N
			acc.Correct += t.Correct
			a.ByShape[s] = acc
		}
		for api, t := range res.ByAPI {
			if a.ByAPI == nil {
				a.ByAPI = map[string]replayShape{}
			}
			acc := a.ByAPI[api]
			acc.N += t.N
			acc.Correct += t.Correct
			a.ByAPI[api] = acc
		}
		if res.Echoes != nil {
			if a.Echoes == nil {
				a.Echoes, a.SentinelAnswers = new(int), new(int)
			}
			*a.Echoes += *res.Echoes
			*a.SentinelAnswers += *res.SentinelAnswers
		}
	}
	a.Top1 = ratio(a.Correct, a.Answers)
	for s, t := range a.ByShape {
		t.Top1 = ratio(t.Correct, t.N)
		a.ByShape[s] = t
	}
	for s, t := range a.ByAPI {
		t.Top1 = ratio(t.Correct, t.N)
		a.ByAPI[s] = t
	}

	reading := replayReading{
		Reading:     "TL-7 localize replay over fault-injected fixture apps (docs/plans/telemetry-links.md)",
		Row:         "TL-7",
		Acceptance:  "top-1 enclosing symbol >= 0.90 per language and overall; below that, a written ceiling",
		Truth:       "tier A: the runtime's own structured trace (the innermost frame inside the app of the deepest error that reaches it), recorded by each app's capture runner",
		MissColumns: replayMissColumns,
	}
	if mode.logs {
		reading.Reading = "TL-4 localize replay of log lines the fixture apps' logging libraries wrote (docs/plans/telemetry-links.md)"
		reading.Row = "TL-4"
		reading.Acceptance = "top-1 enclosing symbol of the emitting logger call >= 0.90 per language and overall, and no answer that matched a template echoes the line (echoes 0); below that, a written ceiling"
		reading.Truth = "tier A: the call site the runtime reports for the logging call (Python's LogRecord and structlog's CallsiteParameterAdder, the JVM's StackWalker in the appender and handler, V8's call sites in the destination, format and console, Monolog's IntrospectionProcessor), recorded by each app's log capture runner"
	}
	if *baseline != "" {
		b, err := os.ReadFile(*baseline)
		if err != nil {
			fmt.Fprintf(stderr, "localize-replay: %v\n", err)
			return 1
		}
		var prev replayReading
		if err := json.Unmarshal(b, &prev); err != nil {
			fmt.Fprintf(stderr, "localize-replay: %s: %v\n", *baseline, err)
			return 1
		}
		reading.Arms = append(reading.Arms, prev.Arms...)
	}
	reading.Arms = append(reading.Arms, a)
	reading.Verdict = replayVerdict(a)
	data, err := json.MarshalIndent(reading, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "localize-replay: %v\n", err)
		return 1
	}
	data = append(collapseScalarArrays(data), '\n')
	if *out == "" {
		_, err = stdout.Write(data)
	} else {
		err = os.WriteFile(*out, data, 0o644)
	}
	if err != nil {
		fmt.Fprintf(stderr, "localize-replay: %v\n", err)
		return 1
	}
	for _, l := range sortedKeys(a.Langs) {
		r := a.Langs[l]
		fmt.Fprintf(stderr, "localize-replay: %s %s top-1 %d/%d = %.3f\n", a.Arm, l, r.Correct, r.Answers, r.Top1)
	}
	fmt.Fprintf(stderr, "localize-replay: %s overall top-1 %d/%d = %.3f (%s at %.2f)\n", a.Arm, a.Correct, a.Answers, a.Top1, reading.Verdict, replayBar)
	return 0
}

// scalarArrayRE matches an indented JSON array holding only strings and
// numbers: a miss row, a column list.
var scalarArrayRE = regexp.MustCompile(`\[\s*\n\s*(?:"(?:[^"\\]|\\.)*"|-?[0-9.]+)(?:,\s*\n\s*(?:"(?:[^"\\]|\\.)*"|-?[0-9.]+))*\s*\n\s*\]`)

// collapseScalarArrays writes each such array on one line, so a reading
// with hundreds of rows stays readable.
func collapseScalarArrays(data []byte) []byte {
	return scalarArrayRE.ReplaceAllFunc(data, func(m []byte) []byte {
		var v []any
		if err := json.Unmarshal(m, &v); err != nil {
			return m
		}
		b, err := json.Marshal(v)
		if err != nil {
			return m
		}
		return b
	})
}

// replayExplain is -explain: "case/shape".
var replayExplain string

// replayMode is -logs and -deps.
type replayMode struct {
	logs bool
	deps string
}

// captureCommand is the runner replayLanguage starts: the trace capture, or
// with -logs the log capture with its environment and "{deps}" filled in.
func captureCommand(m incidentManifest, lang string, mode replayMode) ([]string, []string, error) {
	if !mode.logs {
		return m.Args, []string{"PYTHONDONTWRITEBYTECODE=1"}, nil
	}
	if len(m.LogArgs) == 0 {
		return nil, nil, fmt.Errorf("incident.json has no log_args")
	}
	dir := filepath.Join(mode.deps, lang)
	fill := func(s string) string { return strings.ReplaceAll(s, "{deps}", dir) }
	args := make([]string, len(m.LogArgs))
	for i, a := range m.LogArgs {
		args[i] = fill(a)
	}
	env := []string{"PYTHONDONTWRITEBYTECODE=1", "INCIDENT_DEPS=" + dir}
	for _, k := range sortedKeys(m.LogEnv) {
		env = append(env, k+"="+fill(m.LogEnv[k]))
	}
	return args, env, nil
}

// replayLanguage captures, ingests and replays one language's app.
func replayLanguage(ctx context.Context, corpus, work, lang string, bins map[string]*string, mode replayMode, stderr io.Writer) (replayLang, error) {
	dir := filepath.Join(corpus, lang)
	mb, err := os.ReadFile(filepath.Join(dir, "incident.json"))
	if err != nil {
		return replayLang{}, err
	}
	var m incidentManifest
	if err := json.Unmarshal(mb, &m); err != nil {
		return replayLang{}, fmt.Errorf("incident.json: %w", err)
	}
	bin, ok := bins[m.Runtime]
	if !ok {
		return replayLang{}, fmt.Errorf("unknown runtime %q", m.Runtime)
	}

	// Capture: the app runs natively, in its own directory.
	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	runArgs, runEnv, err := captureCommand(m, lang, mode)
	if err != nil {
		return replayLang{}, err
	}
	cmd := exec.CommandContext(cctx, *bin, runArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), runEnv...)
	cmd.Stderr = stderr
	raw, err := cmd.Output()
	if err != nil {
		return replayLang{}, fmt.Errorf("capture: %w", err)
	}
	captures := lang + "-captures.jsonl"
	if mode.logs {
		captures = lang + "-log-captures.jsonl"
	}
	if err := os.WriteFile(filepath.Join(work, captures), raw, 0o644); err != nil {
		return replayLang{}, err
	}
	var caps []incidentCapture
	for _, ln := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var c incidentCapture
		if err := json.Unmarshal([]byte(ln), &c); err != nil {
			return replayLang{}, fmt.Errorf("capture line: %w", err)
		}
		caps = append(caps, c)
	}

	// Ingest: one scratch store per language, removed after its replay.
	store := filepath.Join(work, lang+".sqlite")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(store + suffix)
	}
	appAbs, err := filepath.Abs(filepath.Join(dir, m.App))
	if err != nil {
		return replayLang{}, err
	}
	c, err := client.NewConstantClient(client.ModeLocal, store)
	if err != nil {
		return replayLang{}, err
	}
	defer func() {
		_ = c.Close()
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(store + suffix)
		}
	}()
	started := time.Now()
	rep, err := c.Ingest(ctx, core.ServiceRef{ID: m.Service, Path: appAbs})
	if err != nil {
		return replayLang{}, fmt.Errorf("ingest: %w", err)
	}
	res := replayLang{Service: m.Service, Cases: len(caps), ByShape: map[string]replayShape{}, Misses: []replayCase{}, Symbols: rep.Symbols, Ingested: time.Since(started).Round(time.Millisecond).String()}
	if mode.logs {
		res.ByAPI = map[string]replayShape{}
		res.Echoes, res.SentinelAnswers = new(int), new(int)
	}

	// Replay: the MCP localize tool, as an agent calls it.
	srv := mcp.NewServer(c)
	for _, cp := range caps {
		res.Truths = append(res.Truths, incidentTruthRow{Case: cp.Case, incidentTruth: cp.Truth})
		for _, shape := range sortedKeys(cp.Payloads) {
			input := "stack_trace"
			if shape == replayLogShape {
				input = "error"
			}
			cands, body, err := localizeCandidates(ctx, srv, m.Service, input, cp.Payloads[shape])
			if replayExplain == cp.Case+"/"+shape {
				fmt.Fprintf(stderr, "localize-replay: %s %s/%s answer:\n%s\n", lang, cp.Case, shape, body)
			}
			if err != nil {
				return replayLang{}, fmt.Errorf("%s/%s: %w", cp.Case, shape, err)
			}
			rc := replayCase{Case: cp.Case, Shape: shape}
			if len(cands) > 0 {
				rc.TopFile, rc.TopSymbol = cands[0].File, cands[0].Symbol
			}
			for i, cd := range cands {
				if replayHit(cd.File, cd.Symbol, cp.Truth) {
					rc.Rank = i + 1
					break
				}
			}
			rc.Correct = rc.Rank == 1
			if !rc.Correct {
				res.Misses = append(res.Misses, rc)
			}
			t := res.ByShape[shape]
			t.N++
			res.Answers++
			if rc.Correct {
				t.Correct++
				res.Correct++
			}
			res.ByShape[shape] = t
			if res.ByAPI != nil && shape == replayLogShape {
				api := res.ByAPI[cp.API]
				api.N++
				if rc.Correct {
					api.Correct++
				}
				res.ByAPI[cp.API] = api
				if replayEchoes(body) {
					*res.Echoes++
				}
				if strings.Contains(body, replayLogSentinel) {
					*res.SentinelAnswers++
				}
			}
		}
	}
	res.Top1 = ratio(res.Correct, res.Answers)
	for s, t := range res.ByShape {
		t.Top1 = ratio(t.Correct, t.N)
		res.ByShape[s] = t
	}
	for s, t := range res.ByAPI {
		t.Top1 = ratio(t.Correct, t.N)
		res.ByAPI[s] = t
	}
	return res, nil
}

// replayEchoes reports whether a localize answer that matched a log
// template carries the sentinel: any of the pasted line beyond the
// template's own text (ADR-058 decision 3).
func replayEchoes(body string) bool {
	return strings.Contains(body, `"log_template"`) && strings.Contains(body, replayLogSentinel)
}

type replayCandidate struct {
	File   string `json:"file"`
	Symbol string `json:"symbol"`
}

// localizeCandidates calls localize through the server's JSON-RPC handler,
// the payload given as input (stack_trace, or error for a log line).
func localizeCandidates(ctx context.Context, srv *mcp.Server, service, input, payload string) ([]replayCandidate, string, error) {
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "localize", "arguments": map[string]any{"service": service, input: payload}},
	})
	if err != nil {
		return nil, "", err
	}
	raw, ok := srv.HandleRequest(ctx, req)
	if !ok {
		return nil, "", fmt.Errorf("no response")
	}
	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, "", err
	}
	if resp.Error != nil {
		return nil, "", fmt.Errorf("localize: %s", resp.Error.Message)
	}
	if len(resp.Result.Content) == 0 {
		return nil, "", fmt.Errorf("localize: empty content")
	}
	var body struct {
		Candidates []replayCandidate `json:"candidates"`
	}
	text := resp.Result.Content[0].Text
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		return nil, "", err
	}
	return body.Candidates, text, nil
}

// replayHit: the candidate's file is the truth's (by path suffix) and its
// symbol's bare name is the injected function's.
func replayHit(file, symbol string, t incidentTruth) bool {
	if file != t.File && !strings.HasSuffix(file, "/"+t.File) && !strings.HasSuffix(t.File, "/"+file) {
		return false
	}
	name := symbol
	for _, sep := range []string{".", "::", "#", "->", "\\"} {
		if i := strings.LastIndex(name, sep); i >= 0 {
			name = name[i+len(sep):]
		}
	}
	return name == t.Name
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(int(float64(n)/float64(d)*1000+0.5)) / 1000
}
