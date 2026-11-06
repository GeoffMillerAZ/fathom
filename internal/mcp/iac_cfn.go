package mcp

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// CloudFormation facts on the same two surfaces as the Terraform ones (docs/design/iac-facts.md section 6, ADR-091
// amendment, rows IF-C3 and IF-C4): symbol_context on a CloudFormation resource carries an `iac` block, and
// review_diff carries `iac.cloudformation` for the changed templates. The facts are read from the registry-schema
// catalog an operator imported (`fathom iac import --region`). A template names no version, so every answer says the
// facts are the schemas of one region as of the catalog's date. With no CloudFormation catalog installed neither
// surface adds a key.

// kindCFNResource is the source kind the CloudFormation dialect gives a resource.
const kindCFNResource core.SourceKind = "CFNResource"

// Bounds of the CloudFormation reads: bytes of one template, changed templates read, findings listed and the
// disclosures kept.
const (
	cfnFileCap     = 2 << 20
	cfnReviewFiles = 64
	cfnFindingCap  = 200
	cfnNoteCap     = 20
)

// cfnCatalogs lists the installed catalogs when at least one is a CloudFormation catalog.
func cfnCatalogs() []iacfacts.IaCCatalogInfo {
	_, list := iacCatalogs()
	for _, c := range list {
		if c.Manifest.Kind == iacfacts.KindCloudFormation {
			return list
		}
	}
	return nil
}

// readCFNFile reads one repo-relative file through the corpus guard, at most cfnFileCap bytes. ok is false for a file
// that is not there, is over the cap, escapes the corpus or is not a CloudFormation template.
func readCFNFile(root, name string) ([]byte, bool) {
	f, err := corpusFS{root: root}.Open(name)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	src, err := io.ReadAll(io.LimitReader(f, cfnFileCap+1))
	if err != nil || len(src) > cfnFileCap || !iacfacts.LooksLikeCloudFormation(src) {
		return nil, false
	}
	return src, true
}

