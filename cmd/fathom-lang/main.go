// Command fathom-lang prints what fathom's language library reads from files,
// as JSON, for tools not written in Go (ADR-092). It uses the standard flag
// package only, so the binary's closure is the library's and nothing else.
//
//	fathom-lang extract [-root DIR] [-siblings FILE|-] [-walk] [-cache DIR] [-pack DIR] PATH...
//	fathom-lang dialects [-pack DIR]
//
// PATH may be relative to -root or absolute. A file that cannot be read is a
// result of its own (its error is in the JSON) and the batch goes on.
//
// The JSON is compatibility surface: fields are added, never renamed, and the
// top-level schema_version says which version of the shape this is. Exit 0 when
// every file was read, 1 when a file could not be read or the run stopped on an
// error, 2 on misuse (a bad flag, no PATH, an unknown command).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

const usage = `usage:
  fathom-lang extract [-root DIR] [-siblings FILE|-] [-walk] [-cache DIR] [-pack DIR] PATH...
  fathom-lang dialects [-pack DIR]

extract reads each PATH (absolute, or relative to -root, default the working
directory) and prints {"schema_version":1,"files":[{"path":...,"error":...,
"result":...}]} as JSON. A file that cannot be read has an error and a null
result, and the other files are still read.
dialects prints {"schema_version":1,"dialects":[...]} with every dialect and
its measured level.
Exit 0 on success, 1 when a file could not be read or the run failed, 2 on misuse.
-pack DIR adds the dialect pack in DIR (dialects/<id>/dialect.json and the
queries and doc specs it names). A pack that breaks a rule is refused whole.`

// schemaVersion is the version of the JSON shape both commands print. It
// changes only with a break; added fields do not change it.
const schemaVersion = 1

// usageError is misuse of the command line: a bad flag, a missing PATH.
type usageError struct{ error }

// errUnreadable is the error of a run whose JSON was printed but in which a
// file could not be read.
var errUnreadable = errors.New("one or more files could not be read; see their error fields")

// maxWalk bounds -walk: the most files one sibling listing may hold.
const maxWalk = 200000

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, in io.Reader, out, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "extract":
		err = cmdExtract(ctx, args[1:], in, out)
	case "dialects":
		err = cmdDialects(args[1:], out)
	case "-h", "-help", "--help", "help":
		fmt.Fprintln(out, usage)
		return 0
	default:
		fmt.Fprintf(errw, "fathom-lang: unknown command %q\n%s\n", args[0], usage)
		return 2
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(errw, "fathom-lang:", err)
		var ue usageError
		if errors.As(err, &ue) {
			return 2
		}
		return 1
	}
	return 0
}

// fileResult is one file's entry in the extract output. Error is "" for a file
// that was read, and Result is null for one that was not.
type fileResult struct {
	Path   string       `json:"path"`
	Error  string       `json:"error"`
	Result *lang.Result `json:"result"`
}

