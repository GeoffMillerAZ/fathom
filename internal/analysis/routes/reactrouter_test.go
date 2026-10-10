package routes

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestReadReactRouter (LB-F378): React Router v6/v7 route objects and <Route>
// elements are GET page routes at their joined paths, handled by the component
// they render.
func TestReadReactRouter(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		src    string
		routes []string
	}{
		{
			"createBrowserRouter: nesting, index, element, Component, both lazy forms, splat, a wrapper's one child",
			"src/router.tsx",
			`import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import { Suspense } from 'react';
import Root from './root';
const router = createBrowserRouter([
  {
    path: '/',
    element: <Root />,
    children: [
      { index: true, element: <Home /> },
      { path: 'about', lazy: () => import('./pages/About') },
      {
        path: 'dashboard',
        async lazy() {
          let { DashboardLayout } = await import('./pages/Dashboard');
          return { Component: DashboardLayout };
        },
        children: [{ path: 'messages/:id', Component: Message }],
      },
      { path: '*', element: <NoMatch /> },
    ],
  },
  { path: '/login', element: <Suspense fallback={<Spinner />}><Login /></Suspense> },
]);
export default function App() { return <RouterProvider router={router} />; }
`,
			[]string{"GET / Home", "GET / Root", "GET /* NoMatch", "GET /about Component", "GET /dashboard DashboardLayout", "GET /dashboard/messages/:id Message", "GET /login Login"},
		},
		{
			"useRoutes over a const array, a path from a const",
			"src/App.jsx",
			`import { useRoutes } from 'react-router';
const COURSES = 'courses';
const routes = [
  { path: '/', element: <Layout />, children: [
    { path: COURSES, element: <Courses />, children: [{ path: ':id', element: <Course /> }] },
  ] },
];
export default function App() { return useRoutes(routes); }
`,
			[]string{"GET / Layout", "GET /courses Courses", "GET /courses/:id Course"},
		},
		{
			"<Routes> elements: a closing tag and an opening one on one line, an apostrophe in JSX text",
			"src/App.js",
			`import { Routes, Route } from 'react-router-dom';
export default function App() {
  return (
    <div>
      <h1>Don't panic</h1>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Home />} />
          <Route path="about" element={<About />} /></Route><Route path="/x/:y" element={<X />} />
      </Routes>
    </div>
  );
}
`,
			[]string{"GET / Home", "GET / Layout", "GET /about About", "GET /x/:y X"},
		},
		{
			"createRoutesFromElements: element before lazy, a pathless layout adds no segment and is no route",
			"src/main.tsx",
			`import { createBrowserRouter, createRoutesFromElements, Route } from 'react-router-dom';
const router = createBrowserRouter(createRoutesFromElements(
  <Route path="/" element={<Root />}>
    <Route path="contact/:contactId" element={<Contact />} lazy={() => import('./contact')} />
    <Route element={<Auth />}><Route path="settings" Component={Settings} /></Route>
  </Route>
));
`,
			[]string{"GET / Root", "GET /contact/:contactId Contact", "GET /settings Settings"},
		},
		{
			"a RouteObject[] module read without a router call",
			"src/routes.ts",
			`import type { RouteObject } from 'react-router';
import { A } from './a';
export const routes: RouteObject[] = [{ path: '/a', Component: A }, { path: '/b', children: [{ index: true, Component: B }] }];
const n = <number>(x as unknown);
`,
			[]string{"GET /a A", "GET /b B"},
		},
		{
			"a path no reader can resolve skips the route and its children",
			"src/router.tsx",
			`import { createBrowserRouter } from 'react-router';
import { paths } from '@/config/paths';
export const r = createBrowserRouter([
  { path: paths.home.path, element: <Home />, children: [{ path: 'x', element: <X /> }] },
  { path: '/ok', element: <Ok /> },
]);
`,
			[]string{"GET /ok Ok"},
		},
		{
			"a Component method is a function named Component",
			"src/app.tsx",
			"import { createBrowserRouter } from 'react-router-dom';\nlet router = createBrowserRouter([{ path: '/', loader: () => ({}), Component() { return <p />; } }]);\n",
			[]string{"GET / Component"},
		},
		{
			"an absolute child path stands alone",
			"src/r.jsx",
			`import { createHashRouter } from 'react-router-dom';
createHashRouter([{ path: '/app', element: <App />, children: [{ path: '/app/inbox', element: <Inbox /> }] }]);
`,
			[]string{"GET /app App", "GET /app/inbox Inbox"},
		},
		{"another router's Route is not React Router's", "src/App.jsx", "import { Route } from 'wouter';\nexport const A = () => <Route path=\"/x\" component={X} />;\n", nil},
		{"no react-router import, no routes", "src/App.jsx", "const r = createBrowserRouter([{ path: '/', element: <Home /> }]);\n", nil},
		{"node_modules is never read", "node_modules/react-router/dist/x.js", "import { createBrowserRouter } from 'react-router';\ncreateBrowserRouter([{ path: '/', element: <Home /> }]);\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := readReactRouter(c.path, []byte(c.src))
			got.Mounts = nil // TestReadReactRouterDescendants reads them
			checkResult(t, got, c.routes, nil)
			for _, r := range got.Routes {
				if r.Line < 1 {
					t.Errorf("route %+v has no line", r)
				}
			}
		})
	}
}

