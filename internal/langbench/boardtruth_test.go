package langbench

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langclass"
	"github.com/geoffmilleraz/fathom/internal/langroute"
)

// GAP1-1 checks names, not generated disclosure parity. Unsupported and refused
// rows already explicitly say they are not dispatched in RenderTracker. Registered
// rows must earn every selector against the same router ingest uses. Only synthetic
// path/content witnesses are used; no calibration or external tools run here.
func TestBoardNamesOnlyDispatchedFiles(t *testing.T) {
	in, err := TrackerInputFromRepo("../..")
	if err != nil {
		t.Fatal(err)
	}
	p := fusion.New(nil, nil, fusion.Config{})
	defer p.Close()
	check := boardNameChecker{in: in, router: p.Router, table: fusion.DispatchTable(), rules: langroute.DialectRules()}
	t.Run("controls", func(t *testing.T) {
		for _, c := range []struct {
			id, name, selector string
			bad                bool
		}{
			{"python", "Python (.py)", "", false},
			{"python", "Python (.py .pyi)", ".pyi", true},
			{"python-packaging", "Python packaging (pyproject.toml)", "", false},
			{"python-packaging", "Python packaging (pyproject.toml, requirements*.txt)", "requirements*.txt", true},
			{"python-packaging", "Python packaging (pyproject.toml, setup.cfg)", "setup.cfg", true},
			{"ruby", "Ruby (.rb, Rakefile by basename)", "", false},
			{"ruby", "Ruby (.rb, Gemfile)", "Gemfile", true},
			{"python", "Python (.pyi not dispatched)", "", false},
			{"python", "Python (.py not dispatched)", "contradicts route", true},
			{"python", "Python (.rb)", "another dialect", true},
			{"terraform", "Terraform (.tf.json)", "ownership", true},
			{"python", "Python (.pyi by telepathy)", "unknown qualification", true},
			{"python", "Python (.pyi?)", "unknown selector syntax", true},
			{"python", "Python (.pyi", "unbalanced", true},
			{"makefile", "Make (Makefile, *.mk, GNUmakefile)", "", false},
			{"nginx", "nginx (.conf by sniff, sites-enabled/* by glob)", "", false},
			{"nginx", "nginx (.conf by sniff, sites-disabled/* by glob)", "sites-disabled/*", true},
			{"matlab", "MATLAB (.m by sniff)", "", false},
			{"python", "Python (.py by glob)", "no declared glob", true},
		} {
			t.Run(c.id+"/"+c.name, func(t *testing.T) {
				errs, _, _ := check.errors(QueueEntry{ID: c.id, Name: c.name})
				got := strings.Join(errs, "\n")
				if c.bad && (len(errs) == 0 || !strings.Contains(got, c.selector)) {
					t.Errorf("want rejection containing %q, got %q", c.selector, got)
				}
				if !c.bad && len(errs) != 0 {
					t.Errorf("valid claim rejected: %s", got)
				}
			})
		}
		// The same selector changes meaning when its row is explicitly refused or
		// unregistered. This exercises the qualification, not a queue-id exception.
		for _, state := range []string{"refused", "unsupported"} {
			t.Run(state, func(t *testing.T) {
				qualified := check
				qualified.in = TrackerInput{Registered: map[string]Registration{}, Refused: map[string]string{}}
				if state == "refused" {
					qualified.in.Registered["python"] = in.Registered["python"]
					qualified.in.Refused["python"] = "LB-Q-python: refused control"
				}
				errs, _, qualification := qualified.errors(QueueEntry{ID: "python", Name: "Python (.pyi, Gemfile, requirements*.txt)"})
				if len(errs) != 0 || qualification != state {
					t.Errorf("%s: errors=%v qualification=%q", state, errs, qualification)
				}
				// Qualification never excuses an unrecognized name grammar.
				errs, _, _ = qualified.errors(QueueEntry{ID: "python", Name: "Python (.pyi by magic)"})
				if len(errs) == 0 {
					t.Error("qualification hid unknown syntax")
				}
			})
		}
	})
	t.Run("queue", func(t *testing.T) {
		for _, e := range in.Queue {
			t.Run(e.ID, func(t *testing.T) {
				errs, n, qualification := check.errors(e)
				t.Logf("selectors=%d qualification=%s name=%q", n, qualification, e.Name)
				for _, err := range errs {
					t.Error(err)
				}
			})
		}
		t.Logf("covered all %d queue entries", len(in.Queue))
	})
}

