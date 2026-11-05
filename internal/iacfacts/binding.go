package iacfacts

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Binding statuses (design section 5). unbound carries no facts and never falls back to a nearest version.
const (
	BindLocked            = "locked"
	BindConstraintMatched = "constraint-matched"
	BindAmbiguous         = "ambiguous"
	BindUnbound           = "unbound"
)

// Per-answer bounds on what binding reads (design section 9). Each is disclosed when hit.
const (
	// MaxBindingLockfiles is the most lockfiles one answer reads.
	MaxBindingLockfiles = 16
	// maxBindingModules is the most modules walked from one root.
	maxBindingModules = 64
	// maxBindingModuleDepth is how deep module calls are followed.
	maxBindingModuleDepth = 8
	// maxBindingWalk is the most directory entries a lockfile search visits.
	maxBindingWalk = 50_000
	// maxBindingTFFiles is the most .tf files read in one module directory.
	maxBindingTFFiles = 256
	// maxBindingFileBytes is the largest .tf or lock file read.
	maxBindingFileBytes = 1 << 20
)

// LockfileName is Terraform's dependency lock file.
const LockfileName = ".terraform.lock.hcl"

// IaCBlockRef names a resource or data block to bind: the repository-relative directory of its module, its type
// and its `provider =` meta-argument (for example "aws.east") when it has one.
type IaCBlockRef struct {
	Dir      string
	Type     string
	Provider string
}

// IaCBindingQuery is the input to ResolveIaCBinding.
type IaCBindingQuery struct {
	// Repo is the repository root. Reads are bounded and read-only.
	Repo fs.FS
	// Catalogs are the installed catalogs, from ListIaCCatalogs.
	Catalogs []IaCCatalogInfo
	Block    IaCBlockRef
	// Lockfiles are repository-relative paths of every .terraform.lock.hcl. Nil makes the resolver find them.
	Lockfiles []string
}

// IaCBindingRoot is one root whose lockfile reaches the block's module.
type IaCBindingRoot struct {
	// Dir is the root's directory.
	Dir string
	// Version is what that root's lockfile pins for the block's provider; empty when it has no entry.
	Version string
	// Via says how the root reaches the module: "self", "module call" or "nearest lockfile".
	Via string
}

// IaCBinding answers which contract applies to a block. Facts attach only to locked and constraint-matched
// bindings: ambiguous and unbound carry none.
type IaCBinding struct {
	Status string
	// Source is the provider address the block resolves to, host included.
	Source    string
	LocalName string
	Alias     string
	// Version is the locked or constraint-matched release; empty when ambiguous or unbound.
	Version string
	// Locked is the version a lockfile pins when no catalog holds it (an unbound answer).
	Locked     string
	Constraint string
	// Catalog is the installed catalog file that holds Version.
	Catalog string
	// AddLine is the major line to add when a block is unbound for want of a catalog.
	AddLine string
	// Roots are the lockfile roots that reach the module.
	Roots []IaCBindingRoot
	// Candidates are the distinct versions behind an ambiguous answer.
	Candidates  []string
	Disclosures []string
}

