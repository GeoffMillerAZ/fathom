// Explicit P09 repair proof. The frozen base diagnostic and its measurement functions remain unchanged.
// No CSS zoom, signed-in profile, downloads, global settings or consumer mutations.
import assert from 'node:assert/strict';
import {createHash, randomBytes} from 'node:crypto';
import {spawnSync} from 'node:child_process';
import {createRequire} from 'node:module';
import fs from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {navbarZoomCaptureClip} from './zoom-contract.mjs';

const [baseArg, repairArg, repairSHA] = process.argv.slice(2);
assert(process.argv.length===5&&path.isAbsolute(baseArg)&&path.isAbsolute(repairArg)&&/^[a-f0-9]{64}$/.test(repairSHA??''),
  'usage: node repair.mjs PRIVATE_BASE_PROBE PRIVATE_REPAIR_PROBE REPAIR_COMPILE_SHA256');
const probe=await fs.realpath(baseArg),repairProbe=await fs.realpath(repairArg);
assert(repairProbe.startsWith(await fs.realpath(os.tmpdir())+path.sep));
const priorPath=path.join(probe,'browser-Tn5991/receipt.json');
const priorSHA='2dfad48ffd46e6187ab4809b3a7dd65675a1af48fdbf1702c1e01ca1102dbe48';
const baselineSHA='d537109542046601845d0e53f45dbf6dea92efc74abe0138490cd1da731e412f';
const pins = {
  artifact: '5559fb79f3d06d312df0a6a7bafe216815b5f34499b28a804862291d3f8cb642',
  compile: 'd2d19352a2ef101db2a8351c1015afaedbb4ee4b0daf438669b5d45585072062',
  normalHarness: '7388a1e05b53a4eb907081ba449af0aa043167ac88718256d50d60d977348c3e',
};
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const inputs = new Map(), outputs = new Map();
async function input(file, expected) {
  const name = file instanceof URL ? fileURLToPath(file) : file, stat = await fs.lstat(name);
  assert(stat.isFile() && stat.nlink === 1 && stat.size > 0 && stat.size <= 8 * 2**20, 'invalid bounded input');
  const bytes = await fs.readFile(name), hash = sha(bytes);
  if (expected) assert.equal(hash, expected, 'input identity differs: ' + path.basename(name));
  inputs.set(name, hash); return bytes;
}
const out = await fs.mkdtemp(path.join(repairProbe, 'browser-')); await fs.chmod(out, 0o700);
async function write(name, bytes) {
  await fs.writeFile(path.join(out, name), bytes, {mode:0o600, flag:'wx'});
  outputs.set(name, {sha256:sha(bytes), bytes:bytes.length});
}
const report = {schema:1, kind:'private-p09-navbar-repair-browser', verification:'not-certified',
  normal_receipt_sha256:priorSHA, base_artifact_sha256:pins.artifact, base_compile_receipt_sha256:pins.compile, repair_compile_receipt_sha256:repairSHA, baseline_failure_sha256:baselineSHA,
  checks:[], captures:[], requests:[], expected_runtime_failures:[], errors:[], hosts:[], cleanup:'pending',
  focus_policy:{minimum_css_px:2, minimum_contrast:3, minimum_target_css_px:24,
    accepted:'focus-induced opaque solid outline or zero-offset zero-blur box-shadow ring, inset or outset; unclipped; composited backdrop',
    note:'Conservative screen, not WCAG certification. Focused background and thin/auto native outlines are recorded separately.'},
  limits:['Installed headless Chrome, actual 200% Appearance setting, 1440/780 by 1000 outer windows, light/dark, reduced motion.',
    'Protocol keyboard input, not physical input, touch, screen-reader, cross-browser or performance evidence.',
    'Synthetic typed images and native GET destinations; no original-image fidelity, account backend or real consumer proof.',
    'Runtime-loss lane returns an explicit HTTP 503 for the module; it does not model every partial upgrade or late runtime failure.',
    'Focus facts and captures require review. Conservative focus/geometry failures remain failures, not certified accessibility violations.']};
const check = (name, ok, details={}) => report.checks.push({name, ok:Boolean(ok), ...details});
let context, server, base, profile, pid, physicalWidth;
function alive() {
  if (!pid || !profile) return false;
  const p = spawnSync('/bin/ps', ['-p',String(pid),'-o','command='], {timeout:2000,maxBuffer:16384});
  if (p.status === 1) return false;
  assert.equal(p.status,0); assert(p.stdout.toString().includes('--user-data-dir=' + profile), 'owned browser identity differs'); return true;
}
async function bounded(promise) {
  let timer;
  try { return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('operation deadline')),10000);})]); }
  finally { clearTimeout(timer); }
}
const watchdog = setTimeout(()=>{report.errors.push('six-minute deadline'); process.exitCode=1;
  try { if (alive()) process.kill(pid,'SIGKILL'); } catch { report.errors.push('owned process stop failed'); }
  server?.closeAllConnections();},360000);

