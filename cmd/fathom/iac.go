package main

// iac.go — `fathom iac`: catalog management for IaC facts (ADR-091, docs/design/iac-facts.md section 4).
//
// A catalog is a local file the operator placed under $FATHOM_HOME/iac. The verb has no download path (I56): `add`
// takes a local path and refuses a URL, and `import` builds a catalog from an export the operator ran themselves.
// Nothing here runs a provider, `init`, a plan or a registry client.

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// iacCatalogDir is where installed catalogs live: $FATHOM_HOME/iac. internal/iacfacts takes the directory in so its
// import closure stays free of fathomhome (lang-library section 4). The product fills it only from a local file on an
// operator command and never from the network (I56).
func iacCatalogDir() string { return fathomhome.Path("iac") }

// cmdIaC dispatches `fathom iac add|import|list|remove`.
func cmdIaC(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: fathom iac add|import|list|remove …")
		return 1
	}
	switch args[0] {
	case "add":
		return cmdIaCAdd(args[1:], stdout, stderr)
	case "import":
		return cmdIaCImport(args[1:], stdout, stderr)
	case "list":
		return cmdIaCList(args[1:], stdout, stderr)
	case "remove":
		return cmdIaCRemove(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "unknown iac verb %q (add|import|list|remove)\n", args[0])
	return 1
}

// iacCatalogJSON is one catalog as `--json` prints it.
type iacCatalogJSON struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Bytes       int64    `json:"bytes"`
	SHA256      string   `json:"sha256,omitempty"`
	Kind        string   `json:"kind"`
	Source      string   `json:"source"`
	Line        string   `json:"line"`
	Versions    []string `json:"versions"`
	Tool        string   `json:"tool,omitempty"`
	Licence     string   `json:"licence,omitempty"`
	Replacement string   `json:"replacement"`
}

func iacJSON(c iacfacts.IaCCatalogInfo) iacCatalogJSON {
	m := c.Manifest
	return iacCatalogJSON{
		Name: c.Name, Path: c.Path, Bytes: c.Bytes, SHA256: c.SHA256, Kind: m.Kind, Source: m.Source, Line: m.Line,
		Versions: m.Versions, Tool: m.Tool, Licence: m.Licence, Replacement: m.Replacement,
	}
}

func printIaCInstalled(stdout io.Writer, verb string, c iacfacts.IaCCatalogInfo) {
	m := c.Manifest
	fmt.Fprintf(stdout, "%s %s\n", verb, c.Name)
	fmt.Fprintf(stdout, "  source       %s\n", m.Source)
	fmt.Fprintf(stdout, "  line         %s (%d releases, %s to %s)\n", m.Line, len(m.Versions), m.Versions[0], m.Versions[len(m.Versions)-1])
	fmt.Fprintf(stdout, "  replacement  %s\n", m.Replacement)
	fmt.Fprintf(stdout, "  sha256       %s\n", c.SHA256)
	fmt.Fprintf(stdout, "  path         %s\n", c.Path)
}

func cmdIaCAdd(args []string, stdout, stderr io.Writer) int {
	var lead string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		lead, args = args[0], args[1:]
	}
	fs := newFlagSet("iac add", stderr)
	jsonOut := fs.Bool("json", false, "emit the installed catalog as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if lead == "" && fs.NArg() == 1 {
		lead = fs.Arg(0)
	}
	if lead == "" || (fs.NArg() > 0 && lead != fs.Arg(0)) {
		fmt.Fprintln(stderr, "usage: fathom iac add <catalog file> [--json]   (a local path only: fetch the file with your own tools first)")
		return 1
	}
	info, err := iacfacts.AddIaCCatalog(iacCatalogDir(), lead)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if *jsonOut {
		return writeIaCJSON(stdout, stderr, iacJSON(info))
	}
	printIaCInstalled(stdout, "installed", info)
	fmt.Fprintln(stdout, "  check the sha256 against the publisher's attestation (gh attestation verify) before trusting the file")
	return 0
}

// iacImportUsage is the help text of `fathom iac import`.
const iacImportUsage = `usage: fathom iac import [--version V] [--licence SPDX] [--tool NAME] [--replace] [--json] <export.json | V=export.json>...

An export is the output of ` + "`tofu providers schema -json`" + ` for exactly ONE provider. A multi-provider export is refused.
Make one in a scratch directory whose configuration requires only that provider:
  mkdir scratch && cd scratch
  write a main.tf with one required_providers entry, then run: tofu init && tofu providers schema -json > export.json
All inputs must be one major line of that provider. --version (or the V= prefix) is what YOU assert the export was
made at: it is recorded as given and is not checked against the export.

A catalog of the same provider line that is already installed is merged, never dropped: every release it holds stays,
the exports you name are added, and the result lists what was added, kept, unchanged and replaced. A release you
name that the installed catalog holds with different content is refused unless you pass --replace.`

