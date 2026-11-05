// Command fathom-lang prints what fathom's language library reads from files,
// as JSON, for tools not written in Go (ADR-092). It uses the standard flag
// package only, so the binary's closure is the library's and nothing else.
//
//	fathom-lang extract [-root DIR] [-siblings FILE|-] [-walk] [-cache DIR] PATH...
//	fathom-lang dialects
//
// The JSON is compatibility surface: fields are added, never renamed.
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
  fathom-lang extract [-root DIR] [-siblings FILE|-] [-walk] [-cache DIR] PATH...
  fathom-lang dialects

extract reads each PATH (relative to -root, default the working directory)
and prints {"files":[{"path":...,"result":...}]} as JSON.
dialects prints every dialect with its measured level as JSON.`

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
		return 1
	}
	return 0
}

// fileResult is one file's entry in the extract output.
type fileResult struct {
	Path   string      `json:"path"`
	Result lang.Result `json:"result"`
}

// cmdExtract reads each named file and prints its result as JSON.
func cmdExtract(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	root := fs.String("root", ".", "directory the PATH arguments are relative to")
	siblings := fs.String("siblings", "", "file listing sibling paths, one per line ('-' reads standard input)")
	walk := fs.Bool("walk", false, "use every file under -root as the sibling list")
	cache := fs.String("cache", "", "directory for wazero's compilation cache (off by default)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("extract needs at least one PATH")
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
	r, err := lang.New()
	if err != nil {
		return err
	}
	defer r.Close()
	res := struct {
		Files []fileResult `json:"files"`
	}{Files: []fileResult{}}
	for _, p := range fs.Args() {
		rel := filepath.ToSlash(filepath.Clean(p))
		src, err := os.ReadFile(filepath.Join(*root, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		got, err := r.Extract(ctx, lang.File{Path: rel, Src: src, Siblings: sibs})
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		res.Files = append(res.Files, fileResult{Path: rel, Result: got})
	}
	return writeJSON(out, res)
}

// cmdDialects prints every dialect with its measured level.
func cmdDialects(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("dialects", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	r, err := lang.New()
	if err != nil {
		return err
	}
	defer r.Close()
	return writeJSON(out, struct {
		Dialects []lang.DialectInfo `json:"dialects"`
	}{r.Dialects()})
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
