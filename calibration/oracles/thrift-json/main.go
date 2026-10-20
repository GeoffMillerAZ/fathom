// Command thrift-json is the language-breadth harness's tier-A oracle for the thrift dialect (design §1.3: the
// format's reference compiler). It writes indexbench lang-oracle's tool-oracle JSON (-tool-oracle, design §10):
//
//	go run . -subject <checkout> [-thrift <thrift>] [-I <root>]... > workspace/tmp/oracle/thrift-<subject>.json
//
// Every .thrift file of the subject is compiled on its own, `thrift --gen json -I <root> -o <dir> <file>`. A file the
// compiler rejects (an include no root holds, or a syntax the Apache compiler does not read) is named on stderr and
// contributes nothing. `thrift --gen json` carries no source positions and resolves typedef names away, so the
// harness places declarations and type names with its own scanner (scan.go) and holds the scanner to the compiler: a
// file is written only when the names the scanner finds equal the compiler's (every namespace scope, typedef, const,
// enum and value, struct, union, exception and field, service and function) and every user type the compiler writes
// into a field, argument, return, throws or extends is a type the scanner found at that member. What the scanner adds
// beyond the compiler's attestation is only where each name is written and the references through a typedef name
// (the compiler's JSON writes the typedef's target type there, not its name); the compiler confirms that the name is
// a typedef.
//
// What it writes:
//
//   - definitions: namespace (named by its scope: java, py, *), typedef, const, enum, enum_value, struct, union,
//     exception, field, service, function, each by its own name, at its first line, with its last line as end.
//   - references: (enclosing member, referenced type's own name) for every user type written in a field, a function's
//     argument, return or throws list, a typedef's or const's type, and a service's `extends`; a member that is a
//     function, field, typedef, const or service is the enclosing one.
//   - imports: every include statement whose file is a file of the subject, at the statement's line.
//   - cross_references: a reference written `file.Type` whose file the include names, resolved to the definition in
//     that file.
//
// Bench-only; never a product dependency.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	End  int    `json:"end"`
	Kind string `json:"kind"`
}

type ref struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type imp struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	ToPath string `json:"to_path"`
	Module string `json:"module"`
}

type xref struct {
	Path   string `json:"path"`
	From   string `json:"from"`
	ToPath string `json:"to_path"`
	To     string `json:"to"`
}

type output struct {
	Tool            string `json:"tool"`
	Version         string `json:"version"`
	Tier            string `json:"tier"`
	Definitions     []def  `json:"definitions"`
	References      []ref  `json:"references"`
	Imports         []imp  `json:"imports"`
	CrossReferences []xref `json:"cross_references"`
}

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

