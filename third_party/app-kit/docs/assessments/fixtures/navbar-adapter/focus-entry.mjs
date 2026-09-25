// P09 observational focus-entry trace. Does not modify runtime, CSS or acceptance.
// Private synthetic fixture only; no downloads, signed-in profile or consumer writes.
import assert from 'node:assert/strict';
import {createHash, randomBytes} from 'node:crypto';
import {spawnSync} from 'node:child_process';
import fs from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';

const [baseArg,repairArg,repairSHA,supplementalArg,supplementalSHA]=process.argv.slice(2);
assert(process.argv.length===7&&[baseArg,repairArg,supplementalArg].every(path.isAbsolute)&&[repairSHA,supplementalSHA].every(v=>/^[a-f0-9]{64}$/.test(v??'')),
  'usage: node focus-entry.mjs BASE_PROBE REPAIR_PROBE REPAIR_COMPILE_SHA256 SUPPLEMENTAL_ROOT SUPPLEMENTAL_RECEIPT_SHA256');
const baseProbe=await fs.realpath(baseArg),repairProbe=await fs.realpath(repairArg),supplemental=await fs.realpath(supplementalArg),temp=await fs.realpath(os.tmpdir());
assert(supplemental.startsWith(temp+path.sep),'new outputs and profiles must be below command-scoped temporary storage');
const sha=bytes=>createHash('sha256').update(bytes).digest('hex'),inputs=new Map(),outputs=new Map();
async function input(file,expected){const name=file instanceof URL?fileURLToPath(file):file,st=await fs.lstat(name);assert(st.isFile()&&st.nlink===1&&st.size>0&&st.size<=8*2**20,'invalid bounded input');const body=await fs.readFile(name),hash=sha(body);if(expected)assert.equal(hash,expected,'input identity differs: '+path.basename(name));inputs.set(name,hash);return body;}
const out=await fs.mkdtemp(path.join(supplemental,'focus-entry-'));await fs.chmod(out,0o700);
async function write(name,body){await fs.writeFile(path.join(out,name),body,{mode:0o600,flag:'wx'});outputs.set(name,{sha256:sha(body),bytes:body.length});}
const report={schema:1,kind:'private-p09-navbar-focus-entry-trace',verification:'not-certified',repair_compile_receipt_sha256:repairSHA,supplemental_receipt_sha256:supplementalSHA,
  checks:[],diagnostics:[],captures:[],requests:[],blocked_external:[],errors:[],hosts:[],cleanup:'pending',
  limits:['Synthetic private fixtures at normal 100% in installed headless Chrome; protocol keyboard/pointer input is not physical input or assistive-technology proof.',
    'Script-disabled lanes prove the native details directory, not fallback interaction inside unavailable Elements regions.',
    'Maximum and repeated fixtures are bounded generated data, not real consumer content, source-image fidelity, performance or target certification.']};
const check=(name,ok,details={})=>report.checks.push({name,ok:Boolean(ok),...details});
let context,server,base,profile,pid;
function alive(){if(!pid||!profile)return false;const p=spawnSync('/bin/ps',['-p',String(pid),'-o','command='],{timeout:2000,maxBuffer:16384});if(p.status===1)return false;assert.equal(p.status,0);assert(p.stdout.toString().includes('--user-data-dir='+profile));return true;}
async function bounded(promise){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('operation deadline')),12000);})]);}finally{clearTimeout(timer);}}
const watchdog=setTimeout(()=>{report.errors.push('seven-minute deadline');process.exitCode=1;try{if(alive())process.kill(pid,'SIGKILL');}catch{report.errors.push('owned process stop failed');}server?.closeAllConnections();},420000);

// Kept byte-for-byte with the frozen P09 diagnostic; acceptance changes no threshold.
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

