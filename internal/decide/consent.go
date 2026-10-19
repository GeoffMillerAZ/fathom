package decide

// consent.go — the decision port's consent and kill (KS-1 to KS-6,
// docs/plans/decision-port.md section I; ADR-073 decision 5; I66). One gate
// with one precedence closes the port before any key read, exec or dial:
// the environment (FATHOM_DECIDE=off, read once per process at Open), the
// kill file (~/.fathom/decide.off, stat-ed before each ask, never read),
// the user-only [decide] enabled (re-checked before each ask, the user
// config reloaded only when its mtime changed), the user's consent to a
// provider only the repository names (allow_repo_provider), then, per site,
// deny_sites, then the site's miss kill (KS-12, sitekill.go), then sites
// and session_sites. The first source that closes it is returned and named
// (Gate), recorded in decisions.jsonl (kill_source, consent_layer),
// disclosed once and shown by doctor. Before each ask the port then counts
// the ask against the per-site cap (KS-10, caps.go).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// KillEnv is the variable whose exact value KillValue closes the port in
// every process whose environment carries it (KS-1). Any other value is
// ignored and named (Gate.IgnoredEnv).
const (
	KillEnv   = "FATHOM_DECIDE"
	KillValue = "off"
)

// KillFileName is the kill file under the per-user fathom directory
// (fathomhome: $FATHOM_HOME, else ~/.fathom): present means closed (KS-2).
const KillFileName = "decide.off"

// The closing sources (Gate.Source, decisions.jsonl kill_source), in KS-6's
// precedence order.
const (
	KillSourceEnv          = "env"
	KillSourceFile         = "file"
	KillSourceEnabled      = "enabled"
	KillSourceConsent      = "consent"
	KillSourceDeny         = "deny_sites"
	KillSourceSites        = "sites"
	KillSourceSessionSites = "session_sites"
)

// consentRefused is the consent refusal's own words (KS-4).
const consentRefused = "consent: provider named by the repository, not allowed by the user"

// Gate is the port's verdict: open, or closed by the first source in
// precedence order, named.
type Gate struct {
	Closed bool
	// Source is the first closing source (KillSource*), "" when open.
	Source string
	// Layer is where that source lives: "environment", "file", "user",
	// "project" or "default"; "log" for a site's miss kill (decisions.jsonl)
	// and "context" for a spent ceiling.
	Layer string
	// Reason is one line naming the source and the way to change it.
	Reason string
	// ConsentLayer is the layer whose consent opens a configured
	// provider: "user" (the user layer named it), "project" (only the
	// repository named it and the user's allow_repo_provider allowed it),
	// or "" (provider none, or a Decide built in code).
	ConsentLayer string
	// IgnoredEnv is a FATHOM_DECIDE value other than KillValue, which
	// closes nothing (doctor names it); "" when unset or KillValue.
	IgnoredEnv string
}

// gateInput is what portGate reads; the callers gather it.
type gateInput struct {
	env             string // FATHOM_DECIDE's value, "" when unset
	killFile        string // the kill file's path, for the Reason
	killFilePresent bool
	disabled        bool   // the user layer's enabled = false
	disabledWhy     string // the Reason for disabled, "" for the default
	cfg             config.Decide
	site            string // "" for the port as a whole
	siteKill        Gate   // the site's miss verdict (KS-12), open when none
}

