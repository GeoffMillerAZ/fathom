// route_oracle.mjs: React Router's own route table for one app (LB-F380,
// README.md). esbuild bundles the app's route module with React Router
// replaced by a thin shim over the real package: every route array the app
// hands React Router (createBrowserRouter, createHashRouter, createMemoryRouter,
// createStaticHandler, useRoutes, createRoutesFromElements) is recorded, and
// <Routes> records the route objects React Router's own
// createRoutesFromChildren makes of its children, with the pathname base of
// the route it renders under (descendant routes). The bundle is run in Node:
// the module is imported, an exported router factory named by --call is
// called, and a default-exported component is rendered at "/" inside a
// MemoryRouter, then at every path found, until no new path appears. The
// recorded route objects are walked as React Router joins them, and each
// lazy() is called to load what it returns.
//
// A route is GET (a page view) at its joined path when it has a path or is an
// index route and renders something (element, Component or lazy). Its
// handler is the component it renders: the element's type, through an element
// whose only child is one component element (<Suspense><Page/></Suspense> is
// Page), or Component, or what lazy() resolves to; named by the function's own
// name ("" for an anonymous or exotic component).
//
//	node route_oracle.mjs --tools <npm prefix: esbuild, react, react-dom, react-router-dom> \
//	  --rr <the React Router package to wrap, a directory> --app <app dir> --entry <module, app-relative> \
//	  --repo <checkout> --work <scratch dir> --out <dump.json> [--call <export>] [--alias <specifiers>] [--dom 1] [--env-file <app-relative .env>]
//
// --alias names the specifiers the app imports React Router by (default
// react-router-dom; react-router for a v7 app), each replaced by the shim.
//
// The dump is {tool, tool_version, routes: [{method, path, handler, entry}]},
// entry the app directory relative to --repo; calibration/oracles/express/
// to_route_oracle.mjs --group entry turns dumps into route-oracle files.
import { createRequire } from "node:module";
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { Writable } from "node:stream";

const args = {};
for (let i = 2; i < process.argv.length; i += 2) args[process.argv[i].replace(/^--/, "")] = process.argv[i + 1];
for (const k of ["tools", "rr", "app", "entry", "repo", "work", "out"]) {
  if (!args[k]) {
    console.error(`missing --${k}`);
    process.exit(2);
  }
}
const tools = path.resolve(args.tools);
const require = createRequire(path.join(tools, "package.json"));
const esbuild = require("esbuild");
const rrDir = path.resolve(args.rr);
const rrPkg = JSON.parse(fs.readFileSync(path.join(rrDir, "package.json"), "utf8"));
const rrModule = path.join(rrDir, rrPkg.module || rrPkg.main);
const app = path.resolve(args.app);
const work = path.resolve(args.work);
fs.mkdirSync(work, { recursive: true });

