package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// A third-party definitions query as a tier-C L1 oracle (LB-F46, LB-F52;
// ADR-048's D5 extension, design §1.3 and §1.5). A tags.scm, an
// nvim-treesitter locals.scm or a Zed outline.scm, fetched at measurement
// time by pinned revision, runs through fathom's own engine on the
// dialect's pinned grammar; its definitions become a neutral oracle file at
// tier C, scored like any other (scoreToolOracle), so a perfect match
// grades good, never excellent. Definitions only: D5's calls path (an
// independent runtime) is unchanged.

// tagsQueryOpts are lang-oracle's -tags-query flags.
type tagsQueryOpts struct {
	query, rev, source, licence, out, format string
}

// tagsQueryFormats are the capture conventions a query may use.
var tagsQueryFormats = []string{"tags", "locals", "outline"}

// tagsQueryFlags registers the -tags-query flags on lang-oracle's flag set.
func tagsQueryFlags(fs *flag.FlagSet) *tagsQueryOpts {
	o := &tagsQueryOpts{}
	fs.StringVar(&o.query, "tags-query", "", "a third-party definitions query (tags.scm, nvim-treesitter locals.scm or Zed outline.scm) fetched at measurement time into workspace/tmp/oracle/ and run by fathom's engine on the dialect's pinned grammar: the tier-C L1 oracle (ADR-048's D5 extension)")
	fs.StringVar(&o.rev, "tags-query-rev", "", "the 40-hex commit the -tags-query file was fetched at (required with -tags-query)")
	fs.StringVar(&o.source, "tags-query-source", "", `"<owner/repo> <path>" the -tags-query file was fetched from (required with -tags-query)`)
	fs.StringVar(&o.licence, "tags-query-licence", "", "the -tags-query file's SPDX licence, recorded in the reading (required with -tags-query; AGPL is refused)")
	fs.StringVar(&o.out, "tags-query-out", "", "where to write the query's neutral oracle file (default workspace/tmp/oracle/<dialect>-<subject>-tagsq.json)")
	fs.StringVar(&o.format, "tags-query-format", "", "the query's capture convention: tags, locals or outline (default: detected from its capture names)")
	return o
}

// tagsQueryRefusal is an input ADR-048's D5 extension refuses (exit 2), as
// opposed to a run that failed (exit 1).
type tagsQueryRefusal struct{ why string }

func (e *tagsQueryRefusal) Error() string { return "-tags-query refused: " + e.why }

func refusef(format string, args ...any) error {
	return &tagsQueryRefusal{why: fmt.Sprintf(format, args...)}
}

// tagsQueryExit is lang-oracle's exit code for a -tags-query error.
func tagsQueryExit(err error) int {
	var r *tagsQueryRefusal
	if errors.As(err, &r) {
		return 2
	}
	return 1
}

// tqDef is one definition of the neutral oracle file: an alias of
// toolOracleFile's definition type, so the file is written through it.
type tqDef = struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	End  int    `json:"end"`
	Kind string `json:"kind"`
}