type boardSelector struct{ text, mode string }
type boardNameChecker struct {
	in     TrackerInput
	router *langroute.Router
	table  map[string]string
	rules  []langclass.DialectRule
}

// These are descriptive clauses, not file selectors. The grammar is deliberately
// closed: a new clause must be reviewed, never silently discarded by a regex.
// There are no queue IDs here, and a changed filename cannot match a prose clause.
var boardProse = strings.Split("schema and migrations|workspaces|shebangs|incl. CRDs|included files|indented syntax|routes|receivers|inhibit rules|time intervals|dashboard JSON|v2 resources|providers|datasources|alerting|templates|Nomad jobspecs|Starlark|bootstrap|xDS resource files|processors|exporters|connectors|extensions|pipelines|decK|DB-less|Liquibase changelogs|by content|programs ride their language|Kustomization|HelmRelease|GitRepository|OCIRepository|ClusterPolicy|Policy|PolicyException|policies ride Rego|JSON by content|inline in CloudFormation and Terraform|policyRule JSON|policies YAML|YAML graph checks|Python checks|Alembic|Rails db/migrate|Django migrations|EF Core migrations", "|")
var boardToken = regexp.MustCompile(`^[\pL\pN\pS_.*+/<>{}-]+$`)

func boardSelectors(name string) ([]boardSelector, []string) {
	var out []boardSelector
	var errs []string
	depth, start := 0, 0
	var groups []string
	for i, r := range name {
		switch r {
		case '(':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, []string{"unbalanced name parentheses"}
			}
			if depth == 0 {
				groups = append(groups, name[start:i])
			}
		}
	}
	if depth != 0 {
		return nil, []string{"unbalanced name parentheses"}
	}
	for _, g := range groups {
		for _, clause := range strings.FieldsFunc(g, func(r rune) bool { return r == ',' || r == ';' }) {
			clause = strings.TrimSpace(clause)
			if slices.Contains(boardProse, clause) {
				continue
			}
			mode := "plain"
			if before, after, ok := strings.Cut(clause, " by "); ok {
				clause = before
				mode = after
				if !slices.Contains([]string{"sniff", "content sniff", "glob", "basename"}, mode) {
					errs = append(errs, "unknown qualification: "+after)
					continue
				}
			}
			if before, ok := strings.CutSuffix(clause, " not dispatched"); ok {
				clause = before
				mode = "not dispatched"
			}
			// A few clauses name the tool or the condition before/after the selector.
			clause = strings.TrimPrefix(clause, "go-task: ")
			clause = strings.TrimPrefix(clause, "Packer ")
			clause = strings.TrimPrefix(clause, "Flyway ")
			clause = strings.TrimPrefix(clause, "a ")
			clause = strings.TrimSuffix(clause, " naming")
			clause = strings.TrimSuffix(clause, " shape")
			clause = strings.TrimSuffix(clause, " with ref()")
			for _, token := range strings.Fields(clause) {
				if token == "and" {
					continue
				}
				if !boardToken.MatchString(token) || strings.ContainsAny(token, "{}") || strings.Contains(token, "…") {
					errs = append(errs, "unknown selector syntax: "+token)
					continue
				}
				// The only symbolic filename currently used by the queue.
				if strings.ContainsAny(token, "<>") && !strings.Contains(token, "<stack>") {
					errs = append(errs, "unknown selector syntax: "+token)
					continue
				}
				out = append(out, boardSelector{token, mode})
			}
		}
	}
	// Extensionless filenames can be the entire title (Dockerfile / Containerfile).
	// Other titles describe a language/tool; parentheses above carry their selectors.
	title, _, _ := strings.Cut(name, " (")
	if len(out) == 0 {
		for _, titlePart := range strings.Split(title, " / ") {
			if !strings.Contains(titlePart, " ") && strings.HasSuffix(strings.ToLower(titlePart), "file") {
				out = append(out, boardSelector{titlePart, "plain"})
			}
		}
	}
	return out, errs
}