// cfnBindingJSON is the binding of a CloudFormation answer.
func cfnBindingJSON(b iacfacts.CFNBinding) map[string]any {
	status := "unbound"
	if b.Bound {
		status = iacfacts.CFNUnversioned
	}
	out := map[string]any{"status": status, "version": b.Version, "note": b.Note}
	for k, v := range map[string]string{"catalog": b.Catalog, "region": b.Region, "date": b.Date, "schema": b.Schema} {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

func cfnRef(b iacfacts.CFNBinding, digest, field string) iacfacts.IaCFactRef {
	return iacfacts.IaCFactRef{Catalog: b.Catalog, Source: iacfacts.CFNSource, Version: iacfacts.CFNUnversioned, Digest: digest, Path: field}
}

// cfnUseFinding turns one attribute finding of CheckCloudFormationUse into the shape the Terraform findings use.
func cfnUseFinding(u iacfacts.CFNUse, f iacfacts.CFNFinding) iacfacts.IaCFinding {
	var msg string
	switch f.Kind {
	case iacfacts.CFNUnknownProperty:
		msg = fmt.Sprintf("%q is not a property of %s in the registry schema", f.Property, u.Type)
	case iacfacts.CFNMissingRequired:
		msg = fmt.Sprintf("%q is required by %s and is not set", f.Property, u.Type)
	case iacfacts.CFNReadOnlySet:
		msg = fmt.Sprintf("%q is read-only in %s and cannot be set", f.Property, u.Type)
	case iacfacts.CFNDeprecated:
		msg = fmt.Sprintf("%q is deprecated in %s", f.Property, u.Type)
	default:
		msg = f.Kind
	}
	return iacfacts.IaCFinding{Kind: f.Kind, Block: u.Name, Path: f.Property, File: f.Span.File, Line: f.Span.StartLine,
		Binding: iacfacts.CFNUnversioned, Message: msg, Fact: cfnRef(u.Binding, f.Digest, f.Path)}
}

// cfnChangeFinding turns one replacement finding into the same shape. The message names the schema field, so a
// reader sees where the claim comes from and that no change set confirmed it.
func cfnChangeFinding(c iacfacts.CFNChange, f iacfacts.CFNChangeFinding) iacfacts.IaCFinding {
	var msg string
	if f.Kind == iacfacts.CFNForcesReplacement {
		msg = fmt.Sprintf("%s %q: the registry schema of %s lists it in %s (%s), so changing it forces replacement", f.Change, f.Property, c.Type, f.Path, f.Pointer)
	} else {
		msg = fmt.Sprintf("%s %q: the registry schema of %s lists it in %s (%s), so changing it may replace the resource under conditions the schema states", f.Change, f.Property, c.Type, f.Path, f.Pointer)
	}
	return iacfacts.IaCFinding{Kind: f.Kind, Block: c.Name, Path: f.Property, File: f.Span.File, Line: f.Span.StartLine,
		Binding: iacfacts.CFNUnversioned, Message: msg, Fact: cfnRef(c.Binding, f.Digest, f.Path)}
}

// symbolContextCFNFacts is the `iac` block of symbol_context on a CloudFormation resource: the binding, the
// attribute findings of its Properties, and the properties it writes that its schema says replace the resource when
// changed. It is nil for any other symbol, when no CloudFormation catalog is installed, or when the resource cannot
// be found in its template.
func symbolContextCFNFacts(ctx context.Context, backend QueryInterface, service string, def core.SymbolNode) map[string]any {
	if def.Kind != kindCFNResource || def.FilePath == "" || def.Name == "" {
		return nil
	}
	list := cfnCatalogs()
	if len(list) == 0 {
		return nil
	}
	root, err := resolveCorpusRoot(ctx, backend, service)
	if err != nil {
		return nil
	}
	file := path.Clean(filepath.ToSlash(def.FilePath))
	src, ok := readCFNFile(root, file)
	if !ok {
		return nil
	}
	set, err := iacfacts.ReadCloudFormationFile(file, src)
	if err != nil {
		return map[string]any{"status": "unread", "reason": err.Error()}
	}
	for _, r := range set.Resources {
		if r.Name != def.Name {
			continue
		}
		u := iacfacts.CheckCloudFormationUse(list, "", r)
		out := map[string]any{"binding": cfnBindingJSON(u.Binding)}
		if !u.Binding.Bound {
			return out
		}
		var findings []iacfacts.IaCFinding
		for _, f := range u.Findings {
			findings = append(findings, cfnUseFinding(u, f))
		}
		if len(findings) > 0 {
			out["findings"] = findings
		}
		facts, _ := iacfacts.CFNReplacementFacts(list, "", r)
		var rep []map[string]any
		for _, f := range facts {
			effect := iacfacts.CFNForcesReplacement
			if f.Class == iacfacts.CFNConditional {
				effect = iacfacts.CFNMayReplace
			}
			rep = append(rep, map[string]any{"property": f.Property, "effect": effect, "line": f.Span.StartLine, "fact": cfnRef(u.Binding, f.Digest, f.Path)})
		}
		if len(rep) > 0 {
			out["replacement_if_changed"] = rep
		}
		out["disclosures"] = []string{u.Binding.Note, "the schema states the replacement; no change set confirmed it, and only top-level properties are checked for unknown, missing, read-only and deprecated"}
		return out
	}
	return nil
}

// cfnBaseFile reads a file as the base commit had it. ok is false when the base has no such file or it is not a
// readable template. base is a resolved commit id, never a ref a caller wrote.
func cfnBaseFile(ctx context.Context, root, base, name string) ([]byte, bool) {
	if base == "" || strings.HasPrefix(base, "-") || !fsValidName(name) {
		return nil, false
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "cat-file", "blob", base+":"+name)
	out, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return nil, false
	}
	src, rerr := io.ReadAll(io.LimitReader(out, cfnFileCap+1))
	if len(src) > cfnFileCap {
		_ = cmd.Process.Kill()
	}
	werr := cmd.Wait()
	if rerr != nil || werr != nil || len(src) > cfnFileCap || !iacfacts.LooksLikeCloudFormation(src) {
		return nil, false
	}
	return src, true
}

func fsValidName(name string) bool {
	return name != "" && name == path.Clean(name) && !strings.HasPrefix(name, "/") && !strings.HasPrefix(name, "../") && name != ".."
}

// cfnTemplateExt reports whether a changed file may be a CloudFormation template.
func cfnTemplateExt(f string) bool {
	switch strings.ToLower(path.Ext(f)) {
	case ".yaml", ".yml", ".json", ".template", ".cfn":
		return true
	}
	return false
}

// reviewCFNFindings is the `cloudformation` part of review_diff's `iac` section: the attribute findings of the
// changed CloudFormation templates, and, when base names the commit the change is measured from, each changed
// property their schema says forces replacement or may. A file the base lacks is new and replaces nothing. It is nil
// when no CloudFormation catalog is installed or no changed file is a template. The list is capped, and the response
// cap trims it further like any other field.
func reviewCFNFindings(ctx context.Context, backend QueryInterface, service string, changed []string, base string) map[string]any {
	var files []string
	seen := map[string]bool{}
	for _, f := range changed {
		f = path.Clean(filepath.ToSlash(f))
		if cfnTemplateExt(f) && fsValidName(f) && !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return nil
	}
	list := cfnCatalogs()
	if len(list) == 0 {
		return nil
	}
	root, err := resolveCorpusRoot(ctx, backend, service)
	if err != nil {
		return nil
	}
	sort.Strings(files)
	notes := map[string]bool{}
	if len(files) > cfnReviewFiles {
		notes[fmt.Sprintf("only the first %d of %d changed files were read", cfnReviewFiles, len(files))] = true
		files = files[:cfnReviewFiles]
	}
	var findings []iacfacts.IaCFinding
	read, bound, unbound := 0, 0, 0
	catalogs := map[string]bool{}
	for _, f := range files {
		if ctx.Err() != nil {
			break
		}
		src, ok := readCFNFile(root, f)
		if !ok {
			continue
		}
		head, err := iacfacts.ReadCloudFormationFile(f, src)
		if err != nil {
			notes[f+" was not read: "+err.Error()] = true
			continue
		}
		read++
		var baseRes map[string]iacfacts.Resource
		var baseSource []byte
		if baseSrc, ok := cfnBaseFile(ctx, root, base, f); ok {
			if bs, err := iacfacts.ReadCloudFormationFile(f, baseSrc); err == nil {
				baseSource = baseSrc
				baseRes = map[string]iacfacts.Resource{}
				for _, r := range bs.Resources {
					baseRes[r.Name] = r
				}
			}
		}
		for _, r := range head.Resources {
			u := iacfacts.CheckCloudFormationUse(list, "", r)
			if !u.Binding.Bound {
				unbound++
				notes[u.Binding.Note] = true
				continue
			}
			bound++
			catalogs[fmt.Sprintf("%s (region %s, %s)", u.Binding.Catalog, u.Binding.Region, u.Binding.Date)] = true
			for _, uf := range u.Findings {
				findings = append(findings, cfnUseFinding(u, uf))
			}
			if br, ok := baseRes[r.Name]; ok {
				c := iacfacts.CheckCloudFormationChange(list, "", br, r, baseSource, src)
				for _, cf := range c.Findings {
					findings = append(findings, cfnChangeFinding(c, cf))
				}
				for _, n := range c.Notes {
					notes[n] = true
				}
			}
		}
	}
	if read == 0 {
		return nil
	}
	if base == "" && bound > 0 {
		notes["replacement findings compare a template with its base: pass since (the base ref) to get them"] = true
	}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Path < b.Path
	})
	total := len(findings)
	if total > cfnFindingCap {
		findings = findings[:cfnFindingCap]
		notes[fmt.Sprintf("%d of %d findings are listed", cfnFindingCap, total)] = true
	}
	if findings == nil {
		findings = []iacfacts.IaCFinding{}
	}
	out := map[string]any{
		"findings":       findings,
		"findings_total": total,
		"resources":      map[string]int{"bound": bound, "unbound": unbound},
		"scope":          "contract facts from the registry schemas of an installed catalog, unversioned because a template names no version; the schema states replacement, no change set confirmed it",
	}
	if len(catalogs) > 0 {
		cs := make([]string, 0, len(catalogs))
		for c := range catalogs {
			cs = append(cs, c)
		}
		sort.Strings(cs)
		out["catalogs"] = cs
	}
	if len(notes) > 0 {
		ns := make([]string, 0, len(notes))
		for n := range notes {
			ns = append(ns, n)
		}
		sort.Strings(ns)
		if len(ns) > cfnNoteCap {
			ns = append(ns[:cfnNoteCap], "more disclosures were dropped")
		}
		out["disclosures"] = ns
	}
	return out
}
