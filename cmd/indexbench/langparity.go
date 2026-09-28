package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// incumbentExtractors names the bespoke extractors a port item retires
// (design §11, the queue rows' "retires <file>"), by their source file's
// stem. -incumbent picks one; empty measures fusion's own dispatch.
var incumbentExtractors = map[string]func() extractor.Extractor{
	"bashextract":       func() extractor.Extractor { return extractor.NewBashExtractor() },
	"cextract":          func() extractor.Extractor { return extractor.NewCExtractor() },
	"cheaderextract":    func() extractor.Extractor { return extractor.NewCHeaderExtractor() },
	"cmakeextract":      func() extractor.Extractor { return extractor.NewCMakeExtractor() },
	"composeextract":    func() extractor.Extractor { return extractor.NewComposeExtractor() },
	"cppextract":        func() extractor.Extractor { return extractor.NewCppExtractor() },
	"csharpextract":     func() extractor.Extractor { return extractor.NewCSharpExtractor() },
	"cueextract":        func() extractor.Extractor { return extractor.NewCUEExtractor() },
	"dockerextract":     func() extractor.Extractor { return extractor.NewDockerfileExtractor() },
	"ghaextract":        func() extractor.Extractor { return extractor.NewGHAExtractor() },
	"gomodextract":      func() extractor.Extractor { return extractor.NewGoModExtractor() },
	"graphqlextract":    func() extractor.Extractor { return extractor.NewGraphQLExtractor() },
	"helmchartextract":  func() extractor.Extractor { return extractor.NewHelmChartExtractor() },
	"helmtemplateyaml":  func() extractor.Extractor { return extractor.NewHelmTemplateYAMLExtractor() },
	"helmtplextract":    func() extractor.Extractor { return extractor.NewHelmTemplateExtractor() },
	"helmvaluesextract": func() extractor.Extractor { return extractor.NewHelmValuesExtractor() },
	"javaextract":       func() extractor.Extractor { return extractor.NewJavaExtractor() },
	"jsonextract":       func() extractor.Extractor { return extractor.NewJSONExtractor() },
	"k8sextract":        func() extractor.Extractor { return extractor.NewK8sExtractor() },
	"kustomizeextract":  func() extractor.Extractor { return extractor.NewKustomizeExtractor() },
	"luaextract":        func() extractor.Extractor { return extractor.NewLuaExtractor() },
	"mdextract":         func() extractor.Extractor { return extractor.NewMarkdownExtractor() },
	"openapiextract":    func() extractor.Extractor { return extractor.NewOpenAPIExtractor() },
	"phpextract":        func() extractor.Extractor { return extractor.NewPhpExtractor() },
	"protoextract":      func() extractor.Extractor { return extractor.NewProtoExtractor() },
	"rustextract":       func() extractor.Extractor { return extractor.NewRustExtractor() },
	"scalaextract":      func() extractor.Extractor { return extractor.NewScalaExtractor() },
	"sqlextract":        func() extractor.Extractor { return extractor.NewSQLExtractor() },
	"templextract":      func() extractor.Extractor { return extractor.NewTemplExtractor() },
	"tomlextract":       func() extractor.Extractor { return extractor.NewTOMLExtractor() },
}

// multiFlag is a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

