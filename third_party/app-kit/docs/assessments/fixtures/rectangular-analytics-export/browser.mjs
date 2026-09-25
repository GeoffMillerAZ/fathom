import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {trackOwnedChild,terminateOwned} from '../bar-resolution-2026-09-22/browser-owned_process.mjs';
const [manifestPath,output]=process.argv.slice(2);assert([manifestPath,output].every(x=>x?.startsWith('/')));
const manifest=JSON.parse(await fs.readFile(manifestPath,'utf8')),I=manifest.inputs,sha=b=>createHash('sha256').update(b).digest('hex');
const report={startedAt:new Date().toISOString(),checks:[],cells:[],errors:[],unexpectedRequests:[],cleanup:{},timedOut:false};
const check=(ok,name,detail={})=>{report.checks.push({ok,name,detail});if(!ok)report.errors.push({name,detail});};
const bound=async(p,ms,name)=>{let timer;try{return await Promise.race([p,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(name+' timeout')),ms);})]);}finally{clearTimeout(timer);}};
await fs.mkdir(output,{mode:0o700});let server,browser,owned;
const watchdog=setTimeout(()=>{report.timedOut=true;try{owned?.child.kill('SIGTERM');}catch{}},90000);
try {
 for(const input of Object.values(I)){assert(input.path.startsWith('/')&&/^[0-9a-f]{64}$/.test(input.sha256));assert.equal(sha(await fs.readFile(input.path)),input.sha256);}
 assert.equal(fileURLToPath(import.meta.url),I.driver.path);assert.equal(fileURLToPath(new URL('../bar-resolution-2026-09-22/browser-owned_process.mjs',import.meta.url)),I.helper.path);assert.equal(process.execPath,I.node.path);assert.equal(process.version,manifest.node_version);
 const pkg=JSON.parse(await fs.readFile(new URL('package.json',pathToFileURL(I.pwPath.path)),'utf8'));assert.equal(pkg.version,manifest.pw_version);
 const responses=Object.fromEntries(JSON.parse(await fs.readFile(I.responses.path,'utf8')).map(x=>[x.Path,x]));
 const lookup=url=>{const u=new URL(url);return Object.values(responses).find(x=>{const v=new URL(x.Path,'http://127.0.0.1');v.searchParams.sort();u.searchParams.sort();return v.pathname+v.search===u.pathname+u.search;});};
 const {chromium}=await import(pathToFileURL(I.pwPath.path).href);
 server=await chromium.launchServer({executablePath:I.chromePath.path,headless:true,args:['--no-proxy-server','--disable-background-networking']});owned=trackOwnedChild(server.process(),{name:'chrome'});browser=await chromium.connect(server.wsEndpoint());assert.equal(browser.version(),manifest.chrome_version);
 for(const lane of [{name:'wide-light',width:1440,theme:'light'},{name:'narrow-dark',width:390,theme:'dark'}]) {
  for(const state of ['complete','missing']) {
   let context;const query=`?period=2026-09&category=all&state=${state}&theme=${lane.theme}&compare=none`,entry='/analytics/recipe'+query;
   try {
    context=await browser.newContext({viewport:{width:lane.width,height:1000},colorScheme:lane.theme,javaScriptEnabled:false,serviceWorkers:'block',acceptDownloads:true});context.setDefaultTimeout(5000);const requests=[];
    await context.route('**/*',route=>{const r=route.request(),u=new URL(r.url()),response=lookup(r.url()),allowed=r.method()==='GET'&&u.origin==='http://127.0.0.1'&&!!response;requests.push({url:r.url(),allowed});if(!allowed){report.unexpectedRequests.push({lane:lane.name,state,url:r.url()});return route.abort('blockedbyclient');}return route.fulfill({status:response.Status,headers:Object.fromEntries(Object.entries(response.Header).map(([k,v])=>[k.toLowerCase(),v.join(', ')])),body:response.Body});});
    const page=await context.newPage(),styleReady=page.waitForResponse(r=>new URL(r.url()).pathname==='/static/app.css'),doc=await page.goto('http://127.0.0.1'+entry,{waitUntil:'load'}),style=await styleReady;
    await Promise.all([doc.finished(),style.finished(),bound(page.evaluate(()=>document.fonts.ready),3000,'fonts')]);
    check(sha(await doc.body())===sha(responses[entry].Body)&&sha(await style.body())===sha(responses['/static/app.css'].Body),`${lane.name}/${state} exact completed document/CSS`);
    check(Buffer.byteLength(responses[entry].Body)<=32768&&await page.locator('script').count()===0,`${lane.name}/${state} scriptless payload budget`,{bytes:Buffer.byteLength(responses[entry].Body)});
    check(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${lane.name}/${state} no page overflow`);
    const table=page.getByRole('link',{name:'Export spreadsheet CSV',exact:true}),legacy=page.getByRole('link',{name:'Export renderer CSV',exact:true});
    check(await table.count()===1&&await legacy.count()===1,`${lane.name}/${state} both export formats discoverable`);
    check(await table.getAttribute('aria-describedby')==='analytics-table-export-help'&&await page.locator('#analytics-table-export-help').isVisible(),`${lane.name}/${state} format explanation associated and visible`);
    const rows=await page.locator('.ak-vizframe table tbody tr').evaluateAll(rs=>rs.map(r=>[...r.children].map(c=>c.textContent)));
    check(JSON.stringify(rows.map(r=>r[0]))===JSON.stringify(['Gateway API','Search API','Report worker'])&&JSON.stringify(rows.map(r=>r[1]))===JSON.stringify(state==='missing'?['Missing','Missing','Missing']:['4','1','0']),`${lane.name}/${state} independently expected visible values`,rows);
    const downloads=[];
    for(const [kind,link,filename] of [['table',table,'service-operations-table-2026-09.csv'],['report',legacy,'service-operations-recipe-2026-09.csv']]) {
     const href=await link.getAttribute('href'),u=new URL(href,'http://127.0.0.1');
     check(u.searchParams.get('period')==='2026-09'&&u.searchParams.get('category')==='all'&&u.searchParams.get('state')===state&&u.searchParams.get('theme')===lane.theme&&u.searchParams.get('compare')==='none',`${lane.name}/${state}/${kind} exact filter context`,{href});
     let tabs=0;while(tabs++<80){await page.keyboard.press('Tab');if(await link.evaluate(e=>e===document.activeElement))break;}
     check(await link.evaluate(e=>e===document.activeElement&&getComputedStyle(e).outlineStyle!=='none'),`${lane.name}/${state}/${kind} native keyboard focus`,{tabs});
     const rect=await link.boundingBox();check(rect&&rect.height>=44&&rect.x>=0&&rect.x+rect.width<=lane.width+1,`${lane.name}/${state}/${kind} visible target size`,rect);
     const pending=page.waitForEvent('download');await page.keyboard.press('Enter');const download=await pending;
     const target=`${output}/${lane.name}-${state}-${kind}.csv`;await download.saveAs(target);
     const bytes=await fs.readFile(target),expected=lookup(download.url());
     check(download.suggestedFilename()===filename&&expected?.Status===200&&sha(bytes)===sha(expected.Body)&&!await download.failure(),`${lane.name}/${state}/${kind} native download exact bytes`,{filename:download.suggestedFilename(),sha256:sha(bytes),bytes:bytes.length});
     check(page.url()==='http://127.0.0.1'+entry,`${lane.name}/${state}/${kind} download retains report`);
     downloads.push({kind,href,path:target,sha256:sha(bytes)});
    }
    await page.keyboard.press('Home');await page.screenshot({path:`${output}/${lane.name}-${state}.png`,fullPage:true});
    report.cells.push({lane,state,requests,downloads,rows});
   }finally{if(context)await bound(context.close(),5000,'context close');}
  }
 }
}catch(e){report.errors.push({name:'run error',detail:{error:String(e),stack:e?.stack}});}
finally {
 try{if(browser)await bound(browser.close(),5000,'browser close');}catch(e){report.cleanup.browserCloseError=String(e);}try{if(server)await bound(server.close(),5000,'server close');}catch(e){report.cleanup.serverCloseError=String(e);}if(owned)report.cleanup.chrome={termination:await terminateOwned(owned,{graceMs:3000,killMs:2000,drainMs:2000}),lifecycle:owned.state};clearTimeout(watchdog);
 report.endedAt=new Date().toISOString();report.summary={checks:report.checks.length,failures:report.errors.length,cells:report.cells.length};report.pass=!report.timedOut&&report.errors.length===0&&report.unexpectedRequests.length===0&&report.cells.length===4&&report.cleanup.chrome?.termination?.ok&&!report.cleanup.browserCloseError&&!report.cleanup.serverCloseError;await fs.writeFile(`${output}/report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx'});console.log(JSON.stringify({pass:report.pass,summary:report.summary,errors:report.errors,cleanup:report.cleanup.chrome?.termination}));process.exitCode=report.pass?0:1;
}