func cmdIaCImport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("iac import", stderr)
	version := fs.String("version", "", "the provider version of a single export file (or write each input as VERSION=export.json)")
	licence := fs.String("licence", "", "SPDX licence identifier to record in the manifest")
	tool := fs.String("tool", "", "the tool and version that produced the exports, recorded in the manifest")
	replace := fs.Bool("replace", false, "replace a release the installed catalog holds with different content (default: refuse)")
	jsonOut := fs.Bool("json", false, "emit the installed catalog as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	// Flags may follow the inputs: `iac import export.json --version 5.1.0`. Go's flag package stops at the first
	// positional, so each one is taken and the rest parsed again.
	var inputs []string
	for fs.NArg() > 0 {
		inputs = append(inputs, fs.Arg(0))
		if err := fs.Parse(fs.Args()[1:]); err != nil {
			return 1
		}
	}
	if len(inputs) == 0 {
		fmt.Fprintln(stderr, iacImportUsage)
		return 1
	}
	var in []iacfacts.IaCImportInput
	for _, a := range inputs {
		v, p := *version, a
		if i := strings.IndexByte(a, '='); i > 0 {
			if _, err := iacfacts.ParseIaCVersion(a[:i]); err == nil {
				v, p = a[:i], a[i+1:]
			}
		}
		if v == "" {
			fmt.Fprintf(stderr, "error: %s has no version: pass --version, or write the input as VERSION=%s\n", a, a)
			return 1
		}
		in = append(in, iacfacts.IaCImportInput{Version: v, Path: p})
	}
	res, err := iacfacts.ImportIaCExportsReport(iacCatalogDir(), in, iacfacts.IaCWriterOptions{Licence: *licence, Tool: *tool}, *replace)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if *jsonOut {
		return writeIaCJSON(stdout, stderr, iacImportJSON{
			iacCatalogJSON: iacJSON(res.Info), Merged: res.Merged, Added: res.Added, Kept: res.Kept, Unchanged: res.Unchanged, Replaced: res.Replaced,
		})
	}
	printIaCInstalled(stdout, "imported", res.Info)
	for i, s := range res.Stats {
		fmt.Fprintf(stdout, "  %-12s %d resources, %d data sources, %d ephemeral\n", in[i].Version, s.Resources, s.Data, s.Ephemeral)
	}
	if res.Merged {
		fmt.Fprintln(stdout, "  merged into the catalog already installed for this line")
	}
	for _, g := range []struct {
		label string
		vs    []string
	}{{"added", res.Added}, {"kept", res.Kept}, {"unchanged", res.Unchanged}, {"replaced", res.Replaced}} {
		if len(g.vs) > 0 {
			fmt.Fprintf(stdout, "  %-12s %s\n", g.label, strings.Join(g.vs, ", "))
		}
	}
	fmt.Fprintln(stdout, "  facts: attributes, required, sensitive, deprecated and upgrade impact; no replacement facts from an import")
	return 0
}

// iacImportJSON is `iac import --json`: the installed catalog and how the import changed the line.
type iacImportJSON struct {
	iacCatalogJSON
	Merged    bool     `json:"merged"`
	Added     []string `json:"added"`
	Kept      []string `json:"kept,omitempty"`
	Unchanged []string `json:"unchanged,omitempty"`
	Replaced  []string `json:"replaced,omitempty"`
}

func cmdIaCList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("iac list", stderr)
	jsonOut := fs.Bool("json", false, "emit the catalogs as JSON")
	digest := fs.Bool("digest", false, "also hash each file (reads every catalog in full)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	cats, bad := iacfacts.ListIaCCatalogs(iacCatalogDir(), *digest)
	if *jsonOut {
		out := struct {
			Catalogs []iacCatalogJSON  `json:"catalogs"`
			Unread   map[string]string `json:"unread,omitempty"`
		}{Catalogs: []iacCatalogJSON{}}
		for _, c := range cats {
			out.Catalogs = append(out.Catalogs, iacJSON(c))
		}
		if len(bad) > 0 {
			out.Unread = map[string]string{}
			for n, e := range bad {
				out.Unread[n] = e.Error()
			}
		}
		return writeIaCJSON(stdout, stderr, out)
	}
	if len(cats) == 0 && len(bad) == 0 {
		fmt.Fprintf(stdout, "no IaC catalogs in %s — `fathom iac add <file>` installs one, `fathom iac import <export>` builds one\n", iacCatalogDir())
		return 0
	}
	for _, c := range cats {
		m := c.Manifest
		fmt.Fprintf(stdout, "%s\n  %s line %s: %d releases %s to %s, %d bytes, replacement %s\n", c.Name, m.Source, m.Line, len(m.Versions), m.Versions[0], m.Versions[len(m.Versions)-1], c.Bytes, m.Replacement)
		if c.SHA256 != "" {
			fmt.Fprintf(stdout, "  sha256 %s\n", c.SHA256)
		}
	}
	names := make([]string, 0, len(bad))
	for n := range bad {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(stdout, "%s\n  unreadable: %v\n", n, bad[n])
	}
	return 0
}

func cmdIaCRemove(args []string, stdout, stderr io.Writer) int {
	var lead string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		lead, args = args[0], args[1:]
	}
	fs := newFlagSet("iac remove", stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if lead == "" && fs.NArg() == 1 {
		lead = fs.Arg(0)
	}
	if lead == "" || (fs.NArg() > 0 && lead != fs.Arg(0)) {
		fmt.Fprintln(stderr, "usage: fathom iac remove <catalog name>   (names: `fathom iac list`)")
		return 1
	}
	name, err := iacfacts.RemoveIaCCatalog(iacCatalogDir(), lead)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "removed %s\n", name)
	return 0
}

func writeIaCJSON(stdout, stderr io.Writer, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

type iacRunner struct{}

func (iacRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdIaC(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "iac", Group: "govern", Order: 160, Run: iacRunner{},
		Summary: "IaC provider catalogs: add | import | list | remove — local files only, never a download"})
}
