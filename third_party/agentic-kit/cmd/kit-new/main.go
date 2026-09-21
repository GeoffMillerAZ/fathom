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
	"embed"
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

// generate writes the scaffolded module to sc.dir.
func generate(sc scaffold) error {
	// Guard: refuse to overwrite an existing module.
	goModPath := filepath.Join(sc.dir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		return fmt.Errorf("go.mod already exists at %s; remove it or choose a different -dir", goModPath)
	}

	if err := os.MkdirAll(sc.dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// workspace/ must exist so the agent can open its journal on first run.
	if err := os.MkdirAll(filepath.Join(sc.dir, "workspace"), 0o755); err != nil {
		return fmt.Errorf("create workspace dir: %w", err)
	}

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

	// Render go.mod
	if err := renderTemplate("templates/go.mod.tmpl", filepath.Join(sc.dir, "go.mod"), data); err != nil {
		return err
	}

	// Render go.work
	if err := renderTemplate("templates/go.work.tmpl", filepath.Join(sc.dir, "go.work"), data); err != nil {
		return err
	}

	// Render main.go
	mainTmpl := "templates/main_governed.go.tmpl"
	if sc.minimal {
		mainTmpl = "templates/main_minimal.go.tmpl"
	}
	if err := renderTemplate(mainTmpl, filepath.Join(sc.dir, "main.go"), data); err != nil {
		return err
	}

	// Render .gitignore
	if err := renderTemplate("templates/gitignore.tmpl", filepath.Join(sc.dir, ".gitignore"), data); err != nil {
		return err
	}

	// Render README.md
	if err := renderTemplate("templates/README.md.tmpl", filepath.Join(sc.dir, "README.md"), data); err != nil {
		return err
	}

	return nil
}

// renderTemplate renders the named embedded template to the given output path.
func renderTemplate(name, outPath string, data templateData) error {
	src, err := templateFS.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read template %s: %w", name, err)
	}

	tmpl, err := template.New(filepath.Base(name)).Parse(string(src))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", name, err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}
	return nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
