// classify.go assigns a Category to one patch by its shape alone (SB-33,
// vision 27 §7, ADR-033) — no model in the loop, deterministic, and
// reusable by DeriveScenarioTasks and by hand-authored corpora alike.
package scenariobench

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// Evidence names the deciding facts behind a ClassifyChange verdict — the
// deterministic "why" beside the "what", so a corpus row can be audited
// without re-running the classifier.
type Evidence struct {
	// Rule names which numbered rule decided (see ClassifyChange's doc).
	Rule string `json:"rule"`
	// Files are the patch-touched paths the rule turned on.
	Files []string `json:"files,omitempty"`
	// Functions are the exported func/method qualified names whose
	// signature differed between before and after (rule 2 only).
	Functions []string `json:"functions,omitempty"`
	// Packages are the distinct non-test Go package directories the patch
	// touches (rule 3 only).
	Packages []string `json:"packages,omitempty"`
	// Skipped names checks that could not run and why (e.g. "signature:
	// no before/after tree" ) — never silent, per ADR-027.
	Skipped []string `json:"skipped,omitempty"`
}

var governanceBasenames = map[string]bool{
	"CONTRIBUTING.md": true,
	"CLAUDE.md":       true,
	"AGENTS.md":       true,
}

// isGovernancePath reports whether p is one of the named rule files or
// lives under docs/.
func isGovernancePath(p string) bool {
	if governanceBasenames[path.Base(p)] {
		return true
	}
	return strings.HasPrefix(p, "docs/")
}

// touchedPath is whichever side of a patch file is non-empty, preferring
// the after-path (a rename's identity for classification purposes is what
// it is called after the change; a pure delete falls back to the before
// path).
func touchedPath(pf publicloc.PatchFile) string {
	if pf.NewPath != "" {
		return pf.NewPath
	}
	return pf.OldPath
}

// ClassifyChange assigns a Category to one patch by its shape — files and
// packages spanned, an exported signature changed, go.mod touched, a
// governance file touched alongside code — never by reading the prompt or
// running a model. Rules apply in this fixed order; the first match wins,
// and Evidence names exactly what decided it:
//
//  1. go.mod touched                                   -> dependency_upgrade
//  2. an exported func/method's signature differs        -> signature_change
//     between before and after (go/parser; a file this
//     rule cannot read from both trees is skipped, never
//     guessed — before/after may be nil, in which case
//     the whole rule is skipped)
//  3. the patch touches >= 2 distinct non-test Go        -> cross_package_feature
//     package directories
//  4. a governance file (CONTRIBUTING.md, CLAUDE.md,      -> governance_bound
//     AGENTS.md, or anything under docs/) is touched
//     alongside a non-governance file
//  5. otherwise                                          -> bug_fix
func ClassifyChange(patch []byte, before, after fs.FS) (Category, Evidence) {
	files, err := publicloc.ParsePatch(string(patch))
	if err != nil {
		return CategoryBugFix, Evidence{Rule: "unparseable patch: " + err.Error()}
	}

	// Rule 1: go.mod.
	for _, pf := range files {
		if path.Base(touchedPath(pf)) == "go.mod" {
			return CategoryUpgrade, Evidence{Rule: "go.mod touched", Files: []string{touchedPath(pf)}}
		}
	}

	// Rule 2: an exported signature changed.
	if cat, ev, ok := classifySignatureChange(files, before, after); ok {
		return cat, ev
	} else if len(ev.Skipped) > 0 {
		// Carried into whichever later rule fires, so a reader can see the
		// check ran and why it could not decide — folded into rule 3/4/5's
		// evidence below.
		return classifyRemaining(files, ev.Skipped)
	}

	return classifyRemaining(files, nil)
}

func classifyRemaining(files []publicloc.PatchFile, skipped []string) (Category, Evidence) {
	// Rule 3: >= 2 non-test Go packages.
	pkgs := map[string]bool{}
	for _, pf := range files {
		p := touchedPath(pf)
		if publicloc.IsGoSource(p) {
			pkgs[path.Dir(p)] = true
		}
	}
	if len(pkgs) >= 2 {
		list := make([]string, 0, len(pkgs))
		for p := range pkgs {
			list = append(list, p)
		}
		sort.Strings(list)
		return CategoryFeature, Evidence{Rule: fmt.Sprintf("%d non-test Go packages touched", len(list)), Packages: list, Skipped: skipped}
	}

	// Rule 4: a governance file touched alongside code.
	var governance, other []string
	for _, pf := range files {
		p := touchedPath(pf)
		if isGovernancePath(p) {
			governance = append(governance, p)
		} else {
			other = append(other, p)
		}
	}
	if len(governance) > 0 && len(other) > 0 {
		sort.Strings(governance)
		sort.Strings(other)
		return CategoryGovernance, Evidence{Rule: "governance file touched alongside code", Files: append(append([]string{}, governance...), other...), Skipped: skipped}
	}

	return CategoryBugFix, Evidence{Rule: "none of the higher rules matched", Skipped: skipped}
}

// classifySignatureChange runs rule 2. ok is false when the rule did not
// fire (either nothing differed, or nothing could be compared); Evidence
// in that case carries only Skipped, which the caller folds forward.
func classifySignatureChange(files []publicloc.PatchFile, before, after fs.FS) (Category, Evidence, bool) {
	if before == nil || after == nil {
		return "", Evidence{Skipped: []string{"signature check: no before/after tree given"}}, false
	}
	var changed []string
	var skipped []string
	for _, pf := range files {
		if pf.Added || pf.Deleted {
			continue // nothing to diff a signature against
		}
		oldPath, newPath := pf.OldPath, pf.NewPath
		if !publicloc.IsGoSource(oldPath) {
			continue
		}
		beforeSrc, err := fs.ReadFile(before, oldPath)
		if err != nil {
			skipped = append(skipped, oldPath+": not readable in the before tree ("+err.Error()+")")
			continue
		}
		afterSrc, err := fs.ReadFile(after, newPath)
		if err != nil {
			skipped = append(skipped, newPath+": not readable in the after tree ("+err.Error()+")")
			continue
		}
		beforeSigs, err := exportedSignatures(beforeSrc, oldPath)
		if err != nil {
			skipped = append(skipped, oldPath+": before tree did not parse ("+err.Error()+")")
			continue
		}
		afterSigs, err := exportedSignatures(afterSrc, newPath)
		if err != nil {
			skipped = append(skipped, newPath+": after tree did not parse ("+err.Error()+")")
			continue
		}
		for name, sig := range beforeSigs {
			if got, ok := afterSigs[name]; ok && got != sig {
				changed = append(changed, name)
			}
		}
	}
	if len(changed) == 0 {
		return "", Evidence{Skipped: skipped}, false
	}
	sort.Strings(changed)
	return CategorySignature, Evidence{Rule: "exported signature changed (go/parser before/after)", Functions: changed, Skipped: skipped}, true
}

// exportedSignatures maps every exported top-level func/method's qualified
// name (Recv.Method, or Name for a plain function) to a rendering of its
// signature (receiver, parameters, results — never the body), so two
// versions of the same file can be compared textually without a type
// checker.
func exportedSignatures(src []byte, filename string) (map[string]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || !fd.Name.IsExported() {
			continue
		}
		name := publicloc.QualifiedFuncName(fd)
		sig := ast.FuncDecl{Name: fd.Name, Type: fd.Type, Recv: fd.Recv}
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, &sig); err != nil {
			return nil, err
		}
		out[name] = buf.String()
	}
	return out, nil
}
