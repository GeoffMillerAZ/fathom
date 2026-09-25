// Private diagnostic, not a benchmark or a certification oracle.
import fs from 'node:fs/promises';
import path from 'node:path';
import http from 'node:http';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {spawn} from 'node:child_process';
import {fileURLToPath,pathToFileURL} from 'node:url';
const sha=b=>createHash('sha256').update(b).digest('hex');
const write=(p,b)=>fs.writeFile(p,b,{flag:'wx',mode:0o600});
const bounded=async(p,ms,label)=>{let t;try{return await Promise.race([p,new Promise((_,r)=>{t=setTimeout(()=>r(new Error(label+' timeout')),ms)})])}finally{clearTimeout(t)}};
const self=fileURLToPath(import.meta.url),args=process.argv.slice(2);
if(args[0]!=='--retained'){
  assert.ok(args.length===3||(args.length===4&&args[3]==='fetch'),'usage: ABS_PERFORMANCE_RECEIPT SHA256 ABS_PRIVATE_PARENT [fetch]');
  assert.ok(path.isAbsolute(args[0])&&path.isAbsolute(args[2])&&/^[a-f0-9]{64}$/.test(args[1]));
  const parent=await fs.realpath(args[2]),root=await fs.mkdtemp(path.join(parent,'cache-probe-'));
  await fs.chmod(root,0o700);const source=await fs.readFile(self);
  await write(path.join(root,'probe.mjs'),source);
  await write(path.join(root,'snapshot.json'),JSON.stringify({source:self,sha256:sha(source),args,node:process.version},null,2)+'\n');
  console.log(root);
  const child=spawn(process.execPath,[path.join(root,'probe.mjs'),'--retained',root,...args],{stdio:['ignore','pipe','pipe']});
  let out='',err='',forced=false,kill;
  const stop=()=>{if(forced)return;forced=true;child.kill('SIGTERM');kill=setTimeout(()=>child.kill('SIGKILL'),20000)};
  const deadline=setTimeout(stop,150000);
  child.stdout.on('data',b=>{out+=b;if(out.length>1024*1024)stop()});child.stderr.on('data',b=>{err+=b;if(err.length>1024*1024)stop()});
  const end=await new Promise(resolve=>{child.once('error',e=>{err+=String(e)});child.once('close',(code,signal)=>resolve({code,signal}))});
  clearTimeout(deadline);clearTimeout(kill);
  await write(path.join(root,'stdout.log'),out);await write(path.join(root,'stderr.log'),err);
  assert.equal(sha(await fs.readFile(self)),sha(source),'source drift');
  await write(path.join(root,'launcher.json'),JSON.stringify({...end,forced,pid:child.pid,stdoutSHA:sha(out),stderrSHA:sha(err)},null,2)+'\n');
  console.log(out);if(err)console.error(err);process.exitCode=end.code===0&&!forced&&!end.signal?0:1;
}else{
  const [,root,receiptPath,receiptSHA,,instrument]=args;assert.equal(path.dirname(self),root);
  const snapshot=JSON.parse(await fs.readFile(path.join(root,'snapshot.json')));assert.equal(sha(await fs.readFile(self)),snapshot.sha256);
  const report={kind:'private-request-bound-cache-diagnostic-v3',instrument:instrument??'network',startedAt:new Date().toISOString(),snapshotSHA:snapshot.sha256,inputReceiptSHA:receiptSHA,resources:[],serverRequests:[],interceptions:[],lanes:[],failures:[]};
  let server,owned,browser,proc,base,activeLane,closing,aborted=false;
  const close=()=>closing??=(async()=>{
    if(owned){try{await bounded(owned.close(),10000,'browser close')}catch(e){report.failures.push(String(e));await bounded(owned.kill(),5000,'browser kill').catch(e=>{report.failures.push(String(e));if(proc.exitCode===null&&proc.signalCode===null)proc.kill('SIGKILL')})}
      report.browserExit={pid:proc.pid,exitCode:proc.exitCode,signal:proc.signalCode};if(proc.exitCode!==0||proc.signalCode!==null)report.failures.push('abnormal browser exit');}
    if(server){server.closeAllConnections();await bounded(new Promise(r=>server.close(r)),3000,'server close');report.serverClosed=!server.listening;}
  })();
  const deadline=setTimeout(()=>{aborted=true;report.failures.push('watchdog');close().catch(e=>report.failures.push(String(e)))},120000);
  process.once('SIGTERM',()=>{aborted=true;report.failures.push('SIGTERM');close().catch(e=>report.failures.push(String(e)))});
  try{
    const receiptBytes=await fs.readFile(receiptPath);assert.equal(sha(receiptBytes),receiptSHA);await write(path.join(root,'input-receipt.json'),receiptBytes);
    const prior=JSON.parse(receiptBytes),priorRoot=path.dirname(receiptPath),first=prior.samples.find(s=>s.phase==='cold');
    assert.equal(prior.deliveryProfile,'separate-font-subset-candidate');assert.equal(first.requestGraph.length,7);
    const resources=new Map();await fs.mkdir(path.join(root,'inputs'),{mode:0o700});
    for(const graph of first.requestGraph){
      const source=graph.path==='/enhanced'?path.join(priorRoot,'document.html'):path.join(priorRoot,'inputs',graph.path==='/source.css'?'font-candidate/source.css':graph.path.slice(1));
      const data=await fs.readFile(source);assert.equal(sha(data),graph.expectedSHA256);assert.equal(data.length,graph.expectedBodyBytes);
      const name=graph.path.slice(1).replaceAll('/','__');await write(path.join(root,'inputs',name),data);
      resources.set(graph.path,{body:data,type:graph.mime});report.resources.push({path:graph.path,source,name,sha256:sha(data),bytes:data.length});
    }
    const pw=process.env.APPKIT_PLAYWRIGHT_MODULE;assert.ok(pw&&path.isAbsolute(pw));assert.equal(sha(await fs.readFile(pw)),prior.inputs.find(i=>i.name==='playwright-index.mjs').sha256);
    const executable=prior.browserExecutable.path;assert.equal(sha(await fs.readFile(executable)),prior.browserExecutable.sha256);report.browserExecutable=prior.browserExecutable;
    server=http.createServer((req,res)=>{
      if(req.method!=='GET'||req.headers.host!==new URL(base).host||!resources.has(req.url)){report.failures.push('unexpected server request '+req.url);res.writeHead(403).end();return}
      const item=resources.get(req.url);report.serverRequests.push({lane:activeLane,path:req.url,bytes:item.body.length});
      res.writeHead(200,{'Content-Type':item.type,'Content-Length':item.body.length,'Content-Security-Policy':prior.instrumentation.csp,'X-Content-Type-Options':'nosniff','Cache-Control':req.url==='/enhanced'?'no-store':'private,max-age=3600,immutable'}).end(item.body);
    });
    await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve)});base='http://127.0.0.1:'+server.address().port;report.origin=base;
    const {chromium}=await import(pathToFileURL(pw));owned=await chromium.launchServer({headless:true,executablePath:executable,args:['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1']});proc=owned.process();
    await write(path.join(root,'browser-owned.json'),JSON.stringify({pid:proc.pid,executable},null,2)+'\n');browser=await chromium.connect(owned.wsEndpoint());assert.equal(browser.version(),prior.browser);
    const modes=instrument==='fetch'?[{id:'fetch-always',enable:{},fetch:'always'},{id:'fetch-warm-only',enable:{},fetch:'warm'}]:[{id:'default-throttled',enable:{}},{id:'configured-early-stream',enable:{maxTotalBufferSize:32*1024*1024,maxResourceBufferSize:8*1024*1024},stream:true}];report.modes=modes;
    report.network={offline:false,latency:80,downloadThroughput:1250000,uploadThroughput:625000,connectionType:'cellular4g'};
    for(const mode of modes){
      if(aborted)throw new Error('aborted');
      const ctx=await browser.newContext({viewport:{width:390,height:1000},colorScheme:'light',reducedMotion:'reduce',deviceScaleFactor:1,serviceWorkers:'block'});
      const page=await ctx.newPage(),cdp=await ctx.newCDPSession(page);page.setDefaultTimeout(5000);
      await cdp.send('Network.enable',mode.enable);await cdp.send('Network.clearBrowserCache');await cdp.send('Network.setCacheDisabled',{cacheDisabled:false});
      if(mode.stream)await cdp.send('Network.configureDurableMessages',mode.enable);
      await cdp.send('Network.emulateNetworkConditions',report.network);
      let observed=new Map(),pending=[];
      const probeBody=async(r,label)=>{try{const result=await cdp.send('Network.getResponseBody',{requestId:r.id}),body=Buffer.from(result.body,result.base64Encoded?'base64':'utf8');r[label]={bytes:body.length,sha256:sha(body),match:sha(body)===sha(resources.get(r.path)?.body??'')};await write(path.join(root,activeLane+'-'+r.id.replaceAll('.','_')+'-'+label+'.body'),body)}catch(e){r[label]={error:String(e)}}};
      cdp.on('Network.requestWillBeSent',e=>{
        const u=new URL(e.request.url);if(u.origin!==base||!resources.has(u.pathname)||u.search||e.request.method!=='GET')report.failures.push('unexpected browser request '+e.request.url);
        const r={id:e.requestId,path:u.pathname,type:e.type,frameId:e.frameId,loaderId:e.loaderId,cache:false,chunks:[]};observed.set(e.requestId,r);
        if(mode.stream&&r.path==='/assets/inter-subset.woff2'){
          r.streamCommandAt='requestWillBeSent';
          pending.push((async()=>{try{const s=await cdp.send('Network.streamResourceContent',{requestId:r.id});r.buffered=s.bufferedData;r.streamAccepted=true;r.streamReplyWhen=r.finished?'after-finish':r.status?'after-response':'before-response'}catch(e){r.streamError=String(e)}})());
        }
      });
      cdp.on('Network.requestServedFromCache',e=>{const r=observed.get(e.requestId);if(r)r.cache=true});
      cdp.on('Network.responseReceived',e=>{
        const r=observed.get(e.requestId);if(!r)return;Object.assign(r,{status:e.response.status,cache:r.cache||!!e.response.fromDiskCache,fromDiskCache:!!e.response.fromDiskCache,fromServiceWorker:!!e.response.fromServiceWorker});
      });
      cdp.on('Network.dataReceived',e=>{const r=observed.get(e.requestId);if(r){r.events=(r.events??0)+1;r.dataLength=(r.dataLength??0)+e.dataLength;if(e.data)r.chunks.push(e.data)}});
      cdp.on('Network.loadingFinished',e=>{const r=observed.get(e.requestId);if(r){r.finished=true;r.encodedDataLength=e.encodedDataLength;if(!mode.stream)pending.push(probeBody(r,'atFinish'))}});
      cdp.on('Network.loadingFailed',e=>{const r=observed.get(e.requestId);if(r)r.error=e.errorText});
      if(mode.fetch)cdp.on('Fetch.requestPaused',e=>{
        const lane=activeLane,event={lane,fetchId:e.requestId,networkId:e.networkId,frameId:e.frameId,url:e.request.url,status:e.responseStatusCode,paused:e};
        report.interceptions.push(event);
        pending.push((async()=>{
          try{
            assert.equal(e.request.url,base+'/assets/inter-subset.woff2');assert.equal(e.request.method,'GET');
            assert.equal(e.resourceType,'Font');assert.ok(e.networkId);
            // The protocol also emits error-stage responses without HTTP metadata.
            // Record those as unavailable, never as bytes or an inferred cache miss.
            // Cross-domain Network correlation is checked after navigation settles.
            if(e.responseErrorReason!==undefined){
              assert.equal(e.responseStatusCode,undefined);assert.ok(typeof e.responseErrorReason==='string'&&e.responseErrorReason.length>0);
              event.body={availability:'unavailable-response-error',reason:e.responseErrorReason};return;
            }
            assert.equal(e.responseStatusCode,200);
            const result=await bounded(cdp.send('Fetch.getResponseBody',{requestId:e.requestId}),5000,'paused body');
            const body=Buffer.from(result.body,result.base64Encoded?'base64':'utf8');
            event.body={availability:'observed',bytes:body.length,sha256:sha(body),match:sha(body)===sha(resources.get('/assets/inter-subset.woff2').body)};
            await write(path.join(root,lane+'-fetch-'+e.requestId.replaceAll(/[^a-zA-Z0-9_-]/g,'_')+'.body'),body);
          }catch(err){event.error=String(err);report.failures.push('Fetch probe: '+String(err))}
          finally{try{await bounded(cdp.send('Fetch.continueResponse',{requestId:e.requestId}),5000,'continue response');event.continued=true}catch(err){event.continueError=String(err);report.failures.push('Fetch continuation: '+String(err))}}
        })());
      });
      for(const phase of ['cold','warm']){
        if(aborted)throw new Error('aborted');activeLane=mode.id+'-'+phase;observed=new Map();pending=[];
        if(mode.fetch&&(phase==='warm'||mode.fetch==='always')){
          await cdp.send('Fetch.enable',{patterns:[{urlPattern:base+'/assets/inter-subset.woff2',resourceType:'Font',requestStage:'Response'}]});
          // Raw CDP only: Playwright routing separately disables the cache.
          // Retain the original cache control and prove actual cache/server facts.
          await cdp.send('Network.setCacheDisabled',{cacheDisabled:false});
        }
        await page.goto(base+'/enhanced',{waitUntil:'load',timeout:15000});await bounded(page.evaluate(async()=>{await document.fonts.ready;await Promise.all([...document.images].map(i=>i.decode()));await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))}),10000,'font readiness');
        await bounded(Promise.all(pending),10000,'request probes');
        for(const event of report.interceptions.filter(e=>e.lane===activeLane)){
          const r=observed.get(event.networkId);assert.ok(r,'paused response lacks a settled Network request');
          assert.equal(event.url,base+r.path);assert.equal(r.type,'Font');assert.equal(event.frameId,r.frameId);assert.ok(r.loaderId);
          event.linkedLoaderId=r.loaderId;
          if(event.body?.availability==='observed'){
            assert.equal(r.fetch,undefined,'duplicate body-bearing interception for Network request');assert.equal(event.body.match,true);r.fetch=event;
          }else{(r.fetchUnavailable??=[]).push(event)}
        }
        if(mode.fetch==='always'&&phase==='cold')assert.equal(report.interceptions.filter(e=>e.lane===activeLane&&e.body?.availability==='observed').length,1,'missing cold Fetch positive control');
        for(const r of observed.values()){
          if(mode.stream&&r.streamAccepted){const data=Buffer.concat([Buffer.from(r.buffered??'','base64'),...r.chunks.map(b=>Buffer.from(b,'base64'))]);r.stream={bytes:data.length,sha256:sha(data),match:sha(data)===sha(resources.get(r.path)?.body??''),chunks:r.chunks.length};await write(path.join(root,activeLane+'-'+r.id.replaceAll('.','_')+'-stream.body'),data)}
          await probeBody(r,'afterLoad');
          delete r.buffered;delete r.chunks;
        }
        const lane={id:activeLane,phase,requests:[...observed.values()],serverRequests:report.serverRequests.filter(r=>r.lane===activeLane)};
        lane.cacheHits=lane.requests.filter(r=>r.cache).length;report.lanes.push(lane);
        const document=lane.requests.find(r=>r.type==='Document');
        assert.ok(document?.frameId&&document?.loaderId,'missing navigation identity');
        assert.ok(lane.requests.every(r=>r.frameId===document.frameId&&r.loaderId===document.loaderId),'resource navigation identity differs');
        assert.equal(lane.requests.length,7);assert.equal(lane.cacheHits,phase==='cold'?0:6);assert.equal(lane.serverRequests.length,phase==='cold'?7:1);
        assert.ok(lane.requests.every(r=>r.status===200&&r.finished&&!r.error&&!r.fromServiceWorker));
        console.log(JSON.stringify({lane:activeLane,cacheHits:lane.cacheHits,font:lane.requests.find(r=>r.type==='Font')}));
      }
      await ctx.close();
    }
    assert.equal(sha(await fs.readFile(self)),snapshot.sha256);assert.equal(sha(await fs.readFile(receiptPath)),receiptSHA);assert.equal(sha(await fs.readFile(executable)),prior.browserExecutable.sha256);
    for(const item of report.resources){assert.equal(sha(await fs.readFile(item.source)),item.sha256);assert.equal(sha(await fs.readFile(path.join(root,'inputs',item.name))),item.sha256)}
  }catch(e){report.failures.push(String(e.stack||e))}
  finally{clearTimeout(deadline);await close().catch(e=>report.failures.push(String(e)));report.finishedAt=new Date().toISOString();report.completed=report.failures.length===0&&report.lanes.length===4&&report.serverClosed===true;await write(path.join(root,'receipt.json'),JSON.stringify(report,null,2)+'\n')}
  console.log(JSON.stringify({root,completed:report.completed,failures:report.failures}));process.exitCode=report.completed?0:1;
}
