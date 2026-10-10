// dump_routes.mjs — the Fastify route oracle (tier A, runtime): the app is
// registered the way its own server entry registers it, with Fastify's
// onRoute hook recording every route Fastify builds (final url with every
// register prefix and @fastify/autoload folder prefix applied), then
// ready() and printRoutes() for the count cross-check.
//
//   node dump_routes.mjs --repo <repo root> --app <compiled app plugin .js> \
//        --typescript <typescript module dir> [--fp] [--service <name>] \
//        [--env K=V,K=V] [--out routes.json]
//
// --fp wraps the plugin in fastify-plugin, as the fastify demo's server.ts
// does. A handler is located by V8 (../express/locate.js) in the compiled
// file and named by ../express/namer.js (the TypeScript parser, fathom's
// dialect naming), then filed under the source path (dist/x.js ->
// src/x.ts). A route whose handler lives in node_modules is a library's
// (swagger-ui, static files): kept, marked library. A HEAD route Fastify
// adds for a GET (exposeHeadRoutes) is marked implicit.

import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { pathToFileURL } from 'node:url'

const o = {}
const argv = process.argv.slice(2)
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === '--fp') o.fp = true
  else o[argv[i].slice(2)] = argv[++i]
}
for (const kv of (o.env || '').split(',').filter(Boolean)) {
  const [k, ...v] = kv.split('=')
  process.env[k] = v.join('=')
}
const here = createRequire(import.meta.url)
const locate = here('../express/locate.js')()
const namer = here('../express/namer.js')(o.typescript)
const appReq = createRequire(o.app)
const Fastify = (await import(pathToFileURL(appReq.resolve('fastify')).href)).default
const plugin = (await import(pathToFileURL(o.app).href)).default

const app = Fastify({ logger: false })
const seen = []
app.addHook('onRoute', (opts) => {
  const methods = Array.isArray(opts.method) ? opts.method : [opts.method]
  for (const m of methods) seen.push({ method: m, url: opts.url, fn: opts.handler, implicitHead: false })
})
if (o.fp) {
  const fp = (await import(pathToFileURL(appReq.resolve('fastify-plugin')).href)).default
  app.register(fp(plugin))
} else {
  app.register(plugin)
}
await app.ready()
const printed = app.printRoutes({ commonPrefix: false })

const service = o.service || ''
const routes = []
for (const r of seen) {
  const loc = locate(r.fn)
  let handler = ''
  let file = ''
  let library = false
  if (loc && loc.file && fs.existsSync(loc.file)) {
    library = loc.file.includes(path.sep + 'node_modules' + path.sep)
    handler = namer.name(loc.file, loc.line0, loc.col0)
    file = path.relative(o.repo, loc.file)
    if (file.startsWith('dist/')) file = file.replace(/^dist\//, 'src/').replace(/\.js$/, '.ts')
  }
  routes.push({ service, method: r.method === 'HEAD' ? 'HEAD' : r.method, path: r.url, handler, file, library })
}
// A HEAD route with the same url (trailing slash aside: a prefixed '/'
// is also served at the prefix with one) and handler as a GET is
// exposeHeadRoutes'.
const bare = (p) => p.length > 1 ? p.replace(/\/+$/, '') : p
for (const r of routes) {
  if (r.method === 'HEAD' && routes.some((g) => g.method === 'GET' && bare(g.path) === bare(r.path) && g.file === r.file && g.handler === r.handler)) r.implicit = true
}
await app.close()
const key = (r) => [r.service, r.path, r.method, r.handler].join('\u0000')
routes.sort((a, b) => key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0)
const version = JSON.parse(fs.readFileSync(path.join(path.dirname(appReq.resolve('fastify')), 'package.json'), 'utf8')).version
const doc = { tool: 'fastify ' + version + ' onRoute hook + printRoutes (dump_routes.mjs)', routes, print_routes: printed.split('\n').filter(Boolean) }
const json = JSON.stringify(doc, null, 2) + '\n'
if (o.out) fs.writeFileSync(o.out, json)
else process.stdout.write(json)
process.exit(0)
