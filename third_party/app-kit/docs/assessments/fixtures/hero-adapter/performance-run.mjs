// Executed only from the private snapshot made by performance.mjs. No fetches or installs.
import fs from 'node:fs/promises';
import {createReadStream} from 'node:fs';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
import path from 'node:path';
import os from 'node:os';
import http from 'node:http';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {evaluateSample,evaluateMatrix,evaluateCanary} from './performance-oracle.mjs';
import {makeFontCSS} from './font-delivery.mjs';
import {paintedFonts} from './font-painted.mjs';

const argv=process.argv.slice(2),[probeArg,referenceArg,parentArg,mode='none',candidateReceiptArg,candidateSHA]=argv;
const candidate=mode==='--font-candidate',canary=candidate?'none':mode;
assert.ok([probeArg,referenceArg,parentArg].every(p=>p && path.isAbsolute(p)) &&
  (candidate?argv.length===6&&path.isAbsolute(candidateReceiptArg)&&/^[a-f0-9]{64}$/.test(candidateSHA):argv.length>=3&&argv.length<=4&&['none','missing-font','delayed-css','oversize-image'].includes(canary)), 'execute through performance.mjs with valid baseline/candidate arguments');
const sha=b=>createHash('sha256').update(b).digest('hex');
const read=async p=>{const s=await fs.lstat(p);assert.ok(s.isFile()&&!s.isSymbolicLink()&&s.size<64*1024*1024,'bounded real input: '+p);return fs.readFile(p)};
const write=(p,b)=>fs.writeFile(p,b,{flag:'wx',mode:0o600});
const dir=path.dirname(fileURLToPath(import.meta.url)),contractBytes=await read(path.join(dir,'performance-contract.json'));
const contract=JSON.parse(contractBytes);
const probe=await fs.realpath(probeArg),referencePath=await fs.realpath(referenceArg),parent=await fs.realpath(parentArg);
const root=process.env.APPKIT_HERO_PERF_RUN_ROOT;
assert.ok(root&&path.isAbsolute(root)&&path.dirname(dir)===root&&path.basename(dir)==='runner','execute retained snapshot through performance.mjs');
assert.equal(path.dirname(root),parent,'run root must belong to explicit private parent');
const envelope=JSON.parse(await read(path.join(root,'snapshot.json')));
const report={schema:1,kind:contract.id,verification:'not-certified',canary,startedAt:new Date().toISOString(),contractSHA:sha(contractBytes),host:{platform:os.platform(),architecture:os.arch(),release:os.release(),cpu:os.cpus()[0]?.model,node:process.version},inputs:[],samples:[],failures:[],screenshots:[],serverRequests:[],limits:contract.limits};
const bind=async(name,file,expected)=>{const bytes=await read(file);if(expected)assert.equal(sha(bytes),expected,name+' identity');const copy=path.join(root,'inputs',name);await fs.mkdir(path.dirname(copy),{recursive:true,mode:0o700});await write(copy,bytes);report.inputs.push({name,source:file,sha256:sha(bytes),bytes:bytes.length});return bytes};
let browser,browserServer,browserProcess,closingBrowser,server,base,activeLane,aborted=false;const pendingTimers=new Set();
report.browserProcesses=[];
const bounded=async(promise,ms,label)=>{let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error(label+' exceeded '+ms+'ms')),ms)})])}finally{clearTimeout(timer)}};
const waitForExit=proc=>proc.exitCode!==null||proc.signalCode!==null?Promise.resolve():bounded(new Promise(resolve=>proc.once('close',resolve)),5000,'owned browser exit');
const hashFile=async file=>{const hash=createHash('sha256');for await(const chunk of createReadStream(file))hash.update(chunk);return hash.digest('hex')};
const closeBrowser=()=>{
  if(closingBrowser)return closingBrowser;
  if(!browserServer)return Promise.resolve();
  const owned=browserServer,proc=browserProcess;browserServer=undefined;browser=undefined;
  closingBrowser=(async()=>{
  try{await bounded(owned.close(),10000,'browser close')}catch(error){
    report.failures.push({code:'browser-cleanup',detail:String(error)});
    try{await bounded(owned.kill(),5000,'owned browser kill')}catch(killError){
      report.failures.push({code:'browser-kill',detail:String(killError)});
      if(proc.exitCode===null&&proc.signalCode===null)proc.kill('SIGKILL');
      try{await waitForExit(proc)}catch(exitError){report.failures.push({code:'browser-exit-timeout',detail:String(exitError)})}
    }
  }
  report.browserProcesses.push({pid:proc.pid,exitCode:proc.exitCode,signalCode:proc.signalCode});
  if(proc.exitCode!==0||proc.signalCode!==null)report.failures.push({code:'browser-exit',detail:JSON.stringify(report.browserProcesses.at(-1))});
  })().finally(()=>{closingBrowser=undefined});
  return closingBrowser;
};
const deadline=setTimeout(()=>{aborted=true;report.failures.push({code:'watchdog',detail:'120s bound'});closeBrowser().catch(error=>report.failures.push({code:'watchdog-cleanup',detail:String(error)}));server?.closeAllConnections()},120000);
process.once('SIGTERM',()=>{
  aborted=true;report.failures.push({code:'launcher-stop',detail:'SIGTERM received; bounded owned cleanup requested'});
  server?.closeAllConnections();
  closeBrowser().catch(error=>report.failures.push({code:'signal-cleanup',detail:String(error)}));
});
try {
  for(const input of envelope.inputs)assert.equal(sha(await read(path.join(dir,input.name))),input.sha256,'snapshot identity: '+input.name);
  report.executedSnapshot=envelope;
  const reference=JSON.parse(await bind('reference-receipt.json',referencePath,contract.pins.referenceReceipt));
  const original=await bind('original.html',path.join(probe,'original.html'),contract.pins.source);
  const artifact=JSON.parse(await bind('artifact.json',path.join(probe,'delivery/artifact.json'),contract.pins.artifact));
  assert.equal(artifact.source.sha256,sha(original));assert.equal(artifact.verification,'not-verified');
  for(const [name,digest] of [['hero.templ',artifact.templ_sha256],['hero.go',artifact.model_sha256],['defaults.json',artifact.defaults_sha256]])await bind('delivery/'+name,path.join(probe,'delivery',name),digest);
  const fragment=await bind('enhanced.html',path.join(probe,'enhanced.html'),contract.pins.enhanced);
  const sourceCSS=await bind('source.css',path.join(path.dirname(referencePath),'source.css'),reference.inputs.tailwind.css_sha256);
  const heroCSS=await bind('hero.css',path.join(probe,'delivery/hero.css'),artifact.css_sha256);
  const resources=new Map([['/source.css',{type:'text/css',body:sourceCSS}],['/hero.css',{type:'text/css',body:heroCSS}]]);
  const assets=JSON.parse(await bind('assets.json',path.join(path.dirname(referencePath),'assets.json')));
  assert.equal(assets.assets.length,3);
  const specs={'logo-light.svg':artifact.source_assets.logo_light_url,'logo-dark.svg':artifact.source_assets.logo_dark_url,'hero.jpg':artifact.source_assets.image_url};
  for(const a of assets.assets){assert.equal(a.url,specs[a.name]);const pin=reference.assets.find(p=>p.name===a.name);assert.ok(pin);assert.equal(a.sha256,pin.sha256);const body=await bind('assets/'+a.name,path.join(path.dirname(referencePath),'assets',a.name),a.sha256);assert.equal(body.length,a.bytes);resources.set('/assets/'+a.name,{type:a.mime,body})}
  const font=await bind('assets/inter.woff2',path.join(path.dirname(referencePath),'assets/inter.woff2'),reference.inputs.font.sha256);resources.set('/assets/inter.woff2',{type:'font/woff2',body:font});
  report.deliveryProfile='unchanged-original-assets';
  if(candidate){
    // Admit a separate, explicitly pinned and already reviewed fidelity run.
    // The original source, performance contract and baseline receipt stay fixed.
    const candidatePath=await fs.realpath(candidateReceiptArg),candidateRoot=path.dirname(candidatePath);
    const proof=JSON.parse(await bind('font-candidate/proof.json',candidatePath,candidateSHA));
    assert.equal(proof.passed,true);assert.equal(proof.preparationPassed,true);assert.equal(proof.browserPassed,true);assert.deepEqual(proof.failures,[]);
    assert.equal(proof.shaping.length,70);assert.ok(proof.shaping.every(p=>p.equal===true));
    const browserProof=JSON.parse(await bind('font-candidate/browser.json',path.join(candidateRoot,'font-browser-receipt.json'),proof.browserReceiptSHA));
    assert.equal(browserProof.kind,'p01-font-delivery-browser-v1');
    assert.equal(browserProof.passed,true);for(const field of ['fidelity','typography','selection','negativeControl'])assert.equal(browserProof[field].passed,true);
    const prepared=JSON.parse(await bind('font-candidate/prepared.json',path.join(candidateRoot,'prepared.json'),proof.preparedSHA));
    assert.equal(prepared.font.originalSHA,sha(font));
    const manifest=JSON.parse(await bind('font-candidate/font-receipt.json',path.join(candidateRoot,'build-a/font-receipt.json'),proof.generation.manifestSHA));
    assert.equal(manifest.sourceSHA,sha(font));assert.equal(manifest.outputs['inter-subset.woff2'].sha256,prepared.font.subsetSHA);
    const subsetFont=await bind('assets/inter-subset.woff2',path.join(candidateRoot,'build-a/inter-subset.woff2'),prepared.font.subsetSHA);
    assert.equal(subsetFont.length,manifest.outputs['inter-subset.woff2'].bytes);
    const candidateCSS=await bind('font-candidate/source.css',path.join(candidateRoot,'inputs/candidate.css'),prepared.resources.find(r=>r.path==='/candidate.css').sha256);
    assert.deepEqual(candidateCSS,Buffer.from(makeFontCSS(sourceCSS.toString())),'only the declared font faces change');
    resources.set('/source.css',{type:'text/css',body:candidateCSS});resources.set('/assets/inter-subset.woff2',{type:'font/woff2',body:subsetFont});
    report.deliveryProfile='separate-font-subset-candidate';report.fontCandidate={receiptSHA:candidateSHA,originalFontSHA:sha(font),subsetFontSHA:sha(subsetFont),sourceCSS:sha(sourceCSS),candidateCSS:sha(candidateCSS)};
  }
  const document=Buffer.from('<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Private hero enhanced</title><link rel="icon" href="data:,"><link rel="stylesheet" href="/source.css"><link rel="stylesheet" href="/hero.css"></head><body>'+fragment.toString()+'</body></html>');
  resources.set('/enhanced',{type:'text/html',body:document});await write(path.join(root,'document.html'),document);
  const csp="default-src 'none'; style-src 'self'; font-src 'self'; img-src 'self' data:; script-src 'none'; connect-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'none'";
  report.instrumentation={javaScriptEnabled:true,applicationScripts:0,csp,inlineFavicon:'data:,',note:'CDP instrument callbacks require script execution; CSP blocks application scripts. Empty inline favicon prevents implicit /favicon.ico traffic.'};
  server=http.createServer((req,res)=>{
    if(req.method!=='GET'||req.headers.host!==new URL(base).host||!resources.has(req.url)){res.writeHead(403).end();report.failures.push({code:'server-request',detail:req.url});return}
    const item=resources.get(req.url);let body=item.body,status=200;
    if(canary==='missing-font' && req.url==='/assets/inter.woff2'){body=Buffer.from('missing fixture font');status=404}
    if(canary==='oversize-image' && req.url==='/assets/hero.jpg')body=Buffer.concat([body,Buffer.alloc(600*1024)]);
    report.serverRequests.push({lane:activeLane,path:req.url,status,bodyBytes:body.length});
    const send=()=>{res.writeHead(status,{'Content-Type':item.type,'Content-Length':body.length,'Content-Security-Policy':csp,'X-Content-Type-Options':'nosniff','Cache-Control':req.url==='/enhanced'?'no-store':'private,max-age=3600,immutable'});res.end(body)};
    if(canary==='delayed-css'&&req.url==='/source.css'){const t=setTimeout(()=>{pendingTimers.delete(t);send()},2500);pendingTimers.add(t)}else send();
  });
  await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve)});base='http://127.0.0.1:'+server.address().port;report.origin=base;
  const pw=process.env.APPKIT_PLAYWRIGHT_MODULE;assert.ok(pw&&path.isAbsolute(pw),'explicit existing approved Playwright module required');await bind('playwright-index.mjs',pw,contract.pins.playwrightEntry);
  const {chromium}=await import(pathToFileURL(pw));
  const executablePath=process.env.APPKIT_BROWSER_EXECUTABLE||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';assert.ok(path.isAbsolute(executablePath));
  const executableStat=await fs.stat(executablePath);assert.ok(executableStat.isFile());
  report.browserExecutable={path:executablePath,bytes:executableStat.size,sha256:await hashFile(executablePath)};
  const version=await promisify(execFile)(executablePath,['--version'],{timeout:10000,maxBuffer:4096});
  report.browserExecutable.versionOutput=version.stdout.trim();assert.ok(version.stdout.trim().endsWith(' '+contract.pins.browserVersion));
  report.playwrightIdentityScope='Selected entry module only, not the transitive Playwright installation';
  const repeats=canary==='none'?contract.repetitions:1;
  for(let repeat=1;repeat<=repeats;repeat++){
    assert.equal(aborted,false,'watchdog already fired');
    browserServer=await chromium.launchServer({headless:true,executablePath,args:['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1']});
    browserProcess=browserServer.process();
    assert.equal(aborted,false,'watchdog fired during browser launch');
    await write(path.join(root,'browser-'+repeat+'.json'),JSON.stringify({pid:browserProcess.pid,executable:report.browserExecutable,startedAt:new Date().toISOString()},null,2)+'\n');
    browser=await chromium.connect(browserServer.wsEndpoint());assert.equal(browser.version(),contract.pins.browserVersion);report.browser=browser.version();
    for(const width of (canary==='none'?contract.widths:[390]))for(const scheme of (canary==='none'?contract.schemes:['light'])){
      const context=await browser.newContext({javaScriptEnabled:true,viewport:{width,height:contract.viewportHeight},colorScheme:scheme,reducedMotion:'reduce',deviceScaleFactor:1,serviceWorkers:'block'});
      const page=await context.newPage();page.setDefaultTimeout(7000);const cdp=await context.newCDPSession(page);
      await cdp.send('Network.enable');await cdp.send('Network.clearBrowserCache');await cdp.send('Network.setCacheDisabled',{cacheDisabled:false});
      await cdp.send('Network.emulateNetworkConditions',{offline:false,latency:contract.network.latencyMs,downloadThroughput:contract.network.downloadBytesPerSecond,uploadThroughput:contract.network.uploadBytesPerSecond,connectionType:'cellular4g'});
      await cdp.send('Emulation.setCPUThrottlingRate',{rate:contract.cpuSlowdown});await cdp.send('Performance.enable');
      let observed=new Map(),unexpected=[];
      cdp.on('Network.requestWillBeSent',e=>{const u=new URL(e.request.url);if(u.origin!==base||!resources.has(u.pathname)||u.search||e.request.method!=='GET')unexpected.push(e.request.url);observed.set(e.requestId,{id:e.requestId,url:e.request.url,path:u.pathname,type:e.type,cache:false,started:e.timestamp})});
      cdp.on('Network.requestServedFromCache',e=>{const r=observed.get(e.requestId);if(r)r.cache=true});
      cdp.on('Network.responseReceived',e=>{const r=observed.get(e.requestId);if(r)Object.assign(r,{status:e.response.status,cache:Boolean(r.cache||e.response.fromDiskCache||e.response.fromPrefetchCache||e.response.fromServiceWorker),mime:e.response.mimeType})});
      cdp.on('Network.loadingFinished',e=>{const r=observed.get(e.requestId);if(r){r.finished=true;r.encodedDataLength=e.encodedDataLength;r.durationMs=(e.timestamp-r.started)*1000}});
      cdp.on('Network.loadingFailed',e=>{const r=observed.get(e.requestId);if(r)Object.assign(r,{finished:false,error:e.errorText,durationMs:(e.timestamp-r.started)*1000})});
      for(const phase of (canary==='none'?contract.phases:['cold'])){
        assert.equal(aborted,false,'watchdog already fired');
        observed=new Map();unexpected=[];activeLane=`${repeat}-${width}-${scheme}-${phase}`;
        const sample={repeat,width,scheme,phase,assetFailures:[],unexpectedRequests:unexpected,keyboardLinks:[]};
        try{
          await page.goto(base+'/enhanced',{waitUntil:'load',timeout:15000});
          const state=await bounded(page.evaluate(async()=>{
            let fontReady=false;try{await document.fonts.ready;fontReady=document.fonts.check('16px InterVariable')}catch{}
            await Promise.all([...document.images].map(i=>i.decode().catch(()=>{})));
            await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
            const visible=e=>{const b=e.getBoundingClientRect();return b.width>0&&b.height>0};
            const usableMs=performance.now();let lcpMs=null;
            await new Promise(resolve=>{const observer=new PerformanceObserver(list=>{for(const e of list.getEntries())lcpMs=e.startTime});observer.observe({type:'largest-contentful-paint',buffered:true});setTimeout(()=>{observer.disconnect();resolve()},200)});
            return {navigationType:performance.getEntriesByType('navigation')[0]?.type,fontReady,fontSetStatus:document.fonts.status,imagesDecoded:[...document.images].filter(i=>i.complete&&i.naturalWidth>0).length,visibleImages:[...document.images].filter(visible).length,visibleLinks:[...document.querySelectorAll('a')].filter(visible).map(a=>a.getAttribute('href')),scripts:document.scripts.length,usableMs,fcpMs:performance.getEntriesByType('paint').find(e=>e.name==='first-contentful-paint')?.startTime??null,lcpMs};
          }),10000,'measurement instrumentation');Object.assign(sample,state);
          sample.heapBytes=(await cdp.send('Performance.getMetrics')).metrics.find(m=>m.name==='JSHeapUsedSize')?.value??null;
          if(candidate){
            // check() includes the unused unrestricted fallback. Preserve its
            // result diagnostically; prove the actual rendered fonts instead.
            sample.fontEligibilityCheck=state.fontReady;sample.paintedFonts=await paintedFonts(page,cdp);
            sample.fontReady=state.fontSetStatus==='loaded'&&sample.paintedFonts.failures.length===0;
            for(const detail of sample.paintedFonts.failures)sample.assetFailures.push({code:'painted-font',detail});
          }
          for(const wanted of (width<640?['/start','/learn']:['/announcement','/start','/learn'])){await page.keyboard.press('Tab');sample.keyboardLinks.push(await page.evaluate(()=>document.activeElement?.getAttribute('href')))}
          sample.requests=observed.size;sample.bodyBytes=0;sample.cacheHits=0;sample.requestGraph=[];
          for(const item of observed.values()){
            if(item.cache)sample.cacheHits++;
            const expected=resources.get(item.path)?.body||Buffer.alloc(0);
            const graph={...item,expectedBodyBytes:expected.length,expectedSHA256:sha(expected),serverDelayMs:canary==='delayed-css'&&item.path==='/source.css'?2500:0};try{
              const response=await cdp.send('Network.getResponseBody',{requestId:item.id});const bytes=Buffer.from(response.body,response.base64Encoded?'base64':'utf8');graph.bodyBytes=bytes.length;graph.sha256=sha(bytes);
              if(!item.cache)sample.bodyBytes+=bytes.length;
              if(item.status!==200||!item.finished||item.error||sha(resources.get(item.path)?.body||Buffer.alloc(0))!==graph.sha256)sample.assetFailures.push({path:item.path,status:item.status,identity:'mismatch-or-incomplete'});
            }catch(error){sample.assetFailures.push({path:item.path,error:String(error)})}
            sample.requestGraph.push(graph);
          }
          sample.documentCache=[...observed.values()].find(r=>r.path==='/enhanced')?.cache;
          sample.serverDocumentRequests=report.serverRequests.filter(r=>r.lane===activeLane&&r.path==='/enhanced').length;
          const requiredPaths=[...resources.keys()].filter(url=>!(candidate&&url==='/assets/inter.woff2'));
          for(const url of requiredPaths)if(![...observed.values()].some(r=>r.path===url))sample.assetFailures.push({path:url,error:'not observed in browser request graph'});
          if(candidate&&[...observed.values()].some(r=>r.path==='/assets/inter.woff2'))sample.assetFailures.push({path:'/assets/inter.woff2',error:'Latin-only hero unexpectedly loaded the full font'});
          const png=await page.screenshot({fullPage:false,animations:'disabled'});const name=activeLane+'.png';await write(path.join(root,name),png);report.screenshots.push({name,sha256:sha(png),bytes:png.length});
        }catch(error){sample.assetFailures.push({error:String(error.stack||error)})}
        sample.failures=evaluateSample(contract,sample);report.samples.push(sample);console.log(activeLane+': '+sample.failures.map(f=>f.code).join(',')+' body='+sample.bodyBytes);
      }
      await context.close();
    }
    await closeBrowser();
  }
  if(canary==='none')report.failures.push(...evaluateMatrix(contract,report.samples));
  else{
    report.canaryResult=evaluateCanary(contract,canary,report.samples.length===1?report.samples[0]:null);
    if(!report.canaryResult.detected)report.failures.push({code:'canary-not-detected',detail:report.canaryResult.failures});
  }
  for(const input of report.inputs)assert.equal(sha(await read(input.source)),input.sha256,'input drift: '+input.name);
  for(const input of envelope.inputs)assert.equal(sha(await read(path.join(dir,input.name))),input.sha256,'executed snapshot drift: '+input.name);
  assert.equal(await hashFile(executablePath),report.browserExecutable.sha256,'browser executable drift');
}catch(error){report.failures.push({code:'execution',detail:String(error.stack||error)})}
finally{
  clearTimeout(deadline);for(const timer of pendingTimers)clearTimeout(timer);
  await closeBrowser();
  if(server){server.closeAllConnections();try{await bounded(new Promise(resolve=>server.close(resolve)),3000,'HTTP close');report.serverClosed=!server.listening}catch(error){report.failures.push({code:'server-cleanup',detail:String(error)})}}
  report.finishedAt=new Date().toISOString();report.executionPassed=report.failures.length===0&&report.serverClosed===true;
  report.passed=canary==='none'&&report.executionPassed;
  await write(path.join(root,'receipt.json'),JSON.stringify(report,null,2)+'\n');
}
console.log(JSON.stringify({root,passed:report.passed,canaryDetected:report.canaryResult?.detected,samples:report.samples.length,failures:report.failures},null,2));process.exitCode=report.executionPassed?0:1;