// ResolveIaCBinding answers locked, constraint-matched, ambiguous or unbound for one block.
//
//   - locked: the lockfile of every root that reaches the module pins one version of the provider, and an installed
//     catalog holds exactly that version.
//   - constraint-matched: no lockfile reaches the module (a reusable module is the common case), so the highest
//     installed release satisfying `required_providers` is used, and the answer says it is not what any root installs.
//   - ambiguous: roots pin different versions, or a module mapping names a different provider. No first-root choice.
//   - unbound: no catalog holds the exact version (or any release the constraint admits). The disclosure names the
//     major line to add. A nearest version is never used, labelled or not.
//
// The provider's local name is the `provider =` meta-argument's name, else the type's prefix. Its source is the
// module's `required_providers` entry, else Terraform's implied hashicorp/<name>. A module call's `providers` map
// is checked against the caller's own source for the name it maps. Only `./` and `../` module sources are followed.
func ResolveIaCBinding(q IaCBindingQuery) IaCBinding {
	b := IaCBinding{Status: BindUnbound}
	dir := cleanRepoDir(q.Block.Dir)
	local, alias := providerLocalName(q.Block)
	b.LocalName, b.Alias = local, alias
	if local == "terraform" {
		b.Disclosures = append(b.Disclosures, "terraform_* types belong to the built-in terraform provider, which has no catalog")
		return b
	}
	rc := &bindingReader{fsys: q.Repo, mods: map[string]*bindModule{}}
	mod := rc.module(dir)
	b.Disclosures = append(b.Disclosures, rc.notes...)
	rc.notes = nil
	req, declared := mod.required[local]
	b.Source = normalizeProviderSource(req.source, local)
	b.Constraint = req.constraint
	if !declared {
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("provider %q is not in required_providers; Terraform implies %s", local, b.Source))
	}
	cons, err := ParseIaCConstraint(req.constraint)
	if err != nil {
		b.Disclosures = append(b.Disclosures, "the version constraint is unreadable ("+err.Error()+"), so it is ignored")
		cons = IaCConstraint{}
	}

	locks := q.Lockfiles
	if locks == nil {
		var found []string
		found, rc.notes = findLockfiles(q.Repo)
		locks = found
		b.Disclosures = append(b.Disclosures, rc.notes...)
		rc.notes = nil
	}
	sort.Strings(locks)
	if len(locks) > MaxBindingLockfiles {
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("the repository has %d lockfiles; the first %d by path were read", len(locks), MaxBindingLockfiles))
		locks = locks[:MaxBindingLockfiles]
	}

	var mismatch []string
	for _, lf := range locks {
		root := path.Dir(lf)
		via, edge := rc.reaches(root, dir)
		if via == "" {
			continue
		}
		r := IaCBindingRoot{Dir: root, Via: via}
		if lock := rc.lockfile(lf); lock != nil {
			r.Version = lock[b.Source]
		}
		b.Roots = append(b.Roots, r)
		if edge != nil {
			if msg := rc.checkMapping(edge, local, alias, b.Source); msg != "" {
				mismatch = append(mismatch, msg)
			}
		}
	}
	if len(b.Roots) == 0 {
		if lf := nearestLockfile(locks, dir); lf != "" {
			r := IaCBindingRoot{Dir: path.Dir(lf), Via: "nearest lockfile"}
			if lock := rc.lockfile(lf); lock != nil {
				r.Version = lock[b.Source]
			}
			b.Roots = append(b.Roots, r)
			b.Disclosures = append(b.Disclosures, "no module call reaches this module from a lockfile root; the nearest lockfile above it was read")
		}
	}
	b.Disclosures = append(b.Disclosures, rc.notes...)

	if len(mismatch) > 0 {
		b.Status = BindAmbiguous
		b.Disclosures = append(b.Disclosures, mismatch...)
		return b
	}

	pinned := map[string][]string{}
	for _, r := range b.Roots {
		if r.Version != "" {
			pinned[r.Version] = append(pinned[r.Version], r.Dir)
		}
	}
	switch {
	case len(pinned) > 1:
		b.Status = BindAmbiguous
		for v := range pinned {
			b.Candidates = append(b.Candidates, v)
		}
		sortVersionStrings(b.Candidates)
		var parts []string
		for _, v := range b.Candidates {
			parts = append(parts, fmt.Sprintf("%s in %s", v, strings.Join(pinned[v], ", ")))
		}
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("%d roots reach this module and pin different %s versions (%s); no root was chosen", len(b.Roots), b.Source, strings.Join(parts, "; ")))
	case len(pinned) == 1:
		for v := range pinned {
			b.Version = v
		}
		if cat := pickCatalog(q.Catalogs, b.Source, func(v string) bool { return v == b.Version }); cat != nil {
			b.Status, b.Catalog = BindLocked, cat.Name
			b.noteHost(cat)
			return b
		}
		b.Status = BindUnbound
		b.AddLine = lineOf(b.Version)
		b.Disclosures = append(b.Disclosures, unboundNote(q.Catalogs, b.Source, b.Version, b.AddLine))
		b.Locked, b.Version = b.Version, ""
	default:
		b.constraintMatch(q.Catalogs, cons)
	}
	return b
}

func (b *IaCBinding) noteHost(cat *IaCCatalogInfo) {
	want := hostOf(b.Source)
	if got := hostOf(cat.Manifest.Source); got != want {
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("the catalog was built from %s and the block resolves to %s; the two registries' builds of a release read equal at the versions measured (IF-P2), so the facts carry the catalog's address", got, want))
	}
}

// constraintMatch binds a module no lockfile pins: the highest installed release the constraint admits.
func (b *IaCBinding) constraintMatch(catalogs []IaCCatalogInfo, cons IaCConstraint) {
	best := ""
	var bestV IaCVersion
	var bestCat *IaCCatalogInfo
	for i := range catalogs {
		c := &catalogs[i]
		if !sameProvider(c.Manifest.Source, b.Source) {
			continue
		}
		for _, s := range c.Manifest.Versions {
			v, err := ParseIaCVersion(s)
			if err != nil || !cons.Matches(v) {
				continue
			}
			if best == "" || CompareIaCVersions(v, bestV) > 0 {
				best, bestV, bestCat = s, v, c
			}
		}
	}
	if best == "" {
		b.Status = BindUnbound
		if line, ok := cons.LowerLine(); ok {
			b.AddLine = line
		}
		what := "any release"
		if cons.Raw != "" {
			what = "a release satisfying " + cons.Raw
		}
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("no installed catalog holds %s of %s%s", what, b.Source, addLineNote(b.AddLine)))
		return
	}
	b.Status, b.Version, b.Catalog = BindConstraintMatched, best, bestCat.Name
	b.noteHost(bestCat)
	if cons.Raw == "" {
		b.Disclosures = append(b.Disclosures, "no lockfile reaches this module and it states no version constraint: the highest installed release was used, which no root has been shown to install")
	} else {
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("no lockfile reaches this module: the highest installed release satisfying %s was used, which no root has been shown to install", cons.Raw))
	}
}