// compiled is what the compiler says about one file.
type compiled struct {
	names    map[string]bool   // kind|container|name
	typedefs map[string]bool   // typedef names
	noConsts bool              // the constants section was unreadable: constants are not attested
	classes  map[string]string // from-member (container|name) -> "T1,T2" user types the JSON writes (last segment, sorted)
}

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	thrift := flag.String("thrift", "thrift", "thrift compiler binary")
	tmp := flag.String("tmp", "", "scratch directory for the compiler's output (default: a new directory under $TMPDIR)")
	var roots multi
	flag.Var(&roots, "I", "include root, subject-relative (repeatable; default: the repository root)")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: thrift-json -subject <checkout> [-thrift <bin>] [-I <root>]...")
		os.Exit(2)
	}
	root, err := filepath.Abs(*subject)
	if err != nil {
		fail(err)
	}
	files, err := thriftFiles(root)
	if err != nil {
		fail(err)
	}
	if len(roots) == 0 {
		roots = multi{""}
	}
	work, err := os.MkdirTemp(*tmp, "thrift-json-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(work)
	ctx := context.Background()
	version := compilerVersion(ctx, *thrift)

	docs := map[string]*Doc{}
	comp := map[string]*compiled{}
	var rejected []string
	for _, rel := range files {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			fail(err)
		}
		doc, serr := Scan(string(src))
		c, cerr := compile(ctx, *thrift, root, rel, roots, work)
		switch {
		case cerr != nil:
			rejected = append(rejected, rel+": thrift: "+cerr.Error())
			continue
		case serr != nil:
			rejected = append(rejected, rel+": scanner: "+serr.Error())
			continue
		}
		docs[rel], comp[rel] = doc, c
	}

	// The scanner must equal the compiler, file by file.
	typedefs := map[string]bool{}
	for _, c := range comp {
		for t := range c.typedefs {
			typedefs[t] = true
		}
	}
	for _, rel := range sortedKeys(docs) {
		if why := agree(docs[rel], comp[rel], typedefs); why != "" {
			rejected = append(rejected, rel+": scanner disagrees with thrift: "+why)
			delete(docs, rel)
		}
	}

	res := output{Tool: "thrift", Version: version, Tier: "A"}
	byPath := map[string]map[string][]string{} // file -> type name -> kinds (for cross references)
	for _, rel := range sortedKeys(docs) {
		byPath[rel] = map[string][]string{}
		for _, d := range docs[rel].Decls {
			if d.Container == "" {
				byPath[rel][d.Name] = append(byPath[rel][d.Name], d.Kind)
			}
		}
	}
	for _, rel := range sortedKeys(docs) {
		doc := docs[rel]
		for _, d := range doc.Decls {
			res.Definitions = append(res.Definitions, def{Path: rel, Name: d.Name, Line: d.Line, End: d.End, Kind: d.Kind})
		}
		// includes: program name -> file
		prog := map[string]string{}
		for _, inc := range doc.Includes {
			if to := resolveInclude(root, rel, inc.Path, roots, docs); to != "" {
				res.Imports = append(res.Imports, imp{Path: rel, Line: inc.Line, ToPath: to, Module: inc.Path})
				prog[strings.TrimSuffix(path.Base(inc.Path), ".thrift")] = to
			}
		}
		seen := map[ref]bool{}
		seenX := map[xref]bool{}
		for _, d := range doc.Decls {
			if !referencing(d.Kind) {
				continue
			}
			for _, r := range d.Refs {
				to := lastSegment(r.Written)
				k := ref{Path: rel, From: d.Name, To: to}
				if !seen[k] {
					seen[k] = true
					res.References = append(res.References, k)
				}
				if q, _, ok := strings.Cut(r.Written, "."); ok {
					if tp, ok := prog[q]; ok && byPath[tp] != nil && len(byPath[tp][to]) > 0 {
						x := xref{Path: rel, From: d.Name, ToPath: tp, To: to}
						if !seenX[x] {
							seenX[x] = true
							res.CrossReferences = append(res.CrossReferences, x)
						}
					}
				}
			}
		}
	}
	sortAll(&res)
	fmt.Fprintf(os.Stderr, "thrift-json: %d files compiled and agreed, %d rejected\n", len(docs), len(rejected))
	for _, r := range rejected {
		fmt.Fprintln(os.Stderr, "  rejected", r)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(res); err != nil {
		fail(err)
	}
}

func referencing(kind string) bool {
	switch kind {
	case "field", "function", "typedef", "const", "service":
		return true
	}
	return false
}

func lastSegment(written string) string {
	if i := strings.LastIndex(written, "."); i >= 0 {
		return written[i+1:]
	}
	return written
}

// resolveInclude is the subject file an include names: beside the including file first, then under each root.
func resolveInclude(root, from, inc string, roots []string, docs map[string]*Doc) string {
	cands := []string{path.Clean(path.Join(path.Dir(from), inc))}
	for _, r := range roots {
		cands = append(cands, path.Clean(path.Join(r, inc)))
	}
	for _, c := range cands {
		if _, ok := docs[c]; ok {
			return c
		}
	}
	return ""
}

func compilerVersion(ctx context.Context, bin string) string {
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "Thrift version"))
}

