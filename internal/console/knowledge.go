package console

// knowledge.go — the one read behind the Knowledge pages (vision 35 §3:
// semantic layers, packs, the serving explainer): every indexed service's
// claims and compile runs, the store-wide claim events, and the layer-pack
// catalog each service's root resolves (embedded ∪ that root's
// .fathom/layers). The strata, the coverage matrix and the layer page are
// folds over this one read, so they cannot disagree about a count (OC-9's
// rule, applied within the Knowledge group).
//
// Every figure keeps its truth state: a service whose claims could not be
// read is an error that names the failure, never a zero; a layer nothing
// ever compiled is absent ("not compiled"), never 0.

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// KnowledgeSource is what the Knowledge projections read.
type KnowledgeSource struct {
	// Store is the fleet store — the daemon's own inside serve-http, a
	// read-only mount of the daemon's store for a CLI verb.
	Store store.Store
	// Layers lists the layer packs a repository root resolves (embedded ∪
	// root/.fathom/layers). nil is semantic.ListLayersIn.
	Layers func(root string) ([]semantic.LayerInfo, error)
	// EdgeTally tallies one service's typed call edges; nil is
	// TallyTypedEdges over Store. The daemon caches it between ingests.
	EdgeTally func(ctx context.Context, svc string) (EdgeTally, error)
	// StorePath is the store's file path, named in the remedy commands the
	// projections print: resuming a compile runs in direct --store mode
	// (`fathom compile --continue` refuses without --store). "" prints a
	// placeholder the reader fills in.
	StorePath string
}

func (k KnowledgeSource) layers(root string) ([]semantic.LayerInfo, error) {
	if k.Layers != nil {
		return k.Layers(root)
	}
	return semantic.ListLayersIn(root)
}

func (k KnowledgeSource) edgeTally(ctx context.Context, svc string) (EdgeTally, error) {
	if k.EdgeTally != nil {
		return k.EdgeTally(ctx, svc)
	}
	return TallyTypedEdges(ctx, k.Store, svc)
}

// storeSource names the store a knowledge figure was read from.
const storeSource = "the fleet store (semantic_claims, compile_runs, claim_events)"

// serviceKnowledge is one service's slice of the read.
type serviceKnowledge struct {
	Service string
	Root    string
	Claims  []core.SemanticClaim
	Runs    []core.CompileRun // newest first
	// Err is non-empty when the service's claims or runs could not be read.
	Err string
}

// packInfo is one layer pack as the fleet resolves it.
type packInfo struct {
	semantic.LayerInfo
	// InstalledIn names the services whose root carries this pack as a
	// project pack (empty for an embedded-only pack).
	InstalledIn []string
}

// knowledge is the one read.
type knowledge struct {
	Services []serviceKnowledge
	Packs    map[string]*packInfo
	// PackErr names roots whose pack catalog could not be listed;
	// PackUnreadable is the services those roots belong to.
	PackErr        []string
	PackUnreadable map[string]bool
	Events         []core.ClaimEvent
	// EventsErr is non-empty when claim_events could not be read.
	EventsErr string
}

// eventsUntil bounds the store-wide claim-event read without a clock: the
// read is "every event", deterministically. (The store compares UnixNano,
// so the bound is the largest instant that still has one.)
var eventsUntil = time.Unix(0, math.MaxInt64).UTC()

