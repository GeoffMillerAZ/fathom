// Command maven-oracle is the language-breadth harness's oracle for the maven
// doc-engine dialect (design §1.3, owner decision D4). It writes indexbench
// lang-oracle's tool-oracle JSON (-tool-oracle, design §10) in one of two modes:
//
//	go run . -subject <checkout> -effective <file>          # tier A: mvn help:effective-pom
//	go run . -subject <checkout> -effective <file> -refs    # tier C: references and cross-file references
//	go run . -subject <checkout> -effective <file> -refs -verbose <file> -ablate <dir>
//	                                                        # tier A: the same, attested by Maven (verbose.go)
//
// Tier A. `mvn help:effective-pom` is handed the reactor's root POM and
// evaluates every project Maven itself builds (module discovery, parent and
// property resolution, profile activation) into one `<projects>` document
// (-profiles activates the profiles whose <modules> a subject keeps out of its
// default reactor). That
// document carries each project's coordinates but no file and no line, so an
// encoding/xml walk supplies them: the reactor's files are the root POM plus the
// module directories (or POM files) a walk of `<modules>` reaches, a project's
// file is the one whose declared artifactId is the tool's, its line the
// `<project>` start tag's. The two sets must be one set: a walked file the tool
// has no project for, or a tool project no walked file declares, is counted on
// stderr (a walked file with no project is a module of a profile the run left
// inactive; a tool project with no walked file would be a walk defect and is
// zero on the subjects). A POM outside the reactor (a
// test project under src/test, an archetype resource) is not a project the tool
// builds and is not in the truth, so a reading measures a reactor's files with
// -only (the list is written to -files-out). The run is offline once the local
// repository holds the help plugin and the parents (`-offline`).
//
// Tier C (-refs). The effective POM merges inheritance, so what a file itself
// declares comes from an encoding/xml struct decode of the raw POM (an
// independent implementation of the walk the dialect makes over the doc engine's
// token tree): the parent's artifactId and each direct <dependencies> entry's,
// and every declared name is cross-checked against the effective POM
// (`declared-not-effective` is zero when the file's own words are what Maven
// evaluated). Cross-file references are the edges to another project of the
// reactor: a <module> to the POM Maven's module rule names, a <parent> to the
// reactor project whose coordinates equal the effective parent's, and a declared
// dependency to the reactor project whose groupId:artifactId equals the
// effective dependency's — coordinates, not names, so a dependency that shares
// an artifactId with a reactor module but not its group is not an edge (the
// dialect resolves by artifactId alone, which the xref metrics then measure).
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
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
	CrossReferences []xref `json:"cross_references,omitempty"`
}

// ── the effective POM ─────────────────────────────────────────────────────

type coord struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
}

func (c coord) ga() string {
	return strings.TrimSpace(c.GroupID) + ":" + strings.TrimSpace(c.ArtifactID)
}

type effDep struct {
	coord
	Type       string `xml:"type"`
	Classifier string `xml:"classifier"`
	Scope      string `xml:"scope"`
}

type plugin struct {
	ArtifactID string `xml:"artifactId"`
}

type effProject struct {
	coord
	Packaging    string   `xml:"packaging"`
	Parent       coord    `xml:"parent"`
	Modules      []string `xml:"modules>module"`
	Dependencies []effDep `xml:"dependencies>dependency"`
	Plugins      []plugin `xml:"build>plugins>plugin"`
}

type effProjects struct {
	Projects []effProject `xml:"project"`
}

func parseEffective(b []byte) ([]effProject, error) {
	var ps effProjects
	if err := xml.Unmarshal(b, &ps); err != nil {
		return nil, err
	}
	if len(ps.Projects) == 0 {
		// A one-project build writes the <project> alone, without <projects>.
		var one effProject
		if err := xml.Unmarshal(b, &one); err == nil && one.ArtifactID != "" {
			ps.Projects = []effProject{one}
		}
	}
	for i := range ps.Projects {
		p := &ps.Projects[i]
		p.GroupID, p.ArtifactID = strings.TrimSpace(p.GroupID), strings.TrimSpace(p.ArtifactID)
		p.Parent.GroupID, p.Parent.ArtifactID = strings.TrimSpace(p.Parent.GroupID), strings.TrimSpace(p.Parent.ArtifactID)
		for j := range p.Dependencies {
			d := &p.Dependencies[j]
			d.GroupID, d.ArtifactID = strings.TrimSpace(d.GroupID), strings.TrimSpace(d.ArtifactID)
		}
	}
	return ps.Projects, nil
}

