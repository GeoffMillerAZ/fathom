import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {trackOwnedChild,terminateOwned} from '../bar-resolution-2026-09-22/browser-owned_process.mjs';
const [manifestPath,output]=process.argv.slice(2);
assert([manifestPath,output].every(x=>x?.startsWith('/')));
const manifest=JSON.parse(await fs.readFile(manifestPath,'utf8')),I=manifest.inputs;
const sha=b=>createHash('sha256').update(b).digest('hex');
const report={startedAt:new Date().toISOString(),checks:[],cells:[],errors:[],unexpectedRequests:[],cleanup:{},timedOut:false};
const check=(ok,name,detail={})=>{report.checks.push({ok,name,detail});if(!ok)report.errors.push({name,detail});};
const bound=async(p,ms,name)=>{let timer;try{return await Promise.race([p,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(name+' timeout')),ms);})]);}finally{clearTimeout(timer);}};
const parseCSV=text=>{const rows=[];let row=[],cell='',quoted=false;for(let i=0;i<text.length;i++){const c=text[i];if(c==='"'){if(quoted&&text[i+1]==='"'){cell+='"';i++;}else quoted=!quoted;}else if(!quoted&&(c===','||c==='\n')){row.push(cell.replace(/\r$/,''));cell='';if(c==='\n'){rows.push(row);row=[];}}else cell+=c;}if(cell||row.length){row.push(cell);rows.push(row);}assert(!quoted);return rows;};
await fs.mkdir(output,{recursive:false,mode:0o700});
let server,browser,owned;
const watchdog=setTimeout(()=>{report.timedOut=true;try{owned?.child.kill('SIGTERM');}catch{}},90000);
try {
 for(const input of Object.values(I)){assert(input.path.startsWith('/')&&/^[0-9a-f]{64}$/.test(input.sha256));assert.equal(sha(await fs.readFile(input.path)),input.sha256);}
 assert.equal(fileURLToPath(import.meta.url),I.driver.path);assert.equal(process.execPath,I.node.path);assert.equal(process.version,manifest.node_version);
 assert.equal(fileURLToPath(new URL('../bar-resolution-2026-09-22/browser-owned_process.mjs',import.meta.url)),I.helper.path);
 const pkg=JSON.parse(await fs.readFile(new URL('package.json',pathToFileURL(I.pwPath.path)),'utf8'));assert.equal(pkg.version,manifest.pw_version);
 const responses=JSON.parse(await fs.readFile(I.responses.path,'utf8'));
 const {chromium}=await import(pathToFileURL(I.pwPath.path).href);
 server=await chromium.launchServer({executablePath:I.chromePath.path,headless:true,args:['--no-proxy-server','--disable-background-networking']});owned=trackOwnedChild(server.process(),{name:'chrome'});browser=await chromium.connect(server.wsEndpoint());assert.equal(browser.version(),manifest.chrome_version);report.chromeVersion=browser.version();
 for(const lane of [{name:'wide-light',theme:'light',width:1440,height:1000,period:'2026-09',category:'all',state:'complete',values:['4','1','0']},{name:'narrow-dark',theme:'dark',width:390,height:844,period:'2026-08',category:'api',state:'missing',values:['Missing','Missing']}])for(const kind of ['recipe','edge']) {
  const id=kind+'-'+lane.name,query=`?category=${lane.category}&compare=none&period=${lane.period}&state=${lane.state}&theme=${lane.theme}`,entry=kind==='recipe'?'/analytics/recipe'+query:'/edge?theme='+lane.theme;
  let context;
  try {
   context=await browser.newContext({viewport:{width:lane.width,height:lane.height},colorScheme:lane.theme,javaScriptEnabled:false,serviceWorkers:'block',acceptDownloads:true});context.setDefaultTimeout(5000);
   const requests=[];
   await context.route('**/*',route=>{const r=route.request(),u=new URL(r.url()),response=responses[u.pathname+u.search],allowed=r.method()==='GET'&&u.origin==='http://127.0.0.1'&&!!response;requests.push({url:r.url(),allowed});if(!allowed){report.unexpectedRequests.push({id,url:r.url()});return route.abort('blockedbyclient');}const headers=Object.fromEntries(Object.entries(response.Headers).map(([k,v])=>[k.toLowerCase(),v.join(', ')]));return route.fulfill({status:response.Status,headers,body:response.Body});});
   const page=await context.newPage();
   const complete=async(action,path)=>{const cssReady=page.waitForResponse(r=>new URL(r.url()).pathname==='/static/app.css');const doc=await action(),style=await cssReady;assert(doc);await Promise.all([doc.finished(),style.finished(),bound(page.evaluate(()=>document.fonts.ready),3000,'fonts')]);check(sha(await doc.body())===sha(responses[path].Body)&&sha(await style.body())===sha(responses['/static/app.css'].Body),`${id} exact completed response ${path}`);return doc;};
   await complete(()=>page.goto('http://127.0.0.1'+entry,{waitUntil:'load'}),entry);
   check(await page.locator('script').count()===0&&await page.locator('html').getAttribute('data-theme')===lane.theme,`${id} scriptless theme`);
   check(Buffer.byteLength(responses[entry].Body)<=32768,`${id} existing HTML budget`,{bytes:Buffer.byteLength(responses[entry].Body)});
   const rows=await page.locator('.ak-vizframe tbody tr').evaluateAll(rs=>rs.map(r=>[...r.children].map(c=>c.textContent)));
   const expectedLabels=kind==='recipe'?(lane.category==='all'?['Gateway API','Search API','Report worker']:['Gateway API','Search API']):['Long'.repeat(100),'Zero','Missing observation'];
   check(JSON.stringify(rows.map(r=>r[0]))===JSON.stringify(expectedLabels)&&JSON.stringify(rows.map(r=>r[1]))===JSON.stringify(kind==='recipe'?lane.values:['-12.5','0','Missing']),`${id} exact labels and values`,rows);
   const links=page.locator('.ak-vizframe tbody th a');check(await links.count()===expectedLabels.length,`${id} every app-supplied row link retained`);
   const target=links.first();let tabs=0;while(tabs++<80){await page.keyboard.press('Tab');if(await target.evaluate(e=>e===document.activeElement))break;}
   const geometry=await target.evaluate(e=>{const b=e.getBoundingClientRect(),s=getComputedStyle(e),span=e.firstElementChild,r=document.createRange();r.selectNodeContents(span);const sb=span.getBoundingClientRect();return {active:e===document.activeElement,width:b.width,height:b.height,visible:b.top>=0&&b.bottom<=innerHeight,outline:s.outlineStyle,outlineWidth:parseFloat(s.outlineWidth),underline:s.textDecorationLine,wrap:getComputedStyle(span).overflowWrap,textContained:[...r.getClientRects()].every(x=>x.left>=sb.left-1&&x.right<=sb.right+1)};});
   check(geometry.active&&geometry.width>=44&&geometry.height>=44&&geometry.outline!=='none'&&geometry.outlineWidth>=2&&geometry.underline.includes('underline')&&geometry.visible&&geometry.textContained,`${id} keyboard focus and readable target`,{tabs,geometry});
   check(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${id} no page-level horizontal overflow`);
   await page.screenshot({path:`${output}/${id}.png`,fullPage:true});
   if(kind==='recipe') {
    const detail='/analytics/gateway-api'+query;check(await target.getAttribute('href')===detail,`${id} canonical app-owned destination`);
    await complete(async()=>{const nav=page.waitForNavigation({waitUntil:'load'});await page.keyboard.press('Enter');return await nav;},detail);
    check(page.url()==='http://127.0.0.1'+detail,`${id} native Enter navigation preserves selection/theme`);
    const detailRows=await page.locator('tr[data-value-id="gateway-api"]').evaluateAll(rs=>rs.map(r=>[...r.children].map(c=>c.textContent)));
    check(detailRows.length===1&&detailRows[0][0]==='Gateway API'&&detailRows[0][1]===lane.values[0],`${id} detail retains selected value`,detailRows);
    check((await page.locator('h1').textContent()).includes('Gateway API')&&(await page.locator('main').textContent()).includes('failed operations'),`${id} detail heading and unit`);
    await complete(()=>page.goBack({waitUntil:'load'}),entry);check(page.url()==='http://127.0.0.1'+entry,`${id} browser Back restores exact recipe URL`);
    const exportLink=page.getByRole('link',{name:'Export renderer CSV',exact:true});let n=0;while(n++<100){await page.keyboard.press('Tab');if(await exportLink.evaluate(e=>e===document.activeElement))break;}
    check(await exportLink.evaluate(e=>e===document.activeElement),`${id} export reachable by Tab`,{tabs:n});
    const downloadReady=page.waitForEvent('download');await page.keyboard.press('Enter');const download=await downloadReady;await download.saveAs(`${output}/${id}.csv`);check(await download.failure()===null,`${id} native CSV download`);
    const csv=await fs.readFile(`${output}/${id}.csv`,'utf8'),parsed=parseCSV(csv),header=parsed.findIndex(r=>r[0]==='id'&&r[1]==='label');
    check(csv===responses['/analytics/recipe/export.csv'+query].Body&&header>=0&&parsed[header].length===10,`${id} exact ten-column export`);
    const data=parsed.slice(header+1).filter(r=>r.length>1);check(JSON.stringify(data.map(r=>r[1]))===JSON.stringify(expectedLabels)&&JSON.stringify(data.map(r=>r[2]))===JSON.stringify(lane.values.map(v=>v==='Missing'?'':v))&&data.every(r=>r.length===10&&r[3]==='failed operations'&&r[4]===(lane.state==='missing'?'missing':'present')),`${id} independently expected CSV values/unit/state`,data);
   } else {
    check(await page.locator('.ak-vizframe svg').count()===0&&(await page.locator('main').textContent()).includes('Bar chart omitted'),`${id} long signed label has truthful table-only fallback`);
    check(geometry.wrap==='anywhere'&&geometry.height>44,`${id} full long label wraps`);
    check((await links.nth(1).getAttribute('href'))==='/detail/zero'&&(await links.nth(2).getAttribute('href'))==='/detail/missing',`${id} zero and missing destinations retained`);
   }
   report.cells.push({id,requests,rows,geometry});
  } finally {if(context)await bound(context.close(),5000,'context close');}
 }
}catch(e){report.errors.push({name:'run error',detail:{error:String(e),stack:e?.stack}});}
finally {
 try{if(browser)await bound(browser.close(),5000,'browser close');}catch(e){report.cleanup.browserCloseError=String(e);}try{if(server)await bound(server.close(),5000,'server close');}catch(e){report.cleanup.serverCloseError=String(e);}if(owned)report.cleanup.chrome={termination:await terminateOwned(owned,{graceMs:3000,killMs:2000,drainMs:2000}),lifecycle:owned.state};clearTimeout(watchdog);
 report.endedAt=new Date().toISOString();report.summary={checks:report.checks.length,failures:report.errors.length,cells:report.cells.length};report.pass=!report.timedOut&&report.errors.length===0&&report.unexpectedRequests.length===0&&report.cells.length===4&&report.cleanup.chrome?.termination?.ok&&!report.cleanup.browserCloseError&&!report.cleanup.serverCloseError;await fs.writeFile(`${output}/report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx'});console.log(JSON.stringify({pass:report.pass,summary:report.summary,errors:report.errors,cleanup:report.cleanup.chrome?.termination}));process.exitCode=report.pass?0:1;
}
