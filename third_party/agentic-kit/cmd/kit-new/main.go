// Command kit-new scaffolds a new consumer module from the agent-cli blueprint
// with governance ON by default, or — with -package — a new kit package in this
// repository's six-file shape together with the ledger lines it owes.
//
// Usage:
//
//	kit-new -name myagent -dir ../myagent [-module github.com/me/myagent] [-minimal] [-kit-version v0.2.0]
//	kit-new -package foundations/<category>/<name> [-practice N] [-interface Name] [-consumer <who>] [-apply]
//
// Consumer-module flags:
//
//	-name        agent/binary name (required; used as module path suffix and binary name)
//	-dir         output directory (required; must not already contain a go.mod)
//	-module      Go module path (default: github.com/<name>/<name>)
//	-minimal     generate bare client + harness Run, no governance stack
//	-kit-version agentic-kit version to require in go.mod (default: v0.0.0)
//	-kit-path    replace path for agentic-kit in go.mod (default: ../agentic-kit)
//
// The consumer scaffold writes:
//
//	go.mod         — require + replace for agentic-kit
//	go.work        — workspace file for local sibling development
//	main.go        — governed (default) or minimal (-minimal) agent
//	.gitignore     — ignores workspace/ and *.jsonl
//	README.md      — next steps and what was generated
//	workspace/     — runtime dir (journal, etc.)
//
// Package-mode flags (all exclusive with the consumer-module flags):
//
//	-package          <root>/<category>/<name> for foundations, <root>/<name> for
//	                  kits, runtimes, surfaces, toolchains; kebab-case dirs
//	-repo             kit repository root holding go.mod (default: .)
//	-practice N       docs/practices.md section the package carries (0 = none)
//	-interface Name   declare an exported interface in types.go and ship mock.go
//	-consumer <who>   the consumer or future workload that pulls for it (junk gate)
//	-apply            append the ledger lines instead of only printing them
//	-verified-against text for doc.go's Verified-against line (default: from go.mod + cue.mod)
//
// The package scaffold writes, under <repo>/<package>:
//
//	schema.cue            — #Config stub (CUE owns the shape)
//	types.go              — Config mirror + Validate (+ the -interface)
//	doc.go                — package doc: shape contract, concurrency contract,
//	                        non-guarantees, Verified-against line
//	<name>.go             — New with an injected `now func() time.Time` seam
//	<name>_test.go        — table-driven tests with a fixed clock
//	testdata/valid.cue    — positive fixture (`cue vet ./testdata/`)
//	testdata/invalid.cue  — negative fixtures, commented, mirrored by the Go test
//	mock.go               — only with -interface
//
// and prints the exact TRACKING.md row, docs/compat.md row (experimental),
// CHANGELOG.md line, docs/practices.md row (with -practice) and README.md
// package-count bump the maintainer must add; -apply appends them (each
// once — a line already present is skipped). It refuses to overwrite any
// existing scaffold file and, in -apply mode, refuses before writing
// anything when a ledger anchor is missing.
//
// Generated files contain no timestamps — output is deterministic.
//
// Exit codes: 0 = success; 1 = generation error; 2 = usage error.
package main

