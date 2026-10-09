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
}

// incidentCapture is one line of a capture runner's output.
type incidentCapture struct {
	Case     string            `json:"case"`
	Lang     string            `json:"lang"`
	Truth    incidentTruth     `json:"truth"`
	Payloads map[string]string `json:"payloads"`
}

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
	Langs   map[string]replayLang  `json:"langs"`
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

// replayVerdict reads an arm against replayBar.
func replayVerdict(a replayArm) string {
	if a.Answers == 0 || a.Top1 < replayBar {
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
	if err := fs.Parse(args); err != nil {
		return 2
	}
	replayExplain = *explain
	if *work == "" {
		fmt.Fprintln(stderr, "localize-replay: -work is required")
		return 2
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
		res, err := replayLanguage(ctx, *corpus, *work, lang, bins, stderr)
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
	}
	a.Top1 = ratio(a.Correct, a.Answers)
	for s, t := range a.ByShape {
		t.Top1 = ratio(t.Correct, t.N)
		a.ByShape[s] = t
	}

	reading := replayReading{
		Reading:     "TL-7 localize replay over fault-injected fixture apps (docs/plans/telemetry-links.md)",
		Row:         "TL-7",
		Acceptance:  "top-1 enclosing symbol >= 0.90 per language and overall; below that, a written ceiling",
		Truth:       "tier A: the runtime's own structured trace (the innermost frame inside the app of the deepest error that reaches it), recorded by each app's capture runner",
		MissColumns: replayMissColumns,
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

// replayLanguage captures, ingests and replays one language's app.
func replayLanguage(ctx context.Context, corpus, work, lang string, bins map[string]*string, stderr io.Writer) (replayLang, error) {
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
	cmd := exec.CommandContext(cctx, *bin, m.Args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	cmd.Stderr = stderr
	raw, err := cmd.Output()
	if err != nil {
		return replayLang{}, fmt.Errorf("capture: %w", err)
	}
	if err := os.WriteFile(filepath.Join(work, lang+"-captures.jsonl"), raw, 0o644); err != nil {
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

	// Replay: the MCP localize tool, as an agent calls it.
	srv := mcp.NewServer(c)
	for _, cp := range caps {
		res.Truths = append(res.Truths, incidentTruthRow{Case: cp.Case, incidentTruth: cp.Truth})
		for _, shape := range sortedKeys(cp.Payloads) {
			cands, body, err := localizeCandidates(ctx, srv, m.Service, cp.Payloads[shape])
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
		}
	}
	res.Top1 = ratio(res.Correct, res.Answers)
	for s, t := range res.ByShape {
		t.Top1 = ratio(t.Correct, t.N)
		res.ByShape[s] = t
	}
	return res, nil
}

type replayCandidate struct {
	File   string `json:"file"`
	Symbol string `json:"symbol"`
}

// localizeCandidates calls localize through the server's JSON-RPC handler.
func localizeCandidates(ctx context.Context, srv *mcp.Server, service, payload string) ([]replayCandidate, string, error) {
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "localize", "arguments": map[string]any{"service": service, "stack_trace": payload}},
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
