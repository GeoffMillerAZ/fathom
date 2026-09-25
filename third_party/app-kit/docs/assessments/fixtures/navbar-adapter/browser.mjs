// Local P09 screening. Immutable inputs; no downloads, signed-in profile or app data.
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
  const name = file instanceof URL ? fileURLToPath(file) : file, stat = await fs.lstat(name);
  assert(stat.isFile() && stat.nlink === 1 && stat.size > 0 && stat.size <= 8 * 2**20, 'invalid bounded input');
  const bytes = await fs.readFile(name), hash = sha(bytes);
  if (expected) assert.equal(hash, expected, 'input identity differs');
  inputs.set(name, hash); return bytes;
}
const out = await fs.mkdtemp(path.join(probe, 'browser-'));
await fs.chmod(out, 0o700);
async function write(name, bytes) {
  const target = path.join(out, name);
  await fs.writeFile(target, bytes, {mode:0o600, flag:'wx'});
  outputs.set(name, {sha256:sha(bytes), bytes:bytes.length}); return target;
}
const report = {schema:1, kind:'private-p09-navbar-browser-screening', verification:'not-certified', compile_receipt_sha256:receiptSHA,
  checks:[], captures:[], requests:[], blocked_external:[], csp_blocks:[], errors:[], cleanup:'pending',
  limits:['Original external images are blocked, not substituted: original-image fidelity remains unverified.',
    'Typed images and destinations are first-party synthetic fixtures, not app backends.',
    'Protocol keyboard/pointer automation is not physical input or screen-reader proof.',
    'No actual 200% zoom, touch, Datastar morph, adverse network, performance, native-host or consumer acceptance claim.']};
const check = (name, ok, detail={}) => report.checks.push({name, ok:Boolean(ok), ...detail});
const containsRect = (outer,inner,t=1) => inner.left>=outer.left-t&&inner.right<=outer.right+t&&inner.top>=outer.top-t&&inner.bottom<=outer.bottom+t;
const responsiveOK = (state,width,open=false) => state.desktopTotal>0&&state.desktopTotal===state.mobileTotal&&
  (width===390 ? state.triggerVisible&&state.desktopVisible===0&&state.mobileVisible===(open?state.mobileTotal:0)
    : !state.triggerVisible&&state.desktopVisible===state.desktopTotal&&state.mobileVisible===0);
const classifySourceRefusals = (events,expected,external) => external.length===0&&expected.length===3&&new Set(expected).size===3&&
  events.length===3&&events.every(e=>e.directive==='img-src'&&expected.includes(e.blocked))&&
  expected.every(url=>events.filter(e=>e.blocked===url).length===1);
