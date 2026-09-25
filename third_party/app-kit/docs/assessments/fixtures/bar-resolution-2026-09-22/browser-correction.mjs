import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {spawn} from 'node:child_process';
import {pathToFileURL} from 'node:url';
import {trackOwnedChild, terminateOwned} from './browser-owned_process.mjs';

const [baselineBin, candidateBin, cssPath, pwPath, chromePath, output] = process.argv.slice(2);
if (![baselineBin, candidateBin, cssPath, pwPath, chromePath, output].every(p => p?.startsWith('/'))) throw Error('six absolute arguments required');
const sha = b => createHash('sha256').update(b).digest('hex');
const startedAt = new Date().toISOString();
const report = {startedAt, identities:{}, checks:[], failures:[], cells:[], cleanup:{}, timedOut:false};
const check = (ok, name, detail={}) => { const item={ok,name,detail}; report.checks.push(item); if(!ok) report.failures.push(item); };
const expectedCSP = "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'";
const expectedTable = kind => [
  ['Small','1','Not provided','Present','Measured quantity'],
  ['Large',kind==='spread'?'1,000,000,000,000':'10','Not provided','Present','Measured quantity'],
  ['Zero','0','Not provided','Present','Measured quantity'],
  ['Missing','Missing','Not provided','Not observed','Measured quantity'],
];
const csvRows = text => {
  const rows=[]; let row=[], field='', quoted=false;
  for(let i=0;i<text.length;i++){const c=text[i]; if(quoted){if(c==='"'&&text[i+1]==='"'){field+='"';i++;}else if(c==='"')quoted=false;else field+=c;}else if(c==='"')quoted=true;else if(c===','){row.push(field);field='';}else if(c==='\n'){row.push(field.replace(/\r$/,''));rows.push(row);row=[];field='';}else field+=c;}
  if(field||row.length){row.push(field);rows.push(row);} return rows;
};
const expectedData = kind => [
  ['small','Small','1','items','present','','','','accent','Measured quantity'],
  ['large','Large',kind==='spread'?'1000000000000':'10','items','present','','','','accent','Measured quantity'],
  ['zero','Zero','0','items','present','','','','accent','Measured quantity'],
  ['missing','Missing','','items','missing','Not observed','','','accent','Measured quantity'],
];

async function identity(path){const b=await fs.readFile(path);return {path,size:b.length,sha256:sha(b)};}
async function startServer(name, bin){
  const cp=spawn(bin,[cssPath],{stdio:['ignore','pipe','pipe'],detached:false});
  const owned=trackOwnedChild(cp,{name}); let stdout='',stderr=''; cp.stdout.on('data',b=>stdout+=b); cp.stderr.on('data',b=>stderr+=b);
  const origin=await new Promise((resolve,reject)=>{let timer=setTimeout(()=>reject(Error(`${name} READY timeout`)),5000);cp.stdout.on('data',()=>{const m=stdout.match(/READY (http:\/\/127\.0\.0\.1:\d+)/);if(m){clearTimeout(timer);resolve(m[1]);}});cp.once('exit',(c,s)=>{clearTimeout(timer);reject(Error(`${name} exited ${c}/${s}: ${stderr}`));});});
  return {name,bin,owned,origin,get stdout(){return stdout;},get stderr(){return stderr;}};
}
async function refused(origin){try{await fetch(origin+'/?case=spread&theme=light',{signal:AbortSignal.timeout(800)});return false;}catch{return true;}}

