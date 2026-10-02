package main

// tiera.go — the tier-A path (wave ox, LB-F154): Helm itself decides what a
// chart holds, for the charts it renders offline.
//
//   - Scope: a chart Helm loads (helm.sh/helm/v4 v4.3.0's own loader,
//     loader.LoadDir: the code `helm show chart`, `helm show values` and
//     `helm template` run) whose every dependency is present under charts/
//     (vendored, or a file:// dependency unpacked there), and which
//     `helm template` renders offline. A chart with a missing dependency (an
//     oci:// registry or a chart repository URL, which this oracle never
//     fetches) or a render error contributes nothing and is counted by reason.
//   - The chart and its dependencies are Helm's metadata; the values keys are
//     Helm's values tree (loader.LoadValues), one key path per key down
//     valuesMaxDepth descending only into non-empty maps (the dialect's rule);
//     the template files are those Helm loads (templates/, .helmignore
//     applied).
//   - Template entities are Go's text/template/parse nodes, the parser Helm
//     renders with, over the files Helm loaded and rendered: a named template,
//     an include/template call and a .Values accessor. Each define is attested
//     by ablation: in a scratch copy its name is renamed and a probe template
//     includes the original name; Helm's render must fail with "no template
//     <name> associated" (a define shadowed by another of the same name does
//     not fail and is unevidenced). A call is attested when a define of its
//     name is, in the same chart, a subchart or a parent; a call to anything
//     else is unevidenced. Unevidenced entries are counted on stderr, never in
//     the truth: the dialect pays for emitting them in precision.
//   - Lines: the parse tree's positions for template entities, and for
//     Chart.yaml and values.yaml an independent gopkg.in/yaml.v3 reader by
//     key path (the doc engine's library too: a line is a decoder detail,
//     never an entity decision, as calibration/oracles/envoy's README says).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	hchart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
)

// astats are the tier-A path's counts, printed on stderr.
type astats struct {
	charts, rendered                     int
	outOfScope                           map[string]int
	defines, attested, unevidencedDefine int
	calls, unevidencedCalls              int
	keysHelm, keysUnlocated, keysWalk    int
	nameMismatch, notLoaded              int
}

type tierAResult struct {
	defs  []def
	refs  []ref
	files []string
	st    astats
}

// valueKeyPaths names every key of a values tree down maxDepth, descending
// only into non-empty maps.
func valueKeyPaths(vals map[string]any, maxDepth int) []string {
	var out []string
	var walk func(m map[string]any, prefix string, depth int)
	walk = func(m map[string]any, prefix string, depth int) {
		if depth > maxDepth {
			return
		}
		for _, k := range slices.Sorted(maps.Keys(m)) {
			if k == "" {
				continue
			}
			name := k
			if prefix != "" {
				name = prefix + "." + k
			}
			out = append(out, name)
			if sub, ok := m[k].(map[string]any); ok && len(sub) > 0 {
				walk(sub, name, depth+1)
			}
		}
	}
	walk(vals, "", 1)
	return out
}

// ablateDefine renames the define (or block) of name in src to repl.
func ablateDefine(src, name, repl string) (string, bool) {
	re := regexp.MustCompile("(\\{\\{-?\\s*(?:define|block)\\s+)(?:\"" + regexp.QuoteMeta(name) + "\"|`" + regexp.QuoteMeta(name) + "`)")
	loc := re.FindStringSubmatchIndex(src)
	if loc == nil {
		return src, false
	}
	return src[:loc[3]] + strconv.Quote(repl) + src[loc[1]:], true
}

// visible reports whether a define in chart directory defChart can serve a
// call in chart directory callChart: the same chart, a subchart or a parent.
func visible(callChart, defChart string) bool {
	under := func(a, b string) bool { return a == b || strings.HasPrefix(a, b+"/") }
	return under(callChart, defChart) || under(defChart, callChart)
}

// helmEnv keeps Helm's caches and configuration out of the subject and HOME.
func helmEnv(scratch string) []string {
	return append(os.Environ(),
		"HELM_CACHE_HOME="+filepath.Join(scratch, "helm-cache"),
		"HELM_CONFIG_HOME="+filepath.Join(scratch, "helm-config"),
		"HELM_DATA_HOME="+filepath.Join(scratch, "helm-data"),
		"KUBECONFIG=/dev/null")
}

// render runs `helm template` on dir; the error carries Helm's first error line.
func render(bin, dir string, env []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "template", "ox", dir)
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = nil, &stderr
	if err := cmd.Run(); err != nil {
		// Helm 4 spreads one error over several lines: keep them all, joined,
		// from the "Error:" line on.
		msg := strings.TrimSpace(stderr.String())
		if i := strings.Index(msg, "Error:"); i >= 0 {
			msg = msg[i:]
		}
		msg, _, _ = strings.Cut(msg, "\n\nUse --debug")
		return fmt.Errorf("%v: %s", err, strings.Join(strings.Fields(msg), " "))
	}
	return nil
}

