// Actual Elements lifecycle proof for the explicitly selected P09 runtime derivative.
// Synthetic private fixtures only; no downloads, consumer writes, or global settings.
import assert from 'node:assert/strict';
import {createHash,randomBytes} from 'node:crypto';
import {spawnSync} from 'node:child_process';
import fs from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';

const [baseArg,repairArg,compileSHA,supplementalArg,supplementalSHA,runtimeArg,runtimeReceiptSHA]=process.argv.slice(2);
assert(process.argv.length===9&&[baseArg,repairArg,supplementalArg,runtimeArg].every(path.isAbsolute)&&[compileSHA,supplementalSHA,runtimeReceiptSHA].every(v=>/^[a-f0-9]{64}$/.test(v??'')),
  'usage: node runtime-lifecycle.mjs BASE_PROBE REPAIR_PROBE COMPILE_SHA SUPPLEMENTAL_ROOT SUPPLEMENTAL_SHA RUNTIME_ROOT RUNTIME_RECEIPT_SHA');
const [baseProbe,repairProbe,supplemental,runtimeRoot,temp]=await Promise.all([baseArg,repairArg,supplementalArg,runtimeArg,os.tmpdir()].map(value=>fs.realpath(value)));
assert(runtimeRoot.startsWith(temp+path.sep),'outputs must remain below command-scoped temporary storage');
const sha=body=>createHash('sha256').update(body).digest('hex'),inputs=new Map(),outputs=new Map();
async function input(file,expected){const name=file instanceof URL?fileURLToPath(file):file,st=await fs.lstat(name);assert(st.isFile()&&st.nlink===1&&st.size>0&&st.size<=8*2**20,'invalid bounded input');const body=await fs.readFile(name),hash=sha(body);if(expected)assert.equal(hash,expected,'input identity differs: '+path.basename(name));inputs.set(name,hash);return body;}
const out=await fs.mkdtemp(path.join(runtimeRoot,'lifecycle-'));await fs.chmod(out,0o700);
const report={schema:1,kind:'private-p09-navbar-runtime-lifecycle',verification:'not-certified',checks:[],observations:[],requests:[],errors:[],hosts:[],cleanup:'pending',limits:['Playwright pointer and keyboard input is protocol-driven, not physical-input or assistive-technology proof.','Close and detach are programmatic lifecycle probes, not physical-input tasks.','The unselected maximum fixture is synthetic isolation evidence, not original-source fidelity or consumer certification.']};
const check=(name,ok,details={})=>report.checks.push({name,ok:Boolean(ok),...details});
let context,server,base,profile,pid;
function alive(){if(!pid||!profile)return false;const p=spawnSync('/bin/ps',['-p',String(pid),'-o','command='],{timeout:2000,maxBuffer:16384});if(p.status===1)return false;assert.equal(p.status,0);assert(p.stdout.toString().includes('--user-data-dir='+profile));return true;}
async function bounded(p){let timer;try{return await Promise.race([p,new Promise((_,reject)=>timer=setTimeout(()=>reject(Error('operation deadline')),12000))]);}finally{clearTimeout(timer);}}
const watchdog=setTimeout(()=>{report.errors.push('five-minute deadline');process.exitCode=1;try{if(alive())process.kill(pid,'SIGKILL');}catch{}server?.closeAllConnections();},300000);

