package main

// resources.go -- fathom-lang resources: the resources and attributes an infrastructure-as-code file declares
// (pkg/lang/iac, LL-35). It reads in the supervised child extract uses (supervise.go), so a parser that never
// returns, a fatal stack overflow or a read that allocates without bound costs that one file.

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

// resourceResult is an entry of resources' output. Result is an iac.ResourceSet, with its field names.
type resourceResult = entry[*iac.ResourceSet]

// moduleItem marks an item that names a Terraform directory to read as a module, in the argument list and on the
// child's input. A file name cannot hold a NUL, so no path is mistaken for it.
const moduleItem = "\x00module\x00"

// resourcesConfig is what builds a resource read, in this process or the child.
type resourcesConfig struct {
	deadline time.Duration
	values   bool
}

func (c resourcesConfig) options() []iac.Option {
	opts := []iac.Option{iac.WithDeadline(c.deadline)}
	if c.values {
		opts = append(opts, iac.WithValues())
	}
	return opts
}

// cmdResources reads each PATH, and the -module directory, and prints the resources as JSON.
func cmdResources(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("resources", flag.ContinueOnError)
	root := fs.String("root", ".", "directory the PATH arguments and -module are relative to")
	module := fs.String("module", "", "a Terraform directory to read as a module, before the PATH arguments")
	values := fs.Bool("values", false, "print attribute values (a value the source marks secret is never printed)")
	deadline := fs.Duration("deadline", iac.DefaultReadDeadline, "the time one read may take; it may only be lowered")
	maxMemory := fs.String("max-memory", defaultMaxMemory, "the memory the supervised child may hold, such as 512MiB or 4GiB")
	inProcess := fs.Bool("in-process", false, "read in this process, with no supervised child")
	child := fs.Bool("supervised-child", false, "run as the supervised child: read the batch from standard input (internal)")
	if err := fs.Parse(args); err != nil {
		return parseError(err)
	}
	if *deadline <= 0 || *deadline > iac.DefaultReadDeadline {
		return usageError{fmt.Errorf("-deadline %v must be above 0 and at most %v", *deadline, iac.DefaultReadDeadline)}
	}
	memory, err := parseSize(*maxMemory)
	if err != nil {
		return usageError{err}
	}
	cfg := resourcesConfig{deadline: *deadline, values: *values}
	if *child {
		return runResourcesChild(ctx, cfg, *root, memory, in, out)
	}
	items := fs.Args()
	if *module != "" {
		items = append([]string{moduleItem + *module}, items...)
	}
	if len(items) == 0 {
		return usageError{errors.New("resources needs at least one PATH or a -module DIR")}
	}
	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	var files []resourceResult
	var unreadable bool
	if *inProcess {
		files, unreadable, err = resourcesInProcess(ctx, cfg, rootAbs, items)
	} else {
		files, unreadable, err = resourcesSupervised(ctx, cfg, rootAbs, memory, items)
	}
	if err != nil {
		return err
	}
	res := struct {
		SchemaVersion int              `json:"schema_version"`
		Files         []resourceResult `json:"files"`
	}{SchemaVersion: schemaVersion, Files: files}
	if err := writeJSON(out, res); err != nil {
		return err
	}
	if unreadable {
		return errUnreadable
	}
	return nil
}

func resourcesInProcess(ctx context.Context, cfg resourcesConfig, rootAbs string, items []string) ([]resourceResult, bool, error) {
	files := []resourceResult{}
	unreadable := false
	for _, it := range items {
		fr, err := readResources(ctx, cfg, rootAbs, it)
		if err != nil {
			return nil, false, err
		}
		unreadable = unreadable || fr.Error != ""
		files = append(files, fr)
	}
	return files, unreadable, nil
}

