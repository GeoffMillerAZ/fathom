package decide

// scope.go — KS-14 (O1131, ADR-073 decision 6: a miss is disclosed once per
// session, site and class). A live kill's `killed` line is keyed by who
// asks: the session and the site, carried on the ask's context because
// decision.Request names neither. serve-http's /rpc sets the session from
// X-Fathom-Caller (rpcContext), /route sets both, and the client's re-rank
// and pre-filter set their site (AtSite). serve-mcp and the CLI verbs set
// nothing: the process is the session, and each verb asks one site.

import (
	"context"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// Scope is who asks the decision port: the session and the site. The zero
// Scope is the process.
type Scope struct {
	Session string
	Site    string
}

// String names the scope for a disclosure line, "" for the process.
func (s Scope) String() string {
	switch {
	case s.Site != "" && s.Session != "":
		return "site " + s.Site + ", session " + s.Session
	case s.Site != "":
		return "site " + s.Site
	case s.Session != "":
		return "session " + s.Session
	}
	return ""
}

type scopeKey struct{}

// ScopeFrom is the Scope ctx carries, the zero Scope when none.
func ScopeFrom(ctx context.Context) Scope {
	if ctx == nil {
		return Scope{}
	}
	s, _ := ctx.Value(scopeKey{}).(Scope)
	return s
}

// WithSession is ctx asking as session, its site kept.
func WithSession(ctx context.Context, session string) context.Context {
	s := ScopeFrom(ctx)
	s.Session = session
	return context.WithValue(ctx, scopeKey{}, s)
}

// WithSite is ctx asking from site, its session kept.
func WithSite(ctx context.Context, site string) context.Context {
	s := ScopeFrom(ctx)
	s.Site = site
	return context.WithValue(ctx, scopeKey{}, s)
}

// AtSite is d asked from site: every ask's context carries site. A nil d
// and decision.None stay as they are, since callers type-check None to
// mean no provider.
func AtSite(d decision.Decider, site string) decision.Decider {
	if d == nil {
		return nil
	}
	if _, isNone := d.(decision.None); isNone {
		return d
	}
	return sited{inner: d, site: site}
}

type sited struct {
	inner decision.Decider
	site  string
}

func (s sited) Name() string { return s.inner.Name() }

func (s sited) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	return s.inner.Decide(WithSite(ctx, s.site), req)
}

// Capabilities declares what the wrapped provider declares.
func (s sited) Capabilities() decision.Capabilities { return decision.CapabilitiesOf(s.inner) }
