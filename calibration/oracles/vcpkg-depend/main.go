// Command vcpkg-depend-oracle is the language-breadth harness's oracle for the
// vcpkg doc-engine dialect (design §1.3, owner decision D4). It writes
// indexbench lang-oracle's tool-oracle JSON (-tool-oracle, design §10), tier A:
//
//	go run . -subject <checkout> -vcpkg <vcpkg> -vcpkg-root <registry checkout> -scratch <dir> \
//	    [-under <dir>] [-files-out <file>] [-xref-out <file>] [-depend-info] > out.json
//
// The tool is vcpkg itself. Each vcpkg.json of the subject is copied into a
// scratch overlay directory (a port needs no portfile for the query) and
// evaluated by
//
//	vcpkg x-package-info <name> --overlay-ports=<overlay> --x-json
//
// vcpkg parses and validates the manifest and prints the normalized one: the
// name, the version, the dependencies (a string or an object, each with its
// host, platform and feature qualifiers) and every feature with its own
// dependencies. A manifest vcpkg refuses (a template with an unexpanded
// version, a document that is not JSON) is not attested: it is no definition,
// and the reading's dialect-side count shows it as a file the oracle cannot
// speak for.
//
//   - definitions: one `package` per attested manifest, named by the tool's
//     name, at the line of the manifest's opening brace (the package entity is
//     the document root); one `key` per feature the tool lists, named
//     `features.<name>` as the dialect names it, at the line of its key (the
//     tool writes no lines; a stdlib token scan of the file finds them).
//   - references: (package, name) for every dependency the tool lists for the
//     package or for any of its features, by dependency name.
//   - cross_references (-xref-out, entity granularity): a reference whose name
//     is declared by exactly one other attested manifest of the subject, to
//     that manifest's package. A name two manifests declare is ambiguous and
//     is not an edge (the resolver refuses to guess; counted on stderr). With
//     -depend-info the declared edges are cross-checked against
//     `vcpkg depend-info '<name>[*]' --max-recurse=1 --triplet=<t>` on the
//     triplets arm64-osx, x64-windows, x64-linux (the tool's own resolved
//     direct dependencies with every feature on; a host dependency carries
//     `:host`): declared-and-resolved, declared-only (a platform the three
//     triplets exclude, or a name the registry resolves elsewhere) and
//     resolved-only are printed on stderr.
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	Definitions     []def  `json:"definitions,omitempty"`
	References      []ref  `json:"references,omitempty"`
	CrossReferences []xref `json:"cross_references,omitempty"`
}

type depObj struct {
	Name string `json:"name"`
}

type toolPkg struct {
	Name         string            `json:"name"`
	Dependencies []json.RawMessage `json:"dependencies"`
	Features     map[string]struct {
		Dependencies []json.RawMessage `json:"dependencies"`
	} `json:"features"`
}

func depName(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var o depObj
	if json.Unmarshal(raw, &o) == nil {
		return o.Name
	}
	return ""
}

type attested struct {
	path string // subject-relative
	name string
	pkg  toolPkg
	src  []byte
	deps map[string]bool
}

func lineAt(src []byte, off int) int {
	if off > len(src) {
		off = len(src)
	}
	return 1 + bytes.Count(src[:off], []byte("\n"))
}

// rootLine is the line of the document's opening brace.
func rootLine(src []byte) int {
	i := 0
	for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\r' || src[i] == '\n' || bytes.HasPrefix(src[i:], []byte("\xef\xbb\xbf"))) {
		if bytes.HasPrefix(src[i:], []byte("\xef\xbb\xbf")) {
			i += 3
			continue
		}
		i++
	}
	return lineAt(src, i)
}

