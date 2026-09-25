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
 const packet=JSON.parse(await fs.readFile(I.responses.path,'utf8')),responses=packet.Responses,{chromium}=await import(pathToFileURL(I.pwPath.path).href);
 server=await chromium.launchServer({executablePath:I.chromePath.path,headless:true,args:['--no-proxy-server','--disable-background-networking']});owned=trackOwnedChild(server.process(),{name:'chrome'});browser=await chromium.connect(server.wsEndpoint());assert.equal(browser.version(),manifest.chrome_version);
 for(const lane of [{name:'wide-light',width:1440,theme:'light'},{name:'narrow-dark',width:390,theme:'dark'}]) {
  let context;const routes=packet.Routes[lane.theme],requests=[],observations=[];
  try {
   context=await browser.newContext({viewport:{width:lane.width,height:1000},colorScheme:lane.theme,javaScriptEnabled:false,serviceWorkers:'block'});context.setDefaultTimeout(5000);
   await context.route('**/*',route=>{const r=route.request(),u=new URL(r.url()),response=responses[u.pathname+u.search],allowed=r.method()==='GET'&&u.origin==='http://127.0.0.1'&&!!response;requests.push({url:r.url(),allowed});if(!allowed){report.unexpectedRequests.push({lane:lane.name,url:r.url()});return route.abort('blockedbyclient');}return route.fulfill({status:response.Status,headers:Object.fromEntries(Object.entries(response.Headers).map(([k,v])=>[k.toLowerCase(),v.join(', ')])),body:response.Body});});
   const page=await context.newPage();
   async function observe(action,expected,status){
    const url='http://127.0.0.1'+expected,docReady=page.waitForResponse(r=>r.url()===url&&r.request().isNavigationRequest()),cssReady=page.waitForResponse(r=>new URL(r.url()).pathname==='/static/app.css'),completed=Promise.all([docReady,cssReady]);
    completed.catch(()=>{}); // Handle pending response rejection even if the action fails first.
    await action();const [doc,css]=await completed;await page.waitForLoadState('load');await Promise.all([doc.finished(),css.finished(),bound(page.evaluate(()=>document.fonts.ready),3000,'fonts')]);
    check(doc.status()===status&&sha(await doc.body())===sha(responses[expected].Body)&&sha(await css.body())===sha(responses['/static/app.css'].Body),`${lane.name} completed exact response ${expected}`);
    check(await page.locator('script').count()===0&&await page.locator('html').getAttribute('data-theme')===lane.theme,`${lane.name} no-script theme ${expected}`);
    check(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${lane.name} no page overflow ${expected}`);
    observations.push({url:page.url(),status:doc.status(),bytes:(await doc.body()).length});
   }
   async function activate(href){
    const target=page.locator('main').locator(`a[href=${JSON.stringify(href)}]`);assert.equal(await target.count(),1);
    let tabs=0;while(tabs++<100){await page.keyboard.press('Tab');if(await target.evaluate(e=>e===document.activeElement))break;}
    const shape=await target.evaluate(e=>{const r=e.getBoundingClientRect();return {focused:e===document.activeElement,height:r.height,visible:r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight};});
    check(shape.focused&&shape.height>=44&&shape.visible,`${lane.name} native link focus ${href}`,{tabs,...shape});assert(shape.focused,'native Tab did not reach link');await page.keyboard.press('Enter');
   }
   await observe(()=>page.goto('http://127.0.0.1'+routes.List),routes.List,200);
   check(new URL(routes.Detail,'http://127.0.0.1').searchParams.get('expected_revision')===packet.Revision,`${lane.name} expected catalog revision`);
   await observe(()=>activate(routes.Detail),routes.Detail,200);
   check(await page.getByRole('heading',{name:'Meaning-bearing analytics',exact:true}).count()===1,`${lane.name} selected capability visible`);
   await observe(()=>activate(routes.List),routes.List,200);
   await observe(()=>page.goto('http://127.0.0.1'+routes.Stale),routes.Stale,409);
   check(await page.getByRole('heading',{name:'This catalog selection needs review',exact:true}).count()===1&&await page.getByRole('heading',{name:'Meaning-bearing analytics',exact:true}).count()===0,`${lane.name} stale review refuses capability`);
   await page.screenshot({path:`${output}/${lane.name}-stale.png`,fullPage:true});
   await observe(()=>activate(routes.List),routes.List,200);
   await observe(()=>activate(routes.Detail),routes.Detail,200);
   await page.screenshot({path:`${output}/${lane.name}-recovered.png`,fullPage:true});
   await observe(()=>page.goto('http://127.0.0.1'+routes.Invalid),routes.Invalid,409);
   check(await page.getByText('Some saved filters are no longer valid. Filters have been cleared so you can review the current catalog.',{exact:true}).count()===1,`${lane.name} invalid filter reset disclosed`);
   await page.screenshot({path:`${output}/${lane.name}-invalid.png`,fullPage:true});
   await observe(()=>activate(routes.Reset),routes.Reset,200);
   check(await page.getByRole('heading',{name:'What are you building?',exact:true}).count()===1,`${lane.name} invalid saved selection recovered`);
   report.cells.push({lane,routes,requests,observations});
  }finally{if(context)await bound(context.close(),5000,'context close');}
 }
}catch(e){report.errors.push({name:'run error',detail:{error:String(e),stack:e?.stack}});}
finally {
 try{if(browser)await bound(browser.close(),5000,'browser close');}catch(e){report.cleanup.browserCloseError=String(e);}try{if(server)await bound(server.close(),5000,'server close');}catch(e){report.cleanup.serverCloseError=String(e);}if(owned)report.cleanup.chrome={termination:await terminateOwned(owned,{graceMs:3000,killMs:2000,drainMs:2000}),lifecycle:owned.state};clearTimeout(watchdog);
 report.endedAt=new Date().toISOString();report.summary={checks:report.checks.length,failures:report.errors.length,cells:report.cells.length};report.pass=!report.timedOut&&report.errors.length===0&&report.unexpectedRequests.length===0&&report.cells.length===2&&report.cleanup.chrome?.termination?.ok&&!report.cleanup.browserCloseError&&!report.cleanup.serverCloseError;await fs.writeFile(`${output}/report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx'});console.log(JSON.stringify({pass:report.pass,summary:report.summary,errors:report.errors,cleanup:report.cleanup.chrome?.termination}));process.exitCode=report.pass?0:1;
}
