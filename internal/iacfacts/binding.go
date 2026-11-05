package iacfacts

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/readlimit"
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
	// Via says how the root reaches the module: "self" or "module call".
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
	Candidates []string
	// Refusal is set when a bound stopped the read and any answer could be wrong. The status is unbound, with no facts.
	Refusal     *IaCRefusal
	Disclosures []string
}

// ResolveIaCBinding answers locked, constraint-matched, ambiguous or unbound for one block.
//
//   - locked: the lockfile of every root that reaches the module pins one version of the provider, and an installed
//     catalog holds exactly that version.
//   - constraint-matched: no lockfile reaches the module (a reusable module is the common case), so the highest
//     installed release satisfying `required_providers` is used, and the answer says it is not what any root installs.
//   - ambiguous: roots pin different versions, or a module mapping names a different provider. No first-root choice.
//   - unbound: no catalog holds the exact version (or any release the constraint admits), the constraint is
//     unreadable, or the provider's registry host is neither default registry and no catalog is for that host. The
//     disclosure names the major line to add. A nearest version is never used, labelled or not.
//
// A bound that stops the read (more than MaxBindingLockfiles lockfiles, a stopped lockfile search, or a module walk
// that stopped before it saw every module) refuses the answer with a reason: it is unbound with Refusal set, never a
// definite answer a hidden root could contradict.
//
// The provider's local name is the `provider =` meta-argument's name, else the type's prefix. Its source is the
// module's `required_providers` entry, else Terraform's implied hashicorp/<name>. A module call's `providers` map
// is checked against the caller's own source for the name it maps. Only `./` and `../` module sources are followed.
//
// The answer comes by a deadline (BindingDeadline). An overrun is refused with a reason, as a bound is, and the work
// it was doing is abandoned, not stopped: Go cannot interrupt it, and it runs on until it returns, its answer
// discarded. Once GOMAXPROCS abandoned answers are still running, a new one is refused at once.
func ResolveIaCBinding(q IaCBindingQuery) IaCBinding {
	if limit := runtime.GOMAXPROCS(0); bindingGroup.Saturated(limit) {
		return bindingRefused(q, fmt.Sprintf("%d earlier answers past their %v deadline are still running, the most that may run is %d, so this one was not started", bindingGroup.Abandoned(), bindingDeadline, limit))
	}
	b, out, err := readlimit.Run(context.Background(), &bindingGroup, bindingDeadline, func() (IaCBinding, error) {
		return resolveIaCBinding(q), nil
	})
	switch {
	case out == readlimit.DeadlineExceeded:
		return bindingRefused(q, fmt.Sprintf("the answer passed its %v deadline and was abandoned", bindingDeadline))
	case err != nil:
		return bindingRefused(q, "the answer failed: "+err.Error())
	}
	return b
}

// BindingDeadline is how long ResolveIaCBinding works on one answer before it refuses it.
const BindingDeadline = 30 * time.Second

// bindingDeadline is BindingDeadline, a variable so a test can lower it.
var bindingDeadline = BindingDeadline

// bindingGroup counts the abandoned answers still running.
var bindingGroup readlimit.Group

// bindingRefused is the answer for a block whose read could not finish: unbound with a reason and no facts.
func bindingRefused(q IaCBindingQuery, reason string) IaCBinding {
	local, alias := providerLocalName(q.Block)
	return IaCBinding{
		Status: BindUnbound, LocalName: local, Alias: alias,
		Refusal:     refuse(RefuseDeadline, "%s", reason),
		Disclosures: []string{"the answer is refused: " + reason + "; no facts are attached"},
	}
}

