// Approved local Chrome only. Source/reference use the pinned Elements runtime;
// native Details runs with document JavaScript disabled.
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import crypto from 'node:crypto';
import {pathToFileURL} from 'node:url';
import {parsePreparedManifest, requestsAreLocalGET, runtimeESMSHA256, sourceSHA256, validatePreparedBindings} from './contract.mjs';

if (process.argv.length !== 4) throw Error('usage: node browser.mjs /private/faq-preparation EXPECTED_MANIFEST_SHA256');
const root = fs.realpathSync(process.argv[2] || '');
const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const manifestBytes = fs.readFileSync(path.join(root, 'manifest.json'));
const manifest = parsePreparedManifest(manifestBytes, process.argv[3]);
const files = new Map();
for (const [name, identity] of Object.entries(manifest.files || {})) {
  if (!/^[A-Za-z0-9._-]+$/.test(name)) throw Error('non-flat prepared filename');
  const file = path.join(root, name), stat = fs.lstatSync(file);
  if (!stat.isFile() || stat.isSymbolicLink() || stat.size > 8 * 1024 * 1024) throw Error('invalid prepared file');
  const body = fs.readFileSync(file);
  if (body.length !== identity.size || sha(body) !== identity.sha256) throw Error(`prepared file changed: ${name}`);
  files.set(name, body);
}
validatePreparedBindings(manifest, files);
if (sha(files.get('source.html')) !== sourceSHA256 || sha(files.get('elements.js')) !== runtimeESMSHA256) throw Error('source/runtime pin mismatch');

const entry = process.env.APPKIT_PLAYWRIGHT_MODULE || '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
const version = JSON.parse(fs.readFileSync(path.join(path.dirname(entry), 'package.json'))).version;
if (version !== '1.63.0') throw Error('unexpected Playwright pin');
const {chromium} = await import(pathToFileURL(entry).href);
const chrome = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const out = fs.mkdtempSync(path.join(root, 'browser-')); fs.chmodSync(out, 0o700);
const save = (name, body) => fs.writeFileSync(path.join(out, name), body, {flag: 'wx', mode: 0o600});
const report = {schema: 1, kind: 'p03-faq-typed-browser', verification: 'not-verified',
  manifest_sha256: sha(manifestBytes), harness_sha256: sha(fs.readFileSync(new URL(import.meta.url))),
  contract_sha256: sha(fs.readFileSync(new URL('./contract.mjs', import.meta.url))), versions: {playwright: version},
  checks: [], requests: [], errors: [], screenshots: [], source_findings: [], native_observations: [], limits: [
    'Scoped Chrome evidence; no screen reader, touch, cross-browser, consumer, or certification claim.',
    'Source small targets remain findings. Native styling is an adaptation, never source fidelity.',
    'Browser state resets to supplied model on navigation; no morph or persisted-state claim.']};
const check = (name, ok, detail = null) => report.checks.push({name, ok: !!ok, detail});
const screenshots = new Map();
const renderedStates = new Map();
let base, browser, withholdRuntime = false;
const prefix = `/faq-${crypto.randomBytes(18).toString('hex')}/`;
const csp = "default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; connect-src 'none'; form-action 'none'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'";
const server = http.createServer((req, res) => {
  res.setHeader('Content-Security-Policy', csp); res.setHeader('Cache-Control', 'no-store');
  res.setHeader('X-Content-Type-Options', 'nosniff'); res.setHeader('Referrer-Policy', 'same-origin');
  if (!base || req.headers.host !== new URL(base).host || req.method !== 'GET' || req.headers['sec-fetch-site'] === 'cross-site' || (req.headers.origin !== undefined && req.headers.origin !== base)) {res.writeHead(403); res.end(); return;}
  const url = new URL(req.url, base), name = url.pathname.startsWith(prefix) ? url.pathname.slice(prefix.length) : '';
  if (url.search || !files.has(name)) {res.writeHead(404); res.end(); return;}
  const type = name.endsWith('.html') ? 'text/html; charset=utf-8' : name.endsWith('.js') ? 'text/javascript; charset=utf-8' :
    name.endsWith('.css') ? 'text/css; charset=utf-8' : name.endsWith('.woff2') ? 'font/woff2' : 'application/octet-stream';
  res.writeHead(200, {'Content-Type': type});
  res.end(withholdRuntime && name === 'elements.js' ? '// runtime deliberately withheld\n' : files.get(name));
});