try{
  report.harness_sha256=sha(await input(new URL(import.meta.url)));
  const frozen=await input(new URL('./acceptance.mjs',import.meta.url),'5517f72b46b986954ee6001d36388c583b38ff260a03ead9404f51f7f95ee590');
  const runtimeHarness=await input(new URL('./runtime-acceptance.mjs',import.meta.url));
  report.provenance={frozen_acceptance_sha256:sha(frozen),runtime_acceptance_sha256:sha(runtimeHarness)};
  const compile=JSON.parse(await input(path.join(repairProbe,'compile-receipt.json'),compileSHA));
  const supplement=JSON.parse(await input(path.join(supplemental,'acceptance-fixtures.json'),supplementalSHA));
  assert.equal(supplement.artifact_sha256,compile.artifact_sha256);
  const runtimeReceipt=JSON.parse(await input(path.join(runtimeRoot,'runtime-receipt.json'),runtimeReceiptSHA));
  const runtimeArtifact=JSON.parse(await input(path.join(runtimeRoot,'runtime-artifact.json'),runtimeReceipt.artifact_sha256));
  assert.equal(runtimeReceipt.runtime_sha256,runtimeArtifact.runtime_sha256);assert.equal(runtimeArtifact.repair_sha256,compile.artifact_sha256);
  const derivative=await input(path.join(runtimeRoot,'elements-navbar.js'),runtimeArtifact.runtime_sha256);
  const kit=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../../..'),assetsRoot=path.join(kit,'data/design-assets/reference-assets/elements-1.0.22-pilot');
  const original=await input(path.join(assetsRoot,'elements.js'),runtimeArtifact.original_runtime_sha256);
  const prior=JSON.parse(await input(path.join(baseProbe,'browser-Tn5991/receipt.json'),'2dfad48ffd46e6187ab4809b3a7dd65675a1af48fdbf1702c1e01ca1102dbe48'));
  const sourceCSS=await input(path.join(baseProbe,'browser-Tn5991/source.css'),prior.outputs['source.css'].sha256),font=await input(path.join(assetsRoot,'inter.woff2'),prior.assets['inter.woff2'].sha256);
  const repairArtifact=JSON.parse(await input(path.join(repairProbe,'artifact.json'),compile.artifact_sha256)),repairCSS=await input(path.join(repairProbe,'navbar_repair.css'),repairArtifact.css_sha256);
  const small=await input(path.join(repairProbe,'repair-typed.html'),compile.rendered_sha256['repair-typed.html']),maximum=await input(path.join(supplemental,'repair-maximum.html'),supplement.rendered_sha256['repair-maximum.html']);
  await input('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs','a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
  const unselect=body=>{const text=body.toString();assert.equal(text.split('<div data-appkit-navbar-repair>').length-1,1);return Buffer.from(text.replace('<div data-appkit-navbar-repair>','<div>'));};
  const unselectedSmall=unselect(small),makeScrollable=body=>{let text=body.toString().replace('<div>','<div data-runtime-isolation>');const match=text.match(/(<el-menu\b[^>]*>)([\s\S]*?)(<\/el-menu>)/);assert(match);text=text.replace(match[0],match[1]+match[2].repeat(6)+match[3]);return Buffer.from(text);};
  const fragments={small,maximum,'small-unselected':unselectedSmall,'scrollable-unselected':makeScrollable(unselectedSmall)},prefix='/lifecycle-'+randomBytes(16).toString('hex');
  const fixtureCSS=Buffer.from('html{font-family:InterVariable,system-ui,sans-serif}body{margin:0}[data-runtime-isolation] el-menu{block-size:8rem;overflow:auto}');
  const logo=Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" fill="purple"/></svg>');
  const resources=new Map([[prefix+'/original.js',{type:'text/javascript',body:original}],[prefix+'/derivative.js',{type:'text/javascript',body:derivative}],[prefix+'/source.css',{type:'text/css',body:sourceCSS}],[prefix+'/repair.css',{type:'text/css',body:repairCSS}],[prefix+'/fixture.css',{type:'text/css',body:fixtureCSS}],['/navbar-9e32853ebb59f523d1b17d1d25edea5aef2c/inter.woff2',{type:'font/woff2',body:font}],[prefix+'/logo.svg',{type:'image/svg+xml',body:logo}]]),documents=new Map();
  for(const body of Object.values(fragments))for(const match of body.toString().matchAll(/(?:href|src)="([^"]+)"/g)){const value=match[1].replaceAll('&amp;','&');if(value.startsWith('/assets/')){const url=new URL(value,'http://fixture.invalid');resources.set(url.pathname+url.search,{type:'image/svg+xml',body:logo});}}
  for(const [kind,fragment] of Object.entries(fragments))for(const runtime of ['original','derivative'])documents.set(prefix+'/'+kind+'-'+runtime,{type:'text/html; charset=utf-8',body:Buffer.from('<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="icon" href="'+prefix+'/logo.svg"><link rel="stylesheet" href="'+prefix+'/source.css"><link rel="stylesheet" href="'+prefix+'/fixture.css"><link rel="stylesheet" href="'+prefix+'/repair.css"><script type="module" src="'+prefix+'/'+runtime+'.js"></script></head><body><main>'+fragment+'</main><button id="sentinel">Sentinel</button></body></html>')});
  server=http.createServer((req,res)=>{const url=new URL(req.url,base||'http://invalid'),key=url.pathname+url.search,r=documents.get(key)||resources.get(key);if(!base||req.method!=='GET'||req.headers.host!==new URL(base).host||url.origin!==base||!r){res.writeHead(r?403:404).end();return;}report.requests.push(key);res.writeHead(200,{'Content-Type':r.type,'Cache-Control':'no-store','X-Content-Type-Options':'nosniff','Content-Security-Policy':"default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self'; connect-src 'none'; form-action 'none'; base-uri 'none'; object-src 'none'"});res.end(r.body);});
  await bounded(new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);}));base='http://127.0.0.1:'+server.address().port;
  const {chromium}=await import(pathToFileURL('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs').href);profile=await fs.mkdtemp(path.join(out,'profile-'));await fs.chmod(profile,0o700);
  context=await chromium.launchPersistentContext(profile,{executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true,viewport:{width:1440,height:1000},reducedMotion:'reduce',serviceWorkers:'block',args:['--enable-automation','--disable-background-networking','--no-proxy-server','--no-first-run','--no-default-browser-check','--window-size=1440,1000']});report.chrome_version=context.browser().version();report.viewport={width:1440,height:1000,deviceScaleFactor:1};
  const host=await context.browser().newBrowserCDPSession();try{const info=(await host.send('SystemInfo.getProcessInfo')).processInfo.filter(p=>p.type==='browser');assert.equal(info.length,1);pid=info[0].id;assert(alive());report.hosts.push({pid,profile});}finally{await host.detach();}
  await context.route('**/*',route=>{const u=new URL(route.request().url()),key=u.pathname+u.search;if(u.origin!==base||route.request().method()!=='GET'||(!documents.has(key)&&!resources.has(key))){report.errors.push('unexpected request');return route.abort();}return route.continue();});
  const page=context.pages()[0];page.setDefaultTimeout(6000);page.on('pageerror',e=>report.errors.push('page: '+e.message));page.on('requestfailed',r=>report.errors.push('request: '+r.failure()?.errorText));
  async function settle(){await page.waitForFunction(()=>document.fonts.status==='loaded'&&['el-dropdown','el-menu'].every(tag=>customElements.get(tag)));let stable=0;for(let i=0;i<100;i++){const done=await page.evaluate(()=>document.getAnimations().every(a=>a.playState!=='running'&&!a.pending)&&[...document.querySelectorAll('el-menu:popover-open')].every(e=>getComputedStyle(e).opacity==='1'));stable=done?stable+1:0;if(stable===3)return;await page.waitForTimeout(50);}throw Error('runtime did not settle');}
  async function visit(kind,runtime){await page.goto(base+prefix+'/'+kind+'-'+runtime,{waitUntil:'networkidle'});await settle();await page.evaluate(()=>{window.__events=[];for(const type of ['focusin','beforetoggle','toggle','pointerdown','keydown'])document.addEventListener(type,e=>window.__events.push({type,tag:e.target?.tagName,key:e.key??null,pointerType:e.pointerType??null,trusted:e.isTrusted,href:e.target?.getAttribute?.('href')??null}),true);});}
  async function state(){return page.locator('el-menu').evaluate(menu=>{const links=[...menu.querySelectorAll('a')],active=document.activeElement,internal=menu.getActiveItem();return{activeTag:active?.tagName,activeIndex:links.indexOf(active),internalActiveIndex:links.indexOf(internal),tabindexZeroIndex:links.findIndex(a=>a.getAttribute('tabindex')==='0'),open:menu.matches(':popover-open'),scrollHeight:menu.scrollHeight,clientHeight:menu.clientHeight,scrollTop:menu.scrollTop,events:window.__events};});}
  async function tabTo(locator){for(let i=0;i<30;i++){await page.keyboard.press('Tab');if(await locator.evaluate(e=>e===document.activeElement))return true;}return false;}
  async function keyboard(kind,runtime){await visit(kind,runtime);const trigger=page.locator('el-dropdown > button');assert(await tabTo(trigger));await page.keyboard.press('Enter');await settle();return state();}
  async function pointer(kind,runtime){await visit(kind,runtime);const trigger=page.locator('el-dropdown > button');await trigger.scrollIntoViewIfNeeded();const setup=await trigger.evaluate(e=>{const r=e.getBoundingClientRect();return{visible:e.checkVisibility(),rect:{left:r.left,right:r.right,top:r.top,bottom:r.bottom},viewport:{width:innerWidth,height:innerHeight}};});assert(setup.visible&&setup.rect.left>=0&&setup.rect.right<=setup.viewport.width&&setup.rect.top>=0&&setup.rect.bottom<=setup.viewport.height,'pointer trigger is not visibly contained');await trigger.click();await settle();return{...await state(),pointerSetup:setup};}

  for(const kind of ['small','maximum']){
    const entered=await keyboard(kind,'derivative');check('selected '+kind+': Enter focuses item zero',entered.activeTag==='A'&&entered.activeIndex===0&&entered.internalActiveIndex===0&&entered.open,entered);
    await page.keyboard.press('ArrowDown');await settle();let moved=await state();check('selected '+kind+': ArrowDown focuses item one',moved.activeIndex===1&&moved.internalActiveIndex===1,moved);
    await page.keyboard.press('Home');await settle();moved=await state();check('selected '+kind+': Home restores item zero',moved.activeIndex===0&&moved.internalActiveIndex===0,moved);
    await page.keyboard.press('Escape');await settle();check('selected '+kind+': Escape restores trigger',await page.locator('el-dropdown > button').evaluate(e=>e===document.activeElement)&&!(await page.locator('el-menu').evaluate(e=>e.matches(':popover-open'))));
    const pointed=await pointer(kind,'derivative');check('selected '+kind+': trusted pointer open focuses empty menu',pointed.activeTag==='EL-MENU'&&pointed.internalActiveIndex===-1&&pointed.open&&pointed.events.some(e=>e.type==='pointerdown'&&e.trusted&&e.pointerType==='mouse'),pointed);
  }

  for(const action of ['close','detach']){
    await visit('small','derivative');const observed=await page.evaluate(action=>{const menu=document.querySelector('el-menu'),sentinel=document.querySelector('#sentinel');sentinel.focus();menu.showPopover();if(action==='close')menu.hidePopover();else menu.remove();return{immediate:document.activeElement===sentinel};},action);await page.waitForTimeout(50);const final=await page.evaluate(()=>({active:document.activeElement?.id,menu:!!document.querySelector('el-menu'),open:document.querySelector('el-menu')?.matches(':popover-open')??false}));check('selected '+action+': deferred timer cannot steal focus',observed.immediate&&final.active==='sentinel'&&!final.open,{observed,final});
  }

  for(const kind of ['small-unselected','scrollable-unselected']){
    const originalKeyboard=await keyboard(kind,'original'),derivedKeyboard=await keyboard(kind,'derivative');const comparable=s=>({activeTag:s.activeTag,activeIndex:s.activeIndex,internalActiveIndex:s.internalActiveIndex,tabindexZeroIndex:s.tabindexZeroIndex,open:s.open,scrollHeight:s.scrollHeight,clientHeight:s.clientHeight,scrollable:s.scrollHeight>s.clientHeight});
    check(kind+': keyboard behavior unchanged',JSON.stringify(comparable(originalKeyboard))===JSON.stringify(comparable(derivedKeyboard))&&(kind!=='scrollable-unselected'||comparable(originalKeyboard).scrollable),{original:comparable(originalKeyboard),derivative:comparable(derivedKeyboard)});
    const originalPointer=await pointer(kind,'original'),derivedPointer=await pointer(kind,'derivative');check(kind+': pointer behavior unchanged',JSON.stringify(comparable(originalPointer))===JSON.stringify(comparable(derivedPointer))&&(kind!=='scrollable-unselected'||comparable(originalPointer).scrollable),{original:comparable(originalPointer),derivative:comparable(derivedPointer),originalSetup:originalPointer.pointerSetup,derivativeSetup:derivedPointer.pointerSetup});
    report.observations.push({kind,originalKeyboard:comparable(originalKeyboard),derivedKeyboard:comparable(derivedKeyboard),originalPointer:comparable(originalPointer),derivedPointer:comparable(derivedPointer)});
  }
  for(const [file,hash] of inputs)assert.equal(sha(await fs.readFile(file)),hash,'input changed during proof');report.inputs_unchanged=true;
}catch(error){report.errors.push(String(error));process.exitCode=1;}
finally{try{if(context)await bounded(context.close());if(pid){assert(!alive(),'owned Chrome remains');report.hosts.at(-1).absent=true;}if(server?.listening){server.closeAllConnections();await bounded(new Promise((resolve,reject)=>server.close(e=>e?reject(e):resolve())));}if(base)await bounded(new Promise((resolve,reject)=>{const socket=net.createConnection({host:'127.0.0.1',port:Number(new URL(base).port)});socket.once('connect',()=>{socket.destroy();reject(Error('listener remains'));});socket.once('error',e=>{socket.destroy();e.code==='ECONNREFUSED'?resolve():reject(e);});}));report.cleanup='ok';report.listener_refused=Boolean(base);}catch(error){report.errors.push('cleanup: '+String(error));try{if(alive())process.kill(pid,'SIGKILL');}catch{}server?.closeAllConnections();process.exitCode=1;}clearTimeout(watchdog);report.inputs=Object.fromEntries(inputs);report.outputs=Object.fromEntries(outputs);report.passed=report.checks.filter(c=>c.ok).length;report.failed=report.checks.filter(c=>!c.ok).length;if(report.failed||report.errors.length)process.exitCode=1;const body=Buffer.from(JSON.stringify(report,null,2)+'\n'),name=report.failed||report.errors.length?'failure.json':'receipt.json';await fs.writeFile(path.join(out,name),body,{mode:0o600,flag:'wx'});process.stdout.write(JSON.stringify({out,passed:report.passed,failed:report.failed,errors:report.errors,file:name,sha256:sha(body)})+'\n');}