// achart is one chart directory the tier-A path measures.
type achart struct {
	dir     string // subject-relative
	defs    []def
	refs    []ref
	files   []string
	defines []def // the chart's define entities, to attest
	calls   []def // its include/template calls
	attest  map[string]bool
	reason  string // out of scope: why
}

// tierA measures every chart of the subject Helm loads and renders offline.
func tierA(subject, bin, scratch string, workers int) (tierAResult, error) {
	res := tierAResult{st: astats{outOfScope: map[string]int{}}}
	var dirs []string
	err := filepath.WalkDir(subject, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if !d.IsDir() && d.Name() == "Chart.yaml" {
			rel, _ := filepath.Rel(subject, filepath.Dir(p))
			dirs = append(dirs, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	sort.Strings(dirs)
	env := helmEnv(scratch)
	parallel := func(f func(i int, dir string)) {
		sem := make(chan struct{}, max(1, workers))
		var wg sync.WaitGroup
		for i, dir := range dirs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				f(i, dir)
			}()
		}
		wg.Wait()
	}
	// pass 1: which charts Helm loads and renders on their own
	scopes := make([]chartScope, len(dirs))
	parallel(func(i int, dir string) { scopes[i] = scopeChart(subject, dir, bin, env) })
	// a subchart that renders only inside its parent (it reads the parent's
	// values) is measured by the parent's render: Helm loads it from charts/
	for i, dir := range dirs {
		if scopes[i].reason != "render-error" {
			continue
		}
		for j := len(dirs) - 1; j >= 0; j-- {
			if j != i && scopes[j].reason == "" && scopes[j].root == dirs[j] && strings.HasPrefix(dir, dirs[j]+"/charts/") {
				fmt.Fprintf(os.Stderr, "helm-oracle: %s renders inside its parent %s\n", dir, dirs[j])
				scopes[i].reason, scopes[i].root = "", dirs[j]
				break
			}
		}
	}
	charts := make([]*achart, len(dirs))
	var mu sync.Mutex
	parallel(func(i int, dir string) {
		charts[i] = measureChart(subject, dir, scopes[i], dirs, bin, filepath.Join(scratch, fmt.Sprintf("chart-%d", i)), env, &mu, &res.st)
	})
	res.st.charts = len(dirs)
	for _, c := range charts {
		if c.reason != "" {
			res.st.outOfScope[c.reason]++
			continue
		}
		res.st.rendered++
		res.defs = append(res.defs, c.defs...)
		res.files = append(res.files, c.files...)
		res.st.defines += len(c.defines)
		for _, d := range c.defines {
			if c.attest[d.Path+"\x00"+d.Name] {
				res.st.attested++
				res.defs = append(res.defs, d)
			} else {
				res.st.unevidencedDefine++
				fmt.Fprintf(os.Stderr, "helm-oracle: unevidenced define %s:%d %s\n", d.Path, d.Line, d.Name)
			}
		}
	}
	// attested names by chart directory, for the calls
	attested := map[string][]string{}
	for _, c := range charts {
		if c.reason != "" {
			continue
		}
		for _, d := range c.defines {
			if c.attest[d.Path+"\x00"+d.Name] {
				attested[d.Name] = append(attested[d.Name], c.dir)
			}
		}
	}
	served := func(chartDir, name string) bool {
		for _, dd := range attested[name] {
			if visible(chartDir, dd) {
				return true
			}
		}
		return false
	}
	for _, c := range charts {
		if c.reason != "" {
			continue
		}
		for _, call := range c.calls {
			res.st.calls++
			if served(c.dir, call.Name) {
				res.defs = append(res.defs, call)
			} else {
				res.st.unevidencedCalls++
			}
		}
		for _, r := range c.refs {
			if served(c.dir, r.To) {
				res.refs = append(res.refs, r)
			}
		}
	}
	sort.Slice(res.defs, func(i, j int) bool {
		a, b := res.defs[i], res.defs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Kind < b.Kind
	})
	sort.Slice(res.refs, func(i, j int) bool {
		a, b := res.refs[i], res.refs[j]
		return a.Path+"\x00"+a.From+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.To
	})
	sort.Strings(res.files)
	return res, nil
}

// ownChart is the innermost chart directory holding rel.
func ownChart(rel string, dirs []string) string {
	best := ""
	for _, d := range dirs {
		if (d == "." || rel == d || strings.HasPrefix(rel, d+"/")) && len(d) >= len(best) {
			best = d
		}
	}
	return best
}

func joinRel(dir, name string) string {
	if dir == "." {
		return name
	}
	return dir + "/" + name
}

// chartScope is one chart's standing: loaded by Helm (ch), the chart
// directory whose render covers it (root: itself, or a parent), or why it is
// out of scope.
type chartScope struct {
	ch     *hchart.Chart
	root   string
	reason string
}

// scopeChart loads a chart with Helm's loader, checks its dependencies are
// all under charts/ (as `helm template` does) and renders it offline.
func scopeChart(subject, dir, bin string, env []string) chartScope {
	abs := filepath.Join(subject, filepath.FromSlash(dir))
	ch, err := loader.LoadDir(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "helm-oracle: helm cannot load %s: %v\n", dir, err)
		return chartScope{reason: "unloadable"}
	}
	for _, d := range ch.Metadata.Dependencies {
		found := false
		for _, sc := range ch.Dependencies() {
			if sc.Name() == d.Name {
				found = true
			}
		}
		if !found {
			return chartScope{ch: ch, reason: "missing-dependency-" + repoShape(d.Repository)}
		}
	}
	if err := render(bin, abs, env); err != nil {
		fmt.Fprintf(os.Stderr, "helm-oracle: helm template %s: %v\n", dir, err)
		return chartScope{ch: ch, reason: "render-error"}
	}
	return chartScope{ch: ch, root: dir}
}