// portGate is the one gate (KS-6): the environment, the kill file,
// enabled, consent, then for a site deny_sites, then its miss kill
// (KS-12), then sites and session_sites. It reads nothing itself.
func portGate(in gateInput) Gate {
	g := Gate{}
	if in.env != "" && in.env != KillValue {
		g.IgnoredEnv = in.env
	}
	closed := func(source, layer, reason string) Gate {
		g.Closed, g.Source, g.Layer, g.Reason = true, source, layer, reason
		return g
	}
	switch {
	case in.env == KillValue:
		return closed(KillSourceEnv, "environment", KillEnv+"=off in this process's environment")
	case in.killFilePresent:
		return closed(KillSourceFile, "file", fmt.Sprintf("the kill file %s is present", displayKillFile(in.killFile)))
	case in.disabled:
		why := in.disabledWhy
		if why == "" {
			why = "[decide] enabled = false in the user config (~/.fathom/config.toml)"
		}
		return closed(KillSourceEnabled, "user", why)
	}
	layer, ok := providerConsented(in.cfg)
	if !ok {
		return closed(KillSourceConsent, "project", consentRefused+" (set [decide] allow_repo_provider = true in ~/.fathom/config.toml to allow it)")
	}
	g.ConsentLayer = layer
	if in.site == "" {
		return g
	}
	sg := siteGate(in.cfg, in.site, in.siteKill)
	sg.ConsentLayer, sg.IgnoredEnv = g.ConsentLayer, g.IgnoredEnv
	return sg
}

// siteGate is the site half of portGate (KS-5, I60 as amended by ADR-073):
// deny_sites refuses whatever any list says; then kill, the site's miss
// verdict (KS-12), when closed; a session site opens only from
// session_sites; an opt-in site only from the repository's own sites
// (JF-4); every other site from sites.
func siteGate(cfg config.Decide, site string, kill Gate) Gate {
	closed := func(source, layer, reason string) Gate {
		return Gate{Closed: true, Source: source, Layer: layer, Reason: reason}
	}
	switch {
	case slices.Contains(cfg.DenySites, site):
		return closed(KillSourceDeny, "user", fmt.Sprintf("site %s is in [decide] deny_sites", site))
	case kill.Closed:
		return kill
	case IsSessionSite(site):
		if slices.Contains(cfg.SessionSites, site) {
			return Gate{}
		}
		return closed(KillSourceSessionSites, "user", fmt.Sprintf("session site %s is not in [decide] session_sites (~/.fathom/config.toml)", site))
	case IsOptInSite(site) && !slices.Contains(cfg.RepoSites, site):
		return closed(KillSourceSites, "project", fmt.Sprintf("opt-in site %s is not in the repository's own [decide] sites", site))
	case !slices.Contains(cfg.Sites, site):
		return closed(KillSourceSites, sitesLayer(cfg), fmt.Sprintf("site %s is not in [decide] sites", site))
	}
	return Gate{}
}

// sitesLayer names the layer whose [decide] sites is in effect.
func sitesLayer(cfg config.Decide) string {
	if cfg.SitesSource == "" {
		return "default"
	}
	return string(cfg.SitesSource)
}

// providerConsented reports the layer whose consent opens cfg's provider
// (KS-4, I66): the user layer when it named the provider and the
// transport in effect; the repository's when only it did and the user's
// allow_repo_provider allows it; refused otherwise. Provider none needs no
// consent, and a Decide built in code (no layer recorded) is its caller's
// own, as checkKeyTransport already treats it.
func providerConsented(cfg config.Decide) (string, bool) {
	switch cfg.Provider {
	case "", "none":
		return "", true
	}
	if cfg.ProviderSource == "" && cfg.TransportSource == "" {
		return "", true
	}
	if userNamedProvider(cfg) {
		return "user", true
	}
	if cfg.AllowRepoProvider {
		return "project", true
	}
	return "project", false
}

// userNamedProvider reports whether the user layer named cfg's provider and
// the transport in effect (its own, or the provider's default).
func userNamedProvider(cfg config.Decide) bool {
	if cfg.UserProvider != cfg.Provider {
		return false
	}
	user := config.Decide{Provider: cfg.UserProvider, Transport: cfg.UserTransport}
	return user.EffectiveTransport() == cfg.EffectiveTransport()
}

// KillFilePath is the kill file's path ("" when no per-user directory
// resolves).
func KillFilePath() string { return fathomhome.Path(KillFileName) }

// displayKillFile names the kill file for a line.
func displayKillFile(path string) string {
	if path == "" {
		return "~/.fathom/" + KillFileName
	}
	return path
}

