// Bounded private P08 browser probe. No downloads, owner profile, backend, or
// source-image substitution. Browser input is protocol automation, not physical input.
import assert from 'node:assert/strict';
import {createHash, randomBytes} from 'node:crypto';
import {spawnSync} from 'node:child_process';
import fs from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';

const [kitArg, probeArg, receiptSHA] = process.argv.slice(2);
assert(process.argv.length === 5 && path.isAbsolute(kitArg) && path.isAbsolute(probeArg) && /^[a-f0-9]{64}$/.test(receiptSHA ?? ''),
  'usage: node browser.mjs KIT PRIVATE_PROBE COMPILE_RECEIPT_SHA256');
const kit = await fs.realpath(kitArg), probe = await fs.realpath(probeArg);
assert(probe.startsWith(await fs.realpath(os.tmpdir()) + path.sep), 'probe must be below command-scoped temporary storage');
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const inputs = new Map(), outputs = new Map();
async function input(file, expected) {
  const normalized = file instanceof URL ? fileURLToPath(file) : file;
  const stat = await fs.lstat(normalized);
  assert(stat.isFile() && stat.nlink === 1 && stat.size > 0 && stat.size <= 8 * 2 ** 20, 'invalid bounded input');
  const bytes = await fs.readFile(normalized), digest = sha(bytes);
  if (expected) assert.equal(digest, expected, 'input identity differs');
  inputs.set(normalized, digest); return bytes;
}
const out = await fs.mkdtemp(path.join(probe, 'browser-'));
await fs.chmod(out, 0o700);
async function write(name, bytes) {
  const file = path.join(out, name); await fs.writeFile(file, bytes, {mode: 0o600, flag: 'wx'});
  outputs.set(name, {sha256: sha(bytes), bytes: bytes.length}); return file;
}
const report = {schema: 1, kind: 'private-p08-drawer-browser-screening', verification: 'not-certified', compile_receipt_sha256: receiptSHA,
  checks: [], captures: [], requests: [], blocked_external: [], csp_blocks: [], errors: [], cleanup: 'pending',
  limits: ['Original external profile photo is deliberately unavailable; visual and photo fidelity are unverified.', 'Reference placeholder destinations are never activated.', 'Synthetic loopback destinations are not messaging, calling, profile, or history backends.', 'Keyboard and pointer actions use browser-protocol automation, not OS or physical input.', 'No zoom, touch, screen-reader, adverse-network, performance, consumer-adoption, or target-certification claim.']};
