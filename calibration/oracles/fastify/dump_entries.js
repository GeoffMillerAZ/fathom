'use strict'
// dump_entries.js — the Fastify route oracle for apps that build their own
// instance and call listen() (fastify/fastify's examples/): each entry runs
// in a child process with the checkout's fastify factory wrapped so every
// instance it creates gets an onRoute hook, and listen() becomes ready() +
// print + exit (an entry that never calls listen is readied after 3 s).
// Handlers are located by V8 and named by the TypeScript parser
// (../express/locate.js, ../express/namer.js); a HEAD route Fastify adds
// beside a GET is marked implicit, a handler in node_modules library.
//
//   node dump_entries.js --fastify <fastify checkout> --repo <repo root> \
//        --typescript <typescript module dir> [--out routes.json] <entry>...
//
// Output: {"tool", "routes": [{entry, method, path, handler, file, library,
// implicit}], "errors": [{entry, error}]}, sorted.

const path = require('node:path')
const fs = require('node:fs')
const { execFileSync } = require('node:child_process')

function args (argv) {
  const o = { entries: [] }
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]
    if (['--fastify', '--typescript', '--repo', '--out', '--one'].includes(a)) o[a.slice(2)] = argv[++i]
    else o.entries.push(a)
  }
  return o
}

function one (o) {
  const locate = require('../express/locate.js')()
  const namer = require('../express/namer.js')(o.typescript)
  const fpath = require.resolve(o.fastify)
  const orig = require(fpath)
  const instances = []
  const seen = []
  const rel = path.relative(o.repo, path.resolve(o.one))
  let dumped = false
  function dump () {
    if (dumped) return
    dumped = true
    const out = []
    for (const r of seen) {
      const loc = locate(r.fn)
      let handler = ''
      let file = ''
      let library = false
      if (loc && loc.file && fs.existsSync(loc.file)) {
        library = loc.file.includes(path.sep + 'node_modules' + path.sep)
        handler = namer.name(loc.file, loc.line0, loc.col0)
        file = path.relative(o.repo, loc.file)
      }
      out.push({ entry: rel, method: r.method, path: r.url, handler, file, library })
    }
    const bare = (p) => p.length > 1 ? p.replace(/\/+$/, '') : p
    for (const r of out) {
      if (r.method === 'HEAD' && out.some((g) => g.method === 'GET' && bare(g.path) === bare(r.path) && g.file === r.file && g.handler === r.handler)) r.implicit = true
    }
    for (const r of out) process.stdout.write('\u0001' + JSON.stringify({ route: r }) + '\n')
    process.exit(0)
  }
  function wrapped (opts) {
    const app = orig(opts)
    instances.push(app)
    app.addHook('onRoute', (ro) => {
      for (const m of [].concat(ro.method)) seen.push({ method: m, url: ro.url, fn: ro.handler })
    })
    app.listen = function () {
      app.ready().then(dump, (e) => {
        process.stdout.write('\u0001' + JSON.stringify({ error: { entry: rel, error: String(e && e.message || e).split('\n')[0] } }) + '\n')
        dump()
      })
      return new Promise(() => {})
    }
    return app
  }
  Object.assign(wrapped, orig)
  wrapped.fastify = wrapped
  wrapped.default = wrapped
  require.cache[fpath].exports = wrapped
  setTimeout(() => { Promise.all(instances.map((a) => a.ready().catch(() => {}))).then(dump) }, 3000).unref()
  const entry = path.resolve(o.one)
  const done = (e) => {
    if (e) process.stdout.write('\u0001' + JSON.stringify({ error: { entry: rel, error: String(e && e.message || e).split('\n')[0] } }) + '\n')
  }
  if (/\.(mjs|ts|mts)$/.test(entry)) import(require('node:url').pathToFileURL(entry).href).catch(done)
  else {
    try { require(entry) } catch (e) { done(e) }
  }
  setTimeout(() => {}, 4000)
}

function main () {
  const o = args(process.argv.slice(2))
  if (o.one) return one(o)
  const routes = []
  const errors = []
  for (const e of o.entries) {
    let text = ''
    try {
      text = execFileSync(process.execPath, [__filename, '--fastify', o.fastify, '--typescript', o.typescript, '--repo', o.repo, '--one', e], { encoding: 'utf8', timeout: 60000, cwd: path.dirname(path.resolve(e)), stdio: ['ignore', 'pipe', 'pipe'] })
    } catch (err) {
      text = String(err.stdout || '')
      if (!text) errors.push({ entry: path.relative(o.repo, path.resolve(e)), error: String(err.message).split('\n')[0] })
    }
    for (const line of text.split('\n')) {
      if (!line.startsWith('\u0001')) continue
      const m = JSON.parse(line.slice(1))
      if (m.route) routes.push(m.route)
      if (m.error) errors.push(m.error)
    }
  }
  const key = (r) => [r.entry, r.path, r.method, r.handler].join('\u0000')
  routes.sort((a, b) => key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0)
  const version = JSON.parse(fs.readFileSync(path.join(o.fastify, 'package.json'), 'utf8')).version
  const doc = { tool: 'fastify ' + version + ' onRoute hook on every instance (dump_entries.js)', routes, errors }
  const json = JSON.stringify(doc, null, 2) + '\n'
  if (o.out) fs.writeFileSync(o.out, json)
  else process.stdout.write(json)
}

main()