// killFilePresent stats path, never reads it: present unless the stat says
// it does not exist, so an unreadable kill file, or one whose directory
// refuses a stat, still closes the port (fail closed).
func killFilePresent(stat func(string) (fs.FileInfo, error), path string) bool {
	if path == "" {
		return false
	}
	_, err := stat(path)
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}

// Check is the gate as this process sees it now (doctor, a CLI verb, the
// tests): FATHOM_DECIDE, the kill file, cfg's enabled and consent, and for
// a site ("" for the port) its lists. It reads no key, execs nothing and
// dials nothing.
func Check(cfg config.Decide, site string, opts ...Option) Gate {
	return CheckSite(cfg, site, nil, opts...)
}

// CheckSite is Check with rec's miss verdict for site (KS-12), in the one
// precedence after deny_sites: doctor seeds rec from decisions.jsonl's
// tail. A nil rec judges no site.
func CheckSite(cfg config.Decide, site string, rec *Recorder, opts ...Option) Gate {
	o := resolveOptions(opts)
	return portGate(gateInput{
		env:             os.Getenv(KillEnv),
		killFile:        o.killFile,
		killFilePresent: killFilePresent(o.stat, o.killFile),
		disabled:        cfg.Disabled,
		cfg:             cfg,
		site:            site,
		siteKill:        rec.siteKill(site),
	})
}

// KilledError is what a site's Decide returns when a live kill closed an
// open port (KS-2), or the asking site's miss kill (KS-12), its ask cap or
// a spent ceiling (KS-10) refused the ask: it is decision.ErrNoProvider, so
// every site takes its deterministic path, and RecordError writes its
// kill_source.
type KilledError struct{ Gate Gate }

func (e *KilledError) Error() string {
	return fmt.Sprintf("decision port %s (kill_source %s, %s): %s", closedVerb(e.Gate.Source), e.Gate.Source, e.Gate.Layer, e.Gate.Reason)
}

// Unwrap makes a KilledError decision.ErrNoProvider.
func (e *KilledError) Unwrap() error { return decision.ErrNoProvider }

// liveGate re-checks, before each ask, the two kills that reach a port
// already open (KS-2): the kill file (one stat) and the user layer's
// enabled (one stat of the user config, and a reload only when its mtime
// or size changed). The environment and consent were settled at Open.
type liveGate struct {
	env      string
	killFile string
	userPath string
	stat     func(string) (fs.FileInfo, error)
	cfg      config.Decide
	w        io.Writer
	rec      *Recorder        // the site miss verdicts (KS-12); nil judges none
	now      func() time.Time // the clock a ceiling is read against (KS-10)

	mu          sync.Mutex
	userMod     time.Time
	userSize    int64
	userExists  bool
	disabled    bool
	disabledWhy string
	disclosed   map[disclosure]bool
	// siteDisclosed holds the site-level lines (the miss kill, the cap),
	// which hold for the process or the session and so never clear;
	// asks is each scope's asks the cap counted (KS-10).
	siteDisclosed map[disclosure]bool
	asks          map[Scope]int
}

// disclosure is one `killed` line's key: the closing source and who asked
// (KS-14), so each session and site hears of a kill once while it holds.
type disclosure struct {
	source string
	scope  Scope
}

func newLiveGate(cfg config.Decide, env string, o openOptions, rec *Recorder) *liveGate {
	g := &liveGate{env: env, killFile: o.killFile, userPath: o.userConfig, stat: o.stat, cfg: cfg, w: o.disclose, rec: rec, now: time.Now, disabled: cfg.Disabled,
		disclosed: map[disclosure]bool{}, siteDisclosed: map[disclosure]bool{}, asks: map[Scope]int{}}
	if g.userPath != "" {
		if info, err := g.stat(g.userPath); err == nil {
			g.userExists, g.userMod, g.userSize = true, info.ModTime(), info.Size()
		}
	}
	return g
}