func resolveIaCBinding(q IaCBindingQuery) IaCBinding {
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
	b.Disclosures = append(b.Disclosures, rc.takeNotes()...)
	req, declared := mod.required[local]
	b.Source = normalizeProviderSource(req.source, local)
	implied := providerHostImplied(req.source)
	constraint := req.constraint
	if vs := mod.providerVersions[local]; len(vs) > 0 {
		// OpenTofu honours a provider block's version beside required_providers and installs what satisfies all of them.
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("provider %q has block version constraints (%s), which OpenTofu deprecates but honours together with required_providers", local, strings.Join(vs, "; ")))
		constraint = strings.Join(append(nonEmpty(req.constraint), vs...), ", ")
	}
	b.Constraint = constraint
	if !declared {
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("provider %q is not in required_providers; Terraform implies %s", local, b.Source))
	}
	cons, consErr := ParseIaCConstraint(constraint)

	locks := q.Lockfiles
	incomplete := ""
	if locks == nil {
		found, notes, unreadDirs, cut := findLockfiles(q.Repo)
		locks = found
		b.Disclosures = append(b.Disclosures, notes...)
		for _, u := range unreadDirs {
			rc.skip("%s", u)
		}
		if cut {
			incomplete = fmt.Sprintf("the lockfile search stopped after %d entries, so a lockfile that reaches this module may be unread", maxBindingWalk)
		}
	}
	sort.Strings(locks)
	if len(locks) > MaxBindingLockfiles {
		incomplete = fmt.Sprintf("the repository has %d lockfiles, over the %d one answer reads, so a root that reaches this module may be unread", len(locks), MaxBindingLockfiles)
		locks = locks[:MaxBindingLockfiles]
	}

	var mismatch []string
	walkStopped := ""
	// lockHosts are the registry hosts of the lock keys that matched an implied source.
	lockHosts := map[string]bool{}
	for _, lf := range locks {
		root := path.Dir(lf)
		via, edges, cut := rc.reaches(root, dir)
		if via == "" {
			if cut && walkStopped == "" {
				walkStopped = fmt.Sprintf("the module walk from %s stopped before it saw every module, so that root may reach this one", root)
			}
			continue
		}
		r := IaCBindingRoot{Dir: root, Via: via}
		if lock := rc.lockfile(lf); lock != nil {
			var conflict, host string
			r.Version, host, conflict = lockVersion(lock, b.Source, implied)
			if conflict != "" {
				mismatch = append(mismatch, fmt.Sprintf("%s: %s", lf, conflict))
			}
			if host != "" {
				lockHosts[host] = true
			}
		}
		b.Roots = append(b.Roots, r)
		for _, edge := range edges {
			if msg := rc.checkMapping(edge, local, alias, b.Source); msg != "" {
				mismatch = append(mismatch, msg)
			}
		}
	}
	if len(b.Roots) == 0 && walkStopped == "" {
		// A lockfile applies to the module graph of its own directory. Running OpenTofu in this module's directory
		// reads that directory's lockfile and none above it, so one that no module call reaches is no pin for it.
		if lf := nearestLockfile(locks, dir); lf != "" {
			b.Disclosures = append(b.Disclosures, fmt.Sprintf("the nearest lockfile above this module, %s, is not reached by any module call, so it pins nothing for this module", lf))
		}
	}
	b.Disclosures = append(b.Disclosures, rc.takeNotes()...)

	unread := ""
	if len(rc.unread) > 0 {
		unread = "an input binding needs was not read (" + rc.unread[0] + "), so the answer could be contradicted by it"
	}
	if reason := firstNonEmpty(firstNonEmpty(incomplete, walkStopped), unread); reason != "" {
		b.Status = BindUnbound
		b.Refusal = refuse(RefuseBindingBound, "%s", reason)
		b.Disclosures = append(b.Disclosures, "the answer is refused: "+reason+"; no facts are attached")
		return b
	}
	if len(mismatch) > 0 {
		b.Status = BindAmbiguous
		b.Disclosures = append(b.Disclosures, mismatch...)
		return b
	}

	// A source with no host resolves to the registry its lock key names: OpenTofu writes registry.opentofu.org for
	// the shorthand, and the address a tool reports for the block is that one. Roots that name two hosts leave the
	// default, since no one host is the block's.
	if implied && len(lockHosts) == 1 {
		for h := range lockHosts {
			b.Source = h + "/" + nsName(b.Source)
		}
	}

	pinned := map[string][]string{}
	var unpinned []string
	for _, r := range b.Roots {
		if r.Version != "" {
			pinned[r.Version] = append(pinned[r.Version], r.Dir)
		} else {
			unpinned = append(unpinned, r.Dir)
		}
	}
	switch {
	case len(pinned) > 1 || (len(pinned) == 1 && len(unpinned) > 0):
		b.Status = BindAmbiguous
		for v := range pinned {
			b.Candidates = append(b.Candidates, v)
		}
		sortVersionStrings(b.Candidates)
		var parts []string
		for _, v := range b.Candidates {
			parts = append(parts, fmt.Sprintf("%s in %s", v, strings.Join(pinned[v], ", ")))
		}
		if len(unpinned) > 0 {
			parts = append(parts, "no entry in "+strings.Join(unpinned, ", "))
		}
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("%d roots reach this module and do not all pin one %s version (%s); no root was chosen", len(b.Roots), b.Source, strings.Join(parts, "; ")))
	case len(pinned) == 1:
		for v := range pinned {
			b.Version = v
		}
		if consErr != nil {
			b.Disclosures = append(b.Disclosures, "the version constraint is unreadable ("+consErr.Error()+"); the lockfile pin was used instead")
		}
		if consErr == nil {
			if pv, err := ParseIaCVersion(b.Version); err == nil && !cons.Matches(pv) {
				b.Disclosures = append(b.Disclosures, fmt.Sprintf("the lockfile pins %s, which the constraint %q does not admit; OpenTofu would stop at init until the constraint or the lock changes", b.Version, constraint))
			}
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
		if consErr != nil {
			b.Status = BindUnbound
			b.Disclosures = append(b.Disclosures, fmt.Sprintf("the version constraint %q is unreadable (%v), so no release can be shown to satisfy it", constraint, consErr))
			return b
		}
		b.constraintMatch(q.Catalogs, cons, len(b.Roots))
	}
	return b
}

