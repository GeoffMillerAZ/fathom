import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {trackOwnedChild,terminateOwned} from '../bar-resolution-2026-09-22/browser-owned_process.mjs';

const [manifestPath,output]=process.argv.slice(2);
assert(manifestPath?.startsWith('/')&&output?.startsWith('/'),'absolute manifest and output are required');
const manifest=JSON.parse(await fs.readFile(manifestPath,'utf8'));
const I=manifest.inputs,variant=manifest.variant;
assert(['baseline','candidate'].includes(variant));
const lanes=[
  {name:'wide-light',width:1440,height:1000,theme:'light'},
  {name:'narrow-dark',width:390,height:844,theme:'dark'},
];
const report={schema:'appkit-table-links-browser-v1',startedAt:new Date().toISOString(),variant,checks:[],baselineFailures:[],cells:[],captures:[],errors:[],unexpectedRequests:[],cleanup:{},timedOut:false};
const sha=value=>createHash('sha256').update(value).digest('hex');
const check=(ok,name,detail={})=>{const item={ok,name,detail};report.checks.push(item);if(!ok)report.errors.push(item);};
const baselineFailure=(ok,name,detail={})=>{if(!ok)report.baselineFailures.push({ok:false,name,detail});};
const bounded=async(p,ms,name)=>{let timer;try{return await Promise.race([p,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(`${name} timeout`)),ms);})]);}finally{clearTimeout(timer);}};
const bodyBytes=value=>Buffer.from(value??'');
const headerObject=x=>Object.fromEntries(Object.entries(x.header??{}).map(([key,value])=>[key.toLowerCase(),value.join(', ')]));
const exact=(map,key)=>{const value=map[key];assert(value,`missing retained response ${key}`);return value;};
const hashInputs=async()=>{const hashes={};for(const input of Object.values(I)){assert(input?.path?.startsWith('/')&&/^[0-9a-f]{64}$/.test(input.sha256));const bytes=await fs.readFile(input.path);assert.equal(sha(bytes),input.sha256,`input changed: ${input.path}`);hashes[input.path]=sha(bytes);}return hashes;};
const hrefFor=(kind,theme)=>`./${kind}?theme=${theme}`;