func (c boardNameChecker) errors(e QueueEntry) ([]string, int, string) {
	selectors, errs := boardSelectors(e.Name)
	qualification := "registered"
	if _, ok := c.in.Dispatched()[e.ID]; !ok {
		qualification = "unsupported"
		if _, ok := c.in.Refused[e.ID]; ok {
			qualification = "refused"
		}
	}
	for _, s := range selectors {
		if qualification != "registered" {
			continue
		} // explicit existing tracker cell, not name trust
		if err := c.selectorError(e.ID, s); err != "" {
			errs = append(errs, fmt.Sprintf("%s selector %q: %s", e.ID, s.text, err))
		}
	}
	return errs, len(selectors), qualification
}

func boardPath(selector string) string {
	selector = strings.ReplaceAll(selector, "<stack>", "example")
	selector = strings.ReplaceAll(selector, "**/", "")
	selector = strings.ReplaceAll(selector, "*", "board")
	if strings.HasPrefix(selector, ".") && !strings.Contains(selector, "/") {
		return "board" + selector
	}
	return selector
}

func (c boardNameChecker) selectorError(id string, s boardSelector) string {
	rel := boardPath(s.text)
	content := []byte(boardSniffContent[id])
	for _, r := range c.rules {
		if r.ID == id && slices.Contains(r.Basenames, s.text) {
			rel = s.text
		}
	}
	// Qualified content claims have explicit synthetic witnesses. Missing fixture
	// coverage fails visibly. The fixture must win classification and real dispatch.
	if s.mode == "sniff" || s.mode == "content sniff" {
		content = []byte(boardSniffContent[id])
		if len(content) == 0 {
			return "no synthetic content witness for qualified sniff"
		}
	}
	if s.mode == "glob" && strings.HasPrefix(s.text, ".") {
		found := false
		for _, r := range c.rules {
			if r.ID == id {
				for _, g := range r.Globs {
					candidate := boardPath(g)
					if strings.HasSuffix(candidate, s.text) {
						rel = candidate
						found = true
						break
					}
				}
			}
		}
		if !found {
			return "no declared glob for suffix"
		}
	}
	// Helm templates require a chart ancestor: the declaration alone does not
	// establish that context. The shared router consumes this synthetic sibling.
	idx := langroute.BuildSiblingIndex([]core.SourceRef{{Path: "Chart.yaml"}, {Path: rel}})
	ex, kind, ok, unclassified := c.router.ResolveExtractor(rel, content, idx)
	verdict := langclass.ClassifyDialect(c.rules, rel, content)
	label := c.table[s.text]
	if strings.HasPrefix(s.text, "*.") {
		label = c.table[strings.TrimPrefix(s.text, "*")]
	}
	witness := fmt.Sprintf("path=%q dialect=%q by=%q extractor=%T kind=%q ok=%v unclassified=%v table=%q", rel, verdict.Dialect, verdict.By, ex, kind, ok, unclassified, label)
	if s.mode == "not dispatched" {
		if ok && (verdict.Dialect == id || strings.Contains(label, "`"+id+"`")) {
			return "non-dispatch qualification contradicts route: " + witness
		}
		return ""
	}
	if !ok {
		return "no actual dispatch: " + witness
	}

	if s.mode == "sniff" || s.mode == "content sniff" {
		if h, header := ex.(*extractor.CHeaderExtractor); header {
			selected, yes := h.Route(content).(*extractor.TSExtractor)
			if !yes || selected.Name() != id {
				return "header content witness does not select claimed dialect: " + witness
			}
		} else if verdict.Dialect != id || (verdict.By != langclass.ClassifiedBySniff && verdict.By != langclass.ClassifiedByExtension) {
			return "content witness does not select claimed dialect: " + witness
		}
	}
	if s.mode == "glob" && (verdict.Dialect != id || verdict.By != langclass.ClassifiedByGlob) {
		return "glob witness does not select claimed dialect: " + witness
	}
	if s.mode == "basename" && (verdict.Dialect != id || verdict.By != langclass.ClassifiedByBasename) {
		return "basename witness does not select claimed dialect: " + witness
	}
	// A different dialect dispatching this filename does not substantiate this row.
	if verdict.Dialect != "" && verdict.Dialect != id && !strings.Contains(label, "`"+id+"`") {
		return "dispatch belongs to another dialect: " + witness
	}
	// Require evidence of ownership as well as existence. In particular, generic
	// JSON dispatch cannot substantiate Terraform's multi-dot .tf.json claim.
	key := path.Base(rel)
	registered := c.router.Exts()[key]
	if registered == nil {
		key = path.Ext(rel)
		registered = c.router.Exts()[key]
	}
	owns := false
	switch x := ex.(type) {
	case *extractor.TSExtractor:
		owns = x.Name() == id || (verdict.Dialect == id && registered == ex)
	case *extractor.CHeaderExtractor:
		selected, yes := x.Route(content).(*extractor.TSExtractor)
		owns = yes && selected.Name() == id
	default:
		owns = string(kind) == id || (verdict.Dialect == id && (c.router.SniffExts()[id] == ex || strings.HasPrefix(string(kind), id+"-")))
		if registered == ex {
			owns = owns || verdict.Dialect == id
			actualLabel := c.table[key]
			owns = owns || strings.Contains(actualLabel, "`"+id+"`")
			reg := c.in.Registered[id]
			// Legacy native walkers have no dialect id; their registered language
			// tag is compared only after actual direct-registry dispatch is proven.
			owns = owns || (reg.LangTag != "" && string(langclass.LanguageForPath(rel)) == reg.LangTag && !strings.Contains(actualLabel, "dialect"))
		}
	}
	if !owns {
		return "dispatch ownership is not established: " + witness
	}
	if strings.Contains(label, "refused") {
		return "refused registry dispatch: " + witness
	}
	return ""
}