// featureLines scans src with encoding/json tokens and returns the line of
// each key of the top-level "features" object.
func featureLines(src []byte) map[string]int {
	res := map[string]int{}
	dec := json.NewDecoder(bytes.NewReader(src))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return res
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return res
		}
		k, _ := kt.(string)
		if k != "features" {
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return res
			}
			continue
		}
		if t, err := dec.Token(); err != nil || t != json.Delim('{') {
			return res
		}
		for dec.More() {
			ft, err := dec.Token()
			if err != nil {
				return res
			}
			name, _ := ft.(string)
			res[name] = lineAt(src, int(dec.InputOffset()))
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return res
			}
		}
		dec.Token()
	}
	return res
}

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	vcpkg := flag.String("vcpkg", "vcpkg", "the vcpkg binary")
	root := flag.String("vcpkg-root", os.Getenv("VCPKG_ROOT"), "the registry checkout (VCPKG_ROOT)")
	scratch := flag.String("scratch", "", "scratch directory outside the subject (required)")
	under := flag.String("under", "", "only manifests below this subject-relative directory")
	filesOut := flag.String("files-out", "", "write the attested manifests, one per line")
	xrefOut := flag.String("xref-out", "", "write the cross_references file here")
	depend := flag.Bool("depend-info", false, "cross-check the declared edges against vcpkg depend-info")
	jobs := flag.Int("j", 6, "parallel vcpkg processes")
	flag.Parse()
	if *subject == "" || *scratch == "" || *root == "" {
		fmt.Fprintln(os.Stderr, "usage: vcpkg-depend-oracle -subject <checkout> -vcpkg-root <registry> -scratch <dir> [-vcpkg <bin>]")
		os.Exit(2)
	}
	subj, err := filepath.Abs(*subject)
	if err != nil {
		fatal(err)
	}
	if real, err := filepath.EvalSymlinks(subj); err == nil {
		subj = real
	}
	downloads := filepath.Join(*scratch, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		fatal(err)
	}
	env := append(os.Environ(), "VCPKG_ROOT="+*root, "VCPKG_DISABLE_METRICS=1", "VCPKG_DOWNLOADS="+downloads)
	version := "unknown"
	if b, err := exec.Command(*vcpkg, "version").Output(); err == nil {
		version = strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0])
	}

	var manifests []string
	startDir := subj
	if *under != "" {
		startDir = filepath.Join(subj, filepath.FromSlash(*under))
	}
	filepath.WalkDir(startDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && d.Name() == "vcpkg.json" {
			r, _ := filepath.Rel(subj, p)
			manifests = append(manifests, filepath.ToSlash(r))
		}
		return nil
	})
	sort.Strings(manifests)

	ovRoot := filepath.Join(*scratch, "overlay")
	os.RemoveAll(ovRoot)
	type job struct {
		idx  int
		path string
	}
	results := make([]*attested, len(manifests))
	notes := make([]string, len(manifests))
	var wg sync.WaitGroup
	ch := make(chan job)
	for w := 0; w < *jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				src, err := os.ReadFile(filepath.Join(subj, filepath.FromSlash(j.path)))
				if err != nil {
					notes[j.idx] = j.path + ": " + err.Error()
					continue
				}
				var head struct {
					Name string `json:"name"`
				}
				if json.Unmarshal(src, &head) != nil || head.Name == "" {
					notes[j.idx] = j.path + ": no name or not JSON (not attested)"
					continue
				}
				dir := filepath.Join(ovRoot, fmt.Sprintf("m%d", j.idx), head.Name)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					notes[j.idx] = j.path + ": " + err.Error()
					continue
				}
				os.WriteFile(filepath.Join(dir, "vcpkg.json"), src, 0o644)
				os.WriteFile(filepath.Join(dir, "portfile.cmake"), nil, 0o644)
				cmd := exec.Command(*vcpkg, "x-package-info", head.Name, "--overlay-ports="+filepath.Dir(dir), "--x-json")
				cmd.Env = env
				var so, se bytes.Buffer
				cmd.Stdout, cmd.Stderr = &so, &se
				done := make(chan error, 1)
				go func() { done <- cmd.Run() }()
				select {
				case err = <-done:
				case <-time.After(2 * time.Minute):
					cmd.Process.Kill()
					err = fmt.Errorf("timeout")
				}
				// x-package-info exits 1 even when it answers, so the answer
				// is the judge: a result for the name, or the tool's own message.
				var res struct {
					Results map[string]toolPkg `json:"results"`
				}
				p, ok := toolPkg{}, false
				if json.Unmarshal(so.Bytes(), &res) == nil {
					p, ok = res.Results[head.Name]
				}
				if !ok {
					msg := firstLine(se.String() + so.String())
					if err != nil && strings.Contains(msg, "Operation not permitted") {
						fatal(fmt.Errorf("%s: %s", j.path, msg))
					}
					notes[j.idx] = j.path + ": vcpkg refused: " + msg
					continue
				}
				a := &attested{path: j.path, name: p.Name, pkg: p, src: src, deps: map[string]bool{}}
				for _, d := range p.Dependencies {
					if n := depName(d); n != "" {
						a.deps[n] = true
					}
				}
				for _, f := range p.Features {
					for _, d := range f.Dependencies {
						if n := depName(d); n != "" {
							a.deps[n] = true
						}
					}
				}
				results[j.idx] = a
			}
		}()
	}
	for i, m := range manifests {
		ch <- job{i, m}
	}
	close(ch)
	wg.Wait()

	var atts []*attested
	refused := 0
	for i, a := range results {
		if a != nil {
			atts = append(atts, a)
		} else if notes[i] != "" {
			refused++
			fmt.Fprintln(os.Stderr, "note:", notes[i])
		}
	}
	fmt.Fprintf(os.Stderr, "manifests %d, attested %d, not attested %d\n", len(manifests), len(atts), refused)

	o := out{Tool: "vcpkg x-package-info --x-json", Version: version, Tier: "A"}
	byName := map[string][]*attested{}
	for _, a := range atts {
		byName[a.name] = append(byName[a.name], a)
		o.Definitions = append(o.Definitions, def{Path: a.path, Name: a.name, Line: rootLine(a.src), Kind: "package"})
		lines := featureLines(a.src)
		var fnames []string
		for f := range a.pkg.Features {
			fnames = append(fnames, f)
		}
		sort.Strings(fnames)
		for _, f := range fnames {
			ln, ok := lines[f]
			if !ok {
				fmt.Fprintf(os.Stderr, "note: %s: feature %s has no key line\n", a.path, f)
				continue
			}
			o.Definitions = append(o.Definitions, def{Path: a.path, Name: "features." + f, Line: ln, Kind: "key"})
		}
		var dn []string
		for d := range a.deps {
			dn = append(dn, d)
		}
		sort.Strings(dn)
		for _, d := range dn {
			o.References = append(o.References, ref{Path: a.path, From: a.name, To: d})
		}
	}

	xo := out{Tool: "vcpkg x-package-info --x-json (dependencies joined by name to the attested manifests of the subject)", Version: version, Tier: "A"}
	ambiguous := 0
	declared := map[[2]string]bool{} // (from name, to name) per path pair
	for _, a := range atts {
		var dn []string
		for d := range a.deps {
			dn = append(dn, d)
		}
		sort.Strings(dn)
		for _, d := range dn {
			ts := byName[d]
			var others []*attested
			for _, t := range ts {
				if t.path != a.path {
					others = append(others, t)
				}
			}
			if len(ts) == 0 || (len(others) == 0) {
				continue
			}
			if len(ts) > 1 {
				ambiguous++
				continue
			}
			xo.CrossReferences = append(xo.CrossReferences, xref{Path: a.path, From: a.name, ToPath: others[0].path, To: others[0].name})
			declared[[2]string{a.name, d}] = true
		}
	}
	fmt.Fprintf(os.Stderr, "cross references %d, ambiguous names skipped %d\n", len(xo.CrossReferences), ambiguous)

	if *depend {
		crossCheck(*vcpkg, env, ovRoot, atts, byName, declared, *jobs)
	}

	if *filesOut != "" {
		var lines []string
		for _, a := range atts {
			lines = append(lines, a.path)
		}
		os.WriteFile(*filesOut, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	}
	if *xrefOut != "" {
		writeJSON(*xrefOut, xo)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(o); err != nil {
		fatal(err)
	}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

func writeJSON(path string, v any) {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "vcpkg-depend-oracle:", err)
	os.Exit(1)
}

// crossCheck compares the declared in-subject edges with what depend-info
// resolves with every feature on, over three triplets.
func crossCheck(vcpkg string, env []string, ovRoot string, atts []*attested, byName map[string][]*attested, declared map[[2]string]bool, jobs int) {
	// One overlay directory holding the first manifest of each name.
	all := filepath.Join(ovRoot, "all")
	os.MkdirAll(all, 0o755)
	seen := map[string]bool{}
	for _, a := range atts {
		if seen[a.name] {
			continue
		}
		seen[a.name] = true
		d := filepath.Join(all, a.name)
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "vcpkg.json"), a.src, 0o644)
		os.WriteFile(filepath.Join(d, "portfile.cmake"), nil, 0o644)
	}
	triplets := []string{"arm64-osx", "x64-windows", "x64-linux"}
	var mu sync.Mutex
	resolved := map[[2]string]bool{}
	failed := 0
	var wg sync.WaitGroup
	ch := make(chan *attested)
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for a := range ch {
				for _, t := range triplets {
					cmd := exec.Command(vcpkg, "depend-info", a.name+"[*]", "--max-recurse=1", "--format=list", "--triplet="+t, "--overlay-ports="+all)
					cmd.Env = env
					b, err := cmd.CombinedOutput() // depend-info writes its list to stderr
					if err != nil && len(b) == 0 {
						mu.Lock()
						failed++
						mu.Unlock()
						continue
					}
					for _, ln := range strings.Split(string(b), "\n") {
						head, tail, ok := strings.Cut(ln, ": ")
						if !ok {
							continue
						}
						n := head
						if i := strings.IndexByte(n, '['); i >= 0 {
							n = n[:i]
						}
						if n != a.name {
							continue
						}
						for _, d := range strings.Split(tail, ", ") {
							d = strings.TrimSuffix(strings.TrimSpace(d), ":host")
							if i := strings.IndexByte(d, '['); i >= 0 {
								d = d[:i]
							}
							if d == "" || !seen[d] {
								continue
							}
							mu.Lock()
							resolved[[2]string{a.name, d}] = true
							mu.Unlock()
						}
					}
				}
			}
		}()
	}
	for _, a := range atts {
		ch <- a
	}
	close(ch)
	wg.Wait()
	both, declOnly, resOnly := 0, 0, 0
	var ex []string
	for k := range declared {
		if resolved[k] {
			both++
		} else {
			declOnly++
			if len(ex) < 8 {
				ex = append(ex, k[0]+"->"+k[1])
			}
		}
	}
	var rex []string
	for k := range resolved {
		if !declared[k] && k[0] != k[1] {
			// a name two manifests declare is not a declared edge
			if len(byName[k[1]]) > 1 {
				continue
			}
			resOnly++
			if len(rex) < 8 {
				rex = append(rex, k[0]+"->"+k[1])
			}
		}
	}
	sort.Strings(ex)
	sort.Strings(rex)
	fmt.Fprintf(os.Stderr, "depend-info cross-check: declared %d, resolved by the tool %d, declared-only %d (%v), resolved-only %d (%v), failed runs %d\n",
		len(declared), both, declOnly, ex, resOnly, rex, failed)
}
