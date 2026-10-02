package main

// verbose.go: tier-A references from Maven's own input location tracking
// (-verbose). `mvn help:effective-pom -Dverbose` writes, after every element of
// each project's effective model, the model that declared it and the line it
// is on (`<!-- org.apache.maven:maven-core:3.9.16, line 37 -->`). A reference a
// POM writes for itself (its <parent>'s artifactId, each <dependencies>
// entry's) is attested when the effective model holds that element located in
// the POM's own model at the line the file writes it on. The raw walk only
// selects which located elements are the file's own top-level ones (a
// profile's dependencies are located in the same file but are not the
// dialect's); the name and the line are Maven's.
//
// The <module> edges are attested by ablation: the module is cut from its
// aggregator in a scratch copy of the reactor's POMs, and the module's project
// must leave the reactor Maven builds (help:effective-pom's project list).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// located is where Maven says an element was declared: the declaring model's
// groupId:artifactId (its version, often an unresolved ${revision}, is dropped)
// and the line.
type located struct {
	model string
	line  int
}

var locationRe = regexp.MustCompile(`^\s*([^\s:,]+):([^\s:,]+):[^\s,]*, line (\d+)\s*$`)

func parseLocation(c string) (located, bool) {
	m := locationRe.FindStringSubmatch(c)
	if m == nil {
		return located{}, false
	}
	n, err := strconv.Atoi(m[3])
	if err != nil {
		return located{}, false
	}
	return located{m[1] + ":" + m[2], n}, true
}

type vdep struct {
	ga, art string
	loc     located
}

type vmod struct {
	name string
	loc  located
}

// vproj is one project of the verbose effective POM.
type vproj struct {
	group, art string
	parentArt  string
	parentLoc  located
	deps       []vdep
	modules    []vmod
}

func (p *vproj) ga() string { return p.group + ":" + p.art }

// parseVerbose reads a verbose effective POM into its projects, by
// groupId:artifactId.
func parseVerbose(b []byte) (map[string]*vproj, error) {
	dec := xml.NewDecoder(bytes.NewReader(b))
	dec.Strict = false
	out := map[string]*vproj{}
	var stack []string
	var cur *vproj
	base := -1 // stack index of the current <project>
	var text strings.Builder
	var dep *vdep
	// pending is the leaf whose location comment comes next.
	var pending func(located)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			pending = nil
			stack = append(stack, t.Name.Local)
			text.Reset()
			if t.Name.Local == "project" && (len(stack) == 1 || (len(stack) == 2 && stack[0] == "projects")) {
				cur, base = &vproj{}, len(stack)-1
				continue
			}
			if cur != nil && rel(stack, base) == "dependencies/dependency" {
				dep = &vdep{}
			}
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			pending = nil
			if cur == nil {
				stack = stack[:len(stack)-1]
				continue
			}
			v := strings.TrimSpace(text.String())
			p := cur
			switch rel(stack, base) {
			case "":
				if p.art != "" {
					out[p.ga()] = p
				}
				cur, base = nil, -1
			case "groupId":
				p.group = v
			case "artifactId":
				p.art = v
			case "parent/artifactId":
				p.parentArt = v
				pending = func(l located) { p.parentLoc = l }
			case "dependencies/dependency/groupId":
				if dep != nil {
					dep.ga = v + dep.ga
				}
			case "dependencies/dependency/artifactId":
				if dep != nil {
					dep.art = v
					d := dep
					pending = func(l located) { d.loc = l }
				}
			case "dependencies/dependency":
				if dep != nil {
					dep.ga = dep.ga + ":" + dep.art
					p.deps = append(p.deps, *dep)
					dep = nil
				}
			case "modules/module":
				p.modules = append(p.modules, vmod{name: v})
				i := len(p.modules) - 1
				pending = func(l located) { p.modules[i].loc = l }
			}
			stack = stack[:len(stack)-1]
			text.Reset()
		case xml.Comment:
			if pending != nil {
				if l, ok := parseLocation(string(t)); ok {
					pending(l)
				}
				pending = nil
			}
		}
	}
	return out, nil
}

