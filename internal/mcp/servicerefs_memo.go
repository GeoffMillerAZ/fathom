package mcp

// servicerefs_memo.go — RT-89 (docs/design/responsible-tenant.md §6): a
// service's ingest-extracted routes and client calls are read once per
// (store, service, watermark) and shared by blast_radius,
// cross_repo_blast_radius and review_diff, never re-read per call. A
// per-file refresh replaces one file's rows without moving the watermark,
// so the refresh path drops the entry (Server.DropServiceRefs).

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/store"
)

type serviceRefsKey struct{ memo.Scope }

func (k serviceRefsKey) MemoScope() (string, string) {
	return k.Store + "\x00" + k.Service, k.Watermark
}

type serviceRefsVal struct {
	refs     store.ServiceRefs
	complete bool
}

var serviceRefsGroup = memo.NewGroup[serviceRefsKey, serviceRefsVal]("service_refs", memo.Options{WatermarkScoped: true})

// memoRefReader is a serviceref.RefReader over rr memoized per watermark.
// A backend that names no store or watermark is read every call (I21).
type memoRefReader struct {
	backend QueryInterface
	rr      serviceref.RefReader
}

func (m memoRefReader) ServiceRefsFor(ctx context.Context, service string) (store.ServiceRefs, bool, error) {
	sc := navScopeNoEnsure(ctx, m.backend, service)
	v, err := serviceRefsGroup.Get(ctx, serviceRefsKey{sc}, func(bctx context.Context) (serviceRefsVal, int64, error) {
		refs, complete, err := m.rr.ServiceRefsFor(bctx, service)
		if err != nil {
			return serviceRefsVal{}, 0, err
		}
		size := int64(64)
		for _, r := range refs.Routes {
			size += 64 + int64(len(r.Method)+len(r.Path)+len(r.Handler)+len(r.File))
		}
		for _, c := range refs.Calls {
			size += 64 + int64(len(c.Method)+len(c.Template)+len(c.RawURL)+len(c.Symbol)+len(c.File))
		}
		return serviceRefsVal{refs: refs, complete: complete}, size, nil
	})
	return v.refs, v.complete, err
}

// dropServiceRefs forgets service's memoized refs at the backend's current
// watermark.
func dropServiceRefs(backend QueryInterface, service string) {
	serviceRefsGroup.Remove(serviceRefsKey{navScopeNoEnsure(context.Background(), backend, service)}, memo.ReasonWatermark)
}

// DropServiceRefs forgets service's memoized routes and client calls — the
// per-file refresh (POST /refresh-file) replaced a file's rows without
// moving the watermark the memo is keyed on.
func (s *Server) DropServiceRefs(service string) {
	dropServiceRefs(s.backend, service)
}
