'use strict'
// dump_routes.js — the Express route oracle (tier A, runtime): load each
// app, then walk Express's own router stack.
//
//   node dump_routes.js --express <express checkout> --typescript <typescript module dir> \
//        --repo <repo root> [--out routes.json] <entry.js>...
//
// Each entry runs in its own child process. Before the entry loads, the
// router package's use() and route() are wrapped only to remember the path
// a layer was mounted at (Express 5's Layer keeps a matcher, not the path)
// and which routers registered routes, and application.use() to remember
// which sub-app a mounted_app layer runs. Then the router stack is walked
// from the entry's exported app: a route's path is the mount paths above it
// joined with route.path; its methods are route.methods (`_all` is ANY); its
// handler is the last function of route.stack, located by V8
// ([[FunctionLocation]], locate.js) and named the way fathom's TS/JS
// dialect names that definition (namer.js, the TypeScript parser). A router
// or app that registered routes but is never reached from the exported app
// is walked at "" and marked unmounted: its routes are served at those
// paths only if something mounts it at runtime in a way the stack walk
// cannot see (a vhost, a closure).
//
// Output: {"tool": …, "routes": [{entry, method, path, handler, handler_file,
// handler_line, mounted}], "errors": [{entry, error}]}, sorted.

const path = require('node:path')
const fs = require('node:fs')
const { execFileSync } = require('node:child_process')
const HTTP_METHODS = require('node:http').METHODS.map((m) => m.toLowerCase())

function args (argv) {
  const o = { entries: [] }
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]
    if (a === '--express' || a === '--typescript' || a === '--repo' || a === '--out' || a === '--one') o[a.slice(2)] = argv[++i]
    else o.entries.push(a)
  }
  return o
}

function one (o) {
  const locate = require('./locate.js')()
  const namer = require('./namer.js')(o.typescript)
  const Router = require(path.join(o.express, 'node_modules', 'router'))
  const express = require(o.express)
  const routers = new Set()
  const origUse = Router.prototype.use
  Router.prototype.use = function (p) {
    const before = this.stack.length
    const out = origUse.apply(this, arguments)
    const mount = typeof p === 'function' || p === undefined ? '/' : p
    for (let i = before; i < this.stack.length; i++) this.stack[i].__fathomMount = mount
    return out
  }
  const origRoute = Router.prototype.route
  Router.prototype.route = function () {
    routers.add(this)
    return origRoute.apply(this, arguments)
  }
  const appUse = express.application.use
  express.application.use = function () {
    const r = this.router
    const before = r.stack.length
    const out = appUse.apply(this, arguments)
    const subs = Array.prototype.slice.call(arguments).flat(Infinity).filter((f) => f && typeof f.handle === 'function' && typeof f.set === 'function')
    let k = 0
    for (let i = before; i < r.stack.length; i++) {
      if (r.stack[i].handle && r.stack[i].handle.name === 'mounted_app' && k < subs.length) r.stack[i].__fathomApp = subs[k++]
    }
    return out
  }

  const entry = path.resolve(o.one)
  const rel = path.relative(o.repo, entry)
  const out = []
  let app
  try {
    app = require(entry)
  } catch (e) {
    process.stdout.write('\u0001' + JSON.stringify({ error: { entry: rel, error: String(e && e.message || e).split('\n')[0] } }) + '\n')
    process.exit(0)
  }
  const reached = new Set()
  function join (a, b) {
    const s = (a.replace(/\/+$/, '') + '/' + String(b).replace(/^\/+/, '')).replace(/\/+$/, '')
    return s === '' ? '/' : s
  }
  function walk (router, prefix, mounted, depth) {
    if (!router || !router.stack || depth > 16 || reached.has(router)) return
    reached.add(router)
    for (const layer of router.stack) {
      if (layer.route) {
        const paths = Array.isArray(layer.route.path) ? layer.route.path : [layer.route.path]
        let methods = Object.keys(layer.route.methods).filter((m) => layer.route.methods[m])
        // app.all() registers every method one by one: that is ANY.
        if (HTTP_METHODS.every((m) => methods.includes(m))) methods = ['_all']
        const stack = layer.route.stack || []
        const fn = stack.length ? stack[stack.length - 1].handle : null
        const loc = locate(fn)
        let handler = ''
        let hfile = ''
        let hline = 0
        if (loc && loc.file && fs.existsSync(loc.file)) {
          hfile = path.relative(o.repo, loc.file)
          hline = loc.line0 + 1
          handler = namer.name(loc.file, loc.line0, loc.col0)
        }
        for (const p of paths) {
          if (typeof p !== 'string') continue
          for (const m of methods) {
            out.push({ entry: rel, method: m === '_all' ? 'ANY' : m.toUpperCase(), path: join(prefix, p), handler, handler_file: hfile, handler_line: hline, mounted })
          }
        }
        continue
      }
      const mount = layer.__fathomMount
      if (typeof mount !== 'string') continue
      if (layer.__fathomApp) walk(layer.__fathomApp.router, join(prefix, mount), mounted, depth + 1)
      else if (layer.handle && Array.isArray(layer.handle.stack)) walk(layer.handle, join(prefix, mount), mounted, depth + 1)
    }
  }
  if (app && app.router) walk(app.router, '', true, 0)
  for (const r of routers) if (!reached.has(r)) walk(r, '', false, 0)
  for (const r of out) process.stdout.write('\u0001' + JSON.stringify({ route: r }) + '\n')
  process.exit(0)
}

function main () {
  const o = args(process.argv.slice(2))
  if (o.one) return one(o)
  const routes = []
  const errors = []
  for (const e of o.entries) {
    let text = ''
    try {
      text = execFileSync(process.execPath, [__filename, '--express', o.express, '--typescript', o.typescript, '--repo', o.repo, '--one', e], { encoding: 'utf8', timeout: 60000, cwd: path.dirname(path.resolve(e)), env: { ...process.env, NODE_ENV: 'test' } })
    } catch (err) {
      text = String(err.stdout || '')
      if (!text) errors.push({ entry: path.relative(o.repo, path.resolve(e)), error: String(err.message).split('\n')[0] })
    }
    for (const line of text.split('\n')) {
      if (!line.startsWith('\u0001')) continue // the app's own output
      const m = JSON.parse(line.slice(1))
      if (m.route) routes.push(m.route)
      if (m.error) errors.push(m.error)
    }
  }
  const key = (r) => [r.entry, r.path, r.method, r.handler].join('\u0000')
  routes.sort((a, b) => key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0)
  const doc = { tool: 'express ' + require(path.join(o.express, 'package.json')).version + ' router stack walk (dump_routes.js)', routes, errors }
  const json = JSON.stringify(doc, null, 2) + '\n'
  if (o.out) fs.writeFileSync(o.out, json)
  else process.stdout.write(json)
}

main()
