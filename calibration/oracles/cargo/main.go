// Command cargo-oracle is the language-breadth harness's oracle for the cargo
// doc-engine dialect (design §1.3, owner decision D4). It writes indexbench
// lang-oracle's tool-oracle JSON (-tool-oracle, design §10), tier A:
//
//	go run . -subject <checkout> -cargo <cargo> [-root <manifest>]... [-home <CARGO_HOME>] [-offline]
//
// The tool is cargo itself, run at each root manifest (default: the subject's
// Cargo.toml; a subject with workspaces the root leaves out, such as tokio's fuzz
// crates, names them with -root):
//
//   - `cargo metadata --no-deps --format-version 1` for what each workspace
//     contains and declares: which manifests are members (the `members` globs and
//     `exclude`, plus every path dependency inside the root), each package's name,
//     and each package's dependencies with `workspace = true` inheritance, renames
//     (`package = "real"`) and target tables resolved;
//   - `cargo metadata --all-features --format-version 1` for the resolved graph:
//     which dependency edges cargo really draws between two packages that live in
//     the subject, every [patch] applied (tokio patches each member it lists as a
//     versioned dependency, so a `--no-deps` reading of `path` would call those
//     registry edges). This run fetches the workspace's registry dependencies into
//     -home, so it needs the network once; -offline reuses a warm -home.
//
// The tool writes no line, so a package is placed at line 1: the dialect's package
// entity is the manifest itself (its root table begins the file) and the oracle
// names that anchor, not a `[package]` header a walk would have to find.
//
//   - definitions: one `package` per package the tool lists as a workspace
//     member, at its manifest.
//   - references: (package, dependency name) for every dependency the tool lists
//     for it, whatever its kind or target table; the name is the real package
//     name, not a rename.
//   - cross_references, entity granularity: a resolved dependency edge from one
//     subject package to another (the resolve graph, all kinds and targets), and
//     the workspace root's `workspace` entity to each member the tool lists, the
//     root manifest's own package included (cargo makes it a member whether or
//     not `members` names ".", so the dialect misses it unless it is listed). A
//     registry dependency that merely shares a name with a package of the subject
//     and resolves to the registry is not an edge (counted on stderr as a namesake).
//
// The dialect's features and explicit targets ([[bin]], [lib], [[example]],
// [[test]], [[bench]]) are not scored: the tool lists implicit ones too (a
// feature for every optional dependency, a target for src/main.rs), so what
// would be scored is the set a raw decode of the manifest declares. Instead each
// attested manifest's raw declarations are cross-checked against the tool's
// (declared-not-in-tool counts, all zero when the file's own words are what
// cargo evaluated) and printed on stderr with the implicit counts.
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	Kind string `json:"kind"`
}

type ref struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type xref struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to,omitempty"`
}

type out struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	CrossReferences []xref `json:"cross_references"`
}

// ── cargo metadata ────────────────────────────────────────────────────────

type metaDep struct {
	Name   string  `json:"name"`
	Path   *string `json:"path"`
	Source *string `json:"source"`
}

type metaTarget struct {
	Kind []string `json:"kind"`
	Name string   `json:"name"`
}

type metaPkg struct {
	Name         string              `json:"name"`
	ID           string              `json:"id"`
	Source       *string             `json:"source"`
	ManifestPath string              `json:"manifest_path"`
	Dependencies []metaDep           `json:"dependencies"`
	Targets      []metaTarget        `json:"targets"`
	Features     map[string][]string `json:"features"`
}

type resolveNode struct {
	ID   string `json:"id"`
	Deps []struct {
		Pkg string `json:"pkg"`
	} `json:"deps"`
}

type metadata struct {
	Packages         []metaPkg `json:"packages"`
	WorkspaceMembers []string  `json:"workspace_members"`
	WorkspaceRoot    string    `json:"workspace_root"`
	Resolve          *struct {
		Nodes []resolveNode `json:"nodes"`
	} `json:"resolve"`
}