async function settle(page) {
  // Page timers/RAF do not execute with document scripting disabled. Poll from
  // Node, preserving real source transitions instead of disabling animation.
  const deadline = Date.now() + 5000;
  let stable = 0;
  while (Date.now() < deadline) {
    const ready = await page.evaluate(() => document.fonts.status === 'loaded' && document.getAnimations().every(a => a.playState !== 'running' && !a.pending));
    stable = ready ? stable + 1 : 0;
    if (stable >= 3) return;
    await new Promise(resolve => setTimeout(resolve, 50));
  }
  throw Error('font/animation settlement deadline');
}
const visible = locator => locator.evaluate(e => !e.hidden && getComputedStyle(e).display !== 'none' &&
  [e, ...e.querySelectorAll('*')].some(n => n.checkVisibility() && n.getBoundingClientRect().height > 0));
async function dom(page) {
  return page.locator('#specimen').evaluate(root => {
    const visit = n => ({type:n.nodeType, ns:n.namespaceURI, name:n.nodeName, data:n.nodeValue,
      attrs:n.attributes ? [...n.attributes].map(a => [a.namespaceURI,a.name,a.value]).sort((a,b)=>JSON.stringify(a).localeCompare(JSON.stringify(b))) : [],
      children:[...n.childNodes].map(visit)});
    return JSON.stringify([...root.childNodes].map(visit));
  });
}
async function capture(page, name, pairKey, lane) {
  // Independently stabilize each lane; never compare against the other lane to
  // choose a frame, mask pixels, disable source transitions, or apply tolerance.
  let body = await page.screenshot(), stable = false, attempts = 1;
  for (; attempts < 8; attempts++) {
    await settle(page);
    const next = await page.screenshot();
    if (body.equals(next)) { stable = true; break; }
    body = next;
  }
  check(`${lane}:${pairKey}: screenshot stabilized independently`, stable, {captures:attempts+1});
  const state = {dom:await dom(page),svg:await page.locator('#specimen svg').evaluateAll(items=>items.map(e=>{
    const rect=n=>{const r=n.getBoundingClientRect();return [r.x,r.y,r.width,r.height];};
    const style=n=>{const s=getComputedStyle(n);return {color:s.color,stroke:s.stroke,opacity:s.opacity,display:s.display,transform:s.transform};};
    return {box:rect(e),style:style(e),paths:[...e.querySelectorAll('path')].map(p=>({box:rect(p),style:style(p)}))};
  }))};
  const previousState=renderedStates.get(pairKey);
  if(previousState) check(`${pairKey}: exact runtime DOM and SVG geometry/styles`,JSON.stringify(previousState)===JSON.stringify(state));
  else renderedStates.set(pairKey,state);
  const identity = sha(body); save(name, body); report.screenshots.push({name, sha256: identity, pair: pairKey, lane,dom_sha256:sha(Buffer.from(state.dom)),svg:state.svg});
  const prior = screenshots.get(pairKey);
  if (!prior) screenshots.set(pairKey, {identity, lane});
  else check(`${pairKey}: source/reference PNG bytes match`, prior.identity === identity, {source: prior.lane === 'source' ? prior.identity : identity, reference: lane === 'reference' ? identity : prior.identity});
}
async function contextFor(width, theme, javascript = true) {
  const context = await browser.newContext({viewport:{width,height:1000}, colorScheme:theme, reducedMotion:'reduce', javaScriptEnabled:javascript, serviceWorkers:'block'});
  await context.route('**/*', route => {
    const request = route.request(), url = new URL(request.url());
    const allowed = url.origin === base && url.pathname.startsWith(prefix) && files.has(url.pathname.slice(prefix.length)) && request.method() === 'GET' && !url.search;
    report.requests.push({url:request.url(), method:request.method(), allowed}); return allowed ? route.continue() : route.abort('blockedbyclient');
  });
  return context;
}
async function load(page, name, runtime = true) {
  await page.goto(base + prefix + name);
  if (runtime) await page.waitForFunction(() => customElements.get('el-disclosure'));
  await settle(page);
}
async function faqState(page, selector = '#specimen') {
  return page.locator(selector).evaluate(root => [...root.querySelectorAll('el-disclosure')].map(e => e.hidden));
}
async function iconState(page, rootSelector, triggerSelector) {
  return page.locator(`${rootSelector} ${triggerSelector} svg`).evaluateAll(icons => icons.map(icon => icon.checkVisibility()));
}

