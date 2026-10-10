package main

// matchers_diff_test.go — the differential of fathom's promql.ParseMatchers
// (the selector scanner's brace grammar, reading an Alertmanager matcher
// list; LB-F361) against Alertmanager's own parser (matcher/compat, the one
// its configuration decodes `matchers`, `source_matchers` and
// `target_matchers` with), over hand-written cases and every matcher line of
// the corpus. It lives in this nested, test-only module so Alertmanager never
// becomes a dependency of fathom's main module (ADR-020, ADR-025).
//
//	FATHOM_AM_CORPUS=<dir>[:<dir>...] go test -run TestParseMatchersAgreeWithAlertmanager -v .

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/prometheus/alertmanager/matcher/compat"
	"github.com/prometheus/alertmanager/pkg/labels"
	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/promql"
)

var amOps = map[labels.MatchType]string{labels.MatchEqual: "=", labels.MatchNotEqual: "!=", labels.MatchRegexp: "=~", labels.MatchNotRegexp: "!~"}

func amSet(line string) ([]string, error) {
	ms, err := compat.Matchers(line, "config")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, m := range ms {
		out = append(out, m.Name+amOps[m.Type]+m.Value)
	}
	sort.Strings(out)
	return out, nil
}

func fathomSet(line string) ([]string, error) {
	ms, err := promql.ParseMatchers(line)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, m := range ms {
		out = append(out, m.Name+m.Op+m.Value)
	}
	sort.Strings(out)
	return out, nil
}

// corpusMatcherLines is every scalar of a matchers, source_matchers or
// target_matchers list in the YAML files under the dirs.
func corpusMatcherLines(t *testing.T, dirs []string) []string {
	var out []string
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n == nil {
			return
		}
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				switch n.Content[i].Value {
				case "matchers", "source_matchers", "target_matchers":
					if v := n.Content[i+1]; v.Kind == yaml.SequenceNode {
						for _, it := range v.Content {
							if it.Kind == yaml.ScalarNode {
								out = append(out, it.Value)
							}
						}
					}
				}
				walk(n.Content[i+1])
			}
			return
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	for _, d := range dirs {
		_ = filepath.WalkDir(d, func(p string, e os.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return nil
			}
			if ext := strings.ToLower(filepath.Ext(p)); ext != ".yml" && ext != ".yaml" {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			var n yaml.Node
			if yaml.Unmarshal(b, &n) == nil {
				walk(&n)
			}
			return nil
		})
	}
	return out
}

// TestParseMatchersAgreeWithAlertmanager: on every line Alertmanager parses,
// ParseMatchers names the same matchers (as a sorted set); a line
// Alertmanager refuses is counted, and ParseMatchers must not read it as
// something else.
func TestParseMatchersAgreeWithAlertmanager(t *testing.T) {
	lines := []string{
		`alertname = Watchdog`, `severity =~ warning|info`, `severity="critical"`, `{team="a", service!~"x|y"}`,
		`foo=bar,baz!=qux`, `env = "prod, eu"`, `msg="a \"quoted\" word"`, `instance=`, `{}`, `a=b, c="d"`,
		`job!~"node|kube.*"`, `cluster = "prod-1"`,
	}
	if c := os.Getenv("FATHOM_AM_CORPUS"); c != "" {
		lines = append(lines, corpusMatcherLines(t, strings.Split(c, ":"))...)
	}
	agree, refused, both := 0, 0, 0
	var disagree []string
	for _, l := range lines {
		want, aerr := amSet(l)
		got, ferr := fathomSet(l)
		switch {
		case aerr != nil && ferr != nil:
			both++
		case aerr != nil:
			refused++
			disagree = append(disagree, fmt.Sprintf("%q: Alertmanager refuses (%v), fathom reads %q", l, aerr, got))
		case ferr != nil:
			disagree = append(disagree, fmt.Sprintf("%q: fathom refuses (%v), Alertmanager reads %q", l, ferr, want))
		case strings.Join(want, "\x00") != strings.Join(got, "\x00"):
			disagree = append(disagree, fmt.Sprintf("%q: Alertmanager %q, fathom %q", l, want, got))
		default:
			agree++
		}
	}
	t.Logf("%d matcher lines: %d agree, %d refused by both, %d disagree", len(lines), agree, both, len(disagree))
	for _, d := range disagree {
		t.Error(d)
	}
	_ = refused
}
