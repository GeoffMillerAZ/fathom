import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import http from 'node:http'
import crypto from 'node:crypto'
import { pathToFileURL } from 'node:url'

const MAX_DOCUMENT = 1024 * 1024
const PLAYWRIGHT = '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs'
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
const CASE = {
  case_id: 'P04-table',
  archive_id: 'application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8',
  path: 'html/lists/tables/07-with-stacked-columns-on-mobile.html',
  sha256: '5d00e2755dcf060cceafa4c2dba4f15452719e54f5cd40646ab3155c89dc8288',
}
const CSS_INPUT_SHA256 = '7d92b4e13ebe3b82279aa67500ae4d2a383733ef394f12b4b24eda515af7802e'
const FONT_SHA256 = '693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3'
const LICENSE_SHA256 = '262481e844521b326f5ecd053e59b98c8b2da78c8ee1bdbb6e8174305e54935a'
const TAILWIND_VERSION = 'v4.3.0'
const TAILWIND_SHA256 = '56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e'
const STYLE_LICENSE_PREFIX = '/* Inter v4.1 SIL Open Font License\n'
const STYLE_LICENSE_SUFFIX = '\n*/\n@font-face{font-family:InterVariable;font-style:normal;font-weight:100 900;font-display:block;src:url(data:font/woff2;base64,'
const STYLE_FONT_SUFFIX = ") format('woff2')}\n"
const USERS = [
  ['Lindsay Walton', 'Front-end Developer', 'lindsay.walton@example.com', 'Member'],
  ['Courtney Henry', 'Designer', 'courtney.henry@example.com', 'Admin'],
  ['Tom Cook', 'Director of Product', 'tom.cook@example.com', 'Member'],
  ['Whitney Francis', 'Copywriter', 'whitney.francis@example.com', 'Admin'],
  ['Leonard Krasner', 'Senior Designer', 'leonard.krasner@example.com', 'Owner'],
  ['Floyd Miles', 'Principal Designer', 'floyd.miles@example.com', 'Member'],
]
const exactKeys = (value, expected) => value && typeof value === 'object' && !Array.isArray(value) &&
  JSON.stringify(Object.keys(value).sort()) === JSON.stringify([...expected].sort())
const digest = data => crypto.createHash('sha256').update(data).digest('hex')
const snapshot = file => { const data = fs.readFileSync(file); return { size:data.length, sha256:digest(data), data } }
const boundedSnapshot = file => {
  const before = fs.lstatSync(file)
  if (!before.isFile() || before.isSymbolicLink() || before.size > MAX_DOCUMENT) fail('unsafe-input')
  const fd = fs.openSync(file, fs.constants.O_RDONLY | (fs.constants.O_NOFOLLOW || 0))
  try {
    const opened = fs.fstatSync(fd)
    if (opened.dev !== before.dev || opened.ino !== before.ino || opened.size !== before.size) fail('input-changed')
    const data = Buffer.alloc(before.size); let offset = 0
    while (offset < data.length) { const count=fs.readSync(fd,data,offset,data.length-offset,null); if (!count) fail('input-short'); offset+=count }
    const extra=Buffer.alloc(1), count=fs.readSync(fd,extra,0,1,null), after=fs.fstatSync(fd)
    if (count || after.dev!==before.dev || after.ino!==before.ino || after.size!==before.size) fail('input-changed')
    return {size:data.length,sha256:digest(data),data}
  } finally { fs.closeSync(fd) }
}
const fail = category => { throw new Error(category) }

const args = process.argv.slice(2)
if (args.length !== 1) fail('usage')
const supplied = args[0]
const probe = fs.realpathSync(supplied)
const temp = fs.realpathSync(os.tmpdir()) + path.sep
if (!probe.startsWith(temp) || !fs.statSync(probe).isDirectory() || fs.lstatSync(supplied).isSymbolicLink()) fail('unsafe-probe-root')
const names = ['reference.json', 'source-light.html', 'source-dark.html']
const inputs = new Map()
for (const name of names) {
  inputs.set(name, boundedSnapshot(path.join(probe, name)))
}
const ref = JSON.parse(inputs.get('reference.json').data)
if (!exactKeys(ref, ['schema','case_id','source','documents','css_sha256','css_input_sha256','font_sha256','license_sha256','tailwind_version','tailwind_sha256','scope','verification']) ||
    ref.schema !== 1 || ref.case_id !== CASE.case_id || ref.scope !== 'controlled-source-reference' || ref.verification !== 'not-verified' ||
    !exactKeys(ref.source, ['archive_id','path','sha256','size']) || ref.source.archive_id !== CASE.archive_id ||
    ref.source.path !== CASE.path || ref.source.sha256 !== CASE.sha256 || !Number.isSafeInteger(ref.source.size) || ref.source.size < 1 ||
    !exactKeys(ref.documents, ['source-light.html','source-dark.html']) || !/^[a-f0-9]{64}$/.test(ref.css_sha256) ||
    ref.css_input_sha256 !== CSS_INPUT_SHA256 || ref.font_sha256 !== FONT_SHA256 || ref.license_sha256 !== LICENSE_SHA256 ||
    ref.tailwind_version !== TAILWIND_VERSION || ref.tailwind_sha256 !== TAILWIND_SHA256) fail('invalid-reference')
