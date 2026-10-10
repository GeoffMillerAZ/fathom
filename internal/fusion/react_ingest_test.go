package fusion

// Wave c3 item react (LB-F377, LB-F378): an ingest stores React's kinds
// (components, class components, hooks, props) from TSX, TS and JSX files,
// draws a JSX element's edge to its component (same file and imported alike)
// and a hook call's to its hook, and stores React Router's routes as
// service_routes rows that compose into route: keys, each handled by an
// indexed component.

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/syslink"
)

func reactCorpus() map[string]string {
	return map[string]string{
		"src/Button.tsx": `export interface ButtonProps {
  label: string;
  onClick?: () => void;
}

export function Button({ label, onClick }: ButtonProps) {
  return <button onClick={onClick}>{label}</button>;
}
`,
		"src/useAuth.ts": `import { useContext } from 'react';
import { AuthContext } from './context';

export function useAuth() {
  return useContext(AuthContext);
}
`,
		"src/context.ts": `import { createContext } from 'react';
export const AuthContext = createContext({ name: '' });
`,
		"src/Page.tsx": `import { Button } from './Button';
import { useAuth } from './useAuth';

function Title() {
  return <h1>Shop</h1>;
}

export default function Page() {
  const user = useAuth();
  return (
    <main>
      <Title />
      <Button label={user.name} />
    </main>
  );
}
`,
		"src/Clock.jsx": `import React from 'react';
export class Clock extends React.Component {
  render() {
    return <span>{Date.now()}</span>;
  }
}
`,
		"src/router.tsx": `import { createBrowserRouter, Route, Routes } from 'react-router-dom';
import Page from './Page';
import { Clock } from './Clock';
import Admin from './admin/Admin';

export const router = createBrowserRouter([
  { path: '/', element: <Page />, children: [{ path: 'clock', element: <Clock /> }] },
  { path: '/admin/*', element: <Admin /> },
]);
`,
		"src/admin/Admin.tsx": `import { Routes, Route } from 'react-router-dom';
import { Button } from '../Button';

export default function Admin() {
  return (
    <Routes>
      <Route path="users/:id" element={<Users />} />
    </Routes>
  );
}

function Users() {
  return <Button label="users" />;
}
`,
	}
}

// TestIngest_ReactComponents (LB-F377).
func TestIngest_ReactComponents(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, reactCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "shop", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "shop", "")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]core.SourceKind{}
	for _, o := range obs {
		kinds[strings.SplitN(o.SourceRef, ":", 2)[0]+"#"+o.Symbol] = o.SourceKind
	}
	want := map[string]core.SourceKind{
		"src/Button.tsx#Button":              core.KindJSComponent,
		"src/Button.tsx#ButtonProps":         core.KindTSInterface,
		"src/Button.tsx#ButtonProps.label":   core.KindJSProp,
		"src/Button.tsx#ButtonProps.onClick": core.KindJSProp,
		"src/useAuth.ts#useAuth":             core.KindJSHook,
		"src/Page.tsx#Title":                 core.KindJSComponent,
		"src/Page.tsx#Page":                  core.KindJSComponent,
		"src/Clock.jsx#Clock":                core.KindJSClassComponent,
		"src/admin/Admin.tsx#Admin":          core.KindJSComponent,
		"src/admin/Admin.tsx#Users":          core.KindJSComponent,
	}
	for k, w := range want {
		if kinds[k] != w {
			t.Errorf("%s: kind %q, want %q", k, kinds[k], w)
		}
	}
	if k := kinds["src/context.ts#AuthContext"]; k == core.KindJSComponent {
		t.Error("a context object is no component") // a module-level const since LB-F382
	}

	// A JSX element is an edge to its component, in the same file (Page ->
	// Title) and across files (Page -> Button), and a hook call an edge to the
	// hook (Page -> useAuth); an intrinsic element (<main>) is none.
	all, err := st.TypedEdges(ctx, "shop")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved && strings.HasPrefix(e.FromFile, "src/Page.tsx") {
			got = append(got, e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(got)
	joined := strings.Join(got, ";")
	for _, w := range []string{"shop::src/Page.Page->shop::src/Page.Title", "shop::src/Page.Page->shop::src/Button.Button", "shop::src/Page.Page->shop::src/useAuth.useAuth"} {
		if !strings.Contains(joined, w) {
			t.Errorf("Page.tsx file-resolved edges %q lack %s", got, w)
		}
	}
	for _, e := range got {
		if strings.HasSuffix(e, ".main") || strings.HasSuffix(e, ".h1") {
			t.Errorf("an intrinsic element drew an edge: %s", e)
		}
	}
}

// TestIngest_ReactRouterRoutes (LB-F378): React Router's routes are stored
// and compose (a splat route mounts its component's module, whose own
// <Routes> serve below it); each is a route: key handled by an indexed
// component.
func TestIngest_ReactRouterRoutes(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, reactCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "shop", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "shop", "")
	if err != nil {
		t.Fatal(err)
	}
	components := map[string]bool{}
	for _, o := range obs {
		if o.SourceKind == core.KindJSComponent || o.SourceKind == core.KindJSClassComponent {
			components[o.Symbol] = true
		}
	}
	refs, complete, err := st.ServiceRefsFor(ctx, "shop")
	if err != nil || !complete {
		t.Fatalf("ServiceRefsFor: complete=%v err=%v", complete, err)
	}
	got := map[string]string{}
	for _, r := range serviceref.RoutesFromRows(refs.Routes) {
		if r.Method == serviceref.MountMethod {
			t.Fatalf("a mount row leaked: %+v", r)
		}
		key, err := syslink.ParseContractKey("route:" + r.Method + " " + r.Path)
		if err != nil {
			t.Fatalf("route %+v is no route: key: %v", r, err)
		}
		got[key.Body] = r.Handler
		if !components[r.Handler] {
			t.Errorf("route %s handler %q is no indexed component", key.Body, r.Handler)
		}
	}
	want := map[string]string{
		"GET /":               "Page",
		"GET /clock":          "Clock",
		"GET /admin/{}":       "Admin", // the key grammar reads a splat as a hole
		"GET /admin/users/{}": "Users",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stored routes = %v, want %v", got, want)
	}
}