// cmdLangParity is `indexbench lang-parity` (LB-F18, design §11 step 2): a
// port's dialect against the incumbent it retires, on the same subjects and
// the same oracles, per-metric new minus old and the SourceKind diff. The
// exit code is the merge rule: 0 when every L1 metric is at least the
// incumbent's and the dialect meets its target level, 1 when not, 2 on a
// usage or operational error.
func cmdLangParity(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-parity", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dialect := fs.String("dialect", "", "dialect id (required; its internal/extractor/dialects/<id>/dialect.json)")
	manifestPath := fs.String("manifest", "", "the dialect's dialect.json (default: internal/extractor/dialects/<id>/dialect.json)")
	incumbent := fs.String("incumbent", "", "the incumbent extractor by source stem (e.g. dockerextract); empty = what fusion dispatches today")
	var subjects, scips, toolOracles multiFlag
	fs.Var(&subjects, "subject", "subject checkout, `[name=]dir` (repeatable; required)")
	fs.Var(&scips, "scip", "SCIP index for a subject, `[name=]path` (repeatable)")
	fs.Var(&toolOracles, "tool-oracle", "tool-native oracle JSON for a subject, `[name=]path` (repeatable)")
	ctagsPath := fs.String("ctags", "", "universal-ctags binary (as lang-oracle)")
	queuePath := fs.String("queue", "docs/plans/language-breadth.queue.json", "the queue (for the dialect's target level)")
	target := fs.String("target", "", "target level (default: the queue entry's)")
	out := fs.String("out", "", "write the parity report as JSON (optional)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dialect == "" || len(subjects) == 0 || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: indexbench lang-parity -dialect <id> [-incumbent <extractor>] -subject [name=]dir ... [-ctags <bin>] [-scip [name=]path] [-tool-oracle [name=]path] [-out <report.json>]")
		return 2
	}
	spec, source, err := resolveBenchSpec(*dialect, *manifestPath)
	if err != nil || source != "dialect" {
		if err == nil {
			err = fmt.Errorf("dialect %q has no dialect.json: a port measures a written dialect against its incumbent (§11 step 1)", *dialect)
		}
		fmt.Fprintln(stderr, "lang-parity:", err)
		return 2
	}
	dialX, err := dialectExtractor(spec.Manifest)
	if err != nil {
		fmt.Fprintln(stderr, "lang-parity:", err)
		return 2
	}
	var incX extractor.Extractor
	incName := *incumbent
	if incName != "" {
		mk, ok := incumbentExtractors[incName]
		if !ok {
			fmt.Fprintf(stderr, "lang-parity: unknown incumbent %q (known: %s)\n", incName, strings.Join(sortedKeys(incumbentExtractors), ", "))
			return 2
		}
		incX = mk()
	} else {
		incName = "fusion dispatch"
	}
	tgt := *target
	if tgt == "" {
		tgt = spec.Manifest.TargetLevel
		if q, qerr := langbench.LoadQueue(*queuePath); qerr == nil {
			for _, e := range q {
				if e.ID == spec.Manifest.ID {
					tgt = e.TargetLevel
				}
			}
		}
	}
	if tgt == "" {
		tgt = "L1"
	}

	type subj struct{ name, dir, scip, tool string }
	var subs []subj
	byName := map[string]int{}
	for _, s := range subjects {
		name, dir := splitNamed(s)
		if name == "" {
			name = filepath.Base(filepath.Clean(dir))
		}
		if _, dup := byName[name]; dup {
			fmt.Fprintf(stderr, "lang-parity: subject %q given twice\n", name)
			return 2
		}
		byName[name] = len(subs)
		subs = append(subs, subj{name: name, dir: dir})
	}
	attach := func(vals []string, set func(*subj, string)) error {
		for _, v := range vals {
			name, path := splitNamed(v)
			if name == "" {
				if len(subs) != 1 {
					return fmt.Errorf("%q: name the subject (name=path) when measuring more than one", v)
				}
				name = subs[0].name
			}
			i, ok := byName[name]
			if !ok {
				return fmt.Errorf("%q: no subject named %q", v, name)
			}
			set(&subs[i], path)
		}
		return nil
	}
	if err := attach(scips, func(s *subj, p string) { s.scip = p }); err != nil {
		fmt.Fprintln(stderr, "lang-parity:", err)
		return 2
	}
	if err := attach(toolOracles, func(s *subj, p string) { s.tool = p }); err != nil {
		fmt.Fprintln(stderr, "lang-parity:", err)
		return 2
	}

	// The incumbent's own kinds (its LB-M1 spec, when it has one) join the
	// dialect's for scoring its side, so its output is judged by the same
	// oracle mapping rather than excluded wholesale.
	incSpec := spec
	if inc, ok := langbench.IncumbentSpec(spec.Manifest.ID); ok {
		incSpec.Manifest = spec.Manifest.WithKindsOf(inc.Manifest)
	}
	ctx := context.Background()
	incR := langbench.NewReading(spec.Manifest, tgt)
	dialR := langbench.NewReading(spec.Manifest, tgt)
	incKinds, dialKinds := map[string]int{}, map[string]int{}
	for _, s := range subs {
		for _, side := range []struct {
			spec  langbench.BenchSpec
			x     extractor.Extractor
			r     *langbench.Reading
			kinds map[string]int
		}{{incSpec, incX, &incR, incKinds}, {spec, dialX, &dialR, dialKinds}} {
			m, err := measureSubject(ctx, side.spec, s.dir, s.name, *ctagsPath, s.scip, s.tool, side.x)
			if err != nil {
				fmt.Fprintln(stderr, "lang-parity:", err)
				return 2
			}
			side.r.Oracles = mergeOracles(side.r.Oracles, m.oracles)
			langbench.MergeSubject(side.r, m.subject)
			addCounts(side.kinds, m.kinds)
		}
	}
	p := langbench.ComparePortParity(incR, dialR, incName, tgt, incKinds, dialKinds)
	if err := dialR.Validate(); err != nil {
		p.Merge = false
		p.Reasons = append(p.Reasons, "the dialect's reading would be refused: "+err.Error())
	}
	writeParity(stdout, p)
	if *out != "" {
		b, err := json.MarshalIndent(p, "", "  ")
		if err == nil {
			err = os.WriteFile(*out, append(b, '\n'), 0o644)
		}
		if err != nil {
			fmt.Fprintln(stderr, "lang-parity:", err)
			return 2
		}
	}
	if !p.Merge {
		return 1
	}
	return 0
}

