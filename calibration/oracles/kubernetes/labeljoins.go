package main

// labeljoins.go — the Stage A static truth (TL-8's deploy half, wave c1):
// which Service each PrometheusRule expression's series come from, by the
// scrape configuration prometheus-operator generates, never by fathom's
// route rule. A ServiceMonitor scrapes every Service its selector matches in
// the namespaces its namespaceSelector names (all of them, not the nearest),
// and gives each target the job label the Service's own label named by
// `jobLabel` carries, else the Service's name (the operator's API reference,
// ServiceMonitorSpec.jobLabel), and the `service` label the Service's name.
// An expression's selector with `job="X"` therefore reads the Services whose
// job is X, and one with `service="X"` the Services named X; the namespace
// matcher of such a selector names the series' namespace, which for an
// exporter that keeps its own labels (honorLabels) is not the Service's, so
// it does not narrow the truth. Every alert and recording expression of
// every PrometheusRule is written, with or without a truth, so the reading's
// denominator is all of them. Tier C: a rule-based reading of the
// operator's documented semantics over the parsed manifests.

import (
	"encoding/json"
	"os"
	"sort"

	promlabels "github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"
)

// joinTarget is one Service an expression reads.
type joinTarget struct {
	ToPath string `json:"to_path"`
	To     string `json:"to"`
}

// joinExpr is one rule's expression and the Services its identifying
// matchers read.
type joinExpr struct {
	Path string `json:"path"`
	From string `json:"from"` // the PrometheusRule
	Line int    `json:"line"` // the expr value's line
	Rule string `json:"rule"` // alert or record name
	Kind string `json:"kind"` // alert | record
	// Matchers are the identifying equality matchers read (job, service).
	Matchers []string     `json:"matchers"`
	Truth    []joinTarget `json:"truth"`
	// Parsed is false when the Prometheus parser refuses the expression.
	Parsed bool `json:"parsed"`
}

type labelJoins struct {
	Tool    string     `json:"tool"`
	Version string     `json:"version"`
	Tier    string     `json:"tier"`
	Exprs   []joinExpr `json:"exprs"`
}

type ljService struct {
	path, entity, ns, name string
	labels                 map[string]string
}

type ljMonitor struct {
	sel      labelSel
	nss      []string
	jobLabel string
}

type ljRule struct {
	path, entity, ns string
	root             *yaml.Node
}

// ljWalk gathers the Services, ServiceMonitors and PrometheusRules.
type ljWalk struct {
	services []ljService
	monitors []ljMonitor
	rules    []ljRule
}

func (j *ljWalk) document(rel string, root *yaml.Node, kind, api, name, entity, ns string) {
	switch {
	case kind == "Service" && api == "v1":
		j.services = append(j.services, ljService{rel, entity, ns, name, labels(path(root, "metadata", "labels"))})
	case kind == "ServiceMonitor" && len(api) > 21 && api[:21] == "monitoring.coreos.com":
		s := readSelector(path(root, "spec", "selector"))
		if !s.ok {
			return
		}
		nss := []string{ns}
		nsSel := path(root, "spec", "namespaceSelector")
		if scalar(get(nsSel, "any")) == "true" {
			nss = []string{"*"}
		} else if names := seq(get(nsSel, "matchNames")); len(names) > 0 {
			nss = nil
			for _, n := range names {
				nss = append(nss, scalar(n))
			}
		}
		j.monitors = append(j.monitors, ljMonitor{s, nss, scalar(path(root, "spec", "jobLabel"))})
	case kind == "PrometheusRule" && len(api) > 21 && api[:21] == "monitoring.coreos.com":
		j.rules = append(j.rules, ljRule{rel, entity, ns, root})
	}
}

// write resolves every expression and writes the truth file.
func (j *ljWalk) write(p string) error {
	jobs := map[string][]joinTarget{}
	seenJob := map[string]bool{}
	for _, m := range j.monitors {
		for _, s := range j.services {
			in := false
			for _, n := range m.nss {
				in = in || n == "*" || n == s.ns
			}
			if !in || !m.sel.matches(s.labels) {
				continue
			}
			job := s.name
			if v, ok := s.labels[m.jobLabel]; m.jobLabel != "" && ok && v != "" {
				job = v
			}
			k := job + "\x00" + s.path + "\x00" + s.entity
			if !seenJob[k] {
				seenJob[k] = true
				jobs[job] = append(jobs[job], joinTarget{s.path, s.entity})
			}
		}
	}
	byName := map[string][]joinTarget{}
	for _, s := range j.services {
		byName[s.name] = append(byName[s.name], joinTarget{s.path, s.entity})
	}
	out := labelJoins{Tool: "kubernetes-oracle label joins (prometheus-operator ServiceMonitor jobLabel semantics; promql/parser)", Version: "github.com/prometheus/prometheus v0.315.0", Tier: "C", Exprs: []joinExpr{}}
	for _, r := range j.rules {
		for _, g := range seq(path(r.root, "spec", "groups")) {
			for _, rule := range seq(get(g, "rules")) {
				exprNode := get(rule, "expr")
				expr := scalar(exprNode)
				if expr == "" {
					continue
				}
				e := joinExpr{Path: r.path, From: r.entity, Line: exprNode.Line, Matchers: []string{}, Truth: []joinTarget{}}
				if n := scalar(get(rule, "alert")); n != "" {
					e.Rule, e.Kind = n, "alert"
				} else {
					e.Rule, e.Kind = scalar(get(rule, "record")), "record"
				}
				seen := map[joinTarget]bool{}
				add := func(ts []joinTarget) {
					for _, t := range ts {
						if !seen[t] {
							seen[t] = true
							e.Truth = append(e.Truth, t)
						}
					}
				}
				if ast, err := promParser.ParseExpr(expr); err == nil {
					e.Parsed = true
					for _, sel := range parser.ExtractSelectors(ast) {
						for _, mt := range sel {
							if mt.Type != promlabels.MatchEqual || mt.Value == "" {
								continue
							}
							switch mt.Name {
							case "job":
								e.Matchers = append(e.Matchers, `job="`+mt.Value+`"`)
								add(jobs[mt.Value])
							case "service":
								e.Matchers = append(e.Matchers, `service="`+mt.Value+`"`)
								add(byName[mt.Value])
							}
						}
					}
				}
				sort.Strings(e.Matchers)
				sort.Slice(e.Truth, func(a, b int) bool {
					return e.Truth[a].ToPath+"\x00"+e.Truth[a].To < e.Truth[b].ToPath+"\x00"+e.Truth[b].To
				})
				out.Exprs = append(out.Exprs, e)
			}
		}
	}
	sort.SliceStable(out.Exprs, func(a, b int) bool {
		x, y := out.Exprs[a], out.Exprs[b]
		if x.Path != y.Path {
			return x.Path < y.Path
		}
		return x.Line < y.Line
	})
	b, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}