// tagsQueryOracle is the -tags-query path (LB-F46): refuse the query per
// the D5 extension's conditions, run it over the subject's files the
// manifest owns, write its definitions as a tier-C neutral oracle file, and
// return tools with that file appended. With no -tags-query, tools is
// returned unchanged. Files the engine could not parse are named on warn.
func tagsQueryOracle(ctx context.Context, o tagsQueryOpts, spec langbench.BenchSpec, manifestPath, root, name string, only, tools []string, warn io.Writer) ([]string, error) {
	if o.query == "" {
		if o.rev+o.source+o.licence+o.out+o.format != "" {
			return nil, refusef("-tags-query-rev, -tags-query-source, -tags-query-licence, -tags-query-out and -tags-query-format need -tags-query")
		}
		return tools, nil
	}
	m := spec.Manifest
	if err := refuseTagsQuery(ctx, o, m.ID, manifestPath); err != nil {
		return nil, err
	}
	if len(m.OracleKinds["tool"]) == 0 {
		return nil, refusef("the %s manifest has no oracle_kinds.tool: map the query's kinds there, as for any neutral oracle file", m.ID)
	}
	b, err := os.ReadFile(o.query)
	if err != nil {
		return nil, err
	}
	format := o.format
	switch {
	case format == "":
		if format, err = detectTagsQueryFormat(string(b)); err != nil {
			return nil, err
		}
	case !slices.Contains(tagsQueryFormats, format):
		return nil, refusef("-tags-query-format %q: want one of %s", format, strings.Join(tagsQueryFormats, ", "))
	}
	defs, err := runTagsQuery(ctx, m, root, only, o.query, string(b), format, warn)
	if err != nil {
		return nil, err
	}
	if len(defs) == 0 {
		return nil, fmt.Errorf("-tags-query %s made no definitions on %s's %s files", o.query, name, m.ID)
	}
	tf := toolOracleFile{Tool: "tags-query/" + format, Tier: "C", Source: o.source, Revision: o.rev, Licence: strings.TrimSpace(o.licence), Definitions: defs}
	out := o.out
	if out == "" {
		slug := strings.NewReplacer("/", "-", " ", "-", string(filepath.Separator), "-").Replace(name)
		out = filepath.Join("workspace", "tmp", "oracle", fmt.Sprintf("%s-%s-tagsq.json", m.ID, slug))
	}
	raw, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(out, append(raw, '\n'), 0o644); err != nil {
		return nil, err
	}
	return append(slices.Clone(tools), out), nil
}

// refuseTagsQuery applies the D5 extension's conditions (LB-F52): (a) the
// query is not the source the dialect was drafted from — no `; source:`
// header of the dialect's own queries names the same repository (any case)
// and path; (c) it is pinned to a 40-hex commit and not vendored — its file
// is not inside this git work tree outside workspace/; (d) its licence is
// given, and it is not AGPL. (b), the tier-C cap, is the oracle file's tier.
func refuseTagsQuery(ctx context.Context, o tagsQueryOpts, dialect, manifestPath string) error {
	repo, qpath, ok := splitQuerySource(o.source)
	if !ok {
		return refusef("-tags-query-source %q: want \"<owner/repo> <path>\"", o.source)
	}
	if !langbench.RevisionPattern.MatchString(o.rev) {
		return refusef("-tags-query-rev %q is not a commit: a query is fetched by pinned revision, 40 lowercase hex (condition c)", o.rev)
	}
	licence := strings.TrimSpace(o.licence)
	if licence == "" {
		return refusef("-tags-query-licence is required: the reading records the query's licence (condition d)")
	}
	if strings.Contains(strings.ToUpper(licence), "AGPL") {
		return refusef("-tags-query-licence %s: AGPL is refused", licence)
	}
	if top := gitOut(ctx, ".", "rev-parse", "--show-toplevel"); top != "" {
		if rel, inside := pathInside(top, o.query); inside && rel != "workspace" && !strings.HasPrefix(rel, "workspace"+string(filepath.Separator)) {
			return refusef("%s is inside the work tree outside workspace/, so it is vendored: a query is fetched at measurement time and never committed (condition c)", o.query)
		}
	}
	dirs := []string{filepath.Join("internal", "extractor", "dialects", dialect)}
	if manifestPath != "" && filepath.Clean(filepath.Dir(manifestPath)) != dirs[0] {
		dirs = append(dirs, filepath.Dir(manifestPath))
	}
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.scm"))
		sort.Strings(files)
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(string(b), "\n") {
				hrepo, hpath, ok := sourceHeader(line)
				if ok && normRepo(hrepo) == normRepo(repo) && cleanQueryPath(hpath) == cleanQueryPath(qpath) {
					return refusef("%s says the dialect was drafted from %s %s: the source a dialect was drafted from is no oracle for it (condition a)", f, hrepo, hpath)
				}
			}
		}
	}
	return nil
}

// splitQuerySource splits "<owner/repo> <path>".
func splitQuerySource(s string) (repo, path string, ok bool) {
	f := strings.Fields(s)
	if len(f) != 2 || !strings.Contains(strings.Trim(f[0], "/"), "/") {
		return "", "", false
	}
	return f[0], f[1], true
}