// dialectExtractor builds the dialect's own extractor from its engine,
// independent of what fusion dispatches.
func dialectExtractor(m langbench.Manifest) (extractor.Extractor, error) {
	switch m.Engine {
	case "tags":
		return extractor.NewTSExtractor(m.ID)
	case "doc":
		return extractor.NewDocDialectExtractor(m.ID)
	}
	return nil, fmt.Errorf("dialect %s: engine %q has no stand-alone extractor for a parity run (tags and doc do)", m.ID, m.Engine)
}

func splitNamed(v string) (name, path string) {
	if i := strings.Index(v, "="); i > 0 {
		return v[:i], v[i+1:]
	}
	return "", v
}

func fmtMetric(v *float64) string {
	if v == nil {
		return "—"
	}
	return fmt.Sprintf("%.4f", *v)
}

func writeParity(w io.Writer, p langbench.Parity) {
	fmt.Fprintf(w, "lang-parity %s vs %s (target %s)\n", p.Dialect, p.Incumbent, p.Target)
	fmt.Fprintf(w, "  %-20s %10s %10s %10s\n", "metric", "incumbent", "dialect", "delta")
	for _, m := range p.Metrics {
		d := "—"
		if m.Delta != nil {
			d = fmt.Sprintf("%+.4f", *m.Delta)
		}
		gate := ""
		if m.Gated {
			gate = "  (gated)"
		}
		fmt.Fprintf(w, "  %-20s %10s %10s %10s%s\n", m.Metric, fmtMetric(m.Incumbent), fmtMetric(m.Dialect), d, gate)
	}
	fmt.Fprintf(w, "  %-20s %10s %10s\n", "level_met", p.IncumbentLevel, p.DialectLevel)
	if len(p.KindsLost) > 0 {
		fmt.Fprintf(w, "kinds lost: %s (a regression unless the port row's reason names why, §11 step 3)\n", strings.Join(p.KindsLost, ", "))
	}
	if len(p.KindsGained) > 0 {
		fmt.Fprintf(w, "kinds gained: %s\n", strings.Join(p.KindsGained, ", "))
	}
	if p.Merge {
		fmt.Fprintln(w, "verdict: MERGE (every L1 metric >= the incumbent's; target met)")
		return
	}
	reasons := append([]string(nil), p.Reasons...)
	sort.Strings(reasons)
	fmt.Fprintf(w, "verdict: REFUSED — %s\n", strings.Join(reasons, "; "))
}