// compile runs the compiler on one file and reads its JSON.
func compile(ctx context.Context, bin, root, rel string, roots []string, work string) (*compiled, error) {
	out := filepath.Join(work, "out")
	if err := os.RemoveAll(out); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	args := []string{"--gen", "json", "-o", out}
	for _, r := range roots {
		args = append(args, "-I", filepath.Join(root, filepath.FromSlash(r)))
	}
	args = append(args, filepath.Join(root, filepath.FromSlash(rel)))
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr, cmd.Stdout = &stderr, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.Join(strings.Fields(stderr.String()), " ")
		if len(msg) > 160 {
			msg = msg[:160]
		}
		return nil, fmt.Errorf("%v: %s", err, msg)
	}
	ents, err := os.ReadDir(filepath.Join(out, "gen-json"))
	if err != nil {
		return nil, err
	}
	var raw []byte
	want := strings.TrimSuffix(path.Base(rel), ".thrift") + ".json"
	for _, e := range ents {
		if e.Name() == want {
			if raw, err = os.ReadFile(filepath.Join(out, "gen-json", e.Name())); err != nil {
				return nil, err
			}
		}
	}
	if raw == nil {
		return nil, fmt.Errorf("no %s written", want)
	}
	return parseJSON(raw)
}

type jtype struct {
	TypeID   string `json:"typeId"`
	Class    string `json:"class"`
	Type     *jtype `json:"type"`
	ElemType *jtype `json:"elemType"`
	KeyType  *jtype `json:"keyType"`
	ValType  *jtype `json:"valueType"`
}

func (t *jtype) classes(out *[]string) {
	if t == nil {
		return
	}
	if t.Class != "" {
		*out = append(*out, lastSegment(t.Class))
	}
	t.Type.classes(out)
	t.ElemType.classes(out)
	t.KeyType.classes(out)
	t.ValType.classes(out)
}

type jfield struct {
	Name string `json:"name"`
	jtype
}

func (f *jfield) UnmarshalJSON(b []byte) error {
	var head struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return err
	}
	f.Name = head.Name
	return json.Unmarshal(b, &f.jtype)
}

type jdoc struct {
	Namespaces map[string]string `json:"namespaces"`
	Enums      []struct {
		Name    string `json:"name"`
		Members []struct {
			Name string `json:"name"`
		} `json:"members"`
	} `json:"enums"`
	Typedefs  []struct{ Name string } `json:"typedefs"`
	Constants []struct {
		Name string `json:"name"`
		jtype
	} `json:"constants"`
	Structs []struct {
		Name        string   `json:"name"`
		IsException bool     `json:"isException"`
		IsUnion     bool     `json:"isUnion"`
		Fields      []jfield `json:"fields"`
	} `json:"structs"`
	Services []struct {
		Name      string `json:"name"`
		Extends   string `json:"extends"`
		Functions []struct {
			Name       string      `json:"name"`
			ReturnType *jtype      `json:"returnType"`
			Arguments  []jfield    `json:"arguments"`
			Exceptions []jfield    `json:"exceptions"`
			Oneway     interface{} `json:"oneway"`
		} `json:"functions"`
	} `json:"services"`
}