// ── the raw POMs ──────────────────────────────────────────────────────────

type rawDep struct {
	ArtifactID string `xml:"artifactId"`
}

type rawProfile struct {
	Modules []string `xml:"modules>module"`
}

type rawPOM struct {
	XMLName      xml.Name     `xml:"project"`
	ArtifactID   string       `xml:"artifactId"`
	Parent       coord        `xml:"parent"`
	Modules      []string     `xml:"modules>module"`
	Profiles     []rawProfile `xml:"profiles>profile"`
	Dependencies []rawDep     `xml:"dependencies>dependency"`
	Plugins      []plugin     `xml:"build>plugins>plugin"`
}

// allModules is every module the file writes: its own <modules> and each
// profile's, in file order per section, each once. Which profiles are active
// is the tool's knowledge, not the walk's; join() keeps only the projects the
// tool built.
func (p rawPOM) allModules() []string {
	var out []string
	seen := map[string]bool{}
	add := func(ms []string) {
		for _, m := range ms {
			m = strings.TrimSpace(m)
			if m != "" && !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	add(p.Modules)
	for _, pr := range p.Profiles {
		add(pr.Modules)
	}
	return out
}

func readRaw(file string) (rawPOM, []byte, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return rawPOM{}, nil, err
	}
	var p rawPOM
	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.Strict = false
	if err := dec.Decode(&p); err != nil {
		return rawPOM{}, src, err
	}
	p.ArtifactID = strings.TrimSpace(p.ArtifactID)
	p.Parent.ArtifactID = strings.TrimSpace(p.Parent.ArtifactID)
	return p, src, nil
}

// projectLine is the 1-based line of the root element's start tag.
func projectLine(src []byte) int {
	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.Strict = false
	for {
		off := int(dec.InputOffset())
		tok, err := dec.Token()
		if err != nil {
			return 0
		}
		if _, ok := tok.(xml.StartElement); ok {
			for off < len(src) && src[off] != '<' {
				off++
			}
			return 1 + bytes.Count(src[:off], []byte{'\n'})
		}
	}
}

// moduleFile is Maven's own module rule: a module names a directory (its
// pom.xml) or a POM file, relative to the referring POM's directory.
func moduleFile(root, referrer, module string) (string, bool) {
	module = strings.TrimSpace(module)
	if module == "" {
		return "", false
	}
	p := filepath.Join(filepath.Dir(filepath.Join(root, filepath.FromSlash(referrer))), filepath.FromSlash(module))
	st, err := os.Stat(p)
	if err != nil {
		return "", false
	}
	if st.IsDir() {
		p = filepath.Join(p, "pom.xml")
		if _, err := os.Stat(p); err != nil {
			return "", false
		}
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

type reactorFile struct {
	Path string
	Raw  rawPOM
	Line int
	// Children is each raw module string's resolved file (in module order);
	// a module Maven cannot resolve is absent.
	Children []string
}

// walkReactor reads the root POM and every POM a <modules> walk reaches (a
// profile's modules included), once each, in discovery order.
func walkReactor(root string) ([]reactorFile, error) {
	var files []reactorFile
	seen := map[string]bool{}
	var visit func(rel string) error
	visit = func(rel string) error {
		if seen[rel] {
			return nil
		}
		seen[rel] = true
		raw, src, err := readRaw(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		rf := reactorFile{Path: rel, Raw: raw, Line: projectLine(src)}
		for _, m := range raw.allModules() {
			if child, ok := moduleFile(root, rel, m); ok {
				rf.Children = append(rf.Children, child)
			}
		}
		files = append(files, rf)
		for _, c := range rf.Children {
			if err := visit(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit("pom.xml"); err != nil {
		return nil, err
	}
	return files, nil
}

// ── running the tool ──────────────────────────────────────────────────────

var helpPluginRe = regexp.MustCompile(`^[0-9]`)

// toolVersion names Maven and the help plugin the run used.
func toolVersion(ctx context.Context, mvn, m2 string) string {
	b, err := exec.CommandContext(ctx, mvn, "--version").Output()
	v := "unknown"
	if err == nil {
		v = strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0])
		v = strings.TrimPrefix(v, "Apache ")
		if i := strings.Index(v, " ("); i > 0 {
			v = v[:i] // "Maven 3.9.11 (<commit>)" names the version, not the build
		}
	}
	if ents, err := os.ReadDir(filepath.Join(m2, "org", "apache", "maven", "plugins", "maven-help-plugin")); err == nil {
		var vs []string
		for _, e := range ents {
			if e.IsDir() && helpPluginRe.MatchString(e.Name()) {
				vs = append(vs, e.Name())
			}
		}
		sort.Strings(vs)
		if len(vs) > 0 {
			v += "; maven-help-plugin " + vs[len(vs)-1]
		}
	}
	return v
}

func runEffective(ctx context.Context, mvn, subject, m2, dest, profiles, goal string, offline, verbose bool) error {
	args := []string{"-B", "-q"}
	if offline {
		args = append(args, "-o")
	}
	if profiles != "" {
		args = append(args, "-P"+profiles)
	}
	if m2 != "" {
		args = append(args, "-Dmaven.repo.local="+m2)
	}
	if goal == "" {
		goal = "help:effective-pom"
	}
	args = append(args, goal, "-Doutput="+dest)
	if verbose {
		args = append(args, "-Dverbose")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nice", append([]string{"-n", "10", mvn}, args...)...)
	cmd.Dir = subject
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mvn help:effective-pom: %v: %s", err, strings.TrimSpace(tail(stderr.String(), 2000)))
	}
	return nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// ── the join ──────────────────────────────────────────────────────────────

type project struct {
	eff  effProject
	file reactorFile
}

// join pairs each walked file with the tool's project of the same artifactId.
// Reactor artifactIds are unique (Maven rejects a duplicate groupId:artifactId,
// and the subjects keep artifactIds distinct); a collision, an unmatched file
// and an unmatched project are counted.
func join(files []reactorFile, effs []effProject) (ps []project, walkOnly, toolOnly, ambiguous int) {
	byArt := map[string][]int{}
	for i, e := range effs {
		byArt[e.ArtifactID] = append(byArt[e.ArtifactID], i)
	}
	used := map[int]bool{}
	for _, f := range files {
		idx := byArt[f.Raw.ArtifactID]
		switch {
		case len(idx) == 0:
			walkOnly++
		case len(idx) > 1:
			ambiguous++
		default:
			used[idx[0]] = true
			ps = append(ps, project{eff: effs[idx[0]], file: f})
		}
	}
	for i := range effs {
		if !used[i] {
			toolOnly++
		}
	}
	return
}

func definitions(ps []project) []def {
	var out []def
	for _, p := range ps {
		out = append(out, def{Path: p.file.Path, Name: p.eff.ArtifactID, Line: p.file.Line, Kind: "package"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

type refsResult struct {
	refs  []ref
	xrefs []xref
	// counts, reported on stderr
	declaredNotEffective, parentMismatch int
	moduleEdges, parentEdges, depEdges   int
	externalDeps, sameNameOtherGroup     int
	// plugins the files declare under build/plugins (the dialect's MavenPlugin
	// entities, which no oracle scores: the L1 file carries one tier), how many
	// the effective POM also lists, and how many repeat an artifactId in their file.
	pluginsDeclared, pluginsNotEffective, pluginsRepeated int
}

func references(ps []project) refsResult {
	var r refsResult
	byPath := map[string]project{}
	byGA := map[string]project{}
	for _, p := range ps {
		byPath[p.file.Path] = p
		byGA[p.eff.ga()] = p
	}
	byArt := map[string]int{}
	for _, p := range ps {
		byArt[p.eff.ArtifactID]++
	}
	seenRef := map[ref]bool{}
	seenX := map[xref]bool{}
	addRef := func(x ref) {
		if !seenRef[x] {
			seenRef[x] = true
			r.refs = append(r.refs, x)
		}
	}
	addX := func(x xref) bool {
		if x.Path == x.ToPath || seenX[x] {
			return false
		}
		seenX[x] = true
		r.xrefs = append(r.xrefs, x)
		return true
	}
	for _, p := range ps {
		from := p.eff.ArtifactID
		// parent
		if pa := p.file.Raw.Parent.ArtifactID; pa != "" {
			addRef(ref{Path: p.file.Path, From: from, To: pa})
			if p.eff.Parent.ArtifactID != pa {
				r.parentMismatch++
			}
			if t, ok := byGA[p.eff.Parent.ga()]; ok && p.eff.Parent.Version == t.eff.Version {
				if addX(xref{Path: p.file.Path, From: from, ToPath: t.file.Path, To: t.eff.ArtifactID}) {
					r.parentEdges++
				}
			}
		}
		// declared build plugins: each must be one the effective POM lists
		seenPlugin := map[string]bool{}
		for _, pl := range p.file.Raw.Plugins {
			pa := strings.TrimSpace(pl.ArtifactID)
			if pa == "" {
				continue
			}
			r.pluginsDeclared++
			if seenPlugin[pa] {
				r.pluginsRepeated++
			}
			seenPlugin[pa] = true
			ok := false
			for _, e := range p.eff.Plugins {
				if strings.TrimSpace(e.ArtifactID) == pa {
					ok = true
				}
			}
			if !ok {
				r.pluginsNotEffective++
			}
		}
		// declared dependencies
		for _, d := range p.file.Raw.Dependencies {
			da := strings.TrimSpace(d.ArtifactID)
			if da == "" {
				continue
			}
			addRef(ref{Path: p.file.Path, From: from, To: da})
			found := false
			var cands []effDep
			for _, e := range p.eff.Dependencies {
				if e.ArtifactID == da {
					found = true
					cands = append(cands, e)
				}
			}
			if !found {
				r.declaredNotEffective++
				continue
			}
			linked := false
			for _, c := range cands {
				if t, ok := byGA[c.ga()]; ok {
					if addX(xref{Path: p.file.Path, From: from, ToPath: t.file.Path, To: t.eff.ArtifactID}) {
						r.depEdges++
					}
					linked = true
				}
			}
			if !linked {
				if byArt[da] > 0 {
					r.sameNameOtherGroup++
				} else {
					r.externalDeps++
				}
			}
		}
		// modules
		for _, c := range p.file.Children {
			if t, ok := byPath[c]; ok {
				if addX(xref{Path: p.file.Path, From: from, ToPath: t.file.Path, To: t.eff.ArtifactID}) {
					r.moduleEdges++
				}
			}
		}
	}
	sort.Slice(r.refs, func(i, j int) bool {
		a, b := r.refs[i], r.refs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.To < b.To
	})
	sort.Slice(r.xrefs, func(i, j int) bool {
		a, b := r.xrefs[i], r.xrefs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.ToPath < b.ToPath
	})
	return r
}

func main() {
	var (
		subject   = flag.String("subject", "", "the subject checkout (its root pom.xml is the reactor root)")
		mvn       = flag.String("mvn", "mvn", "the mvn binary")
		m2        = flag.String("m2", "", "the local repository (-Dmaven.repo.local); keep it outside the subject")
		effective = flag.String("effective", "", "the effective POM file: read when it exists, else written by `mvn help:effective-pom`")
		offline   = flag.Bool("offline", false, "run mvn with -o (the local repository must hold the help plugin and the parents)")
		profiles  = flag.String("profiles", "", "profiles to activate (mvn -P), for a subject whose modules sit in an inactive profile; recorded in the tool version")
		refs      = flag.Bool("refs", false, "tier C: references and cross-file references instead of definitions")
		filesOut  = flag.String("files-out", "", "write the reactor's POM paths (comma-separated, for lang-oracle -only) here")
		goal      = flag.String("goal", "help:effective-pom", "the effective-pom goal (a full plugin coordinate pins the help plugin; -verbose needs 3.2.0 or later)")
		verboseF  = flag.String("verbose", "", "with -refs: the verbose effective POM (-Dverbose, input location tracking), read when it exists, else written; the references are then tier A (verbose.go)")
		ablate    = flag.String("ablate", "", "with -verbose: a scratch directory outside the subject for the <module> ablations")
	)
	flag.Parse()
	if *subject == "" || *effective == "" {
		fmt.Fprintln(os.Stderr, "usage: maven-oracle -subject <checkout> -effective <file> [-mvn mvn] [-m2 dir] [-offline] [-refs] [-files-out file]")
		os.Exit(2)
	}
	root, err := filepath.Abs(*subject)
	if err != nil {
		fatal(err)
	}
	ctx := context.Background()
	if _, err := os.Stat(*effective); errors.Is(err, os.ErrNotExist) {
		if err := runEffective(ctx, *mvn, root, *m2, *effective, *profiles, *goal, *offline, false); err != nil {
			fatal(err)
		}
	}
	eb, err := os.ReadFile(*effective)
	if err != nil {
		fatal(err)
	}
	effs, err := parseEffective(eb)
	if err != nil {
		fatal(fmt.Errorf("%s: %w", *effective, err))
	}
	files, err := walkReactor(root)
	if err != nil {
		fatal(err)
	}
	ps, walkOnly, toolOnly, ambiguous := join(files, effs)
	fmt.Fprintf(os.Stderr, "maven-oracle: %d tool projects, %d walked POMs, %d joined; walk-only %d, tool-only %d, ambiguous artifactId %d\n",
		len(effs), len(files), len(ps), walkOnly, toolOnly, ambiguous)
	if *filesOut != "" {
		var paths []string
		for _, p := range ps {
			paths = append(paths, p.file.Path)
		}
		sort.Strings(paths)
		if err := os.WriteFile(*filesOut, []byte(strings.Join(paths, ",")), 0o644); err != nil {
			fatal(err)
		}
	}
	o := out{Version: toolVersion(ctx, *mvn, *m2)}
	if *profiles != "" {
		o.Version += "; -P " + *profiles
	}
	if *refs && *verboseF != "" {
		if _, err := os.Stat(*verboseF); errors.Is(err, os.ErrNotExist) {
			if err := runEffective(ctx, *mvn, root, *m2, *verboseF, *profiles, *goal, *offline, true); err != nil {
				fatal(err)
			}
		}
		vb, err := os.ReadFile(*verboseF)
		if err != nil {
			fatal(err)
		}
		vps, err := parseVerbose(vb)
		if err != nil {
			fatal(fmt.Errorf("%s: %w", *verboseF, err))
		}
		var ma *moduleAblator
		if *ablate != "" {
			abs, err := filepath.Abs(*ablate)
			if err != nil {
				fatal(err)
			}
			if rel, err := filepath.Rel(root, abs); err == nil && !strings.HasPrefix(rel, "..") {
				fatal(fmt.Errorf("-ablate %s is inside the subject", abs))
			}
			ma = &moduleAblator{mvn: *mvn, m2: *m2, profiles: *profiles, goal: *goal, dir: filepath.Join(abs, "work"), cache: filepath.Join(abs, "answers"), files: map[string][]byte{}}
			for _, f := range files {
				b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
				if err != nil {
					fatal(err)
				}
				ma.files[f.Path] = b
			}
		}
		rs, xs, c, err := verboseReferences(ctx, root, ps, vps, ma, 3)
		if err != nil {
			fatal(err)
		}
		fmt.Fprintf(os.Stderr, "maven-oracle (verbose): %d references (parent attested %d, not %d; dependencies attested %d (%d at another line of the same POM, an active profile re-declaring them), not %d; own-model dependencies that are no top-level entry %d; projects the verbose POM lacks %d)\n",
			len(rs), c.parentAttested, c.parentNot, c.depAttested, c.depOtherLine, c.depNot, c.ownDepsNotTopLevel, c.projectsMissing)
		fmt.Fprintf(os.Stderr, "maven-oracle (verbose): cross-references: %d module (ablation, every occurrence of the module in its POM cut together: attested %d, not %d; a module naming no reactor project %d; scratch baseline differs %v), %d parent, %d dependency; dependencies outside the reactor %d, sharing a reactor artifactId with another group %d\n",
			c.moduleEdges, c.moduleAttested, c.moduleNot, c.moduleNoTo, c.baselineDiffers, c.parentEdges, c.depEdges, c.externalDeps, c.sameNameOtherGroup)
		o.Tool = "mvn help:effective-pom -Dverbose (input locations) + <module> ablation"
		o.Tier = "A"
		if ma == nil {
			o.Tool = "mvn help:effective-pom -Dverbose (input locations)"
		}
		o.Definitions = []def{}
		o.References = rs
		o.CrossReferences = xs
	} else if *refs {
		r := references(ps)
		fmt.Fprintf(os.Stderr, "maven-oracle: %d references; cross-references: %d module, %d parent, %d dependency; declared-not-effective %d, parent-mismatch %d; dependencies outside the reactor %d, sharing a reactor artifactId with another group %d\n",
			len(r.refs), r.moduleEdges, r.parentEdges, r.depEdges, r.declaredNotEffective, r.parentMismatch, r.externalDeps, r.sameNameOtherGroup)
		fmt.Fprintf(os.Stderr, "maven-oracle: build/plugins entries declared %d, not in the effective POM %d, repeating an artifactId within their file %d\n",
			r.pluginsDeclared, r.pluginsNotEffective, r.pluginsRepeated)
		o.Tool = "maven-pom-walk (independent encoding/xml decode of each POM's own parent and dependencies, checked against mvn help:effective-pom)"
		o.Version = runtime.Version() + " encoding/xml; " + o.Version
		o.Tier = "C"
		o.Definitions = []def{}
		o.References = r.refs
		o.CrossReferences = r.xrefs
	} else {
		o.Tool = "mvn help:effective-pom"
		o.Tier = "A"
		o.Definitions = definitions(ps)
		o.References = []ref{}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "maven-oracle:", err)
	os.Exit(1)
}
