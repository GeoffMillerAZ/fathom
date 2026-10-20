package projectdir

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// schemaFS holds verbatim copies of schema/{project,guard,binding,techpack,risk,
// learning}/schema.cue (TestEmbeddedSchemasMatchSource keeps them in sync):
// Go embeds only files under this package's directory.
//
//go:embed schemas
var schemaFS embed.FS

// FileError is a project file that exists but could not be read or does not
// parse. It names its file; the other files still load.
type FileError struct {
	Path string
	Err  error
}

func (e *FileError) Error() string { return fmt.Sprintf("%s: %v", e.Path, e.Err) }

func (e *FileError) Unwrap() error { return e.Err }

// KeyError is one top-level key that fails its #Project sub-schema. It names
// the file and the key; every other key still loads.
type KeyError struct {
	Path string
	Key  string
	Err  error
}

func (e *KeyError) Error() string { return fmt.Sprintf("%s: key %q: %v", e.Path, e.Key, e.Err) }

func (e *KeyError) Unwrap() error { return e.Err }

// ConflictError is one key set by more than one file.
type ConflictError struct {
	Key   string
	Paths []string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("key %q is set by more than one file: %s", e.Key, strings.Join(e.Paths, ", "))
}

// FileStatus is one project file as Load read it.
type FileStatus struct {
	Name File
	Path string
	// ReadErr is os.ReadFile's error, os.ErrNotExist included, so a reader
	// can report exactly what it reported when it read the file itself.
	ReadErr error
	// Value is the file compiled on its own; Value.Err() is a syntax error.
	Value cue.Value
}

// Absent reports whether the file does not exist.
func (f FileStatus) Absent() bool { return errors.Is(f.ReadErr, fs.ErrNotExist) }

// Err is the file's read or syntax error as a *FileError, or nil when the
// file is absent or parsed.
func (f FileStatus) Err() error {
	switch {
	case f.Absent():
		return nil
	case f.ReadErr != nil:
		return &FileError{Path: f.Path, Err: f.ReadErr}
	case f.Value.Err() != nil:
		return &FileError{Path: f.Path, Err: f.Value.Err()}
	}
	return nil
}

// Key is one top-level key of the project value: a #Project key, or
// risk/learning mapped by file name, or a key fathom does not know.
type Key struct {
	Name string
	// File is the file whose value the key carries (on a conflict, the
	// key's home file when it is one of them, else the first by name).
	File File
	Path string
	// Value is the key's value in File.
	Value cue.Value
	// Known is false for a key #Project does not define: a doctor warning,
	// never an error.
	Known bool
	// Entries is the value's size: list elements, struct fields, else 1.
	Entries int
	// Err is a *KeyError (the value fails its sub-schema) or a
	// *ConflictError; nil when the key loads.
	Err error
}

// Project is a repository's .fathom/*.cue files loaded as one value.
type Project struct {
	root  string
	files []FileStatus
	keys  []Key
	known []string
}

// KnownKeys is every key fathom defines: the #Project keys and the keys
// mapped by file name, sorted.
func (p Project) KnownKeys() []string { return append([]string(nil), p.known...) }

// knownFiles are the project files a reader reads by name; Load reports
// each one, absent or not.
var knownFiles = [...]File{Capabilities, Concerns, Guard, Learning, Modules, Pack, Risk}

// fileMapped reports the schema definition a whole file validates against
// when it is mapped by file name (flat content), else "".
func fileMapped(f File) (key, pkg, def string) {
	switch f {
	case Risk:
		return "risk", "risk", "#RiskTable"
	case Learning:
		return "learning", "learning", "#Learning"
	}
	return "", "", ""
}

// Home is the file that holds a known key by convention
// (schema/project/schema.cue's composition table), or "" for a key fathom
// does not define.
func Home(key string) File { return keyHome(key) }

func keyHome(name string) File {
	switch name {
	case "capabilities", "budgets":
		return Capabilities
	case "packs", "concerns":
		return Concerns
	case "bindings":
		return Modules
	case "guards", "layer_rules":
		return Guard
	case "orientation":
		return Pack
	case "risk":
		return Risk
	case "learning":
		return Learning
	}
	return ""
}