func parseJSON(raw []byte) (*compiled, error) {
	var d jdoc
	noConsts := false
	if err := json.Unmarshal(raw, &d); err != nil {
		// A const whose value is a map with non-string keys is written as invalid JSON. The constants section sits
		// between "structs" and "services": read the file without it and leave constants unattested.
		a, b := bytes.Index(raw, []byte(`"constants": [`)), bytes.Index(raw, []byte(`"services": [`))
		if a < 0 || b < a {
			return nil, err
		}
		cut := append(append(append([]byte{}, raw[:a]...), []byte(`"constants": [],`+"\n  ")...), raw[b:]...)
		d = jdoc{}
		if err2 := json.Unmarshal(cut, &d); err2 != nil {
			return nil, err
		}
		noConsts = true
	}
	c := &compiled{names: map[string]bool{}, typedefs: map[string]bool{}, classes: map[string]string{}, noConsts: noConsts}
	add := func(kind, container, name string) { c.names[kind+"|"+container+"|"+name] = true }
	addRefs := func(member string, ts ...*jtype) {
		var cl []string
		for _, t := range ts {
			t.classes(&cl)
		}
		sort.Strings(cl)
		c.classes[member] = strings.Join(dedupe(cl), ",")
	}
	for scope := range d.Namespaces {
		add("namespace", "", scope)
	}
	for _, t := range d.Typedefs {
		add("typedef", "", t.Name)
		c.typedefs[t.Name] = true
	}
	for _, k := range d.Constants {
		add("const", "", k.Name)
	}
	for _, e := range d.Enums {
		add("enum", "", e.Name)
		for _, m := range e.Members {
			add("enum_value", e.Name, m.Name)
		}
	}
	for _, s := range d.Structs {
		kind := "struct"
		switch {
		case s.IsException:
			kind = "exception"
		case s.IsUnion:
			kind = "union"
		}
		add(kind, "", s.Name)
		for i := range s.Fields {
			add("field", s.Name, s.Fields[i].Name)
			addRefs("field|"+s.Name+"|"+s.Fields[i].Name, &s.Fields[i].jtype)
		}
	}
	for _, s := range d.Services {
		add("service", "", s.Name)
		for _, f := range s.Functions {
			add("function", s.Name, f.Name)
			ts := []*jtype{f.ReturnType}
			for i := range f.Arguments {
				ts = append(ts, &f.Arguments[i].jtype)
			}
			for i := range f.Exceptions {
				ts = append(ts, &f.Exceptions[i].jtype)
			}
			addRefs("function|"+s.Name+"|"+f.Name, ts...)
		}
	}
	return c, nil
}

func dedupe(xs []string) []string {
	var out []string
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}

// agree holds the scanner to the compiler; it returns why not, or "".
func agree(doc *Doc, c *compiled, typedefs map[string]bool) string {
	have := map[string]bool{}
	for _, d := range doc.Decls {
		have[d.Kind+"|"+d.Container+"|"+d.Name] = true
	}
	for k := range c.names {
		if !have[k] {
			return "thrift defines " + k + ", the scanner found none"
		}
	}
	for k := range have {
		if c.noConsts && strings.HasPrefix(k, "const|") {
			continue
		}
		if !c.names[k] {
			return "the scanner found " + k + ", thrift defines none"
		}
	}
	for _, d := range doc.Decls {
		if d.Kind != "field" && d.Kind != "function" {
			continue
		}
		key := d.Kind + "|" + d.Container + "|" + d.Name
		viaTypedef := false
		for _, r := range d.Refs {
			if typedefs[lastSegment(r.Written)] {
				viaTypedef = true // the compiler writes a typedef's target, not its name: the class is not this member's own
			}
		}
		if viaTypedef {
			continue
		}
		for _, w := range strings.Split(c.classes[key], ",") {
			if w != "" && !scannedAll(d.Refs, w) {
				return fmt.Sprintf("thrift writes type %s at %s, the scanner found none there", w, key)
			}
		}
	}
	return ""
}

func scannedAll(refs []TypeRef, name string) bool {
	for _, r := range refs {
		if lastSegment(r.Written) == name {
			return true
		}
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func thriftFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".thrift") {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func sortAll(r *output) {
	sort.SliceStable(r.Definitions, func(i, j int) bool {
		a, b := r.Definitions[i], r.Definitions[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.SliceStable(r.References, func(i, j int) bool {
		a, b := r.References[i], r.References[j]
		return a.Path+"\x00"+a.From+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.To
	})
	sort.SliceStable(r.Imports, func(i, j int) bool {
		a, b := r.Imports[i], r.Imports[j]
		return a.Path+"\x00"+fmt.Sprint(a.Line) < b.Path+"\x00"+fmt.Sprint(b.Line)
	})
	sort.SliceStable(r.CrossReferences, func(i, j int) bool {
		a, b := r.CrossReferences[i], r.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "thrift-json:", err)
	os.Exit(1)
}
