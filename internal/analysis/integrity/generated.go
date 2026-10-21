package integrity

// generated.go — CV-11 (ADR-044 §1.5): GeneratedHandEdits, a hand edit to
// a generated file whose generator inputs did not change in the range.
// FD-16 (docs/design/fathom-dir.md section 7.4): declared `generated`
// outputs are read first.

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/techpack"
)

// generatorInputExts is the same-directory generator-input class this
// function checks (design §1.5's "same-directory .proto, .cue, .sql").
var generatorInputExts = []string{".proto", ".cue", ".sql"}

// GeneratedInput bundles what GeneratedHandEdits needs.
type GeneratedInput struct {
	// Root is the repository root the `generated` declarations are read
	// from: the project pack's (.fathom/*.cue) and the tech packs the
	// repository's own pins put on disk (techpack.Discover). "" reads none,
	// leaving the same-directory heuristic alone.
	Root         string
	ChangedFiles []string
	BaseFile     func(path string) ([]byte, bool, error)
	HeadFile     func(path string) ([]byte, bool, error)
	// HeadGenerated classifies a worktree header without retaining the full
	// file. Nil preserves the legacy HeadFile-based classifier.
	HeadGenerated func(path string) (generated bool, exists bool, err error)
	// SiblingFiles maps a directory (repo-relative, "." for the root) to
	// every OTHER repo-relative file that exists in it at HEAD —
	// conserve.Input carries no directory-listing primitive of its own,
	// so the caller supplies this (e.g. from the store's indexed source
	// list, or a filepath.Glob against the worktree). A directory absent
	// from this map is treated as "could not be listed": GeneratedHandEdits
	// then names it in the returned Missing rather than either fabricating
	// a hand-edit finding it cannot actually prove, or a silent clean pass.
	SiblingFiles map[string][]string
}

// GeneratedHandEdits reports a changed file connector.IsGenerated tags at
// HEAD whose same-directory .proto/.cue/.sql generator inputs (design
// §1.5) did not change between base and head. missing names every changed
// generated file whose directory in.SiblingFiles could not list (I44: an
// unlistable directory means this function cannot PROVE the inputs were
// unchanged, so it reports the gap rather than staying silent).
//
// Scope (disclosed, narrower than design §1.5): a `//go:generate` source
// named in the package is NOT checked — only the same-directory .proto/
// .cue/.sql class is. This function only ever ADDS evidence it can prove;
// a generated file whose only generator input is a go:generate source is
// simply never flagged here (a false negative, never a false positive).
// CV-M3's 30-day kill window (docs/plans/refactor-conservation.md) is what
// will show whether this narrower version already catches real cases
// before the go:generate class is worth the extra parsing it would need.
//
// Declarations first (FD-16): a changed file a `generated` declaration
// names as its output (the project's first, then each discovered pack's by
// name) is judged by that declaration alone. When none of its declared
// sources changed, the finding cites the declaration and its regenerate
// command; the heuristic below runs only for what no declaration covers.
func GeneratedHandEdits(in GeneratedInput) (findings []Finding, missing []string) {
	missingSet := map[string]bool{}
	decls, declMissing := generatedDeclarations(in.Root)
	for _, m := range declMissing {
		missingSet[m] = true
	}
	for _, p := range in.ChangedFiles {
		if d, sources, ok := declarationFor(decls, p); ok {
			if f, edited, err := declaredHandEdit(in, d, p, sources); err != nil {
				missingSet[fmt.Sprintf("generated declaration %s could not be checked: %v", p, err)] = true
			} else if edited {
				findings = append(findings, f)
			}
			continue
		}
		var head []byte
		var ok, generated bool
		var err error
		if in.HeadGenerated != nil {
			generated, ok, err = in.HeadGenerated(p)
		} else {
			head, ok, err = in.HeadFile(p)
			generated = ok && connector.IsGenerated(p, head)
		}
		if err != nil {
			missingSet[fmt.Sprintf("generated header %s could not be read: %v", p, err)] = true
			continue
		}
		if !ok || !generated {
			continue
		}
		if in.HeadGenerated != nil {
			head, ok, err = in.HeadFile(p)
			if err != nil {
				missingSet[fmt.Sprintf("generated file %s could not be read: %v", p, err)] = true
				continue
			}
			if !ok {
				missingSet[fmt.Sprintf("generated file %s disappeared after header classification", p)] = true
				continue
			}
		}
		base, baseOK, err := in.BaseFile(p)
		if err != nil {
			missingSet[fmt.Sprintf("generated base %s could not be read: %v", p, err)] = true
			continue
		}
		if !baseOK {
			continue // newly added generated file: nothing to compare against, never a hand edit
		}
		if bytes.Equal(base, head) {
			continue // unchanged content: nothing to flag regardless of inputs
		}

		dir := path.Dir(p)
		siblings, listed := in.SiblingFiles[dir]
		if !listed {
			missingSet[fmt.Sprintf("directory %s could not be listed for %s's generator inputs", dir, p)] = true
			continue
		}
		inputs := generatorInputCandidates(dir, siblings)
		if len(inputs) == 0 {
			continue // no same-directory .proto/.cue/.sql input found — nothing this function can check
		}

		anyChanged := false
		inputUnknown := false
		for _, ip := range inputs {
			b, _, berr := in.BaseFile(ip)
			h, _, herr := in.HeadFile(ip)
			if berr != nil || herr != nil {
				missingSet[fmt.Sprintf("generator input %s could not be read: base=%v head=%v", ip, berr, herr)] = true
				inputUnknown = true
				continue
			}
			if !bytes.Equal(b, h) {
				anyChanged = true
				break
			}
		}
		if anyChanged || inputUnknown {
			continue // a real generator input changed too — regenerating explains the diff
		}

		findings = append(findings, Finding{
			Kind:     KindHandEdit,
			Key:      p,
			Detail:   fmt.Sprintf("generated file changed but its generator input(s) (%s) did not", strings.Join(inputs, ", ")),
			BaseFile: p,
			HeadFile: p,
		})
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Key < findings[j].Key })
	for m := range missingSet {
		missing = append(missing, m)
	}
	sort.Strings(missing)
	return findings, missing
}