// Explicit, small test data. This is not a second dispatcher: every assertion
// still asks ClassifyDialect and ResolveExtractor, and missing data is an error.
var boardSniffContent = map[string]string{
	"c":                "int board(void);\n",
	"cpp":              "template<class T> class Board {};\n",
	"matlab":           "function board()\nend\n",
	"objective-c":      "@interface Board\n@end\n",
	"perl":             "use strict;\n",
	"glsl":             "#version 330\nvoid main() {}\n",
	"nginx":            "server { listen 80; }\n",
	"haproxy":          "global\n",
	"terragrunt":       "include { path = \"root.hcl\" }\n",
	"fsharp":           "module Board\nlet board = 1\n",
	"pascal":           "program Board; begin end.\n",
	"prolog":           "board(X) :- member(X, []).\n",
	"vba":              "Attribute VB_Name = \"Board\"\nSub Board()\nEnd Sub\n",
	"verilog":          "module board; endmodule\n",
	"latex":            "\\documentclass{article}\n\\begin{document}Board\\end{document}\n",
	"fathom-plan-rows": "- [ ] Board {id: BOARD, oracle: lexical, path: board.go, pattern: board}\n",
	"arm-templates":    `{"$schema":"https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#","resources":[{"type":"Microsoft.Storage/storageAccounts","apiVersion":"2023-01-01","name":"board","location":"west","properties":{}}]}`,
	"hlsl":             "cbuffer Board { float value; };\n",
	"azure-pipelines":  "steps:\n- script: echo board\n",
	"pulumi-yaml":      "config:\n  board:key: value\n",
	"pgbouncer":        "[databases]\nboard = host=localhost dbname=board\n",
	"proxysql":         "admin_variables={admin_credentials=\"board:board\";}\n",
	"go-template":      "{{ define \"board\" }}{{ .Name }}{{ end }}\n",
	"sql-dialects":     "CREATE OR REPLACE PACKAGE board AS END;\n",
}