// cmdExtract reads each named file and prints its result as JSON.
func cmdExtract(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	root := fs.String("root", ".", "directory the PATH arguments are relative to")
	siblings := fs.String("siblings", "", "file listing sibling paths, one per line ('-' reads standard input)")
	walk := fs.Bool("walk", false, "use every file under -root as the sibling list")
	cache := fs.String("cache", "", "directory for wazero's compilation cache (off by default)")
	pack := fs.String("pack", "", "directory of a dialect pack to add")
	if err := fs.Parse(args); err != nil {
		return parseError(err)
	}
	if fs.NArg() == 0 {
		return usageError{errors.New("extract needs at least one PATH")}
	}
	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	if *cache != "" {
		if err := lang.EnableCompilationCache(*cache); err != nil {
			return err
		}
	}
	var sibs []string
	switch {
	case *siblings != "":
		list, err := readList(*siblings, in)
		if err != nil {
			return err
		}
		sibs = list
	case *walk:
		list, err := walkRoot(*root)
		if err != nil {
			return err
		}
		sibs = list
	}
	r, err := lang.New(packOptions(*pack)...)
	if err != nil {
		return err
	}
	defer r.Close()
	res := struct {
		SchemaVersion int          `json:"schema_version"`
		Files         []fileResult `json:"files"`
	}{SchemaVersion: schemaVersion, Files: []fileResult{}}
	unreadable := false
	for _, p := range fs.Args() {
		readPath, shown, classified := resolvePath(rootAbs, p)
		src, err := os.ReadFile(readPath)
		if err != nil {
			res.Files = append(res.Files, fileResult{Path: shown, Error: err.Error()})
			unreadable = true
			continue
		}
		got, err := r.Extract(ctx, lang.File{Path: classified, Src: src, Siblings: sibs})
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("%s: %w", shown, err) // the run was cancelled: stop
			}
			res.Files = append(res.Files, fileResult{Path: shown, Error: err.Error()})
			unreadable = true
			continue
		}
		res.Files = append(res.Files, fileResult{Path: shown, Result: &got})
	}
	if err := writeJSON(out, res); err != nil {
		return err
	}
	if unreadable {
		return errUnreadable
	}
	return nil
}

// parseError turns a flag parsing failure into misuse; -h stays a help request.
func parseError(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	return usageError{err}
}

// resolvePath maps a PATH argument to the file to read, the path the output
// shows and the path the library classifies by. A relative PATH is read under
// root and is its own classified path. An absolute PATH is read as given; the
// library sees it relative to root when it lies under root, and otherwise by
// its slash form, so a directory name such as .github/workflows still
// classifies.
func resolvePath(rootAbs, p string) (readPath, shown, classified string) {
	if !filepath.IsAbs(p) {
		rel := filepath.ToSlash(filepath.Clean(p))
		return filepath.Join(rootAbs, filepath.FromSlash(rel)), rel, rel
	}
	clean := filepath.Clean(p)
	if rel, err := filepath.Rel(rootAbs, clean); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return clean, filepath.ToSlash(rel), filepath.ToSlash(rel)
	}
	slash := filepath.ToSlash(clean)
	return clean, slash, slash
}

// cmdDialects prints every dialect with its measured level.
func cmdDialects(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("dialects", flag.ContinueOnError)
	pack := fs.String("pack", "", "directory of a dialect pack to add")
	if err := fs.Parse(args); err != nil {
		return parseError(err)
	}
	if fs.NArg() != 0 {
		return usageError{fmt.Errorf("dialects takes no PATH, got %q", fs.Arg(0))}
	}
	r, err := lang.New(packOptions(*pack)...)
	if err != nil {
		return err
	}
	defer r.Close()
	return writeJSON(out, struct {
		SchemaVersion int                `json:"schema_version"`
		Dialects      []lang.DialectInfo `json:"dialects"`
	}{schemaVersion, r.Dialects()})
}

// packOptions is the option that adds the pack in dir, none for "".
func packOptions(dir string) []lang.Option {
	if dir == "" {
		return nil
	}
	return []lang.Option{lang.WithDialects(os.DirFS(dir))}
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// readList reads a path list from a file or, for "-", standard input: one
// path per line, blank lines and surrounding space dropped, sorted.
func readList(name string, in io.Reader) ([]string, error) {
	var b []byte
	var err error
	if name == "-" {
		b, err = io.ReadAll(in)
	} else {
		b, err = os.ReadFile(name)
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, filepath.ToSlash(l))
		}
	}
	sort.Strings(out)
	return out, nil
}

// walkRoot lists every regular file under root, relative and slash-separated,
// skipping .git, in sorted order, to at most maxWalk files.
func walkRoot(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if len(out) >= maxWalk {
			return fmt.Errorf("-walk: more than %d files under %s; give -siblings instead", maxWalk, root)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out, err
}