func addLineNote(line string) string {
	if line == "" {
		return ""
	}
	return fmt.Sprintf(": add the catalog for major line %s (`fathom iac add` or `fathom iac import`)", line)
}

func unboundNote(catalogs []IaCCatalogInfo, source, version, line string) string {
	for i := range catalogs {
		c := &catalogs[i]
		if sameProvider(c.Manifest.Source, source) && c.Manifest.Line == line {
			return fmt.Sprintf("%s %s is not in the installed catalog %s, which holds the %s line from %s to %s: import an export at that version or add a newer catalog of the line; no nearest version is used", source, version, c.Name, line, c.Manifest.Versions[0], c.Manifest.Versions[len(c.Manifest.Versions)-1])
		}
	}
	return fmt.Sprintf("%s %s has no installed catalog%s; no nearest version is used", source, version, addLineNote(line))
}

func lineOf(version string) string {
	v, err := ParseIaCVersion(version)
	if err != nil {
		return ""
	}
	return fmt.Sprint(v.Major)
}

func sortVersionStrings(vs []string) {
	sort.Slice(vs, func(i, j int) bool {
		a, aerr := ParseIaCVersion(vs[i])
		b, berr := ParseIaCVersion(vs[j])
		if aerr != nil || berr != nil {
			return vs[i] < vs[j]
		}
		return CompareIaCVersions(a, b) < 0
	})
}

// pickCatalog returns the installed catalog for source that holds a version accepted by ok. A catalog built from
// the same registry host wins over one built from another, then name order.
func pickCatalog(catalogs []IaCCatalogInfo, source string, ok func(string) bool) *IaCCatalogInfo {
	var best *IaCCatalogInfo
	for i := range catalogs {
		c := &catalogs[i]
		if !sameProvider(c.Manifest.Source, source) {
			continue
		}
		held := false
		for _, v := range c.Manifest.Versions {
			if ok(v) {
				held = true
				break
			}
		}
		if !held {
			continue
		}
		if best == nil || (hostOf(c.Manifest.Source) == hostOf(source) && hostOf(best.Manifest.Source) != hostOf(source)) ||
			(hostOf(c.Manifest.Source) == hostOf(best.Manifest.Source) && c.Name < best.Name) {
			best = c
		}
	}
	return best
}

const defaultRegistryHost = "registry.terraform.io"

// normalizeProviderSource expands a required_providers source to host/namespace/name. With none, Terraform implies
// hashicorp/<local name>.
func normalizeProviderSource(src, local string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return defaultRegistryHost + "/hashicorp/" + local
	}
	switch parts := strings.Split(src, "/"); len(parts) {
	case 1:
		return defaultRegistryHost + "/hashicorp/" + parts[0]
	case 2:
		return defaultRegistryHost + "/" + src
	}
	return src
}

func hostOf(source string) string {
	if i := strings.IndexByte(source, '/'); i >= 0 {
		return source[:i]
	}
	return ""
}

// sameProvider compares namespace and name; the registry host is a disclosure, not a barrier (design section 5).
func sameProvider(a, b string) bool { return nsName(a) == nsName(b) && nsName(a) != "" }

func nsName(source string) string {
	parts := strings.Split(source, "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.ToLower(parts[len(parts)-2] + "/" + parts[len(parts)-1])
}

// providerLocalName is the name a block's provider is known by: the meta-argument's, else the type's prefix.
func providerLocalName(b IaCBlockRef) (local, alias string) {
	if b.Provider != "" {
		local, alias, _ = strings.Cut(b.Provider, ".")
		return local, alias
	}
	local, _, _ = strings.Cut(b.Type, "_")
	return local, ""
}

func cleanRepoDir(d string) string {
	d = path.Clean(strings.TrimPrefix(strings.ReplaceAll(d, "\\", "/"), "./"))
	if d == "" {
		return "."
	}
	return d
}

// nearestLockfile finds the lockfile closest above (or in) dir.
func nearestLockfile(locks []string, dir string) string {
	byDir := map[string]string{}
	for _, l := range locks {
		byDir[path.Dir(l)] = l
	}
	for d := dir; ; d = path.Dir(d) {
		if l, ok := byDir[d]; ok {
			return l
		}
		if d == "." || d == "/" {
			return ""
		}
	}
}