for (const name of names.slice(1)) {
  const expected = ref.documents[name], actual = inputs.get(name)
  if (!exactKeys(expected, ['sha256','size']) || expected.size !== actual.size || expected.sha256 !== actual.sha256 || actual.size > MAX_DOCUMENT) fail('document-identity')
}
function inspectDocument(name, theme) {
  const text = inputs.get(name).data.toString('utf8')
  const head = `<!doctype html>\n<html lang="en" class="${theme}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>P04 table: controlled source reference</title><style>`
  const styleEnd = '</style></head><body>', tail = '</body></html>\n'
  if (!text.startsWith(head) || !text.endsWith(tail)) fail('document-envelope')
  const boundary = text.indexOf(styleEnd, head.length)
  if (boundary < 0 || boundary !== text.lastIndexOf(styleEnd)) fail('document-envelope')
  const style = text.slice(head.length, boundary), body = text.slice(boundary + styleEnd.length, -tail.length)
  if (Buffer.byteLength(body) !== ref.source.size || digest(Buffer.from(body)) !== CASE.sha256 || /<(?:script|form|img)\b/i.test(body)) fail('source-body-identity')
  if (!style.startsWith(STYLE_LICENSE_PREFIX)) fail('style-envelope')
  const licenseEnd = style.indexOf(STYLE_LICENSE_SUFFIX, STYLE_LICENSE_PREFIX.length)
  if (licenseEnd < 0) fail('style-envelope')
  const license = Buffer.from(style.slice(STYLE_LICENSE_PREFIX.length, licenseEnd))
  if (digest(license) !== LICENSE_SHA256) fail('license-identity')
  const fontStart = licenseEnd + STYLE_LICENSE_SUFFIX.length, fontEnd = style.indexOf(STYLE_FONT_SUFFIX, fontStart)
  if (fontEnd < 0) fail('style-envelope')
  const encoded = style.slice(fontStart, fontEnd), font = Buffer.from(encoded, 'base64')
  if (!encoded || font.toString('base64') !== encoded || digest(font) !== FONT_SHA256) fail('font-identity')
  const css = Buffer.from(style.slice(fontEnd + STYLE_FONT_SUFFIX.length))
  if (!css.length || digest(css) !== ref.css_sha256) fail('css-identity')
  return { text, styleHash:crypto.createHash('sha256').update(style).digest('base64') }
}
const documents = new Map()
let styleHash
for (const theme of ['light','dark']) {
  const name = `source-${theme}.html`, inspected = inspectDocument(name, theme)
  if (styleHash && styleHash !== inspected.styleHash) fail('style-mismatch')
  styleHash = inspected.styleHash
  documents.set(`/source-${theme}`, inspected.text)
}
const out = path.join(probe, 'browser-source')
if (fs.existsSync(out)) fail('output-exists')
fs.mkdirSync(out, {mode:0o700}); fs.chmodSync(out, 0o700)

const csp = `default-src 'none'; style-src 'sha256-${styleHash}'; font-src data:; script-src 'none'; connect-src 'none'; img-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'none'`
const requests = [], serverErrors = [], boundaryFailures = [], browserFailures = [], qualityFailures = [], checks = [], screenshots = []
const check = (name, ok, detail, classification='source-contract') => {
  const row = {name, ok:!!ok, classification, ...(detail === undefined ? {} : {detail})}
  checks.push(row); if (!ok) (classification === 'inherited-source-quality' ? qualityFailures : boundaryFailures).push(name)
}
const server = http.createServer((req,res) => {
  const address = server.address(), expectedHost = address && `${address.address}:${address.port}`
  const exact = req.method === 'GET' && req.headers.host === expectedHost && req.socket.remoteAddress === '127.0.0.1' &&
    req.socket.localAddress === '127.0.0.1' && documents.has(req.url)
  if (!exact) { boundaryFailures.push('server-refused-request'); res.writeHead(403).end('refused'); return }
  requests.push({method:req.method,path:req.url})
  res.writeHead(200, {'Content-Type':'text/html; charset=utf-8','Content-Security-Policy':csp,'Cache-Control':'no-store','X-Content-Type-Options':'nosniff','Referrer-Policy':'no-referrer'}).end(documents.get(req.url))
})
server.on('clientError', (error,socket) => { serverErrors.push(error?.code || 'client-error'); socket.destroy() })

