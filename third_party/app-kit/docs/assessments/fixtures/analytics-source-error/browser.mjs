// Assessment-only browser evidence over source-collected responses; no live server.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {trackOwnedChild,terminateOwned} from '../bar-resolution-2026-09-22/browser-owned_process.mjs';

const [packet]=process.argv.slice(2);assert(packet?.startsWith('/'),'usage: node browser.mjs ABSOLUTE_PACKET');
const output=`${packet}/browser-candidate-corrected`,helper=fileURLToPath(new URL('../bar-resolution-2026-09-22/browser-owned_process.mjs',import.meta.url));
const paths={driver:fileURLToPath(import.meta.url),helper,node:process.execPath,
  playwright:'/Users/geoff/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs',
  chrome:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',responses:`${packet}/responses.json`,
  collector:`${packet}/collector`,collectorSource:`${packet}/collector.go`,sourceArchive:`${packet}/candidate.tar`};
const sha=b=>createHash('sha256').update(b).digest('hex'),canonical=value=>{const u=new URL(value,'http://127.0.0.1');u.searchParams.sort();return u.pathname+(u.search?`?${u.searchParams}`:'');};
const bound=async(p,ms,name)=>{let timer;try{return await Promise.race([p,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(`${name} timeout`)),ms);})]);}finally{clearTimeout(timer);}};
const snapshot=async()=>{const out={};for(const [name,path] of Object.entries(paths)){const b=await fs.readFile(path);out[name]={path,bytes:b.length,sha256:sha(b)};}return out;};
const report={schema:'appkit-gallery-analytics-source-error-browser-v1',startedAt:new Date().toISOString(),checks:[],observations:[],responses:[],errors:[],unexpectedRequests:[],downloads:[],cleanup:{},timedOut:false,
  limits:['Synthetic Gallery fixture only; no live source, persistence, consumer adoption, dropdown-key choice, production correctness, or human acceptance claim.']};
const check=(ok,name,detail={})=>{const item={ok:Boolean(ok),name,detail};report.checks.push(item);if(!ok)report.errors.push(item);};
const headers=h=>Object.fromEntries(Object.entries(h).map(([k,v])=>[k.toLowerCase(),v.join(', ')]));
const header=(h,n)=>Object.entries(h).find(([k])=>k.toLowerCase()===n)?.[1]?.join(', ');
const focus=link=>link.evaluate(e=>{const r=e.getBoundingClientRect(),s=getComputedStyle(e);return{active:e===document.activeElement,width:r.width,height:r.height,visible:r.top>=0&&r.bottom<=innerHeight&&r.left>=0&&r.right<=innerWidth,outline:s.outlineStyle,outlineWidth:parseFloat(s.outlineWidth)}});

await fs.mkdir(output,{recursive:false,mode:0o700});
const frozen=JSON.parse(await fs.readFile(paths.responses,'utf8'));assert.equal(frozen.schema,'appkit-gallery-analytics-source-error-v1');assert.equal(frozen.lanes.length,2);
const retained=new Map(frozen.responses.map(r=>[canonical(r.path),r]));
report.inputHashesBefore=await snapshot();
assert.equal(process.version,'v26.3.0');const pw=JSON.parse(await fs.readFile(new URL('package.json',pathToFileURL(paths.playwright))));assert.equal(pw.version,'1.62.1');
let server,browser,owned,runError;const watchdog=setTimeout(()=>{report.timedOut=true;try{owned?.child.kill('SIGTERM')}catch{}},120000);

async function navigation(page,path,want,id,action=()=>page.goto(`http://127.0.0.1${path}`,{waitUntil:'load'})){
  const key=canonical(path),expected=retained.get(key);assert(expected,`missing retained ${key}`);
  const doc=page.waitForResponse(r=>r.request().isNavigationRequest()&&canonical(r.url())===key),css=page.waitForResponse(r=>new URL(r.url()).pathname==='/static/app.css');
  const [response,cssResponse]=await bound(Promise.all([doc,css,action()]),12000,`${id} navigation`);await Promise.all([response.finished(),cssResponse.finished()]);
  const [body,cssBody]=await Promise.all([response.body(),cssResponse.body()]),expectedCSS=retained.get('/static/app.css');
  check(response.status()===want&&sha(body)===sha(Buffer.from(expected.body)),`${id} exact completed document`,{status:response.status(),bytes:body.length});
  check(cssResponse.status()===200&&sha(cssBody)===sha(Buffer.from(expectedCSS.body)),`${id} exact completed CSS`,{status:cssResponse.status(),bytes:cssBody.length});
  check(body.length<=32768,`${id} HTML <=32KiB`,{bytes:body.length});report.responses.push({id,path:key,status:response.status(),sha256:sha(body)});return response;
}