async function sourceReferenceTask(page, key, pairKey, lane) {
  const states = await faqState(page);
  check(`${key}: six independent source disclosures`, JSON.stringify(states) === JSON.stringify([false,true,true,true,true,true]), states);
  const trigger = page.locator('#specimen button').nth(1), panel = page.locator('#specimen el-disclosure').nth(1);
  const beforeIcons = await trigger.locator('svg').evaluateAll(items => items.map(item => item.checkVisibility()));
  await trigger.focus(); await page.keyboard.press('Enter'); await settle(page);
  check(`${key}: Enter opens answer and preserves focus`, await visible(panel) && await trigger.evaluate(e => e === document.activeElement));
  const after = await faqState(page); check(`${key}: sibling states remain independent`, JSON.stringify(after) === JSON.stringify([false,false,true,true,true,true]), after);
  const icons = await trigger.locator('svg').evaluateAll(items => items.map(item => item.checkVisibility()));
  check(`${key}: exactly one open-state icon is visible`, icons.filter(Boolean).length === 1 && JSON.stringify(icons) !== JSON.stringify(beforeIcons), {before:beforeIcons, after:icons});
  await capture(page, `${key}-open.png`, `${pairKey}-open`, lane);
  await page.keyboard.press('Space'); await settle(page); check(`${key}: Space closes answer`, !(await visible(panel)));
  await trigger.click(); await settle(page); check(`${key}: pointer reopens answer`, await visible(panel));
  const boxes = await page.locator('#specimen button').evaluateAll(items => items.map(e => {const r=e.getBoundingClientRect();return {width:r.width,height:r.height};}));
  for (const box of boxes) if (box.width < 44 || box.height < 44) report.source_findings.push({key, finding:'source target below 44px', box});
}