await fs.mkdir(output,{mode:0o700});
let server,browser,owned,runError;
const watchdog=setTimeout(()=>{report.timedOut=true;try{owned?.child.kill('SIGTERM');}catch{}},120000);
try{
  report.inputHashesBefore=await hashInputs();
  assert.equal(fileURLToPath(import.meta.url),I.driver.path);
  assert.equal(fileURLToPath(new URL('../bar-resolution-2026-09-22/browser-owned_process.mjs',import.meta.url)),I.helper.path);
  assert.equal(process.execPath,I.node.path);assert.equal(process.version,manifest.node_version);
  const pwPackage=JSON.parse(await fs.readFile(new URL('package.json',pathToFileURL(I.pwPath.path)),'utf8'));
  assert.equal(pwPackage.version,manifest.pw_version);
  const retained=JSON.parse(await fs.readFile(I.responses.path,'utf8'));
  const responses=Object.fromEntries(retained.responses.map(x=>[x.path,x]));
  const themes=['light','dark'];
  for(const theme of themes)for(const path of ['/table-links','/alpha','/beta'])exact(responses,`${path}?theme=${theme}`);
  exact(responses,'/static/app.css');
  const {chromium}=await import(pathToFileURL(I.pwPath.path).href);
  server=await bounded(chromium.launchServer({executablePath:I.chromePath.path,headless:true,chromiumSandbox:true,args:['--no-proxy-server','--disable-background-networking'],timeout:15000}),15000,'Chrome launch');
  owned=trackOwnedChild(server.process(),{name:'chrome'});
  browser=await bounded(chromium.connect(server.wsEndpoint(),{timeout:10000}),10000,'Chrome connect');
  assert.equal(browser.version(),manifest.chrome_version);

  for(const lane of lanes){
    const id=`${variant}-${lane.name}`;let context;
    const requests=[];
    try{
      context=await browser.newContext({viewport:{width:lane.width,height:lane.height},colorScheme:lane.theme,javaScriptEnabled:false,serviceWorkers:'block'});
      context.setDefaultTimeout(6000);
      await context.route('**/*',async route=>{
        const req=route.request(),url=new URL(req.url());
        const key=url.pathname+url.search,allowed=req.method()==='GET'&&url.origin==='http://127.0.0.1'&&responses[key];
        requests.push({url:req.url(),method:req.method(),allowed:!!allowed});
        if(!allowed){report.unexpectedRequests.push({id,url:req.url(),method:req.method()});return route.abort('blockedbyclient');}
        const response=responses[key];return route.fulfill({status:response.status,headers:headerObject(response),body:response.body});
      });
      const page=await context.newPage();
      const verify=async(doc,css,key,kind)=>{
        await bounded(Promise.all([doc.finished(),css.finished()]),5000,`${id} ${kind} response completion`);
        check(doc.status()===200&&css.status()===200,`${id} ${kind} successful responses`);
        const documentBytes=bodyBytes(await doc.body()),cssBytes=bodyBytes(await css.body()),expected=exact(responses,key),expectedCSS=exact(responses,'/static/app.css');
        check(doc.status()===expected.status&&css.status()===expectedCSS.status&&sha(documentBytes)===sha(bodyBytes(expected.body))&&sha(cssBytes)===sha(bodyBytes(expectedCSS.body)),`${id} ${kind} exact document/CSS hashes`,{path:key,document:sha(documentBytes),css:sha(cssBytes),documentStatus:doc.status(),cssStatus:css.status()});
        check(documentBytes.length<=32768,`${id} ${kind} document body <=32KiB`,{bytes:documentBytes.length});
        check(await page.locator('script,iframe').count()===0,`${id} ${kind} no script or iframe`);
        check(await page.locator('html').getAttribute('data-theme')===lane.theme,`${id} ${kind} exact theme`,{theme:await page.locator('html').getAttribute('data-theme')});
        check(await page.evaluate(()=>document.fonts.status==='loaded'&&[...document.fonts].every(font=>font.status==='loaded')),`${id} ${kind} fonts loaded synchronously`);
        check(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${id} ${kind} no horizontal overflow`);
        report.captures.push({id,kind,path:key,documentBytes:documentBytes.length,documentSHA256:sha(documentBytes),cssSHA256:sha(cssBytes)});
      };
      const navigate=async(path,kind,action)=>{
        const navigation=page.waitForNavigation({waitUntil:'load'});
        const cssWait=page.waitForResponse(response=>new URL(response.url()).pathname==='/static/app.css');
        navigation.catch(()=>{});cssWait.catch(()=>{});
        await bounded(action(),15000,`${id} ${kind} action`);
        const [documentResponse,css]=await Promise.all([
          bounded(navigation,15000,`${id} ${kind} navigation`),
          bounded(cssWait,5000,`${id} ${kind} CSS response`),
        ]);
        await verify(documentResponse,css,path,kind);return documentResponse;
      };
      const tabTo=async(locator,max=30)=>{for(let index=0;index<max;index++){await page.keyboard.press('Tab');if(await locator.evaluate(node=>node===document.activeElement))return index+1;}return 0;};
      const focusState=locator=>locator.evaluate(node=>{const style=getComputedStyle(node),box=node.getBoundingClientRect();return {tag:node.tagName,role:node.getAttribute('role'),ariaDisabled:node.getAttribute('aria-disabled'),href:node.getAttribute('href'),tabindex:node.getAttribute('tabindex'),text:node.textContent?.trim()??'',active:node===document.activeElement,width:box.width,height:box.height,outlineStyle:style.outlineStyle,outlineWidth:style.outlineWidth};});
      const target=async(locator,name)=>{const tabs=await tabTo(locator);const state=await focusState(locator);const ok=tabs>0&&state.active&&state.width>=44&&state.height>=44&&state.outlineStyle!=='none'&&parseFloat(state.outlineWidth)>=2;(variant==='candidate'?check:baselineFailure)(ok,`${id} ${name} native target/focus`,{tabs,state});return {tabs,state};};
      const listKey=`/table-links?theme=${lane.theme}`;
      await navigate(listKey,'list',()=>page.goto(`http://127.0.0.1${listKey}`,{waitUntil:'load'}));
      const rowTexts=await page.locator('tbody tr').evaluateAll(rows=>rows.map(row=>[...row.cells].map(cell=>cell.textContent?.replace(/\s+/g,' ').trim()).join(' ').trim()));
      const unsafeText='Unsafe 7 Open unavailable';
      check(JSON.stringify(rowTexts)===JSON.stringify(['Alpha 4 Open →',unsafeText,'Beta 9 Open →','No destination 12']),`${id} table row text contract`,{rowTexts});
      const shape=await page.locator('table').evaluate(table=>({headers:[...table.tHead.rows[0].cells].map(cell=>cell.textContent.trim()),rows:[...table.tBodies[0].rows].map(row=>({id:row.dataset.tableRow,cells:row.cells.length,last:row.cells[row.cells.length-1].textContent.trim()}))}));
      (variant==='candidate'?check:baselineFailure)(JSON.stringify(shape.headers)===JSON.stringify(['Name','Count','Actions'])&&shape.rows.every(row=>row.cells===3),`${id} inferred action header and rectangular rows`,shape);
      (variant==='candidate'?check:baselineFailure)(shape.rows.find(row=>row.id==='plain')?.last==='',`${id} unlinked row reserves an empty action cell`,shape);
      const alpha=page.locator('tr[data-table-row="alpha"] a, tr[data-table-row="alpha"] [role="link"]').first();
      const unsafe=page.locator('tr[data-table-row="unsafe"] a, tr[data-table-row="unsafe"] [role="link"]').first();
      const beta=page.locator('tr[data-table-row="beta"] a, tr[data-table-row="beta"] [role="link"]').first();
      const plain=page.locator('tr[data-table-row="plain"] a, tr[data-table-row="plain"] [role="link"]').first();
      check(await alpha.getAttribute('href')===hrefFor('alpha',lane.theme)&&await beta.getAttribute('href')===hrefFor('beta',lane.theme),`${id} valid relative destinations retained`);
      check(await plain.count()===0,`${id} noHref row has no action`);
      const unsafeCount=await unsafe.count();
      const state=unsafeCount===1?await focusState(unsafe):{};
      const inert=unsafeCount===1&&state.tag==='SPAN'&&state.role==='link'&&state.ariaDisabled==='true'&&state.href===null&&state.tabindex===null&&state.text==='Open unavailable';
      (variant==='candidate'?check:baselineFailure)(inert,`${id} unsafe destination is inert unavailable role`,state);
      if(inert){const beforeURL=page.url(),beforeRequests=requests.length;const box=await unsafe.boundingBox();assert(box&&box.width>0&&box.height>0);await page.mouse.click(box.x+box.width/2,box.y+box.height/2);check(page.url()===beforeURL&&requests.length===beforeRequests,`${id} pointer click on unavailable destination is inert`,{beforeURL,afterURL:page.url(),newRequests:requests.slice(beforeRequests)});}
      await target(alpha,'alpha');
      await navigate(`/alpha?theme=${lane.theme}`,'alpha detail',()=>page.keyboard.press('Enter'));
      check(await page.getByRole('heading',{name:'alpha detail',exact:true}).count()===1,`${id} alpha detail reached`);
      const back=page.getByRole('link',{name:'Back to list',exact:true});await tabTo(back);await navigate(listKey,'back to list',()=>page.keyboard.press('Enter'));
      await target(beta,'beta');
      await navigate(`/beta?theme=${lane.theme}`,'beta detail',()=>page.keyboard.press('Enter'));
      check(await page.getByRole('heading',{name:'beta detail',exact:true}).count()===1,`${id} beta detail reached`);
      const backAgain=page.getByRole('link',{name:'Back to list',exact:true});await tabTo(backAgain);await navigate(listKey,'beta back to list',()=>page.keyboard.press('Enter'));
      const order=[];for(let index=0;index<30;index++){await page.keyboard.press('Tab');const active=await page.evaluate(()=>{const node=document.activeElement;return node?.closest('tr')?.dataset.tableRow??null;});if(active)order.push(active);if(active==='beta')break;}
      const expectedPrefix=['alpha','beta'];
      (variant==='candidate'?check:baselineFailure)(JSON.stringify(order.slice(0,expectedPrefix.length))===JSON.stringify(expectedPrefix),`${id} native tab order reaches alpha then expected next row`,{order,expectedPrefix});
      const skippedUnsafe=!order.includes('unsafe');(variant==='candidate'?check:baselineFailure)(skippedUnsafe,`${id} native Tab skips unsafe row to beta`,{order});
      check(order.includes('beta'),`${id} native Tab reaches beta after alpha`,{order});
      report.cells.push({id,requests,rowTexts,order,unsafeCount});
      await bounded(page.screenshot({path:`${output}/${id}-recovered.png`,fullPage:true}),5000,`${id} screenshot`);
    }finally{if(context)await bounded(context.close(),5000,`${id} context close`);}
  }
}catch(error){runError=error;report.errors.push({ok:false,name:'run error',detail:{error:String(error),stack:error?.stack}});}
finally{
  try{if(browser)await bounded(browser.close(),5000,'browser close');}catch(error){report.cleanup.browserCloseError=String(error);}
  try{if(server)await bounded(server.close(),5000,'server close');}catch(error){report.cleanup.serverCloseError=String(error);}
  if(owned){try{report.cleanup.chrome={termination:await terminateOwned(owned,{graceMs:3000,killMs:2000,drainMs:2000}),lifecycle:owned.state};}catch(error){report.cleanup.chromeError=String(error);}}
  clearTimeout(watchdog);
  try{report.inputHashesAfter=await hashInputs();if(JSON.stringify(report.inputHashesBefore)!==JSON.stringify(report.inputHashesAfter))report.errors.push({ok:false,name:'input hashes changed during acquisition'});}catch(error){report.inputHashesAfter={error:String(error)};report.errors.push({ok:false,name:'input hash recheck failed',detail:{error:String(error)}});}
  report.endedAt=new Date().toISOString();report.runError=runError?String(runError):null;
  report.summary={checks:report.checks.length,errors:report.errors.length,baselineFailures:report.baselineFailures.length,cells:report.cells.length,unexpectedRequests:report.unexpectedRequests.length};
  report.pass=!runError&&!report.timedOut&&report.errors.length===0&&report.baselineFailures.length===0&&report.unexpectedRequests.length===0&&report.cells.length===2&&report.cleanup.chrome?.termination?.ok&&!report.cleanup.browserCloseError&&!report.cleanup.serverCloseError;
  await fs.writeFile(`${output}/report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx'});console.log(JSON.stringify({pass:report.pass,summary:report.summary}));process.exitCode=report.pass?0:1;
}