// Load reads every .fathom/*.cue file under root, compiles each on its own
// and validates each top-level key against its #Project sub-schema
// (docs/design/fathom-dir.md section 5). A syntax error names its file, a
// failing key names its file and key, and every other key still loads (I9,
// I38). Two files setting one key conflict by name. A key #Project does not
// define loads with Known false. risk.cue and learning.cue map by file name
// to the keys "risk" and "learning". An absent directory is an empty
// project; a directory that exists and cannot be listed is an error, never
// an empty project (I38).
func Load(root string) (Project, error) {
	p := Project{root: root}
	names := map[File]bool{}
	for _, f := range knownFiles {
		names[f] = true
	}
	listing, err := os.ReadDir(Under(root))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Project{}, fmt.Errorf("projectdir: list %s: %w", Under(root), err)
	}
	for _, e := range listing {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".cue") {
			names[File(e.Name())] = true
		}
	}
	ordered := make([]File, 0, len(names))
	for f := range names {
		ordered = append(ordered, f)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })

	ctx := cuecontext.New()
	schema, err := compileSchemas(ctx)
	if err != nil {
		return Project{}, err
	}
	project := schema["project"].LookupPath(cue.ParsePath("#Project"))
	known := map[string]bool{}
	if it, ferr := project.Fields(cue.Optional(true)); ferr == nil {
		for it.Next() {
			known[it.Selector().Unquoted()] = true
		}
	}
	for _, f := range knownFiles {
		if key, _, _ := fileMapped(f); key != "" {
			known[key] = true
		}
	}
	for name := range known {
		p.known = append(p.known, name)
	}
	sort.Strings(p.known)

	byName := map[string][]Key{}
	for _, f := range ordered {
		st := FileStatus{Name: f, Path: f.Path(root)}
		data, rerr := os.ReadFile(st.Path)
		if rerr != nil {
			st.ReadErr = rerr
			p.files = append(p.files, st)
			continue
		}
		st.Value = ctx.CompileBytes(data, cue.Filename(st.Path))
		p.files = append(p.files, st)
		if st.Value.Err() != nil {
			continue
		}
		if key, pkg, def := fileMapped(f); key != "" {
			k := Key{Name: key, File: f, Path: st.Path, Value: st.Value, Known: true, Entries: entries(st.Value)}
			if verr := schema[pkg].LookupPath(cue.ParsePath(def)).Unify(st.Value).Validate(cue.Concrete(true)); verr != nil {
				k.Err = &KeyError{Path: st.Path, Key: key, Err: verr}
			}
			byName[key] = append(byName[key], k)
			continue
		}
		it, ferr := st.Value.Fields(cue.Optional(true))
		if ferr != nil {
			continue // a file whose top level is not a struct sets no key
		}
		for it.Next() {
			name := it.Selector().Unquoted()
			k := Key{Name: name, File: f, Path: st.Path, Value: it.Value(), Known: known[name], Entries: entries(it.Value())}
			switch {
			case !k.Known:
			case it.IsOptional():
				k.Err = &KeyError{Path: st.Path, Key: name, Err: fmt.Errorf("declared optional (%s?:), so it sets nothing", name)}
			default:
				one := ctx.CompileString("{}").FillPath(cue.MakePath(cue.Str(name)), k.Value)
				if verr := project.Unify(one).Validate(cue.Concrete(true)); verr != nil {
					k.Err = &KeyError{Path: st.Path, Key: name, Err: verr}
				}
			}
			byName[name] = append(byName[name], k)
		}
	}

	for name, ks := range byName {
		chosen := ks[0]
		if len(ks) > 1 {
			paths := make([]string, 0, len(ks))
			for _, k := range ks {
				paths = append(paths, k.Path)
				if k.File == keyHome(name) {
					chosen = k
				}
			}
			chosen.Err = &ConflictError{Key: name, Paths: paths}
		}
		p.keys = append(p.keys, chosen)
	}
	sort.Slice(p.keys, func(i, j int) bool { return p.keys[i].Name < p.keys[j].Name })
	return p, nil
}

// compileSchemas builds the embedded schema packages, keyed by package
// name, through cue/load over an in-memory module (schema/project imports
// schema/guard and schema/binding).
func compileSchemas(ctx *cue.Context) (map[string]cue.Value, error) {
	base := filepath.Join(string(filepath.Separator), "fathom-embedded")
	overlay := map[string]load.Source{
		filepath.Join(base, "cue.mod", "module.cue"): load.FromString("module: \"github.com/geoffmilleraz/fathom\"\nlanguage: version: \"v0.16.1\"\n"),
	}
	pkgs := []string{"project", "guard", "binding", "techpack", "risk", "learning", "readouts"}
	for _, name := range pkgs {
		data, err := schemaFS.ReadFile("schemas/" + name + "/" + name + ".cue")
		if err != nil {
			return nil, fmt.Errorf("projectdir: embedded schema %s: %w", name, err)
		}
		overlay[filepath.Join(base, "schema", name, "schema.cue")] = load.FromBytes(data)
	}
	insts := load.Instances([]string{"./schema/project", "./schema/techpack", "./schema/risk", "./schema/learning"}, &load.Config{Dir: base, ModuleRoot: base, Overlay: overlay})
	out := map[string]cue.Value{}
	for _, inst := range insts {
		if inst.Err != nil {
			return nil, fmt.Errorf("projectdir: load embedded schema: %w", inst.Err)
		}
		v := ctx.BuildInstance(inst)
		if v.Err() != nil {
			return nil, fmt.Errorf("projectdir: build embedded schema %s: %w", inst.PkgName, v.Err())
		}
		out[inst.PkgName] = v
	}
	return out, nil
}

// entries is v's size: list elements, struct fields, else 1.
func entries(v cue.Value) int {
	n := 0
	switch v.IncompleteKind() {
	case cue.ListKind:
		if it, err := v.List(); err == nil {
			for it.Next() {
				n++
			}
		}
		return n
	case cue.StructKind:
		if it, err := v.Fields(); err == nil {
			for it.Next() {
				n++
			}
		}
		return n
	}
	return 1
}

// Files is every .fathom/*.cue file read, plus each file a reader reads by
// name that is absent, sorted by name.
func (p Project) Files() []FileStatus { return append([]FileStatus(nil), p.files...) }

// File is f's status; a name Load did not read is absent.
func (p Project) File(f File) FileStatus {
	for _, st := range p.files {
		if st.Name == f {
			return st
		}
	}
	return FileStatus{Name: f, Path: f.Path(p.root), ReadErr: fs.ErrNotExist}
}

// Keys is every top-level key, sorted by name; a key with Known false is
// one #Project does not define.
func (p Project) Keys() []Key { return append([]Key(nil), p.keys...) }

// Key is the named key, when some file sets it.
func (p Project) Key(name string) (Key, bool) {
	for _, k := range p.keys {
		if k.Name == name {
			return k, true
		}
	}
	return Key{}, false
}

// Source is the file a reader decodes key from: the file that sets it, or
// home when no file does (so a scaffold, an absent file and a syntax error
// read exactly as they did when the reader read home itself). Nothing is
// parsed again.
func (p Project) Source(key string, home File) FileStatus {
	if k, ok := p.Key(key); ok {
		return p.File(k.File)
	}
	return p.File(home)
}