try{
  report.harness_sha256=sha(await input(new URL(import.meta.url)));
  const frozen=await input(new URL('./zoom.mjs',import.meta.url),'8b840c6c978d6a2ebc1820c76d9decd4eb063948b1ed12b39a30f25d28b1c2f3');
  assert(frozen.toString().includes(focusFacts.toString())&&frozen.toString().includes(geometry.toString()),'frozen quality oracle changed');
  const frozenRepair=JSON.parse(await input(path.join(repairProbe,'browser-3hka4c/receipt.json'),'adf849f125c20154a4fd8162c18feb4af9d763f64811b2662f55483506790b51'));
  assert.equal(frozenRepair.passed,466);assert.equal(frozenRepair.failed,0);assert.deepEqual(frozenRepair.errors,[]);
  const compile=JSON.parse(await input(path.join(repairProbe,'compile-receipt.json'),repairSHA));
  const artifact=JSON.parse(await input(path.join(repairProbe,'artifact.json'),compile.artifact_sha256));
  const supplement=JSON.parse(await input(path.join(supplemental,'acceptance-fixtures.json'),supplementalSHA));
  assert.equal(supplement.kind,'private-p09-repair-acceptance-fixtures');assert.equal(supplement.repair_compile_receipt_sha256,repairSHA);assert.equal(supplement.artifact_sha256,compile.artifact_sha256);
  for(const name of ['repair-maximum.html','repair-multiple.html'])assert.match(supplement.rendered_sha256?.[name],/^[a-f0-9]{64}$/);
  const fragments=new Map();
  fragments.set('typed',await input(path.join(repairProbe,'repair-typed.html'),compile.rendered_sha256['repair-typed.html']));
  fragments.set('maximum',await input(path.join(supplemental,'repair-maximum.html'),supplement.rendered_sha256['repair-maximum.html']));
  fragments.set('multiple',await input(path.join(supplemental,'repair-multiple.html'),supplement.rendered_sha256['repair-multiple.html']));
  for(const [name,key] of [['navbar_repair.css','css_sha256'],['navbar_routes.go','routes_go_sha256'],['navbar_routes.templ','routes_templ_sha256']])await input(path.join(repairProbe,name),artifact[key]);
  const prior=JSON.parse(await input(path.join(baseProbe,'browser-Tn5991/receipt.json'),'2dfad48ffd46e6187ab4809b3a7dd65675a1af48fdbf1702c1e01ca1102dbe48'));
  const assetPath=prior.requests.find(r=>r.path.endsWith('/source.css'))?.path;assert(assetPath);const assetPrefix=path.posix.dirname(assetPath),prefix='/acceptance-'+randomBytes(18).toString('hex');
  const sourceCSS=await input(path.join(path.dirname(path.join(baseProbe,'browser-Tn5991/receipt.json')),'source.css'),prior.outputs['source.css'].sha256);
  await input('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs','a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
  const kit=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../../..'),assetsRoot=path.join(kit,'data/design-assets/reference-assets/elements-1.0.22-pilot');
  const runtime=await input(path.join(assetsRoot,'elements.js'),artifact.base.runtime.esm_sha256),font=await input(path.join(assetsRoot,'inter.woff2'),prior.assets['inter.woff2'].sha256),repairCSS=await input(path.join(repairProbe,'navbar_repair.css'),artifact.css_sha256);
  const logo=Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" rx="5" fill="oklch(0.58 0.20 280)"/></svg>');
  const fixtureCSS=Buffer.from('html{font-family:InterVariable,system-ui,sans-serif;color-scheme:light}html.dark{color-scheme:dark}body{margin:0;background:Canvas;color:CanvasText}section{margin-block:1rem}');
  const resources=new Map([[assetPrefix+'/elements.js',{type:'text/javascript',body:runtime}],[assetPrefix+'/inter.woff2',{type:'font/woff2',body:font}],[assetPrefix+'/source.css',{type:'text/css',body:sourceCSS}],[assetPrefix+'/fixture.css',{type:'text/css',body:fixtureCSS}],[assetPrefix+'/navbar_repair.css',{type:'text/css',body:repairCSS}],[assetPrefix+'/logo.svg',{type:'image/svg+xml',body:logo}]]);
  const decode=s=>s.replaceAll('&amp;','&');
  const localURLs=new Set(['/dashboard','/notifications','/profile','/settings','/assets/brand-light.svg','/assets/brand-dark.svg','/assets/avatar.jpg']);
  for(const body of fragments.values())for(const match of body.toString().matchAll(/(?:href|src)="([^"]+)"/g)){const value=decode(match[1]);if(value.startsWith('/'))localURLs.add(value);}
  for(const value of localURLs){const url=new URL(value,'http://fixture.invalid');if(url.pathname.startsWith('/assets/'))resources.set(url.pathname+url.search,{type:'image/svg+xml',body:logo});else resources.set(url.pathname+url.search,{type:'text/html; charset=utf-8',body:Buffer.from('<!doctype html><title>Fixture destination</title><h1>Synthetic destination</h1>')});}
  const documents=new Map();
  for(const [name,fragment] of fragments)for(const scheme of ['light','dark'])for(const scripts of ['ready','disabled']){
    const script='<script type="module" src="'+assetPrefix+'/elements.js"></script>';
    const body=Buffer.from('<!doctype html><html lang="en" class="'+(scheme==='dark'?'dark':'')+'"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="icon" href="'+assetPrefix+'/logo.svg"><link rel="stylesheet" href="'+assetPrefix+'/source.css"><link rel="stylesheet" href="'+assetPrefix+'/fixture.css"><link id="repair-css" rel="stylesheet" href="'+assetPrefix+'/navbar_repair.css">'+script+'</head><body><main>'+fragment+'</main><button id="outside">Outside</button></body></html>');
    documents.set(prefix+'/'+name+'-'+scheme+'-'+scripts,{type:'text/html; charset=utf-8',body});
  }
  server=http.createServer((req,res)=>{const url=new URL(req.url,base||'http://invalid');if(!base||req.method!=='GET'||req.headers.host!==new URL(base).host||url.origin!==base){res.writeHead(403).end();return;}const key=url.pathname+url.search,r=documents.get(key)||resources.get(key);report.requests.push({path:key,status:r?200:404});if(!r){res.writeHead(404).end();return;}res.writeHead(200,{'Content-Type':r.type,'Cache-Control':'no-store','X-Content-Type-Options':'nosniff','Content-Security-Policy':"default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self'; connect-src 'none'; form-action 'none'; base-uri 'none'; object-src 'none'"});res.end(r.body);});
  await bounded(new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);}));base='http://127.0.0.1:'+server.address().port;
  const {chromium}=await import(pathToFileURL('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs').href);
  async function launch(width,scheme,javaScriptEnabled=true){profile=await fs.mkdtemp(path.join(out,'profile-'));await fs.chmod(profile,0o700);context=await chromium.launchPersistentContext(profile,{executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true,viewport:{width,height:1000},colorScheme:scheme,reducedMotion:'reduce',serviceWorkers:'block',javaScriptEnabled,args:['--enable-automation','--disable-background-networking','--no-proxy-server','--no-first-run','--no-default-browser-check','--window-size='+width+',1000']});report.chrome_version??=context.browser().version();const host=await context.browser().newBrowserCDPSession();try{const info=(await host.send('SystemInfo.getProcessInfo')).processInfo.filter(p=>p.type==='browser');assert.equal(info.length,1);pid=info[0].id;assert(alive());report.hosts.push({pid,profile,requested_window:{width,height:1000}});}finally{await host.detach();}await context.route('**/*',route=>{const url=new URL(route.request().url());if(url.origin!==base||route.request().method()!=='GET'||(!documents.has(url.pathname+url.search)&&!resources.has(url.pathname+url.search))){report.blocked_external.push({url_sha256:sha(Buffer.from(route.request().url()))});return route.abort();}return route.continue();});const page=context.pages()[0];page.on('pageerror',error=>report.errors.push('page: '+String(error)));page.on('requestfailed',request=>report.errors.push('request: '+request.failure()?.errorText));page.on('response',response=>{if(response.status()>=400)report.errors.push('response: '+response.status());});return page;}
  async function closeLane(lane){await bounded(context.close());context=undefined;check(lane+': owned Chrome exited',!alive());report.hosts.at(-1).absent=true;pid=undefined;profile=undefined;}
  async function ready(page,withRuntime=true){await page.waitForFunction(()=>document.fonts.status==='loaded');if(withRuntime)await page.waitForFunction(()=>['el-disclosure','el-dropdown','el-menu'].every(tag=>customElements.get(tag)));let stable=0;for(let i=0;i<100;i++){const settled=await page.evaluate(()=>document.getAnimations().every(a=>a.playState!=='running'&&!a.pending)&&[...document.querySelectorAll('el-menu:popover-open')].every(e=>getComputedStyle(e).opacity==='1'));stable=settled?stable+1:0;if(stable===3)return;await page.waitForTimeout(50);}throw Error('runtime did not settle');}
  async function visit(page,name,scheme,scripts='ready'){await page.goto(base+prefix+'/'+name+'-'+scheme+'-'+scripts,{waitUntil:'networkidle'});await ready(page,scripts==='ready');await page.evaluate(()=>{window.__p09Unfocused=new Map([...document.querySelectorAll('a,button,summary')].map(e=>{const s=getComputedStyle(e);return[e,{outline:s.outline,boxShadow:s.boxShadow,background:s.backgroundColor,focusVisible:e.matches(':focus-visible')}];}));});}
  async function tabTo(page,selector,max=80){for(let i=0;i<max;i++){await page.keyboard.press('Tab');if(await page.locator(selector).evaluate(e=>e===document.activeElement))return true;}return false;}
  async function tabToLocator(page,locator,max=80){for(let i=0;i<max;i++){await page.keyboard.press('Tab');if(await locator.evaluate(e=>e===document.activeElement))return true;}return false;}
  async function textFits(locator){return locator.evaluate(link=>{const box=link.getBoundingClientRect(),parts=[],clips=[],walker=document.createTreeWalker(link,NodeFilter.SHOW_TEXT);while(walker.nextNode()){if(!walker.currentNode.textContent.trim())continue;const range=document.createRange();range.selectNodeContents(walker.currentNode);for(const r of range.getClientRects())if(r.width&&r.height)parts.push({left:r.left,right:r.right,top:r.top,bottom:r.bottom});}const topLayer=link.closest(':popover-open');for(let n=link;n;n=n.parentElement){const s=getComputedStyle(n),b=n.getBoundingClientRect();if(/hidden|clip|scroll|auto/.test(s.overflowX)||/hidden|clip|scroll|auto/.test(s.overflowY))clips.push({left:b.left,right:b.right,top:b.top,bottom:b.bottom,x:/hidden|clip|scroll|auto/.test(s.overflowX),y:/hidden|clip|scroll|auto/.test(s.overflowY)});if(n===topLayer)break;}return{box:{left:box.left,right:box.right,top:box.top,bottom:box.bottom},parts,clips,passes:parts.length>0&&parts.every(r=>r.left>=box.left-1&&r.right<=box.right+1&&r.top>=box.top-1&&r.bottom<=box.bottom+1&&r.left>=-1&&r.right<=innerWidth+1&&r.top>=-1&&r.bottom<=innerHeight+1&&clips.every(c=>(!c.x||(r.left>=c.left-1&&r.right<=c.right+1))&&(!c.y||(r.top>=c.top-1&&r.bottom<=c.bottom+1))))&&document.documentElement.scrollWidth<=innerWidth};});}
  async function capture(page,name){const body=await page.screenshot({animations:'disabled'});await write(name+'.png',body);report.captures.push({file:name+'.png',sha256:sha(body),bytes:body.length});}

  await input(new URL('./acceptance.mjs',import.meta.url),'5517f72b46b986954ee6001d36388c583b38ff260a03ead9404f51f7f95ee590');
  await input(path.join(supplemental,'acceptance-5JN3xE/failure.json'),'47927d1d646fdab6f84178e53513ba670be043cfed94256be66daff3054bcc99');
  report.traces=[];
  report.limits=['Observational event listeners and bounded animation-frame samples; no runtime/CSS/DOM behavior patch.',
    'Normal CSS viewport emulation, light theme, typed versus maximum fixtures; not performance, rapid-input or target certification.',
    'Home/arrow observations diagnose reachability; they do not waive failed first-link entry acceptance.'];
  for(const width of [1440,390]) {
    const page=await launch(width,'light',true);page.setDefaultTimeout(6000);
    for(const name of ['typed','maximum']) {
      const lane=name+'-'+width;
      await visit(page,name,'light');
      await page.evaluate(()=>{
        const menu=document.querySelector('el-menu'),links=[...menu.querySelectorAll('a')];
        const rect=e=>{if(!e)return null;const r=e.getBoundingClientRect();return{x:r.x,y:r.y,width:r.width,height:r.height};};
        const element=e=>({tag:e?.tagName,id:e?.id,linkIndex:links.indexOf(e),tabindex:e?.getAttribute?.('tabindex'),hrefPrefix:e?.getAttribute?.('href')?.slice(0,80)});
        window.__entryEvents=[];
        window.__entryState=()=>({time:performance.now(),active:element(document.activeElement),internalActiveIndex:links.indexOf(menu.getActiveItem()),
          first:element(links[0]),menuTabindex:menu.getAttribute('tabindex'),open:menu.matches(':popover-open'),scrollTop:menu.scrollTop,docScroll:scrollY,
          menu:rect(menu),firstRect:rect(links[0]),opacity:getComputedStyle(menu).opacity,transform:getComputedStyle(menu).transform,
          animations:document.getAnimations().map(a=>({state:a.playState,pending:a.pending,currentTime:a.currentTime}))});
        const capture=e=>{if(window.__entryEvents.length<256)window.__entryEvents.push({event:e.type,key:e.key,trusted:e.isTrusted,target:element(e.target),related:element(e.relatedTarget),oldState:e.oldState,newState:e.newState,stack:new Error().stack,...window.__entryState()});};
        for(const type of ['focusin','focusout','keydown','keyup','click','mouseover','mouseout','beforetoggle','toggle'])document.addEventListener(type,capture,{capture:true,passive:true});
      });
      check(lane+': trigger keyboard reached',await tabTo(page,'el-dropdown > button'));
      const before=await page.evaluate(()=>window.__entryState());
      await page.evaluate(()=>{window.__entryFrames=[];window.__entryFramesDone=false;let remaining=24;const sample=()=>{window.__entryFrames.push(window.__entryState());if(--remaining>0)requestAnimationFrame(sample);else window.__entryFramesDone=true;};requestAnimationFrame(sample);});
      await page.keyboard.press('Enter');await ready(page);await page.waitForFunction(()=>window.__entryFramesDone);
      const entered=await page.evaluate(()=>window.__entryState()),paint=await focusFacts(page);
      check(lane+': Enter settles on first account link',entered.open&&entered.active.linkIndex===0&&paint.passes,{state:entered,paint});
      await capture(page,lane+'-entered');
      await page.keyboard.press('ArrowDown');await ready(page);const down=await page.evaluate(()=>window.__entryState());
      await page.keyboard.press('Home');await ready(page);const home=await page.evaluate(()=>window.__entryState());
      await page.keyboard.press('Escape');await ready(page);const escaped=await page.evaluate(()=>window.__entryState());
      report.traces.push({lane,before,entered,down,home,escaped,...await page.evaluate(()=>({events:window.__entryEvents,frames:window.__entryFrames}))});
    }
    await closeLane('width-'+width);
  }
  check('no external traffic',report.blocked_external.length===0,{count:report.blocked_external.length});
  for(const [file,hash] of inputs)assert.equal(sha(await fs.readFile(file)),hash,'input changed during proof');report.inputs_unchanged=true;
}catch(error){report.errors.push(String(error));process.exitCode=1;}
finally{try{if(context)await bounded(context.close());if(pid){assert(!alive(),'owned Chrome remains');report.hosts.at(-1).absent=true;}if(server?.listening){server.closeAllConnections();await bounded(new Promise((resolve,reject)=>server.close(e=>e?reject(e):resolve())));}if(base)await bounded(new Promise((resolve,reject)=>{const socket=net.createConnection({host:'127.0.0.1',port:Number(new URL(base).port)});socket.once('connect',()=>{socket.destroy();reject(Error('listener remains'));});socket.once('error',e=>{socket.destroy();e.code==='ECONNREFUSED'?resolve():reject(e);});}));report.cleanup='ok';report.listener_refused=Boolean(base);}catch(error){report.errors.push('cleanup: '+String(error));try{if(alive())process.kill(pid,'SIGKILL');}catch{}server?.closeAllConnections();process.exitCode=1;}clearTimeout(watchdog);report.inputs=Object.fromEntries(inputs);report.outputs=Object.fromEntries(outputs);report.passed=report.checks.filter(c=>c.ok).length;report.failed=report.checks.filter(c=>!c.ok).length;if(report.failed||report.errors.length)process.exitCode=1;const body=Buffer.from(JSON.stringify(report,null,2)+'\n'),name=report.failed||report.errors.length?'failure.json':'receipt.json';await fs.writeFile(path.join(out,name),body,{mode:0o600,flag:'wx'});process.stdout.write(JSON.stringify({out,passed:report.passed,failed:report.failed,errors:report.errors,file:name,sha256:sha(body)})+'\n');}