func runMetadata(ctx context.Context, cargo, home, subject, manifest string, args ...string) (metadata, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	full := append([]string{"-n", "10", cargo, "metadata", "--format-version", "1"}, args...)
	full = append(full, "--manifest-path", filepath.Join(subject, filepath.FromSlash(manifest)))
	cmd := exec.CommandContext(ctx, "nice", full...)
	cmd.Dir = subject
	cmd.Env = append(os.Environ(), "CARGO_HOME="+home, "CARGO_TERM_COLOR=never")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return metadata{}, fmt.Errorf("cargo metadata %s: %v: %s", manifest, err, strings.TrimSpace(tail(stderr.String(), 1500)))
	}
	var m metadata
	if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
		return metadata{}, fmt.Errorf("cargo metadata %s: decode: %v", manifest, err)
	}
	return m, nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func toolVersion(ctx context.Context, cargo string) string {
	b, err := exec.CommandContext(ctx, cargo, "--version").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

// ── the attested packages ─────────────────────────────────────────────────

type pkgInfo struct {
	name     string
	manifest string // subject-relative, slash-separated
	raw      metaPkg
}

type wsRun struct {
	root    string // subject-relative manifest of the workspace root
	members []string
}

func rel(subject, abs string) (string, bool) {
	r, err := filepath.Rel(subject, abs)
	if err != nil || strings.HasPrefix(r, "..") {
		return "", false
	}
	return filepath.ToSlash(r), true
}

func evalDir(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// manifestOf is the subject-relative Cargo.toml of a package whose manifest_path
// the tool printed.
func manifestOf(subject, manifestPath string) (string, bool) {
	r, ok := rel(subject, evalDir(filepath.Dir(manifestPath)))
	if !ok {
		return "", false
	}
	if r == "." {
		return "Cargo.toml", true
	}
	return r + "/Cargo.toml", true
}

// ── the raw cross-check ───────────────────────────────────────────────────

type rawManifest struct {
	Features  map[string]any            `toml:"features"`
	Lib       map[string]any            `toml:"lib"`
	Bin       []map[string]any          `toml:"bin"`
	Example   []map[string]any          `toml:"example"`
	Test      []map[string]any          `toml:"test"`
	Bench     []map[string]any          `toml:"bench"`
	Deps      map[string]any            `toml:"dependencies"`
	DevDeps   map[string]any            `toml:"dev-dependencies"`
	BuildDeps map[string]any            `toml:"build-dependencies"`
	Target    map[string]map[string]any `toml:"target"`
}

func depNames(sec map[string]any) []string {
	var out []string
	for k, v := range sec {
		name := k
		if m, ok := v.(map[string]any); ok {
			if p, ok := m["package"].(string); ok && p != "" {
				name = p
			}
		}
		out = append(out, name)
	}
	return out
}

type crossCheck struct {
	manifests                                       int
	declaredFeatures, implicitFeatures, featMissing int
	declaredTargets, autoTargets, targetMissing     int
	declaredDeps, toolDeps, declaredNotInTool       int
	toolNotDeclared                                 int
	notes                                           []string
}

func (c *crossCheck) check(subject string, p pkgInfo) {
	src, err := os.ReadFile(filepath.Join(subject, filepath.FromSlash(p.manifest)))
	if err != nil {
		c.notes = append(c.notes, p.manifest+": "+err.Error())
		return
	}
	var r rawManifest
	if err := toml.Unmarshal(src, &r); err != nil {
		c.notes = append(c.notes, p.manifest+": raw decode: "+err.Error())
		return
	}
	c.manifests++
	for f := range r.Features {
		c.declaredFeatures++
		if _, ok := p.raw.Features[f]; !ok {
			c.featMissing++
			c.notes = append(c.notes, p.manifest+": declared feature "+f+" not in the tool's")
		}
	}
	if n := len(p.raw.Features) - len(r.Features); n > 0 {
		c.implicitFeatures += n
	}
	has := func(kind, name string) bool {
		for _, t := range p.raw.Targets {
			for _, k := range t.Kind {
				if k == kind && t.Name == name {
					return true
				}
			}
		}
		return false
	}
	declared := 0
	chk := func(kind string, name any) {
		s, _ := name.(string)
		if s == "" {
			return
		}
		declared++
		if !has(kind, s) {
			c.targetMissing++
			c.notes = append(c.notes, p.manifest+": declared "+kind+" "+s+" not in the tool's targets")
		}
	}
	if r.Lib != nil {
		chk("lib", r.Lib["name"])
	}
	for _, b := range r.Bin {
		chk("bin", b["name"])
	}
	for _, b := range r.Example {
		chk("example", b["name"])
	}
	for _, b := range r.Test {
		chk("test", b["name"])
	}
	for _, b := range r.Bench {
		chk("bench", b["name"])
	}
	c.declaredTargets += declared
	if n := len(p.raw.Targets) - declared; n > 0 {
		c.autoTargets += n
	}
	rawDeps := map[string]bool{}
	for _, sec := range []map[string]any{r.Deps, r.DevDeps, r.BuildDeps} {
		for _, n := range depNames(sec) {
			rawDeps[n] = true
		}
	}
	for _, t := range r.Target {
		for _, key := range []string{"dependencies", "dev-dependencies", "build-dependencies"} {
			if sec, ok := t[key].(map[string]any); ok {
				for _, n := range depNames(sec) {
					rawDeps[n] = true
				}
			}
		}
	}
	toolDeps := map[string]bool{}
	for _, d := range p.raw.Dependencies {
		toolDeps[d.Name] = true
	}
	c.declaredDeps += len(rawDeps)
	c.toolDeps += len(toolDeps)
	for n := range rawDeps {
		if !toolDeps[n] {
			c.declaredNotInTool++
			c.notes = append(c.notes, p.manifest+": declared dependency "+n+" not in the tool's")
		}
	}
	for n := range toolDeps {
		if !rawDeps[n] {
			c.toolNotDeclared++
			c.notes = append(c.notes, p.manifest+": the tool's dependency "+n+" is not spelled under that name in the file (workspace-inherited rename)")
		}
	}
}

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(s string) error { *m = append(*m, s); return nil }

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	cargo := flag.String("cargo", "cargo", "the cargo binary (rustc must be on PATH for the resolving run)")
	home := flag.String("home", "", "CARGO_HOME for the runs (default: a temp dir outside the subject)")
	offline := flag.Bool("offline", false, "run the resolving cargo metadata offline (a warm -home)")
	verbose := flag.Bool("v", false, "list the registry namesakes")
	filesOut := flag.String("files-out", "", "write the attested manifests, one per line")
	var roots multi
	flag.Var(&roots, "root", "a workspace root manifest, subject-relative (repeatable; default Cargo.toml)")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: cargo-oracle -subject <checkout> -cargo <cargo> [-root <manifest>]...")
		os.Exit(2)
	}
	if len(roots) == 0 {
		roots = multi{"Cargo.toml"}
	}
	if *home == "" {
		d, err := os.MkdirTemp("", "cargo-oracle-home-")
		if err != nil {
			fatal(err)
		}
		defer os.RemoveAll(d)
		*home = d
	}
	abs, err := filepath.Abs(*subject)
	if err != nil {
		fatal(err)
	}
	subj := evalDir(abs)
	ctx := context.Background()

	// Pass 1, `--no-deps`: the packages each workspace holds.
	pkgs := map[string]pkgInfo{} // by manifest
	var runs []wsRun
	for _, root := range roots {
		m, err := runMetadata(ctx, *cargo, *home, subj, root, "--no-deps", "--offline")
		if err != nil {
			fatal(err)
		}
		byID := map[string]pkgInfo{}
		for _, p := range m.Packages {
			mf, ok := manifestOf(subj, p.ManifestPath)
			if !ok {
				continue
			}
			info := pkgInfo{name: p.Name, manifest: mf, raw: p}
			byID[p.ID] = info
			pkgs[mf] = info
		}
		wr, ok := rel(subj, evalDir(m.WorkspaceRoot))
		if !ok {
			continue
		}
		rootMF := "Cargo.toml"
		if wr != "." {
			rootMF = wr + "/Cargo.toml"
		}
		run := wsRun{root: rootMF}
		for _, id := range m.WorkspaceMembers {
			if info, ok := byID[id]; ok {
				run.members = append(run.members, info.manifest)
			}
		}
		runs = append(runs, run)
	}

	// Pass 2, resolving: the edges cargo draws between packages of the subject.
	// A node's package may be a member or any other path package the graph
	// reaches; both ends must be a package pass 1 attested (a manifest of the
	// subject that some run listed as a member).
	type edge struct{ from, to string }
	resolved := map[edge]bool{}
	for _, root := range roots {
		args := []string{"--all-features"}
		if *offline {
			args = append(args, "--offline")
		}
		m, err := runMetadata(ctx, *cargo, *home, subj, root, args...)
		if err != nil {
			fatal(err)
		}
		if m.Resolve == nil {
			fatal(fmt.Errorf("cargo metadata %s: no resolve graph", root))
		}
		mfOfID := map[string]string{}
		for _, p := range m.Packages {
			if p.Source != nil {
				continue // a registry or git package
			}
			if mf, ok := manifestOf(subj, p.ManifestPath); ok {
				mfOfID[p.ID] = mf
			}
		}
		for _, n := range m.Resolve.Nodes {
			from, ok := mfOfID[n.ID]
			if !ok {
				continue
			}
			for _, d := range n.Deps {
				if to, ok := mfOfID[d.Pkg]; ok && to != from {
					resolved[edge{from, to}] = true
				}
			}
		}
	}

	var o out
	o.Tool = "cargo metadata (--no-deps, and --all-features for the resolved graph)"
	o.Version = toolVersion(ctx, *cargo)
	o.Tier = "A"
	var manifests []string
	for mf := range pkgs {
		manifests = append(manifests, mf)
	}
	sort.Strings(manifests)
	seenRef := map[ref]bool{}
	seenX := map[xref]bool{}
	addX := func(x xref) bool {
		if seenX[x] {
			return false
		}
		seenX[x] = true
		o.CrossReferences = append(o.CrossReferences, x)
		return true
	}
	pkgNames := map[string]bool{}
	for _, p := range pkgs {
		pkgNames[p.name] = true
	}
	var cc crossCheck
	depEdges, memberEdges, namesakes, unattested := 0, 0, 0, 0
	for e := range resolved {
		f, okf := pkgs[e.from]
		t, okt := pkgs[e.to]
		if !okf || !okt {
			unattested++
			continue
		}
		if addX(xref{Path: e.from, From: f.name, ToPath: e.to, To: t.name}) {
			depEdges++
		}
	}
	for _, mf := range manifests {
		p := pkgs[mf]
		o.Definitions = append(o.Definitions, def{Path: mf, Name: p.name, Line: 1, Kind: "package"})
		cc.check(subj, p)
		for _, d := range p.raw.Dependencies {
			r := ref{Path: mf, From: p.name, To: d.Name}
			if !seenRef[r] {
				seenRef[r] = true
				o.References = append(o.References, r)
			}
			if d.Path == nil && pkgNames[d.Name] && d.Name != p.name {
				// A registry or git dependency that shares a name with a package
				// of the subject: an edge only if the resolve graph drew one.
				var to string
				for _, q := range pkgs {
					if q.name == d.Name {
						to = q.manifest
					}
				}
				if !resolved[edge{mf, to}] {
					namesakes++
					if *verbose {
						fmt.Fprintf(os.Stderr, "  namesake resolved to the registry, not an edge: %s depends on %s\n", mf, d.Name)
					}
				}
			}
		}
	}
	for _, run := range runs {
		for _, mf := range run.members {
			t := pkgs[mf]
			if addX(xref{Path: run.root, From: "workspace", ToPath: t.manifest, To: t.name}) {
				memberEdges++
			}
		}
	}
	sort.Slice(o.References, func(i, j int) bool {
		a, b := o.References[i], o.References[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.ToPath < b.ToPath
	})
	if o.References == nil {
		o.References = []ref{}
	}
	if o.CrossReferences == nil {
		o.CrossReferences = []xref{}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(o); err != nil {
		fatal(err)
	}
	if *filesOut != "" {
		if err := os.WriteFile(*filesOut, []byte(strings.Join(manifests, "\n")+"\n"), 0o644); err != nil {
			fatal(err)
		}
	}
	fmt.Fprintf(os.Stderr, "cargo-oracle: %d workspace run(s), %d packages, %d references, %d cross-file references (%d workspace members, %d resolved dependency edges; %d edges left out for an end no run attested)\n",
		len(runs), len(o.Definitions), len(o.References), len(o.CrossReferences), memberEdges, depEdges, unattested)
	fmt.Fprintf(os.Stderr, "registry dependencies that share a name with a package of the subject and are not edges: %d\n", namesakes)
	fmt.Fprintf(os.Stderr, "cross-check over %d manifests: features declared %d (missing from the tool %d), implicit %d; explicit targets declared %d (missing from the tool %d), auto-discovered %d; dependencies declared %d, tool %d (declared-not-in-tool %d, tool-not-declared %d)\n",
		cc.manifests, cc.declaredFeatures, cc.featMissing, cc.implicitFeatures, cc.declaredTargets, cc.targetMissing, cc.autoTargets, cc.declaredDeps, cc.toolDeps, cc.declaredNotInTool, cc.toolNotDeclared)
	for _, n := range cc.notes {
		fmt.Fprintln(os.Stderr, "  note:", n)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "cargo-oracle:", err)
	os.Exit(1)
}
