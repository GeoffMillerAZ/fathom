// Command telemetry-census is the C0 census (TL-6, ADR-062 decision 3): how
// many of the metric names an observability stack's PromQL reads can a
// static pass name the emitter of? It gates Stage B (name joins, F66): the
// emitter share E must be at least 25 percent.
//
//	telemetry-census -corpus <name>=<checkout> [-corpus …] -out census-c0.json
//
// Static files only, never run: every expression of a Prometheus rule (a
// rule file's or a PrometheusRule custom resource's groups), of a Grafana
// alert rule's query, and of a Grafana dashboard panel's targets[].expr (a
// dashboard file, or one held in a ConfigMap) is read by fathom's own selector
// scanner (internal/promql), so the census and the dialects read PromQL
// alike. Dashboards are data: no Grafana code is read. Each metric name is
// one of three classes (classes.json, data):
//
//   - E, emitter: a string literal of the corpus' source code names it (a
//     .go, .py, .java, .cs, .js, … file), compared after Prometheus' series
//     suffixes and the OpenTelemetry translation's unit suffixes are stripped,
//     with `.`, `-` and `/` read as `_` (`demo.cart.add_item.latency` emits
//     demo_cart_add_item_latency_seconds_bucket);
//   - P, platform: a platform, exporter or agent prefix (node_, kube_, …);
//   - U: everything else (a recording rule's output is counted within U).
//
// It also counts the expressions with a service-identifying equality matcher
// (an identifying label other than the namespace, a literal value). The
// denominator is every expression, an unscannable one included. A $0 script:
// no extractor, no store, no model.
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/promql"
)

//go:embed classes.json
var classesJSON []byte

type classes struct {
	Platform         []string `json:"platform"`
	SeriesSuffixes   []string `json:"series_suffixes"`
	UnitSuffixes     []string `json:"unit_suffixes"`
	SourceExtensions []string `json:"source_extensions"`
	IdentityExcluded []string `json:"identity_excluded"`
}

// expr is one PromQL expression and where it was written.
type expr struct {
	Path   string `json:"path"`
	Source string `json:"source"` // rule | grafana-alert | dashboard
	Text   string `json:"-"`
}

type corpusFlag []string

func (c *corpusFlag) String() string     { return strings.Join(*c, ",") }
func (c *corpusFlag) Set(v string) error { *c = append(*c, v); return nil }

