// P07 actual browser zoom. Reuses independently hash-bound private inputs.
// No downloads, dependency installation, signed-in profile or source edits.
import assert from 'node:assert/strict';
import {createHash, randomBytes} from 'node:crypto';
import {spawnSync} from 'node:child_process';
import fs from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import {createRequire} from 'node:module';
import {pathToFileURL} from 'node:url';

const [probeArg, priorArg, priorSHA, repairFlag, repairKitArg, repairSHA] = process.argv.slice(2);
const deliveredRepair = process.argv.length === 8 && repairFlag === '--repair-delivery' && path.isAbsolute(repairKitArg ?? '') && /^[a-f0-9]{64}$/.test(repairSHA ?? '');
const repair = deliveredRepair || process.argv.length === 7 && repairFlag === '--repair' && path.isAbsolute(repairKitArg ?? '');
assert((process.argv.length === 5 || repair) && path.isAbsolute(probeArg) && path.isAbsolute(priorArg) && /^[a-f0-9]{64}$/.test(priorSHA ?? ''),
  'usage: node zoom.mjs PRIVATE_PROBE BROWSER_RECEIPT EXPECTED_SHA256 [--repair KIT | --repair-delivery PRIVATE_DELIVERY ARTIFACT_SHA256]');
const probe = await fs.realpath(probeArg), priorPath = await fs.realpath(priorArg);
assert(probe.startsWith(await fs.realpath(os.tmpdir()) + path.sep) && priorPath.startsWith(probe + path.sep));
const sha = b => createHash('sha256').update(b).digest('hex');
const write = (p, b) => fs.writeFile(p, b, {mode: 0o600, flag: 'wx'});
const out = await fs.mkdtemp(path.join(probe, 'zoom-')); await fs.chmod(out, 0o700);
const report = {kind: 'p07-menu-actual-zoom', verification: 'not-verified', browser_receipt_sha256: priorSHA,
  variant: deliveredRepair ? 'delivered-repair' : repair ? 'experimental-repair' : 'unchanged-v1',
  checks: [], captures: [], requests: [], actions: [], errors: [], hosts: [], inputs: {}, cleanup: 'pending',
  limits: ['Actual 200% zoom at one desktop size in headless Chrome with browser-protocol input, not OS/physical input.',
    'Synthetic app-owned action receiver, not real identity or consumer adoption.', 'No touch, screen-reader, Datastar lifecycle or performance certification.']};
