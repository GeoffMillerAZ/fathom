// Explicit local/private P07 task probe. No downloads, installs or owner profile.
import assert from 'node:assert/strict';
import {createHash, randomBytes} from 'node:crypto';
import {spawnSync} from 'node:child_process';
import fs from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import {pathToFileURL} from 'node:url';

const [kitArg, probeArg, receiptSHA, repairFlag] = process.argv.slice(2);
const repair = process.argv.length === 6 && repairFlag === '--repair';
assert((process.argv.length === 5 || repair) && path.isAbsolute(kitArg) && path.isAbsolute(probeArg) && /^[a-f0-9]{64}$/.test(receiptSHA ?? ''), 'usage: node browser.mjs KIT PRIVATE_PROBE COMPILE_RECEIPT_SHA256 [--repair]');
const kit = await fs.realpath(kitArg), probe = await fs.realpath(probeArg);
assert(probe.startsWith(await fs.realpath(os.tmpdir()) + path.sep));
const sha = b => createHash('sha256').update(b).digest('hex');
const write = (p, b) => fs.writeFile(p, b, {mode: 0o600, flag: 'wx'});
const inputs = new Map();
async function input(file, expected) {
  const stat = await fs.lstat(file);
  assert(stat.isFile() && stat.nlink === 1 && stat.size > 0 && stat.size <= 8 * 2 ** 20, 'invalid bounded input');
  const bytes = await fs.readFile(file), digest = sha(bytes);
  if (expected) assert.equal(digest, expected, 'input identity differs');
  inputs.set(file, digest); return bytes;
}
const out = await fs.mkdtemp(path.join(probe, 'browser-'));
await fs.chmod(out, 0o700);
const report = {kind: 'p07-menu-browser', variant: repair ? 'experimental-repair' : 'unchanged-v1', verification: 'not-verified', compile_receipt_sha256: receiptSHA,
  checks: [], captures: [], requests: [], actions: [], errors: [], inputs: {}, cleanup: 'pending',
  limits: ['Synthetic local action receiver, not an authentication backend or real consumer.', 'Keyboard and pointer use browser-protocol automation, not OS/physical input. Native refers to browser link/form behavior, not physical input.', 'No actual zoom, touch, screen-reader, Datastar lifecycle or full performance certification.']};