// TestReactRouterRegistered: the reader runs for every JS and TS file.
func TestReactRouterRegistered(t *testing.T) {
	src := []byte("import { createBrowserRouter } from 'react-router';\ncreateBrowserRouter([{ path: '/', element: <Home /> }]);\n")
	for _, lang := range []string{"js", "ts"} {
		if r := routeLines(ExtractFile(lang, "src/r.tsx", src).Routes); !reflect.DeepEqual(r, []string{"GET / Home"}) {
			t.Errorf("%s: routes %q", lang, r)
		}
	}
}

// TestLexJSX: JSX is tokens, not text: a "</" never opens a regular
// expression and an apostrophe in JSX text never opens a string, so the
// tokens after either are the source's own.
func TestLexJSX(t *testing.T) {
	src := "const a = <p>Don't</p>; const b = '/x' // c\nconst e = <A b=\"1\" c={d / 2}>{f}</A>;\nx = 4 / 2 / 1\n"
	toks, elems := lexJSX([]byte(src), true)
	var got []string
	for _, tk := range toks {
		got = append(got, fmt.Sprintf("%d:%s", tk.kind, tk.val))
	}
	want := []string{
		"1:const", "1:a", "6:=", "6:<", "1:p", "6:>", "6:<", "6:/", "1:p", "6:>", "6:;", "1:const", "1:b", "6:=", "2:/x",
		"1:const", "1:e", "6:=", "6:<", "1:A", "1:b", "6:=", "2:1", "1:c", "6:=", "6:{", "1:d", "6:/", "4:2", "6:}", "6:>", "6:{", "1:f", "6:}", "6:<", "6:/", "1:A", "6:>", "6:;",
		"1:x", "6:=", "4:4", "6:/", "4:2", "6:/", "4:1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens:\n got %s\nwant %s", strings.Join(got, " "), strings.Join(want, " "))
	}
	if len(elems) != 2 || elems[0].name != "p" || elems[1].name != "A" || elems[1].line != 2 {
		t.Fatalf("elements: %+v", elems)
	}
	if a := elems[1].attrs["b"]; !a.isStr || a.str != "1" {
		t.Errorf("attribute b: %+v", a)
	}
	// In a .ts file `<T>x` is a type assertion: no element.
	if _, e := lexJSX([]byte("const n = <number>(x as unknown);\n"), false); len(e) != 0 {
		t.Errorf("a .ts type assertion lexed as JSX: %+v", e)
	}
}

// TestReadReactRouterDescendants: a splat route ("dashboard/*") rendering a
// component imported from a module of the repo (or a React.lazy of one)
// mounts that module's routes
// (its own <Routes>, descendant routes) under the route's path; Compose
// serves them there.
func TestReadReactRouterDescendants(t *testing.T) {
	app := readReactRouter("src/App.tsx", []byte(`import * as React from 'react';
import { Routes, Route } from 'react-router-dom';
import Dashboard from './pages/Dashboard';
import { Lib } from 'some-lib';
const Inbox = React.lazy(() => import('./pages/Inbox'));
export default function App() {
  return (
    <Routes>
      <Route path="dashboard/*" element={<Dashboard />} />
      <Route path="lib/*" element={<Lib />} />
      <Route path="inbox/*" element={<React.Suspense fallback={<p />}><Inbox /></React.Suspense>} />
    </Routes>
  );
}
`))
	checkResult(t, app, []string{"GET /dashboard/* Dashboard", "GET /inbox/* Inbox", "GET /lib/* Lib"}, []string{"/dashboard -> ./pages/Dashboard", "/inbox -> ./pages/Inbox"})
	dash := readReactRouter("src/pages/Dashboard.tsx", []byte(`import { Routes, Route } from 'react-router-dom';
export default function Dashboard() {
  return (
    <Routes>
      <Route path="/" element={<DashboardHome />} />
      <Route path="messages" element={<Messages />} />
    </Routes>
  );
}
`))
	// Compose joins a mounted "/" as "/dashboard/" (serviceref.NormalizeTemplate
	// reads a trailing slash alike).
	if got := routeLines(Compose([]Result{app, dash})); !reflect.DeepEqual(got, []string{"GET /dashboard/ DashboardHome", "GET /dashboard/* Dashboard", "GET /dashboard/messages Messages", "GET /inbox/* Inbox", "GET /lib/* Lib"}) {
		t.Errorf("composed: %q", got)
	}
}