// readKnowledge reads every service's claims and runs, the claim events
// and the pack catalog.
func readKnowledge(ctx context.Context, src KnowledgeSource) (knowledge, error) {
	st := src.Store
	var k knowledge
	ids, err := st.ListServiceIDs(ctx)
	if err != nil {
		return k, err
	}
	claimIDs, err := st.ClaimServiceIDs(ctx)
	if err != nil {
		return k, err
	}
	seen := map[string]bool{}
	var svcs []string
	for _, id := range append(ids, claimIDs...) {
		if id != "" && !seen[id] {
			seen[id] = true
			svcs = append(svcs, id)
		}
	}
	sort.Strings(svcs)
	for _, svc := range svcs {
		sk := serviceKnowledge{Service: svc}
		if meta, found, merr := st.GetServiceMeta(ctx, svc); merr == nil && found {
			sk.Root = meta.Path
		}
		var errs []string
		if cl, cerr := st.ClaimsForService(ctx, svc, ""); cerr != nil {
			errs = append(errs, "claims: "+cerr.Error())
		} else {
			sk.Claims = cl
		}
		if runs, rerr := st.CompileRuns(ctx, svc, ""); rerr != nil {
			errs = append(errs, "compile runs: "+rerr.Error())
		} else {
			sk.Runs = runs
		}
		sk.Err = strings.Join(errs, "; ")
		k.Services = append(k.Services, sk)
	}
	if evs, eerr := st.ClaimEventsSince(ctx, time.Unix(0, 0).UTC(), eventsUntil); eerr != nil {
		k.EventsErr = eerr.Error()
	} else {
		k.Events = evs
	}
	k.Packs, k.PackErr, k.PackUnreadable = readPacks(src, k.Services)
	return k, nil
}

// readPacks resolves the pack catalog: the embedded catalog once, plus the
// project packs of every service root that has a .fathom/layers directory.
//
// semantic's union loader treats an unreadable .fathom/layers directory as
// "embedded packs only" with nothing but a once-per-process stderr warning
// (its serve-path resilience contract: one bad directory must not disable
// serving). A catalog view cannot take that silence: the root's project
// packs would vanish from every band, count and table while each read as
// complete. So readPacks lists the directory itself first, and an
// unreadable one is a named error — the service's catalog is unknown, never
// "embedded only".
func readPacks(src KnowledgeSource, svcs []serviceKnowledge) (map[string]*packInfo, []string, map[string]bool) {
	packs := map[string]*packInfo{}
	var errs []string
	unreadable := map[string]bool{}
	embedded, err := src.layers("")
	if err != nil {
		errs = append(errs, "embedded catalog: "+err.Error())
	}
	for _, li := range embedded {
		packs[li.Name] = &packInfo{LayerInfo: li}
	}
	for _, s := range svcs {
		if s.Root == "" {
			continue
		}
		dir := filepath.Join(s.Root, filepath.FromSlash(semantic.ProjectLayersDir))
		fi, serr := os.Stat(dir)
		switch {
		case errors.Is(serr, fs.ErrNotExist):
			// No .fathom/layers: embedded packs only, by design, and
			// whatever a pinned tech pack ships (FD-18). An unreadable
			// catalog here is not a project pack's breakage.
			if infos, lerr := src.layers(s.Root); lerr == nil {
				addPacks(packs, s.Service, infos)
			}
			continue
		case serr != nil:
			errs = append(errs, s.Service+": "+dir+" could not be read ("+serr.Error()+"); its project packs are not listed")
			unreadable[s.Service] = true
			continue
		case !fi.IsDir():
			continue
		}
		if _, rerr := os.ReadDir(dir); rerr != nil {
			errs = append(errs, s.Service+": "+dir+" could not be read ("+rerr.Error()+"); its project packs are not listed")
			unreadable[s.Service] = true
			continue
		}
		infos, lerr := src.layers(s.Root)
		if lerr != nil {
			errs = append(errs, s.Service+": "+lerr.Error())
			unreadable[s.Service] = true
			continue
		}
		addPacks(packs, s.Service, infos)
	}
	return packs, errs, unreadable
}

// PackSource names where a layer pack comes from.
const (
	PackEmbedded = "embedded" // fathom's own catalog
	PackProject  = "project"  // a repository's .fathom/layers
	PackKit      = "kit"      // a project pack installed from a kit's fathom-pack
	PackTech     = "tech"     // a layer a pinned tech pack ships (FD-18)
	PackOrphan   = "orphan"   // claims in the store, no pack defines the layer
	// PackUnknown is a layer with claims or runs in the store whose pack
	// could not be looked up: the catalog of a project holding it was
	// unreadable, so whether a pack defines it is not known.
	PackUnknown = "unknown"
)