func main() {
	var corpora corpusFlag
	flag.Var(&corpora, "corpus", "name=checkout (repeatable)")
	out := flag.String("out", "", "census JSON to write (default stdout)")
	date := flag.String("date", "", "measured_at (YYYY-MM-DD)")
	commit := flag.String("commit", "", "the fathom commit whose scanner read the corpus")
	flag.Parse()
	if len(corpora) == 0 {
		fmt.Fprintln(os.Stderr, "usage: telemetry-census -corpus name=dir [-corpus …] [-out file]")
		os.Exit(2)
	}
	var cl classes
	if err := json.Unmarshal(classesJSON, &cl); err != nil {
		panic(err)
	}
	rep := report{Census: "C0", Plan: "TL-6", MeasuredAt: *date, FathomCommit: *commit, Scanner: "internal/promql.Selectors",
		Classes: cl, Gate: 0.25}
	for _, c := range corpora {
		name, dir, ok := strings.Cut(c, "=")
		if !ok {
			fmt.Fprintln(os.Stderr, "-corpus wants name=dir:", c)
			os.Exit(2)
		}
		rep.Corpora = append(rep.Corpora, censusCorpus(name, dir, cl))
	}
	rep.total()
	b, _ := json.MarshalIndent(rep, "", "  ")
	b = append(b, '\n')
	if *out == "" {
		os.Stdout.Write(b)
		return
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ── the reading ──────────────────────────────────────────────────────────

type counts struct {
	Exprs              int            `json:"exprs"`
	ExprsBySource      map[string]int `json:"exprs_by_source"`
	ExprsUnscannable   int            `json:"exprs_unscannable"`
	ExprsGrafanaVars   int            `json:"exprs_with_grafana_variables"`
	ExprsWithIdentity  int            `json:"exprs_with_identity_matcher"`
	ExprsWithTemplated int            `json:"exprs_with_templated_identity_matcher"`
	IdentityShare      float64        `json:"identity_matcher_share"`
	Names              int            `json:"metric_names"`
	E                  int            `json:"E"`
	P                  int            `json:"P"`
	U                  int            `json:"U"`
	URecorded          int            `json:"U_recorded"`
	EmitterShare       float64        `json:"emitter_share"`
	PlatformShare      float64        `json:"platform_share"`
	UnknownShare       float64        `json:"unknown_share"`
	ExprsReadingE      int            `json:"exprs_reading_an_E_name"`
	ExprsReadingEShare float64        `json:"exprs_reading_an_E_name_share"`
}

func (c *counts) shares() {
	if c.Names > 0 {
		c.EmitterShare = round(float64(c.E) / float64(c.Names))
		c.PlatformShare = round(float64(c.P) / float64(c.Names))
		c.UnknownShare = round(float64(c.U) / float64(c.Names))
	}
	if c.Exprs > 0 {
		c.IdentityShare = round(float64(c.ExprsWithIdentity) / float64(c.Exprs))
		c.ExprsReadingEShare = round(float64(c.ExprsReadingE) / float64(c.Exprs))
	}
}

func round(f float64) float64 { return float64(int(f*10000+0.5)) / 10000 }

type corpusReport struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	SHA         string `json:"sha"`
	SourceFiles int    `json:"source_files"`
	counts
	// Examples: up to five names of each class, for the reader.
	Examples map[string][]string `json:"examples"`
}

type report struct {
	Census       string         `json:"census"`
	Plan         string         `json:"plan_row"`
	MeasuredAt   string         `json:"measured_at"`
	FathomCommit string         `json:"fathom_commit"`
	Scanner      string         `json:"scanner"`
	Classes      classes        `json:"classes"`
	Corpora      []corpusReport `json:"corpora"`
	// Pooled is the sum over corpora (a name in two corpora counts twice: each
	// corpus' emitter search is its own source).
	Pooled counts  `json:"pooled"`
	Gate   float64 `json:"stage_b_gate"`
	// StageBClears: the pooled emitter share is at least the gate.
	StageBClears bool `json:"stage_b_clears"`
	// EmitterShare is the pooled E share (TL-6's lexical key).
	EmitterShare float64 `json:"emitter_share"`
}

func (r *report) total() {
	p := counts{ExprsBySource: map[string]int{}}
	for _, c := range r.Corpora {
		p.Exprs += c.Exprs
		for k, v := range c.ExprsBySource {
			p.ExprsBySource[k] += v
		}
		p.ExprsUnscannable += c.ExprsUnscannable
		p.ExprsGrafanaVars += c.ExprsGrafanaVars
		p.ExprsWithIdentity += c.ExprsWithIdentity
		p.ExprsWithTemplated += c.ExprsWithTemplated
		p.Names += c.Names
		p.E += c.E
		p.P += c.P
		p.U += c.U
		p.URecorded += c.URecorded
		p.ExprsReadingE += c.ExprsReadingE
	}
	p.shares()
	r.Pooled = p
	r.EmitterShare = p.EmitterShare
	r.StageBClears = p.EmitterShare >= r.Gate
}

// ── one corpus ───────────────────────────────────────────────────────────

var skipDirs = map[string]bool{".git": true, "node_modules": true, "vendor": true}

func censusCorpus(name, dir string, cl classes) corpusReport {
	cr := corpusReport{Name: name, Examples: map[string][]string{}}
	cr.ExprsBySource = map[string]int{}
	cr.SHA = gitOut(dir, "rev-parse", "HEAD")
	cr.URL = gitOut(dir, "config", "--get", "remote.origin.url")
	srcExt := map[string]bool{}
	for _, e := range cl.SourceExtensions {
		srcExt[e] = true
	}
	var exprs []expr
	literals := map[string]bool{}
	recorded := map[string]bool{}
	err := filepath.WalkDir(dir, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if p != dir && skipDirs[e.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !e.Type().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		ext := strings.ToLower(filepath.Ext(p))
		switch {
		case srcExt[ext]:
			b, err := os.ReadFile(p)
			if err != nil || len(b) > 4<<20 {
				return nil
			}
			cr.SourceFiles++
			for _, m := range literalRE.FindAllSubmatch(b, -1) {
				for _, g := range m[1:] {
					if len(g) > 0 {
						literals[normalize(string(g))] = true
					}
				}
			}
		case ext == ".yml" || ext == ".yaml":
			b, err := os.ReadFile(p)
			if err != nil || len(b) > 8<<20 {
				return nil
			}
			exprs = append(exprs, yamlExprs(rel, b, recorded)...)
		case ext == ".json":
			b, err := os.ReadFile(p)
			if err != nil || len(b) > 8<<20 {
				return nil
			}
			var v any
			if json.Unmarshal(b, &v) == nil {
				exprs = append(exprs, dashboardExprs(rel, v)...)
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	ident := map[string]bool{}
	for _, l := range promql.IdentityLabels() {
		ident[l] = true
	}
	for _, l := range cl.IdentityExcluded {
		delete(ident, l)
	}
	class := map[string]string{}
	for _, x := range exprs {
		cr.Exprs++
		cr.ExprsBySource[x.Source]++
		text, templated := degrafana(x.Text)
		if templated {
			cr.ExprsGrafanaVars++
		}
		sels, err := promql.Selectors(text)
		if err != nil {
			cr.ExprsUnscannable++
			continue
		}
		hasIdent, hasTmpl, readsE := false, false, false
		for _, s := range sels {
			for _, m := range s.Matchers {
				if ident[m.Name] && m.Op == promql.OpEqual && m.Value != "" {
					if strings.Contains(m.Value, "$") {
						hasTmpl = true
					} else {
						hasIdent = true
					}
				}
			}
			if s.Metric == "" || s.Metric == grafanaVar {
				continue
			}
			c, ok := class[s.Metric]
			if !ok {
				c = classify(s.Metric, literals, cl)
				class[s.Metric] = c
			}
			readsE = readsE || c == "E"
		}
		if hasIdent {
			cr.ExprsWithIdentity++
		}
		if hasTmpl && !hasIdent {
			cr.ExprsWithTemplated++
		}
		if readsE {
			cr.ExprsReadingE++
		}
	}
	names := make([]string, 0, len(class))
	for n := range class {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := class[n]
		cr.Names++
		switch c {
		case "E":
			cr.E++
		case "P":
			cr.P++
		default:
			cr.U++
			if recorded[n] {
				cr.URecorded++
			}
		}
		if len(cr.Examples[c]) < 5 {
			cr.Examples[c] = append(cr.Examples[c], n)
		}
	}
	cr.shares()
	return cr
}

// grafanaVar stands for a Grafana template variable written outside a
// string (`topk($n, …)`, `by ($label)`): an identifier the scanner reads
// where PromQL wants one, never counted as a metric name.
const grafanaVar = "grafana_var__"

var (
	grafanaRange   = regexp.MustCompile(`^\$\{?__(rate_interval|interval|range)\}?`)
	grafanaNumber  = regexp.MustCompile(`^\$\{?__(interval|range)_(ms|s)\}?`)
	grafanaGeneric = regexp.MustCompile(`^\$(\{[^}]*\}|[A-Za-z_][A-Za-z0-9_]*)`)
)

// degrafana reads a dashboard expression's Grafana variables outside its
// strings as PromQL: a range variable is the duration 5m, a millisecond or
// second count the number 1, any other variable grafanaVar; a variable inside
// a string (`job="$job"`) stays as written. templated reports a substitution.
func degrafana(e string) (string, bool) {
	if !strings.Contains(e, "$") {
		return e, false
	}
	var b strings.Builder
	templated := false
	var quote byte
	for i := 0; i < len(e); i++ {
		c := e[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == '\\' && quote != '`' && i+1 < len(e) {
				i++
				b.WriteByte(e[i])
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch {
		case c == '"' || c == '\'' || c == '`':
			quote = c
			b.WriteByte(c)
		case c == '$':
			rest := e[i:]
			var m string
			switch {
			case grafanaNumber.MatchString(rest):
				m = grafanaNumber.FindString(rest)
				b.WriteString("1")
			case grafanaRange.MatchString(rest):
				m = grafanaRange.FindString(rest)
				b.WriteString("5m")
			case grafanaGeneric.MatchString(rest):
				m = grafanaGeneric.FindString(rest)
				b.WriteString(grafanaVar)
			default:
				b.WriteByte(c)
				continue
			}
			templated = true
			i += len(m) - 1
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), templated
}

// literalRE is a quoted string literal of source code: "…", '…' or `…`, on
// one line, at most 200 bytes.
var literalRE = regexp.MustCompile("\"([^\"\\\\\\n]{1,200})\"|'([^'\\\\\\n]{1,200})'|`([^`\\n]{1,200})`")

// normalize reads a name as Prometheus does after the OpenTelemetry
// translation: `.`, `-` and `/` are `_`.
func normalize(s string) string {
	return strings.NewReplacer(".", "_", "-", "_", "/", "_").Replace(s)
}

// classify is a metric name's class: E when a source literal names it (the
// name, less one series suffix, less one unit suffix), else P by prefix,
// else U.
func classify(name string, literals map[string]bool, cl classes) string {
	cands := []string{name}
	stem := name
	for _, s := range cl.SeriesSuffixes {
		if t, ok := strings.CutSuffix(name, s); ok && t != "" {
			stem = t
			cands = append(cands, t)
			break
		}
	}
	for _, base := range []string{stem, strings.TrimSuffix(stem, "_total")} {
		for _, u := range cl.UnitSuffixes {
			if t, ok := strings.CutSuffix(base, u); ok && t != "" {
				cands = append(cands, t)
				break
			}
		}
	}
	for _, c := range cands {
		if literals[normalize(c)] {
			return "E"
		}
	}
	for _, p := range cl.Platform {
		if strings.HasSuffix(p, "_") || strings.HasSuffix(p, ":") {
			if strings.HasPrefix(name, p) {
				return "P"
			}
		} else if name == p {
			return "P"
		}
	}
	return "U"
}

// yamlExprs reads every document of a YAML file: a rule file's or a
// PrometheusRule's groups[].rules[].expr (recording each record name), a
// Grafana alert rule's data[].model.expr, and a dashboard held in a
// ConfigMap's data.
func yamlExprs(rel string, b []byte, recorded map[string]bool) []expr {
	var out []expr
	dec := yaml.NewDecoder(bytes.NewReader(b))
	for {
		var doc any
		if err := dec.Decode(&doc); err != nil {
			break
		}
		m, _ := doc.(map[string]any)
		if m == nil {
			continue
		}
		groups := m["groups"]
		if spec, ok := m["spec"].(map[string]any); ok && m["kind"] == "PrometheusRule" {
			groups = spec["groups"]
		}
		for _, g := range asList(groups) {
			gm, _ := g.(map[string]any)
			for _, r := range asList(gm["rules"]) {
				rm, _ := r.(map[string]any)
				if rec, ok := rm["record"].(string); ok && rec != "" {
					recorded[rec] = true
				}
				if s, ok := rm["expr"].(string); ok {
					out = append(out, expr{Path: rel, Source: "rule", Text: s})
				}
				for _, d := range asList(rm["data"]) { // a Grafana alert rule
					dm, _ := d.(map[string]any)
					if model, ok := dm["model"].(map[string]any); ok {
						if s, ok := model["expr"].(string); ok && s != "" {
							out = append(out, expr{Path: rel, Source: "grafana-alert", Text: s})
						}
					}
				}
			}
		}
		// ConfigMaps (and a ConfigMapList's items) holding dashboards
		items := []any{m}
		if l, ok := m["items"].([]any); ok {
			items = append(items, l...)
		}
		for _, it := range items {
			im, _ := it.(map[string]any)
			data, _ := im["data"].(map[string]any)
			keys := make([]string, 0, len(data))
			for k := range data {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				s, _ := data[k].(string)
				if !strings.HasSuffix(k, ".json") || s == "" {
					continue
				}
				var v any
				if json.Unmarshal([]byte(s), &v) == nil {
					out = append(out, dashboardExprs(rel+"#"+k, v)...)
				}
			}
		}
	}
	return out
}

// dashboardExprs is every panel target's expr of a Grafana dashboard: a
// `targets` list's objects anywhere under the dashboard (rows and nested
// panels included).
func dashboardExprs(rel string, v any) []expr {
	var out []expr
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if k == "targets" {
					for _, tg := range asList(t[k]) {
						if tm, ok := tg.(map[string]any); ok {
							if s, ok := tm["expr"].(string); ok && strings.TrimSpace(s) != "" {
								out = append(out, expr{Path: rel, Source: "dashboard", Text: s})
							}
						}
					}
					continue
				}
				walk(t[k])
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	walk(v)
	return out
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func gitOut(dir string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
