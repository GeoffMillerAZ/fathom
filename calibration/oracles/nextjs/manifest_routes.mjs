// manifest_routes.mjs — the Next.js route oracle (tier A): the routes Next
// itself compiled, read from the build's own manifests after
// `next build --experimental-build-mode=compile` (compile only: no page is
// prerendered, so no data source is needed).
//
//   node manifest_routes.mjs --app <Next project dir, built> --repo <repo root> \
//        --typescript <typescript module dir> [--service <name>] [--out routes.json]
//
//   - .next/routes-manifest.json: every route Next serves, static and
//     dynamic (its page key: /product/[productId]).
//   - .next/app-paths-manifest.json: each app/ entry's source path below
//     app/ (/dashboard/(overview)/page, /seed/route).
//   - .next/server/pages-manifest.json: each pages/ entry.
//
// A route's source file is the app/ entry Next names, or the pages/ file
// Next resolves for the page (pages/<page>.<ext> or pages/<page>/index.<ext>).
// Its method is GET for a page, ANY for a pages/api route, and for an app/
// route handler one route per exported HTTP-method function. The handler is
// named from the source with the TypeScript compiler: the default export's
// name (a wrapper call's innermost named argument), the method function's
// name for a route handler. Next's own entries that are not source routes
// (_app, _document, _error, 404, 500, /_not-found, metadata files such as
// /favicon.ico) are listed under excluded. [x], [...x] and [[...x]] are {}.

import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'

const o = {}
const argv = process.argv.slice(2)
for (let i = 0; i < argv.length; i += 2) o[argv[i].slice(2)] = argv[i + 1]
const require = createRequire(import.meta.url)
const ts = require(o.typescript)

const next = path.join(o.app, '.next')
const read = (p) => fs.existsSync(p) ? JSON.parse(fs.readFileSync(p, 'utf8')) : {}
const routesManifest = read(path.join(next, 'routes-manifest.json'))
const appPaths = read(path.join(next, 'app-paths-manifest.json'))
const appPathsServer = read(path.join(next, 'server', 'app-paths-manifest.json'))
const pagesManifest = read(path.join(next, 'server', 'pages-manifest.json'))
const EXTS = ['.tsx', '.ts', '.jsx', '.js']
const METHODS = new Set(['GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS'])

function firstExisting (cands) { return cands.find((c) => fs.existsSync(c)) || '' }
function rootOf (name) {
  for (const base of [o.app, path.join(o.app, 'src')]) if (fs.existsSync(path.join(base, name))) return path.join(base, name)
  return ''
}
const appDir = rootOf('app')
const pagesDir = rootOf('pages')

function template (page) {
  const segs = page.split('/').map((s) => /^\[.*\]$/.test(s) ? '{}' : s)
  const p = segs.join('/')
  return p === '' ? '/' : p
}

function source (file) {
  const ext = path.extname(file)
  const kind = ext === '.tsx' ? ts.ScriptKind.TSX : ext === '.jsx' ? ts.ScriptKind.JSX : ext === '.ts' ? ts.ScriptKind.TS : ts.ScriptKind.JS
  return ts.createSourceFile(file, fs.readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true, kind)
}
const hasExport = (n) => (ts.getCombinedModifierFlags(n) & ts.ModifierFlags.Export) !== 0
const hasDefault = (n) => (ts.getCombinedModifierFlags(n) & ts.ModifierFlags.Default) !== 0

function exprName (e) {
  while (ts.isParenthesizedExpression(e) || ts.isAsExpression(e)) e = e.expression
  if (ts.isIdentifier(e)) return e.text
  if (ts.isCallExpression(e)) {
    for (let i = e.arguments.length - 1; i >= 0; i--) {
      const n = exprName(e.arguments[i])
      if (n) return n
    }
  }
  if ((ts.isFunctionExpression(e) || ts.isClassExpression(e)) && e.name) return e.name.text
  return ''
}

function defaultExport (sf) {
  for (const st of sf.statements) {
    if ((ts.isFunctionDeclaration(st) || ts.isClassDeclaration(st)) && hasExport(st) && hasDefault(st)) return st.name ? st.name.text : ''
    if (ts.isExportAssignment(st) && !st.isExportEquals) return exprName(st.expression)
    if (ts.isExportDeclaration(st) && st.exportClause && ts.isNamedExports(st.exportClause)) {
      for (const el of st.exportClause.elements) if (el.name.text === 'default') return (el.propertyName || el.name).text
    }
  }
  return null
}