// broken reports a project-only pack that failed to load: its name is all
// that is known — no kind, stratum or role.
func (p *packInfo) broken() bool {
	return p != nil && p.LoadError != "" && p.Source == semantic.SourceProject
}

// packSource classifies layer name's pack. A layer no readable catalog
// defines is an orphan only when every project holding it had its catalog
// read; otherwise it is unknown, naming the unreadable catalogs.
func (k knowledge) packSource(name string) (source, origin string) {
	p := k.Packs[name]
	if p == nil {
		if holders := k.unreadableHolders(name); len(holders) > 0 {
			return PackUnknown, "no readable pack defines this layer, but the pack catalog of " + strings.Join(holders, ", ") + " could not be read — a project pack there may define it"
		}
	}
	return packSourceOf(p)
}

// unreadableHolders names the projects holding claims or compile runs of
// layer whose pack catalog could not be read.
func (k knowledge) unreadableHolders(layer string) []string {
	var out []string
	for _, s := range k.Services {
		if !k.PackUnreadable[s.Service] {
			continue
		}
		held := false
		for _, c := range s.Claims {
			if c.Layer == layer {
				held = true
				break
			}
		}
		for _, r := range s.Runs {
			if held {
				break
			}
			held = r.Layer == layer
		}
		if held {
			out = append(out, s.Service)
		}
	}
	return out
}

// packSourceOf classifies a resolved pack.
func packSourceOf(p *packInfo) (source, origin string) {
	if p == nil {
		return PackOrphan, "no embedded or project pack defines this layer; its claims keep serving until refuted or recompiled"
	}
	if p.Source == semantic.SourceEmbedded {
		return PackEmbedded, "fathom's embedded catalog"
	}
	if p.Source == semantic.SourceTech {
		return PackTech, "shipped by tech pack " + p.TechPack + ", pinned by the repository"
	}
	if strings.HasPrefix(p.PromptFile, "fathom-pack/") {
		return PackKit, "installed from a kit's fathom-pack (" + p.PromptFile + ")"
	}
	return PackProject, "a repository's own .fathom/layers"
}

// StatusCounts counts claims by lifecycle status.
type StatusCounts struct {
	Fresh        int `json:"fresh"`
	Stale        int `json:"stale"`
	Contradicted int `json:"contradicted"`
	Superseded   int `json:"superseded"`
	Refuted      int `json:"refuted"`
}

// Current is the claims serving can still use (fresh, stale, contradicted).
func (s StatusCounts) Current() int { return s.Fresh + s.Stale + s.Contradicted }

func (s *StatusCounts) add(st core.ClaimStatus) {
	switch st {
	case core.ClaimFresh:
		s.Fresh++
	case core.ClaimStale:
		s.Stale++
	case core.ClaimContradicted:
		s.Contradicted++
	case core.ClaimSuperseded:
		s.Superseded++
	case core.ClaimRefuted:
		s.Refuted++
	}
}

// realRun reports whether a compile run spent and wrote: neither a dry-run
// estimate nor a refusal.
func realRun(r core.CompileRun) bool { return !r.DryRun && !r.Refused }