import (
	"bytes"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

// templateData is the value passed to every template.
type templateData struct {
	// Name is the agent/binary name.
	Name string

	// Module is the Go module path.
	Module string

	// KitVersion is the agentic-kit version string to require (e.g. "v0.0.0").
	KitVersion string

	// KitVersionClean is KitVersion without the leading "v" for prose.
	KitVersionClean string

	// KitReplacePath is the replace directive target path.
	KitReplacePath string

	// APIKeyEnv is the environment variable for the Anthropic API key.
	APIKeyEnv string

	// Minimal is true when -minimal was given.
	Minimal bool

	// BudgetCap is the USD/day cap string, for the README.
	BudgetCap string
}

// scaffold holds the resolved parameters for a generation run.
type scaffold struct {
	name       string
	dir        string
	module     string
	kitVersion string
	kitPath    string
	minimal    bool
}

// run is the testable entry point: flags in, exit code out.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "host" {
		return runHost(args[1:], os.Stdin, stdout, stderr, hostDeps{})
	}
	fs := flag.NewFlagSet("kit-new", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		name       = fs.String("name", "", "agent/binary name (required)")
		dir        = fs.String("dir", "", "output directory (required)")
		module     = fs.String("module", "", "Go module path (default: github.com/<name>/<name>)")
		minimal    = fs.Bool("minimal", false, "generate bare client + harness Run, no governance")
		kitVersion = fs.String("kit-version", "v0.0.0", "agentic-kit version to require in go.mod")
		kitPath    = fs.String("kit-path", "../agentic-kit", "replace path for agentic-kit in go.mod (and go.work)")

		pkg       = fs.String("package", "", "package mode: <root>/<category>/<name> to scaffold in the kit repo (e.g. foundations/telemetry/read-manifest)")
		repo      = fs.String("repo", ".", "package mode: kit repository root (must hold go.mod)")
		practice  = fs.Int("practice", 0, "package mode: docs/practices.md section number the package carries (0 = none)")
		iface     = fs.String("interface", "", "package mode: exported interface name to declare in types.go and mock in mock.go")
		consumer  = fs.String("consumer", "", "package mode: the consumer or future workload that pulls for the package (junk gate)")
		apply     = fs.Bool("apply", false, "package mode: append the ledger lines to TRACKING.md, docs/compat.md, CHANGELOG.md, docs/practices.md and bump the README count")
		verifiedA = fs.String("verified-against", "", "package mode: text for doc.go's Verified-against line (default: derived from go.mod and cue.mod)")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// ---- package mode ---------------------------------------------------------
	if *pkg != "" {
		if *name != "" || *dir != "" || *module != "" || *minimal {
			fmt.Fprintf(stderr, "kit-new: %v\n", errPackageMode)
			fs.Usage()
			return 2
		}
		return runPackage(packageScaffold{
			repo:            *repo,
			pkgPath:         *pkg,
			practice:        *practice,
			iface:           *iface,
			consumer:        *consumer,
			apply:           *apply,
			verifiedAgainst: *verifiedA,
		}, stdout, stderr)
	}

	// ---- validate required flags --------------------------------------------
	if *name == "" {
		fmt.Fprintln(stderr, "kit-new: -name is required")
		fs.Usage()
		return 2
	}
	if *dir == "" {
		fmt.Fprintln(stderr, "kit-new: -dir is required")
		fs.Usage()
		return 2
	}
	if strings.ContainsAny(*name, " /\\") {
		fmt.Fprintf(stderr, "kit-new: -name %q must not contain spaces or slashes\n", *name)
		return 2
	}

	mod := *module
	if mod == "" {
		mod = "github.com/" + *name + "/" + *name
	}

	sc := scaffold{
		name:       *name,
		dir:        *dir,
		module:     mod,
		kitVersion: *kitVersion,
		kitPath:    *kitPath,
		minimal:    *minimal,
	}

	if err := generate(sc); err != nil {
		fmt.Fprintf(stderr, "kit-new: %v\n", err)
		return 1
	}

	mode := "governed"
	if sc.minimal {
		mode = "minimal"
	}
	fmt.Fprintf(stdout, "kit-new: generated %s module %q in %s\n", mode, sc.module, sc.dir)
	fmt.Fprintf(stdout, "kit-new: next: cd %s && go run . -mode mock\n", sc.dir)
	return 0
}

type scaffoldOutput struct {
	name    string
	content []byte
}

type ownedOutput struct {
	name     string
	info     os.FileInfo
	filePin  *os.File
	dirPin   *os.Root
	expected []byte
	written  int // -1 means the writer did not report a valid byte count.
}

type scaffoldHooks struct {
	afterCreate   func(string) error
	mkdirParent   func(string) error
	dirInfo       func(string) (os.FileInfo, error)
	openRoot      func(string) (*os.Root, error)
	workspaceInfo func(*os.Root) (os.FileInfo, error)
	fileInfo      func(*os.File) (os.FileInfo, error)
	openFilePin   func(*os.Root, string) (*os.File, error)
	openDirPin    func(*os.Root, string) (*os.Root, error)
	write         func(*os.File, []byte) (int, error)
	close         func(*os.File) error
	removeOutput  func(*os.Root, string) error
	removeDir     func(string) error
}

