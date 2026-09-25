import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {trackOwnedChild,terminateOwned} from '../bar-resolution-2026-09-22/browser-owned_process.mjs';
const [manifestPath,output]=process.argv.slice(2);assert([manifestPath,output].every(x=>x?.startsWith('/')));
const manifest=JSON.parse(await fs.readFile(manifestPath,'utf8')),I=manifest.inputs,sha=b=>createHash('sha256').update(b).digest('hex');
const report={variant:manifest.variant,startedAt:new Date().toISOString(),checks:[],cells:[],errors:[],unexpectedRequests:[],cleanup:{},timedOut:false};
const check=(ok,name,detail={})=>{report.checks.push({ok,name,detail});if(!ok)report.errors.push({name,detail});};
const bound=async(p,ms,name)=>{let timer;try{return await Promise.race([p,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(name+' timeout')),ms);})]);}finally{clearTimeout(timer);}};
const delay=ms=>new Promise(r=>setTimeout(r,ms));
async function settle(region){const start=performance.now();let last=await region.evaluate(e=>e.scrollLeft),stable=start;while(performance.now()-start<1000){await delay(25);const x=await region.evaluate(e=>e.scrollLeft),now=performance.now();if(Math.abs(x-last)>0.1){last=x;stable=now;}if(now-stable>=100)return {left:x,elapsed:now-start};}throw Error('native scroll did not settle within 1000ms');}
async function metrics(region){return region.evaluate(e=>{
 const box=r=>({left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:r.width,height:r.height}),b=e.getBoundingClientRect();
 const rects=cell=>{const range=document.createRange();range.selectNodeContents(cell);return [...range.getClientRects()];};
 const visible=rs=>rs.length>0&&rs.every(x=>x.left>=b.left-1&&x.right<=b.right+1&&x.top>=0&&x.bottom<=innerHeight);
 return {region:box(b),left:e.scrollLeft,clientWidth:e.clientWidth,scrollWidth:e.scrollWidth,rootFont:parseFloat(getComputedStyle(document.documentElement).fontSize),rows:[...e.querySelectorAll('tbody tr')].map(row=>{
  const cell=row.children[3],r=cell.getBoundingClientRect(),rs=rects(cell);
  return {height:row.getBoundingClientRect().height,text:cell.textContent,cell:box(r),textContained:rs.length>0&&rs.every(x=>x.left>=r.left-1&&x.right<=r.right+1&&x.top>=r.top-1&&x.bottom<=r.bottom+1),textVisible:visible(rs),labelTextVisible:visible(rects(row.children[0]))};
 })};
});}
await fs.mkdir(output,{mode:0o700});let server,browser,owned;
const watchdog=setTimeout(()=>{report.timedOut=true;try{owned?.child.kill('SIGTERM');}catch{}},90000);
try {
 for(const input of Object.values(I)){assert(input.path.startsWith('/')&&/^[0-9a-f]{64}$/.test(input.sha256));assert.equal(sha(await fs.readFile(input.path)),input.sha256);}
 assert.equal(fileURLToPath(import.meta.url),I.driver.path);assert.equal(fileURLToPath(new URL('../bar-resolution-2026-09-22/browser-owned_process.mjs',import.meta.url)),I.helper.path);assert.equal(process.execPath,I.node.path);assert.equal(process.version,manifest.node_version);
 const pkg=JSON.parse(await fs.readFile(new URL('package.json',pathToFileURL(I.pwPath.path)),'utf8'));assert.equal(pkg.version,manifest.pw_version);
 const responses=JSON.parse(await fs.readFile(I.responses.path,'utf8')),{chromium}=await import(pathToFileURL(I.pwPath.path).href);
 server=await chromium.launchServer({executablePath:I.chromePath.path,headless:true,args:['--no-proxy-server','--disable-background-networking']});owned=trackOwnedChild(server.process(),{name:'chrome'});browser=await chromium.connect(server.wsEndpoint());assert.equal(browser.version(),manifest.chrome_version);
 for(const lane of [{name:'missing-320-light',width:320,theme:'light',missing:true},{name:'missing-390-dark',width:390,theme:'dark',missing:true},{name:'missing-1440-light',width:1440,theme:'light',missing:true},{name:'complete-390-dark',width:390,theme:'dark',missing:false}]) {
  let context;const query=lane.missing?`?category=api&compare=none&period=2026-08&state=missing&theme=${lane.theme}`:`?category=all&compare=none&period=2026-09&state=complete&theme=${lane.theme}`,entry='/analytics/recipe'+query;
  try {
   context=await browser.newContext({viewport:{width:lane.width,height:1000},colorScheme:lane.theme,javaScriptEnabled:false,serviceWorkers:'block'});context.setDefaultTimeout(5000);const requests=[];
   await context.route('**/*',route=>{const r=route.request(),u=new URL(r.url()),response=responses[u.pathname+u.search],allowed=r.method()==='GET'&&u.origin==='http://127.0.0.1'&&!!response;requests.push({url:r.url(),allowed});if(!allowed){report.unexpectedRequests.push({lane:lane.name,url:r.url()});return route.abort('blockedbyclient');}return route.fulfill({status:response.Status,headers:Object.fromEntries(Object.entries(response.Headers).map(([k,v])=>[k.toLowerCase(),v.join(', ')])),body:response.Body});});
   const page=await context.newPage(),styleReady=page.waitForResponse(r=>new URL(r.url()).pathname==='/static/app.css'),doc=await page.goto('http://127.0.0.1'+entry,{waitUntil:'load'}),style=await styleReady;
   await Promise.all([doc.finished(),style.finished(),bound(page.evaluate(()=>document.fonts.ready),3000,'fonts')]);
   check(sha(await doc.body())===sha(responses[entry].Body)&&sha(await style.body())===sha(responses['/static/app.css'].Body),`${lane.name} exact completed document/CSS`);
   check(Buffer.byteLength(responses[entry].Body)<=32768&&await page.locator('script').count()===0,`${lane.name} scriptless payload budget`,{bytes:Buffer.byteLength(responses[entry].Body)});
   const region=page.locator('.ak-vizframe [role="region"]').filter({has:page.locator('table')});assert.equal(await region.count(),1);
   const initial=await metrics(region),rows=await region.locator('tbody tr').evaluateAll(rs=>rs.map(r=>[...r.children].map(c=>c.textContent)));
   const labels=lane.missing?['Gateway API','Search API']:['Gateway API','Search API','Report worker'];
   check(JSON.stringify(rows.map(r=>r[0]))===JSON.stringify(labels)&&JSON.stringify(rows.map(r=>r[1]))===JSON.stringify(lane.missing?['Missing','Missing']:['4','1','0']),`${lane.name} exact labels/values`,rows);
   check(JSON.stringify(initial.rows.map(r=>r.text))===JSON.stringify(lane.missing?labels.map(n=>`No source report for ${n} in 2026-08; missing is not zero.`):['Present','Present','Present']),`${lane.name} exact state explanations`);
   check(await region.locator('tbody th a').count()===labels.length,`${lane.name} existing detail links retained`);
   check(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${lane.name} no page-level overflow`);
   if(lane.missing){check(initial.rows.every(r=>r.cell.width>=192),`${lane.name} reason width >=192px`,initial);check(initial.rows.every(r=>r.height<=128),`${lane.name} bounded fixture row height`,initial);}
   let tabs=0;while(tabs++<80){await page.keyboard.press('Tab');if(await region.evaluate(e=>e===document.activeElement))break;}
   check(await region.evaluate(e=>e===document.activeElement&&getComputedStyle(e).outlineStyle!=='none'),`${lane.name} native scroll-region focus`,{tabs});
   const before=await metrics(region),start=performance.now();await delay(250);const idle=await metrics(region),elapsed=performance.now()-start;
   check(elapsed>=200&&Math.abs(idle.left-before.left)<0.1,`${lane.name} no-input scroll control`,{elapsed,before:before.left,after:idle.left});
   let reached=idle,presses=0;const visible=m=>m.rows.every(r=>r.cell.left>=m.region.left-1&&r.cell.right<=m.region.right+1);
   while(!visible(reached)&&presses<30){await page.keyboard.press('ArrowRight');await settle(region);presses++;reached=await metrics(region);if(reached.left===reached.scrollWidth-reached.clientWidth&&!visible(reached))break;}
   check(visible(reached)&&(!presses||reached.left>idle.left),`${lane.name} native scroll reaches State`,{presses,reached});
   check(reached.rows.every(r=>r.textContained&&r.textVisible),`${lane.name} every complete explanation readable after scroll`,reached.rows);
   await page.screenshot({path:`${output}/${lane.name}-reason.png`,fullPage:true});
   let returned=reached,leftPresses=0;while(returned.left>0.1&&leftPresses<30){await page.keyboard.press('ArrowLeft');await settle(region);leftPresses++;returned=await metrics(region);}
   check(returned.left<=0.1&&returned.rows.every(r=>r.labelTextVisible),`${lane.name} native return to visible row labels`,{leftPresses,returned});
   await page.keyboard.press('Tab');check(await region.locator('tbody th a').first().evaluate(e=>e===document.activeElement),`${lane.name} first detail link follows table in Tab order`);
   report.cells.push({lane,requests,initial,idle,reached,returned,rows});
  }finally{if(context)await bound(context.close(),5000,'context close');}
 }
}catch(e){report.errors.push({name:'run error',detail:{error:String(e),stack:e?.stack}});}
finally {
 try{if(browser)await bound(browser.close(),5000,'browser close');}catch(e){report.cleanup.browserCloseError=String(e);}try{if(server)await bound(server.close(),5000,'server close');}catch(e){report.cleanup.serverCloseError=String(e);}if(owned)report.cleanup.chrome={termination:await terminateOwned(owned,{graceMs:3000,killMs:2000,drainMs:2000}),lifecycle:owned.state};clearTimeout(watchdog);
 report.endedAt=new Date().toISOString();report.summary={checks:report.checks.length,failures:report.errors.length,cells:report.cells.length};report.pass=!report.timedOut&&report.errors.length===0&&report.unexpectedRequests.length===0&&report.cells.length===4&&report.cleanup.chrome?.termination?.ok&&!report.cleanup.browserCloseError&&!report.cleanup.serverCloseError;await fs.writeFile(`${output}/report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx'});console.log(JSON.stringify({pass:report.pass,summary:report.summary,errors:report.errors,cleanup:report.cleanup.chrome?.termination}));process.exitCode=report.pass?0:1;
}
