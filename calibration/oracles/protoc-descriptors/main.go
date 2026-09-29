// Command protoc-descriptors is the language-breadth harness's tier-A oracle for
// the protobuf dialect (design §1.3: the format's reference compiler). It writes
// indexbench lang-oracle's tool-oracle JSON (-tool-oracle, design §10):
//
//	go run . -subject <checkout> [-protoc <protoc>] [-I <root>]... > protobuf-<subject>.json
//	go run . -subject <checkout> -image <buf build -o -#format=json output> > protobuf-<subject>-buf.json
//
// Every .proto file of the subject (the connector's own exclusions: vendor,
// testdata, node_modules and dot directories are not skipped here; the harness
// drops an entry on a file fathom does not read) is compiled by protoc on its
// own, `protoc --include_source_info --descriptor_set_out=<f> -I <roots>
// <file>`, and the FileDescriptorSet protoc writes is decoded. The roots are the
// -I flags when given, else every buf module root the subject declares (a v2
// buf.yaml's modules[].path, a v1 buf.yaml's own directory, a buf.work.yaml's
// directories) and the repository root; a file is compiled under the longest root
// that holds it, with every root on the proto path so an import resolves as buf
// would resolve it inside the workspace. A file protoc rejects (an import no root
// holds: an external dependency) is counted and named on stderr and contributes
// nothing. With -image the same walk reads the FileDescriptorSet buf built
// (protojson), a cross-check of protoc's reading by a second compiler.
//
// What it writes, every line from the descriptor's source_code_info spans:
//
//   - definitions: a `message` per message (nested ones too, named by their own
//     name; the synthetic map-entry messages protoc creates for a map field are
//     not written), an `enum` per enum (nested ones too), a `service` per
//     service and an `rpc` per method, each at its span's first line with its
//     last line as end. A proto2 group is the nested message protoc makes of it,
//     at the group statement.
//   - references: (enclosing definition, referenced type's own name) for every
//     field whose type is a message or an enum — a message's own fields, its
//     oneof members, a map field's value (attributed to the message holding the
//     map, not to the synthetic entry) and an extension field declared inside a
//     message — and (rpc, request type) and (rpc, response type) for every
//     method. A group field is not a reference (the group declares its type in
//     place, no type name is written), nor is an extension declared at file
//     level (no definition encloses it) or an extend's target.
//   - imports: every import statement whose file is a file of the subject
//     (protoc's own resolution of the path through the roots), at the
//     statement's line; a well-known or external import is not written.
//   - cross_references: a reference whose type protoc resolved to a definition
//     in another file of the subject (file and entity granularity).
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
	"path"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"gopkg.in/yaml.v3"
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