const pwEntry = fs.realpathSync(PLAYWRIGHT), chrome = fs.realpathSync(CHROME)
const pwRoot = path.dirname(pwEntry), pwPackage = snapshot(path.join(pwRoot,'package.json'))
const pw = JSON.parse(pwPackage.data)
if (pw.version !== '1.63.0') fail('playwright-version')
const {chromium} = await import(pathToFileURL(pwEntry).href)
let browser, browserVersion = '', timedOut = false
const watchdog = setTimeout(async () => { timedOut = true; try { await browser?.close() } catch {}; server.closeAllConnections?.(); server.close() }, 120000)
watchdog.unref()
const closeBounded = promise => Promise.race([promise, new Promise((_,reject)=>setTimeout(()=>reject(new Error('close-timeout')),5000))])

const inspect = page => page.evaluate(({users}) => {
  const visibleExact = (root, value) => [...(root?.querySelectorAll('*') || [])].filter(el =>
    [...el.childNodes].some(node => node.nodeType === Node.TEXT_NODE && node.textContent.trim() === value) && el.getClientRects().length).length
  const accessibleExact = (root, value) => [...(root?.querySelectorAll('.sr-only') || [])].filter(el => el.textContent.trim() === value).length
  const table = document.querySelector('table'), body = table?.querySelector('tbody'), rows = [...document.querySelectorAll('tbody tr')]
  const actions = [...document.querySelectorAll('button,a')].map(el => { const r=el.getBoundingClientRect(); return {tag:el.tagName,text:el.textContent.trim().replace(/\s+/g,' '),href:el.getAttribute('href'),width:r.width,height:r.height} })
  return {
    ready:document.readyState, theme:document.documentElement.className, fontStatus:document.fonts.status,
    interFaces:[...document.fonts].filter(face=>face.family.replaceAll('"','')==='InterVariable').map(face=>face.status),
    bodyFamily:getComputedStyle(document.body).fontFamily.split(',')[0].trim().replaceAll('"',''),
    scripts:document.scripts.length, forms:document.forms.length, images:document.images.length,
    headings:[...document.querySelectorAll('h1,h2,h3')].map(x=>`${x.tagName}:${x.textContent.trim()}`),
    description:visibleExact(document.body,'A list of all the users in your account including their name, title, email and role.'),
    columns:[...document.querySelectorAll('thead th')].map(x=>x.textContent.trim()), rowCount:rows.length,
    users:users.map((fields,index)=>fields.map(value=>visibleExact(rows[index],value))),
    accessibleLabels:{title:accessibleExact(body,'Title'),email:accessibleExact(body,'Email')},
    overflow:Math.max(0,document.documentElement.scrollWidth-innerWidth), actions,
  }
}, {users:USERS})
const contrast = page => page.evaluate(() => {
  const canvas=document.createElement('canvas'); canvas.width=canvas.height=1; const ctx=canvas.getContext('2d',{willReadFrequently:true})
  const rgba=value=>{ctx.clearRect(0,0,1,1);ctx.fillStyle=value;ctx.fillRect(0,0,1,1);return [...ctx.getImageData(0,0,1,1).data].map((v,i)=>i===3?v/255:v)}
  const over=(a,b)=>{const x=a[3]+b[3]*(1-a[3]);return x?[(a[0]*a[3]+b[0]*b[3]*(1-a[3]))/x,(a[1]*a[3]+b[1]*b[3]*(1-a[3]))/x,(a[2]*a[3]+b[2]*b[3]*(1-a[3]))/x,x]:[255,255,255,1]}
  const lum=c=>{const f=v=>(v/=255)<=.04045?v/12.92:((v+.055)/1.055)**2.4;return .2126*f(c[0])+.7152*f(c[1])+.0722*f(c[2])}
  let sampled=0,min=Infinity,failed=0,unknown=0
  const all=[...document.querySelectorAll('*')].slice(0,512)
  for(const el of all){if(![...el.childNodes].some(n=>n.nodeType===3&&n.textContent.trim())||!el.getClientRects().length)continue;let bg=[255,255,255,1],bad=false,chain=[];for(let n=el;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const s=getComputedStyle(n);if(parseFloat(s.opacity)!==1||s.backgroundImage!=='none'){bad=true;break}bg=over(rgba(s.backgroundColor),bg)}if(bad){unknown++;continue}const cs=getComputedStyle(el),fg=over(rgba(cs.color),bg),ratio=(Math.max(lum(fg),lum(bg))+.05)/(Math.min(lum(fg),lum(bg))+.05),large=parseFloat(cs.fontSize)>=24||(parseFloat(cs.fontSize)>=18.66&&Number(cs.fontWeight)>=700),limit=large?3:4.5;sampled++;min=Math.min(min,ratio);if(ratio+1e-6<limit)failed++}
  return {sampled,min:Number.isFinite(min)?min:0,failed,unknown,truncated:document.querySelectorAll('*').length>512}
})

