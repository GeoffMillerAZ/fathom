// dump_routes.mjs — the Nest route oracle (tier A, runtime): each sample
// app is compiled with the TypeScript compiler and bootstrapped by its own
// main.ts with preload.mjs, which records every route Nest's RouterExplorer
// hands the HTTP adapter (final path, method, Controller.method handler).
//
//   node dump_routes.mjs --repo <nest checkout> --env <dir with node_modules> \
//        [--out routes.json] <sample dir under repo>...
//
// env holds one node_modules with the samples' pinned dependencies
// (calibration/oracles/nest/README.md); each sample's src/ is copied to
// env/work/<sample>/ (so its imports resolve there), compiled with the
// sample's own tsconfig compilerOptions (outDir dist, emit on type errors),
// and run. A sample whose run maps no route (a provider that opens a
// database never lets init() finish) is run again with Nest's preview
// mode, in which providers are not instantiated. A route whose handler is
// not named Controller.method was registered by a library (serve-static),
// not by a controller: it is kept and marked controller: false.

import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const o = { samples: [] }
const argv = process.argv.slice(2)
for (let i = 0; i < argv.length; i++) {
  if (['--repo', '--env', '--out'].includes(argv[i])) o[argv[i].slice(2)] = argv[++i]
  else o.samples.push(argv[i])
}

function stripJSONComments (s) {
  return s.replace(/("(?:\\.|[^"\\])*")|\/\/[^\n]*|\/\*[\s\S]*?\*\//g, (m, str) => str || '')
}

// A sample is "<dir>" (its app is <dir>/src/main.ts) or "<dir>:<subdir>"
// for a second app of the same sample (<dir>/<subdir>/main.ts).
function prepare (spec) {
  const [sample, sub = 'src'] = spec.split(':')
  const srcDir = path.join(o.repo, sample)
  const work = path.join(o.env, 'work', spec.replace(/[/:]/g, '__'))
  fs.rmSync(work, { recursive: true, force: true })
  fs.mkdirSync(work, { recursive: true })
  fs.cpSync(path.join(srcDir, sub), path.join(work, 'src'), { recursive: true })
  const pkg = JSON.parse(fs.readFileSync(path.join(srcDir, 'package.json'), 'utf8'))
  fs.writeFileSync(path.join(work, 'package.json'), JSON.stringify({ name: 'w', private: true, type: pkg.type || 'commonjs' }))
  let co = {}
  const tsc = path.join(srcDir, 'tsconfig.json')
  if (fs.existsSync(tsc)) co = JSON.parse(stripJSONComments(fs.readFileSync(tsc, 'utf8'))).compilerOptions || {}
  Object.assign(co, {
    outDir: './dist', rootDir: './src', types: ['node'], typeRoots: [path.join(o.env, 'node_modules', '@types')],
    noEmitOnError: false, incremental: false, declaration: false, sourceMap: false, skipLibCheck: true,
    allowJs: true, experimentalDecorators: true, emitDecoratorMetadata: true
  })
  if (pkg.type === 'module' && !co.module) Object.assign(co, { module: 'nodenext', moduleResolution: 'nodenext' })
  delete co.baseUrl
  delete co.paths
  fs.writeFileSync(path.join(work, 'tsconfig.json'), JSON.stringify({ compilerOptions: co, include: ['src/**/*'], exclude: ['**/*.spec.ts', '**/*.test.ts'] }, null, 2))
  try {
    execFileSync(process.execPath, [path.join(o.env, 'node_modules', 'typescript', 'bin', 'tsc'), '-p', work], { cwd: work, stdio: 'pipe' })
  } catch (e) { /* type errors: tsc still emits */ }
  copyAssets(path.join(work, 'src'), path.join(work, 'dist'))
  return work
}

// copyAssets copies what tsc does not emit (.proto, .hbs, .json) next to the
// compiled files, as the Nest CLI's assets option does.
function copyAssets (from, to) {
  for (const e of fs.readdirSync(from, { withFileTypes: true })) {
    const a = path.join(from, e.name)
    const b = path.join(to, e.name)
    if (e.isDirectory()) copyAssets(a, b)
    else if (!/\.(ts|js|mjs|cjs|mts|cts)$/.test(e.name)) {
      fs.mkdirSync(to, { recursive: true })
      fs.copyFileSync(a, b)
    }
  }
}

function run (work, preview) {
  const main = path.join(work, 'dist', 'main.js')
  if (!fs.existsSync(main)) return { error: 'no dist/main.js' }
  let text = ''
  let error = ''
  try {
    text = execFileSync(process.execPath, ['--import', path.join(here, 'preload.mjs'), main], {
      cwd: work, encoding: 'utf8', timeout: 90000, stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env, FATHOM_NEST_PREVIEW: preview ? '1' : '0', NODE_ENV: 'test' }
    })
  } catch (e) {
    text = String(e.stdout || '')
    error = String(e.stderr || e.message).split('\n').filter((l) => /Error|error/.test(l)).slice(0, 1).join(' ').slice(0, 300)
  }
  const routes = []
  for (const line of text.split('\n')) if (line.startsWith('\u0001')) routes.push(JSON.parse(line.slice(1)))
  return { routes, error }
}

const routes = []
const samples = []
for (const s of o.samples) {
  const work = prepare(s)
  let mode = 'run'
  let r = run(work, false)
  if (!r.routes || r.routes.length === 0) {
    mode = 'preview'
    r = run(work, true)
  }
  samples.push({ sample: s, mode, routes: (r.routes || []).length, error: r.routes && r.routes.length ? '' : (r.error || '') })
  const service = s.includes(':') ? s.replace(':', '/') : s
  for (const x of r.routes || []) routes.push({ service, ...x, controller: /^[A-Za-z_$][\w$]*\.[A-Za-z_$][\w$]*$/.test(x.handler) })
}
const key = (r) => [r.service, r.path, r.method, r.handler].join('\u0000')
routes.sort((a, b) => key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0)
const core = JSON.parse(fs.readFileSync(path.join(o.env, 'node_modules', '@nestjs', 'core', 'package.json'), 'utf8'))
const doc = { tool: '@nestjs/core ' + core.version + ' RouterExplorer via the HTTP adapter (dump_routes.mjs, preload.mjs)', routes, samples }
const json = JSON.stringify(doc, null, 2) + '\n'
if (o.out) fs.writeFileSync(o.out, json)
else process.stdout.write(json)