// lockVersion reads the version a lockfile pins for source. An implied host matches the lock key on either default
// registry, since OpenTofu writes registry.opentofu.org and Terraform registry.terraform.io for the same shorthand;
// an explicit host matches exactly. Keys are lower case. Two default-registry keys that pin different versions are
// a conflict the caller reports as ambiguous.
func lockVersion(lock map[string]string, source string, implied bool) (version, host, conflict string) {
	if !implied {
		return lock[source], "", ""
	}
	ns := nsName(source)
	var vs, hs []string
	for _, h := range []string{defaultRegistryHost, openTofuRegistryHost} {
		if v := lock[h+"/"+ns]; v != "" {
			vs, hs = append(vs, v), append(hs, h)
		}
	}
	switch {
	case len(vs) == 0:
		return "", "", ""
	case len(vs) == 2 && vs[0] != vs[1]:
		return "", "", fmt.Sprintf("the lockfile pins %s at %s on %s and at %s on %s", ns, vs[0], defaultRegistryHost, vs[1], openTofuRegistryHost)
	case len(vs) == 2:
		return vs[0], "", "" // both registries pin one version: neither host is the block's
	}
	return vs[0], hs[0], ""
}

func (b *IaCBinding) noteHost(cat *IaCCatalogInfo) {
	want := hostOf(b.Source)
	if got := hostOf(cat.Manifest.Source); got != want {
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("the catalog was built from %s and the block resolves to %s; the two registries' builds of a release read equal at the versions measured (IF-P2), so the facts carry the catalog's address", got, want))
	}
}

