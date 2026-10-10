// preload.mjs — loaded with `node --import` before a Nest sample's own
// compiled main.js (dump_routes.mjs runs it). The sample's bootstrap runs
// unchanged (its setGlobalPrefix, enableVersioning, adapters, modules) with
// three hooks:
//
//   - NestFactory.create: the created app's HTTP adapter gets instance-level
//     wrappers on its route methods (get, post, …, all), so every route
//     Nest's RouterExplorer registers is recorded as the adapter receives
//     it: the final path and the handler, which Nest names
//     `${Controller}.${method}` before registering it (router-explorer.ts).
//     FATHOM_NEST_PREVIEW=1 adds {preview: true} (providers are not
//     instantiated, so a sample whose providers open a database still maps).
//   - startAllMicroservices: a no-op (no transport is opened).
//   - listen: init() (which maps the routes) instead of binding a port,
//     then the recorded routes are printed and the process exits.
//
// Each route is one line on stdout: "\u0001" + JSON {method, path, handler}.

import { createRequire } from 'node:module'
import { pathToFileURL } from 'node:url'
import path from 'node:path'

const main = process.argv[1]
const req = createRequire(main)
const core = await import(pathToFileURL(req.resolve('@nestjs/core')).href)
const { NestFactory, NestApplication } = core

const METHODS = ['get', 'post', 'put', 'delete', 'patch', 'options', 'head', 'all', 'search',
  'propfind', 'proppatch', 'mkcol', 'copy', 'move', 'lock', 'unlock']
const routes = []

const create = NestFactory.create.bind(NestFactory)
NestFactory.create = async function (module, ...rest) {
  if (process.env.FATHOM_NEST_PREVIEW === '1') {
    const last = rest.length && rest[rest.length - 1] && typeof rest[rest.length - 1] === 'object' && !rest[rest.length - 1].getInstance ? rest.length - 1 : -1
    if (last >= 0) rest[last] = { ...rest[last], preview: true }
    else rest.push({ preview: true })
  }
  const app = await create(module, ...rest)
  const adapter = app.getHttpAdapter()
  for (const m of METHODS) {
    const orig = adapter[m]
    if (typeof orig !== 'function') continue
    adapter[m] = function (...args) {
      const p = typeof args[0] === 'string' ? args[0] : null
      const fn = args[args.length - 1]
      if (p !== null && typeof fn === 'function') {
        routes.push({ method: m === 'all' ? 'ANY' : m.toUpperCase(), path: p, handler: fn.name || '' })
      }
      return orig.apply(this, args)
    }
  }
  return app
}

NestApplication.prototype.startAllMicroservices = async function () { return this }
NestApplication.prototype.listen = async function () {
  await this.init()
  for (const r of routes) process.stdout.write('\u0001' + JSON.stringify(r) + '\n')
  process.exit(0)
}
void path