const inputs = new Map();
async function input(file, expected) {
  const stat = await fs.lstat(file);
  assert(stat.isFile() && stat.nlink === 1 && stat.size > 0 && stat.size <= 8 * 2 ** 20, 'invalid bounded input');
  const bytes = await fs.readFile(file), digest = sha(bytes);
  if (expected) assert.equal(digest, expected, 'input identity differs: ' + path.basename(String(file)));
  inputs.set(String(file), digest); return bytes;
}
const check = (name, ok, detail = {}) => {report.checks.push({name, ok: Boolean(ok), ...detail}); assert(ok, name);};
let context, server, base, profile, pid;
function alive() {
  if (!pid) return false;
  const p = spawnSync('/bin/ps', ['-p', String(pid), '-o', 'command='], {timeout: 2000, maxBuffer: 16384});
  if (p.status === 1) return false;
  assert.equal(p.status, 0, 'could not inspect owned browser');
  assert(p.stdout.toString().includes('--user-data-dir=' + profile), 'browser identity differs'); return true;
}
async function bounded(promise) {
  let timer;
  try {return await Promise.race([promise, new Promise((_, reject) => {timer = setTimeout(() => reject(Error('operation deadline')), 10000);})]);}
  finally {clearTimeout(timer);}
}
const watchdog = setTimeout(() => {
  report.errors.push('four-minute deadline'); process.exitCode = 1;
  try {if (alive()) process.kill(pid, 'SIGKILL');} catch {report.errors.push('owned process stop failed');}
  server?.closeAllConnections();
}, 240000);
try {
  report.harness_sha256 = sha(await input(new URL(import.meta.url)));
  const prior = JSON.parse(await input(priorPath, priorSHA));
  assert.equal(prior.kind, 'p07-menu-browser'); assert.equal(prior.verification, 'not-verified');
  assert.equal(prior.passed, 302); assert.equal(prior.failed, 0); assert.deepEqual(prior.errors, []);
  assert.equal(prior.cleanup, 'ok'); assert(prior.browser_pid_absent && prior.listener_refused);
  const compile = JSON.parse(await input(path.join(probe, 'compile-receipt.json'), prior.compile_receipt_sha256));
  const artifact = JSON.parse(await input(path.join(probe, 'delivery/artifact.json'), compile.artifact_sha256));
  assert.equal(artifact.adapter, 'p07-menu-v1'); assert.equal(artifact.verification, 'not-verified');
  await input(path.join(probe, 'original.html'), artifact.source.sha256);
  for (const [file, key] of [['menu.templ','templ_sha256'], ['menu.go','model_sha256'], ['defaults.json','defaults_sha256']]) await input(path.join(probe, 'delivery', file), artifact[key]);
  const oldRoot = path.dirname(priorPath), prefix = '/zoom-' + randomBytes(18).toString('hex');
  const resources = new Map();
  if(repair){
    const repairKit=await fs.realpath(repairKitArg);
    let js,css;
    if(deliveredRepair){
      assert.equal((await fs.lstat(repairKit)).mode&0o777,0o700,'private delivery root');
      const names=['USAGE.md','artifact.json','defaults.json','menu.css','menu.go','menu.templ','menu_repair.css','menu_repair.mjs'];
      assert.deepEqual((await fs.readdir(repairKit)).sort(),names,'exact eight-file repair inventory');
      for(const name of names)assert.equal((await fs.lstat(path.join(repairKit,name))).mode&0o777,0o600,'private delivery file');
      const selected=JSON.parse(await input(path.join(repairKit,'artifact.json'),repairSHA));
      assert.equal(selected.schema_version,1);assert.equal(selected.adapter,'p07-menu-repair-v1');
      assert.equal(selected.verification,'not-verified');assert.deepEqual(selected.base,artifact);
      assert.deepEqual(selected.mount,{gap:8,edge:16});assert.match(selected.rule_sha256,/^[a-f0-9]{64}$/);
      for(const [file,key] of [['menu.templ','templ_sha256'],['menu.go','model_sha256'],['menu.css','css_sha256'],['defaults.json','defaults_sha256']])await input(path.join(repairKit,file),artifact[key]);
      js=await input(path.join(repairKit,'menu_repair.mjs'),selected.module_sha256);
      css=await input(path.join(repairKit,'menu_repair.css'),selected.css_sha256);
      assert.equal(js.toString(),selected.module);assert.equal(css.toString(),selected.css);
      await input(path.join(repairKit,'USAGE.md'));
      report.repair_delivery={path:repairKit,artifact_sha256:repairSHA,rule_sha256:selected.rule_sha256};
    }else{
      js=await input(path.join(repairKit,'internal/designadapt/menu_repair.mjs'));
      css=await input(path.join(repairKit,'internal/designadapt/menu_repair.css'));
    }
    resources.set(prefix+'/repair.mjs',{type:'text/javascript',body:js});
    resources.set(prefix+'/repair.css',{type:'text/css',body:css});
    // Test-only host handle: exercise real disposal without exporting this API.
    const boot=Buffer.from('import {mountMenuRepair} from "./repair.mjs"; await customElements.whenDefined("el-menu"); const disposers = new Map(); for (const root of document.querySelectorAll("[data-appkit-menu]")) disposers.set(root, mountMenuRepair(root)); window.menuRepairProbe = {mount: mountMenuRepair, disposers};');
    resources.set(prefix+'/repair-boot.mjs',{type:'text/javascript',body:boot});
    report.repair={module_sha256:sha(js),css_sha256:sha(css),boot_sha256:sha(boot),delivery:deliveredRepair?'exact private eight-file variant; test host owns mounting':'not part of immutable v1 export'};
  }
  const css = await input(path.join(oldRoot, 'source.css'), prior.assets['source.css'].sha256);
  const companion = await input(path.join(probe, 'delivery/menu.css'), artifact.css_sha256);
  const fromPrior = async name => {
    const matches = Object.keys(prior.inputs).filter(p => p.endsWith('/' + name)); assert.equal(matches.length, 1);
    return input(matches[0], prior.assets[name].sha256);
  };
  const runtime = await fromPrior('elements.js'), font = await fromPrior('inter.woff2');
  assert.equal(sha(runtime), artifact.runtime.esm_sha256);
  const fontPath = css.toString().match(/url\(["']?(\/menu-[a-f0-9]+\/inter\.woff2)["']?\)/)?.[1];
  assert(fontPath, 'prior CSS font URL missing');
  resources.set(fontPath, {type: 'font/woff2', body: font});
  for (const [name, body, type] of [['source.css',css,'text/css'], ['menu.css',companion,'text/css'], ['elements.js',runtime,'text/javascript']]) resources.set(prefix+'/'+name, {type,body});
  const fixture = Buffer.from('html{font-family:InterVariable,system-ui,sans-serif}body{margin:0;padding:24px;background:#f9fafb;color:#111827}html.dark body{background:#111827;color:#f9fafb}h1{font-size:1rem;margin:0 0 24px}#outside{display:block;margin-top:32px;padding:12px}#fallback{display:block;margin-top:16px}');
  assert.equal(sha(fixture), prior.assets['fixture.css'].sha256);
  resources.set(prefix+'/fixture.css', {type:'text/css',body:fixture});
  const kinds = ['enhanced','enhanced-long','disabled','links-only'];
  for (const kind of kinds) {
    const fragment = await input(path.join(probe,kind+'.html'),compile.rendered_sha256[kind+'.html']);
    for (const scheme of ['light','dark']) {
      const body = Buffer.from('<!doctype html><html lang="en" class="'+(scheme==='dark'?'dark':'')+'"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Menu zoom task</title><link rel="icon" href="data:,"><link rel="stylesheet" href="'+prefix+'/source.css"><link rel="stylesheet" href="'+prefix+'/fixture.css"><link rel="stylesheet" href="'+prefix+'/menu.css"><script type="module" src="'+prefix+'/elements.js"></script>'+(repair?'<link rel="stylesheet" href="'+prefix+'/repair.css"><script type="module" src="'+prefix+'/repair-boot.mjs"></script>':'')+'</head><body><h1>Synthetic account actions</h1><main>'+fragment+'</main><button id="outside" type="button">Outside menu</button><a id="fallback" href="/profile">Open profile without the menu</a></body></html>');
      resources.set(prefix+'/'+kind+'-'+scheme,{type:'text/html',body});
    }
  }
  if(repair){
    const long=await input(path.join(probe,'enhanced-long.html'),compile.rendered_sha256['enhanced-long.html']);
    const reference=await input(path.join(probe,'reference.html'),compile.rendered_sha256['reference.html']);
    for(const scheme of ['light','dark']){
      const existing=resources.get(prefix+'/enhanced-long-'+scheme).body.toString();
      const old='<main>'+long+'</main>';
      assert.equal(existing.split(old).length,2,'one immutable specimen insertion point');
      const body=Buffer.from(existing.replace(old,'<main><section id="one">'+long+'</section><section id="two">'+long+'</section><section id="unmounted">'+reference+'</section></main>'));
      resources.set(prefix+'/multiple-'+scheme,{type:'text/html',body});
    }
    report.limits.push('Lifecycle fault injection uses DOM focus, resize events and host disposal; these checks are not physical or browser-keyboard input claims.');
  }
  server = http.createServer(async (req,res) => {
    res.setHeader('Cache-Control','no-store'); res.setHeader('X-Content-Type-Options','nosniff');
    res.setHeader('Content-Security-Policy',"default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src data:; connect-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'");
    const u = new URL(req.url,base);
    if (req.headers.host!==new URL(base).host || u.origin!==base || u.search || req.headers['sec-fetch-site']==='cross-site' || (req.headers.origin!==undefined&&req.headers.origin!==base)) {res.writeHead(403).end();return;}
    if(req.method==='POST'&&u.pathname==='/action') {
      if(req.headers.origin!==base||req.headers['content-type']?.split(';')[0]!=='application/x-www-form-urlencoded'){res.writeHead(403).end();return;}
      let count=0;const chunks=[];
      for await(const chunk of req){count+=chunk.length;if(count>8192){res.writeHead(413).end();return;}chunks.push(chunk);}
      const body=Buffer.concat(chunks), fields=new URLSearchParams(body.toString());
      const valid=fields.size===1&&fields.getAll('csrf').length===1&&fields.get('csrf')==='synthetic-menu-token';
      report.actions.push({valid,sha256:sha(body),bytes:body.length});
      res.writeHead(valid?303:403,valid?{Location:'/complete'}:{}).end();return;
    }
    if(req.method!=='GET'){res.writeHead(405).end();return;}
    if(['/profile','/help','/license','/complete'].includes(u.pathname)){res.writeHead(200,{'Content-Type':'text/html'}).end('<!doctype html><title>Result</title><h1>'+u.pathname.slice(1)+'</h1>');return;}
    const r=resources.get(u.pathname);if(!r){res.writeHead(404).end();return;}
    res.writeHead(200,{'Content-Type':r.type});res.end(r.body);
  });
  await bounded(new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);}));
  base='http://127.0.0.1:'+server.address().port;report.base=base;
  const pw='/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
  await input(pw,'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
  const {chromium}=await import(pathToFileURL(pw));
  const {PNG}=createRequire(import.meta.url)('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright-core/lib/utilsBundle.js');
  for(const scheme of ['light','dark']) {
    profile=await fs.mkdtemp(path.join(out,'profile-'));await fs.chmod(profile,0o700);
    context=await chromium.launchPersistentContext(profile,{executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true,timeout:20000,viewport:{width:1440,height:1000},colorScheme:scheme,reducedMotion:'reduce',serviceWorkers:'block',args:['--enable-automation','--disable-background-networking','--no-proxy-server','--no-first-run']});
    const host=await context.browser().newBrowserCDPSession();
    try {
      const argv=await host.send('Browser.getBrowserCommandLine');assert(argv.arguments.includes('--user-data-dir='+profile));
      const processes=await host.send('SystemInfo.getProcessInfo'), hosts=processes.processInfo.filter(p=>p.type==='browser');assert.equal(hosts.length,1);
      pid=hosts[0].id;assert(alive());report.hosts.push({pid,profile});
    }finally{await host.detach();}
    const page=context.pages()[0];page.setDefaultTimeout(6000);page.setDefaultNavigationTimeout(10000);
    await page.goto('chrome://settings/appearance');await page.locator('#zoomLevel').selectOption('2');
    check(scheme+':actual browser zoom setting',await page.locator('#zoomLevel').inputValue()==='2');
    await page.goto('about:blank');report.browser=context.browser().version();
    await context.routeWebSocket('**/*',socket=>{report.errors.push('unexpected websocket');socket.close();});
    await context.route('**/*',route=>{const r=route.request(),u=new URL(r.url());if(u.origin!==base){report.errors.push('offsite request refused');return route.abort();}report.requests.push({method:r.method(),path:u.pathname});return route.continue();});
    page.on('pageerror',()=>report.errors.push('pageerror'));
    page.on('requestfailed',r=>report.errors.push('request failed: '+new URL(r.url()).pathname));
    page.on('response',r=>{if(resources.has(new URL(r.url()).pathname)&&r.status()!==200)report.errors.push('asset response status '+r.status());});
    async function ready(){await page.waitForFunction(()=>Boolean(customElements.get('el-menu'))&&document.fonts.status==='loaded');if(repair)await page.waitForFunction(()=>document.querySelector('[data-appkit-menu][data-appkit-menu-repair="active"]'));let stable=0;for(let i=0;i<80;i++){const ok=await page.evaluate(()=>document.getAnimations().every(a=>a.playState!=='running'&&!a.pending)&&[...document.querySelectorAll('el-menu:popover-open')].every(e=>getComputedStyle(e).opacity==='1'));stable=ok?stable+1:0;if(stable===3)return;await page.waitForTimeout(50);}throw Error('animation unsettled');}
    async function geometry(){return page.evaluate(()=>{
      const e=document.activeElement,r=e.getBoundingClientRect(),s=getComputedStyle(e),m=e.closest('el-menu'),mr=m?.getBoundingClientRect();
      const canvas=document.createElement('canvas');canvas.width=canvas.height=1;const ctx=canvas.getContext('2d');
      const rgba=c=>{ctx.clearRect(0,0,1,1);ctx.fillStyle=c;ctx.fillRect(0,0,1,1);return [...ctx.getImageData(0,0,1,1).data];};
      let bg;for(let n=e;n;n=n.parentElement){const c=rgba(getComputedStyle(n).backgroundColor);if(c[3]===255){bg=c;break;}}
      const fg=rgba(s.outlineColor),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);
      const a=lum(fg),b=bg?lum(bg):0;
      return {tag:e.tagName,href:e.getAttribute('href'),type:e.getAttribute('type'),width:r.width,height:r.height,content:e.scrollWidth,box:e.clientWidth,outline:s.outlineStyle,outlineWidth:parseFloat(s.outlineWidth),contrast:bg?(Math.max(a,b)+.05)/(Math.min(a,b)+.05):0,
        contained:r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight&&(!mr||(r.left>=mr.left&&r.right<=mr.right&&r.top>=mr.top&&r.bottom<=mr.bottom)),pageContained:document.documentElement.scrollWidth<=innerWidth,menuScroll:m?.scrollTop};
    });}
    async function capture(name){const cdp=await context.newCDPSession(page);try{
      const {data}=await cdp.send('Page.captureScreenshot',{format:'png',fromSurface:true,captureBeyondViewport:false});const bytes=Buffer.from(data,'base64'),png=PNG.sync.read(bytes),colors=new Set();let dark=0,light=0,opaque=0;
      for(let i=0;i<png.data.length;i+=4){const [r,g,b,a]=png.data.subarray(i,i+4);if(Math.max(r,g,b)<100)dark++;if(Math.min(r,g,b)>220)light++;if(a===255)opaque++;if(colors.size<100)colors.add((r<<16)|(g<<8)|b);}
      const pixels=png.width*png.height;check(name+':nonblank painted viewport',png.width===1440&&png.height===1000&&opaque===pixels&&colors.size>=32&&dark/pixels>.001&&light/pixels>.001,{width:png.width,height:png.height,colors:colors.size,dark,light});
      await write(path.join(out,name+'.png'),bytes);report.captures.push({file:name+'.png',sha256:sha(bytes),bytes:bytes.length});
    }finally{await cdp.detach();}}
    for(const kind of kinds){const key=scheme+'-'+kind,navigate=async()=>{await page.goto(base+prefix+'/'+kind+'-'+scheme);await ready();};
      await navigate();const zoom=await page.evaluate(()=>({inner:innerWidth,outer:outerWidth,dpr:devicePixelRatio,scale:visualViewport.scale}));
      check(key+':actual 200 percent',zoom.inner===720&&zoom.outer===1440&&zoom.dpr===2&&zoom.scale===1,zoom);
      await page.keyboard.press('Tab');check(key+':Tab reaches trigger',await page.locator('el-dropdown > button').evaluate(e=>e===document.activeElement));
      await page.keyboard.press('Enter');await ready();check(key+':opens',await page.locator('el-menu').evaluate(e=>e.matches(':popover-open')));
      const items=page.locator('el-menu a,el-menu button:not(:disabled)'),count=await items.count();
      check(key+':enabled item inventory',count===(kind==='disabled'||kind==='links-only'?3:4));
      const menuBounds=await page.locator('el-menu').evaluate(e=>({rect:e.getBoundingClientRect().toJSON(),width:innerWidth,height:innerHeight}));
      // Record placement independently so a focus failure cannot hide the height defect.
      report.checks.push({name:key+':anchored viewport containment',ok:menuBounds.rect.left>=0&&menuBounds.rect.top>=0&&menuBounds.rect.right<=menuBounds.width&&menuBounds.rect.bottom<=menuBounds.height,...menuBounds});
      for(let i=0;i<count;i++){
        if(i)await page.keyboard.press('ArrowDown');await ready();
        const focused=await items.nth(i).evaluate(e=>e===document.activeElement);
        if(!focused){report.focus_failure=await page.evaluate(()=>{const m=document.querySelector('el-menu'),s=getComputedStyle(m);return{activeTag:document.activeElement.tagName,activeIndex:[...document.querySelectorAll('el-menu a,el-menu button:not(:disabled)')].indexOf(document.activeElement),scroll:m.scrollTop,menu:{attrs:[...m.attributes].map(a=>[a.name,a.value]),rect:m.getBoundingClientRect().toJSON(),maxHeight:s.maxHeight,maxBlockSize:s.maxBlockSize,overflow:s.overflow,anchor:s.positionAnchor,position:s.position},items:[...document.querySelectorAll('el-menu a,el-menu button')].map(e=>{const r=e.getBoundingClientRect();return{tag:e.tagName,tabIndex:e.tabIndex,top:r.top,bottom:r.bottom,height:r.height};})};});await capture(key+'-unexpected-focus');}
        check(key+':Arrow focus '+i,focused);
        const g=await geometry();check(key+':visible focus and wrapping '+i,g.width>=44&&g.height>=44&&g.content<=g.box+1&&g.outline==='solid'&&g.outlineWidth>=2&&g.contrast>=3&&g.contained&&g.pageContained,g);
        if(i===count-1)await capture(key+'-last-focus');
      }
      await page.keyboard.press('Home');check(key+':Home first',await items.first().evaluate(e=>e===document.activeElement));
      await page.keyboard.press('Escape');await ready();check(key+':Escape restores trigger',await page.locator('el-dropdown > button').evaluate(e=>e===document.activeElement)&&!(await page.locator('el-menu').evaluate(e=>e.matches(':popover-open'))));
      await page.keyboard.press('Enter');await ready();
      if(kind==='enhanced'||kind==='enhanced-long'){
        await page.keyboard.press('End');const before=report.actions.length;
        await Promise.all([page.waitForURL(base+'/complete'),page.keyboard.press('Enter')]);check(key+':native protected POST',report.actions.length===before+1&&report.actions.at(-1).valid&&await page.locator('h1').textContent()==='complete');
      }else{
        if(kind==='disabled'){await page.keyboard.press('End');check(key+':disabled action skipped',await items.last().evaluate(e=>e===document.activeElement));await page.keyboard.press('Home');}
        if(kind==='links-only')check(key+':no hidden form',await page.locator('form,input').count()===0);
        await Promise.all([page.waitForURL(base+'/profile'),page.keyboard.press('Enter')]);check(key+':native link',await page.locator('h1').textContent()==='profile');
      }
    }
    if(repair){
      await page.goto(base+prefix+'/enhanced-long-'+scheme);await ready();
      await page.keyboard.press('Tab');await page.keyboard.press('Enter');await ready();
      // Real browser viewport changes, not CSS zoom or a smaller screenshot crop.
      for(const size of [{width:780,height:1400},{width:1440,height:1000}]){
        await page.setViewportSize(size);await ready();
        const key=scheme+'-resize-'+size.width;
        const bounds=await page.locator('el-menu').evaluate(e=>({rect:e.getBoundingClientRect().toJSON(),width:innerWidth,height:innerHeight,dpr:devicePixelRatio}));
        check(key+':retains 200 percent',bounds.width===size.width/2&&bounds.height===size.height/2&&bounds.dpr===2,bounds);
        check(key+':anchored containment',bounds.rect.left>=0&&bounds.rect.top>=0&&bounds.rect.right<=bounds.width&&bounds.rect.bottom<=bounds.height,bounds);
        await page.keyboard.press('Home');
        const items=page.locator('el-menu a,el-menu button:not(:disabled)');
        for(let i=0;i<await items.count();i++){
          if(i)await page.keyboard.press('ArrowDown');await ready();
          check(key+':runtime selects '+i,await items.nth(i).evaluate(e=>e===document.activeElement));
          const g=await geometry();check(key+':visible focused item '+i,g.contained&&g.pageContained&&g.content<=g.box+1&&g.width>=44&&g.height>=44,g);
        }
      }
      await page.keyboard.press('Escape');await ready();
      // A position-only host layout shift must also update the height cap.
      const beforeShift=await page.locator('el-dropdown > button').evaluate(e=>e.getBoundingClientRect().top);
      // The enhanced wrapper is display:contents; move the real host layout box.
      await page.locator('main').evaluate(e=>{e.style.paddingTop='220px';});
      check(scheme+':host actually shifts the anchor',await page.locator('el-dropdown > button').evaluate(e=>e.getBoundingClientRect().top)>=beforeShift+220);
      await page.keyboard.press('Enter');await ready();
      const flipped=await page.locator('el-dropdown').evaluate(e=>({menu:e.querySelector('el-menu').getBoundingClientRect().toJSON(),anchor:e.querySelector('button').getBoundingClientRect().toJSON(),height:innerHeight}));
      check(scheme+':runtime flips above shifted anchor',flipped.menu.top>=0&&flipped.menu.bottom<=flipped.anchor.top&&flipped.menu.bottom<=flipped.height,flipped);
      await capture(scheme+'-shifted-anchor');
      await page.goto(base+prefix+'/multiple-'+scheme);await ready();
      const roots=page.locator('[data-appkit-menu]');
      check(scheme+':two explicit mounts',await roots.count()===2&&await page.locator('[data-appkit-menu-repair="active"]').count()===2);
      const unmounted=()=>page.locator('#unmounted').evaluate(root=>[...root.querySelectorAll('*')].map(e=>({tag:e.tagName,marker:e.getAttribute('data-appkit-menu-repair'),variable:e.style.getPropertyValue('--appkit-menu-available-block'),font:getComputedStyle(e).font,color:getComputedStyle(e).color,background:getComputedStyle(e).backgroundColor})));
      const untouched=await unmounted();
      check(scheme+':reference stays unmounted',untouched.every(e=>e.marker===null&&e.variable===''));
      await page.keyboard.press('Tab');await page.keyboard.press('Enter');await ready();
      await page.keyboard.press('ArrowDown');
      check(scheme+':first instance selection',await roots.nth(0).locator('a').nth(1).evaluate(e=>e===document.activeElement));
      await page.keyboard.press('Escape');await ready();
      const firstCap=await roots.nth(0).locator('el-menu').evaluate(e=>e.style.getPropertyValue('--appkit-menu-available-block'));
      await roots.nth(1).locator('el-dropdown > button').click();await ready();
      await page.keyboard.press('Home');await page.keyboard.press('ArrowDown');
      check(scheme+':second instance selection isolated',await roots.nth(1).locator('a').nth(1).evaluate(e=>e===document.activeElement)&&!(await roots.nth(0).locator('el-menu').evaluate(e=>e.matches(':popover-open'))));
      check(scheme+':first instance cap unchanged',await roots.nth(0).locator('el-menu').evaluate(e=>e.style.getPropertyValue('--appkit-menu-available-block'))===firstCap);
      check(scheme+':unmounted reference styling unchanged',JSON.stringify(await unmounted())===JSON.stringify(untouched));
      await page.keyboard.press('Escape');await ready();
      const lifecycle=await page.evaluate(async()=>{
        const root=document.querySelector('#two [data-appkit-menu]'), menu=root.querySelector('el-menu'), outside=document.querySelector('#outside');
        const probe=window.menuRepairProbe, property='--appkit-menu-available-block', marker='data-appkit-menu-repair';
        let refused=false;try{probe.mount(root);}catch(e){refused=e instanceof TypeError;}
        probe.disposers.get(root)();
        menu.style.setProperty(property,'123px','important');root.setAttribute(marker,'host-owned');
        const dispose=probe.mount(root);probe.disposers.set(root,dispose);
        // Lifecycle fault injection: enqueue the companion's real focus microtask
        // and RAF, then dispose in the same JS task before either can execute.
        menu.showPopover();for(const item of menu.querySelectorAll('a,button'))item.tabIndex=-1;
        menu.querySelector('a').tabIndex=0;menu.focus();
        const held=document.activeElement===menu&&[...menu.querySelectorAll('a,button')].filter(e=>e.tabIndex===0&&!e.disabled).length===1;
        window.dispatchEvent(new Event('resize'));dispose();dispose();
        await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
        const after={refused,held,focusStayed:document.activeElement===menu,marker:root.getAttribute(marker),value:menu.style.getPropertyValue(property),priority:menu.style.getPropertyPriority(property)};
        menu.hidePopover();
        const again=probe.mount(root);
        menu.showPopover();menu.focus();menu.hidePopover();outside.focus();
        await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
        after.closedFocusStayed=document.activeElement===outside&&!menu.matches(':popover-open');
        again();root.remove();
        await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
        return {...after,remountDisposed:root.getAttribute(marker)==='host-owned'&&menu.style.getPropertyValue(property)==='123px'&&menu.style.getPropertyPriority(property)==='important',removed:!root.isConnected,firstActive:document.querySelector('#one [data-appkit-menu]').getAttribute(marker)==='active'};
      });
      check(scheme+':duplicate mount refused',lifecycle.refused);
      check(scheme+':disposal queued-focus precondition',lifecycle.held);
      check(scheme+':disposed work cannot steal focus',lifecycle.focusStayed,lifecycle);
      check(scheme+':closed menu cannot recover old focus',lifecycle.closedFocusStayed,lifecycle);
      check(scheme+':exact host state restored',lifecycle.marker==='host-owned'&&lifecycle.value==='123px'&&lifecycle.priority==='important',lifecycle);
      check(scheme+':remount remove and sibling isolation',lifecycle.remountDisposed&&lifecycle.removed&&lifecycle.firstActive,lifecycle);
      check(scheme+':reference survives other disposal',JSON.stringify(await unmounted())===JSON.stringify(untouched));
      await page.locator('#one el-dropdown > button').click();await ready();
      await page.keyboard.press('Home');
      check(scheme+':remaining instance still interactive',await page.locator('#one el-menu a').first().evaluate(e=>e===document.activeElement));
    }
    await bounded(context.close());context=undefined;
    check(scheme+':owned browser absent',!alive());report.hosts.at(-1).absent=true;pid=undefined;
  }
}catch(error){report.errors.push(String(error));process.exitCode=1;}
finally{
  try{for(const [file,digest] of inputs)assert.equal(sha(await fs.readFile(file.startsWith('file:')?new URL(file):file)),digest,'input changed during proof');report.inputs_unchanged=true;}
  catch(error){report.errors.push('input recheck: '+String(error));process.exitCode=1;}
  try{
    if(context)await bounded(context.close());if(pid){assert(!alive(),'owned browser still alive');report.hosts.at(-1).absent=true;}
    if(server?.listening){server.closeAllConnections();await bounded(new Promise((resolve,reject)=>server.close(e=>e?reject(e):resolve())));}
    if(base)await bounded(new Promise((resolve,reject)=>{const s=net.createConnection({host:'127.0.0.1',port:Number(new URL(base).port)});s.once('connect',()=>{s.destroy();reject(Error('listener still accepts'));});s.once('error',e=>{s.destroy();e.code==='ECONNREFUSED'?resolve():reject(e);});}));
    report.cleanup='ok';report.listener_refused=Boolean(base);
  }catch(error){report.errors.push('cleanup: '+String(error));try{if(alive())process.kill(pid,'SIGKILL');}catch{}server?.closeAllConnections();process.exitCode=1;}
  clearTimeout(watchdog);report.inputs=Object.fromEntries(inputs);report.passed=report.checks.filter(c=>c.ok).length;report.failed=report.checks.filter(c=>!c.ok).length;
  if(report.failed||report.errors.length)process.exitCode=1;const body=JSON.stringify(report,null,2)+'\n';await write(path.join(out,'receipt.json'),body);
  process.stdout.write(JSON.stringify({out,passed:report.passed,failed:report.failed,errors:report.errors.map(e=>e.split('\n')[0]),receipt_sha256:sha(body)})+'\n');
}