// layerNames is every layer the fleet knows: the pack catalog, and every
// layer with claims or compile runs in the store (orphans included).
func (k knowledge) layerNames() []string {
	seen := map[string]bool{}
	for name := range k.Packs {
		seen[name] = true
	}
	for _, s := range k.Services {
		for _, c := range s.Claims {
			seen[c.Layer] = true
		}
		for _, r := range s.Runs {
			seen[r.Layer] = true
		}
	}
	delete(seen, "")
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// cellFacts is one (service, layer) intersection.
type cellFacts struct {
	Statuses  StatusCounts
	Runs      int // real compile runs
	DryRuns   int
	Refusals  int
	LastRun   time.Time
	LastRunID string
	CostUSD   float64
	Emitted   int
	Rejected  int
}

func (k knowledge) cell(s serviceKnowledge, layer string) cellFacts {
	var c cellFacts
	for _, cl := range s.Claims {
		if cl.Layer == layer {
			c.Statuses.add(cl.Status)
		}
	}
	for _, r := range s.Runs {
		if r.Layer != layer {
			continue
		}
		switch {
		case r.DryRun:
			c.DryRuns++
			continue
		case r.Refused:
			c.Refusals++
			continue
		}
		c.Runs++
		c.CostUSD += r.CostUSD
		c.Emitted += r.ClaimsEmitted
		c.Rejected += r.ClaimsRejected
		if r.RanAt.After(c.LastRun) {
			c.LastRun, c.LastRunID = r.RanAt, r.RunID
		}
	}
	return c
}

// cellClaims is a (service, layer) cell's current-claims value: claims when
// any are current, a measured zero when a real compile ran and none are,
// and absent — "not compiled", never 0 — when nothing ever compiled it.
func cellClaims(s serviceKnowledge, c cellFacts) Value[int] {
	if s.Err != "" {
		return Errored[int](s.Service + ": " + s.Err).WithSource(storeSource)
	}
	cur := c.Statuses.Current()
	denom := "current claims (fresh, stale, contradicted)"
	switch {
	case cur > 0:
		v := Present(cur, storeSource).WithDenominator(denom)
		if !c.LastRun.IsZero() {
			v = v.WithAsOf(c.LastRun)
		}
		return v
	case c.Runs > 0:
		why := "compiled, no current claims"
		if n := c.Statuses.Superseded + c.Statuses.Refuted; n > 0 {
			why = "compiled; every claim since superseded or refuted"
		}
		return Measured(0, storeSource).WithDenominator(why).WithAsOf(c.LastRun)
	case c.Statuses.Superseded+c.Statuses.Refuted > 0:
		return Measured(0, storeSource).WithDenominator("no current claims; every claim superseded or refuted")
	case c.DryRuns > 0 || c.Refusals > 0:
		return Absent[int]("not compiled — only dry-run estimates or refused runs recorded").WithSource(storeSource)
	}
	return Absent[int]("not compiled").WithSource(storeSource)
}

// rfc3339 renders a time for JSON, "" for the zero time.
func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// roundCents keeps a dollar figure to the cent, so a sum of float costs is
// the same bytes from every reader.
func roundCents(v float64) float64 {
	if v < 0 {
		return -roundCents(-v)
	}
	return float64(int64(v*100+0.5)) / 100
}

// ResumeCompile is the command that resumes layer's budget-stopped or
// incomplete compile for service from its first uncompiled section.
// `fathom compile --continue` runs in direct --store mode only (it refuses
// without --store), and compile only estimates unless --dry-run=false, so
// the remedy names both; an unknown store path is a placeholder.
func ResumeCompile(layer, service, storePath string) string {
	st := "<store>"
	if storePath != "" {
		st = shellQuote(storePath)
	}
	return "fathom compile --layer " + shellQuote(layer) + " --service " + shellQuote(service) + " --store " + st + " --continue --dry-run=false"
}

// shellQuote quotes a value for a copyable command when it needs it.
func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`*?[]{}()<>|&;#~!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// addPacks records the project and tech layers of one service's catalog. The
// first root (by service name) that carries a pack defines it; an embedded
// pack stays the fleet's definition, and the roots overriding it are named
// in InstalledIn.
func addPacks(packs map[string]*packInfo, service string, infos []semantic.LayerInfo) {
	for _, li := range infos {
		if li.Source != semantic.SourceProject && li.Source != semantic.SourceTech {
			continue
		}
		p := packs[li.Name]
		if p == nil {
			p = &packInfo{LayerInfo: li}
			packs[li.Name] = p
		}
		p.InstalledIn = append(p.InstalledIn, service)
	}
}
