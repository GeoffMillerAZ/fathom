package serviceref_test

// The route seam's imported path constants (SG-46, O774): a module's route
// constants are stored as ConstMethod rows and a route that imports one as a
// route row with a deferred path; RoutesFromRows resolves them, and neither
// a constant nor a deferred path reaches a route.

import (
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/inventory"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const constPaths = `export const paths = {
  app: { root: { path: '/app' }, users: { path: 'users' } },
} as const;
`

const constRouter = `import { createBrowserRouter } from 'react-router';
import { paths } from '@/config/paths';
export const router = createBrowserRouter([
  { path: paths.app.root.path, element: <Root />, children: [
    { path: paths.app.users.path, element: <Users /> },
    { path: paths.app.gone.path, element: <Gone /> },
  ] },
]);
`

func TestRoutesFromRows_ResolvesImportedPathConstants(t *testing.T) {
	var rows []store.ServiceRoute
	for p, src := range map[string]string{"web/src/config/paths.ts": constPaths, "web/src/app/router.tsx": constRouter} {
		rows = append(rows, serviceref.ExtractFile(p, []byte(src)).Routes...)
	}
	consts := 0
	for _, r := range rows {
		if r.Method == serviceref.ConstMethod {
			consts++
		}
	}
	if consts != 2 {
		t.Fatalf("ConstMethod rows = %d, want 2 (app.root.path, app.users.path)", consts)
	}
	var got []string
	for _, r := range serviceref.RoutesFromRows(rows) {
		if r.Method == serviceref.ConstMethod || strings.ContainsAny(r.Path, "\x02\x1e\x1f") {
			t.Fatalf("a constant or a deferred path reached a route: %+v", r)
		}
		got = append(got, r.Method+" "+r.Path+" "+r.Handler)
	}
	sort.Strings(got)
	if want := "GET /app Root;GET /app/users Users"; strings.Join(got, ";") != want {
		t.Fatalf("routes = %v, want %s", got, want)
	}

	items := inventory.Extract([]inventory.File{
		{Path: "web/src/config/paths.ts", Content: []byte(constPaths)},
		{Path: "web/src/app/router.tsx", Content: []byte(constRouter)},
	}, nil)
	var keys []string
	for _, it := range items {
		if strings.HasPrefix(it.Key, "route:") {
			keys = append(keys, it.Key)
		}
	}
	sort.Strings(keys)
	if want := "route:GET /app;route:GET /app/users"; strings.Join(keys, ";") != want {
		t.Fatalf("inventory route keys = %v, want %s", keys, want)
	}
}