// sourceHeader reads a borrowed query's `; source: <repo> <path> …` line
// (design §8.2).
func sourceHeader(line string) (repo, path string, ok bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, ";") {
		return "", "", false
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(strings.TrimLeft(t, ";")), "source:")
	if !ok {
		return "", "", false
	}
	f := strings.Fields(rest)
	if len(f) < 2 {
		return "", "", false
	}
	return f[0], f[1], true
}

func normRepo(r string) string {
	r = strings.ToLower(strings.TrimSpace(r))
	for _, p := range []string{"https://", "http://", "github.com/"} {
		r = strings.TrimPrefix(r, p)
	}
	return strings.Trim(strings.TrimSuffix(r, ".git"), "/")
}

func cleanQueryPath(p string) string { return strings.TrimPrefix(path.Clean(p), "/") }

// pathInside reports whether p resolves inside dir, and its path there.
func pathInside(dir, p string) (string, bool) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", false
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		abs = r
	}
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	rel, err := filepath.Rel(dir, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// detectTagsQueryFormat reads a query's capture convention from its
// capture names (comments and strings skipped): @item is Zed's outline;
// @local.definition nvim-treesitter's locals; @definition.<kind> with @name
// a tags.scm, and without @name a legacy locals.scm.
func detectTagsQueryFormat(query string) (string, error) {
	names := queryCaptureNames(query)
	has := func(pred func(string) bool) bool { return slices.ContainsFunc(names, pred) }
	switch {
	case slices.Contains(names, "item"):
		return "outline", nil
	case has(func(n string) bool { return n == "local.definition" || strings.HasPrefix(n, "local.definition.") }):
		return "locals", nil
	case has(func(n string) bool { return strings.HasPrefix(n, "definition.") }):
		if slices.Contains(names, "name") {
			return "tags", nil
		}
		return "locals", nil
	}
	return "", refusef("no definition capture (@definition.<kind> with @name, @local.definition[.<kind>], or @item with @name): set -tags-query-format tags|locals|outline")
}

// queryCaptureNames lists the capture names a query's source uses, sorted.
func queryCaptureNames(src string) []string {
	seen := map[string]bool{}
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case ';':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case '"':
			for i++; i < len(src) && src[i] != '"'; i++ {
				if src[i] == '\\' {
					i++
				}
			}
		case '@':
			j := i + 1
			for j < len(src) && (src[j] == '_' || src[j] == '.' || src[j] == '-' ||
				'a' <= src[j] && src[j] <= 'z' || 'A' <= src[j] && src[j] <= 'Z' || '0' <= src[j] && src[j] <= '9') {
				j++
			}
			seen[src[i+1:j]] = true
			i = j - 1
		}
	}
	return sortedKeys(seen)
}