const assessTextGeometry = (facts,viewport) => ({
  entries:facts.entries.map(entry=>({...entry,fits:(entry.fragments.length>0||entry.ignored.length>0)&&entry.fragments.every(fragment=>
    containsRect(entry.box,fragment)&&containsRect(viewport,fragment)&&(!entry.popover||containsRect(entry.popover,fragment)))})),
  popoversFit:facts.popovers.every(rect=>containsRect(viewport,rect)),
});
check('classifier canary: simultaneous responsive groups are refused',!responsiveOK({desktopTotal:2,mobileTotal:2,desktopVisible:2,mobileVisible:2,triggerVisible:true},390,true));
check('classifier canary: visible text overflow is refused',!assessTextGeometry({entries:[{box:{left:0,right:20,top:0,bottom:20},fragments:[{left:0,right:22,top:0,bottom:20}],ignored:[],popover:null}],popovers:[]},{left:0,right:100,top:0,bottom:100}).entries[0].fits);
const canaryURLs=['https://example.test/a','https://example.test/b','https://example.test/c'];
const canaryEvents=canaryURLs.map(blocked=>({directive:'img-src',blocked}));
check('classifier canary: unwanted CSP or routed traffic is refused',classifySourceRefusals(canaryEvents,canaryURLs,[])&&!classifySourceRefusals([...canaryEvents,{directive:'script-src',blocked:'https://example.test/x'}],canaryURLs,[])&&!classifySourceRefusals(canaryEvents,canaryURLs,[{url:'x'}]));
let server, browserServer, browser, base;
const prefix = '/navbar-' + randomBytes(18).toString('hex');
const watchdog = setTimeout(() => { report.errors.push('four-minute watchdog'); process.exitCode=1; browserServer?.process()?.kill('SIGKILL'); server?.closeAllConnections(); }, 240000);
async function bounded(promise) {
  let timer;
  try { return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('operation timed out')),10000);})]); }
  finally { clearTimeout(timer); }
}
try {
  report.harness_sha256 = sha(await input(new URL(import.meta.url)));
  const compile = JSON.parse(await input(path.join(probe,'compile-receipt.json'),receiptSHA));
  assert.equal(compile.verification,'not-verified');
  const artifact = JSON.parse(await input(path.join(probe,'delivery/artifact.json'),compile.artifact_sha256));
  assert.equal(artifact.adapter,'p09-responsive-navbar-v1'); assert.equal(artifact.verification,'not-verified');
  const source = await input(path.join(probe,'original.html'),'79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7');
  assert.equal(artifact.source.sha256,sha(source));
  for (const key of ['source','syntax','runtime']) assert.deepEqual(compile[key],artifact[key]);
  for (const key of ['rule_sha256','templ_sha256','model_sha256']) assert.equal(compile[key],artifact[key]);
  for (const [name,key] of [['navbar.templ','templ_sha256'],['navbar.go','model_sha256'],['defaults.json','defaults_sha256']])
    await input(path.join(probe,'delivery',name),artifact[key]);
  const fragments = new Map([['original',source]]);
  for (const name of ['reference','typed','minimal','long','multiple']) {
    const pin = compile.rendered_sha256?.[name+'.html']; assert.match(pin,/^[a-f0-9]{64}$/);
    fragments.set(name,await input(path.join(probe,name+'.html'),pin));
  }
  const assetsRoot = path.join(kit,'data/design-assets/reference-assets/elements-1.0.22-pilot');
  const runtime = await input(path.join(assetsRoot,'elements.js'),'120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013');
  assert.equal(sha(runtime),artifact.runtime.esm_sha256);
  const font = await input(path.join(assetsRoot,'inter.woff2'),'693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3');
  const tw = '/Users/geoff/Library/Caches/app-kit/tailwindcss/v4.3.0/tailwindcss-macos-arm64';
  const twStat = await fs.lstat(tw); assert(twStat.isFile() && twStat.nlink===1 && twStat.size<=96*2**20);
  const twBytes = await fs.readFile(tw); assert.equal(sha(twBytes),'56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e'); inputs.set(tw,sha(twBytes));
  const cssInput = Buffer.from('@import "tailwindcss" source(none);\n@source "../original.html";\n@source "../delivery/navbar.templ";\n@theme{--font-sans:InterVariable,ui-sans-serif,sans-serif}\n@font-face{font-family:InterVariable;font-weight:100 900;font-display:block;src:url("'+prefix+'/inter.woff2") format("woff2")}\n');
  const cssInputPath = await write('input.css',cssInput), cssPath = path.join(out,'source.css');
  const build = spawnSync(tw,['-i',cssInputPath,'-o',cssPath,'--minify'],{timeout:30000,maxBuffer:65536});
  assert.equal(build.status,0,'pinned CSS compilation failed'); await fs.chmod(cssPath,0o600);
  const css = await fs.readFile(cssPath); outputs.set('source.css',{sha256:sha(css),bytes:css.length});
  report.tailwind={version:'4.3.0',executable_sha256:sha(twBytes)};
  const logo = Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><rect x="2" y="2" width="28" height="28" rx="5" fill="oklch(0.58 0.20 280)"/><path d="M9 22V10h14v4H13v8z" fill="oklch(0.98 0.01 280)"/></svg>');
  const fixtureCSS = Buffer.from('html{font-family:InterVariable,system-ui,sans-serif;color-scheme:light}html.dark{color-scheme:dark}body{margin:0;background:Canvas;color:CanvasText}#outside{margin:24px}');
  const assets = new Map([['elements.js',runtime],['inter.woff2',font],['source.css',css],['fixture.css',fixtureCSS],['logo.svg',logo]]);
  report.assets = Object.fromEntries([...assets].map(([name,bytes])=>[name,{sha256:sha(bytes),bytes:bytes.length}]));
  const documents = new Map();
  for (const scheme of ['light','dark']) for (const [name,fragment] of fragments) {
    const bytes = Buffer.from('<!doctype html><html lang="en" class="'+(scheme==='dark'?'dark':'')+'"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Navbar screening</title><link rel="icon" href="'+prefix+'/logo.svg"><link rel="stylesheet" href="'+prefix+'/source.css"><link rel="stylesheet" href="'+prefix+'/fixture.css"><script type="module" src="'+prefix+'/elements.js"></script></head><body><main id="specimen">'+fragment+'</main><button id="outside" type="button">Outside navigation</button></body></html>');
    const key=name+'-'+scheme; documents.set(key,bytes); await write(key+'.html',bytes);
  }
  const destinations = new Map([['/dashboard','Synthetic dashboard'],['/notifications','Synthetic notifications'],['/profile','Synthetic profile'],['/settings','Synthetic settings']]);
  server=http.createServer((req,res)=>{
    const url = new URL(req.url,base || 'http://invalid.test');
    res.setHeader('Cache-Control','no-store'); res.setHeader('X-Content-Type-Options','nosniff'); res.setHeader('Referrer-Policy','same-origin');
    res.setHeader('Content-Security-Policy',"default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self'; connect-src 'none'; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'");
    if (!base || req.method!=='GET' || req.headers.host!==new URL(base).host || url.origin!==base || url.search) {res.writeHead(403);res.end();return;}
    let name=url.pathname.startsWith(prefix+'/')?url.pathname.slice(prefix.length+1):'';
    if (['/assets/brand-light.svg','/assets/brand-dark.svg','/assets/avatar.jpg'].includes(url.pathname)) name='logo.svg';
    const destination=destinations.get(url.pathname);
    const bytes=destination?Buffer.from('<!doctype html><html lang="en"><title>Fixture destination</title><h1>'+destination+'</h1><p>No account or external action occurred.</p></html>'):documents.get(name)||assets.get(name);
    report.requests.push({method:req.method,path:url.pathname,status:bytes?200:404});
    if(!bytes){res.writeHead(404);res.end();return;}
    const type=destination||documents.has(name)?'text/html; charset=utf-8':name.endsWith('.js')?'text/javascript':name.endsWith('.woff2')?'font/woff2':name.endsWith('.svg')?'image/svg+xml':'text/css';
    res.writeHead(200,{'Content-Type':type});res.end(bytes);
  });
  await bounded(new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);}));
  base='http://127.0.0.1:'+server.address().port;
  const pw='/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
  await input(pw,'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
  const {chromium}=await import(pathToFileURL(pw).href);
  browserServer=await chromium.launchServer({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true,timeout:20000});
  report.browser_pid=browserServer.process().pid;
  browser=await chromium.connect(browserServer.wsEndpoint()); report.browser=browser.version();
  async function ready(page) {
    await page.waitForFunction(()=>['el-disclosure','el-dropdown','el-menu'].every(tag=>customElements.get(tag))&&document.fonts.status==='loaded');
    let stable=0;
    for(let i=0;i<100;i++) {
      const settled=await page.evaluate(()=>document.getAnimations().every(a=>a.playState!=='running'&&!a.pending)&&[...document.querySelectorAll('el-menu:popover-open')].every(e=>getComputedStyle(e).opacity==='1'));
      stable=settled?stable+1:0; if(stable===3)return; await page.waitForTimeout(50);
    }
    throw Error('animation did not settle');
  }
  async function visit(page,name,scheme) { await page.goto(base+prefix+'/'+name+'-'+scheme,{waitUntil:'networkidle'});await ready(page); }
  async function tabTo(page,selector) {
    for(let i=0;i<30;i++){await page.keyboard.press('Tab');if(await page.locator(selector).evaluate(e=>e===document.activeElement))return true;}
    return false;
  }
  async function capture(page,name) {
    const bytes=await page.screenshot({animations:'disabled'});await write(name+'.png',bytes);
    report.captures.push({file:name+'.png',sha256:sha(bytes),bytes:bytes.length});return bytes;
  }
  async function responsiveState(page) {
    return page.locator('nav').first().evaluate(nav=>{
      const visible=e=>e.checkVisibility()&&e.getClientRects().length>0;
      const mobile=[...nav.querySelectorAll('el-disclosure a')],labels=new Set(mobile.map(a=>a.textContent.trim()));
      const desktop=[...nav.querySelectorAll('a')].filter(a=>!a.closest('el-disclosure,el-menu')&&labels.has(a.textContent.trim()));
      const trigger=nav.querySelector('button[command="--toggle"]');
      return {desktopTotal:desktop.length,mobileTotal:mobile.length,desktopVisible:desktop.filter(visible).length,
        mobileVisible:mobile.filter(visible).length,triggerVisible:!!trigger&&visible(trigger)};
    });
  }
  async function textGeometryFacts(page) {
    return page.locator('nav').evaluate(nav=>{
      const rect=r=>({left:r.left,right:r.right,top:r.top,bottom:r.bottom});
      const clippedSR=node=>{const s=getComputedStyle(node),r=node.getBoundingClientRect();return node.classList.contains('sr-only')&&s.position==='absolute'&&r.width<=1&&r.height<=1&&['hidden','clip'].includes(s.overflow)&&s.whiteSpace==='nowrap';};
      const entries=[...nav.querySelectorAll('a')].filter(e=>e.checkVisibility()).map(link=>{
        const fragments=[],ignored=[];const walker=document.createTreeWalker(link,NodeFilter.SHOW_TEXT);
        while(walker.nextNode()){
          const node=walker.currentNode;if(!node.textContent.trim())continue;
          let hidden=null;for(let p=node.parentElement;p&&p!==link;p=p.parentElement)if(p.classList.contains('sr-only')){hidden=p;break;}
          if(hidden&&clippedSR(hidden)){ignored.push(node.textContent.trim());continue;}
          const range=document.createRange();range.selectNodeContents(node);
          for(const value of range.getClientRects())if(value.width>0&&value.height>0)fragments.push(rect(value));
        }
        const popover=link.closest('el-menu:popover-open');return{label:link.innerText.trim(),box:rect(link.getBoundingClientRect()),fragments,ignored,popover:popover?rect(popover.getBoundingClientRect()):null};
      });
      return{entries,popovers:[...nav.querySelectorAll('el-menu:popover-open')].map(e=>rect(e.getBoundingClientRect()))};
    });
  }
  async function account(page,key,root='nav') {
    const trigger=root+' el-dropdown > button',menu=page.locator(root+' el-menu');
    check(key+': Tab reaches account',await tabTo(page,trigger));await page.keyboard.press('Enter');await ready(page);
    check(key+': account opens with keyboard focus',await menu.evaluate(e=>e.matches(':popover-open')&&e.contains(document.activeElement)));
    const before=await menu.locator('a').evaluateAll(all=>all.indexOf(document.activeElement));await page.keyboard.press('ArrowDown');
    const after=await menu.locator('a').evaluateAll(all=>all.indexOf(document.activeElement));
    check(key+': ArrowDown changes selected item',before>=0&&after>=0&&before!==after,{before,after});
    await page.keyboard.press('Escape');await ready(page);
    check(key+': Escape closes and restores trigger',!(await menu.evaluate(e=>e.matches(':popover-open')))&&await page.locator(trigger).evaluate(e=>e===document.activeElement));
  }
  for(const width of [390,1440]) for(const scheme of ['light','dark']) {
    const context=await browser.newContext({viewport:{width,height:1000},colorScheme:scheme,reducedMotion:'reduce',serviceWorkers:'block'});
    await context.route('**/*',async route=>{if(new URL(route.request().url()).origin!==base){report.blocked_external.push({sha256:sha(Buffer.from(route.request().url())),type:route.request().resourceType()});await route.abort('blockedbyclient');}else await route.continue();});
    await context.addInitScript(()=>{window.__navbarCSP=[];document.addEventListener('securitypolicyviolation',e=>window.__navbarCSP.push({directive:e.effectiveDirective,blocked:e.blockedURI}));});
    const page=await context.newPage();page.setDefaultTimeout(5000);
    page.on('pageerror',e=>report.errors.push('pageerror: '+e.message));
    page.on('response',r=>{if(new URL(r.url()).origin===base&&r.status()>=400)report.errors.push('local HTTP '+r.status());});
    const shots={};
    for(const name of ['original','reference']) {
      const key=`${scheme}-${width}-${name}`,blockedAt=report.blocked_external.length;await visit(page,name,scheme);
      const expectedImages=await page.locator('nav img').evaluateAll(all=>all.map(img=>img.src));
      const sourceCSP=await page.evaluate(()=>window.__navbarCSP);
      check(key+': exact three source images refused by CSP',classifySourceRefusals(sourceCSP,expectedImages,report.blocked_external.slice(blockedAt)),{expected:expectedImages.map(url=>sha(Buffer.from(url))),events:sourceCSP.map(e=>({directive:e.directive,blocked_sha256:sha(Buffer.from(e.blocked))}))});
      const closed=await responsiveState(page);check(key+': closed responsive groups are exclusive',responsiveOK(closed,width,false),closed);
      shots[name]=await capture(page,key);
      if(width===390){check(key+': Tab reaches disclosure',await tabTo(page,'button[command="--toggle"]'));await page.keyboard.press('Enter');await ready(page);const opened=await responsiveState(page);check(key+': disclosure exposes only four mobile links',responsiveOK(opened,width,true)&&opened.mobileVisible===4,opened);shots[name+'-open']=await capture(page,key+'-open');}
      await account(page,key);check(key+': source placeholders remain unactivated',await page.locator('nav a').evaluateAll(all=>all.length===11&&all.every(a=>a.getAttribute('href')==='#')));
      for(const e of sourceCSP)report.csp_blocks.push({directive:e.directive,blocked_sha256:sha(Buffer.from(e.blocked))});
    }
    check(`${scheme}-${width}: original/reference pixels identical`,shots.original.equals(shots.reference));
    if(width===390)check(`${scheme}-${width}: original/reference open pixels identical`,shots['original-open'].equals(shots['reference-open']));
    const key=`${scheme}-${width}-typed`;await visit(page,'typed',scheme);
    check(key+': one named navigation landmark',await page.getByRole('navigation',{name:'Primary navigation'}).count()===1);
    check(key+': all typed image bytes loaded',await page.locator('nav img').evaluateAll(all=>all.length===3&&all.every(e=>e.complete&&e.naturalWidth>0)));
    check(key+': notification has exact accessible name',await page.getByRole('link',{name:'Notifications',exact:true}).count()===1);
    check(key+': current route has two agreeing renderings',await page.locator('nav a[aria-current="page"]').evaluateAll(all=>all.length===2&&all.every(e=>e.textContent==='Projects'&&e.getAttribute('href')==='https://example.test/projects')));
    check(key+': no page overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),{width});
    const typedClosed=await responsiveState(page);check(key+': closed responsive groups are exclusive',responsiveOK(typedClosed,width,false),typedClosed);
    if(width===390&&scheme==='light'){
      const changed=await page.locator('nav').evaluate(nav=>{
        const visible=e=>e.checkVisibility()&&e.getClientRects().length>0;
        const mobile=[...nav.querySelectorAll('el-disclosure a')],labels=new Set(mobile.map(a=>a.textContent.trim()));
        const desktop=[...nav.querySelectorAll('a')].filter(a=>!a.closest('el-disclosure,el-menu')&&labels.has(a.textContent.trim()));
        const hidden=[...new Set(desktop.map(a=>{for(let p=a.parentElement;p&&p!==nav;p=p.parentElement)if(getComputedStyle(p).display==='none')return p;return null;}).filter(Boolean))];
        for(const node of hidden){node.dataset.navbarCanaryStyle=node.hasAttribute('style')?node.getAttribute('style'):'__missing__';node.style.setProperty('display','flex','important');}
        return hidden.length;
      });
      const broken=await responsiveState(page);
      await page.locator('nav').evaluate(nav=>{for(const node of nav.querySelectorAll('[data-navbar-canary-style]')){const prior=node.dataset.navbarCanaryStyle;delete node.dataset.navbarCanaryStyle;if(prior==='__missing__')node.removeAttribute('style');else node.setAttribute('style',prior);}});
      const restored=await responsiveState(page);
      check('browser canary: lost responsive hiding is refused and restoration passes',changed>0&&responsiveOK(typedClosed,390,false)&&!responsiveOK(broken,390,false)&&responsiveOK(restored,390,false),{changed,broken,restored});
    }
    if(width===390){check(key+': keyboard finds disclosure',await tabTo(page,'button[command="--toggle"]'));await page.keyboard.press('Enter');await ready(page);const typedOpen=await responsiveState(page);check(key+': expanded state and links agree',await page.locator('button[command="--toggle"]').getAttribute('aria-expanded')==='true'&&responsiveOK(typedOpen,width,true)&&typedOpen.mobileVisible===2,typedOpen);}
    await capture(page,key);await account(page,key);
    for(const target of ['/dashboard','/notifications','/profile','/settings']) {
      await visit(page,'typed',scheme);
      if(target==='/dashboard'&&width===390){await page.locator('button[command="--toggle"]').click();await ready(page);}
      if(target==='/profile'||target==='/settings'){await page.locator('el-dropdown > button').click();await ready(page);}
      const selector=target==='/dashboard'&&width===390?'el-disclosure a[href="/dashboard"]':'nav a[href="'+target+'"]:visible';
      await Promise.all([page.waitForURL(base+target,{waitUntil:'networkidle'}),page.locator(selector).click()]);
      check(key+': native GET '+target,await page.locator('h1').textContent()===destinations.get(target));
    }
    await visit(page,'minimal',scheme);check(key+': omitted account and notifications',await page.locator('el-dropdown,el-menu,a[href="/notifications"],a[aria-current]').count()===0);
    await visit(page,'multiple',scheme);
    check(key+': independent disclosure IDs',await page.locator('el-disclosure').evaluateAll(all=>all.length===2&&new Set(all.map(e=>e.id)).size===2));
    if(width===390){await page.locator('nav').nth(1).locator('button[command="--toggle"]').click();await ready(page);check(key+': second disclosure does not open first',!(await page.locator('nav').nth(0).locator('el-disclosure').isVisible())&&await page.locator('nav').nth(1).locator('el-disclosure').isVisible());}
    await account(page,key+' second','nav:nth-of-type(2)');
    for(let cycle=0;cycle<3;cycle++) {
      await page.evaluate(()=>{const n=document.querySelector('nav');const text=n.outerHTML;n.remove();document.querySelector('#specimen').insertAdjacentHTML('afterbegin',text);});await ready(page);
      await account(page,key+' remount '+cycle,'nav:nth-of-type(1)');
      check(key+': remount keeps unique IDs '+cycle,await page.locator('[id]').evaluateAll(all=>all.length===new Set(all.map(e=>e.id)).size));
    }
    await visit(page,'long',scheme);
    if(width===390){await page.locator('button[command="--toggle"]').click();await ready(page);}
    await page.locator('el-dropdown > button').click();await ready(page);
    const facts=await textGeometryFacts(page);
    const geometry=assessTextGeometry(facts,{left:0,right:width,top:0,bottom:1000});
    const ignored=geometry.entries.flatMap(e=>e.ignored);
    check(key+': only exact clipped notification name is excluded from visible text geometry',ignored.length===1&&ignored[0]==='Notifications',{ignored});
    check(key+': long visible text and popovers stay within controls and viewport',geometry.entries.length>0&&geometry.entries.every(e=>e.fits)&&geometry.popoversFit,{geometry});
    if(width===390&&scheme==='light'){
      const changed=await page.locator('nav').evaluate(nav=>{const link=[...nav.querySelectorAll('el-disclosure a')].find(e=>e.checkVisibility());if(!link)return false;link.dataset.navbarCanaryStyle=link.hasAttribute('style')?link.getAttribute('style'):'__missing__';for(const [name,value] of [['display','block'],['width','8px'],['max-width','8px'],['flex','none'],['white-space','nowrap'],['overflow','visible']])link.style.setProperty(name,value,'important');return true;});
      const brokenFacts=await textGeometryFacts(page),brokenGeometry=assessTextGeometry(brokenFacts,{left:0,right:width,top:0,bottom:1000});
      await page.locator('nav').evaluate(nav=>{const link=nav.querySelector('[data-navbar-canary-style]');if(!link)return;const prior=link.dataset.navbarCanaryStyle;delete link.dataset.navbarCanaryStyle;if(prior==='__missing__')link.removeAttribute('style');else link.setAttribute('style',prior);});
      const restoredFacts=await textGeometryFacts(page),restoredGeometry=assessTextGeometry(restoredFacts,{left:0,right:width,top:0,bottom:1000});
      check('browser canary: clipped visible text is refused and restoration passes',changed&&brokenGeometry.entries.some(e=>!e.fits)&&restoredGeometry.entries.every(e=>e.fits)&&restoredGeometry.popoversFit,{broken:brokenGeometry,restored:restoredGeometry});
    }
    check(key+': long page has no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
    await capture(page,key+'-long');
    check(key+': typed CSP has no violations',await page.evaluate(()=>window.__navbarCSP.length===0));
    await context.close();
  }
  check('no unexpected external request reached routing',report.blocked_external.length===0,{count:report.blocked_external.length});
  for(const [name,expected] of inputs)assert.equal(sha(await fs.readFile(name)),expected,'input changed during run');
  report.inputs_unchanged=true;
} catch(error) {report.errors.push(String(error));process.exitCode=1;}
finally {
  try {
    if(browser)await bounded(browser.close());if(browserServer)await bounded(browserServer.close());
    if(server?.listening){server.closeAllConnections();await bounded(new Promise((resolve,reject)=>server.close(e=>e?reject(e):resolve())));}
    if(report.browser_pid){let gone=false;for(let i=0;i<30;i++){try{process.kill(report.browser_pid,0);}catch(e){if(e.code==='ESRCH'){gone=true;break;}throw e;}await new Promise(r=>setTimeout(r,100));}assert(gone,'owned browser PID remains');report.browser_pid_absent=true;}
    if(base)await bounded(new Promise((resolve,reject)=>{const socket=net.createConnection({host:'127.0.0.1',port:Number(new URL(base).port)});socket.once('connect',()=>{socket.destroy();reject(Error('owned listener remains'));});socket.once('error',e=>{socket.destroy();e.code==='ECONNREFUSED'?resolve():reject(e);});}));
    report.cleanup='ok';
  }catch(error){report.errors.push('cleanup: '+String(error));browserServer?.process()?.kill('SIGKILL');server?.closeAllConnections();}
  clearTimeout(watchdog);
  report.inputs=Object.fromEntries(inputs);report.outputs=Object.fromEntries(outputs);
  report.passed=report.checks.filter(c=>c.ok).length;report.failed=report.checks.filter(c=>!c.ok).length;
  if(report.failed||report.errors.length)process.exitCode=1;
  const bytes=Buffer.from(JSON.stringify(report,null,2)+'\n');await fs.writeFile(path.join(out,'receipt.json'),bytes,{mode:0o600,flag:'wx'});
  process.stdout.write(JSON.stringify({out,passed:report.passed,failed:report.failed,errors:report.errors,receipt_sha256:sha(bytes)})+'\n');
}