async function errorPage(page,lane,path,recovery,id){
  await navigation(page,path,503,id);const main=page.locator('main'),text=await main.textContent();
  check(await page.locator('#analytics-source-error').count()===1&&await page.getByRole('heading',{name:'The data source could not be read',exact:true}).count()===1,`${id} explicit source error`);
  check(/synthetic/i.test(text)&&/no live (?:data|service)/i.test(text),`${id} truthful synthetic/no-live-source explanation`);
  const selected=new URL(path,'http://127.0.0.1').searchParams,period=lane.period==='2026-09'?'September 2026':'August 2026',group=lane.category==='api'?'API services':'empty fixture group',comparison=selected.get('compare')==='previous'?'Previous calendar month':'No comparison';
  check(text.includes(period)&&text.includes(group)&&text.includes(comparison),`${id} explicit period group and comparison context`,{period,group,comparison});
  check(await main.locator('figure,table').count()===0&&await main.locator('a[href*="export.csv"],a[href^="/analytics/"]:not([href^="/analytics/recipe"])').count()===0,`${id} no figures tables detail or export links`);
  check(await page.locator('script,iframe').count()===0&&await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${id} scriptless and no overflow`);
  check(await page.locator('html').getAttribute('data-theme')===lane.theme&&canonical(page.url())===canonical(path),`${id} exact theme and selection URL`,{url:page.url()});
  const link=page.getByRole('link',{name:'Load the complete fixture',exact:true});check(await link.count()===1&&await link.getAttribute('href')===recovery,`${id} recovery changes only state`);
  return link;
}

async function nativeRecover(page,link,destination,id){
  let reached=false;for(let i=0;i<160;i++){if(await link.evaluate(e=>e===document.activeElement)){reached=true;break}await page.keyboard.press('Tab')}
  const g=await focus(link);check(reached&&g.active&&g.width>=44&&g.height>=44&&g.visible&&g.outline!=='none'&&g.outlineWidth>=2,`${id} native Tab visible 44px recovery`,g);
  await navigation(page,destination,200,`${id}-complete`,()=>page.keyboard.press('Enter'));check(canonical(page.url())===canonical(destination),`${id} native Enter exact recovery`,{url:page.url()});
}

async function cell(lane,surface){
  const id=`${surface}-${lane.name}`,p=lane.paths,error=surface==='overview'?p.overviewError:p.recipeError,complete=surface==='overview'?p.overviewComplete:p.recipeComplete;
  const exportError=surface==='overview'?p.overviewExportError:p.recipeExportError,requests=[];let context;
  try{
    context=await browser.newContext({viewport:{width:lane.width,height:lane.height},colorScheme:lane.theme,javaScriptEnabled:false,serviceWorkers:'block',acceptDownloads:true});context.setDefaultTimeout(5000);
    await context.route('**/*',route=>{const q=route.request(),u=new URL(q.url()),item=retained.get(canonical(q.url())),allowed=q.method()==='GET'&&u.origin==='http://127.0.0.1'&&Boolean(item);requests.push({url:q.url(),method:q.method(),allowed});if(!allowed){report.unexpectedRequests.push({id,url:q.url(),method:q.method()});return route.abort('blockedbyclient')}return route.fulfill({status:item.status,headers:headers(item.header),body:item.body})});
    const page=await context.newPage();page.on('download',d=>report.downloads.push({id,url:d.url()}));page.on('pageerror',e=>report.errors.push({name:`${id} pageerror`,error:String(e)}));page.on('console',m=>{if(m.type()==='error'&&!m.text().includes('503'))report.errors.push({name:`${id} console`,text:m.text()})});page.on('websocket',s=>report.unexpectedRequests.push({id,websocket:s.url()}));
    const link=await errorPage(page,lane,error,complete,id);await page.screenshot({path:`${output}/${id}-error.png`,fullPage:true});await nativeRecover(page,link,complete,id);
    check(await page.locator('#analytics-source-error').count()===0,`${id} complete recovery leaves error state`);await page.screenshot({path:`${output}/${id}-complete.png`,fullPage:true});
    await navigation(page,error,503,`${id}-back`,()=>page.goBack({waitUntil:'load'}));check(canonical(page.url())===canonical(error),`${id} Back restores source error`);
    const before=report.downloads.length,response=await navigation(page,exportError,503,`${id}-csv-error`);check(header(retained.get(canonical(exportError)).header,'content-type')?.startsWith('text/html')&&!header(retained.get(canonical(exportError)).header,'content-disposition')&&report.downloads.length===before,`${id} CSV route is nonattachment HTML 503`);
    check(response.headers()['content-type']?.startsWith('text/html'),`${id} browser observes HTML export error`);await errorPage(page,lane,exportError,complete,`${id}-csv-page`);
    check(requests.every(r=>r.allowed),`${id} only retained same-origin GETs`,{requests});report.observations.push({id,lane:lane.name,surface,requests});
  }finally{if(context)await bound(context.close(),5000,`${id} context close`).catch(e=>report.errors.push({name:`${id} context close`,error:String(e)}))}
}

try{
  const {chromium}=await import(pathToFileURL(paths.playwright).href);server=await bound(chromium.launchServer({executablePath:paths.chrome,headless:true,chromiumSandbox:true,args:['--no-proxy-server','--disable-background-networking'],timeout:15000}),15000,'Chrome launch');owned=trackOwnedChild(server.process(),{name:'chrome'});browser=await bound(chromium.connect(server.wsEndpoint(),{timeout:10000}),10000,'Chrome connect');assert.equal(browser.version(),'153.0.8010.53');report.versions={node:process.version,playwright:pw.version,chrome:browser.version()};
  report.inputHashesAtAcquisition=await snapshot();check(JSON.stringify(report.inputHashesBefore)===JSON.stringify(report.inputHashesAtAcquisition),'frozen inputs unchanged before acquisition');
  for(const lane of frozen.lanes)for(const surface of ['overview','recipe'])await bound(cell(lane,surface),25000,`${surface}-${lane.name}`);
}catch(e){runError=e;report.errors.push({name:'run error',error:String(e),stack:e?.stack})}
finally{
  try{if(browser)await bound(browser.close(),5000,'browser close')}catch(e){report.cleanup.browserCloseError=String(e)}try{if(server)await bound(server.close(),5000,'server close')}catch(e){report.cleanup.serverCloseError=String(e)}
  if(owned)try{report.cleanup.chrome={termination:await terminateOwned(owned,{graceMs:3000,killMs:2000,drainMs:2000}),lifecycle:owned.state}}catch(e){report.cleanup.chromeError=String(e)}clearTimeout(watchdog);
  try{report.inputHashesAfter=await snapshot();check(JSON.stringify(report.inputHashesBefore)===JSON.stringify(report.inputHashesAfter),'all frozen inputs unchanged after acquisition')}catch(e){report.errors.push({name:'input hash after',error:String(e)})}
  report.endedAt=new Date().toISOString();report.summary={checks:report.checks.length,errors:report.errors.length,observations:report.observations.length,unexpectedRequests:report.unexpectedRequests.length,downloads:report.downloads.length};const cleanup=report.cleanup.chrome?.termination?.ok&&!report.cleanup.browserCloseError&&!report.cleanup.serverCloseError&&!report.cleanup.chromeError;
  report.pass=!runError&&!report.timedOut&&report.errors.length===0&&report.unexpectedRequests.length===0&&report.downloads.length===0&&report.observations.length===4&&cleanup;report.taskAcceptance=report.pass;await fs.writeFile(`${output}/report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx'});console.log(JSON.stringify({pass:report.pass,summary:report.summary}));process.exitCode=report.pass?0:1;
}