// runTagsQuery runs query on every subject file the manifest owns (inside
// -only), on the grammar variant fathom's tags engine parses that file
// with (extractor.ProbeTargetFor), and returns its definitions, sorted and
// deduplicated. A query that does not compile — an unsupported predicate
// among them, named by the engine — is refused; predicates are never
// stripped. A file the engine cannot parse within its budget is left out
// and counted on warn: fathom's own extraction of it, on the same grammar
// under the same budget, faults and leaves scope too.
func runTagsQuery(ctx context.Context, m langbench.Manifest, root string, only []string, queryFile, query, format string, warn io.Writer) ([]tqDef, error) {
	refs, err := connector.NewLocalConnector().Discover(ctx, core.ServiceRef{ID: "langbench", Path: root})
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", root, err)
	}
	type variant struct {
		p *tsengine.Parser
		q *tsengine.Query
	}
	variants := map[string]*variant{}
	drop := func(key string) {
		if v := variants[key]; v != nil {
			v.q.Close()
			_ = v.p.Close()
			delete(variants, key)
		}
	}
	defer func() {
		for _, k := range sortedKeys(variants) {
			drop(k)
		}
	}()
	seen := map[tqDef]bool{}
	var defs []tqDef
	var failed []string
	for _, ref := range refs {
		if !m.Owns(ref.Path) || !onlyMatch(only, ref.Path) {
			continue
		}
		target, err := extractor.ProbeTargetFor(m.ID, ref.Path)
		if err != nil {
			return nil, err
		}
		key := target.Blob + "\x00" + target.EntryPoint
		v := variants[key]
		if v == nil {
			p, err := tsengine.NewParser(ctx, target.Blob, target.EntryPoint)
			if err != nil {
				return nil, err
			}
			q, err := p.CompileQuery(query)
			if err != nil {
				_ = p.Close()
				return nil, refusef("%s does not compile on grammar %s (%s): %v", queryFile, target.Blob, target.EntryPoint, err)
			}
			if format == "outline" {
				q.CaptureNodeTypes()
			}
			v = &variant{p: p, q: q}
			variants[key] = v
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path)))
		if err != nil {
			return nil, err
		}
		tree, err := v.p.Parse(src)
		if err != nil {
			failed = append(failed, ref.Path)
			drop(key) // a parser abandoned on its budget is closed; the next file gets a fresh one
			continue
		}
		matches, err := v.q.Run(tree)
		tree.Close()
		if err != nil {
			failed = append(failed, ref.Path)
			drop(key)
			continue
		}
		for _, d := range tagsQueryDefs(ref.Path, src, matches, format) {
			if !seen[d] {
				seen[d] = true
				defs = append(defs, d)
			}
		}
	}
	if len(failed) > 0 {
		fmt.Fprintf(warn, "lang-oracle: -tags-query: %d files the engine could not parse within its budget were left out of the oracle (first: %s)\n", len(failed), failed[0])
	}
	sort.Slice(defs, func(i, j int) bool {
		a, b := defs[i], defs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.End < b.End
	})
	return defs, nil
}

// tagsQueryDefs reads one file's definitions from a query's matches:
//   - tags: @definition.<kind> on the definition node plus @name; the line
//     is the name's, the end the definition node's;
//   - locals: @local.definition[.<kind>], or legacy @definition[.<kind>],
//     on the name node; no end line (0), so range agreement is not measured;
//   - outline: @item plus @name; the kind is the @item node's type, the end
//     the item's.
//
// A bare capture's kind is "definition". The kinds map through the
// manifest's oracle_kinds.tool when scored.
func tagsQueryDefs(relPath string, src []byte, matches []tsengine.Match, format string) []tqDef {
	text := func(c tsengine.Capture) string { return strings.TrimSpace(c.Node.Text(src)) }
	first := func(m tsengine.Match, name string) *tsengine.Capture {
		for i := range m.Captures {
			if m.Captures[i].Name == name {
				return &m.Captures[i]
			}
		}
		return nil
	}
	var out []tqDef
	add := func(name tsengine.Capture, end int, kind string) {
		if n := text(name); n != "" {
			out = append(out, tqDef{Path: relPath, Name: n, Line: name.Node.Line(), End: end, Kind: kind})
		}
	}
	for _, m := range matches {
		switch format {
		case "tags":
			name := first(m, "name")
			if name == nil {
				continue
			}
			for _, c := range m.Captures {
				if kind, ok := strings.CutPrefix(c.Name, "definition."); ok {
					add(*name, endLine(src, c.Node), kind)
				}
			}
		case "locals":
			for _, c := range m.Captures {
				kind, ok := strings.CutPrefix(c.Name, "local.")
				if !ok {
					kind = c.Name // legacy: @definition[.<kind>]
				}
				if kind == "definition" {
					add(c, 0, "definition")
				} else if k, ok := strings.CutPrefix(kind, "definition."); ok {
					add(c, 0, k)
				}
			}
		case "outline":
			item, name := first(m, "item"), first(m, "name")
			if item != nil && name != nil {
				add(*name, endLine(src, item.Node), item.Type)
			}
		}
	}
	return out
}

// endLine is the 1-based line of a node's last byte.
func endLine(src []byte, n tsengine.Node) int {
	end := min(int(n.EndByte), len(src))
	start := min(int(n.StartByte), end)
	if end > start {
		end-- // the last byte inside the node: a trailing newline is not a line of it
	}
	return n.Line() + strings.Count(string(src[start:end]), "\n")
}
