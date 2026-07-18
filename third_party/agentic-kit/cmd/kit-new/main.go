// Command kit-new scaffolds a new consumer module from the agent-cli blueprint
// with governance ON by default.
//
// Usage:
//
//	kit-new -name myagent -dir ../myagent [-module github.com/me/myagent] [-minimal] [-kit-version v0.2.0]
//
// Flags:
//
//	-name        agent/binary name (required; used as module path suffix and binary name)
//	-dir         output directory (required; must not already contain a go.mod)
//	-module      Go module path (default: github.com/<name>/<name>)
//	-minimal     generate bare client + harness Run, no governance stack
//	-kit-version agentic-kit version to require in go.mod (default: v0.0.0)
//	-kit-path    replace path for agentic-kit in go.mod (default: ../agentic-kit)
//
// The scaffold writes:
//
//	go.mod         — require + replace for agentic-kit
//	go.work        — workspace file for local sibling development
//	main.go        — governed (default) or minimal (-minimal) agent
//	.gitignore     — ignores workspace/ and *.jsonl
//	README.md      — next steps and what was generated
//	workspace/     — runtime dir (journal, etc.)
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
	)
	if err := fs.Parse(args); err != nil {
		return 2
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