// rel is the element path under the current project ("" for the project).
func rel(stack []string, base int) string {
	if base < 0 || len(stack) <= base {
		return ""
	}
	return strings.Join(stack[base+1:], "/")
}

// rawLoc is a name a POM writes and the line it writes it on.
type rawLoc struct {
	name string
	line int
}

type rawLocs struct {
	parent  rawLoc
	deps    []rawLoc
	modules []rawLoc // the file's own and each profile's
}

// rawLocations reads where a POM writes its parent's artifactId, each
// top-level dependency's and each module (encoding/xml token positions).
func rawLocations(src []byte) (rawLocs, error) {
	var r rawLocs
	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.Strict = false
	var stack []string
	var text strings.Builder
	startLine := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				return r, nil
			}
			return r, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			stack = append(stack, t.Name.Local)
			text.Reset()
			startLine, _ = dec.InputPos()
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			v := strings.TrimSpace(text.String())
			switch strings.Join(stack, "/") {
			case "project/parent/artifactId":
				r.parent = rawLoc{v, startLine}
			case "project/dependencies/dependency/artifactId":
				r.deps = append(r.deps, rawLoc{v, startLine})
			case "project/modules/module", "project/profiles/profile/modules/module":
				r.modules = append(r.modules, rawLoc{v, startLine})
			}
			stack = stack[:len(stack)-1]
			text.Reset()
		}
	}
}

// cutModule removes the <module>name</module> written on the given line.
func cutModule(src []byte, m rawLoc) ([]byte, bool) {
	lines := bytes.SplitAfter(src, []byte("\n"))
	if m.line < 1 || m.line > len(lines) {
		return nil, false
	}
	re := regexp.MustCompile(`<module>\s*` + regexp.QuoteMeta(m.name) + `\s*</module>`)
	l := lines[m.line-1]
	loc := re.FindIndex(l)
	if loc == nil {
		return nil, false
	}
	var out bytes.Buffer
	for i, x := range lines {
		if i == m.line-1 {
			out.Write(l[:loc[0]])
			out.Write(l[loc[1]:])
			continue
		}
		out.Write(x)
	}
	return out.Bytes(), true
}

// moduleAblator runs help:effective-pom on scratch copies of the reactor's
// POMs and answers which projects the reactor holds.
type moduleAblator struct {
	mvn, m2, profiles, goal, dir, cache string
	files                               map[string][]byte // reactor POM path -> bytes
}

func (a *moduleAblator) reactor(ctx context.Context, edit map[string][]byte) (map[string]bool, error) {
	h := sha256.New()
	var names []string
	for f := range a.files {
		names = append(names, f)
	}
	sort.Strings(names)
	for _, f := range names {
		b := a.files[f]
		if e, ok := edit[f]; ok {
			b = e
		}
		fmt.Fprintf(h, "%s\x00%d\x00", f, len(b))
		h.Write(b)
	}
	fmt.Fprintf(h, "P%s", a.profiles)
	key := hex.EncodeToString(h.Sum(nil))
	cacheFile := filepath.Join(a.cache, key+".json")
	if b, err := os.ReadFile(cacheFile); err == nil {
		var arts []string
		if json.Unmarshal(b, &arts) == nil {
			return toSet(arts), nil
		}
	}
	work := filepath.Join(a.dir, key[:16])
	if err := os.RemoveAll(work); err != nil {
		return nil, err
	}
	for _, f := range names {
		b := a.files[f]
		if e, ok := edit[f]; ok {
			b = e
		}
		dst := filepath.Join(work, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return nil, err
		}
	}
	dest := filepath.Join(work, "effective.xml")
	if err := runEffective(ctx, a.mvn, work, a.m2, dest, a.profiles, a.goal, true, false); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {
			return nil, err
		}
		// The tool refusing the reactor is an answer: no project.
		_ = os.RemoveAll(work)
		return map[string]bool{}, nil
	}
	eb, err := os.ReadFile(dest)
	if err != nil {
		return nil, err
	}
	effs, err := parseEffective(eb)
	if err != nil {
		return nil, err
	}
	var arts []string
	for _, e := range effs {
		arts = append(arts, e.ga())
	}
	sort.Strings(arts)
	if b, err := json.Marshal(arts); err == nil {
		_ = os.MkdirAll(a.cache, 0o755)
		_ = os.WriteFile(cacheFile, b, 0o644)
	}
	_ = os.RemoveAll(work)
	return toSet(arts), nil
}