async function nativeTask(page, key) {
  const details = page.locator('#specimen details'), summaries = page.locator('#specimen summary');
  check(`${key}: two six-item instances`, await details.count() === 12 && await page.locator('[data-fixture-instance]').count() === 2);
  check(`${key}: native lane contains no Elements commands or scripts`, await page.locator('#specimen el-disclosure, #specimen [command], #specimen [commandfor], script').count() === 0);
  const ids = await page.locator('#specimen [id]').evaluateAll(items => items.map(e => e.id));
  check(`${key}: derived IDs are unique across instances`, ids.length >= 12 && new Set(ids).size === ids.length, {count:ids.length});
  const initial = await details.evaluateAll(items => items.map(e => e.open));
  const defaults = JSON.parse(files.get('defaults.json'));
  const expected = ['primary','secondary'].map((instance,index) => ({heading:index ? 'H'.repeat(256) : defaults.Heading,
    items:defaults.Items.map((item,i) => ({id:`help-${instance}-${item.Key}`,question:index?String.fromCharCode(65+i).repeat(480):item.Question,
      answer:index?String.fromCharCode(97+i).repeat(4096):item.Answer,open:item.Open}))}));
  const actual = await page.locator('[data-fixture-instance]').evaluateAll(instances => instances.map(root => ({heading:root.querySelector('h2')?.textContent,
    items:[...root.querySelectorAll('details')].map(item=>({id:item.querySelector(':scope > div')?.id,
      question:item.querySelector('summary > span')?.textContent,answer:item.querySelector('p')?.textContent,open:item.open}))})));
  check(`${key}: exact ordered model content, identity and initial state`,JSON.stringify(actual)===JSON.stringify(expected));
  check(`${key}: each instance preserves default open states`, initial.filter(Boolean).length === 2 && initial[0] && initial[6], initial);
  const initialOpenIcons = await summaries.nth(0).locator('[data-appkit-faq-icon]').evaluateAll(items => items.map(e => e.checkVisibility()));
  const initialClosedIcons = await summaries.nth(1).locator('[data-appkit-faq-icon]').evaluateAll(items => items.map(e => e.checkVisibility()));
  check(`${key}: native initial icons follow open state`, JSON.stringify(initialOpenIcons) === JSON.stringify([false,true]) && JSON.stringify(initialClosedIcons) === JSON.stringify([true,false]), {open:initialOpenIcons,closed:initialClosedIcons});
  const secondaryBefore = await page.locator('[data-fixture-instance="secondary"] details').evaluateAll(items => items.map(e => e.open));
  await page.locator('[data-fixture-instance="primary"] summary').nth(1).click(); await settle(page);
  const secondaryAfter = await page.locator('[data-fixture-instance="secondary"] details').evaluateAll(items => items.map(e => e.open));
  check(`${key}: instance toggle is isolated`, JSON.stringify(secondaryBefore) === JSON.stringify(secondaryAfter));
  for (let i = 0; i < await details.count(); i++) {
    const item = details.nth(i), summary = summaries.nth(i);
    if (!(await item.evaluate(e => e.open))) { if (i % 2) {await summary.focus(); await page.keyboard.press('Enter');} else await summary.click(); await settle(page); }
    check(`${key}: answer ${i+1} reachable`, await item.evaluate(e => e.open) && await visible(item.locator(':scope > div')));
    const icons = await summary.locator('[data-appkit-faq-icon]').evaluateAll(items => items.map(e => e.checkVisibility()));
    check(`${key}: answer ${i+1} open icon`, icons.length === 2 && icons.filter(Boolean).length === 1 && !icons[0] && icons[1], icons);
  }
  const geometry = await summaries.evaluateAll(items => items.map(e => {const r=e.getBoundingClientRect(),s=getComputedStyle(e);return {width:r.width,height:r.height,scroll:e.scrollWidth,client:e.clientWidth,outline:s.outlineWidth,outlineStyle:s.outlineStyle};}));
  check(`${key}: native 44px targets and wrapping`, geometry.every(g => g.width >= 44 && g.height >= 44 && g.scroll <= g.client + 1) && await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), geometry);
  const focusSummary = summaries.nth(2); await focusSummary.focus(); await page.keyboard.press('Space'); await settle(page);
  const focus = await focusSummary.evaluate(e => {const s=getComputedStyle(e);return {active:e===document.activeElement,visible:e.matches(':focus-visible'),width:parseFloat(s.outlineWidth),style:s.outlineStyle};});
  check(`${key}: keyboard focus remains visibly outlined`, focus.active && focus.visible && focus.width >= 2 && focus.style === 'solid', focus);
  report.native_observations.push({key, details:12, ids:ids.length, geometry});
  const shot = await page.screenshot(); save(`${key}.png`, shot); report.screenshots.push({name:`${key}.png`,sha256:sha(shot),lane:'native'});
  await page.locator('[data-fixture-instance="secondary"] h2').scrollIntoViewIfNeeded(); await settle(page);
  const longShot = await page.screenshot(); save(`${key}-long-heading.png`, longShot);
  report.screenshots.push({name:`${key}-long-heading.png`,sha256:sha(longShot),lane:'native-long-content'});
}