// Raw DOM facts are retained. No DOM insertion, geometry tolerance or component repair.
async function focusFacts(page) {
  return page.evaluate(()=>{
    const e=document.activeElement, s=getComputedStyle(e), r=e.getBoundingClientRect();
    const canvas=document.createElement('canvas'); canvas.width=canvas.height=1;
    const ctx=canvas.getContext('2d'), rgba=color=>{ctx.clearRect(0,0,1,1);ctx.fillStyle=color;ctx.fillRect(0,0,1,1);return [...ctx.getImageData(0,0,1,1).data];};
    const backdrop=start=>{
      const layers=[];let unsupported=false;
      for(let n=start;n;n=n.parentElement){const st=getComputedStyle(n);layers.push(rgba(st.backgroundColor));if(st.backgroundImage!=='none')unsupported=true;}
      let composed=[0,0,0,0];
      for(const src of layers.reverse()){
        const a=src[3]/255,b=composed[3]/255,out=a+b*(1-a);
        composed=out===0?[0,0,0,0]:[...src.slice(0,3).map((v,i)=>(v*a+composed[i]*b*(1-a))/out),out*255];
      }
      return {color:composed[3]===255?composed:null,unsupported,layers};
    };
    const outsideFacts=backdrop(e.parentElement),insideFacts=backdrop(e),outside=outsideFacts.color,inside=insideFacts.color;
    const baseline=window.__p09Unfocused?.get(e);
    const lum=a=>a.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((n,v,i)=>n+v*[.2126,.7152,.0722][i],0);
    const contrast=(a,b)=>b?(Math.max(lum(a),lum(b))+.05)/(Math.min(lum(a),lum(b))+.05):0;
    const candidates=[];
    if(s.outlineStyle!=='none' && parseFloat(s.outlineWidth)>0){
      const width=parseFloat(s.outlineWidth),offset=parseFloat(s.outlineOffset),color=rgba(s.outlineColor);
      candidates.push({kind:'outline',style:s.outlineStyle,width,offset,color,background:offset<0?inside:outside,
        focusInduced:!!baseline&&!baseline.focusVisible&&s.outline!==baseline.outline,
        contrast:contrast(color,offset<0?inside:outside),extent:Math.max(0,offset+width)});
    }
    for(const shadow of s.boxShadow.split(/,(?![^()]*\))/)){
      const inset=/\binset\b/.test(shadow),text=shadow.replace(/\binset\b/g,'').trim();
      const m=text.match(/^(.+?)\s+(-?[\d.]+)px\s+(-?[\d.]+)px\s+(-?[\d.]+)px\s+(-?[\d.]+)px$/);
      if(!m)continue;
      const [,css,x,y,blur,spread]=m,color=rgba(css);
      if(Number(x)!==0||Number(y)!==0||Number(blur)!==0||Number(spread)<=0)continue;
      candidates.push({kind:'box-shadow',style:'solid',inset,width:Number(spread),color,background:inset?inside:outside,
        focusInduced:!!baseline&&!baseline.focusVisible&&!baseline.boxShadow.split(/,(?![^()]*\))/).map(s=>s.trim()).includes(shadow.trim()),
        contrast:contrast(color,inset?inside:outside),extent:inset?0:Number(spread)});
    }
    for(const candidate of candidates){
      const extent=candidate.extent,reasons=[];
      if(r.left-extent<0||r.right+extent>innerWidth)reasons.push('viewport:x');
      if(r.top-extent<0||r.bottom+extent>innerHeight)reasons.push('viewport:y');
      const topLayer=e.closest(':popover-open');
      for(let n=e.parentElement;n;n=n.parentElement){
        if(n===document.body||n===document.documentElement)continue;
        const st=getComputedStyle(n),b=n.getBoundingClientRect(),label=n.tagName.toLowerCase()+(n.id?'#'+n.id:'');
        if(/hidden|clip|scroll|auto/.test(st.overflowX)&&(r.left-extent<b.left||r.right+extent>b.right))reasons.push(label+':x');
        if(/hidden|clip|scroll|auto/.test(st.overflowY)&&(r.top-extent<b.top||r.bottom+extent>b.bottom))reasons.push(label+':y');
        if(n===topLayer)break;
      }
      candidate.clipReasons=reasons;
      candidate.qualifies=candidate.focusInduced&&!outsideFacts.unsupported&&!insideFacts.unsupported&&candidate.style==='solid'&&candidate.width>=2&&candidate.color[3]===255&&candidate.contrast>=3&&reasons.length===0;
    }
    return {tag:e.tagName,href:e.getAttribute('href'),command:e.getAttribute('command'),focusVisible:e.matches(':focus-visible'),
      rect:{left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:r.width,height:r.height},
      outline:s.outline,outlineOffset:s.outlineOffset,boxShadow:s.boxShadow,background:s.backgroundColor,
      baseline,backdrops:{inside:insideFacts,outside:outsideFacts},backgroundContrast:contrast(inside??[0,0,0,0],outside),candidates,
      passes:e.matches(':focus-visible')&&r.width>=24&&r.height>=24&&candidates.some(c=>c.qualifies)};
  });
}
async function geometry(page) {
  return page.locator('nav').evaluate(nav=>{
    const rect=r=>({left:r.left,right:r.right,top:r.top,bottom:r.bottom});
    const contains=(o,i)=>i.left>=o.left-1&&i.right<=o.right+1&&i.top>=o.top-1&&i.bottom<=o.bottom+1;
    const viewport={left:0,right:innerWidth,top:0,bottom:innerHeight},entries=[];
    for(const link of [...nav.querySelectorAll('a')].filter(e=>e.checkVisibility())){
      const box=rect(link.getBoundingClientRect()),fragments=[],ignored=[],clips=[];
      const walker=document.createTreeWalker(link,NodeFilter.SHOW_TEXT);
      while(walker.nextNode()){
        const node=walker.currentNode;if(!node.textContent.trim())continue;
        let hidden=null;for(let n=node.parentElement;n&&n!==link;n=n.parentElement)if(n.classList.contains('sr-only')){hidden=n;break;}
        if(hidden){const s=getComputedStyle(hidden),b=hidden.getBoundingClientRect();if(s.position==='absolute'&&b.width<=1&&b.height<=1&&/hidden|clip/.test(s.overflow)&&s.whiteSpace==='nowrap'){ignored.push(node.textContent.trim());continue;}}
        const range=document.createRange();range.selectNodeContents(node);
        for(const r of range.getClientRects())if(r.width>0&&r.height>0)fragments.push(rect(r));
      }
      const topLayer=link.closest(':popover-open');
      for(let n=link;n;n=n.parentElement){const s=getComputedStyle(n);if(/hidden|clip|scroll|auto/.test(s.overflowX)||/hidden|clip|scroll|auto/.test(s.overflowY))clips.push({box:rect(n.getBoundingClientRect()),x:/hidden|clip|scroll|auto/.test(s.overflowX),y:/hidden|clip|scroll|auto/.test(s.overflowY)});if(n===topLayer)break;}
      const fits=(fragments.length>0||ignored.length>0)&&fragments.every(f=>contains(box,f)&&contains(viewport,f)&&clips.every(c=>(!c.x||(f.left>=c.box.left-1&&f.right<=c.box.right+1))&&(!c.y||(f.top>=c.box.top-1&&f.bottom<=c.box.bottom+1))));
      entries.push({href:link.getAttribute('href'),box,fragments,ignored,clips,fits});
    }
    return {viewport,scrollWidth:document.documentElement.scrollWidth,entries,
      passes:document.documentElement.scrollWidth<=innerWidth&&entries.every(e=>e.fits)};
  });
}