const shim = path.join(work, "rr-shim.mjs");
fs.writeFileSync(
  shim,
  `import * as RR from ${JSON.stringify(rrModule)};
export * from ${JSON.stringify(rrModule)};
import { createElement, useContext } from "react";
const rec = (globalThis.__rrRecorded ||= []);
function base() {
  const ctx = useContext(RR.UNSAFE_RouteContext);
  const m = ctx && ctx.matches;
  return m && m.length ? m[m.length - 1].pathnameBase : "";
}
function note(routes, b) {
  if (Array.isArray(routes)) rec.push({ routes, base: b || "" });
  return routes;
}
export function createBrowserRouter(routes) { note(routes); return { __recorded: true }; }
export function createHashRouter(routes) { note(routes); return { __recorded: true }; }
export function createMemoryRouter(routes) { note(routes); return { __recorded: true }; }
export function createStaticHandler(routes) { note(routes); return { __recorded: true }; }
export function RouterProvider() { return null; }
export function useRoutes(routes, loc) { note(routes, base()); return RR.useRoutes(routes, loc); }
export function createRoutesFromElements(children, parent) { return note(RR.createRoutesFromElements(children, parent)); }
export function Routes(props) { note(RR.createRoutesFromChildren(props.children), base()); return createElement(RR.Routes, props); }
export function BrowserRouter(props) { return createElement(RR.MemoryRouter, { initialEntries: [globalThis.__rrAt || "/"] }, props.children); }
export function HashRouter(props) { return BrowserRouter(props); }
`,
);
const entryFile = path.join(work, "entry.mjs");
fs.writeFileSync(
  entryFile,
  `import * as M from ${JSON.stringify(path.join(app, args.entry))};
import { renderToPipeableStream } from "react-dom/server";
import { createElement } from "react";
import { MemoryRouter } from ${JSON.stringify(rrModule)};
export { M, renderToPipeableStream, createElement, MemoryRouter };
`,
);
const domStub = path.join(work, "dom-stub.mjs");
fs.writeFileSync(domStub, "export function createRoot() { return { render() {} }; }\nexport function hydrateRoot() { return {}; }\nexport default { createRoot, hydrateRoot };\n");
const bundle = path.join(work, "bundle.mjs");
// import.meta.env as Vite gives it a production build, with the variables of
// --env-file (an app's own .env.example) when named.
const viteEnv = { MODE: "production", DEV: false, PROD: true, SSR: false, BASE_URL: "/" };
if (args["env-file"]) {
  for (const line of fs.readFileSync(path.join(app, args["env-file"]), "utf8").split("\n")) {
    const m = line.match(/^\s*([A-Za-z0-9_]+)\s*=\s*(.*?)\s*$/);
    if (m) viteEnv[m[1]] = m[2].replace(/^['"]|['"]$/g, "");
  }
}
await esbuild.build({
  entryPoints: [entryFile],
  outfile: bundle,
  bundle: true,
  platform: "node",
  format: "esm",
  jsx: "automatic",
  keepNames: true,
  // ES module builds first: a package's "main" may be a minified UMD bundle
  // whose components are anonymous functions (react-router's), so no name.
  mainFields: ["module", "main"],
  logLevel: "error",
  nodePaths: [path.join(tools, "node_modules"), path.join(app, "node_modules")],
  // Exact specifiers only (an alias would also take a subpath such as
  // react-router/dom, which the app imports from the real package).
  plugins: [
    {
      name: "react-router-shim",
      setup(b) {
        const swap = new Map([...(args.alias || "react-router-dom").split(",").map((a) => [a, shim]), ["react-dom/client", domStub]]);
        b.onResolve({ filter: /^react-(router|router-dom|dom\/client)$/ }, (a) => (swap.has(a.path) ? { path: swap.get(a.path) } : undefined));
      },
    },
  ],
  loader: { ".css": "empty", ".svg": "empty", ".png": "empty", ".jpg": "empty" },
  define: { "process.env.NODE_ENV": '"production"', "import.meta.env": JSON.stringify(viteEnv) },
  banner: { js: "import { createRequire as __cr } from 'module'; const require = __cr(import.meta.url);" },
});
if (args.dom) {
  // --dom: an entry that touches the page at import (createRoot on an
  // element, hydration data on window) gets an inert document and window;
  // without it none is defined, so a library's browser check stays false.
  globalThis.document = { getElementById: () => ({}), createElement: () => ({ setAttribute() {}, appendChild() {}, style: {} }), cookie: "" };
  globalThis.window = globalThis.window || { location: { pathname: "/", search: "", hash: "" }, addEventListener() {}, removeEventListener() {} };
}
const B = await import(pathToFileURL(bundle).href);
const M = B.M;
if (args.call && typeof M[args.call] === "function") M[args.call]();

const isElement = (x) => x && typeof x === "object" && x.$$typeof && x.type !== undefined;
const LAZY = Symbol.for("react.lazy"), MEMO = Symbol.for("react.memo"), FORWARD_REF = Symbol.for("react.forward_ref");
// nameOf names a component type by its function's own name, through
// React.lazy (loaded), memo and forwardRef; "" when it has none.
async function nameOf(type) {
  for (let i = 0; i < 8 && type && typeof type === "object"; i++) {
    if (type.$$typeof === LAZY) {
      try {
        type = type._init(type._payload);
      } catch (thrown) {
        if (thrown && typeof thrown.then === "function") {
          await thrown.then(() => {}, () => {});
          try {
            type = type._init(type._payload);
          } catch {
            return "";
          }
        } else return "";
      }
    } else if (type.$$typeof === MEMO) type = type.type;
    else if (type.$$typeof === FORWARD_REF) return type.render && type.render.name ? type.render.name : type.displayName || "";
    else return "";
  }
  return typeof type === "function" ? type.name || "" : "";
}
async function componentOf(el) {
  while (isElement(el)) {
    const kids = el.props && el.props.children;
    if (isElement(kids) && typeof kids.type !== "string") {
      el = kids;
      continue;
    }
    break;
  }
  return isElement(el) && typeof el.type !== "string" ? await nameOf(el.type) : "";
}
const join = (parent, p) => {
  const segs = [];
  for (const x of [parent, p]) for (const s of String(x || "").split("/")) if (s) segs.push(s);
  return "/" + segs.join("/");
};
const routes = new Map();
const seenArrays = new Set();
// walk joins as React Router does: a path starting with "/" stands alone,
// except at the top of descendant <Routes>, whose paths are relative to the
// route they render under (React Router matches them against the remaining
// pathname).
async function walk(list, parent, descendant) {
  for (const r of list || []) {
    let full = parent || "/";
    if (r.path !== undefined) full = String(r.path).startsWith("/") && !descendant ? join("", r.path) : join(parent, r.path);
    let handler = "";
    let renders = false;
    if (r.element !== undefined && r.element !== null) {
      renders = true;
      handler = await componentOf(r.element);
    }
    if (r.Component) {
      renders = true;
      handler = handler || (await nameOf(r.Component));
    }
    if (typeof r.lazy === "function") {
      renders = true;
      if (!handler) {
        try {
          const got = await r.lazy();
          handler = (got && got.Component && (await nameOf(got.Component))) || (got && got.element && (await componentOf(got.element))) || "";
        } catch (e) {
          handler = "";
        }
      }
    }
    if ((r.path !== undefined || r.index) && renders) {
      routes.set(["GET", full, handler].join("\0"), { method: "GET", path: full, handler });
    }
    if (r.children) await walk(r.children, full, false);
  }
}
async function drain() {
  for (const { routes: list, base } of globalThis.__rrRecorded || []) {
    if (seenArrays.has(list)) continue;
    seenArrays.add(list);
    await walk(list, base || "/", !!base && base !== "/");
  }
}
await drain();
if (typeof M.default === "function") {
  const visited = new Set();
  for (;;) {
    const next = [...routes.values()].map((r) => r.path.replace(/\/\*$/, "/") || "/").find((p) => !visited.has(p)) ?? (visited.has("/") ? null : "/");
    if (next === null || next === undefined) break;
    visited.add(next);
    globalThis.__rrAt = next;
    // Streamed to completion, so a lazy component inside Suspense loads and
    // renders (its own <Routes> are descendant routes).
    await new Promise((resolve) => {
      const done = () => resolve();
      const timer = setTimeout(done, 10000);
      try {
        const s = B.renderToPipeableStream(B.createElement(B.MemoryRouter, { initialEntries: [next] }, B.createElement(M.default)), {
          onAllReady() { clearTimeout(timer); done(); },
          onShellError() { globalThis.__rrRenderFailures = (globalThis.__rrRenderFailures || 0) + 1; clearTimeout(timer); done(); },
          onError() {},
        });
        s.pipe(new Writable({ write(_c, _e, cb) { cb(); } }));
      } catch {
        // a component that cannot render outside a browser leaves its own
        // descendant routes unrecorded (counted as a render failure)
        globalThis.__rrRenderFailures = (globalThis.__rrRenderFailures || 0) + 1;
        clearTimeout(timer);
        done();
      }
    });
    await drain();
  }
}
const out = {
  tool: "react-router route objects (calibration/oracles/react-router/route_oracle.mjs)",
  tool_version: `${rrPkg.name} ${rrPkg.version}`,
  render_failures: globalThis.__rrRenderFailures || 0,
  routes: [...routes.values()]
    .map((r) => ({ ...r, entry: path.relative(path.resolve(args.repo), app).split(path.sep).join("/") }))
    .sort((a, b) => (a.path + " " + a.handler < b.path + " " + b.handler ? -1 : 1)),
};
fs.writeFileSync(args.out, JSON.stringify(out, null, 1) + "\n");
console.error(`${out.routes.length} routes, ${out.render_failures} render failures`);