const check = (name, ok, detail = {}) => { report.checks.push({name, ok: Boolean(ok), ...detail}); };
let server, browserServer, browser, base, rejectNext = false;
const prefix = '/menu-' + randomBytes(18).toString('hex');
const watchdog = setTimeout(() => { report.errors.push('four-minute watchdog'); browserServer?.process()?.kill('SIGKILL'); server?.closeAllConnections(); process.exitCode = 1; }, 240000);
async function bounded(promise) {
  let timer;
  try { return await Promise.race([promise, new Promise((_, reject) => {timer = setTimeout(() => reject(Error('cleanup deadline')), 10000);})]); }
  finally { clearTimeout(timer); }
}
try {
  report.harness_sha256 = sha(await input(new URL(import.meta.url)));
  const compile = JSON.parse(await input(path.join(probe, 'compile-receipt.json'), receiptSHA));
  const artifact = JSON.parse(await input(path.join(probe, 'delivery/artifact.json'), compile.artifact_sha256));
  assert.equal(artifact.adapter, 'p07-menu-v1'); assert.equal(artifact.verification, 'not-verified');
  const source = await input(path.join(probe, 'original.html'), '10ed82d0c337ad246f1ab8687d73cc0a7dc5d834bb7b3ec46a01bc7c5d5f144a');
  assert.equal(artifact.source.sha256, sha(source));
  for (const [name, key] of [['menu.templ', 'templ_sha256'], ['menu.go', 'model_sha256'], ['menu.css', 'css_sha256'], ['defaults.json', 'defaults_sha256']]) {
    await input(path.join(probe, 'delivery', name), artifact[key]);
  }
  const fragments = new Map([['original', source]]);
  for (const name of ['reference', 'typed', 'enhanced', 'enhanced-long', 'disabled', 'links-only']) {
    const pin = compile.rendered_sha256[name + '.html']; assert.match(pin, /^[a-f0-9]{64}$/);
    fragments.set(name, await input(path.join(probe, name + '.html'), pin));
  }
  const assetsRoot = path.join(kit, 'data/design-assets/reference-assets/elements-1.0.22-pilot');
  const runtime = await input(path.join(assetsRoot, 'elements.js'), '120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013');
  assert.equal(artifact.runtime.esm_sha256, sha(runtime));
  const font = await input(path.join(assetsRoot, 'inter.woff2'), '693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3');
  const tw = '/Users/geoff/Library/Caches/app-kit/tailwindcss/v4.3.0/tailwindcss-macos-arm64';
  assert.equal(sha(await fs.readFile(tw)), '56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e');
  await write(path.join(out, 'input.css'), '@import "tailwindcss" source(none);\n@source "../original.html";\n@source "../delivery/menu.templ";\n@theme{--font-sans:InterVariable,ui-sans-serif,sans-serif}\n@font-face{font-family:InterVariable;font-weight:100 900;font-display:block;src:url("'+prefix+'/inter.woff2") format("woff2")}\n');
  const cssPath = path.join(out, 'source.css');
  const built = spawnSync(tw, ['-i', path.join(out, 'input.css'), '-o', cssPath, '--minify'], {timeout: 30000, maxBuffer: 65536});
  assert.equal(built.status, 0, 'pinned CSS compilation failed'); await fs.chmod(cssPath, 0o600);
  const css = await fs.readFile(cssPath), companion = await fs.readFile(path.join(probe, 'delivery/menu.css'));
  const fixtureCSS = Buffer.from('html{font-family:InterVariable,system-ui,sans-serif}body{margin:0;padding:24px;background:#f9fafb;color:#111827}html.dark body{background:#111827;color:#f9fafb}h1{font-size:1rem;margin:0 0 24px}#outside{display:block;margin-top:32px;padding:12px}#fallback{display:block;margin-top:16px}');
  const assets = new Map([['elements.js', runtime], ['inter.woff2', font], ['source.css', css], ['menu.css', companion], ['fixture.css', fixtureCSS]]);
  const repairEligible = name => name.startsWith('enhanced') || ['disabled','links-only'].includes(name);
  const repairNames = new Set(['menu_repair.mjs', 'menu_repair.css', 'menu_repair_bootstrap.mjs']);
  if (repair) {
    const repairModule = await input(path.join(kit, 'internal/designadapt/menu_repair.mjs'));
    const repairCSS = await input(path.join(kit, 'internal/designadapt/menu_repair.css'));
    const repairBootstrap = Buffer.from('import {mountMenuRepair} from "./menu_repair.mjs";\nawait customElements.whenDefined("el-menu");\nfor (const root of document.querySelectorAll("[data-appkit-menu]")) mountMenuRepair(root);\n');
    assets.set('menu_repair.mjs', repairModule);
    assets.set('menu_repair.css', repairCSS);
    assets.set('menu_repair_bootstrap.mjs', repairBootstrap);
    report.repair = {
      module_sha256: sha(repairModule),
      css_sha256: sha(repairCSS),
      bootstrap_sha256: sha(repairBootstrap),
      delivery: 'external experimental companion; not part of the immutable v1 export',
    };
  }
  const docs = new Map();
  for (const scheme of ['light', 'dark']) for (const [name, fragment] of fragments) {
    const repaired = repair && repairEligible(name);
    const body = Buffer.from('<!doctype html><html lang="en" class="'+(scheme === 'dark' ? 'dark' : '')+'"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Menu task specimen</title><link rel="icon" href="data:,"><link rel="stylesheet" href="'+prefix+'/source.css"><link rel="stylesheet" href="'+prefix+'/fixture.css">'+(repairEligible(name) ? '<link rel="stylesheet" href="'+prefix+'/menu.css">' : '')+(repaired ? '<link rel="stylesheet" href="'+prefix+'/menu_repair.css">' : '')+'<script type="module" src="'+prefix+'/elements.js"></script>'+(repaired ? '<script type="module" src="'+prefix+'/menu_repair_bootstrap.mjs"></script>' : '')+'</head><body><h1>Synthetic account actions</h1><main id="specimen">'+fragment+'</main><button id="outside" type="button">Outside menu</button><a id="fallback" href="/profile">Open profile without the menu</a></body></html>');
    docs.set(name+'-'+scheme, body); await write(path.join(out, name+'-'+scheme+'.html'), body);
  }
  const actionBodies = [];
  server = http.createServer(async (req, res) => {
    const origin = base && new URL(base).origin;
    const denied = !origin || req.headers.host !== new URL(origin).host || req.headers['sec-fetch-site'] === 'cross-site' || (req.headers.origin !== undefined && req.headers.origin !== origin);
    res.setHeader('Cache-Control', 'no-store'); res.setHeader('X-Content-Type-Options', 'nosniff'); res.setHeader('Referrer-Policy', 'same-origin');
    const url = new URL(req.url, origin || 'http://invalid.test');
    const reference = url.pathname.includes('/original-') || url.pathname.includes('/reference-');
    res.setHeader('Content-Security-Policy', "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src data:; connect-src 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action " + (reference ? "'none'" : "'self'"));
    if (denied || url.search) {res.writeHead(403); res.end(); return;}
    if (req.method === 'POST' && url.pathname === '/action') {
      if (req.headers.origin !== origin || req.headers['content-type']?.split(';')[0] !== 'application/x-www-form-urlencoded') {res.writeHead(403); res.end(); return;}
      let bytes = 0, chunks = [];
      for await (const chunk of req) {bytes += chunk.length; if (bytes > 8192) {res.writeHead(413); res.end(); return;} chunks.push(chunk);}
      const body = Buffer.concat(chunks), fields = new URLSearchParams(body.toString());
      const valid = fields.size === 1 && fields.getAll('csrf').length === 1 && fields.get('csrf') === 'synthetic-menu-token';
      const rejected = rejectNext || !valid; rejectNext = false;
      const status = rejected ? 403 : 303;
      actionBodies.push({sha256: sha(body), bytes: body.length, valid, status});
      report.actions.push(actionBodies.at(-1));
      const referringPath = new URL(req.headers.referer || '/', origin).pathname;
      const returnKey = referringPath.startsWith(prefix+'/') ? referringPath.slice(prefix.length+1) : '';
      const returnPath = docs.has(returnKey) ? referringPath : prefix+'/enhanced-light';
      if (rejected) {res.writeHead(403, {'Content-Type':'text/html; charset=utf-8'}); res.end('<!doctype html><title>Action rejected</title><h1>Action was not applied</h1><p>Return to the menu to retry.</p><a href="'+returnPath+'">Return to menu</a>');}
      else {res.writeHead(303, {Location:'/complete'}); res.end();}
      return;
    }
    if (req.method !== 'GET') {res.writeHead(405); res.end(); return;}
    if (['/profile','/help','/license','/complete'].includes(url.pathname)) {
      res.writeHead(200, {'Content-Type':'text/html; charset=utf-8'}); res.end('<!doctype html><title>Fixture result</title><h1>'+({'/profile':'Profile destination','/help':'Help destination','/license':'License destination','/complete':'Action complete'}[url.pathname])+'</h1>'); return;
    }
    const name = url.pathname.startsWith(prefix+'/') ? url.pathname.slice(prefix.length+1) : '';
    const body = docs.get(name) || assets.get(name);
    if (!body) {res.writeHead(404); res.end(); return;}
    res.writeHead(200, {'Content-Type':docs.has(name) ? 'text/html; charset=utf-8' : name.endsWith('.js') || name.endsWith('.mjs') ? 'text/javascript' : name.endsWith('.woff2') ? 'font/woff2' : 'text/css'});
    res.end(body);
  });
  await bounded(new Promise((resolve,reject) => {server.once('error',reject); server.listen(0,'127.0.0.1',resolve);}));
  base = 'http://127.0.0.1:'+server.address().port;
  const entry = '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
  assert.equal(JSON.parse(await fs.readFile(path.join(path.dirname(entry),'package.json'))).version, '1.63.0');
  const {chromium} = await import(pathToFileURL(entry).href);
  browserServer = await chromium.launchServer({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true, timeout:20000});
  report.browser_pid = browserServer.process().pid;
  browser = await chromium.connect(browserServer.wsEndpoint()); report.browser = browser.version();
  async function ready(page, requireRepair = false) {
    await page.waitForFunction(() => Boolean(customElements.get('el-menu')) && document.fonts.status === 'loaded');
    if (requireRepair) await page.waitForFunction(() => document.querySelector('[data-appkit-menu-repair="active"]'));
    // Elements starts its entry animation on a later frame. One immediate empty
    // getAnimations() sample is not a settled state. Keep source transitions intact.
    let stable=0;
    for(let attempt=0;attempt<80;attempt++) {
      const settled=await page.evaluate(()=>document.getAnimations().every(a=>a.playState!=='running'&&!a.pending)&&[...document.querySelectorAll('el-menu:popover-open')].every(e=>getComputedStyle(e).opacity==='1'));
      stable=settled?stable+1:0;
      if(stable>=3) return;
      await page.waitForTimeout(50);
    }
    throw Error('source animation did not settle');
  }
  async function capture(page, name) {
    let bytes=await page.screenshot(), stable=false;
    for(let attempt=0;attempt<6;attempt++) {
      await ready(page); const next=await page.screenshot();
      if(bytes.equals(next)) {stable=true;break;} bytes=next;
    }
    check(name+': independent screenshot stability',stable);
    await write(path.join(out, name+'.png'), bytes);
    report.captures.push({file:name+'.png', sha256:sha(bytes), bytes:bytes.length}); return bytes;
  }
  async function openWithKeys(page, key) {
    await page.keyboard.press('Tab');
    check(key+': Tab reaches trigger', await page.locator('el-dropdown > button').evaluate(e => e === document.activeElement));
    await page.keyboard.press('Enter'); await ready(page);
    check(key+': Enter opens menu', await page.locator('el-menu').evaluate(e => e.matches(':popover-open')));
    check(key+': first item receives focus', await page.locator('el-menu a').first().evaluate(e => e === document.activeElement));
  }
  async function focusGeometry(page) {
    return page.evaluate(() => {
      const e = document.activeElement, r = e.getBoundingClientRect(), s = getComputedStyle(e), menu = e.closest('el-menu'), mr = menu?.getBoundingClientRect();
      // Tailwind 4 computed colors can be OKLCH. Let the browser convert valid
      // computed CSS to sRGB; do not parse their channel numbers as RGB.
      const canvas=document.createElement('canvas'); canvas.width=canvas.height=1;
      const ctx=canvas.getContext('2d',{willReadFrequently:true});
      const rgb = c => {ctx.clearRect(0,0,1,1);ctx.fillStyle=c;ctx.fillRect(0,0,1,1);return [...ctx.getImageData(0,0,1,1).data];};
      let backdrop = e, bg;
      while (backdrop) {const c = rgb(getComputedStyle(backdrop).backgroundColor); if (c[3] === 255) {bg=c; break;} backdrop=backdrop.parentElement;}
      const luminance = c => c.slice(0,3).map(v => v/255).map(v => v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((v,n,i)=>v+n*[.2126,.7152,.0722][i],0);
      const fg=rgb(s.outlineColor), ratio=fg&&bg ? (Math.max(luminance(fg),luminance(bg))+.05)/(Math.min(luminance(fg),luminance(bg))+.05):0;
      return {height:r.height, width:r.width, contents:e.scrollWidth, box:e.clientWidth, outline:s.outlineStyle, outlineWidth:parseFloat(s.outlineWidth), outlineColor:s.outlineColor, outlineRGB:fg, backdropRGB:bg, ratio,
        contained:r.left>=0&&r.top>=0&&r.right<=innerWidth&&r.bottom<=innerHeight&&(!mr||(r.left>=mr.left&&r.right<=mr.right&&r.top>=mr.top&&r.bottom<=mr.bottom)),
        pageContained:document.documentElement.scrollWidth<=innerWidth};
    });
  }
  for (const width of [390,1440]) for (const scheme of ['light','dark']) {
    const context = await browser.newContext({viewport:{width,height:1000},colorScheme:scheme,reducedMotion:'reduce',serviceWorkers:'block'});
    await context.route('**/*', async route => {const u = new URL(route.request().url()); if (u.origin !== base) {report.errors.push('offsite request refused'); await route.abort();} else {report.requests.push({method:route.request().method(),path:u.pathname}); await route.continue();}});
    const page = await context.newPage(); page.setDefaultTimeout(5000);
    page.on('pageerror', e => report.errors.push('pageerror: '+e.message));
    const navigate = async name => {
      const requestStart = report.requests.length;
      await page.goto(base+prefix+'/'+name+'-'+scheme);
      await ready(page, repair && repairEligible(name));
      if (repair && ['original','reference','typed'].includes(name)) {
        const isolation = await page.locator('#specimen').evaluate(specimen => {
          const menus = [...specimen.querySelectorAll('el-menu')];
          return {
            markers: specimen.querySelectorAll('[data-appkit-menu-repair]').length,
            references: document.querySelectorAll('link[href*="menu_repair"],script[src*="menu_repair"]').length,
            variables: menus.filter(menu => menu.style.getPropertyValue('--appkit-menu-available-block') || getComputedStyle(menu).getPropertyValue('--appkit-menu-available-block').trim()).length,
          };
        });
        const repairRequests = report.requests.slice(requestStart).filter(request => repairNames.has(path.posix.basename(request.path)));
        check(`${scheme}-${width}-${name}: repair remains isolated`, isolation.markers === 0 && isolation.references === 0 && isolation.variables === 0 && repairRequests.length === 0, {...isolation, repairRequests});
      }
    };
    let sourceImage;
    for (const name of ['original','reference','typed','enhanced','enhanced-long']) {
      const key = `${scheme}-${width}-${name}`;
      await navigate(name); await openWithKeys(page,key);
      await page.keyboard.press('ArrowDown');
      check(key+': ArrowDown advances', await page.locator('el-menu a').nth(1).evaluate(e=>e===document.activeElement));
      await page.keyboard.press('ArrowUp');
      check(key+': ArrowUp returns', await page.locator('el-menu a').first().evaluate(e=>e===document.activeElement));
      const geometry=await focusGeometry(page);
      if (name.startsWith('enhanced')) check(key+': focus and reflow', geometry.height>=44&&geometry.width>=44&&geometry.contents<=geometry.box+1&&geometry.outline==='solid'&&geometry.outlineWidth>=2&&geometry.ratio>=3&&geometry.contained&&geometry.pageContained,geometry);
      const image=await capture(page,key+'-open');
      if(name==='original') sourceImage=image;
      if(name==='reference') check(key+': exact original open-state pixels',image.equals(sourceImage));
      await page.keyboard.press('Escape'); await ready(page);
      check(key+': Escape returns focus', await page.locator('el-dropdown > button').evaluate(e=>e===document.activeElement)&&!(await page.locator('el-menu').evaluate(e=>e.matches(':popover-open'))));
      await page.locator('el-dropdown > button').click(); await ready(page);
      check(key+': pointer opens', await page.locator('el-menu').evaluate(e=>e.matches(':popover-open')));
      // The menu can legitimately cover lower page content; use the visible heading.
      await page.locator('h1').click(); await ready(page);
      check(key+': outside pointer dismisses', !(await page.locator('el-menu').evaluate(e=>e.matches(':popover-open'))));
    }
    await navigate('enhanced'); await openWithKeys(page,`${scheme}-${width}-navigation`);
    await Promise.all([page.waitForURL(base+'/profile'),page.keyboard.press('Enter')]);
    check(`${scheme}-${width}: native link navigation`, await page.locator('h1').textContent()==='Profile destination');
    await navigate('enhanced'); await openWithKeys(page,`${scheme}-${width}-post`);
    await page.keyboard.press('End');
    check(`${scheme}-${width}: End reaches POST action`, await page.locator('el-menu button[type=submit]').evaluate(e=>e===document.activeElement));
    const before=actionBodies.length;
    await Promise.all([page.waitForURL(base+'/complete'),page.keyboard.press('Enter')]);
    check(`${scheme}-${width}: native POST+CSRF+303`,actionBodies.length===before+1&&actionBodies.at(-1).valid&&actionBodies.at(-1).status===303&&await page.locator('h1').textContent()==='Action complete');
    await navigate('enhanced'); await openWithKeys(page,`${scheme}-${width}-rejection`); await page.keyboard.press('End'); rejectNext=true;
    await Promise.all([page.waitForURL(base+'/action'),page.keyboard.press('Enter')]);
    check(`${scheme}-${width}: rejection is visible`,await page.locator('h1').textContent()==='Action was not applied'&&actionBodies.at(-1).status===403);
    await page.keyboard.press('Tab'); await Promise.all([page.waitForURL(base+prefix+'/enhanced-'+scheme),page.keyboard.press('Enter')]); await ready(page);
    check(`${scheme}-${width}: native recovery returns to menu`,await page.locator('el-dropdown').count()===1);
    await openWithKeys(page,`${scheme}-${width}-retry`); await page.keyboard.press('End');
    const beforeRetry=actionBodies.length;
    await Promise.all([page.waitForURL(base+'/complete'),page.keyboard.press('Enter')]);
    check(`${scheme}-${width}: explicit retry succeeds once`,actionBodies.length===beforeRetry+1&&actionBodies.at(-1).valid&&actionBodies.at(-1).status===303);
    await navigate('disabled'); await openWithKeys(page,`${scheme}-${width}-disabled`); await page.keyboard.press('End');
    check(`${scheme}-${width}: disabled POST skipped by keys`,await page.locator('el-menu a').last().evaluate(e=>e===document.activeElement)&&await page.locator('el-menu button[type=submit]').isDisabled());
    const beforeDisabled=actionBodies.length, disabledURL=page.url();
    const disabledBox=await page.locator('el-menu button[type=submit]').boundingBox(); assert(disabledBox);
    await page.mouse.click(disabledBox.x+disabledBox.width/2,disabledBox.y+disabledBox.height/2);
    await page.waitForTimeout(100);
    check(`${scheme}-${width}: disabled pointer cannot submit`,actionBodies.length===beforeDisabled&&page.url()===disabledURL);
    await navigate('links-only'); await openWithKeys(page,`${scheme}-${width}-links-only`);
    check(`${scheme}-${width}: absent action has no form or token`,await page.locator('form,input').count()===0);
    await context.close();
  }
  // Independent negative controls: wrong token and wrong origin must not succeed.
  for (const [origin,body] of [[base,'csrf=wrong-token'],['https://invalid.test','csrf=synthetic-menu-token']]) {
    const response=await fetch(base+'/action',{method:'POST',headers:{Origin:origin,'Content-Type':'application/x-www-form-urlencoded'},body,redirect:'manual'});
    check('negative POST '+origin+' '+body.split('=')[1],response.status===403);
    await response.arrayBuffer();
  }
  for(const [file,digest] of inputs) assert.equal(sha(await fs.readFile(file)),digest,'input changed during proof');
  report.assets=Object.fromEntries([...assets].map(([name,body])=>[name,{sha256:sha(body),bytes:body.length}]));
} catch(error) {report.errors.push(String(error)); process.exitCode=1;}
finally {
  try {
    if(browser) await bounded(browser.close());
    if(browserServer) await bounded(browserServer.close());
    if(server?.listening) {server.closeAllConnections(); await bounded(new Promise((resolve,reject)=>server.close(e=>e?reject(e):resolve())));}
    if(report.browser_pid) {
      let gone=false;
      for(let i=0;i<30;i++) {try {process.kill(report.browser_pid,0);} catch(e) {if(e.code==='ESRCH') {gone=true;break;} throw e;} await new Promise(r=>setTimeout(r,100));}
      assert(gone,'owned browser PID still exists'); report.browser_pid_absent=true;
    }
    if(base) {
      await bounded(new Promise((resolve,reject)=>{
        const socket=net.createConnection({host:'127.0.0.1',port:Number(new URL(base).port)});
        socket.once('connect',()=>{socket.destroy();reject(Error('owned listener still accepts connections'));});
        socket.once('error',e=>{socket.destroy();e.code==='ECONNREFUSED'?resolve():reject(e);});
      }));
      report.listener_refused=true;
    }
    report.cleanup='ok';
  } catch(error) {report.cleanup=String(error); report.errors.push('cleanup failed'); browserServer?.process()?.kill('SIGKILL'); server?.closeAllConnections(); process.exitCode=1;}
  clearTimeout(watchdog);
  report.inputs=Object.fromEntries(inputs);
  report.passed=report.checks.filter(c=>c.ok).length; report.failed=report.checks.filter(c=>!c.ok).length;
  if(report.failed||report.errors.length) process.exitCode=1;
  const body=JSON.stringify(report,null,2)+'\n'; await write(path.join(out,'receipt.json'),body);
  process.stdout.write(JSON.stringify({out,passed:report.passed,failed:report.failed,errors:report.errors.map(e=>e.split('\n')[0]),receipt_sha256:sha(body)})+'\n');
}