// generate writes the scaffolded module to sc.dir.
func generate(sc scaffold) error {
	return generateWithStep(sc, nil)
}

// generateWithStep permits deterministic failure tests after an output is created.
func generateWithStep(sc scaffold, afterCreate func(string) error) error {
	return generateWithHooks(sc, scaffoldHooks{afterCreate: afterCreate})
}

func generateWithHooks(sc scaffold, hooks scaffoldHooks) (retErr error) {
	ver := sc.kitVersion
	verClean := strings.TrimPrefix(ver, "v")

	data := templateData{
		Name:            sc.name,
		Module:          sc.module,
		KitVersion:      ver,
		KitVersionClean: verClean,
		KitReplacePath:  sc.kitPath,
		APIKeyEnv:       "ANTHROPIC_API_KEY",
		Minimal:         sc.minimal,
		BudgetCap:       "1.00",
	}

	mainTmpl := "templates/main_governed.go.tmpl"
	if sc.minimal {
		mainTmpl = "templates/main_minimal.go.tmpl"
	}
	templates := []struct{ name, source string }{
		{"go.mod", "templates/go.mod.tmpl"},
		{"go.work", "templates/go.work.tmpl"},
		{"main.go", mainTmpl},
		{".gitignore", "templates/gitignore.tmpl"},
		{"README.md", "templates/README.md.tmpl"},
	}
	outputs := make([]scaffoldOutput, 0, len(templates))
	for _, item := range templates {
		content, err := renderTemplate(item.source, data)
		if err != nil {
			return err
		}
		outputs = append(outputs, scaffoldOutput{name: item.name, content: content})
	}

	// Check every intended path before creating the first output. Lstat also
	// catches dangling symlinks, which Stat would miss.
	for _, name := range []string{"go.mod", "go.work", "main.go", ".gitignore", "README.md", "workspace"} {
		path := filepath.Join(sc.dir, name)
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("%s already exists at %s; remove it or choose a different -dir", name, path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
	}

	dirBefore, dirErr := os.Lstat(sc.dir)
	if dirErr != nil && !errors.Is(dirErr, os.ErrNotExist) {
		return fmt.Errorf("inspect output dir: %w", dirErr)
	}
	if dirErr == nil && !dirBefore.IsDir() {
		return fmt.Errorf("output path is not a directory: %s", sc.dir)
	}
	createdDir := errors.Is(dirErr, os.ErrNotExist)
	if createdDir {
		mkdirParent := hooks.mkdirParent
		if mkdirParent == nil {
			mkdirParent = func(path string) error { return os.MkdirAll(path, 0o755) }
		}
		if err := mkdirParent(filepath.Dir(sc.dir)); err != nil {
			return fmt.Errorf("create output parent: %w; partial directories may remain under %s; intended destination %s", err, retainedPath(filepath.Dir(sc.dir), ""), retainedPath(sc.dir, ""))
		}
		if err := os.Mkdir(sc.dir, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w; partial parent directories may remain under %s; intended destination %s", err, retainedPath(filepath.Dir(sc.dir), ""), retainedPath(sc.dir, ""))
		}
	}
	dirInfoFn := hooks.dirInfo
	if dirInfoFn == nil {
		dirInfoFn = os.Lstat
	}
	dirInfo, err := dirInfoFn(sc.dir)
	if err != nil {
		if createdDir {
			return fmt.Errorf("inspect output dir: %w; identity unavailable, retained %s", err, retainedPath(sc.dir, ""))
		}
		return fmt.Errorf("inspect output dir: %w", err)
	}
	if !dirInfo.IsDir() || (!createdDir && !os.SameFile(dirBefore, dirInfo)) {
		return fmt.Errorf("output directory changed: %s", retainedPath(sc.dir, ""))
	}
	openRoot := hooks.openRoot
	if openRoot == nil {
		openRoot = os.OpenRoot
	}
	root, err := openRoot(sc.dir)
	if err != nil {
		if createdDir {
			return fmt.Errorf("open output dir: %w; retained %s", err, retainedPath(sc.dir, ""))
		}
		return fmt.Errorf("open output dir: %w", err)
	}
	defer root.Close()

	var owned []ownedOutput
	// Keep every pin live until cleanup is finished, including on success.
	defer func() {
		for _, entry := range owned {
			if entry.filePin != nil {
				_ = entry.filePin.Close()
			}
			if entry.dirPin != nil {
				_ = entry.dirPin.Close()
			}
		}
	}()
	defer func() {
		if retErr == nil {
			return
		}
		for i := len(owned) - 1; i >= 0; i-- {
			entry := owned[i]
			info, err := root.Lstat(entry.name)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				retErr = errors.Join(retErr, fmt.Errorf("cleanup identity unavailable, retained %s: %w", retainedPath(sc.dir, entry.name), err))
				continue
			}
			if !os.SameFile(info, entry.info) {
				retErr = errors.Join(retErr, fmt.Errorf("cleanup identity changed, retained %s", retainedPath(sc.dir, entry.name)))
				continue
			}
			if info.Mode() != entry.info.Mode() {
				retErr = errors.Join(retErr, fmt.Errorf("cleanup mode changed, retained %s", retainedPath(sc.dir, entry.name)))
				continue
			}
			if entry.dirPin != nil {
				pinned, pinErr := entry.dirPin.Stat(".")
				if pinErr != nil || !os.SameFile(pinned, info) {
					retErr = errors.Join(retErr, fmt.Errorf("cleanup pinned identity unavailable or changed, retained %s: %v", retainedPath(sc.dir, entry.name), pinErr))
					continue
				}
			} else if entry.filePin != nil {
				pinned, pinErr := entry.filePin.Stat()
				if pinErr != nil || !os.SameFile(pinned, info) || !info.Mode().IsRegular() || entry.written < 0 || info.Size() != int64(entry.written) {
					retErr = errors.Join(retErr, fmt.Errorf("cleanup file ownership unavailable or changed, retained %s: %v", retainedPath(sc.dir, entry.name), pinErr))
					continue
				}
				content, readErr := io.ReadAll(io.LimitReader(entry.filePin, int64(entry.written)+1))
				if readErr != nil || !bytes.Equal(content, entry.expected[:entry.written]) {
					retErr = errors.Join(retErr, fmt.Errorf("cleanup bytes unavailable or changed, retained %s: %v", retainedPath(sc.dir, entry.name), readErr))
					continue
				}
			} else {
				retErr = errors.Join(retErr, fmt.Errorf("cleanup pin unavailable, retained %s", retainedPath(sc.dir, entry.name)))
				continue
			}
			removeOutput := hooks.removeOutput
			if removeOutput == nil {
				removeOutput = func(root *os.Root, name string) error { return root.Remove(name) }
			}
			if err := removeOutput(root, entry.name); err != nil {
				retErr = errors.Join(retErr, fmt.Errorf("cleanup failed, retained %s: %w", retainedPath(sc.dir, entry.name), err))
			}
		}
		if createdDir {
			current, err := os.Lstat(sc.dir)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				retErr = errors.Join(retErr, fmt.Errorf("cleanup identity unavailable, retained %s: %w", retainedPath(sc.dir, ""), err))
			} else if err == nil && !os.SameFile(current, dirInfo) {
				retErr = errors.Join(retErr, fmt.Errorf("cleanup identity changed, retained %s", retainedPath(sc.dir, "")))
			} else if err == nil {
				removeDir := hooks.removeDir
				if removeDir == nil {
					removeDir = os.Remove
				}
				if err := removeDir(sc.dir); err != nil && !errors.Is(err, os.ErrNotExist) {
					retErr = errors.Join(retErr, fmt.Errorf("cleanup failed, retained %s: %w", retainedPath(sc.dir, ""), err))
				}
			}
		}
	}()

	for _, name := range []string{"go.mod", "go.work", "main.go", ".gitignore", "README.md", "workspace"} {
		if _, err := root.Lstat(name); err == nil {
			return fmt.Errorf("%s already exists at %s; remove it or choose a different -dir", name, filepath.Join(sc.dir, name))
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect %s: %w", name, err)
		}
	}

	if err := root.Mkdir("workspace", 0o755); err != nil {
		return fmt.Errorf("create workspace dir: %w", err)
	}
	workspaceInfoFn := hooks.workspaceInfo
	if workspaceInfoFn == nil {
		workspaceInfoFn = func(root *os.Root) (os.FileInfo, error) { return root.Lstat("workspace") }
	}
	workspaceInfo, err := workspaceInfoFn(root)
	if err != nil {
		return fmt.Errorf("stat created workspace: %w; identity unavailable, retained %s", err, retainedPath(sc.dir, "workspace"))
	}
	openDirPin := hooks.openDirPin
	if openDirPin == nil {
		openDirPin = func(root *os.Root, name string) (*os.Root, error) { return root.OpenRoot(name) }
	}
	workspacePin, err := openDirPin(root, "workspace")
	if err != nil {
		return fmt.Errorf("pin created workspace: %w; identity unavailable, retained %s", err, retainedPath(sc.dir, "workspace"))
	}
	pinnedWorkspaceInfo, err := workspacePin.Stat(".")
	if err != nil || !workspaceInfo.IsDir() || !os.SameFile(workspaceInfo, pinnedWorkspaceInfo) {
		_ = workspacePin.Close()
		return fmt.Errorf("pin created workspace: identity unavailable or changed: %v; retained %s", err, retainedPath(sc.dir, "workspace"))
	}
	owned = append(owned, ownedOutput{name: "workspace", info: workspaceInfo, dirPin: workspacePin})
	if hooks.afterCreate != nil {
		if err := hooks.afterCreate("workspace"); err != nil {
			return err
		}
	}
	for _, output := range outputs {
		f, err := root.OpenFile(output.name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return fmt.Errorf("create %s: %w", output.name, err)
		}
		fileInfoFn := hooks.fileInfo
		if fileInfoFn == nil {
			fileInfoFn = func(f *os.File) (os.FileInfo, error) { return f.Stat() }
		}
		info, statErr := fileInfoFn(f)
		if statErr != nil {
			_ = f.Close()
			return fmt.Errorf("stat created %s: %w; identity unavailable, retained %s", output.name, statErr, retainedPath(sc.dir, output.name))
		}
		openFilePin := hooks.openFilePin
		if openFilePin == nil {
			openFilePin = func(root *os.Root, name string) (*os.File, error) { return root.Open(name) }
		}
		pin, pinErr := openFilePin(root, output.name)
		if pinErr != nil {
			_ = f.Close()
			return fmt.Errorf("pin created %s: %w; identity unavailable, retained %s", output.name, pinErr, retainedPath(sc.dir, output.name))
		}
		pinInfo, pinErr := pin.Stat()
		if pinErr != nil || !info.Mode().IsRegular() || !os.SameFile(info, pinInfo) {
			_ = pin.Close()
			_ = f.Close()
			return fmt.Errorf("pin created %s: identity unavailable or changed: %v; retained %s", output.name, pinErr, retainedPath(sc.dir, output.name))
		}
		owned = append(owned, ownedOutput{name: output.name, info: info, filePin: pin, expected: output.content, written: -1})
		writeFn := hooks.write
		if writeFn == nil {
			writeFn = func(f *os.File, content []byte) (int, error) { return f.Write(content) }
		}
		n, writeErr := writeFn(f, output.content)
		if n >= 0 && n <= len(output.content) {
			owned[len(owned)-1].written = n
		} else {
			writeErr = errors.Join(writeErr, fmt.Errorf("invalid write count %d for %s", n, output.name))
		}
		if writeErr == nil && n < len(output.content) {
			writeErr = io.ErrShortWrite
		}
		closeFn := hooks.close
		if closeFn == nil {
			closeFn = func(f *os.File) error { return f.Close() }
		}
		closeErr := closeFn(f)
		if writeErr != nil {
			return fmt.Errorf("write %s: %w", output.name, writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s: %w", output.name, closeErr)
		}
		if hooks.afterCreate != nil {
			if err := hooks.afterCreate(output.name); err != nil {
				return err
			}
		}
	}
	return nil
}

func retainedPath(dir, name string) string {
	path := filepath.Join(dir, name)
	if absolute, err := filepath.Abs(path); err == nil {
		return absolute
	}
	return path
}

// renderTemplate renders an embedded template in memory before publication.
func renderTemplate(name string, data templateData) ([]byte, error) {
	src, err := templateFS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", name, err)
	}

	tmpl, err := template.New(filepath.Base(name)).Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	return rendered.Bytes(), nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