func main() {
	subject := flag.String("subject", "", "subject checkout (required)")
	protoc := flag.String("protoc", "protoc", "protoc binary")
	image := flag.String("image", "", "read this `buf build -o -#format=json` image instead of running protoc")
	var roots multi
	flag.Var(&roots, "I", "proto root, subject-relative (repeatable; default: the buf module roots and the repository root)")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: protoc-descriptors -subject <checkout> [-protoc <bin>] [-I <root>]... [-image <buf json>]")
		os.Exit(2)
	}
	root, err := filepath.Abs(*subject)
	if err != nil {
		fail(err)
	}
	files, err := protoFiles(root)
	if err != nil {
		fail(err)
	}
	if len(roots) == 0 {
		roots = bufRoots(root)
	}
	sort.SliceStable(roots, func(i, j int) bool { return len(roots[i]) > len(roots[j]) })

	var set []*descriptorpb.FileDescriptorProto
	tool, version := "protoc", ""
	if *image != "" {
		tool, version = "buf build", "(image)"
		b, err := os.ReadFile(*image)
		if err != nil {
			fail(err)
		}
		var fds descriptorpb.FileDescriptorSet
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, &fds); err != nil {
			fail(fmt.Errorf("%s: %w", *image, err))
		}
		set = fds.File
	} else {
		out, err := exec.Command(*protoc, "--version").Output()
		if err != nil {
			fail(err)
		}
		version = strings.TrimSpace(strings.TrimPrefix(string(out), "libprotoc "))
		tmp, err := os.MkdirTemp("", "protoc-descriptors-*")
		if err != nil {
			fail(err)
		}
		defer os.RemoveAll(tmp)
		rejected := 0
		for i, f := range files {
			own := rootOf(f, roots)
			args := []string{"--include_source_info", "--descriptor_set_out=" + filepath.Join(tmp, fmt.Sprintf("%d.pb", i)), "-I", filepath.Join(root, own)}
			for _, r := range roots {
				if r != own {
					args = append(args, "-I", filepath.Join(root, r))
				}
			}
			args = append(args, filepath.Join(root, f))
			cmd := exec.Command(*protoc, args...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				rejected++
				fmt.Fprintf(os.Stderr, "protoc rejected %s: %s\n", f, firstLine(stderr.String()))
				continue
			}
			b, err := os.ReadFile(filepath.Join(tmp, fmt.Sprintf("%d.pb", i)))
			if err != nil {
				fail(err)
			}
			var fds descriptorpb.FileDescriptorSet
			if err := proto.Unmarshal(b, &fds); err != nil {
				fail(err)
			}
			set = append(set, fds.File...)
		}
		fmt.Fprintf(os.Stderr, "protoc: %d files, %d compiled, %d rejected; roots %q\n", len(files), len(files)-rejected, rejected, roots)
	}

	fileSet := map[string]bool{}
	for _, f := range files {
		fileSet[f] = true
	}
	// repoPath maps a descriptor's virtual name to the subject file it is:
	// the first root (longest first) holding a file of that name.
	repoPath := func(name string) (string, bool) {
		for _, r := range roots {
			if p := joinRel(r, name); fileSet[p] {
				return p, true
			}
		}
		return "", false
	}
	res := output{Tool: tool, Version: version, Tier: "A", Definitions: []def{}, References: []ref{}, Imports: []imp{}, CrossReferences: []xref{}}
	// Every definition protoc resolved, by full name, for cross_references.
	where := map[string][2]string{} // ".pkg.Outer.Inner" -> (file, "Inner")
	type pending struct {
		file, from, typeName string
	}
	var refs []pending
	seen := map[string]bool{}
	for _, fd := range set {
		file, ok := repoPath(fd.GetName())
		if !ok || seen[file] {
			continue
		}
		seen[file] = true
		spans := spanIndex(fd)
		pkg := ""
		if fd.GetPackage() != "" {
			pkg = "." + fd.GetPackage()
		}
		add := func(p []int32, name, kind string) {
			line, end := spans.lines(p)
			res.Definitions = append(res.Definitions, def{Path: file, Name: name, Line: line, End: end, Kind: kind})
		}
		var walkEnum func(p []int32, scope string, e *descriptorpb.EnumDescriptorProto)
		walkEnum = func(p []int32, scope string, e *descriptorpb.EnumDescriptorProto) {
			add(p, e.GetName(), "enum")
			where[scope+"."+e.GetName()] = [2]string{file, e.GetName()}
		}
		var walkMsg func(p []int32, scope string, m *descriptorpb.DescriptorProto, from string)
		walkMsg = func(p []int32, scope string, m *descriptorpb.DescriptorProto, from string) {
			full := scope + "." + m.GetName()
			owner := m.GetName()
			if m.GetOptions().GetMapEntry() {
				owner = from // a map field's value is the holding message's reference
			} else {
				add(p, m.GetName(), "message")
				where[full] = [2]string{file, m.GetName()}
			}
			// A map field's own type is the synthetic entry message: not a written
			// type; its value field's type is (above, attributed to this message).
			entries := map[string]bool{}
			for _, n := range m.GetNestedType() {
				if n.GetOptions().GetMapEntry() {
					entries[full+"."+n.GetName()] = true
				}
			}
			fieldRefs := func(fs []*descriptorpb.FieldDescriptorProto) {
				for _, f := range fs {
					if f.GetTypeName() == "" || f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_GROUP || entries[f.GetTypeName()] {
						continue
					}
					refs = append(refs, pending{file, owner, f.GetTypeName()})
				}
			}
			fieldRefs(m.GetField())
			fieldRefs(m.GetExtension())
			for i, n := range m.GetNestedType() {
				walkMsg(append(clone(p), 3, int32(i)), full, n, owner)
			}
			for i, e := range m.GetEnumType() {
				walkEnum(append(clone(p), 4, int32(i)), full, e)
			}
		}
		for i, m := range fd.GetMessageType() {
			walkMsg([]int32{4, int32(i)}, pkg, m, "")
		}
		for i, e := range fd.GetEnumType() {
			walkEnum([]int32{5, int32(i)}, pkg, e)
		}
		for i, s := range fd.GetService() {
			add([]int32{6, int32(i)}, s.GetName(), "service")
			for j, m := range s.GetMethod() {
				add([]int32{6, int32(i), 2, int32(j)}, m.GetName(), "rpc")
				refs = append(refs, pending{file, m.GetName(), m.GetInputType()}, pending{file, m.GetName(), m.GetOutputType()})
			}
		}
		for j, dep := range fd.GetDependency() {
			to, ok := repoPath(dep)
			if !ok {
				continue
			}
			line, _ := spans.lines([]int32{3, int32(j)})
			res.Imports = append(res.Imports, imp{Path: file, Line: line, ToPath: to, Module: dep})
		}
	}
	for _, r := range refs {
		name := r.typeName[strings.LastIndex(r.typeName, ".")+1:]
		res.References = append(res.References, ref{Path: r.file, From: r.from, To: name})
		if w, ok := where[r.typeName]; ok && w[0] != r.file {
			res.CrossReferences = append(res.CrossReferences, xref{Path: r.file, From: r.from, ToPath: w[0], To: w[1]})
		}
	}
	sortAll(&res)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(res); err != nil {
		fail(err)
	}
}