// declaration is one `generated` declaration and what declared it.
type declaration struct {
	techpack.Generated
	by string
}

// generatedDeclarations reads root's project pack declarations, then every
// discovered tech pack's, in that order. A source that cannot be read is
// named in missing, never skipped silently (I38).
func generatedDeclarations(root string) (decls []declaration, missing []string) {
	if root == "" {
		return nil, nil
	}
	if _, err := os.Stat(projectdir.Under(root)); err == nil {
		proj, lerr := projectdir.Load(root)
		if lerr != nil {
			missing = append(missing, fmt.Sprintf("project generated declarations could not be read: %v", lerr))
		} else if k, ok := proj.Key("generated"); ok {
			rel := k.Path
			if r, rerr := filepath.Rel(root, k.Path); rerr == nil {
				rel = filepath.ToSlash(r)
			}
			var gs []techpack.Generated
			switch {
			case k.Err != nil:
				missing = append(missing, fmt.Sprintf("project generated declarations in %s did not load: %v", rel, k.Err))
			case k.Value.Decode(&gs) != nil:
				missing = append(missing, fmt.Sprintf("project generated declarations in %s did not decode", rel))
			default:
				for _, g := range gs {
					decls = append(decls, declaration{Generated: g, by: rel})
				}
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		missing = append(missing, fmt.Sprintf("project generated declarations could not be read: %v", err))
	}
	d, err := techpack.Served(root)
	if err != nil {
		missing = append(missing, fmt.Sprintf("tech packs could not be discovered for generated declarations: %v", err))
	}
	for _, pk := range d.Packs {
		for _, g := range pk.Generated {
			decls = append(decls, declaration{Generated: g, by: pk.Tag() + " (" + filepath.Join(pk.Dir, techpack.PackFile) + ")"})
		}
	}
	return decls, missing
}

// declarationFor is the first declaration whose output is p, with its
// sources filled.
func declarationFor(decls []declaration, p string) (declaration, []string, bool) {
	for _, d := range decls {
		if sources, ok := d.Match(p); ok {
			return d, sources, true
		}
	}
	return declaration{}, nil, false
}

// declaredHandEdit judges one declared output: changed in the range while
// none of its declared sources (a glob source matches the changed files) did.
func declaredHandEdit(in GeneratedInput, d declaration, p string, sources []string) (Finding, bool, error) {
	head, headOK, err := in.HeadFile(p)
	if err != nil {
		return Finding{}, false, err
	}
	if !headOK {
		return Finding{}, false, nil // deleted at head: nothing hand-edited
	}
	base, baseOK, err := in.BaseFile(p)
	if err != nil {
		return Finding{}, false, err
	}
	if !baseOK || bytes.Equal(base, head) {
		return Finding{}, false, nil // newly added, or unchanged
	}
	var candidates []string
	for _, s := range sources {
		if !techpack.IsGlob(s) {
			candidates = append(candidates, s)
			continue
		}
		for _, c := range in.ChangedFiles {
			if c != p && techpack.GlobMatch(s, c) {
				candidates = append(candidates, c)
			}
		}
	}
	for _, c := range candidates {
		b, _, berr := in.BaseFile(c)
		h, _, herr := in.HeadFile(c)
		if berr != nil || herr != nil {
			return Finding{}, false, fmt.Errorf("declared source %s: base=%v head=%v", c, berr, herr)
		}
		if !bytes.Equal(b, h) {
			return Finding{}, false, nil // a declared source changed: regenerating explains the diff
		}
	}
	return Finding{
		Kind:     KindHandEdit,
		Key:      p,
		Detail:   fmt.Sprintf("declared generated output changed but its source(s) (%s) did not: declared by %s, regenerated by %q", strings.Join(sources, ", "), d.by, d.Regenerate),
		BaseFile: p,
		HeadFile: p,
	}, true, nil
}

// generatorInputCandidates returns every path in siblings (repo-relative,
// living in dir) whose extension is one of generatorInputExts, sorted.
func generatorInputCandidates(dir string, siblings []string) []string {
	var out []string
	for _, s := range siblings {
		if path.Dir(s) != dir {
			continue
		}
		for _, ext := range generatorInputExts {
			if strings.HasSuffix(s, ext) {
				out = append(out, s)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}
