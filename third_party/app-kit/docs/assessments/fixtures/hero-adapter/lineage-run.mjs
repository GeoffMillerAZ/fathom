// Execute only the retained snapshot from lineage-performance.mjs. No downloads.
import fs from 'node:fs/promises';
import {createReadStream} from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import http from 'node:http';
import assert from 'node:assert/strict';
import {createHash,randomUUID} from 'node:crypto';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {paintedFonts} from './font-painted.mjs';
import {evaluateMatrix} from './performance-oracle.mjs';
import {evaluateLineagePair,deriveLineageSample} from './lineage-oracle.mjs';

const [candidateArg,originalArg,parentArg,mode='matrix']=process.argv.slice(2);
assert.ok([candidateArg,originalArg,parentArg].every(p=>p&&path.isAbsolute(p))&&['matrix','cache-clear','representation-policy','original-control'].includes(mode));
const sha=b=>createHash('sha256').update(b).digest('hex');
const read=async p=>{const s=await fs.lstat(p);assert.ok(s.isFile()&&!s.isSymbolicLink()&&s.size<64*1024*1024,'bounded real input: '+p);return fs.readFile(p)};
const write=(p,b)=>fs.writeFile(p,b,{flag:'wx',mode:0o600});
const hashFile=async p=>{const h=createHash('sha256');for await(const chunk of createReadStream(p))h.update(chunk);return h.digest('hex')};
const bounded=async(promise,ms,label)=>{let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' timed out')),ms)})])}finally{clearTimeout(timer)}};
const dir=path.dirname(fileURLToPath(import.meta.url)),root=process.env.APPKIT_LINEAGE_RUN_ROOT;
assert.ok(root&&path.isAbsolute(root)&&path.dirname(dir)===root&&path.basename(dir)==='runner','retained launcher required');
assert.equal(path.dirname(root),await fs.realpath(parentArg));
const contractBytes=await read(path.join(dir,'cache-lineage-contract.json')),contract=JSON.parse(contractBytes);
const v1Bytes=await read(path.join(dir,'performance-contract.json')),v1=JSON.parse(v1Bytes);
assert.equal(sha(v1Bytes),contract.strictV1ContractSHA256);
const envelope=JSON.parse(await read(path.join(root,'snapshot.json')));
const report={schema:2,kind:contract.id,mode,verification:'not-verified',startedAt:new Date().toISOString(),contractSHA:sha(contractBytes),host:{platform:os.platform(),architecture:os.arch(),release:os.release(),cpu:os.cpus()[0]?.model,node:process.version},inputs:[],pairs:[],samples:[],failures:[],screenshots:[],browserProcesses:[],servers:[],limits:contract.limits};
const bind=async(name,file,digest)=>{const bytes=await read(file);assert.equal(sha(bytes),digest,'input identity: '+name);const target=path.join(root,'inputs',name);await fs.mkdir(path.dirname(target),{recursive:true,mode:0o700});await write(target,bytes);report.inputs.push({name,source:file,sha256:digest,bytes:bytes.length});return bytes};
let browser,ownedBrowser,ownedProcess,closingBrowser,server,aborted=false;
async function closeBrowser(){
  if(closingBrowser)return closingBrowser;if(!ownedBrowser)return;
  const owned=ownedBrowser,proc=ownedProcess;ownedBrowser=undefined;browser=undefined;
  closingBrowser=(async()=>{
    try{await bounded(owned.close(),10000,'browser close')}
    catch(e){report.failures.push({code:'browser.cleanup',detail:String(e)});try{await bounded(owned.kill(),5000,'owned browser kill')}catch{if(proc.exitCode===null&&proc.signalCode===null)proc.kill('SIGKILL')}}
    if(proc.exitCode===null&&proc.signalCode===null)try{await bounded(new Promise(resolve=>proc.once('close',resolve)),5000,'owned browser exit')}catch(e){report.failures.push({code:'browser.exitTimeout',detail:String(e)})}
    const record={pid:proc.pid,exitCode:proc.exitCode,signalCode:proc.signalCode};report.browserProcesses.push(record);
    if(record.exitCode!==0||record.signalCode!==null)report.failures.push({code:'browser.exit',detail:record});
  })().finally(()=>{closingBrowser=undefined});return closingBrowser;
}
async function closeServer(){
  if(!server)return;const owned=server;server=undefined;owned.closeAllConnections();
  await bounded(new Promise((resolve,reject)=>owned.close(e=>e?reject(e):resolve())),3000,'HTTP close');
  report.servers.at(-1).closed=!owned.listening;
}
const stop=reason=>{aborted=true;report.failures.push({code:'execution.stop',detail:reason});server?.closeAllConnections();closeBrowser().catch(e=>report.failures.push({code:'stop.cleanup',detail:String(e)}))};
const deadline=setTimeout(()=>stop('120s watchdog'),120000);process.once('SIGTERM',()=>stop('launcher SIGTERM'));
try{
  for(const input of envelope.inputs)assert.equal(sha(await read(path.join(dir,input.name))),input.sha256,'snapshot identity: '+input.name);
  report.executedSnapshot=envelope;
  const candidatePath=await fs.realpath(candidateArg),originalPath=await fs.realpath(originalArg);
  const candidate=JSON.parse(await bind('candidate-receipt.json',candidatePath,contract.candidateReceiptSHA256));
  const original=JSON.parse(await bind('original-control-receipt.json',originalPath,contract.originalControlReceiptSHA256));
  assert.equal(candidate.deliveryProfile,'separate-font-subset-candidate');assert.equal(original.deliveryProfile,'unchanged-original-assets');
  // Pins admit these exact failed historical packets as input provenance, not success.
  assert.equal(candidate.passed,false);assert.equal(original.passed,false);
  const copied=new Map();
  for(const item of candidate.inputs){
    assert.ok(/^[a-zA-Z0-9_./-]+$/.test(item.name)&&!item.name.split('/').includes('..')&&!item.name.startsWith('/'));
    copied.set(item.name,await bind('prior/'+item.name,path.join(path.dirname(candidatePath),'inputs',item.name),item.sha256));
  }
  const originalMode=mode==='original-control';
  const selected=[['/enhanced',null,'text/html','Document'],['/source.css',originalMode?'source.css':'font-candidate/source.css','text/css','Stylesheet'],['/hero.css','hero.css','text/css','Stylesheet'],['/assets/logo-light.svg','assets/logo-light.svg','image/svg+xml','Image'],['/assets/logo-dark.svg','assets/logo-dark.svg','image/svg+xml','Image'],['/assets/hero.jpg','assets/hero.jpg','image/jpeg','Image'],[originalMode?'/assets/inter.woff2':'/assets/inter-subset.woff2',originalMode?'assets/inter.woff2':'assets/inter-subset.woff2','font/woff2','Font']];
  const priorDocument=candidate.samples[0].requestGraph.find(r=>r.path==='/enhanced');
  const document=await bind('prior-document.html',path.join(path.dirname(candidatePath),'document.html'),priorDocument.sha256);assert.equal(document.length,priorDocument.bodyBytes);
  await write(path.join(root,'document.html'),document);
  const resources=new Map(selected.map(([url,name,mime,type])=>[url,{body:name?copied.get(name):document,mime,type}]));
  assert.ok([...resources.values()].every(x=>Buffer.isBuffer(x.body)));
  const manifest=()=>[...resources].map(([url,item])=>({path:url,bytes:item.body.length,sha256:sha(item.body),mime:item.mime,type:item.type}));
  const historicalGraph=(originalMode?original:candidate).samples[0].requestGraph;
  for(const item of manifest()){
    const matches=historicalGraph.filter(r=>r.path===item.path);assert.equal(matches.length,1);
    assert.equal(item.sha256,matches[0].sha256,'unchanged historical resource: '+item.path);assert.equal(item.bytes,matches[0].bodyBytes);
  }
  const manifestSHA=sha(JSON.stringify(manifest()));report.resources=manifest();report.manifestSHA=manifestSHA;
  const fontPath=selected.at(-1)[0],font=resources.get(fontPath);
  const csp="default-src 'none'; style-src 'self'; font-src 'self'; img-src 'self' data:; script-src 'none'; connect-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'none'";
  report.instrumentation={csp,applicationScripts:0,interception:false,serviceWorkers:'block',note:'CDP evaluation only; CSP denies application scripts; no external assets or signed-in profile.'};
  const pw=process.env.APPKIT_PLAYWRIGHT_MODULE;assert.ok(pw&&path.isAbsolute(pw));assert.equal(sha(await read(pw)),contract.playwrightEntrySHA256);
  const {chromium}=await import(pathToFileURL(pw));
  const executablePath='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
  report.browserExecutable={path:executablePath,sha256:await hashFile(executablePath)};report.playwrightIdentityScope='Pinned entry module, not the transitive installation';
  const origins=new Set();
  for(let repeat=1;repeat<=(mode==='matrix'?contract.repetitions:1);repeat++){
    assert.equal(aborted,false);
    ownedBrowser=await chromium.launchServer({headless:true,executablePath,args:['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1']});ownedProcess=ownedBrowser.process();
    await write(path.join(root,'browser-'+repeat+'.json'),JSON.stringify({pid:ownedProcess.pid,startedAt:new Date().toISOString(),executable:report.browserExecutable},null,2)+'\n');
    browser=await chromium.connect(ownedBrowser.wsEndpoint());assert.equal(browser.version(),contract.browserVersion);report.browserVersion=browser.version();
    for(const width of mode==='matrix'?contract.widths:[390])for(const scheme of mode==='matrix'?contract.schemes:['light']){
      assert.equal(aborted,false);let origin,activePhase=null;const serverRecords=[];
      server=http.createServer((req,res)=>{
        const item=resources.get(req.url);res.sendDate=false;
        if(!activePhase||req.method!=='GET'||req.headers.host!==new URL(origin).host||!item||['cookie','authorization','proxy-authorization','range','if-none-match','if-modified-since'].some(h=>Object.hasOwn(req.headers,h))){serverRecords.push({phase:activePhase,url:origin+req.url,method:req.method,status:403,headers:req.headers});res.writeHead(403).end();return}
        const responseHeaders={'content-type':item.mime,'content-length':String(item.body.length),'cache-control':req.url==='/enhanced'?'no-store':'private,max-age=3600,immutable',etag:'"sha256-'+sha(item.body)+'"','content-security-policy':csp,'x-content-type-options':'nosniff',connection:'close'};
        if(mode==='representation-policy'&&req.url===fontPath)responseHeaders.vary='Accept-Encoding';
        serverRecords.push({phase:activePhase,url:origin+req.url,method:req.method,status:200,bytes:item.body.length,sha256:sha(item.body),requestHeaders:req.headers,responseHeaders});
        res.writeHead(200,responseHeaders);res.end(item.body);
      });
      await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve)});origin='http://127.0.0.1:'+server.address().port;
      assert.equal(origins.has(origin),false,'unique origin per pair');origins.add(origin);report.servers.push({origin,closed:false});
      const context=await browser.newContext({javaScriptEnabled:true,viewport:{width,height:contract.viewportHeight},deviceScaleFactor:1,reducedMotion:'reduce',colorScheme:scheme,serviceWorkers:'block'});
      const page=await context.newPage();page.setDefaultTimeout(7000);const cdp=await context.newCDPSession(page);
      const contextId=randomUUID(),pair={repeat,width,scheme,origin,contextId,browserPid:ownedProcess.pid,manifestBefore:sha(JSON.stringify(manifest())),setup:{freshContext:true,cacheClearedBeforeCold:false,serviceWorkersBlocked:true,interceptionEnabled:false,applicationScripts:0,priorRequests:0},navigationCount:0,rawEvents:[],unexpectedEvents:[]};report.pairs.push(pair);
      let seq=0,observed=new Map();const lower=headers=>Object.fromEntries(Object.entries(headers).map(([k,v])=>[k.toLowerCase(),String(v)]).sort(([a],[b])=>a.localeCompare(b)));
      function event(name,e){const sequence=++seq;if(pair.rawEvents.length>=512){stop('raw event bound');return sequence}pair.rawEvents.push({sequence,phase:activePhase,name,event:e});return sequence}
      cdp.on('Network.requestWillBeSent',e=>{
        const s=event('requestWillBeSent',e);if(!activePhase)pair.setup.priorRequests++;
        if(observed.has(e.requestId))pair.unexpectedEvents.push('duplicate request ID or redirect: '+e.requestId);
        observed.set(e.requestId,{id:e.requestId,url:e.request.url,method:e.request.method,type:e.type,frameId:e.frameId,loaderId:e.loaderId,requestSequence:s,requestHeaders:lower(e.request.headers),redirect:!!e.redirectResponse,cacheSequence:null,body:{available:false,error:{name:'Uncollected'}}});
      });
      cdp.on('Network.requestServedFromCache',e=>{const s=event('requestServedFromCache',e),r=observed.get(e.requestId);if(!r||r.cacheSequence!==null)pair.unexpectedEvents.push('unknown or duplicate cache event: '+e.requestId);else r.cacheSequence=s});
      cdp.on('Network.responseReceived',e=>{const s=event('responseReceived',e),r=observed.get(e.requestId);if(!r||r.responseSequence)pair.unexpectedEvents.push('unknown or duplicate response: '+e.requestId);else{
        if(e.frameId!==r.frameId||e.loaderId!==r.loaderId||e.response.url!==r.url)pair.unexpectedEvents.push('response lineage mismatch: '+e.requestId);
        Object.assign(r,{responseSequence:s,status:e.response.status,responseHeaders:lower(e.response.headers),fromDiskCache:e.response.fromDiskCache??false,fromPrefetchCache:e.response.fromPrefetchCache??false,fromServiceWorker:e.response.fromServiceWorker??false});
      }});
      cdp.on('Network.loadingFinished',e=>{const s=event('loadingFinished',e),r=observed.get(e.requestId);if(!r||r.finishSequence)pair.unexpectedEvents.push('unknown or duplicate finish: '+e.requestId);else Object.assign(r,{finishSequence:s,encodedDataLength:e.encodedDataLength})});
      cdp.on('Network.loadingFailed',e=>{event('loadingFailed',e);pair.unexpectedEvents.push('loading failed: '+e.requestId+': '+e.errorText)});
      // Preserve supplementary headers as raw facts; do not synthesize absent warm events.
      for(const name of ['requestWillBeSentExtraInfo','responseReceivedExtraInfo'])cdp.on('Network.'+name,e=>event(name,e));
      await cdp.send('Network.enable');await cdp.send('Network.clearBrowserCache');pair.setup.cacheClearedBeforeCold=true;
      await cdp.send('Network.setCacheDisabled',{cacheDisabled:false});await cdp.send('Performance.enable');
      await cdp.send('Network.emulateNetworkConditions',{offline:false,latency:contract.network.latencyMs,downloadThroughput:contract.network.downloadBytesPerSecond,uploadThroughput:contract.network.uploadBytesPerSecond,connectionType:'cellular4g'});
      await cdp.send('Emulation.setCPUThrottlingRate',{rate:contract.cpuSlowdown});
      for(const phase of originalMode?['cold']:['cold','warm']){
        assert.equal(aborted,false);activePhase=phase;observed=new Map();
        if(phase==='warm'&&mode==='cache-clear'){await cdp.send('Network.clearBrowserCache');pair.liveFault='cache-cleared-between-navigations'}
        const phaseRecord={documentURL:origin+'/enhanced',requests:[],serverRequests:[],unexpectedRequests:[]};pair[phase]=phaseRecord;
        pair.navigationCount++;await page.goto(phaseRecord.documentURL,{waitUntil:'load',timeout:15000});
        const state=await bounded(page.evaluate(async()=>{
          await document.fonts.ready;await Promise.all([...document.images].map(i=>i.decode().catch(()=>{})));
          await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
          const visible=e=>{const b=e.getBoundingClientRect();return b.width>0&&b.height>0};
          const usableMs=performance.now();let lcpMs=null;
          await new Promise(resolve=>{const o=new PerformanceObserver(list=>{for(const e of list.getEntries())lcpMs=e.startTime});o.observe({type:'largest-contentful-paint',buffered:true});setTimeout(()=>{o.disconnect();resolve()},200)});
          return {navigationType:performance.getEntriesByType('navigation')[0]?.type,fontReady:document.fonts.status==='loaded',imagesDecoded:[...document.images].filter(i=>i.complete&&i.naturalWidth>0).length,visibleImages:[...document.images].filter(visible).length,visibleLinks:[...document.querySelectorAll('a')].filter(visible).map(a=>a.getAttribute('href')),scripts:document.scripts.length,usableMs,fcpMs:performance.getEntriesByType('paint').find(e=>e.name==='first-contentful-paint')?.startTime??null,lcpMs};
        }),10000,'measurement');
        const frame=(await cdp.send('Page.getFrameTree')).frameTree.frame;
        Object.assign(phaseRecord,{navigationType:state.navigationType,frameId:frame.id,loaderId:frame.loaderId});
        const painted=await paintedFonts(page,cdp);phaseRecord.metrics={...state,fontReady:state.fontReady&&painted.failures.length===0,keyboardLinks:[],heapBytes:(await cdp.send('Performance.getMetrics')).metrics.find(m=>m.name==='JSHeapUsedSize')?.value??null};
        // Capture before keyboard scrolling, at the same fixed viewport in both phases.
        const geometry=await page.evaluate(()=>[...document.querySelectorAll('h1,p,a,img')].filter(e=>{const r=e.getBoundingClientRect();return r.width>0&&r.height>0}).map(e=>{const r=e.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height}}));
        const png=await page.screenshot({fullPage:false,animations:'disabled'}),pngName=`${repeat}-${width}-${scheme}-${phase}.png`;
        await write(path.join(root,pngName),png);report.screenshots.push({name:pngName,sha256:sha(png),bytes:png.length});
        assert.equal(png.subarray(0,8).toString('hex'),'89504e470d0a1a0a');
        phaseRecord.render={pngSHA:sha(png),pngWidth:png.readUInt32BE(16),pngHeight:png.readUInt32BE(20),geometry,paintedFonts:painted.nodes,fontReady:phaseRecord.metrics.fontReady};
        for(const wanted of width<640?['/start','/learn']:['/announcement','/start','/learn']){await page.keyboard.press('Tab');phaseRecord.metrics.keyboardLinks.push(await page.evaluate(()=>document.activeElement?.getAttribute('href')))}
        for(const r of observed.values()){
          try{const reply=await cdp.send('Network.getResponseBody',{requestId:r.id}),body=Buffer.from(reply.body,reply.base64Encoded?'base64':'utf8');r.body={available:true,bytes:body.length,sha256:sha(body)};
            const bodyName=`body-${repeat}-${width}-${scheme}-${phase}-${phaseRecord.requests.length}.bin`;await write(path.join(root,bodyName),body);r.bodyFile=bodyName;
          }catch(e){r.body={available:false,error:{name:e.name,command:'Network.getResponseBody',message:e.message}};r.bodyErrorText=String(e)}
          phaseRecord.requests.push(r);
        }
        phaseRecord.serverRequests=serverRecords.filter(r=>r.phase===phase);phaseRecord.unexpectedRequests=[...pair.unexpectedEvents];
        console.log(`${repeat}/${width}/${scheme}/${phase}: requests=${phaseRecord.requests.length}, server=${phaseRecord.serverRequests.length}`);
      }
      pair.manifestAfter=sha(JSON.stringify(manifest()));pair.serverTrace=serverRecords;
      if(serverRecords.some(r=>!['cold','warm'].includes(r.phase)))report.failures.push({code:'server.outsidePhase',detail:origin});
      const expected={schema:1,origin,contextId,browserPid:ownedProcess.pid,manifestSHA,font:{url:origin+fontPath,sha256:sha(font.body),bytes:font.body.length,mime:'font/woff2',cacheControl:'private,max-age=3600,immutable'},allowedPaths:[...resources.keys()],missingBodyError:{name:'Error',command:'Network.getResponseBody',message:contract.cacheProof.warmMissingBodyError.replace(/^Error: /,'')},resources:manifest()};pair.expected=expected;
      if(originalMode){
        const sample=deriveLineageSample(v1,expected,pair,'cold','original-control');report.samples.push(sample);
        report.control={kind:mode,detected:sample.bodyBytes===658700&&sample.failures.length===1&&sample.failures[0].code==='budget.bodyBytes'&&sample.assetFailures.length===0&&sample.unexpectedRequests.length===0,failures:sample.failures};
      }else{
        const result=evaluateLineagePair(v1,contract,expected,pair);pair.result=result;report.samples.push(...result.samples);
        if(mode==='matrix')report.failures.push(...result.failures);
        else{
          const codes=result.lineage.failures.map(f=>f.code);
          const wanted=mode==='cache-clear'?['warm.fontBody','server.warm.count']:['phase.cold.responseHeaders','phase.warm.responseHeaders'];
          const permitted=mode==='cache-clear'?['lineage.warm.fontBody','lineage.server.warm.count','warm.budget.bodyBytes','warm.state.warmCache','warm.assets.failed']:['lineage.phase.cold.responseHeaders','lineage.phase.warm.responseHeaders','cold.assets.failed','warm.assets.failed'];
          report.control={kind:mode,detected:result.lineage.matched===false&&wanted.every(c=>codes.includes(c))&&codes.every(c=>wanted.includes(c))&&result.failures.every(f=>permitted.includes(f.code)),codes,failures:result.failures};
        }
      }
      activePhase=null;await context.close();await closeServer();
    }
    await closeBrowser();
  }
  if(mode==='matrix')report.failures.push(...evaluateMatrix(v1,report.samples));
  else if(report.control?.detected!==true)report.failures.push({code:'control.notDetected',detail:mode});
  for(const input of report.inputs)assert.equal(sha(await read(input.source)),input.sha256,'input drift: '+input.name);
  for(const input of envelope.inputs)assert.equal(sha(await read(path.join(dir,input.name))),input.sha256,'runner drift: '+input.name);
  assert.equal(await hashFile(executablePath),report.browserExecutable.sha256,'browser drift');assert.equal(sha(await read(pw)),contract.playwrightEntrySHA256,'Playwright entry drift');
}catch(e){report.failures.push({code:'execution',detail:String(e.stack||e)})}
finally{
  clearTimeout(deadline);await closeBrowser();try{await closeServer()}catch(e){report.failures.push({code:'server.cleanup',detail:String(e)})}
  report.finishedAt=new Date().toISOString();report.executionPassed=report.failures.length===0&&report.servers.length>0&&report.servers.every(s=>s.closed)&&report.browserProcesses.length>0&&report.browserProcesses.every(p=>p.exitCode===0&&p.signalCode===null);
  report.passed=mode==='matrix'&&report.executionPassed;await write(path.join(root,'receipt.json'),JSON.stringify(report,null,2)+'\n');
}
console.log(JSON.stringify({root,passed:report.passed,executionPassed:report.executionPassed,control:report.control?.detected,pairs:report.pairs.length,failures:report.failures},null,2));process.exitCode=report.executionPassed?0:1;