// check is the gate before one ask from scope; a closed verdict is
// disclosed once per closing source, session and site (KS-14) until the
// port opens again.
func (g *liveGate) check(scope Scope) Gate {
	present := killFilePresent(g.stat, g.killFile)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.refreshUserLocked()
	v := portGate(gateInput{env: g.env, killFile: g.killFile, killFilePresent: present, disabled: g.disabled, disabledWhy: g.disabledWhy, cfg: g.cfg})
	if !v.Closed {
		clear(g.disclosed)
		return v
	}
	g.discloseLocked(g.disclosed, v, scope, "every site")
	return v
}

// admit is the gate before one ask from ctx's scope: the port's live kills
// (check), then the asking site's miss kill (KS-12), then a ceiling the
// path's joins spent (KS-10), then the site's ask cap (allowAsk, KS-10).
// A site-level verdict is disclosed once per closing source, session and
// site, and never clears: the miss kill holds for the process, the cap for
// the session.
func (g *liveGate) admit(ctx context.Context) Gate {
	scope := ScopeFrom(ctx)
	if v := g.check(scope); v.Closed {
		return v
	}
	kill := g.rec.siteKill(scope.Site)
	g.mu.Lock()
	defer g.mu.Unlock()
	if kill.Closed {
		g.discloseLocked(g.siteDisclosed, kill, scope, "the site")
		return kill
	}
	if ceiling, ok := ctx.Deadline(); ok {
		if _, left := askDeadline(ceiling, g.now(), 0); !left {
			return Gate{Closed: true, Source: KillSourceDeadline, Layer: "context", Reason: "the path's ceiling was spent before the ask"}
		}
	}
	v := allowAsk(g.asks, scope, g.cfg)
	if v.Closed {
		g.discloseLocked(g.siteDisclosed, v, scope, "the site")
	}
	return v
}

// discloseLocked writes v's one line for scope unless seen already holds
// it (KS-14): `killed` for a kill, `capped` for the ask cap; who names
// what answers on its deterministic path. Callers hold g.mu.
func (g *liveGate) discloseLocked(seen map[disclosure]bool, v Gate, scope Scope, who string) {
	key := disclosure{source: v.Source, scope: scope}
	if seen[key] {
		return
	}
	seen[key] = true
	asker := ""
	if s := scope.String(); s != "" {
		asker = " for " + s
	}
	fmt.Fprintf(g.w, "warn: decision port %s (kill_source %s, %s)%s: %s; %s answers on its deterministic path\n", closedVerb(v.Source), v.Source, v.Layer, asker, v.Reason, who)
}

// refreshUserLocked reloads the user layer's enabled when the user config's
// mtime or size moved (or it appeared or went). A user config that no
// longer loads closes the port (fail closed) until it loads again.
func (g *liveGate) refreshUserLocked() {
	if g.userPath == "" {
		return
	}
	info, err := g.stat(g.userPath)
	exists := err == nil
	var mod time.Time
	var size int64
	if exists {
		mod, size = info.ModTime(), info.Size()
	}
	if exists == g.userExists && mod.Equal(g.userMod) && size == g.userSize {
		return
	}
	g.userExists, g.userMod, g.userSize = exists, mod, size
	g.disabledWhy = ""
	if !exists {
		g.disabled = false
		return
	}
	loaded, err := config.Load(g.userPath, "")
	if err != nil {
		g.disabled, g.disabledWhy = true, fmt.Sprintf("the user config %s no longer loads (%v), so the port stays closed until it does", g.userPath, err)
		return
	}
	g.disabled = loaded.Config.Decide.Disabled
	if g.disabled {
		g.disabledWhy = fmt.Sprintf("[decide] enabled = false in %s", g.userPath)
	}
}

// killable is the port a site asks once Open opened it: the live gate and
// the site's own rails (admit), then the composed provider.
type killable struct {
	inner decision.Decider
	gate  *liveGate
}

func (k killable) Name() string { return k.inner.Name() }

func (k killable) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	if v := k.gate.admit(ctx); v.Closed {
		return decision.Response{Provider: k.inner.Name()}, &KilledError{Gate: v}
	}
	return k.inner.Decide(ctx, req)
}

// Capabilities declares what the composed provider declares.
func (k killable) Capabilities() decision.Capabilities { return decision.CapabilitiesOf(k.inner) }