func measureChart(subject, dir string, sc chartScope, dirs []string, bin, scratch string, env []string, mu *sync.Mutex, st *astats) *achart {
	c := &achart{dir: dir, attest: map[string]bool{}, reason: sc.reason}
	if c.reason != "" {
		return c
	}
	ch := sc.ch
	abs := filepath.Join(subject, filepath.FromSlash(dir))
	// Chart.yaml: Helm's metadata, the walk's lines.
	chartRel := joinRel(dir, "Chart.yaml")
	if b, err := os.ReadFile(filepath.Join(abs, "Chart.yaml")); err == nil {
		var w out
		if _, ok := walkChart(&w, chartRel, b); ok {
			pkgLine := 0
			depLines := map[string][]int{}
			for _, d := range w.Definitions {
				if d.Kind == "package" {
					if d.Name != ch.Metadata.Name {
						mu.Lock()
						st.nameMismatch++
						mu.Unlock()
					}
					pkgLine = d.Line
				} else {
					depLines[d.Name] = append(depLines[d.Name], d.Line)
				}
			}
			c.defs = append(c.defs, def{chartRel, ch.Metadata.Name, pkgLine, "package"})
			for _, d := range ch.Metadata.Dependencies {
				if ls := depLines[d.Name]; len(ls) > 0 {
					c.defs = append(c.defs, def{chartRel, d.Name, ls[0], "data_source"})
					depLines[d.Name] = ls[1:]
				}
			}
			c.files = append(c.files, chartRel)
		}
	}
	// values.yaml: Helm's values tree, the walk's lines.
	valuesRel := joinRel(dir, "values.yaml")
	if b, err := os.ReadFile(filepath.Join(abs, "values.yaml")); err == nil {
		keys := valueKeyPaths(ch.Values, valuesMaxDepth)
		helmKeys := map[string]bool{}
		for _, k := range keys {
			helmKeys[k] = true
		}
		var w out
		walkValues(&w, valuesRel, b)
		placed := map[string]bool{}
		walkOnly := 0
		for _, d := range w.Definitions {
			if helmKeys[d.Name] && !placed[d.Name] {
				placed[d.Name] = true
				c.defs = append(c.defs, d)
			} else if !helmKeys[d.Name] {
				walkOnly++
			}
		}
		mu.Lock()
		st.keysHelm += len(keys)
		st.keysUnlocated += len(keys) - len(placed)
		st.keysWalk += walkOnly
		mu.Unlock()
		c.files = append(c.files, valuesRel)
	}
	// templates: the files Helm loaded, read by Go's parser.
	loaded := map[string]string{}
	for _, f := range ch.Templates {
		rel := joinRel(dir, f.Name)
		if classify(rel) == "template" {
			loaded[rel] = string(f.Data)
		}
	}
	// a template file of this chart the dialect reads and Helm does not load
	_ = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if err == nil && d.IsDir() && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(subject, p)
		rel = filepath.ToSlash(rel)
		if classify(rel) == "template" && ownChart(rel, dirs) == dir {
			if _, ok := loaded[rel]; !ok {
				mu.Lock()
				st.notLoaded++
				mu.Unlock()
			}
		}
		return nil
	})
	var tst stats
	for _, rel := range slices.Sorted(maps.Keys(loaded)) {
		var w out
		walkTemplate(&w, rel, loaded[rel], &tst)
		for _, d := range w.Definitions {
			switch d.Kind {
			case "macro":
				c.defines = append(c.defines, d)
			case "macro_ref":
				c.calls = append(c.calls, d)
			default:
				c.defs = append(c.defs, d)
			}
		}
		c.refs = append(c.refs, w.References...)
		c.files = append(c.files, rel)
	}
	if len(c.defines) == 0 {
		return c
	}
	// ablation, in a scratch copy of the chart whose render covers it
	rootAbs := filepath.Join(subject, filepath.FromSlash(sc.root))
	if err := os.RemoveAll(scratch); err != nil {
		c.reason = "scratch"
		return c
	}
	if err := os.CopyFS(scratch, os.DirFS(rootAbs)); err != nil {
		c.reason = "scratch"
		fmt.Fprintf(os.Stderr, "helm-oracle: copy %s: %v\n", sc.root, err)
		return c
	}
	defer os.RemoveAll(scratch)
	probe := filepath.Join(scratch, "templates", "zz-ox-ablation-probe.yaml")
	for i, d := range c.defines {
		inChart := strings.TrimPrefix(d.Path, sc.root+"/")
		if sc.root == "." {
			inChart = d.Path
		}
		fp := filepath.Join(scratch, filepath.FromSlash(inChart))
		orig, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		ablated, ok := ablateDefine(string(orig), d.Name, fmt.Sprintf("ox-ablated-%d", i))
		if !ok {
			continue
		}
		if os.MkdirAll(filepath.Dir(probe), 0o755) != nil || os.WriteFile(fp, []byte(ablated), 0o644) != nil ||
			os.WriteFile(probe, []byte("{{- $_ := include "+strconv.Quote(d.Name)+" . -}}\n"), 0o644) != nil {
			continue
		}
		err = render(bin, scratch, env)
		if err != nil && strings.Contains(err.Error(), "no template "+strconv.Quote(d.Name)+" associated") {
			c.attest[d.Path+"\x00"+d.Name] = true
		}
		_ = os.WriteFile(fp, orig, 0o644)
		_ = os.Remove(probe)
	}
	return c
}

