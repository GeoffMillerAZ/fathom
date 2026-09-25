import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {trackOwnedChild,terminateOwned} from '../bar-resolution-2026-09-22/browser-owned_process.mjs';

const [manifestPath,output]=process.argv.slice(2);
assert([manifestPath,output].every(x=>x?.startsWith('/')),'two absolute arguments required');
const manifest=JSON.parse(await fs.readFile(manifestPath,'utf8')),I=manifest.inputs;
const required=['baselineLight','baselineDark','candidateLight','candidateDark','cssPath','pwPath','chromePath','node','frameworkPath','driver','helper','fixture','contract'];
const sha=b=>createHash('sha256').update(b).digest('hex');
const oldReason='Bar chart omitted: labels or values do not leave at least 48 pixels on each side of zero. All selected values remain in the data table. Choose wider chart and label columns or an app-owned wrapping layout to show the diverging bars.';
const newReason='Bar chart omitted: one or more nonzero values would render smaller than one logical pixel at this scale. All selected values remain in the data table. Choose a narrower selection or an app-owned alternative representation to compare these values graphically.';
const report={startedAt:new Date().toISOString(),identities:{},checks:[],cells:[],errors:[],unexpectedRequests:[],cleanup:{},timedOut:false};
const check=(ok,name,detail={})=>{report.checks.push({ok,name,detail});if(!ok)report.errors.push({name,detail});};
const bound=async(p,ms,name)=>{let timer;try{return await Promise.race([p,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(name+' timeout')),ms);})]);}finally{clearTimeout(timer);}};
await fs.mkdir(output,{recursive:false,mode:0o700});
let server,browser,owned,error;const watchdog=setTimeout(()=>{report.timedOut=true;try{owned?.child.kill('SIGTERM');}catch{}},90000);
try {
  assert.deepEqual(Object.keys(I).sort(),required.slice().sort());
  for(const key of required){const input=I[key];assert(input.path.startsWith('/')&&/^[0-9a-f]{64}$/.test(input.sha256));const bytes=await fs.readFile(input.path);assert.equal(sha(bytes),input.sha256);report.identities[key]={...input,size:bytes.length};}
  assert.equal(fileURLToPath(import.meta.url),I.driver.path);assert.equal(fileURLToPath(new URL('../bar-resolution-2026-09-22/browser-owned_process.mjs',import.meta.url)),I.helper.path);assert.equal(process.execPath,I.node.path);assert.equal(process.version,manifest.node_version);
  const pkg=JSON.parse(await fs.readFile(new URL('package.json',pathToFileURL(I.pwPath.path)),'utf8'));assert.equal(pkg.version,manifest.pw_version);
  for(const theme of ['Light','Dark']){
    const a=await fs.readFile(I['baseline'+theme].path,'utf8'),b=await fs.readFile(I['candidate'+theme].path,'utf8');
    // Exactly two subpixel explanations change; the long-label explanation and
    // every other HTML byte (including the ordinary SVG and all tables) stay.
    check(a.split(oldReason).length-1===3&&b.split(newReason).length-1===2&&b.split(oldReason).length-1===1,`${theme} explanation counts`);
    let changed=0;const expected=a.replaceAll(oldReason,()=>++changed<=2?newReason:oldReason);
    check(b===expected,`${theme} only two explanation strings differ`);
  }
  const {chromium}=await import(pathToFileURL(I.pwPath.path).href);
  server=await chromium.launchServer({executablePath:I.chromePath.path,headless:true,args:['--no-proxy-server','--disable-background-networking']});owned=trackOwnedChild(server.process(),{name:'chrome'});browser=await chromium.connect(server.wsEndpoint());assert.equal(browser.version(),manifest.chrome_version);report.chromeVersion=browser.version();
  const css=await fs.readFile(I.cssPath.path);
  for(const variant of ['baseline','candidate'])for(const lane of [{name:'wide-light',theme:'light',width:1440,height:1000},{name:'narrow-dark',theme:'dark',width:390,height:844}]){
    const id=variant+'-'+lane.name,input=I[variant+(lane.theme==='light'?'Light':'Dark')],html=await fs.readFile(input.path);let context;
    try {
      context=await browser.newContext({viewport:{width:lane.width,height:lane.height},colorScheme:lane.theme,javaScriptEnabled:false,serviceWorkers:'block'});context.setDefaultTimeout(5000);
      const requests=[];await context.route('**/*',route=>{const r=route.request(),u=new URL(r.url()),doc=u.origin==='http://127.0.0.1'&&u.pathname==='/'&&u.search===`?cell=${id}`,style=u.origin==='http://127.0.0.1'&&u.pathname==='/app.css'&&!u.search,allowed=r.method()==='GET'&&(doc||style);requests.push({url:r.url(),method:r.method(),allowed});if(!allowed){report.unexpectedRequests.push({id,url:r.url()});return route.abort('blockedbyclient');}return route.fulfill({status:200,headers:{'content-type':style?'text/css; charset=utf-8':'text/html; charset=utf-8','content-security-policy':"default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",'cache-control':'no-store','x-content-type-options':'nosniff'},body:style?css:html});});
      const page=await context.newPage(),cssReady=page.waitForResponse(r=>new URL(r.url()).pathname==='/app.css'),doc=await page.goto(`http://127.0.0.1/?cell=${id}`,{waitUntil:'load'}),style=await cssReady;
      await Promise.all([doc.finished(),style.finished(),bound(page.evaluate(()=>document.fonts.ready),3000,'font readiness')]);
      check(doc.status()===200&&style.status()===200&&html.length<=65536&&sha(await doc.body())===input.sha256&&sha(await style.body())===I.cssPath.sha256,`${id} complete exact documents`);
      check(requests.length===2&&requests.every(x=>x.allowed)&&await page.locator('script').count()===0&&await page.locator('html').getAttribute('data-theme')===lane.theme,`${id} exact scriptless routes/theme`,requests);
      const facts={};
      for(const [key,value]of [['positive','0.01'],['negative','-0.01']]){
        const figure=page.locator('#'+key),reason=figure.locator('.ak-vizframe-body > p').last(),text=await reason.textContent();
        const rows=await figure.locator('tbody tr').evaluateAll(rs=>rs.map(r=>[...r.children].map(c=>c.textContent)));
        check(text===(variant==='candidate'?newReason:oldReason),`${id} ${key} truthful reason`,text);
        check(JSON.stringify(rows)===JSON.stringify([['Tiny',value,'Not provided','Present','Observed amounts'],['Large','1,000,000,000,000','Not provided','Present','Observed amounts']]),`${id} ${key} exact table`,rows);
        check(await figure.locator('svg').count()===0&&await figure.locator('[aria-label="Chart legend"]').count()===0,`${id} ${key} no false plot or legend`);
        const geometry=await reason.evaluate(e=>{const r=document.createRange();r.selectNodeContents(e);const b=e.getBoundingClientRect();return {box:{left:b.left,right:b.right},contained:[...r.getClientRects()].every(x=>x.left>=b.left-1&&x.right<=b.right+1&&x.left>=-1&&x.right<=innerWidth+1)};});check(geometry.contained,`${id} ${key} readable explanation`,geometry);
        const region=figure.locator('[role="region"]');let tabs=0;while(tabs<12){tabs++;await page.keyboard.press('Tab');if(await region.evaluate(e=>document.activeElement===e))break;}
        const focus=await region.evaluate(e=>({active:document.activeElement===e,tabIndex:e.tabIndex,role:e.getAttribute('role')}));check(focus.active&&focus.tabIndex===0&&focus.role==='region',`${id} ${key} native table focus`,{tabs,...focus});facts[key]={text,rows,geometry,focus};
      }
      check(await page.locator('#fitting [data-analytics-bar]').count()===2&&await page.locator('#fitting [data-analytics-zero-axis]').count()===1,`${id} ordinary signed chart retained`);
      check(await page.locator('#labels .ak-vizframe-body > p').last().textContent()===oldReason&&await page.locator('#labels svg').count()===0,`${id} actual label-space remedy retained`);
      await page.screenshot({path:`${output}/${id}.png`,fullPage:true});report.cells.push({id,requests,facts});
    }finally{if(context)await bound(context.close(),5000,'context close');}
  }
}catch(e){error=String(e);report.errors.push({name:'run error',detail:{error,stack:e?.stack}});}
finally{
  try{if(browser)await bound(browser.close(),5000,'browser close');}catch(e){report.cleanup.browserCloseError=String(e);}try{if(server)await bound(server.close(),5000,'server close');}catch(e){report.cleanup.serverCloseError=String(e);}if(owned)report.cleanup.chrome={termination:await terminateOwned(owned,{graceMs:3000,killMs:2000,drainMs:2000}),lifecycle:owned.state};clearTimeout(watchdog);
  report.endedAt=new Date().toISOString();report.summary={checks:report.checks.length,failures:report.errors.length,cells:report.cells.length};report.pass=!error&&!report.timedOut&&report.errors.length===0&&report.unexpectedRequests.length===0&&report.cells.length===4&&report.cleanup.chrome?.termination?.ok&&!report.cleanup.browserCloseError&&!report.cleanup.serverCloseError;await fs.writeFile(`${output}/report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx'});process.exitCode=report.pass?0:1;
}
