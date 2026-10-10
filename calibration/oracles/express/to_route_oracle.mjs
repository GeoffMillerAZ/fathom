// to_route_oracle.mjs — turns a Node framework dump (express/dump_routes.js,
// nest/dump_routes.mjs, fastify/dump_routes.mjs, nextjs/manifest_routes.mjs)
// into the route-oracle files `indexbench route-oracle` reads
// (calibration/oracles/routes/README.md): one file per subject, each
// {tool, tool_version, subject, sha, licence, routes: [{method, path, handler}]}.
//
//   node to_route_oracle.mjs --in dump.json --outdir DIR --repo-name owner/repo \
//        --sha SHA --licence SPDX --tool NAME --tool-version V \
//        [--group entry|entry-dir|service|one] [--subject NAME] [--groups g1,g2]
//
// --group entry: one subject per entry file (several apps in one folder);
// entry-dir: one subject per directory of a route's entry
// (express/examples/<app>/index.js -> examples/<app>); service: one per
// route's service field; one: a single subject named --subject.
//
// Kept: every route of the dump except a library's (controller: false,
// library: true) and the HEAD Fastify adds beside a GET (implicit).
// Duplicates on (method, path, handler) collapse. --groups names subjects
// that must be written even when the framework serves no route there (an
// app the dump read and found routeless), so fathom's extras there count. The subject is
// "<repo-name>/<group>" and the file "<group with / as __>.json".

import fs from 'node:fs'
import path from 'node:path'

const o = { group: 'one' }
const argv = process.argv.slice(2)
for (let i = 0; i < argv.length; i += 2) o[argv[i].slice(2)] = argv[i + 1]
const dump = JSON.parse(fs.readFileSync(o.in, 'utf8'))
const groups = new Map()
for (const r of dump.routes) {
  if (r.controller === false || r.library === true || r.implicit === true) continue
  let g = o.subject || ''
  if (o.group === 'entry') g = r.entry
  if (o.group === 'entry-dir') g = path.dirname(r.entry)
  if (o.group === 'service') g = r.service
  if (!groups.has(g)) groups.set(g, new Map())
  const k = [r.method, r.path, r.handler].join('\u0000')
  groups.get(g).set(k, { method: r.method, path: r.path, handler: r.handler })
}
for (const g of (o.groups || '').split(',').filter(Boolean)) if (!groups.has(g)) groups.set(g, new Map())
fs.mkdirSync(o.outdir, { recursive: true })
for (const [g, m] of [...groups].sort()) {
  const routes = [...m.values()].sort((a, b) => (a.path + ' ' + a.method + ' ' + a.handler) < (b.path + ' ' + b.method + ' ' + b.handler) ? -1 : 1)
  const doc = { tool: o.tool, tool_version: o['tool-version'], subject: o['repo-name'] + (g ? '/' + g : ''), sha: o.sha, licence: o.licence, routes }
  fs.writeFileSync(path.join(o.outdir, (g || 'subject').replace(/\//g, '__') + '.json'), JSON.stringify(doc, null, 2) + '\n')
}
process.stdout.write([...groups.keys()].sort().join('\n') + '\n')
