package mcp

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// IaC contract facts on two surfaces (docs/design/iac-facts.md section 6, rows IF-9 and IF-10):
// symbol_context on a Terraform resource or data source carries an `iac` block, and review_diff carries the
// attribute findings of the changed Terraform files.
//
// Catalogs are found only through the one directory `fathom iac` fills ($FATHOM_HOME/iac). Nothing is downloaded
// (I56). With no catalog installed neither surface adds a key: both answer exactly as they did before.

// iacCatalogs lists the installed catalogs. dir is empty and the list nil when there are none.
func iacCatalogs() (string, []iacfacts.IaCCatalogInfo) {
	dir := fathomhome.Path("iac")
	if dir == "" {
		return "", nil
	}
	list, _ := iacfacts.ListIaCCatalogs(dir, false)
	if len(list) == 0 {
		return "", nil
	}
	return dir, list
}

// corpusFS reads a service's corpus and refuses to leave it. A symbolic link whose target resolves outside the
// corpus root is not opened, as read_source refuses it, so a link in a module directory cannot serve a file of the
// daemon's disk.
type corpusFS struct{ root string }

func (c corpusFS) resolve(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	full := filepath.Join(c.root, filepath.FromSlash(name))
	real, err := resolveRealPath(c.root, full, name)
	if err != nil {
		if _, serr := os.Lstat(full); serr != nil {
			return "", &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
		return "", &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return real, nil
}

// Open resolves name inside the corpus. fs.ReadDir lists a directory through Open, so the same resolver refuses a link that leaves it.
func (c corpusFS) Open(name string) (fs.File, error) {
	real, err := c.resolve(name)
	if err != nil {
		return nil, err
	}
	return os.Open(real)
}

type iacRootJSON struct {
	Dir     string `json:"dir"`
	Version string `json:"version,omitempty"`
	Via     string `json:"via"`
}

type iacBindingJSON struct {
	Status      string        `json:"status"`
	Source      string        `json:"source,omitempty"`
	LocalName   string        `json:"local_name,omitempty"`
	Alias       string        `json:"alias,omitempty"`
	Version     string        `json:"version,omitempty"`
	Locked      string        `json:"locked,omitempty"`
	Constraint  string        `json:"constraint,omitempty"`
	Catalog     string        `json:"catalog,omitempty"`
	AddLine     string        `json:"add_line,omitempty"`
	Candidates  []string      `json:"candidates,omitempty"`
	Roots       []iacRootJSON `json:"roots,omitempty"`
	Refusal     string        `json:"refusal,omitempty"`
	Disclosures []string      `json:"disclosures,omitempty"`
}

func iacBinding(b iacfacts.IaCBinding) iacBindingJSON {
	out := iacBindingJSON{Status: b.Status, Source: b.Source, LocalName: b.LocalName, Alias: b.Alias, Version: b.Version, Locked: b.Locked,
		Constraint: b.Constraint, Catalog: b.Catalog, AddLine: b.AddLine, Candidates: b.Candidates, Disclosures: b.Disclosures}
	for _, r := range b.Roots {
		out.Roots = append(out.Roots, iacRootJSON{Dir: r.Dir, Version: r.Version, Via: r.Via})
	}
	if b.Refusal != nil {
		out.Refusal = b.Refusal.Error()
	}
	return out
}

// iacWrittenCap is the most written attributes symbol_context lists; written_total carries the count.
const iacWrittenCap = 100

// symbolContextIaCFacts is the `iac` block of symbol_context on a Terraform resource or data source: the binding
// status, the version, the required arguments the block leaves unset, and the attributes it wrote with their flags
// and findings. It is nil for any other symbol, when no catalog is installed, or when the block cannot be found.
func symbolContextIaCFacts(ctx context.Context, backend QueryInterface, service string, def core.SymbolNode) map[string]any {
	if def.Kind != core.KindTerraformResource && def.Kind != core.KindTerraformData {
		return nil
	}
	if def.FilePath == "" || def.Name == "" {
		return nil
	}
	dir, list := iacCatalogs()
	if len(list) == 0 {
		return nil
	}
	root, err := resolveCorpusRoot(ctx, backend, service)
	if err != nil {
		return nil
	}
	got, err := iacfacts.CheckIaCModule(iacfacts.IaCModuleQuery{
		Repo: corpusFS{root: root}, CatalogDir: dir, Catalogs: list,
		Dir: path.Dir(filepath.ToSlash(def.FilePath)), Blocks: map[string]bool{def.Name: true},
	})
	if err != nil {
		return map[string]any{"status": "unread", "reason": err.Error()}
	}
	for _, b := range got.Blocks {
		if b.Block != def.Name {
			continue
		}
		out := map[string]any{"binding": iacBinding(b.Binding)}
		if b.Contract != nil {
			out["contract"] = b.Contract
		}
		var unset, other []iacfacts.IaCFinding
		for _, f := range b.Check.Findings {
			if f.Kind == iacfacts.FindingRequiredUnset {
				unset = append(unset, f)
			} else {
				other = append(other, f)
			}
		}
		if len(unset) > 0 {
			out["required_unset"] = unset
		}
		if len(other) > 0 {
			out["findings"] = other
		}
		if w := b.Check.Written; len(w) > 0 {
			out["written_total"] = len(w)
			if len(w) > iacWrittenCap {
				w = w[:iacWrittenCap]
			}
			out["written"] = w
		}
		notes := append(append([]string(nil), b.Check.Disclosures...), got.Disclosures...)
		if len(notes) > 0 {
			out["disclosures"] = notes
		}
		return out
	}
	return nil
}

// Bounds of the review section: module directories read, and disclosures listed.
const (
	iacReviewDirCap = 32
	iacNoteCap      = 20
)

// reviewIaCFindings is the `iac` section of review_diff: the unknown attributes, required arguments unset,
// deprecated attributes and unmarked sensitive outputs of the changed Terraform and OpenTofu files, read against
// the contract each block binds to. It is nil when no catalog is installed or no such file changed. The section
// sits in the payload's other fields, so review_diff's response cap trims its findings list like any other.
func reviewIaCFindings(ctx context.Context, backend QueryInterface, service string, changed []string) map[string]any {
	byDir := map[string]map[string]bool{}
	for _, f := range changed {
		f = filepath.ToSlash(filepath.Clean(f))
		if !strings.HasSuffix(f, ".tf") && !strings.HasSuffix(f, ".tofu") {
			continue
		}
		d := path.Dir(f)
		if byDir[d] == nil {
			byDir[d] = map[string]bool{}
		}
		byDir[d][f] = true
	}
	if len(byDir) == 0 {
		return nil
	}
	dir, list := iacCatalogs()
	if len(list) == 0 {
		return nil
	}
	root, err := resolveCorpusRoot(ctx, backend, service)
	if err != nil {
		return nil
	}
	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	var findings []iacfacts.IaCFinding
	counts := map[string]int{}
	addLines := map[string]bool{}
	notes := map[string]bool{}
	blobs := 0
	if len(dirs) > iacReviewDirCap {
		notes["only the first "+strconv.Itoa(iacReviewDirCap)+" of "+strconv.Itoa(len(dirs))+" changed module directories were read"] = true
		dirs = dirs[:iacReviewDirCap]
	}
	for _, d := range dirs {
		if ctx.Err() != nil {
			break
		}
		got, err := iacfacts.CheckIaCModule(iacfacts.IaCModuleQuery{
			Repo: corpusFS{root: root}, CatalogDir: dir, Catalogs: list, Dir: d, Files: byDir[d], Outputs: true,
		})
		if err != nil {
			notes[d+" was not read: "+err.Error()] = true
			continue
		}
		blobs += got.BlobsRead
		for _, b := range got.Blocks {
			counts[b.Binding.Status]++
			if b.Binding.AddLine != "" {
				addLines[b.Binding.AddLine] = true
			}
			findings = append(findings, b.Check.Findings...)
			for _, n := range b.Check.Disclosures {
				notes[n] = true
			}
		}
		findings = append(findings, got.Outputs.Findings...)
		for _, n := range got.Outputs.Disclosures {
			notes[n] = true
		}
		for _, n := range got.Disclosures {
			notes[n] = true
		}
	}
	out := map[string]any{
		"findings":   findings,
		"blocks":     counts,
		"blobs_read": blobs,
		"scope":      "contract facts at each block's bound provider version, read from installed catalogs; not a plan, and no value is evaluated",
	}
	if len(findings) == 0 {
		out["findings"] = []iacfacts.IaCFinding{}
	}
	if len(addLines) > 0 {
		lines := make([]string, 0, len(addLines))
		for l := range addLines {
			lines = append(lines, l)
		}
		sort.Strings(lines)
		out["unbound_add"] = lines
	}
	if len(notes) > 0 {
		ns := make([]string, 0, len(notes))
		for n := range notes {
			ns = append(ns, n)
		}
		sort.Strings(ns)
		if len(ns) > iacNoteCap {
			ns = append(ns[:iacNoteCap], "more disclosures were dropped")
		}
		out["disclosures"] = ns
	}
	return out
}