function methodExports (sf) {
  const out = []
  for (const st of sf.statements) {
    if (ts.isFunctionDeclaration(st) && hasExport(st) && st.name && METHODS.has(st.name.text)) out.push([st.name.text, st.name.text])
    if (ts.isVariableStatement(st) && hasExport(st)) {
      for (const d of st.declarationList.declarations) if (ts.isIdentifier(d.name) && METHODS.has(d.name.text)) out.push([d.name.text, d.name.text])
    }
    if (ts.isExportDeclaration(st) && st.exportClause && ts.isNamedExports(st.exportClause)) {
      for (const el of st.exportClause.elements) if (METHODS.has(el.name.text)) out.push([el.name.text, (el.propertyName || el.name).text])
    }
  }
  return out
}

const routes = []
const excluded = []
const service = o.service || path.relative(o.repo, o.app)
const rel = (f) => path.relative(o.repo, f)

const served = new Set([...(routesManifest.staticRoutes || []), ...(routesManifest.dynamicRoutes || [])].map((r) => r.page))
const appEntries = { ...appPathsServer, ...appPaths }
const appByPage = new Map() // url page -> app entries
for (const entry of Object.keys(appEntries)) {
  const m = entry.match(/^(.*)\/(page|route)$/)
  if (!m) { excluded.push({ entry, reason: 'not a page or route entry' }); continue }
  const url = m[1].split('/').filter((s) => !(s.startsWith('(') && s.endsWith(')')) && !s.startsWith('@')).join('/') || '/'
  if (!appByPage.has(url)) appByPage.set(url, [])
  appByPage.get(url).push({ entry, kind: m[2], dir: m[1] })
}

for (const page of [...served].sort()) {
  const app = appByPage.get(page)
  if (app) {
    for (const a of app) {
      const file = firstExisting(EXTS.map((e) => path.join(appDir, a.dir, a.kind + e)))
      if (!file) { excluded.push({ page, entry: a.entry, reason: 'metadata or generated route (no page/route source)' }); continue }
      const sf = source(file)
      if (a.kind === 'page') {
        routes.push({ service, method: 'GET', path: template(page), handler: defaultExport(sf) || '', file: rel(file), router: 'app' })
      } else {
        for (const [method, local] of methodExports(sf)) routes.push({ service, method, path: template(page), handler: local, file: rel(file), router: 'app' })
      }
    }
    continue
  }
  if (pagesManifest[page] !== undefined && pagesDir) {
    if (['/_app', '/_document', '/_error', '/404', '/500'].includes(page)) { excluded.push({ page, reason: 'special page' }); continue }
    const base = page === '/' ? '/index' : page
    const file = firstExisting([...EXTS.map((e) => path.join(pagesDir, base + e)), ...EXTS.map((e) => path.join(pagesDir, base, 'index' + e))])
    if (!file) { excluded.push({ page, reason: 'no pages/ source' }); continue }
    const api = page === '/api' || page.startsWith('/api/')
    routes.push({ service, method: api ? 'ANY' : 'GET', path: template(page), handler: defaultExport(source(file)) || '', file: rel(file), router: 'pages' })
    continue
  }
  excluded.push({ page, reason: 'served by Next itself (no app/ or pages/ entry)' })
}
for (const page of Object.keys(pagesManifest)) {
  if (!served.has(page)) excluded.push({ page, reason: 'pages-manifest entry Next does not route (special page)' })
}

const key = (r) => [r.service, r.path, r.method, r.handler].join('\u0000')
routes.sort((a, b) => key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0)
excluded.sort((a, b) => JSON.stringify(a) < JSON.stringify(b) ? -1 : 1)
let version = ''
try { version = JSON.parse(fs.readFileSync(path.join(o.app, 'node_modules', 'next', 'package.json'), 'utf8')).version } catch {}
const doc = { tool: 'next ' + version + ' build --experimental-build-mode=compile: routes-manifest.json, app-paths-manifest.json, pages-manifest.json (manifest_routes.mjs)', routes, excluded }
const json = JSON.stringify(doc, null, 2) + '\n'
if (o.out) fs.writeFileSync(o.out, json)
else process.stdout.write(json)