try {
  report.harness_sha256=sha(await input(new URL(import.meta.url)));
  const frozen=(await input(new URL('./zoom.mjs',import.meta.url),'8b840c6c978d6a2ebc1820c76d9decd4eb063948b1ed12b39a30f25d28b1c2f3')).toString();
  assert(frozen.includes(focusFacts.toString())&&frozen.includes(geometry.toString()),'frozen measurement function changed');
  const red=JSON.parse(await input(path.join(probe,'zoom-t4U6Hs/failure.json'),baselineSHA));
  assert.equal(red.failed,38);assert.equal(red.passed,240);assert.deepEqual(red.errors,[]);
  const repairCompile=JSON.parse(await input(path.join(repairProbe,'compile-receipt.json'),repairSHA));
  assert.equal(repairCompile.kind,'private-p09-repair-compiler');
  const repair=JSON.parse(await input(path.join(repairProbe,'artifact.json'),repairCompile.artifact_sha256));
  assert.equal(repair.adapter,'p09-navbar-repair-v1');assert.equal(repair.verification,'not-verified');
  report.repair_artifact_sha256=repairCompile.artifact_sha256;
  for(const [name,key] of [['navbar_repair.css','css_sha256'],['navbar_routes.go','routes_go_sha256'],['navbar_routes.templ','routes_templ_sha256']])await input(path.join(repairProbe,name),repair[key]);
  await input(new URL('./zoom-contract.mjs',import.meta.url));
  const prior=JSON.parse(await input(priorPath,priorSHA));
  assert.equal(prior.kind,'private-p09-navbar-browser-screening');assert.equal(prior.harness_sha256,pins.normalHarness);
  assert.equal(prior.verification,'not-certified');assert.equal(prior.passed,238);assert.equal(prior.failed,0);
  assert.deepEqual(prior.errors,[]);assert.equal(prior.cleanup,'ok');assert(prior.browser_pid_absent);
  const compile=JSON.parse(await input(path.join(probe,'compile-receipt.json'),pins.compile));
  const artifact=JSON.parse(await input(path.join(probe,'delivery/artifact.json'),pins.artifact));
  assert.equal(compile.artifact_sha256,pins.artifact);assert.equal(artifact.adapter,'p09-responsive-navbar-v1');assert.deepEqual(repair.base,artifact);
  await input(path.join(probe,'original.html'),artifact.source.sha256);
  for(const [name,key] of [['navbar.templ','templ_sha256'],['navbar.go','model_sha256'],['defaults.json','defaults_sha256']])await input(path.join(probe,'delivery',name),artifact[key]);
  const oldRoot=path.dirname(priorPath),assetPath=prior.requests.find(r=>r.path.endsWith('/source.css'))?.path;
  assert(assetPath);const assetPrefix=path.posix.dirname(assetPath),docPrefix='/zoom-'+randomBytes(18).toString('hex');
  const css=await input(path.join(oldRoot,'source.css'),prior.outputs['source.css'].sha256);
  const kit=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../../..');
  const assetsRoot=path.join(kit,'data/design-assets/reference-assets/elements-1.0.22-pilot');
  const runtime=await input(path.join(assetsRoot,'elements.js'),artifact.runtime.esm_sha256);
  const repairCSS=await input(path.join(kit,'internal/designadapt/navbar_repair.css'),repair.css_sha256);
  const font=await input(path.join(assetsRoot,'inter.woff2'),prior.assets['inter.woff2'].sha256);
  const logo=Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><rect x="2" y="2" width="28" height="28" rx="5" fill="oklch(0.58 0.20 280)"/><path d="M9 22V10h14v4H13v8z" fill="oklch(0.98 0.01 280)"/></svg>');
  const fixtureCSS=Buffer.from('html{font-family:InterVariable,system-ui,sans-serif;color-scheme:light}html.dark{color-scheme:dark}body{margin:0;background:Canvas;color:CanvasText}#outside{margin:24px}');
  assert.equal(sha(logo),prior.assets['logo.svg'].sha256);assert.equal(sha(fixtureCSS),prior.assets['fixture.css'].sha256);
  const resources=new Map();
  for(const [name,type,body] of [['elements.js','text/javascript',runtime],['inter.woff2','font/woff2',font],['source.css','text/css',css],['fixture.css','text/css',fixtureCSS],['logo.svg','image/svg+xml',logo]])resources.set(assetPrefix+'/'+name,{type,body});
  resources.set(assetPrefix+'/navbar_repair.css',{type:'text/css',body:repairCSS});
  for(const name of ['/assets/brand-light.svg','/assets/brand-dark.svg','/assets/avatar.jpg'])resources.set(name,{type:'image/svg+xml',body:logo});
  const unavailable=assetPrefix+'/elements-unavailable.js';
  resources.set(unavailable,{type:'text/javascript',body:Buffer.from('// Explicit synthetic runtime outage.\n'),status:503});
  for(const kind of ['typed','long'])for(const scheme of ['light','dark'])for(const mode of ['ready','unavailable']){
    const fragment=await input(path.join(repairProbe,'repair-'+kind+'.html'),repairCompile.rendered_sha256['repair-'+kind+'.html']);
    const body=Buffer.from('<!doctype html><html lang="en" class="'+(scheme==='dark'?'dark':'')+'"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Navbar zoom diagnostic</title><link rel="icon" href="'+assetPrefix+'/logo.svg"><link rel="stylesheet" href="'+assetPrefix+'/source.css"><link rel="stylesheet" href="'+assetPrefix+'/fixture.css"><link rel="stylesheet" href="'+assetPrefix+'/navbar_repair.css"><script type="module" src="'+(mode==='ready'?assetPrefix+'/elements.js':unavailable)+'"></script></head><body><main id="specimen">'+fragment+'</main><button id="outside" type="button">Outside navigation</button></body></html>');
    resources.set(docPrefix+'/'+kind+'-'+scheme+'-'+mode,{type:'text/html; charset=utf-8',body});
  }
  const destinations=new Map([['/dashboard','Synthetic dashboard'],['/notifications','Synthetic notifications'],['/profile','Synthetic profile'],['/settings','Synthetic settings']]);
  for(const [url,title] of destinations)resources.set(url,{type:'text/html; charset=utf-8',body:Buffer.from('<!doctype html><html lang="en"><title>Fixture destination</title><h1>'+title+'</h1><p>No account action occurred.</p></html>')});
  report.resources=Object.fromEntries([...resources].map(([name,r])=>[name,{sha256:sha(r.body),bytes:r.body.length,status:r.status??200}]));
  server=http.createServer((req,res)=>{
    const url=new URL(req.url,base||'http://invalid.test');
    if(!base||req.method!=='GET'||req.headers.host!==new URL(base).host||url.origin!==base||url.search){res.writeHead(403).end();return;}
    const r=resources.get(url.pathname);if(!r){report.errors.push('unknown route');res.writeHead(404).end();return;}
    report.requests.push({method:req.method,path:url.pathname,status:r.status??200,sha256:sha(r.body)});
    res.writeHead(r.status??200,{'Content-Type':r.type,'Cache-Control':'no-store','X-Content-Type-Options':'nosniff',
      'Content-Security-Policy':"default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self'; connect-src 'none'; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'"});res.end(r.body);
  });
  await bounded(new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);}));base='http://127.0.0.1:'+server.address().port;
  const pw='/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';await input(pw,'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
  const {chromium}=await import(pathToFileURL(pw));
  const {PNG}=createRequire(import.meta.url)('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright-core/lib/utilsBundle.js');
  async function ready(page,mode='ready'){
    await page.waitForFunction(()=>document.fonts.status==='loaded');
    if(mode!=='ready')return;
    await page.waitForFunction(()=>['el-disclosure','el-dropdown','el-menu'].every(tag=>customElements.get(tag)));
    let stable=0;for(let i=0;i<100;i++){const settled=await page.evaluate(()=>document.getAnimations().every(a=>a.playState!=='running'&&!a.pending)&&[...document.querySelectorAll('el-menu:popover-open')].every(e=>getComputedStyle(e).opacity==='1'));stable=settled?stable+1:0;if(stable===3)return;await page.waitForTimeout(50);}throw Error('runtime did not settle');
  }
  async function tabTo(page,selector){for(let i=0;i<24;i++){await page.keyboard.press('Tab');if(await page.locator(selector).evaluate(e=>e===document.activeElement))return true;}return false;}
  async function screenFocus(page,key){const facts=await focusFacts(page);check(key+': focus screening',facts.passes,facts);}
  async function capture(page,name,full=false){
    const session=await context.newCDPSession(page);
    try {const options={format:'png',fromSurface:true,captureBeyondViewport:full};if(full)options.clip=navbarZoomCaptureClip(await session.send('Page.getLayoutMetrics'),physicalWidth);
      const {data}=await session.send('Page.captureScreenshot',options),bytes=Buffer.from(data,'base64'),png=PNG.sync.read(bytes);
      const colors=new Set();let opaque=0,dark=0,light=0;
      for(let i=0;i<png.data.length;i+=4){const [r,g,b,a]=png.data.subarray(i,i+4);if(colors.size<64)colors.add(png.data.readUInt32BE(i));if(a===255)opaque++;if(Math.max(r,g,b)<100)dark++;if(Math.min(r,g,b)>220)light++;}
      const pixels=png.width*png.height;
      check(name+': capture dimensions/content',png.width===physicalWidth&&png.height>0&&png.height<=20000&&(full||png.height===1000)&&colors.size>=32&&opaque===pixels&&dark/pixels>.001&&light/pixels>.001,{width:png.width,height:png.height,colors:colors.size,opaque,dark,light});
      await write(name+'.png',bytes);report.captures.push({file:name+'.png',sha256:sha(bytes),bytes:bytes.length});
    } finally {await session.detach();}
  }
  for(const scheme of ['light','dark'])for(const width of [1440,780]){
    physicalWidth=width;const mobile=width===780,lane=scheme+'-'+width;
    profile=await fs.mkdtemp(path.join(out,'profile-'));await fs.chmod(profile,0o700);
    context=await chromium.launchPersistentContext(profile,{executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true,timeout:20000,viewport:{width,height:1000},colorScheme:scheme,reducedMotion:'reduce',serviceWorkers:'block',args:['--enable-automation','--disable-background-networking','--no-proxy-server','--no-first-run','--no-default-browser-check']});
    const host=await context.browser().newBrowserCDPSession();
    try {const args=await host.send('Browser.getBrowserCommandLine');assert(args.arguments.includes('--user-data-dir='+profile));const processes=(await host.send('SystemInfo.getProcessInfo')).processInfo.filter(p=>p.type==='browser');assert.equal(processes.length,1);pid=processes[0].id;assert(alive());report.hosts.push({pid,profile});}finally{await host.detach();}
    report.browser=context.browser().version();
    const page=context.pages()[0];const qualityPage={locator:selector=>page.locator(selector==='nav'?'[data-appkit-navbar-repair] > nav':selector)};page.setDefaultTimeout(6000);page.setDefaultNavigationTimeout(12000);
    await page.goto('chrome://settings/appearance');await page.locator('#zoomLevel').selectOption('2');check(lane+': actual Appearance setting',await page.locator('#zoomLevel').inputValue()==='2');await page.goto('about:blank');
    context.on('serviceworker',()=>report.errors.push('unexpected service worker'));await context.routeWebSocket('**/*',socket=>{report.errors.push('unexpected websocket');socket.close();});
    await context.route('**/*',route=>{const r=route.request(),url=new URL(r.url());if(url.origin!==base||r.method()!=='GET'||url.search||!resources.has(url.pathname)){report.errors.push('unexpected routed request');return route.abort();}return route.continue();});
    page.on('pageerror',e=>report.errors.push('pageerror: '+e.message));
    page.on('response',r=>{if(r.status()>=400){if(new URL(r.url()).pathname===unavailable&&r.status()===503)report.expected_runtime_failures.push({event:'response',status:503});else report.errors.push('unexpected HTTP '+r.status());}});
    page.on('requestfailed',r=>{if(new URL(r.url()).pathname===unavailable)report.expected_runtime_failures.push({event:'requestfailed',reason:r.failure()?.errorText});else report.errors.push('request failed: '+new URL(r.url()).pathname);});
    async function visit(kind,mode='ready'){
      await page.goto(base+docPrefix+'/'+kind+'-'+scheme+'-'+mode,{waitUntil:'networkidle'});await ready(page,mode);
      await page.evaluate(()=>{window.__p09Unfocused=new Map([...document.querySelectorAll('nav a,nav button,details[data-appkit-navbar-routes] > summary')].map(e=>{const s=getComputedStyle(e);return[e,{outline:s.outline,boxShadow:s.boxShadow,background:s.backgroundColor,focusVisible:e.matches(':focus-visible')}];}));});
      const z=await page.evaluate(()=>({inner:innerWidth,outer:outerWidth,height:innerHeight,dpr:devicePixelRatio,scale:visualViewport.scale}));
      check(lane+'-'+kind+'-'+mode+': actual 200 percent',z.inner===width/2&&z.outer===width&&z.height===500&&z.dpr===2&&z.scale===1,z);
    }
    for(const kind of ['typed','long']){
      const key=lane+'-'+kind;await visit(kind);
      check(key+': responsive control matches actual CSS width',await page.locator('button[command="--toggle"]').isVisible()===mobile);
      if(mobile){check(key+': Tab reaches mobile toggle',await tabTo(page,'button[command="--toggle"]'));await screenFocus(page,key+' mobile toggle');await page.keyboard.press('Enter');await ready(page);check(key+': mobile destinations disclosed',await page.locator('el-disclosure a').evaluateAll(all=>all.length===2&&all.every(a=>a.checkVisibility())));}
      const primary=mobile?'el-disclosure a[href="/dashboard"]':'nav a[href="/dashboard"]:visible';
      check(key+': Tab reaches primary link',await tabTo(page,primary));await screenFocus(page,key+' primary link');
      let g=await geometry(qualityPage);check(key+': primary text/viewport containment',g.passes,g);await capture(page,key+'-primary',true);
      check(key+': Tab reaches notification',await tabTo(page,'[data-appkit-navbar-repair] > nav a[href="/notifications"]'));await screenFocus(page,key+' notification');
      check(key+': Tab reaches account',await tabTo(page,'el-dropdown > button'));await screenFocus(page,key+' account trigger');
      await page.keyboard.press('Enter');await ready(page);check(key+': account opens with keyboard focus',await page.locator('el-menu').evaluate(e=>e.matches(':popover-open')&&e.contains(document.activeElement)));
      await screenFocus(page,key+' first account item');g=await geometry(qualityPage);check(key+': open account text/viewport containment',g.passes,g);await capture(page,key+'-first-account');
      await page.keyboard.press('ArrowDown');check(key+': next account destination focused',await page.locator('el-menu a').nth(1).evaluate(e=>e===document.activeElement));await screenFocus(page,key+' second account item');await capture(page,key+'-second-account');
      await page.keyboard.press('Escape');await ready(page);check(key+': Escape restores account trigger',await page.locator('el-dropdown > button').evaluate(e=>e===document.activeElement)&&!(await page.locator('el-menu').evaluate(e=>e.matches(':popover-open'))));await screenFocus(page,key+' restored account trigger');
    }
    // Actual DOM canaries validate both supported paint mechanisms and removal/clip refusal.
    if(lane==='light-1440'){
      await visit('typed');await tabTo(page,'[data-appkit-navbar-repair] > nav a[href="/notifications"]');const node=page.locator('[data-appkit-navbar-repair] > nav a[href="/notifications"]'),saved=await node.getAttribute('style');
      try{
        await node.evaluate(e=>{e.style.outline='2px solid rgb(0, 0, 0)';e.style.outlineOffset='2px';e.style.boxShadow='none';});const outline=await focusFacts(page);
        await node.evaluate(e=>{e.style.outline='none';e.style.boxShadow='inset 0px 0px 0px 2px rgb(0, 0, 0)';});const ring=await focusFacts(page);
        const savedBaseline=await node.evaluate(e=>window.__p09Unfocused.get(e));
        check('DOM canary: keyboard leaves seeded ring',await tabTo(page,'#outside'));
        await node.evaluate(e=>{const s=getComputedStyle(e);window.__p09Unfocused.set(e,{outline:s.outline,boxShadow:s.boxShadow,background:s.backgroundColor,focusVisible:e.matches(':focus-visible')});});
        check('DOM canary: keyboard returns to permanently styled ring',await tabTo(page,'[data-appkit-navbar-repair] > nav a[href="/notifications"]'));
        const permanent=await focusFacts(page);
        await node.evaluate((e,b)=>window.__p09Unfocused.set(e,b),savedBaseline);
        const parentSaved=await node.evaluate(e=>e.parentElement.getAttribute('style'));
        try{
          await node.evaluate(e=>{e.parentElement.style.backgroundColor='rgba(0, 0, 0, 0.25)';e.style.backgroundColor='rgba(0, 0, 0, 0.5)';});
          const composed=await focusFacts(page),expected=191*127/255;
          check('DOM canary: two translucent layers composite rather than disappear',composed.backdrops.inside.color?.slice(0,3).every(v=>Math.abs(v-expected)<.001)&&composed.backdrops.inside.color[3]===255,{observed:composed.backdrops.inside.color,expected});
        }finally{await node.evaluate((e,s)=>{e.style.removeProperty('background-color');s===null?e.parentElement.removeAttribute('style'):e.parentElement.setAttribute('style',s);},parentSaved);}
        await node.evaluate(e=>{e.style.boxShadow='none';});const absent=await focusFacts(page);
        await node.evaluate(e=>{e.style.outline='2px solid rgb(0, 0, 0)';e.style.transform='translateX(-2000px)';});const clipped=await focusFacts(page);
        check('DOM canary: induced outline/ring accepted; permanent/absent/clipped paint refused',outline.passes&&ring.passes&&!permanent.passes&&!absent.passes&&!clipped.passes,{outline,ring,permanent,absent,clipped});
      }finally{await node.evaluate((e,s)=>s===null?e.removeAttribute('style'):e.setAttribute('style',s),saved);}
      check('DOM canary: exact original inline style restored',await node.getAttribute('style')===saved);
    }
    // The same failed enhanced regions remain recorded; the explicit native directory supplies access.
    for(const target of ['/dashboard','/notifications','/profile','/settings']){
      const before=report.expected_runtime_failures.filter(e=>e.event==='response').length;
      await visit('typed','unavailable');
      check(lane+target+': exact runtime outage and no custom-element upgrade',report.expected_runtime_failures.filter(e=>e.event==='response').length===before+1&&await page.evaluate(()=>['el-disclosure','el-dropdown','el-menu'].every(tag=>!customElements.get(tag))));
      const needsToggle=target==='/dashboard'&&mobile,needsAccount=target==='/profile'||target==='/settings';
      if(needsToggle||needsAccount){const selector=needsToggle?'button[command="--toggle"]':'el-dropdown > button';check(lane+target+': keyboard reaches unavailable control',await tabTo(page,selector));await page.keyboard.press('Enter');await ready(page,'unavailable');
        const owner=page.locator(needsToggle?'el-disclosure':'el-menu');
        const state=await owner.evaluate(e=>({visible:e.checkVisibility(),popoverOpen:e.matches(':popover-open'),visibleLinks:[...e.querySelectorAll('a')].filter(a=>a.checkVisibility()).length}));
        check(lane+target+': failed runtime-owned region stays wholly unavailable',!state.visible&&!state.popoverOpen&&state.visibleLinks===0,state);
      }
      check(lane+target+': keyboard reaches persistent native summary',await tabTo(page,'details[data-appkit-navbar-routes] > summary'));
      await screenFocus(page,lane+target+' native summary');await page.keyboard.press('Enter');
      check(lane+target+': native directory opens',await page.locator('details[data-appkit-navbar-routes]').evaluate(e=>e.open));
      const links=page.locator('details[data-appkit-navbar-routes] a[href="'+target+'"]:visible'),available=await links.count()>0;
      check(lane+target+': essential destination remains accessible with runtime unavailable',available,{mobile,needsToggle,needsAccount});
      if(available){check(lane+target+': fallback destination keyboard reachable',await tabTo(page,'details[data-appkit-navbar-routes] a[href="'+target+'"]:visible'));await screenFocus(page,lane+target+' native destination');await capture(page,lane+'-native-'+target.slice(1));await Promise.all([page.waitForURL(base+target,{waitUntil:'networkidle'}),page.keyboard.press('Enter')]);check(lane+target+': fallback native GET reaches destination',await page.locator('h1').textContent()===destinations.get(target));}
      else{check(lane+target+': failed disclosure does not navigate or invent success',page.url()===base+docPrefix+'/typed-'+scheme+'-unavailable');await capture(page,lane+'-runtime-unavailable-'+target.slice(1));}
    }
    // A native directory can exceed a viewport vertically. Keyboard scroll must expose
    // each entire focused label; do not misclassify ordinary document scrolling as clipping.
    await visit('long','unavailable');check(lane+': long native summary reachable',await tabTo(page,'details[data-appkit-navbar-routes] > summary'));await page.keyboard.press('Enter');
    const longRoutes=await page.locator('details[data-appkit-navbar-routes] a').evaluateAll(all=>all.map(a=>a.getAttribute('href')));
    check(lane+': long native directory retains all five routes',longRoutes.length===5);
    for(const [i,href] of longRoutes.entries()){
      const selector='details[data-appkit-navbar-routes] a[href="'+href+'"]';
      check(lane+': long native route keyboard reachable '+i,await tabTo(page,selector));await screenFocus(page,lane+' long native route '+i);
      const text=await page.locator(selector).evaluate(link=>{
        const box=link.getBoundingClientRect(),fragments=[],walker=document.createTreeWalker(link,NodeFilter.SHOW_TEXT);
        while(walker.nextNode()){const node=walker.currentNode;if(!node.textContent.trim())continue;const range=document.createRange();range.selectNodeContents(node);for(const r of range.getClientRects())if(r.width>0&&r.height>0)fragments.push({left:r.left,right:r.right,top:r.top,bottom:r.bottom});}
        return {box:{left:box.left,right:box.right,top:box.top,bottom:box.bottom},fragments,scrollY,
          passes:document.documentElement.scrollWidth<=innerWidth&&fragments.length>0&&fragments.every(r=>r.left>=box.left-1&&r.right<=box.right+1&&r.top>=box.top-1&&r.bottom<=box.bottom+1&&r.left>=-1&&r.right<=innerWidth+1&&r.top>=-1&&r.bottom<=innerHeight+1)};
      });check(lane+': complete long native label visible after keyboard scroll '+i,text.passes,text);
      if(i===0||i===3)await capture(page,lane+'-long-native-'+i);
    }
    // Runtime arrives while a native-directory route is focused: no automatic hiding or focus transfer.
    await visit('typed','unavailable');check(lane+': late-load summary reachable',await tabTo(page,'details[data-appkit-navbar-routes] > summary'));await page.keyboard.press('Enter');
    check(lane+': late-load route reachable',await tabTo(page,'details[data-appkit-navbar-routes] a[href="/profile"]'));
    await page.evaluate(src=>{const script=document.createElement('script');script.type='module';script.src=src;document.head.append(script);},base+assetPrefix+'/elements.js');await ready(page);
    check(lane+': actual Elements upgrade retains native route focus and open directory',await page.locator('details[data-appkit-navbar-routes]').evaluate(e=>e.open&&e.contains(document.activeElement)&&document.activeElement.getAttribute('href')==='/profile'));
    await screenFocus(page,lane+' late-load native route');await capture(page,lane+'-late-load');
    await Promise.all([page.waitForURL(base+'/profile',{waitUntil:'networkidle'}),page.keyboard.press('Enter')]);check(lane+': late-load native GET completes',await page.locator('h1').textContent()===destinations.get('/profile'));
    await bounded(context.close());context=undefined;check(lane+': owned Chrome exited',!alive());report.hosts.at(-1).absent=true;pid=undefined;profile=undefined;
  }
  for(const [file,hash] of inputs)assert.equal(sha(await fs.readFile(file)),hash,'input changed during proof');report.inputs_unchanged=true;
} catch(error){report.errors.push(String(error));process.exitCode=1;}
finally {
  try{
    if(context)await bounded(context.close());if(pid){assert(!alive(),'owned Chrome remains');report.hosts.at(-1).absent=true;}
    if(server?.listening){server.closeAllConnections();await bounded(new Promise((resolve,reject)=>server.close(e=>e?reject(e):resolve())));}
    if(base)await bounded(new Promise((resolve,reject)=>{const socket=net.createConnection({host:'127.0.0.1',port:Number(new URL(base).port)});socket.once('connect',()=>{socket.destroy();reject(Error('listener remains'));});socket.once('error',e=>{socket.destroy();e.code==='ECONNREFUSED'?resolve():reject(e);});}));
    report.cleanup='ok';report.listener_refused=Boolean(base);
  }catch(error){report.errors.push('cleanup: '+String(error));try{if(alive())process.kill(pid,'SIGKILL');}catch{}server?.closeAllConnections();process.exitCode=1;}
  clearTimeout(watchdog);report.inputs=Object.fromEntries(inputs);report.outputs=Object.fromEntries(outputs);
  report.passed=report.checks.filter(c=>c.ok).length;report.failed=report.checks.filter(c=>!c.ok).length;if(report.failed||report.errors.length)process.exitCode=1;
  const body=Buffer.from(JSON.stringify(report,null,2)+'\n'),name=report.failed||report.errors.length?'failure.json':'receipt.json';await fs.writeFile(path.join(out,name),body,{mode:0o600,flag:'wx'});
  process.stdout.write(JSON.stringify({out,passed:report.passed,failed:report.failed,errors:report.errors.map(e=>e.split('\n')[0]),file:name,sha256:sha(body)})+'\n');
}
