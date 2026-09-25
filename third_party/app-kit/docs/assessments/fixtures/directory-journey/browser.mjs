// Synthetic native-GET task through the actual privately generated P04 renderer.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {spawn} from 'node:child_process';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {expectedRows,expectedSelection,checkObservation} from './oracle.mjs';
import {parseCSV} from './csv.mjs';

const [buildArg]=process.argv.slice(2);assert.ok(path.isAbsolute(buildArg));
const buildRoot=await fs.realpath(buildArg),fixture=path.dirname(fileURLToPath(import.meta.url));
const sha=b=>createHash('sha256').update(b).digest('hex');
const write=(name,b)=>fs.writeFile(path.join(root,name),b,{flag:'wx',mode:0o600});
const buildBytes=await fs.readFile(path.join(buildRoot,'build-receipt.json')),build=JSON.parse(buildBytes);
assert.equal(build.passed,true);assert.equal(build.root,buildRoot);
const binary=path.join(buildRoot,'directory-journey');assert.equal(build.binary.path,binary);
assert.equal(sha(await fs.readFile(binary)),build.binary.sha256);
const root=await fs.mkdtemp(path.join(buildRoot,'browser-'));await fs.chmod(root,0o700);
const report={kind:'p04-directory-journey-browser-v1',verification:'not-certified',root,startedAt:new Date().toISOString(),buildReceiptSHA:sha(buildBytes),binary:build.binary,sources:{},checks:[],pages:[],downloads:[],requests:[],requestFailures:[],failures:[],limits:['Synthetic app-owned data/task, not real consumer adoption or target certification.','390/1440 light/dark in local Chrome; no screen-reader, physical touch, zoom or full performance claim.','Zero application JavaScript; timings are observations without a new latency threshold.']};
const check=(name,passed,detail)=>{report.checks.push({name,passed,detail});if(!passed)report.failures.push({name,detail})};
async function bounded(promise,ms,label){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' timed out')),ms)})])}finally{clearTimeout(timer)}}
for(const name of ['browser.mjs','oracle.mjs','oracle_test.mjs','csv.mjs','csv_test.mjs']){
  const b=await fs.readFile(path.join(fixture,name));await write(name,b);report.sources[name]=sha(b);
}
await write('build-receipt.json',buildBytes);
const datasetBytes=await fs.readFile(path.join(buildRoot,'dataset.json')),dataset=JSON.parse(datasetBytes);
assert.equal(sha(datasetBytes),build.datasetSHA);assert.deepEqual(dataset.Rows,expectedRows);
assert.equal(dataset.Revision,sha(JSON.stringify(expectedRows)));await write('dataset.json',datasetBytes);
const pw=process.env.APPKIT_PLAYWRIGHT_MODULE;assert.ok(pw&&path.isAbsolute(pw));
report.playwrightEntrySHA=sha(await fs.readFile(pw));assert.equal(report.playwrightEntrySHA,'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
const {chromium}=await import(pathToFileURL(pw));
let host,hostClosed,stdout='',stderr='',browserServer,browserProc,browserExit,browser,origin;
const watchdog=setTimeout(()=>{report.failures.push({name:'watchdog',detail:'180 seconds'});host?.kill('SIGKILL');browserServer?.kill().catch(()=>{})},180000);
async function tabTo(page,selector){
  for(let i=0;i<100;i++){if(await page.evaluate(s=>document.activeElement?.matches(s)===true,selector))return;await page.keyboard.press('Tab')}
  throw Error('native Tab did not reach '+selector);
}
try{
  host=spawn(binary,[],{cwd:buildRoot,stdio:['ignore','pipe','pipe']});
  hostClosed=new Promise(resolve=>host.once('close',resolve));host.stderr.on('data',b=>{stderr+=b.toString()});
  const ready=new Promise((resolve,reject)=>{host.once('error',reject);host.once('exit',()=>reject(Error('host exited before readiness')));host.stdout.on('data',b=>{stdout+=b.toString();const m=stdout.match(/directory-journey → (http:\/\/127\.0\.0\.1:\d+)/);if(m)resolve(m[1])})});
  origin=await bounded(ready,10000,'host readiness');report.origin=origin;
  // Bind actual served assets, independent of browser-request timing.
  report.assets={};
  for(const name of ['style','journey']){
    const r=await fetch(`${origin}/assets/${name}.css`,{signal:AbortSignal.timeout(5000)}),b=Buffer.from(await r.arrayBuffer());
    assert.equal(r.status,200);assert.match(r.headers.get('content-type'),/^text\/css/);
    await write(`${name}.css`,b);report.assets[name]={sha256:sha(b),bytes:b.length};
  }
  assert.equal(report.assets.style.sha256,build.css.sha256);
  browserServer=await chromium.launchServer({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',args:['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1']});
  browserProc=browserServer.process();browserExit=new Promise(resolve=>browserProc.once('exit',resolve));browser=await chromium.connect(browserServer.wsEndpoint());
  report.browserVersion=browser.version();assert.equal(report.browserVersion,'152.0.7977.84');
  for(const width of [390,1440])for(const theme of ['light','dark']){
    const context=await browser.newContext({viewport:{width,height:1000},colorScheme:theme,reducedMotion:'reduce',javaScriptEnabled:false,serviceWorkers:'block',acceptDownloads:true});
    await context.route('**/*',route=>{
      const r=route.request(),u=new URL(r.url());
      const allowed=u.origin===origin&&r.method()==='GET'&&(/^\/directory(?:\.csv|\/[a-z0-9]+)?$/.test(u.pathname)||/^\/assets\/(?:style|journey)\.css$/.test(u.pathname));
      report.requests.push({width,theme,url:r.url(),type:r.resourceType(),allowed});
      if(allowed)return route.continue();check('unexpected browser request',false,r.url());return route.abort();
    });
    const page=await context.newPage();page.setDefaultTimeout(6000);
    let documents=[],styles=[],downloadInFlight=null;
    page.on('pageerror',e=>check('page error',false,String(e)));
    page.on('requestfailed',r=>{report.requestFailures.push({width,theme,url:r.url(),error:r.failure(),downloadURL:downloadInFlight})});
    page.on('response',r=>{if(r.request().resourceType()==='document')documents.push(r);if(/\/assets\/(style|journey)\.css$/.test(r.url()))styles.push(r)});
    const target=(route,selection={})=>{const {id,...query}=selection;return origin+route+'?'+new URLSearchParams({...query,theme})};
    async function observe(label,selection,expectedTheme=theme){
      await page.waitForLoadState('load');
      await page.evaluate(()=>document.fonts.ready);
      const response=documents.at(-1);assert.ok(response);assert.equal(response.url(),page.url());
      const body=await response.body();const expected=expectedSelection({...selection,datasetRevision:dataset.Revision});
      const measured=await page.evaluate(()=>{
        const status=document.querySelector('#directory-status');
        const coverage=status?.dataset.coverage??document.querySelector('[data-coverage]')?.getAttribute('data-coverage')??null;
        const visible=e=>e.checkVisibility()&&!e.closest('.sr-only');
        const direct=e=>[...e.childNodes].filter(n=>n.nodeType===Node.TEXT_NODE).map(n=>n.textContent.trim()).filter(Boolean);
        const rows=[...document.querySelectorAll('tbody tr')].map(row=>({
          ids:[...row.querySelectorAll('a')].map(a=>new URL(a.href).pathname.match(/^\/directory\/(p\d\d)$/)?.[1]).filter(Boolean),
          texts:[...row.querySelectorAll('*')].filter(visible).flatMap(direct),
        }));
        const dl=document.querySelector('dl[data-record-id]');let detail;
        if(dl){const back=new URL(document.querySelector('#directory-back').href);detail={ID:dl.dataset.recordId};for(const field of ['Name','Title','Email','Role'])detail[field]=dl.querySelector(`[data-field="${field}"]`)?.textContent;Object.assign(detail,{backQuery:back.searchParams.get('q')||'',backPage:Number(back.searchParams.get('page')||1),backState:back.searchParams.get('state')||'complete'})}
        const controls=[...document.querySelectorAll('main a,main button,main input:not([type=hidden])')].filter(visible).map(e=>{const r=e.getBoundingClientRect();return {text:(e.textContent||e.id).trim(),width:r.width,height:r.height}});
        // Canvas normalizes the source's oklch colors; no screenshot baseline changes.
        const canvas=document.createElement('canvas');canvas.width=canvas.height=1;const context=canvas.getContext('2d',{willReadFrequently:true});
        const rgba=value=>{context.clearRect(0,0,1,1);context.fillStyle=value;context.fillRect(0,0,1,1);const p=[...context.getImageData(0,0,1,1).data];p[3]/=255;return p};
        const over=(a,b)=>{const alpha=a[3]+b[3]*(1-a[3]);return [(a[0]*a[3]+b[0]*b[3]*(1-a[3]))/alpha,(a[1]*a[3]+b[1]*b[3]*(1-a[3]))/alpha,(a[2]*a[3]+b[2]*b[3]*(1-a[3]))/alpha,alpha]};
        const luminance=c=>{const linear=v=>(v/=255)<=.04045?v/12.92:((v+.055)/1.055)**2.4;return .2126*linear(c[0])+.7152*linear(c[1])+.0722*linear(c[2])};
        const ratio=(a,b)=>(Math.max(luminance(a),luminance(b))+.05)/(Math.min(luminance(a),luminance(b))+.05);
        const background=e=>{let result=[255,255,255,1],chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);for(const n of chain){const s=getComputedStyle(n);if(+s.opacity!==1||s.backgroundImage!=='none')return null;result=over(rgba(s.backgroundColor),result)}return result};
        const samples=[...document.querySelectorAll('#directory-status,.journey-filters label,.journey-control,dl dt,dl dd,.journey-error p')].filter(visible).map(e=>{const s=getComputedStyle(e),bg=background(e);return {text:(e.textContent||e.id).trim(),ratio:bg?ratio(over(rgba(s.color),bg),bg):null}});
        const input=document.querySelector('#directory-query');let inputBoundary=null;
        if(input){const s=getComputedStyle(input),bg=background(input);inputBoundary={width:parseFloat(s.borderTopWidth),style:s.borderTopStyle,ratio:bg?ratio(over(rgba(s.borderTopColor),bg),bg):null}}
        return {ids:rows.flatMap(r=>r.ids),rows,pageCount:status?Number(status.dataset.pageCount):null,matchingCount:status?Number(status.dataset.matchingCount):null,coverage,...(detail?{detail}:{}),statusText:status?.textContent??null,revision:document.querySelector('meta[name="dataset-revision"]')?.content,theme:document.documentElement.className,controls,contrast:samples,inputBoundary,scrollWidth:document.documentElement.scrollWidth,viewport:innerWidth,scripts:document.scripts.length,text:document.body.innerText,query:input?.value??null};
      });
      const actual={status:response.status(),ids:measured.ids,pageCount:measured.pageCount,matchingCount:measured.matchingCount,coverage:measured.coverage,...(measured.detail?{detail:measured.detail}:{})};
      const failures=checkObservation(expected,actual);check('independent selection '+label,failures.length===0,{width,theme,failures,actual});
      check('HTML bound and zero script '+label,body.length<=1048576&&measured.scripts===0,{width,theme,bytes:body.length,scripts:measured.scripts});
      check('restrictive script CSP '+label,(response.headers()['content-security-policy']||'').includes("script-src 'none'"),{width,theme});
      check('dataset metadata '+label,measured.revision===dataset.Revision,{width,theme,revision:measured.revision});
      check('no horizontal page overflow '+label,measured.scrollWidth<=measured.viewport,{width,theme,scrollWidth:measured.scrollWidth});
      check('action inventory '+label,measured.controls.length>0,{width,theme});
      for(const c of measured.controls)check('44px action '+label,c.width>=44&&c.height>=44,{width,theme,...c});
      check('wrapper text contrast '+label,measured.contrast.length>0&&measured.contrast.every(s=>s.ratio!==null&&s.ratio>=4.5),{width,theme,samples:measured.contrast});
      if(measured.inputBoundary)check('visible input boundary '+label,measured.inputBoundary.width>=1&&measured.inputBoundary.style!=='none'&&measured.inputBoundary.ratio>=3,{width,theme,...measured.inputBoundary});
      if(actual.status===200){
        check('scheme retained '+label,measured.theme===expectedTheme,{width,theme,expectedTheme,actual:measured.theme});
        if(!selection.id)check('query retained '+label,measured.query===expected.q,{width,theme,actual:measured.query});
        for(const row of measured.rows){const want=expectedRows.find(r=>r.ID===row.ids[0]);check('visible complete row '+label,!!want&&['Name','Title','Email','Role'].every(k=>row.texts.includes(want[k])),{width,theme,...row})}
        if(selection.state==='partial')check('partial uncertainty visible '+label,/total coverage is unknown/i.test(measured.text)&&!measured.text.includes('12'),{width,theme,text:measured.text});
      }else check('error has recovery and no false table '+label,measured.rows.length===0&&actual.matchingCount===null&&await page.locator('a[href^="/directory"]').count()>0,{width,theme});
      assert.equal(styles.length,2,'both exact stylesheet responses required for '+label);
      for(const style of styles){const name=new URL(style.url()).pathname.split('/').at(-1).split('.')[0],b=await style.body();check('served stylesheet identity '+label,style.status()===200&&sha(b)===report.assets[name].sha256,{width,theme,name,sha256:sha(b)})}
      const stem=`${width}-${theme}-${label}`,png=await page.screenshot({fullPage:true,animations:'disabled'});
      await write(stem+'.png',png);await write(stem+'.html',body);
      report.pages.push({label,width,theme,url:page.url(),status:response.status(),observation:actual,measured,htmlSHA:sha(body),image:stem+'.png',imageSHA:sha(png),imageBytes:png.length});
      return actual;
    }
    async function goto(label,selection={},route='/directory'){
      styles=[];await page.goto(target(route,selection),{waitUntil:'load'});return observe(label,selection);
    }
    async function keyNavigate(selector,predicate){
      await tabTo(page,selector);check('native focus visible',await page.evaluate(()=>{const s=getComputedStyle(document.activeElement);return s.outlineStyle!=='none'&&parseFloat(s.outlineWidth)>=2}),{width,theme,selector});
      styles=[];const pending=page.waitForURL(predicate);await page.keyboard.press('Enter');await pending;await page.waitForLoadState('load');
    }
    async function csv(label,selection,actual){
      const href=await page.locator('#directory-export').getAttribute('href'),url=new URL(href,origin).href;
      check('all-match export scope label',/Download all \d+ matching observed records \(CSV\)/.test(await page.locator('#directory-export').innerText()),{width,theme,label});
      await tabTo(page,'#directory-export');downloadInFlight=url;
      const pending=page.waitForEvent('download');await page.keyboard.press('Enter');const download=await pending;
      assert.equal(download.url(),url);assert.equal(await download.failure(),null);
      const filename=`${width}-${theme}-${label}.csv`;await download.saveAs(path.join(root,filename));await fs.chmod(path.join(root,filename),0o600);
      const bytes=await fs.readFile(path.join(root,filename));
      const response=await fetch(url,{signal:AbortSignal.timeout(5000)}),fetched=Buffer.from(await response.arrayBuffer());
      check('completed CSV download identity',response.status===200&&sha(fetched)===sha(bytes)&&/^text\/csv/.test(response.headers.get('content-type')||''),{width,theme,label,status:response.status,sha256:sha(bytes)});
      const [headers,...rows]=parseCSV(bytes.toString('utf8'));
      const failures=checkObservation(expectedSelection({...selection,datasetRevision:dataset.Revision}),{...actual,csv:{headers,rows}});
      check('independent all-match CSV '+label,failures.length===0,{width,theme,failures,rows});
      report.downloads.push({width,theme,label,url,file:filename,sha256:sha(bytes),bytes:bytes.length,status:response.status});downloadInFlight=null;
    }
    await goto('initial');const started=performance.now();
    await tabTo(page,'#directory-query');await page.keyboard.type('engineer');
    await keyNavigate('form button',u=>u.searchParams.get('q')==='engineer');await observe('filtered',{q:'engineer'});
    await keyNavigate('#directory-next',u=>u.searchParams.get('page')==='2');await observe('page2',{q:'engineer',page:2});
    await keyNavigate('tbody a[href^="/directory/p10"]',u=>u.pathname==='/directory/p10');await observe('detail',{q:'engineer',page:2,id:'p10'});
    check('detail Back retains theme',new URL(await page.locator('#directory-back').getAttribute('href'),origin).searchParams.get('theme')===theme,{width,theme});
    await keyNavigate('#directory-back',u=>u.pathname==='/directory');const returned=await observe('returned',{q:'engineer',page:2});
    await csv('engineers',{q:'engineer',page:2},returned);report.checks.push({name:'local task duration',passed:true,detail:{width,theme,milliseconds:performance.now()-started,threshold:null}});
    const last=await goto('quoted-formula',{page:4});await csv('all-records',{page:4},last);
    const partial=await goto('partial',{q:'engineer',state:'partial'});await csv('partial',{q:'engineer',state:'partial'},partial);
    const empty=await goto('empty',{q:'absent-term'});await csv('empty',{q:'absent-term'},empty);
    const formula=await goto('formula-query',{q:'=1'});await csv('formula-query',{q:'=1'},formula);
    await goto('bad-page',{page:0});await keyNavigate('a[href^="/directory"]',u=>u.pathname==='/directory'&&u.searchParams.get('page')!=='0');await observe('recovered',{});
    await goto('unavailable',{state:'unavailable'});
    await goto('export-unavailable',{state:'unavailable'},'/directory.csv');
    await goto('export-invalid-page',{page:999},'/directory.csv');
    await goto('export-invalid-partial-page',{page:3,state:'partial'},'/directory.csv');
    await goto('excluded',{q:'engineer',id:'p02'},'/directory/p02');
    await goto('unknown',{id:'p99'},'/directory/p99');
    await context.close();
  }
  for(const failure of report.requestFailures){
    const proven=report.downloads.some(d=>d.width===failure.width&&d.theme===failure.theme&&d.url===failure.url&&d.url===failure.downloadURL&&d.status===200);
    check('request failure is completed native download',proven&&failure.error?.errorText==='net::ERR_ABORTED',failure);
  }
  check('no application script requests',!report.requests.some(r=>r.type==='script'),{});
  for(const [name,digest]of Object.entries(report.sources))assert.equal(sha(await fs.readFile(path.join(fixture,name))),digest,'fixture changed during probe');
  assert.equal(sha(await fs.readFile(binary)),build.binary.sha256,'binary drift');
}catch(e){report.failures.push({name:'execution',detail:String(e.stack||e)})}
finally{
  if(browserServer){try{await bounded(browserServer.close(),7000,'Chrome close')}catch(e){report.failures.push({name:'browser cleanup',detail:String(e)});await bounded(browserServer.kill(),3000,'Chrome kill').catch(()=>{})}}
  if(browserProc){try{await bounded(browserExit,3000,'Chrome exit')}catch(e){report.failures.push({name:'browser exit wait',detail:String(e)})}report.browserProcess={pid:browserProc.pid,exitCode:browserProc.exitCode,signalCode:browserProc.signalCode};check('owned Chrome clean exit',browserProc.exitCode===0&&browserProc.signalCode===null,report.browserProcess)}
  if(host){host.kill('SIGTERM');try{await bounded(hostClosed,8000,'host close')}catch(e){report.failures.push({name:'host cleanup',detail:String(e)});host.kill('SIGKILL');await bounded(hostClosed,3000,'host kill').catch(()=>{})}report.hostProcess={pid:host.pid,exitCode:host.exitCode,signalCode:host.signalCode,stdoutSHA:sha(stdout),stderrSHA:sha(stderr)};check('owned host clean exit',host.exitCode===0&&host.signalCode===null,report.hostProcess)}
  clearTimeout(watchdog);await write('host.stdout',stdout);await write('host.stderr',stderr);
  report.finishedAt=new Date().toISOString();report.passed=report.failures.length===0;await write('receipt.json',JSON.stringify(report,null,2)+'\n');
}
console.log(JSON.stringify({root,passed:report.passed,checks:report.checks.length,pages:report.pages.length,downloads:report.downloads.length,failures:report.failures},null,2));process.exitCode=report.passed?0:1;