func toSet(l []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range l {
		m[x] = true
	}
	return m
}

// verboseCounts are reported on stderr.
type verboseCounts struct {
	projectsMissing                       int // joined projects the verbose POM lacks
	parentAttested, parentNot             int
	depAttested, depNot, depOtherLine     int
	ownDepsNotTopLevel                    int // own-model dependencies that are no top-level entry (an active profile's)
	moduleAttested, moduleNot, moduleNoTo int
	baselineDiffers                       bool
	moduleEdges, parentEdges, depEdges    int
	externalDeps, sameNameOtherGroup      int
}

// verboseReferences are the tier-A references and cross-references.
func verboseReferences(ctx context.Context, root string, ps []project, vps map[string]*vproj, ma *moduleAblator, jobs int) ([]ref, []xref, verboseCounts, error) {
	var c verboseCounts
	byPath := map[string]project{}
	byGA := map[string]project{}
	byArt := map[string]int{}
	for _, p := range ps {
		byPath[p.file.Path] = p
		byGA[p.eff.ga()] = p
		byArt[p.eff.ArtifactID]++
	}
	var refs []ref
	var xrefs []xref
	seenRef := map[ref]bool{}
	seenX := map[xref]bool{}
	addRef := func(r ref) {
		if !seenRef[r] {
			seenRef[r] = true
			refs = append(refs, r)
		}
	}
	addX := func(x xref) bool {
		if x.Path == x.ToPath || seenX[x] {
			return false
		}
		seenX[x] = true
		xrefs = append(xrefs, x)
		return true
	}
	// A module written twice in one POM (two profiles) is one edge: every
	// occurrence naming the same project is cut together.
	type cut struct {
		from project
		to   project
		ms   []rawLoc
		src  []byte
	}
	var cuts []cut
	for _, p := range ps {
		vp := vps[p.eff.ga()]
		if vp == nil {
			c.projectsMissing++
			continue
		}
		own := p.eff.ga()
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p.file.Path)))
		if err != nil {
			return nil, nil, c, err
		}
		rl, err := rawLocations(src)
		if err != nil {
			return nil, nil, c, fmt.Errorf("%s: %v", p.file.Path, err)
		}
		from := p.eff.ArtifactID
		if rl.parent.name != "" {
			if vp.parentArt == rl.parent.name && vp.parentLoc == (located{own, rl.parent.line}) {
				c.parentAttested++
				addRef(ref{Path: p.file.Path, From: from, To: rl.parent.name})
				if t, ok := byGA[p.eff.Parent.ga()]; ok && p.eff.Parent.Version == t.eff.Version {
					if addX(xref{Path: p.file.Path, From: from, ToPath: t.file.Path, To: t.eff.ArtifactID}) {
						c.parentEdges++
					}
				}
			} else {
				c.parentNot++
				fmt.Fprintf(os.Stderr, "parent not attested: %s %s (line %d)\n", p.file.Path, rl.parent.name, rl.parent.line)
			}
		}
		matched := map[int]bool{}
		for _, d := range rl.deps {
			found := -1
			for i, vd := range vp.deps {
				if vd.art == d.name && vd.loc == (located{own, d.line}) {
					found = i
					break
				}
			}
			if found < 0 {
				// An active profile of the same POM that re-declares the
				// dependency is where Maven locates the effective one: the
				// POM still declares it, at another of its own lines.
				for i, vd := range vp.deps {
					if vd.art == d.name && vd.loc.model == own {
						found = i
						c.depOtherLine++
						break
					}
				}
			}
			if found < 0 {
				c.depNot++
				fmt.Fprintf(os.Stderr, "dependency not attested: %s %s (line %d)\n", p.file.Path, d.name, d.line)
				continue
			}
			matched[found] = true
			c.depAttested++
			addRef(ref{Path: p.file.Path, From: from, To: d.name})
			vd := vp.deps[found]
			if t, ok := byGA[vd.ga]; ok {
				if addX(xref{Path: p.file.Path, From: from, ToPath: t.file.Path, To: t.eff.ArtifactID}) {
					c.depEdges++
				}
			} else if byArt[d.name] > 0 {
				c.sameNameOtherGroup++
			} else {
				c.externalDeps++
			}
		}
		for i, vd := range vp.deps {
			if vd.loc.model == own && !matched[i] {
				c.ownDepsNotTopLevel++
			}
		}
		for _, m := range rl.modules {
			child, ok := moduleFile(root, p.file.Path, m.name)
			t, okT := byPath[child]
			if !ok || !okT {
				c.moduleNoTo++
				continue
			}
			merged := false
			for i := range cuts {
				if cuts[i].from.file.Path == p.file.Path && cuts[i].to.file.Path == t.file.Path {
					cuts[i].ms = append(cuts[i].ms, m)
					merged = true
				}
			}
			if !merged {
				cuts = append(cuts, cut{from: p, to: t, ms: []rawLoc{m}, src: src})
			}
		}
	}
	if ma != nil && len(cuts) > 0 {
		base, err := ma.reactor(ctx, nil)
		if err != nil {
			return nil, nil, c, err
		}
		for _, p := range ps {
			if !base[p.eff.ga()] {
				c.baselineDiffers = true
			}
		}
		if len(base) != len(ps) {
			c.baselineDiffers = true
		}
		type answer struct {
			attested bool
			err      error
		}
		answers := make([]answer, len(cuts))
		sem := make(chan struct{}, max(1, jobs))
		done := make(chan struct{})
		for i := range cuts {
			go func() {
				sem <- struct{}{}
				defer func() { <-sem; done <- struct{}{} }()
				k := cuts[i]
				if c.baselineDiffers {
					return
				}
				edited := k.src
				for _, m := range k.ms {
					var ok bool
					if edited, ok = cutModule(edited, m); !ok {
						return
					}
				}
				got, err := ma.reactor(ctx, map[string][]byte{k.from.file.Path: edited})
				answers[i] = answer{attested: err == nil && !got[k.to.eff.ga()], err: err}
			}()
		}
		for range cuts {
			<-done
		}
		for i, k := range cuts {
			if answers[i].err != nil {
				return nil, nil, c, answers[i].err
			}
			if !answers[i].attested {
				c.moduleNot++
				fmt.Fprintf(os.Stderr, "module not attested: %s <module>%s</module> (%d occurrences)\n", k.from.file.Path, k.ms[0].name, len(k.ms))
				continue
			}
			c.moduleAttested++
			if addX(xref{Path: k.from.file.Path, From: k.from.eff.ArtifactID, ToPath: k.to.file.Path, To: k.to.eff.ArtifactID}) {
				c.moduleEdges++
			}
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		a, b := refs[i], refs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.To < b.To
	})
	sort.Slice(xrefs, func(i, j int) bool {
		a, b := xrefs[i], xrefs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.ToPath < b.ToPath
	})
	return refs, xrefs, c, nil
}