try {
  await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve)})
  const origin = `http://127.0.0.1:${server.address().port}`
  browser = await chromium.launch({headless:true,executablePath:chrome})
  browserVersion = browser.version()
  for (const theme of ['light','dark']) for (const width of [390,1440]) {
    const tag = `${theme}-${width}`, context = await browser.newContext({viewport:{width,height:900},colorScheme:theme,serviceWorkers:'block'})
    context.setDefaultTimeout(10000); context.setDefaultNavigationTimeout(10000)
    context.on('serviceworker',()=>browserFailures.push(`${tag}:serviceworker`))
    context.on('page',page=>{page.on('pageerror',()=>browserFailures.push(`${tag}:pageerror`));page.on('console',message=>{if(['error','warning'].includes(message.type()))browserFailures.push(`${tag}:console-${message.type()}`)});page.on('dialog',async dialog=>{browserFailures.push(`${tag}:dialog`);await dialog.dismiss().catch(()=>{})})})
    await context.route('**/*',route=>{const request=route.request(),url=new URL(request.url());if(url.origin===origin&&request.method()==='GET'&&!url.search&&url.pathname===`/source-${theme}`)return route.continue();browserFailures.push(`${tag}:blocked`);return route.abort('blockedbyclient')})
    await context.routeWebSocket('**/*',ws=>{browserFailures.push(`${tag}:websocket`);ws.close()})
    const page = await context.newPage(), response = await page.goto(`${origin}/source-${theme}`,{waitUntil:'load'})
    await page.evaluate(()=>document.fonts.ready)
    const state = await inspect(page), ratios = await contrast(page)
    check(`${tag} response`,response?.status()===200&&response.headers()['content-security-policy']===csp)
    check(`${tag} document theme`,state.theme===theme,state.theme)
    check(`${tag} inert boundary`,state.scripts===0&&state.forms===0&&state.images===0,{scripts:state.scripts,forms:state.forms,images:state.images})
    check(`${tag} font ready`,state.ready==='complete'&&state.fontStatus==='loaded'&&state.bodyFamily==='InterVariable'&&state.interFaces.length>0&&state.interFaces.every(status=>status==='loaded'),{status:state.fontStatus,family:state.bodyFamily,faces:state.interFaces})
    check(`${tag} hierarchy`,JSON.stringify(state.headings)===JSON.stringify(['H1:Users'])&&state.description===1,state.headings)
    check(`${tag} table shape`,state.rowCount===6&&JSON.stringify(state.columns)===JSON.stringify(['Name','Title','Email','Role','Edit']),{rows:state.rowCount,columns:state.columns})
    check(`${tag} exact visible data`,state.users.every(fields=>fields.every(count=>count===1)),state.users)
    check(`${tag} sr-only accessible labels`,state.accessibleLabels.title===6&&state.accessibleLabels.email===6,state.accessibleLabels)
    check(`${tag} no outer overflow`,state.overflow===0,state.overflow,'inherited-source-quality')
    const add=state.actions.filter(x=>x.tag==='BUTTON'&&x.text==='Add user'), edits=state.actions.filter(x=>x.tag==='A'&&x.text.startsWith('Edit'))
    check(`${tag} static action identity`,add.length===1&&edits.length===6&&edits.every(x=>x.href==='#'),{add:add.length,edits:edits.length})
    check(`${tag} action targets 44px`,[...add,...edits].every(x=>x.width>=44&&x.height>=44),[...add,...edits].map(x=>({tag:x.tag,width:x.width,height:x.height})),'inherited-source-quality')
    check(`${tag} sampled contrast`,ratios.sampled>0&&ratios.failed===0&&ratios.unknown===0&&!ratios.truncated,ratios,'inherited-source-quality')
    const imagePath=path.join(out,`source-${tag}.png`), image = await page.screenshot({path:imagePath,fullPage:true}), imageName=`source-${tag}.png`
    fs.chmodSync(imagePath,0o600)
    screenshots.push({name:imageName,size:image.length,sha256:digest(image)})
    await page.keyboard.press('Tab')
    const focus = await page.evaluate(()=>{const e=document.activeElement,s=getComputedStyle(e),r=e.getBoundingClientRect();return {tag:e.tagName,text:e.textContent.trim().replace(/\s+/g,' '),outline:s.outlineStyle,width:parseFloat(s.outlineWidth),height:r.height}})
    check(`${tag} first Tab Add user`,focus.tag==='BUTTON'&&focus.text==='Add user',focus)
    check(`${tag} visible keyboard focus`,focus.outline!=='none'&&focus.width>=2,focus,'inherited-source-quality')
    const focusImagePath=path.join(out,`source-${tag}-focus.png`), focusImage = await page.screenshot({path:focusImagePath,fullPage:true})
    fs.chmodSync(focusImagePath,0o600)
    screenshots.push({name:`source-${tag}-focus.png`,size:focusImage.length,sha256:digest(focusImage)})
    await context.close()
  }
} catch (error) {
  browserFailures.push(`operational:${error?.name || 'Error'}`)
} finally {
  clearTimeout(watchdog)
  try { if (browser) await closeBounded(browser.close()) } catch { browserFailures.push('browser-close') }
  server.closeAllConnections?.()
  try { await closeBounded(new Promise(resolve=>server.close(resolve))) } catch { serverErrors.push('server-close') }
}
if (timedOut) browserFailures.push('global-timeout')
for (const name of names) { try { const after=boundedSnapshot(path.join(probe,name)),before=inputs.get(name); if(after.size!==before.size||after.sha256!==before.sha256) boundaryFailures.push('prepared-input-changed') } catch { boundaryFailures.push('prepared-input-changed') } }
if (requests.length !== 4 || requests.some((request,index)=>request.method!=='GET'||request.path!==`/source-${index<2?'light':'dark'}`)) boundaryFailures.push('request-multiset')
if (serverErrors.length) boundaryFailures.push('server-error')
if (browserFailures.length) boundaryFailures.push('browser-error')
screenshots.sort((a,b)=>a.name.localeCompare(b.name))
const harness = snapshot(new URL(import.meta.url))
const receipt = {
  schema:1, generated_at:new Date().toISOString(), scope:'controlled-source-reference', verification:'not-verified', case_id:CASE.case_id,
  source:ref.source, reference_sha256:inputs.get('reference.json').sha256,
  documents:Object.fromEntries(names.slice(1).map(name=>[name,{size:inputs.get(name).size,sha256:inputs.get(name).sha256}])),
  css_sha256:ref.css_sha256,css_input_sha256:ref.css_input_sha256,font_sha256:ref.font_sha256,license_sha256:ref.license_sha256,
  tailwind_version:ref.tailwind_version,tailwind_sha256:ref.tailwind_sha256,
  harness:{sha256:harness.sha256,playwright_version:pw.version,playwright_entry_sha256:snapshot(pwEntry).sha256,playwright_package_sha256:pwPackage.sha256,browser_version:browserVersion||'unavailable'},
  totals:{checks:checks.length,passed:checks.filter(x=>x.ok).length,quality_failures:qualityFailures.length,boundary_failures:boundaryFailures.length,requests:requests.length,browser_failures:browserFailures.length,server_errors:serverErrors.length},
  checks, quality_failures:qualityFailures, boundary_failures:boundaryFailures, screenshots,
  not_working_tasks:['Add user is the original inert button; no create action is claimed.','Edit links retain the original # destinations; they are not navigated or claimed as working.'],
  non_claims:['adapted component','public catalog target','target certification','consumer adoption','form or persistence behavior','pixel equality','screen-reader or touch certification'],
}
const receiptPath=path.join(out,'receipt.json');fs.writeFileSync(receiptPath,JSON.stringify(receipt,null,2)+'\n',{mode:0o600});fs.chmodSync(receiptPath,0o600)
console.log(JSON.stringify({output:out,receipt:receiptPath,receipt_sha256:snapshot(receiptPath).sha256,checks:receipt.totals.checks,quality_failures:qualityFailures.length,boundary_failures:boundaryFailures.length}))
if (qualityFailures.length||boundaryFailures.length) process.exitCode=1