// readResources reads one item: a file, or a module directory. A read that fails is a result of its own; the error
// returned is only for a cancelled run.
func readResources(ctx context.Context, cfg resourcesConfig, rootAbs, item string) (resourceResult, error) {
	var set *iac.ResourceSet
	var err error
	var shown string
	if dir, ok := strings.CutPrefix(item, moduleItem); ok {
		var fsys moduleFS
		fsys, shown = resolveModule(rootAbs, dir)
		if beforeRead != nil {
			beforeRead(shown)
		}
		set, err = iac.ReadModule(ctx, fsys.fs, fsys.dir, cfg.options()...)
	} else {
		var readPath, classified string
		readPath, shown, classified = resolvePath(rootAbs, item)
		if beforeRead != nil {
			beforeRead(shown)
		}
		var src []byte
		src, err = os.ReadFile(readPath)
		if err != nil {
			return resourceResult{Path: shown, Error: err.Error()}, nil
		}
		set, err = iac.ReadFile(ctx, classified, src, cfg.options()...)
	}
	if err != nil {
		if ctx.Err() != nil {
			return resourceResult{}, fmt.Errorf("%s: %w", shown, err) // the run was cancelled: stop
		}
		return resourceResult{Path: shown, Error: err.Error()}, nil
	}
	return resourceResult{Path: shown, Result: set}, nil
}

// moduleFS is a directory to read as a module: the file system and the directory in it.
type moduleFS struct {
	fs  iofs.FS
	dir string
}

// resolveModule maps a -module argument to the file system and directory to read, and the path the output shows. A
// relative directory is read under root, so its spans name files by their path under root. An absolute directory
// under root is read the same way, and one outside it is read as its own root, so its spans name files by their
// name alone.
func resolveModule(rootAbs, dir string) (moduleFS, string) {
	clean := filepath.Clean(dir)
	if filepath.IsAbs(clean) {
		rel, err := filepath.Rel(rootAbs, clean)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return moduleFS{fs: os.DirFS(clean), dir: "."}, filepath.ToSlash(clean)
		}
		clean = rel
	}
	rel := filepath.ToSlash(clean)
	return moduleFS{fs: os.DirFS(rootAbs), dir: rel}, rel
}

// resourcesAbandoned reports whether the child answered by abandoning a read, which keeps running in it.
func resourcesAbandoned(fr resourceResult) bool {
	return strings.HasPrefix(fr.Error, "read-deadline:") || strings.HasPrefix(fr.Error, "read-busy:")
}

// killedResources is the entry of an item the child could not finish. A resource read has no Incomplete result to
// carry the reason in, so the entry has an error, and the run exits 1.
func killedResources(path, why string) resourceResult {
	return resourceResult{Path: path, Error: killedNotePrefix + why + ", so none of its facts are returned"}
}

func resourcesTask(cfg resourcesConfig) task[*iac.ResourceSet] {
	return task[*iac.ResourceSet]{
		childArgs: func(root string, memory uint64) []string {
			args := []string{"resources", "-supervised-child", "-root", root, "-deadline", cfg.deadline.String(), "-max-memory", strconv.FormatUint(memory, 10)}
			if cfg.values {
				args = append(args, "-values")
			}
			return args
		},
		killed:    killedResources,
		abandoned: resourcesAbandoned,
		shown: func(root, item string) string {
			if dir, ok := strings.CutPrefix(item, moduleItem); ok {
				_, shown := resolveModule(root, dir)
				return shown
			}
			_, shown, _ := resolvePath(root, item)
			return shown
		},
	}
}

func resourcesSupervised(ctx context.Context, cfg resourcesConfig, rootAbs string, memory uint64, items []string) ([]resourceResult, bool, error) {
	return superviseAll(ctx, &supervisor[*iac.ResourceSet]{deadline: cfg.deadline, root: rootAbs, memory: memory, task: resourcesTask(cfg)}, items)
}

// runResourcesChild is resources' child half: answer one item per line until standard input ends.
func runResourcesChild(ctx context.Context, cfg resourcesConfig, root string, memory uint64, in io.Reader, out io.Writer) error {
	return serveChild(ctx, memory, in, out, func(childHeader) (func(context.Context, string) (resourceResult, error), func(), error) {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			return nil, nil, err
		}
		read := func(ctx context.Context, item string) (resourceResult, error) {
			return readResources(ctx, cfg, rootAbs, item)
		}
		return read, func() {}, nil
	})
}