// constraintMatch binds a module no lockfile pins: the highest installed release the constraint admits. roots counts
// the lockfile roots that reach the module without an entry for the provider. Among equal versions a catalog built
// from the block's own registry host wins.
func (b *IaCBinding) constraintMatch(catalogs []IaCCatalogInfo, cons IaCConstraint, roots int) {
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
			cmp := 1
			if best != "" {
				cmp = CompareIaCVersions(v, bestV)
			}
			ownHost := hostOf(c.Manifest.Source) == hostOf(b.Source)
			if cmp > 0 || (cmp == 0 && ownHost && hostOf(bestCat.Manifest.Source) != hostOf(b.Source)) {
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
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("no installed catalog holds %s of %s%s", what, b.Source, addLineNote(b.AddLine))+hostNote(catalogs, b.Source))
		return
	}
	b.Status, b.Version, b.Catalog = BindConstraintMatched, best, bestCat.Name
	b.noteHost(bestCat)
	reach := "no lockfile reaches this module"
	switch {
	case roots == 1:
		reach = fmt.Sprintf("1 lockfile root reaches this module but it does not pin %s", b.Source)
	case roots > 1:
		reach = fmt.Sprintf("%d lockfile roots reach this module but none pins %s", roots, b.Source)
	}
	if cons.Raw == "" {
		b.Disclosures = append(b.Disclosures, reach+" and it states no version constraint: the highest installed release was used, which no root has been shown to install")
	} else {
		b.Disclosures = append(b.Disclosures, fmt.Sprintf("%s: the highest installed release satisfying %s was used, which no root has been shown to install", reach, cons.Raw))
	}
}

// hostNote names a catalog of the same provider that was set aside because its registry host cannot serve the block's.
func hostNote(catalogs []IaCCatalogInfo, source string) string {
	for i := range catalogs {
		c := &catalogs[i]
		if nsName(c.Manifest.Source) == nsName(source) && !sameProvider(c.Manifest.Source, source) {
			return fmt.Sprintf("; catalog %s was built from %s, and the block's registry host %s is not one of the two default registries it may stand in for", c.Name, hostOf(c.Manifest.Source), hostOf(source))
		}
	}
	return ""
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
	return fmt.Sprintf("%s %s has no installed catalog%s%s; no nearest version is used", source, version, addLineNote(line), hostNote(catalogs, source))
}

func nonEmpty(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return []string{s}
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

const (
	defaultRegistryHost  = "registry.terraform.io"
	openTofuRegistryHost = "registry.opentofu.org"
)

// normalizeProviderSource expands a required_providers source to a lower case host/namespace/name. With none,
// Terraform implies hashicorp/<local name>. Addresses compare lower case.
func normalizeProviderSource(src, local string) string {
	src = strings.ToLower(strings.TrimSpace(src))
	if src == "" {
		return defaultRegistryHost + "/hashicorp/" + strings.ToLower(local)
	}
	switch parts := strings.Split(src, "/"); len(parts) {
	case 1:
		return defaultRegistryHost + "/hashicorp/" + parts[0]
	case 2:
		return defaultRegistryHost + "/" + src
	}
	return src
}

// providerHostImplied reports that a required_providers source names no registry host.
func providerHostImplied(src string) bool { return strings.Count(strings.TrimSpace(src), "/") < 2 }

// hostOf is the registry host of a provider address; an address without one is on the default registry.
func hostOf(source string) string {
	if parts := strings.Split(strings.ToLower(source), "/"); len(parts) >= 3 {
		return parts[0]
	}
	return defaultRegistryHost
}

// hostsInterchange reports whether a catalog built from one registry host may stand in for another's. Only the two
// default registries do, on the IF-P2 reading that their builds of a release are equal. Any other host needs a
// catalog for exactly that host.
func hostsInterchange(a, b string) bool {
	if a == b {
		return true
	}
	return (a == defaultRegistryHost || a == openTofuRegistryHost) && (b == defaultRegistryHost || b == openTofuRegistryHost)
}

// sameProvider compares namespace and name and requires the registry hosts to interchange (design section 5).
func sameProvider(a, b string) bool {
	return nsName(a) == nsName(b) && nsName(a) != "" && hostsInterchange(hostOf(a), hostOf(b))
}

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