await fs.mkdir(output,{recursive:false,mode:0o700});
let browserServer,browser,browserOwned; const servers=[]; let runError;
const watchdog=setTimeout(()=>{report.timedOut=true;for(const s of servers)try{s.owned.child.kill('SIGTERM');}catch{}try{browserOwned?.child.kill('SIGTERM');}catch{}},175000);
try {
  for(const [k,p] of Object.entries({baselineBin,candidateBin,cssPath,pwPath,chromePath,driver:new URL(import.meta.url).pathname,helper:new URL('./browser-owned_process.mjs',import.meta.url).pathname})) report.identities[k]=await identity(p);
  const pkg=JSON.parse(await fs.readFile(new URL('package.json',pathToFileURL(pwPath)),'utf8')); report.playwrightVersion=pkg.version;
  assert.equal(report.playwrightVersion,'1.62.1');
  const {chromium}=await import(pathToFileURL(pwPath).href);
  browserServer=await chromium.launchServer({executablePath:chromePath,headless:true,args:['--no-proxy-server','--disable-background-networking']});
  browserOwned=trackOwnedChild(browserServer.process(),{name:'chrome'}); browser=await chromium.connect(browserServer.wsEndpoint());
  report.chromeVersion=browser.version(); assert.equal(report.chromeVersion,'153.0.8010.53');
  for(const [variant,bin] of [['baseline',baselineBin],['candidate',candidateBin]]) servers.push(await startServer(variant,bin));
  const csvBytes={};
  for(const server of servers) for(const lane of [{name:'wide-light',width:1440,height:1000,theme:'light'},{name:'narrow-dark',width:390,height:844,theme:'dark'}]) for(const kind of ['spread','readable']){
    const id=`${server.name}-${lane.name}-${kind}`, requests=[],responses=[],consoleErrors=[];
    const context=await browser.newContext({viewport:{width:lane.width,height:lane.height},colorScheme:lane.theme,javaScriptEnabled:false,serviceWorkers:'block',acceptDownloads:false});
    context.setDefaultTimeout(8000); const page=await context.newPage();
    page.on('request',r=>requests.push({url:r.url(),method:r.method(),type:r.resourceType()}));
    page.on('response',r=>responses.push({url:r.url(),status:r.status(),type:r.request().resourceType()}));
    page.on('console',m=>{if(m.type()==='error')consoleErrors.push(m.text());}); page.on('pageerror',e=>consoleErrors.push(String(e)));
    const cssPromise=page.waitForResponse(r=>new URL(r.url()).pathname==='/app.css');
    const doc=await page.goto(`${server.origin}/?case=${kind}&theme=${lane.theme}`,{waitUntil:'load'}); const css=await cssPromise;
    await Promise.all([doc.finished(),css.finished()]); const docBody=await doc.body(),cssBody=await css.body();
    check(doc.status()===200&&doc.headers()['content-type']?.startsWith('text/html'),`${id} document complete`,{status:doc.status(),bytes:docBody.length,sha256:sha(docBody)});
    check(css.status()===200&&sha(cssBody)===report.identities.cssPath.sha256,`${id} exact CSS complete`,{status:css.status(),bytes:cssBody.length,sha256:sha(cssBody)});
    check(doc.headers()['content-security-policy']===expectedCSP&&await page.locator('script').count()===0,`${id} no script restrictive CSP`,doc.headers()['content-security-policy']);
    check(requests.every(r=>new URL(r.url).origin===server.origin)&&responses.every(r=>r.status<400)&&consoleErrors.length===0,`${id} local clean transport`,{requests,responses,consoleErrors});
    const table=await page.locator('tbody tr').evaluateAll(rows=>rows.map(r=>[...r.children].map(c=>c.textContent.trim())));
    check(JSON.stringify(table)===JSON.stringify(expectedTable(kind)),`${id} exact table truth`,table);
    const chart=page.locator('svg[aria-label="Synthetic value comparison (items)"]'),legend=page.locator('[aria-label="Chart legend"]');
    if(kind==='spread'){
      const warning=(await page.locator('body').innerText()).includes('one or more positive values would render smaller than one logical pixel at this scale');
      check(await chart.count()===0&&await legend.count()===0&&warning,`${id} spread semantic fallback`,{chart:await chart.count(),legend:await legend.count(),warning});
    } else {
      check(await chart.count()===1&&await legend.count()===1,`${id} readable real chart`,{chart:await chart.count(),legend:await legend.count()});
      const chartText=await chart.textContent(); const fills=await chart.locator('rect[fill="var(--ak-color-accent)"]').count();
      check(chartText.includes('Small')&&chartText.includes('Large')&&chartText.includes('Zero')&&!chartText.includes('Missing')&&fills===2,`${id} zero and missing chart semantics`,{chartText,fills});
    }
    const csvResponse=await context.request.get(`${server.origin}/export.csv?case=${kind}&theme=${lane.theme}`); const csv=Buffer.from(await csvResponse.body());
    check(csvResponse.status()===200&&csvResponse.headers()['content-type']?.startsWith('text/csv'),`${id} CSV response`,{status:csvResponse.status(),sha256:sha(csv),bytes:csv.length});
    const parsed=csvRows(csv.toString()); const header=parsed.findIndex(r=>r.join(',')==='id,label,value,unit,state,missing_reason,lower,upper,role,role_label');
    check(header>=0&&JSON.stringify(parsed.slice(header+1))===JSON.stringify(expectedData(kind)),`${id} independent CSV rows`,header>=0?parsed.slice(header+1):parsed);
    csvBytes[`${server.name}-${kind}`]=csv;
    if(lane.width===390){const region=page.getByRole('region',{name:/data table, horizontally scrollable/});await region.evaluate(e=>e.scrollLeft=0);let reached=false;for(let i=0;i<25;i++){await page.keyboard.press('Tab');if(await region.evaluate(e=>document.activeElement===e)){reached=true;break;}}const before=await region.evaluate(e=>e.scrollLeft);if(reached)await page.keyboard.press('ArrowRight');const state=await region.evaluate(e=>({after:e.scrollLeft,overflow:e.scrollWidth>e.clientWidth,outline:parseFloat(getComputedStyle(e).outlineWidth)}));check(reached&&state.overflow&&state.outline>=2&&state.after>before,`${id} native table keyboard scroll`,{reached,before,...state});}
    await page.screenshot({path:`${output}/${id}.png`,fullPage:true}); report.cells.push({id,documentSha256:sha(docBody),cssSha256:sha(cssBody),csvSha256:sha(csv),requests,responses}); await context.close();
  }
  for(const kind of ['spread','readable']) check(Buffer.compare(csvBytes[`baseline-${kind}`],csvBytes[`candidate-${kind}`])===0,`${kind} CSV baseline candidate byte identity`,{baseline:sha(csvBytes[`baseline-${kind}`]),candidate:sha(csvBytes[`candidate-${kind}`])});
} catch(error){runError=error;report.errors=[String(error),error?.stack];}
finally{
  for(const server of servers.reverse()){server.termination=await terminateOwned(server.owned,{graceMs:2000,killMs:2000,drainMs:2000});server.refusedAfter=await refused(server.origin);report.cleanup[server.name]={termination:server.termination,lifecycle:server.owned.state,stdout:server.stdout,stderr:server.stderr,refusedAfter:server.refusedAfter};}
  try{if(browser)await browser.close();}catch(e){report.cleanup.browserCloseError=String(e);} try{if(browserServer)await browserServer.close();}catch(e){report.cleanup.browserServerCloseError=String(e);} if(browserOwned)report.cleanup.chrome={termination:await terminateOwned(browserOwned,{graceMs:3000,killMs:2000,drainMs:2000}),lifecycle:browserOwned.state};
  clearTimeout(watchdog); report.endedAt=new Date().toISOString(); report.runError=runError?String(runError):null;
  const expectedBaseline=report.failures.filter(x=>x.name.startsWith('baseline-')&&x.name.endsWith('spread semantic fallback'));
  const unexpected=report.failures.filter(x=>!expectedBaseline.includes(x)); report.summary={checks:report.checks.length,failures:report.failures.length,expectedBaselineSemanticFailures:expectedBaseline.length,unexpectedFailures:unexpected.length,cells:report.cells.length};
  const clean=servers.length===2&&servers.every(s=>s.termination?.ok&&s.refusedAfter)&&report.cleanup.chrome?.termination?.ok&&!report.cleanup.browserCloseError&&!report.cleanup.browserServerCloseError;
  report.pass=!runError&&!report.timedOut&&report.cells.length===8&&expectedBaseline.length===2&&unexpected.length===0&&clean;
  await fs.writeFile(`${output}/report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx'});
}
process.exitCode=report.pass?0:1;