try {
  await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});
  base = `http://127.0.0.1:${server.address().port}`;
  for (const origin of ['https://example.invalid', 'null']) {
    const response = await fetch(base + prefix + manifest.documents.native.light, {headers:{Origin:origin}});
    check(`server refuses Origin ${origin}`,response.status===403); await response.arrayBuffer();
  }
  browser = await chromium.launch({executablePath:chrome,headless:true}); report.versions.chrome = await browser.version();
  for (const width of [390,1440]) for (const theme of ['light','dark']) {
    let sourceDOM;
    for (const lane of ['source','reference']) {
      const context = await contextFor(width,theme,true), page = await context.newPage(), key=`${lane}-${width}-${theme}`, pairKey=`${width}-${theme}`;
      try {
        await load(page,manifest.documents[lane][theme],true);
        const currentDOM=await dom(page); if(lane==='source')sourceDOM=currentDOM; else check(`${pairKey}: source/reference exact runtime DOM`,sourceDOM===currentDOM);
        check(`${key}: pinned font loaded`,await page.evaluate(()=>[...document.fonts].some(f=>f.family.replaceAll('"','')==='InterVariable'&&f.status==='loaded')));
        await capture(page,`${key}-initial.png`,`${pairKey}-initial`,lane); await sourceReferenceTask(page,key,pairKey,lane);
      } catch(error){check(`${key}: task completes`,false,String(error));} finally {await context.close();}
    }
    const nativeRequestStart=report.requests.length,context=await contextFor(width,theme,false),page=await context.newPage(),key=`native-${width}-${theme}`;
    try {await load(page,manifest.documents.native[theme],false);await nativeTask(page,key);check(`${key}: native requested no module`,!report.requests.slice(nativeRequestStart).some(r=>new URL(r.url).pathname.endsWith('.js')));} catch(error){check(`${key}: task completes`,false,String(error));} finally {await context.close();}
  }
  withholdRuntime=true;
  { const context=await contextFor(390,'light',true),page=await context.newPage();try{await load(page,manifest.documents.source.light,false);const before=await faqState(page);await page.locator('#specimen button').nth(1).click();await settle(page);check('runtime-withheld source cannot toggle',JSON.stringify(before)===JSON.stringify(await faqState(page)));}finally{await context.close();withholdRuntime=false;} }

  // Real Chrome setting, not CSS zoom or viewport-only emulation.
  const profile=fs.mkdtempSync(path.join(out,'chrome-profile-'));fs.chmodSync(profile,0o700);
  const zoomContext=await chromium.launchPersistentContext(profile,{executablePath:chrome,headless:true,viewport:{width:1440,height:1000},colorScheme:'dark',serviceWorkers:'block'});
  try {
    const page=zoomContext.pages()[0];await page.goto('chrome://settings/appearance');await page.locator('#zoomLevel').selectOption('2');check('Chrome setting is 200 percent',await page.locator('#zoomLevel').inputValue()==='2');
    const control=await zoomContext.newCDPSession(page);await control.send('Emulation.setScriptExecutionDisabled',{value:true});
    await zoomContext.route('**/*',route=>{const r=route.request(),u=new URL(r.url());const allowed=u.origin===base&&u.pathname.startsWith(prefix)&&files.has(u.pathname.slice(prefix.length))&&r.method()==='GET';report.requests.push({url:r.url(),method:r.method(),allowed});return allowed?route.continue():route.abort();});
    await load(page,manifest.documents.native.dark,false);const zoom=await page.evaluate(()=>({dpr:devicePixelRatio,inner:innerWidth,outer:outerWidth,client:document.documentElement.clientWidth,scroll:document.documentElement.scrollWidth}));
    check('actual 200 percent native FAQ zoom',zoom.dpr===2&&zoom.inner===720&&zoom.outer===1440&&zoom.client===720&&zoom.scroll<=zoom.client,zoom);await nativeTask(page,'native-200-percent');
  } finally {await zoomContext.close();fs.rmSync(profile,{recursive:true});report.zoom_profile_disposition='fresh isolated profile removed after browser close';}
  check('all browser traffic is admitted local GET',requestsAreLocalGET(report.requests,base));
  check('every source screenshot has one byte-identical reference',report.checks.filter(c=>c.name.endsWith('PNG bytes match')).length===8&&report.checks.filter(c=>c.name.endsWith('PNG bytes match')).every(c=>c.ok));
} catch(error){report.errors.push(String(error));}
finally {
  await browser?.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));
  report.status=report.errors.length===0&&report.checks.every(c=>c.ok)&&report.checks.length>150?'passed-with-source-findings':'failed';
  save('report.json',Buffer.from(JSON.stringify(report,null,2)+'\n'));
  console.log(JSON.stringify({status:report.status,output:out,checks:report.checks.length,failed:report.checks.filter(c=>!c.ok).map(c=>c.name),source_findings:report.source_findings.length,errors:report.errors}));
  if(report.status==='failed')process.exitCode=1;
}
