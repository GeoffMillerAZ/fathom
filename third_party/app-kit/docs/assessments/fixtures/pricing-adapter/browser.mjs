import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import http from 'node:http'
import crypto from 'node:crypto'
import { pathToFileURL } from 'node:url'

const [kitArg, probeArg] = process.argv.slice(2)
if (!kitArg || !probeArg) throw new Error('usage: browser.mjs APP_KIT_ROOT PRIVATE_TEMP_PROBE_DIR')
const kit = fs.realpathSync(kitArg), probe = fs.realpathSync(probeArg), temp = fs.realpathSync(os.tmpdir()) + path.sep
if (!probe.startsWith(temp) || !fs.statSync(probe).isDirectory() || fs.lstatSync(probeArg).isSymbolicLink()) throw new Error('probe dir must be a real system-temp child')
const required = ['source.html', 'baseline.html', 'long.html', 'defaults.json', 'artifact.json']
for (const name of required) { const p = path.join(probe, name), s = fs.lstatSync(p); if (!s.isFile() || s.isSymbolicLink()) throw new Error(`unsafe fixture: ${name}`) }
const out = path.join(probe, 'browser-output')
if (fs.existsSync(out)) throw new Error('browser-output already exists')
fs.mkdirSync(out, { mode: 0o700 })
const digest = data => crypto.createHash('sha256').update(data).digest('hex')
const read = p => fs.readFileSync(p)
const assertFile = (p, size, sha) => { const b = read(p); if (b.length !== size || digest(b) !== sha) throw new Error(`provenance mismatch: ${path.basename(p)}`); return b.toString() }
const ref = path.join(kit, 'data/design-assets/previews/p02-pricing/62a91c913f3860527b9d77b0b2227595d21b35ae1d06fb37a03a55091a30c8b8')
assertFile(path.join(ref, 'receipt.json'), 2711, '62a91c913f3860527b9d77b0b2227595d21b35ae1d06fb37a03a55091a30c8b8')
const prepared = {
  light: assertFile(path.join(ref, 'original-light.html'), 493525, '5d3a5473878298809b4e3c6d0a142e8379c56ffc974ee1a7dcadce0614ca04ad'),
  dark: assertFile(path.join(ref, 'original-dark.html'), 493524, '6b0352450e4f1672e6e42e64e8c0471a0461430bf9dcf4bf3e89cb0e5c8f148a'),
}
const source = assertFile(path.join(probe, 'source.html'), 4710, 'f545f273689b9a4312e5d0eeb66c15dc21d7d318e102e37c39b40f494e017e0f')
if ((source.match(/href="#"/g) || []).length !== 1) throw new Error('source placeholder count changed')
const sourceFixture = source.replace('href="#"', 'href="/pricing/access"')
const target = read(path.join(probe, 'baseline.html')).toString(), long = read(path.join(probe, 'long.html')).toString()
for (const [name, body] of [['target', target], ['long', long]]) {
  const href = name === 'target' ? 'href="/pricing/access"' : 'href="/pricing/access?fixture=long"'
  if ((body.match(/<a\b/g) || []).length !== 1 || (body.match(new RegExp(href.replace(/[?]/g, '\\?'), 'g')) || []).length !== 1) throw new Error(`${name} CTA changed`)
  if (/<(?:script|form|button)\b/i.test(body)) throw new Error(`${name} invented behavior`)
}
const defaults = JSON.parse(read(path.join(probe, 'defaults.json'))), artifact = JSON.parse(read(path.join(probe, 'artifact.json')))
if (defaults.URL !== '/pricing/access' || artifact?.source?.sha256 !== 'f545f273689b9a4312e5d0eeb66c15dc21d7d318e102e37c39b40f494e017e0f' || artifact.verification !== 'not-verified') throw new Error('fixture identity changed')
const body = (shell, fragment) => { const hits = shell.match(/<body>[\s\S]*<\/body>/g) || []; if (hits.length !== 1) throw new Error('prepared body boundary changed'); const doc = shell.replace(hits[0], `<body>${fragment}</body>`); if (Buffer.byteLength(doc) > 1048576) throw new Error('document exceeds 1 MiB'); return doc }
const docs = new Map()
for (const theme of ['light', 'dark']) for (const [kind, fragment] of [['source', sourceFixture], ['target', target], ['long', long]]) docs.set(`/${kind}-${theme}`, body(prepared[theme], fragment))
const csp = "default-src 'none'; style-src 'sha256-L8goulLPnZMfYSEGN0oAcVH2CGC2EAQvZohnOLtSAgA='; font-src data:; script-src 'none'; connect-src 'none'; img-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'none'"
const serverErrors = [], requests = []
const server = http.createServer((req, res) => {
  const host = req.headers.host, u = new URL(req.url, `http://${host}`)
  if (req.method !== 'GET' || host !== server.address()?.address + ':' + server.address()?.port || u.username || u.password) { res.writeHead(403).end('refused'); return }
  requests.push({ method: req.method, path: u.pathname, query: u.search })
  let page = docs.get(u.pathname)
  if (u.pathname === '/pricing/access' && (u.search === '' || u.search === '?fixture=long')) page = '<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Local pricing demonstration</title></head><body><main><h1>Local pricing demonstration</h1><p>No purchase or provider action occurred.</p></main></body></html>'
  if (!page || (u.search && u.pathname !== '/pricing/access')) { res.writeHead(404).end('not found'); return }
  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8', 'Content-Security-Policy': csp, 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff', 'Referrer-Policy': 'no-referrer' }).end(page)
})
server.on('clientError', e => serverErrors.push(e.code || 'clientError'))
const pwEntry = fs.realpathSync('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs')
const pwRoot = path.dirname(pwEntry), pkg = JSON.parse(read(path.join(pwRoot, 'package.json')))
if (pkg.version !== '1.63.0') throw new Error('Playwright version changed')
const { chromium } = await import(pathToFileURL(pwEntry).href)
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const browserVersion = browser.version()
const watchdog = setTimeout(() => browser.close(), 120000); watchdog.unref()
try { await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve) }) } catch (error) { await browser.close(); throw error }
const origin = `http://127.0.0.1:${server.address().port}`
const checks = [], failures = [], browserErrors = [], shots = []
const check = (name, ok, detail) => { checks.push({ name, ok: !!ok, ...(detail === undefined ? {} : { detail }) }); if (!ok) failures.push(name) }
const shot = async (page, name) => { const p = path.join(out, name), b = await page.screenshot({ path: p, fullPage: true }); shots.push({ name, size: b.length, sha256: digest(b) }); return b }
const boundary = async (context, tag) => context.route('**/*', route => { const r = route.request(), u = new URL(r.url()), exact = (docs.has(u.pathname) && u.search === '') || (u.pathname === '/pricing/access' && ['', '?fixture=long'].includes(u.search)); if (u.origin !== origin || r.method() !== 'GET' || !exact) { browserErrors.push(`${tag}:blocked`); return route.abort('blockedbyclient') } return route.continue() })
const watch = (page, tag) => { page.on('console', m => { if (['error','warning'].includes(m.type())) browserErrors.push(`${tag}:console-${m.type()}`) }); page.on('pageerror', () => browserErrors.push(`${tag}:pageerror`)); page.on('requestfailed', r => { if (r.failure()?.errorText !== 'net::ERR_BLOCKED_BY_CLIENT') browserErrors.push(`${tag}:requestfailed`) }) }
const geometry = page => page.evaluate(() => { const r=document.querySelector('a')?.getBoundingClientRect(); return { overflow:Math.max(0,document.documentElement.scrollWidth-innerWidth), anchors:document.querySelectorAll('a').length, cta:r?{width:r.width,height:r.height}:null } })
const contrast = page => page.evaluate(() => {
  const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1; const ctx = canvas.getContext('2d', { willReadFrequently:true })
  const rgba = s => { ctx.clearRect(0,0,1,1); ctx.fillStyle=s; ctx.fillRect(0,0,1,1); return [...ctx.getImageData(0,0,1,1).data].map((v,i)=>i===3?v/255:v) }
  const over = (a, b) => { const x = a[3] + b[3] * (1 - a[3]); return x ? [(a[0]*a[3]+b[0]*b[3]*(1-a[3]))/x,(a[1]*a[3]+b[1]*b[3]*(1-a[3]))/x,(a[2]*a[3]+b[2]*b[3]*(1-a[3]))/x,x] : [255,255,255,1] }
  const lum = c => { const f = v => (v/=255) <= .04045 ? v/12.92 : ((v+.055)/1.055)**2.4; return .2126*f(c[0])+.7152*f(c[1])+.0722*f(c[2]) }
  const rows = [], unknown = []
  for (const el of document.body.querySelectorAll('*')) {
    if (![...el.childNodes].some(n => n.nodeType === 3 && n.textContent.trim()) || !el.getClientRects().length) continue
    let bg = [255,255,255,1], chain = []; for (let n=el;n;n=n.parentElement) chain.unshift(n)
    let reason = ''; for (const n of chain) { const s=getComputedStyle(n); if (parseFloat(s.opacity)!==1 || s.backgroundImage!=='none') { reason='opacity, gradient, or image background'; break }; bg=over(rgba(s.backgroundColor),bg) }
    if (reason) { unknown.push({ path:[...document.querySelectorAll('*')].indexOf(el), reason }); continue }
    const cs = getComputedStyle(el), fg = over(rgba(cs.color), bg), ratio = (Math.max(lum(fg),lum(bg))+.05)/(Math.min(lum(fg),lum(bg))+.05)
    const large = parseFloat(cs.fontSize) >= 24 || (parseFloat(cs.fontSize) >= 18.66 && Number(cs.fontWeight) >= 700), limit = large ? 3 : 4.5
    rows.push({ path: [...document.querySelectorAll('*')].indexOf(el), ratio, limit })
  }
  return { sampled: rows.length, min: rows.length ? Math.min(...rows.map(x=>x.ratio)) : 0, failures: rows.filter(x=>x.ratio+1e-6<x.limit), unknown }
})
try {
  for (const theme of ['light', 'dark']) for (const width of [390, 1440]) {
    const context = await browser.newContext({ viewport: { width, height: 900 }, javaScriptEnabled: false, colorScheme: theme })
    context.setDefaultTimeout(10000); context.setDefaultNavigationTimeout(10000)
    await boundary(context, `pixels-${theme}-${width}`); const page = await context.newPage(); watch(page, `pixels-${theme}-${width}`)
    const a = await page.goto(`${origin}/source-${theme}`, { waitUntil: 'load' }); check(`source ${theme} ${width} status`, a?.status() === 200); const sb = await shot(page, `source-${theme}-${width}.png`), sc = await contrast(page), sg = await geometry(page)
    check(`source no overflow ${theme} ${width}`, sg.overflow === 0, sg.overflow); check(`source CTA 44x44 ${theme} ${width}`, sg.anchors === 1 && sg.cta?.width >= 44 && sg.cta?.height >= 44, sg.cta); check(`source absolute contrast ${theme} ${width}`, sc.failures.length === 0 && sc.unknown.length === 0, { classification:'inherited-source', failures:sc.failures.length, unknown:sc.unknown.length, min:sc.min })
    const b = await page.goto(`${origin}/target-${theme}`, { waitUntil: 'load' }); check(`target ${theme} ${width} status`, b?.status() === 200); const tb = await shot(page, `target-${theme}-${width}.png`), tc = await contrast(page), tg = await geometry(page)
    check(`target no overflow ${theme} ${width}`, tg.overflow === 0, tg.overflow); check(`target CTA 44x44 ${theme} ${width}`, tg.anchors === 1 && tg.cta?.width >= 44 && tg.cta?.height >= 44, tg.cta); check(`target absolute contrast ${theme} ${width}`, tc.failures.length === 0 && tc.unknown.length === 0, { classification:'adapted-target', failures:tc.failures.length, unknown:tc.unknown.length, min:tc.min })
    const sourceFails = new Map(sc.failures.map(x => [x.path, x])); const regressions = tc.failures.filter(x => !sourceFails.has(x.path))
    check(`baseline pixels ${theme} ${width}`, sb.equals(tb)); check(`contrast regression ${theme} ${width}`, regressions.length === 0, { inherited: sc.failures.length, target: tc.failures.length, regressions: regressions.length, sourceMin: sc.min, targetMin: tc.min })
    const l = await page.goto(`${origin}/long-${theme}`, { waitUntil: 'load' }); check(`long ${theme} ${width} status`, l?.status() === 200); const layout = await geometry(page); const lc = await contrast(page), inheritedRatios = sc.failures.map(x => [x.ratio,x.limit]); const longRegressions = lc.failures.filter(x => !inheritedRatios.some(([r,l]) => l === x.limit && Math.abs(r-x.ratio) < .01)); check(`long absolute contrast ${theme} ${width}`, lc.failures.length === 0 && lc.unknown.length === 0, { failures:lc.failures.length, unknown:lc.unknown.length, min:lc.min }); check(`long contrast regression ${theme} ${width}`, longRegressions.length === 0, { inheritedStyles: inheritedRatios.length, regressions: longRegressions.length }); check(`long no overflow ${theme} ${width}`, layout.overflow === 0, layout.overflow); check(`long CTA 44x44 ${theme} ${width}`, layout.anchors === 1 && layout.cta?.width >= 44 && layout.cta?.height >= 44, layout.cta); await shot(page, `long-${theme}-${width}.png`)
    await context.close()
  }
  for (const js of [false, true]) for (const mode of ['keyboard', 'pointer']) {
    const tag = `${js?'js':'nojs'}-${mode}`, context = await browser.newContext({ viewport: { width: js ? 1440 : 390, height: 900 }, javaScriptEnabled: js, colorScheme: js ? 'light' : 'dark' })
    context.setDefaultTimeout(10000); context.setDefaultNavigationTimeout(10000)
    await boundary(context, tag); const page = await context.newPage(); watch(page, tag); await page.goto(`${origin}/target-${js?'light':'dark'}`, { waitUntil: 'load' }); const link = page.locator('a')
    if (mode === 'keyboard') { await page.keyboard.press('Tab'); check(`${tag} CTA focused`, await link.evaluate(e => document.activeElement === e)); const box = await link.boundingBox(), focus = await link.evaluate(e => { const s=getComputedStyle(e); return { style:s.outlineStyle, width:parseFloat(s.outlineWidth) } }); check(`${tag} CTA 44px`, !!box && box.height >= 44, box?.height); check(`${tag} visible focus`, focus.style !== 'none' && focus.width >= 2, focus); await shot(page, `${tag}-focus.png`); await Promise.all([page.waitForURL(`${origin}/pricing/access`), page.keyboard.press('Enter')]) }
    else await Promise.all([page.waitForURL(`${origin}/pricing/access`), link.click()])
    check(`${tag} local destination`, page.url() === `${origin}/pricing/access` && await page.getByRole('heading', { name: 'Local pricing demonstration' }).count() === 1)
    await context.close()
  }
  check('no browser boundary errors', browserErrors.length === 0, browserErrors.length); check('no server errors', serverErrors.length === 0, serverErrors.length)
} catch (error) { browserErrors.push(`operational:${error?.name || 'Error'}`); check('probe completed', false) }
finally { clearTimeout(watchdog); await browser.close(); await new Promise(resolve => server.close(resolve)) }
shots.sort((a,b)=>a.name.localeCompare(b.name))
const receipt = { schema:1, generatedAt:new Date().toISOString(), scope:'controlled private browser probe; not target certification', playwright:{version:pkg.version,entrySha256:digest(read(pwEntry)),packageSha256:digest(read(path.join(pwRoot,'package.json')))}, browserVersion, inputs:{source:{size:Buffer.byteLength(source),sha256:digest(Buffer.from(source))},artifactSha256:digest(read(path.join(probe,'artifact.json'))),defaultsSha256:digest(read(path.join(probe,'defaults.json'))),harnessSha256:digest(read(new URL(import.meta.url)))}, totals:{checks:checks.length,passed:checks.filter(x=>x.ok).length,failed:failures.length,requests:requests.length,browserErrors:browserErrors.length,serverErrors:serverErrors.length}, checks, failures, screenshots:shots, nonClaims:['200% zoom','screen reader','touch hardware','adverse network','production task time','consumer integration'] }
const receiptPath = path.join(out, 'receipt.json'); fs.writeFileSync(receiptPath, JSON.stringify(receipt,null,2)+'\n', {mode:0o600})
console.log(JSON.stringify({output:out,receipt:receiptPath,receiptSha256:digest(read(receiptPath)),pass:receipt.totals.passed,fail:receipt.totals.failed,error:browserErrors.length+serverErrors.length}))
if (failures.length || browserErrors.length || serverErrors.length) process.exitCode = 1