const check = (name, ok, detail = {}) => report.checks.push({name, ok: Boolean(ok), ...detail});
let server, browserServer, browser, base;
const prefix = '/drawer-' + randomBytes(18).toString('hex');
const watchdog = setTimeout(() => {report.errors.push('four-minute watchdog'); process.exitCode = 1; browserServer?.process()?.kill('SIGKILL'); server?.closeAllConnections();}, 240000);
async function bounded(promise) {
  let timer;
  try { return await Promise.race([promise, new Promise((_, reject) => {timer = setTimeout(() => reject(Error('bounded operation timed out')), 10000);})]); }
  finally { clearTimeout(timer); }
}
try {
  report.harness_sha256 = sha(await input(new URL(import.meta.url)));
  const compile = JSON.parse(await input(path.join(probe, 'compile-receipt.json'), receiptSHA));
  assert.equal(compile.verification, 'not-verified');
  const artifactBytes = await input(path.join(probe, 'delivery/artifact.json'));
  assert.equal(compile.artifact_sha256, sha(artifactBytes));
  const artifact = JSON.parse(artifactBytes);
  assert.equal(artifact.adapter, 'p08-profile-drawer-v1'); assert.equal(artifact.verification, 'not-verified');
  const source = await input(path.join(probe, 'original.html'), '1fbde54a0be874618cc295bf3b837fee4d89f38a1543dd9a6e2374bbd1d13415');
  const sourceImages = [...source.toString().matchAll(/<img\b[^>]*\bsrc="([^"]+)"/g)].map(match => match[1]);
  assert.equal(sourceImages.length, 1); assert.equal(new URL(sourceImages[0]).protocol, 'https:');
  const sourceImageSHA = sha(Buffer.from(sourceImages[0]));
  assert.equal(artifact.source.sha256, sha(source));
  assert.equal(compile.rule_sha256, artifact.rule_sha256); assert.deepEqual(compile.source, artifact.source);
  assert.deepEqual(compile.syntax, artifact.syntax); assert.deepEqual(compile.runtime, artifact.runtime);
  assert.equal(compile.templ_sha256, artifact.templ_sha256); assert.equal(compile.model_sha256, artifact.model_sha256);
  for (const [name, key] of [['drawer.templ', 'templ_sha256'], ['drawer.go', 'model_sha256'], ['defaults.json', 'defaults_sha256']])
    await input(path.join(probe, 'delivery', name), artifact[key]);
  const fragments = new Map([['original', source]]);
  for (const name of ['reference', 'typed', 'readonly', 'long', 'multiple']) {
    const pin = compile.rendered_sha256?.[name + '.html']; assert.match(pin, /^[a-f0-9]{64}$/);
    fragments.set(name, await input(path.join(probe, name + '.html'), pin));
  }
  const assetsRoot = path.join(kit, 'data/design-assets/reference-assets/elements-1.0.22-pilot');
  const runtime = await input(path.join(assetsRoot, 'elements.js'), '120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013');
  assert.equal(artifact.runtime.esm_sha256, sha(runtime));
  const font = await input(path.join(assetsRoot, 'inter.woff2'), '693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3');
  const tw = '/Users/geoff/Library/Caches/app-kit/tailwindcss/v4.3.0/tailwindcss-macos-arm64';
  const twStat = await fs.lstat(tw); assert(twStat.isFile() && twStat.nlink===1 && twStat.size>0 && twStat.size<=96*2**20, 'invalid pinned compiler');
  const twBytes = await fs.readFile(tw), twSHA = sha(twBytes);
  assert.equal(twSHA, '56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e'); inputs.set(tw, twSHA);
  const cssInput = Buffer.from('@import "tailwindcss" source(none);\n@source "../original.html";\n@source "../delivery/drawer.templ";\n@theme{--font-sans:InterVariable,ui-sans-serif,sans-serif}\n@font-face{font-family:InterVariable;font-weight:100 900;font-display:block;src:url("'+prefix+'/inter.woff2") format("woff2")}\n');
  const cssInputPath = await write('input.css', cssInput), cssPath = path.join(out, 'source.css');
  const built = spawnSync(tw, ['-i', cssInputPath, '-o', cssPath, '--minify'], {timeout: 30000, maxBuffer: 65536});
  assert.equal(built.status, 0, 'pinned CSS compilation failed'); await fs.chmod(cssPath, 0o600);
  const css = await fs.readFile(cssPath); outputs.set('source.css', {sha256: sha(css), bytes: css.length});
  report.tailwind = {version: '4.3.0', executable_sha256: sha(twBytes)};
  const photo = Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96"><rect width="96" height="96" fill="#64748b"/><circle cx="48" cy="37" r="17" fill="#e2e8f0"/><path fill="#e2e8f0" d="M17 94c2-24 14-35 31-35s29 11 31 35"/></svg>');
  const favicon = Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><path fill="#64748b" d="M0 0h1v1H0z"/></svg>');
  const fixtureCSS = Buffer.from('html{font-family:InterVariable,system-ui,sans-serif}body{margin:0;padding:24px;background:#f9fafb;color:#111827}html.dark body{background:#111827;color:#f9fafb}h1{font-size:1rem;margin:0 0 24px}');
  const assets = new Map([['elements.js', runtime], ['inter.woff2', font], ['source.css', css], ['fixture.css', fixtureCSS], ['photo.svg', photo], ['favicon.svg', favicon]]);
  report.assets = Object.fromEntries([...assets].map(([name, body]) => [name, {sha256:sha(body), bytes:body.length}]));
  const documents = new Map();
  for (const scheme of ['light', 'dark']) for (const [name, fragment] of fragments) {
    const body = Buffer.from('<!doctype html><html lang="en" class="'+(scheme === 'dark' ? 'dark' : '')+'"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Drawer specimen</title><link rel="icon" href="'+prefix+'/favicon.svg"><link rel="stylesheet" href="'+prefix+'/source.css"><link rel="stylesheet" href="'+prefix+'/fixture.css"><script type="module" src="'+prefix+'/elements.js"></script></head><body><h1>Synthetic profile drawer screening</h1><main id="specimen">'+fragment+'</main><button id="outside" type="button">Outside drawer</button></body></html>');
    const key = name + '-' + scheme; documents.set(key, body); await write(key + '.html', body);
  }
  const destinations = new Map([['/compose', 'Synthetic compose handoff'], ['/call-setup', 'Synthetic call setup'], ['/profile', 'Synthetic profile destination'], ['/history', 'Synthetic history destination']]);
  const destinationBodies = new Map([...destinations].map(([route,label]) => [route, Buffer.from('<!doctype html><title>Fixture result</title><h1>'+label+'</h1><p>No external or account action occurred.</p>')]));
  report.destination_outputs = Object.fromEntries([...destinationBodies].map(([route,body]) => [route,{sha256:sha(body),bytes:body.length}]));
  server = http.createServer((req, res) => {
    const origin = base && new URL(base).origin, url = new URL(req.url, origin || 'http://invalid.test');
    const denied = !origin || req.method !== 'GET' || req.headers.host !== new URL(origin).host || url.origin !== origin || url.search;
    res.setHeader('Cache-Control', 'no-store'); res.setHeader('X-Content-Type-Options', 'nosniff'); res.setHeader('Referrer-Policy', 'same-origin');
    res.setHeader('Content-Security-Policy', "default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self'; connect-src 'none'; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'");
    if (denied) {res.writeHead(403); res.end(); return;}
    if (destinationBodies.has(url.pathname)) {const body = destinationBodies.get(url.pathname); res.writeHead(200, {'Content-Type':'text/html; charset=utf-8'}); res.end(body); return;}
    const name = url.pathname === '/photo.svg' ? 'photo.svg' : url.pathname.startsWith(prefix + '/') ? url.pathname.slice(prefix.length + 1) : '';
    const body = documents.get(name) || assets.get(name);
    if (!body) {res.writeHead(404); res.end(); return;}
    report.requests.push({method: req.method, path: url.pathname});
    const type = documents.has(name) ? 'text/html; charset=utf-8' : name.endsWith('.js') ? 'text/javascript' : name.endsWith('.woff2') ? 'font/woff2' : name.endsWith('.svg') ? 'image/svg+xml' : 'text/css';
    res.writeHead(200, {'Content-Type':type}); res.end(body);
  });
  await bounded(new Promise((resolve, reject) => {server.once('error', reject); server.listen(0, '127.0.0.1', resolve);}));
  base = 'http://127.0.0.1:' + server.address().port;
  const pw = '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
  const pwBytes = await input(pw, 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
  const {chromium} = await import(pathToFileURL(pw).href);
  browserServer = await chromium.launchServer({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless:true, timeout:20000});
  report.browser_pid = browserServer.process().pid;
  browser = await chromium.connect(browserServer.wsEndpoint()); report.browser = browser.version(); report.playwright_entry_sha256 = sha(pwBytes);
  async function ready(page) {
    await page.waitForFunction(() => ['el-dialog','el-dropdown','el-menu'].every(tag => customElements.get(tag)) && document.fonts.status === 'loaded');
    let stable = 0;
    for (let attempt=0; attempt<100; attempt++) {
      const settled = await page.evaluate(() => document.getAnimations().every(animation => animation.playState!=='running'&&!animation.pending) && [...document.querySelectorAll('el-menu:popover-open')].every(menu => getComputedStyle(menu).opacity==='1'));
      stable = settled ? stable+1 : 0;
      if (stable>=3) return;
      await page.waitForTimeout(50);
    }
    throw Error('Elements animation did not settle');
  }
  async function tabTo(page, selector, max = 24, container = null) {
    for (let i = 0; i < max; i++) {
      await page.keyboard.press('Tab');
      if (container) check('modal Tab remains contained ' + i, await page.locator(container).evaluate(e => e.contains(document.activeElement)));
      if (await page.locator(selector).evaluate(e => e === document.activeElement)) return true;
    }
    return false;
  }
  async function capture(page, name) {
    const bytes = await page.screenshot({animations:'disabled', fullPage:false}); await write(name + '.png', bytes);
    report.captures.push({file:name+'.png', sha256:sha(bytes), bytes:bytes.length}); return bytes;
  }
  async function navigate(page, name, scheme) {await page.goto(base+prefix+'/'+name+'-'+scheme, {waitUntil:'networkidle'}); await ready(page);}
  async function openByKeyboard(page, key, dialog = 'dialog') {
    check(key+': Tab reaches opener', await tabTo(page, 'button[command="show-modal"]'));
    await page.keyboard.press('Enter'); await ready(page);
    check(key+': Enter opens modal with contained focus', await page.locator(dialog).evaluate(e => e.open && e.contains(document.activeElement)));
  }
  async function nestedMenuAndEscape(page, key, dialog = 'dialog') {
    const dropdown = page.locator(dialog+' el-dropdown'), trigger = dropdown.locator('button').first(), menu = dropdown.locator('el-menu');
    check(key+': modal Tab reaches nested menu', await tabTo(page, dialog+' el-dropdown > button', 24, dialog));
    await page.keyboard.press('Enter'); await ready(page);
    check(key+': nested menu opens with focus', await menu.evaluate(e => e.matches(':popover-open') && e.contains(document.activeElement)));
    const before = await menu.locator('a').evaluateAll((all) => all.indexOf(document.activeElement));
    await page.keyboard.press('ArrowDown');
    const after = await menu.locator('a').evaluateAll((all) => all.indexOf(document.activeElement));
    check(key+': ArrowDown advances nested menu', before >= 0 && after >= 0 && before !== after, {before, after});
    await page.keyboard.press('Escape'); await ready(page);
    check(key+': menu Escape preserves dialog and restores trigger', await page.locator(dialog).evaluate(e => e.open) && await trigger.evaluate(e => e === document.activeElement) && !(await menu.evaluate(e => e.matches(':popover-open'))));
    await page.keyboard.press('Escape'); await ready(page);
    check(key+': dialog Escape closes and restores opener', !(await page.locator(dialog).evaluate(e => e.open)) && await page.locator('button[command="show-modal"]').first().evaluate(e => e === document.activeElement));
  }
  for (const width of [390, 1440]) for (const scheme of ['light','dark']) {
    const context = await browser.newContext({viewport:{width,height:1000}, colorScheme:scheme, reducedMotion:'reduce', serviceWorkers:'block'});
    await context.route('**/*', async route => {const url = new URL(route.request().url()); if (url.origin !== base) {report.blocked_external.push({url_sha256:sha(Buffer.from(url.href)), resource:route.request().resourceType()}); await route.abort('blockedbyclient');} else await route.continue();});
    await context.addInitScript(() => {window.__drawerCSP=[]; document.addEventListener('securitypolicyviolation', event => window.__drawerCSP.push({directive:event.effectiveDirective, blocked:event.blockedURI}));});
    const page = await context.newPage(); page.setDefaultTimeout(5000);
    page.on('pageerror', error => report.errors.push('pageerror: '+error.message));
    page.on('response', response => {if (new URL(response.url()).origin===base && response.status()>=400) report.errors.push('local HTTP '+response.status());});
    const referenceShots = {};
    for (const name of ['original','reference']) {
      const key = `${scheme}-${width}-${name}`; await navigate(page,name,scheme);
      referenceShots[name+'-closed'] = await capture(page,key+'-closed');
      for (const violation of await page.evaluate(() => window.__drawerCSP)) report.csp_blocks.push({directive:violation.directive, url_sha256:sha(Buffer.from(violation.blocked))});
      await openByKeyboard(page,key); referenceShots[name+'-open'] = await capture(page,key+'-open');
      check(key+': source placeholder links remain unactivated', await page.locator('dialog a').evaluateAll(all => all.length===2 && all.every(a=>a.getAttribute('href')==='#')));
      await nestedMenuAndEscape(page,key);
    }
    check(`${scheme}-${width}: original/reference closed pixels match`, referenceShots['original-closed'].equals(referenceShots['reference-closed']));
    check(`${scheme}-${width}: original/reference open pixels match`, referenceShots['original-open'].equals(referenceShots['reference-open']));

    const typedKey = `${scheme}-${width}-typed`; await navigate(page,'typed',scheme); await openByKeyboard(page,typedKey);
    await capture(page,typedKey+'-open'); await nestedMenuAndEscape(page,typedKey);
    await page.keyboard.press('Enter'); await ready(page);
    check(typedKey+': reopens from restored trigger', await page.locator('dialog').evaluate(e=>e.open&&e.contains(document.activeElement)));
    check(typedKey+': modal Tab containment', await tabTo(page, 'dialog button[command="close"]', 24, 'dialog'));
    await page.keyboard.press('Escape'); await ready(page);
    await page.locator('button[command="show-modal"]').click(); await ready(page);
    await page.locator('dialog button[command="close"]').click(); await ready(page);
    check(typedKey+': pointer close restores opener', !(await page.locator('dialog').evaluate(e=>e.open)) && await page.locator('button[command="show-modal"]').evaluate(e=>e===document.activeElement));

    for (const target of ['/compose','/call-setup','/profile','/history']) {
      await navigate(page,'typed',scheme); await page.locator('button[command="show-modal"]').click(); await ready(page);
      if (target === '/profile' || target === '/history') {await page.locator('dialog el-dropdown > button').click(); await ready(page);}
      await Promise.all([page.waitForURL(base+target,{waitUntil:'networkidle'}), page.locator(`dialog a[href="${target}"]`).click()]);
      check(`${scheme}-${width}: native ${target} destination`, await page.locator('h1').textContent() === destinations.get(target));
    }

    await navigate(page,'readonly',scheme); await openByKeyboard(page,`${scheme}-${width}-readonly`);
    check(`${scheme}-${width}: read-only has no app actions`, await page.locator('dialog a, dialog el-dropdown').count()===0);
    await page.keyboard.press('Escape'); await ready(page);

    await navigate(page,'multiple',scheme);
    const ids = await page.locator('dialog').evaluateAll(all => all.map(e=>e.id));
    check(`${scheme}-${width}: multiple IDs unique`, JSON.stringify(ids)===JSON.stringify(['person-dialog','second-dialog']));
    const openers = page.locator('button[command="show-modal"]');
    await openers.nth(1).click(); await ready(page);
    check(`${scheme}-${width}: second opener targets only second`, await page.locator('#second-dialog').evaluate(e=>e.open) && !(await page.locator('#person-dialog').evaluate(e=>e.open)));
    await page.keyboard.press('Escape'); await openers.nth(0).click(); await ready(page);
    check(`${scheme}-${width}: first opener targets only first`, await page.locator('#person-dialog').evaluate(e=>e.open) && !(await page.locator('#second-dialog').evaluate(e=>e.open)));
    await page.keyboard.press('Escape');

    await navigate(page,'long',scheme); await openByKeyboard(page,`${scheme}-${width}-long`);
    const geometry = await page.locator('dialog').evaluate(e => {
      const panel=e.querySelector('el-dialog-panel'), r=panel.getBoundingClientRect(), final=e.querySelector('dl>div:last-child dd');
      const scroll=(()=>{for(let n=final;n&&n!==e.parentElement;n=n.parentElement){const s=getComputedStyle(n);if(/auto|scroll/.test(s.overflowY)&&n.scrollHeight>n.clientHeight+1)return n;}return null;})();
      const sr=scroll?.getBoundingClientRect();
      const measure=(selector)=>{const n=e.querySelector(selector), b=n?.getBoundingClientRect();if(!b||!sr)return null;const range=document.createRange();range.selectNodeContents(n);const textRects=[...range.getClientRects()].filter(rect=>rect.width>0&&rect.height>0).map(rect=>({left:rect.left,right:rect.right,top:rect.top,bottom:rect.bottom}));return {left:b.left,right:b.right,top:b.top,bottom:b.bottom,clipLeft:sr.left,clipRight:sr.right,clipTop:sr.top,clipBottom:sr.bottom,elementHorizontal:b.left>=sr.left-1&&b.right<=sr.right+1,textRects,textHorizontal:textRects.length>0&&textRects.every(rect=>rect.left>=sr.left-1&&rect.right<=sr.right+1)};};
      return {page:document.documentElement.scrollWidth,viewport:innerWidth,left:r.left,right:r.right,top:r.top,bottom:r.bottom,height:r.height,scrollHeight:panel.scrollHeight,clientHeight:panel.clientHeight,
        scroll:scroll&&sr?{left:sr.left,right:sr.right,top:sr.top,bottom:sr.bottom,scrollTop:scroll.scrollTop,scrollHeight:scroll.scrollHeight,clientHeight:scroll.clientHeight}:null,
        children:{heading:measure('h2'),name:measure('h3'),action:measure('a[href="/compose"]'),detail:measure('dl>div:last-child dd')}};
    });
    check(`${scheme}-${width}: long content has bounded horizontal layout`, geometry.page<=geometry.viewport && geometry.left>=0 && geometry.right<=geometry.viewport, geometry);
    check(`${scheme}-${width}: long content remains vertically reachable`, geometry.top>=0 && (geometry.bottom<=1000 || geometry.scrollHeight>geometry.clientHeight), geometry);
    check(`${scheme}-${width}: selected long elements and text fit the scroll clip`, geometry.scroll!==null&&Object.values(geometry.children).every(child=>child?.elementHorizontal&&child.textHorizontal), geometry);
    if (geometry.scroll) {
      await page.mouse.move((geometry.scroll.left+geometry.scroll.right)/2,Math.max(geometry.scroll.top+1,Math.min(geometry.scroll.bottom-1,(geometry.scroll.top+geometry.scroll.bottom)/2)));
      for(let i=0;i<4;i++) await page.mouse.wheel(0,2000);
      await page.waitForTimeout(100);
    }
    const reached=await page.locator('dialog').evaluate(e=>{const final=e.querySelector('dl>div:last-child dd');let scroll=null;for(let n=final;n&&n!==e.parentElement;n=n.parentElement){const s=getComputedStyle(n);if(/auto|scroll/.test(s.overflowY)&&n.scrollHeight>n.clientHeight+1){scroll=n;break;}}if(!scroll)return null;const r=final.getBoundingClientRect(),sr=scroll.getBoundingClientRect(),range=document.createRange();range.selectNodeContents(final);const textRects=[...range.getClientRects()].filter(rect=>rect.width>0&&rect.height>0).map(rect=>({left:rect.left,right:rect.right,top:rect.top,bottom:rect.bottom}));return {scrollTop:scroll.scrollTop,scrollHeight:scroll.scrollHeight,clientHeight:scroll.clientHeight,top:r.top,bottom:r.bottom,clipLeft:sr.left,clipRight:sr.right,clipTop:sr.top,clipBottom:sr.bottom,textRects,textFullyVisible:textRects.length>0&&textRects.every(rect=>rect.left>=sr.left-1&&rect.right<=sr.right+1&&rect.top>=sr.top-1&&rect.bottom<=sr.bottom+1),atEnd:scroll.scrollTop+scroll.clientHeight>=scroll.scrollHeight-1};});
    check(`${scheme}-${width}: native wheel fully reveals final-detail text`, reached?.scrollTop>0&&reached.textFullyVisible&&reached.atEnd, reached??{});
    await capture(page,`${scheme}-${width}-long-open`);
    await page.keyboard.press('Escape'); await context.close();
  }
  const sourceBlocks = [...report.blocked_external.map(r=>({directive:'img-src',url_sha256:r.url_sha256})), ...report.csp_blocks];
  check('external refusals are only the declared source image', sourceBlocks.length>0 && report.blocked_external.every(r=>r.resource==='image') && sourceBlocks.every(r=>r.directive==='img-src'&&r.url_sha256===sourceImageSHA), {route_blocks:report.blocked_external.length,csp_blocks:report.csp_blocks.length});
  for (const [file,digest] of inputs) assert.equal(sha(await fs.readFile(file)),digest,'input changed during proof');
} catch (error) {report.errors.push(String(error)); process.exitCode=1;}
finally {
  try {
    if (browser) await bounded(browser.close());
    if (browserServer) await bounded(browserServer.close());
    if (server?.listening) {server.closeAllConnections(); await bounded(new Promise((resolve,reject)=>server.close(error=>error?reject(error):resolve())));}
    if (report.browser_pid) {let gone=false; for(let i=0;i<30;i++){try{process.kill(report.browser_pid,0);}catch(error){if(error.code==='ESRCH'){gone=true;break;}throw error;}await new Promise(resolve=>setTimeout(resolve,100));} assert(gone,'owned browser PID remains'); report.browser_pid_absent=true;}
    if (base) await bounded(new Promise((resolve,reject)=>{const socket=net.createConnection({host:'127.0.0.1',port:Number(new URL(base).port)});socket.once('connect',()=>{socket.destroy();reject(Error('owned listener remains'));});socket.once('error',error=>{socket.destroy();error.code==='ECONNREFUSED'?resolve():reject(error);});}));
    report.cleanup='ok';
  } catch (error) {report.cleanup=String(error); report.errors.push('cleanup failed'); browserServer?.process()?.kill('SIGKILL'); server?.closeAllConnections(); process.exitCode=1;}
  clearTimeout(watchdog);
  report.inputs=Object.fromEntries(inputs); report.outputs=Object.fromEntries(outputs);
  report.passed=report.checks.filter(c=>c.ok).length; report.failed=report.checks.filter(c=>!c.ok).length;
  if(report.failed||report.errors.length) process.exitCode=1;
  const bytes=Buffer.from(JSON.stringify(report,null,2)+'\n');
  await fs.writeFile(path.join(out,'receipt.json'),bytes,{mode:0o600,flag:'wx'});
  process.stdout.write(JSON.stringify({out,passed:report.passed,failed:report.failed,errors:report.errors,receipt_sha256:sha(bytes)})+'\n');
}
