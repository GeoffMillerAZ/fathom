package routes_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
)

// pathsTS is bulletproof-react's config/paths.ts, shortened: an object of
// route-path members, getHref functions beside them.
const pathsTS = `export const paths = {
  home: {
    path: '/',
    getHref: () => '/',
  },
  auth: {
    login: {
      path: '/auth/login',
      getHref: (redirectTo?: string | null) =>
        ` + "`/auth/login${redirectTo ? `?redirectTo=${redirectTo}` : ''}`" + `,
    },
  },
  app: {
    root: { path: '/app', getHref: () => '/app' },
    dashboard: { path: '', getHref: () => '/app' },
    users: { path: 'users', getHref: () => '/app/users' },
    discussion: { path: 'discussions/:discussionId' },
  },
} as const;
`

const routerTSX = `import { createBrowserRouter } from 'react-router';
import { paths } from '@/config/paths';
import { LOGIN } from './constants';
export const router = createBrowserRouter([
  { path: paths.home.path, lazy: () => import('./routes/landing') },
  { path: paths.auth.login.path, element: <Login /> },
  {
    path: paths.app.root.path,
    element: <AppRoot />,
    children: [
      { path: paths.app.users.path, element: <Users /> },
      { path: paths.app.discussion.path, element: <Discussion /> },
      { path: paths.app.dashboard.path, element: <Dashboard /> },
      { path: paths.app.missing.path, element: <Missing /> },
      { path: '/plain', element: <Plain /> },
    ],
  },
  { path: LOGIN, element: <Login2 /> },
  { path: '*', element: <NoMatch /> },
]);
`

func read(t *testing.T, files map[string]string) []routes.Result {
	t.Helper()
	var out []routes.Result
	for p, src := range files {
		lang := "ts"
		out = append(out, routes.ExtractFile(lang, p, []byte(src)))
	}
	return out
}

// TestComposeResolvesImportedPathConstants (SG-46, O774): a React Router
// path written as an imported same-repo constant is a deferred reference the
// route seam resolves against the module's exported string constants.
func TestComposeResolvesImportedPathConstants(t *testing.T) {
	files := read(t, map[string]string{
		"apps/web/src/config/paths.ts":  pathsTS,
		"apps/web/src/app/router.tsx":   routerTSX,
		"apps/web/src/app/constants.ts": `export const LOGIN = '/signin';`,
	})
	got := routes.Compose(files)
	assertKeys(
		t, got,
		"GET / Component apps/web/src/app/router.tsx:5",
		"GET /auth/login Login apps/web/src/app/router.tsx:6",
		"GET /app AppRoot apps/web/src/app/router.tsx:7",
		"GET /app/users Users apps/web/src/app/router.tsx:11",
		"GET /app/discussions/:discussionId Discussion apps/web/src/app/router.tsx:12",
		"GET /app Dashboard apps/web/src/app/router.tsx:13",
		"GET /plain Plain apps/web/src/app/router.tsx:15",
		"GET /signin Login2 apps/web/src/app/router.tsx:18",
		"GET /* NoMatch apps/web/src/app/router.tsx:19",
	)
}

// An import no file of the service exports a constant for stays as today:
// the route is unread, and no deferred path leaks into the output.
func TestComposeUnresolvedPathConstantStaysUnread(t *testing.T) {
	files := read(t, map[string]string{
		"src/router.tsx": routerTSX, // @/config/paths and ./constants exist nowhere
	})
	got := routes.Compose(files)
	assertKeys(
		t, got,
		"GET /* NoMatch src/router.tsx:19",
	)
	for _, r := range got {
		for _, c := range r.Path {
			if c < ' ' {
				t.Fatalf("a deferred path leaked: %q", r.Path)
			}
		}
	}
}

// Two modules match an alias; the one sharing the longest directory prefix
// with the importing file wins, a tie resolves nothing.
func TestComposePathConstantAliasPrefersNearestModule(t *testing.T) {
	a := `export const paths = { users: { path: '/a-users' } };`
	b := `export const paths = { users: { path: '/b-users' } };`
	router := `import { createBrowserRouter } from 'react-router';
import { paths } from '@/config/paths';
export const r = createBrowserRouter([{ path: paths.users.path, element: <U /> }]);
`
	files := read(t, map[string]string{
		"apps/a/src/config/paths.ts": a,
		"apps/b/src/config/paths.ts": b,
		"apps/b/src/app/router.tsx":  router,
	})
	assertKeys(t, routes.Compose(files), "GET /b-users U apps/b/src/app/router.tsx:3")

	files = read(t, map[string]string{
		"apps/a/src/config/paths.ts": a,
		"apps/b/src/config/paths.ts": b,
		"tools/router.tsx":           router,
	})
	assertKeys(t, routes.Compose(files))
}

// A <Route path={paths.x.path}> element resolves the same way.
func TestComposePathConstantInRouteElement(t *testing.T) {
	files := read(t, map[string]string{
		"src/config/paths.ts": `export const paths = { app: { path: '/app' }, users: { path: 'users' } };`,
		"src/App.tsx": `import { Routes, Route } from 'react-router-dom';
import { paths } from './config/paths';
export default () => (
  <Routes>
    <Route path={paths.app.path} element={<Layout />}>
      <Route path={paths.users.path} element={<Users />} />
    </Route>
  </Routes>
);
`,
	})
	assertKeys(
		t, routes.Compose(files),
		"GET /app Layout src/App.tsx:5",
		"GET /app/users Users src/App.tsx:6",
	)
}

// Only a module whose exported constants look like route paths emits them,
// and only shapes the reader evaluates by reading: no call, template
// substitution or spread.
func TestPathConstantsBounded(t *testing.T) {
	res := routes.ExtractFile("ts", "src/other.ts", []byte(`export const A = 'GET';
export const B = { x: 'hello', y: fn('/z'), ...rest, z: `+"`/t/${A}`"+` };`))
	if len(res.Consts) != 0 {
		t.Fatalf("a module with no route-looking constant emitted %v", res.Consts)
	}
	res = routes.ExtractFile("ts", "src/paths.ts", []byte(`export const P = '/p';
export const Q = { a: '/a', b: { c: 'c' }, d: fn('/d'), ...rest, e: `+"`/t/${P}`"+`, f: 42 };
const hidden = '/hidden';`))
	var got []string
	for _, c := range res.Consts {
		got = append(got, c.Name+"="+c.Value)
	}
	want := []string{"P=/p", "Q.a=/a", "Q.b.c=c"}
	if len(got) != len(want) {
		t.Fatalf("consts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("consts = %v, want %v", got, want)
		}
	}
}