// spans maps a source_code_info path to its span.
type spans map[string][]int32

func spanIndex(fd *descriptorpb.FileDescriptorProto) spans {
	out := spans{}
	for _, loc := range fd.GetSourceCodeInfo().GetLocation() {
		k := key(loc.GetPath())
		if _, dup := out[k]; !dup {
			out[k] = loc.GetSpan()
		}
	}
	return out
}

// lines is a path's first and last line, 1-based; a span of three is one line.
func (s spans) lines(p []int32) (int, int) {
	sp := s[key(p)]
	switch len(sp) {
	case 3:
		return int(sp[0]) + 1, int(sp[0]) + 1
	case 4:
		return int(sp[0]) + 1, int(sp[2]) + 1
	}
	return 0, 0
}

func key(p []int32) string {
	var b strings.Builder
	for _, x := range p {
		fmt.Fprintf(&b, "%d,", x)
	}
	return b.String()
}

func clone(p []int32) []int32 { return append([]int32(nil), p...) }

// protoFiles is every .proto file under root, slash-relative and sorted; .git
// is not walked.
func protoFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".proto") {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// bufRoots is every buf module root the subject declares, then the repository
// root ("").
func bufRoots(root string) []string {
	seen := map[string]bool{"": true}
	out := []string{}
	addRoot := func(r string) {
		r = cleanRel(r)
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		dir, _ := filepath.Rel(root, filepath.Dir(p))
		dir = filepath.ToSlash(dir)
		switch d.Name() {
		case "buf.yaml":
			var y struct {
				Version string `yaml:"version"`
				Modules []struct {
					Path string `yaml:"path"`
				} `yaml:"modules"`
			}
			b, _ := os.ReadFile(p)
			if yaml.Unmarshal(b, &y) != nil {
				return nil
			}
			if y.Version == "v2" && len(y.Modules) > 0 {
				for _, m := range y.Modules {
					addRoot(path.Join(dir, m.Path))
				}
				return nil
			}
			addRoot(dir)
		case "buf.work.yaml":
			var y struct {
				Directories []string `yaml:"directories"`
			}
			b, _ := os.ReadFile(p)
			if yaml.Unmarshal(b, &y) == nil {
				for _, d := range y.Directories {
					addRoot(path.Join(dir, d))
				}
			}
		}
		return nil
	})
	sort.Strings(out)
	return append(out, "")
}

// rootOf is the longest root holding f (roots are sorted longest first).
func rootOf(f string, roots []string) string {
	for _, r := range roots {
		if r == "" || strings.HasPrefix(f, r+"/") {
			return r
		}
	}
	return ""
}

func joinRel(elem ...string) string { return cleanRel(path.Join(elem...)) }

func cleanRel(p string) string {
	p = path.Clean(p)
	if p == "." || p == "/" {
		return ""
	}
	return p
}

func sortAll(o *output) {
	sort.Slice(o.Definitions, func(i, j int) bool {
		a, b := o.Definitions[i], o.Definitions[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.Slice(o.References, func(i, j int) bool {
		a, b := o.References[i], o.References[j]
		return a.Path+"\x00"+a.From+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.To
	})
	sort.Slice(o.Imports, func(i, j int) bool {
		a, b := o.Imports[i], o.Imports[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Line < b.Line
	})
	sort.Slice(o.CrossReferences, func(i, j int) bool {
		a, b := o.CrossReferences[i], o.CrossReferences[j]
		return a.Path+"\x00"+a.From+"\x00"+a.ToPath+"\x00"+a.To < b.Path+"\x00"+b.From+"\x00"+b.ToPath+"\x00"+b.To
	})
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "protoc-descriptors:", err)
	os.Exit(1)
}