// defsOut is the tier-A file: definitions and references, no
// cross_references key, so lang-oracle scores no other family at this tier.
type defsOut struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Tier        string `json:"tier"`
	Definitions []def  `json:"definitions"`
	References  []ref  `json:"references"`
}

// helmVersion is `helm version --short`.
func helmVersion(bin string) (string, error) {
	b, err := exec.Command(bin, "version", "--short").Output()
	if err != nil {
		return "", fmt.Errorf("helm version: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// writeTierA runs the tier-A path and writes its three outputs; walk is the
// tier-C walk, whose cross_references go to xrefsOut.
func writeTierA(walk out, subject, bin, scratch, xrefsOut, filesOut string, workers int) error {
	ver, err := helmVersion(bin)
	if err != nil {
		return err
	}
	res, err := tierA(subject, bin, scratch, workers)
	if err != nil {
		return err
	}
	st := res.st
	reasons := slices.Sorted(maps.Keys(st.outOfScope))
	var oos []string
	for _, r := range reasons {
		oos = append(oos, fmt.Sprintf("%s %d", r, st.outOfScope[r]))
	}
	fmt.Fprintf(os.Stderr, "helm-oracle: tier A (helm %s): %d charts, %d rendered offline, out of scope: [%s]; %d defines, %d attested by ablation, %d unevidenced; %d include/template calls, %d unevidenced; %d values keys from Helm, %d unlocated, %d walk-only; %d chart-name mismatches; %d template files Helm does not load; %d definitions, %d references, %d files\n",
		ver, st.charts, st.rendered, strings.Join(oos, ", "), st.defines, st.attested, st.unevidencedDefine, st.calls, st.unevidencedCalls,
		st.keysHelm, st.keysUnlocated, st.keysWalk, st.nameMismatch, st.notLoaded, len(res.defs), len(res.refs), len(res.files))
	if res.defs == nil {
		res.defs = []def{}
	}
	if res.refs == nil {
		res.refs = []ref{}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(defsOut{
		Tool:        "helm (helm.sh/helm/v4 loader, helm template offline with define ablation; templates by Go text/template/parse; lines: gopkg.in/yaml.v3)",
		Version:     "helm " + ver + "; helm.sh/helm/v4 v4.3.0",
		Tier:        "A",
		Definitions: res.defs,
		References:  res.refs,
	}); err != nil {
		return err
	}
	walk.Definitions, walk.References = []def{}, []ref{}
	var buf bytes.Buffer
	xenc := json.NewEncoder(&buf)
	xenc.SetIndent("", " ")
	if err := xenc.Encode(walk); err != nil {
		return err
	}
	if err := os.WriteFile(xrefsOut, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if filesOut != "" {
		return os.WriteFile(filesOut, []byte(strings.Join(res.files, "\n")+"\n"), 0o644)
	}
	return nil
}
