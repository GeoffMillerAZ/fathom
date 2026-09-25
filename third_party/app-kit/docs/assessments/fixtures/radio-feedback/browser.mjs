import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {trackOwnedChild,terminateOwned} from '../bar-resolution-2026-09-22/browser-owned_process.mjs';

const [manifestPath,output]=process.argv.slice(2);assert([manifestPath,output].every(x=>x?.startsWith('/')));
const manifest=JSON.parse(await fs.readFile(manifestPath,'utf8')),I=manifest.inputs,variant=manifest.variant;
assert(['baseline','candidate'].includes(variant));
const sha=b=>createHash('sha256').update(b).digest('hex');
const lanes=[{name:'wide-light',width:1440,height:1000,theme:'light'},{name:'narrow-dark',width:390,height:844,theme:'dark'}];
const errorText='Choose a supported interval before reviewing this selection.';
const scheduleHelp='Choose the interval for the report. No schedule will be saved.';
const channelHelp='Synthetic delivery options, not a notification service.';
const report={startedAt:new Date().toISOString(),variant,checks:[],cells:[],errors:[],unexpectedRequests:[],cleanup:{},timedOut:false};
const check=(ok,name,detail={})=>{const x={ok,name,detail};report.checks.push(x);if(!ok)report.errors.push(x);};
const bound=async(p,ms,name)=>{let timer;try{return await Promise.race([p,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(name+' timeout')),ms);})]);}finally{clearTimeout(timer);}};
const headers=x=>Object.fromEntries(Object.entries(x.Header).map(([k,v])=>[k.toLowerCase(),v.join(', ')]));
const canonical=(responses,path)=>{const x=responses[path];assert(x,`missing retained response ${path}`);return x;};
const focusByTab=async(page,locator,max=30)=>{for(let i=0;i<max;i++){await page.keyboard.press('Tab');if(await locator.evaluate(e=>e===document.activeElement))return i+1;}return 0;};
const description=locator=>locator.evaluate(e=>{const ref=e.getAttribute('aria-describedby')||'',ids=ref.trim()?ref.trim().split(/\s+/):[];return {ref,ids,text:ids.map(id=>document.getElementById(id)?.textContent.trim()||'').join(' ').trim(),valid:ids.length>0&&ids.every(id=>!!document.getElementById(id)),invalid:e.getAttribute('aria-invalid')};});
const radioAX=tree=>tree.nodes.filter(n=>n.role?.value==='radio').map(n=>({name:n.name?.value||'',description:n.description?.value||'',invalid:n.properties?.find(p=>p.name==='invalid')?.value?.value??false,disabled:n.properties?.find(p=>p.name==='disabled')?.value?.value??false,checked:n.properties?.find(p=>p.name==='checked')?.value?.value??false}));
const exactKeys=(u,want)=>{const got=[...u.searchParams.entries()];return got.length===Object.keys(want).length&&Object.entries(want).every(([k,v])=>u.searchParams.getAll(k).length===1&&u.searchParams.get(k)===v);};

await fs.mkdir(output,{mode:0o700});let server,browser,owned,runError;
const watchdog=setTimeout(()=>{report.timedOut=true;try{owned?.child.kill('SIGTERM');}catch{}},90000);
try{
 for(const input of Object.values(I)){assert(input.path.startsWith('/')&&/^[0-9a-f]{64}$/.test(input.sha256));assert.equal(sha(await fs.readFile(input.path)),input.sha256);}
 assert.equal(fileURLToPath(import.meta.url),I.driver.path);assert.equal(fileURLToPath(new URL('../bar-resolution-2026-09-22/browser-owned_process.mjs',import.meta.url)),I.helper.path);assert.equal(process.execPath,I.node.path);assert.equal(process.version,manifest.node_version);
 const pkg=JSON.parse(await fs.readFile(new URL('package.json',pathToFileURL(I.pwPath.path)),'utf8'));assert.equal(pkg.version,manifest.pw_version);
 const retained=JSON.parse(await fs.readFile(I.responses.path,'utf8')),responses=Object.fromEntries(retained.map(x=>[x.Path,x]));
 for(const path of ['/forms?theme=light','/forms?theme=dark','/_fixture/error?theme=light','/_fixture/error?theme=dark','/_fixture/review?theme=light','/_fixture/review?theme=dark','/static/app.css'])canonical(responses,path);
 const {chromium}=await import(pathToFileURL(I.pwPath.path).href);
 server=await chromium.launchServer({executablePath:I.chromePath.path,headless:true,args:['--no-proxy-server','--disable-background-networking']});owned=trackOwnedChild(server.process(),{name:'chrome'});browser=await chromium.connect(server.wsEndpoint());assert.equal(browser.version(),manifest.chrome_version);
 for(const lane of lanes){const id=`${variant}-${lane.name}`,requests=[];let context;
  try{
   context=await browser.newContext({viewport:{width:lane.width,height:lane.height},colorScheme:lane.theme,javaScriptEnabled:false,serviceWorkers:'block'});context.setDefaultTimeout(6000);
   await context.route('**/*',route=>{const r=route.request(),u=new URL(r.url());let response,kind='retained';
    if(r.method()==='GET'&&u.origin==='http://127.0.0.1'&&u.pathname==='/_fixture/review'&&exactKeys(u,{theme:lane.theme,schedule:'weekly','channel filter <&>':'email'})){response=canonical(responses,`/_fixture/review?theme=${lane.theme}`);kind='validated review';}
    else if(r.method()==='GET'&&u.origin==='http://127.0.0.1'&&u.pathname!=='/_fixture/review'){const key=u.pathname+u.search;response=responses[key];}
    const allowed=!!response;requests.push({url:r.url(),method:r.method(),allowed,kind});if(!allowed){report.unexpectedRequests.push({id,url:r.url(),method:r.method()});return route.abort('blockedbyclient');}
    return route.fulfill({status:response.Status,headers:headers(response),body:response.Body});
   });
   const page=await context.newPage();
   const load=async(path,expectedPath,scriptless=true)=>{const cssReady=page.waitForResponse(r=>new URL(r.url()).pathname==='/static/app.css'),doc=await page.goto('http://127.0.0.1'+path,{waitUntil:'load'}),css=await cssReady;await Promise.all([doc.finished(),css.finished(),bound(page.evaluate(()=>document.fonts.ready),3000,'fonts')]);const docBody=Buffer.from(await doc.body()),styleBody=Buffer.from(await css.body());check(sha(docBody)===sha(canonical(responses,expectedPath).Body)&&sha(styleBody)===sha(canonical(responses,'/static/app.css').Body),`${id} ${expectedPath} exact completed document/CSS`,{document:sha(docBody),css:sha(styleBody)});if(scriptless)check(await page.locator('script').count()===0,`${id} ${expectedPath} scriptless`);};
   await load(`/_fixture/error?theme=${lane.theme}`,`/_fixture/error?theme=${lane.theme}`);check(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${id} error fixture no page overflow`);
   const daily=page.locator('#schedule-0'),paused=page.locator('#schedule-1'),weekly=page.locator('#schedule-2'),schedule=page.locator('input[name="schedule"]');
   const errorAssociations=await schedule.evaluateAll((nodes,errorText)=>nodes.map(e=>{const ref=e.getAttribute('aria-describedby')||'',ids=ref.trim()?ref.trim().split(/\s+/):[];return {id:e.id,ref,ids,text:ids.map(id=>document.getElementById(id)?.textContent.trim()||'').join(' ').trim(),valid:ids.length===1&&!!document.getElementById(ids[0]),invalid:e.getAttribute('aria-invalid')};}),errorText);
   const channelAssociations=await page.locator('input[type="radio"]').evaluateAll(nodes=>nodes.filter(e=>e.name==='channel filter <&>').map(e=>{const ref=e.getAttribute('aria-describedby')||'',ids=ref.trim()?ref.trim().split(/\s+/):[];return {id:e.id,ref,ids,text:ids.map(id=>document.getElementById(id)?.textContent.trim()||'').join(' ').trim(),valid:ids.length===1&&!!document.getElementById(ids[0]),invalid:e.getAttribute('aria-invalid')};}));
   const fixedError=errorAssociations.every(x=>x.valid&&x.text===errorText&&x.invalid==='true'),fixedHelp=channelAssociations.length===2&&new Set(channelAssociations.map(x=>x.ref)).size===1&&channelAssociations.every(x=>x.valid&&x.text===channelHelp&&x.invalid===null);
   check(fixedError,`${id} error association and invalid state`,errorAssociations);
   check(fixedHelp,`${id} hostile-name single-IDREF help`,channelAssociations);
   const cdp=await context.newCDPSession(page),ax=await cdp.send('Accessibility.getFullAXTree'),axRadios=radioAX(ax);await fs.writeFile(`${output}/${id}-error-ax.json`,JSON.stringify(ax,null,2)+'\n',{flag:'wx'});
   const scheduleAX=axRadios.filter(x=>['Daily','Paused (unavailable)','Weekly'].includes(x.name)),axFixed=scheduleAX.length===3&&scheduleAX.every(x=>x.description===errorText&&x.invalid==='true');
   check(axFixed,`${id} AX error description and invalid`,scheduleAX);
   const tabs=await focusByTab(page,daily),focus=await daily.evaluate(e=>{const s=getComputedStyle(e);return {focused:e===document.activeElement,boxShadow:s.boxShadow};});
   check(tabs>0&&focus.focused&&focus.boxShadow!=='none',`${id} Daily native Tab and visible focus ring`,{tabs,focus});await page.screenshot({path:`${output}/${id}-error-focus.png`,fullPage:true});
   await page.keyboard.press('ArrowDown');check(await weekly.isChecked()&&!await paused.isChecked()&&await weekly.evaluate(e=>e===document.activeElement),`${id} ArrowDown skips disabled Paused to Weekly`);
   const formData=await page.locator('form').evaluate(f=>Object.fromEntries(new FormData(f)));check(formData.schedule==='weekly'&&formData['channel filter <&>']==='email',`${id} exact native FormData`,formData);
   const review=page.getByRole('button',{name:'Review selection',exact:true}),reviewTabs=await focusByTab(page,review),navigation=page.waitForNavigation({waitUntil:'load'}),reviewCSS=page.waitForResponse(r=>new URL(r.url()).pathname==='/static/app.css');await page.keyboard.press('Enter');const [reviewDoc,reviewStyle]=await Promise.all([navigation,reviewCSS]);await Promise.all([reviewDoc.finished(),reviewStyle.finished(),bound(page.evaluate(()=>document.fonts.ready),3000,'review fonts')]);
   const reviewResponse=canonical(responses,`/_fixture/review?theme=${lane.theme}`),reviewBody=Buffer.from(await reviewDoc.body()),reviewCSSBody=Buffer.from(await reviewStyle.body());check(reviewTabs>0&&sha(reviewBody)===sha(reviewResponse.Body)&&sha(reviewCSSBody)===sha(canonical(responses,'/static/app.css').Body),`${id} native Enter GET and exact reviewed document/CSS`,{reviewTabs,url:page.url()});
   const reviewed=page.locator('input[name="schedule"]'),reviewDescriptions=await reviewed.evaluateAll(nodes=>nodes.map(e=>{const ref=e.getAttribute('aria-describedby')||'',ids=ref.trim()?ref.trim().split(/\s+/):[];return {ref,text:ids.map(id=>document.getElementById(id)?.textContent.trim()||'').join(' ').trim(),invalid:e.getAttribute('aria-invalid'),checked:e.checked,value:e.value};}));
   const reviewSurface=!await page.getByText(errorText,{exact:true}).count()&&await page.getByText(scheduleHelp,{exact:true}).count()===1&&reviewDescriptions.some(x=>x.value==='weekly'&&x.checked);check(reviewSurface,`${id} reviewed state exposes Help, clears Error and keeps Weekly`,reviewDescriptions);const reviewFixed=reviewSurface&&reviewDescriptions.every(x=>x.invalid===null&&x.text===scheduleHelp);
   check(reviewFixed,`${id} reviewed state clears invalid/error description and associates help`,reviewDescriptions);check(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${id} reviewed fixture no page overflow`);await page.screenshot({path:`${output}/${id}-review.png`,fullPage:true});
   const galleryRequestStart=requests.length;await load(`/forms?theme=${lane.theme}`,`/forms?theme=${lane.theme}`,false);const galleryRequests=requests.slice(galleryRequestStart),gallery=page.locator('input[name="has-evidence"]'),galleryDescription=await description(gallery.first()),galleryLegend=await gallery.first().locator('xpath=ancestor::fieldset/legend').textContent(),galleryFixed=galleryDescription.valid&&galleryDescription.ids.length===1&&galleryDescription.text==='Answering Yes lets the matcher propose links once the record is ingested.';
   check(galleryRequests.every(x=>!new URL(x.url).pathname.endsWith('.js')),`${id} Gallery JavaScript-disabled with no script request`,galleryRequests);
   check(galleryLegend?.trim()==='Do you have evidence for this element?',`${id} Gallery fixed radio legend`,{galleryLegend});
   check(galleryFixed,`${id} Gallery radio help description`,galleryDescription);
   const yes=gallery.first(),no=gallery.nth(1);check(await yes.getAttribute('value')==='yes'&&await yes.isChecked()&&await no.getAttribute('value')==='no',`${id} Gallery fixed radio literals`);const galleryTabs=await focusByTab(page,yes,80);await page.keyboard.press('ArrowRight');check(galleryTabs>0&&await no.isChecked()&&await no.evaluate(e=>e===document.activeElement),`${id} Gallery native Tab and ArrowRight selection`,{galleryTabs});
   check(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${id} Gallery no page overflow`);await page.screenshot({path:`${output}/${id}-gallery.png`,fullPage:true});
   report.cells.push({id,requests,errorAssociations,channelAssociations,scheduleAX,focus,formData,reviewDescriptions,galleryDescription});
  }finally{if(context)await bound(context.close(),5000,`${id} context close`);}
 }
}catch(e){runError=e;report.errors.push({name:'run error',detail:{error:String(e),stack:e?.stack}});}
finally{
 try{if(browser)await bound(browser.close(),5000,'browser close');}catch(e){report.cleanup.browserCloseError=String(e);}try{if(server)await bound(server.close(),5000,'server close');}catch(e){report.cleanup.serverCloseError=String(e);}if(owned)report.cleanup.chrome={termination:await terminateOwned(owned,{graceMs:3000,killMs:2000,drainMs:2000}),lifecycle:owned.state};clearTimeout(watchdog);
 report.endedAt=new Date().toISOString();report.runError=runError?String(runError):null;report.summary={checks:report.checks.length,failures:report.errors.length,cells:report.cells.length,unexpectedRequests:report.unexpectedRequests.length};report.pass=!runError&&!report.timedOut&&report.errors.length===0&&report.unexpectedRequests.length===0&&report.cells.length===2&&report.cleanup.chrome?.termination?.ok&&!report.cleanup.browserCloseError&&!report.cleanup.serverCloseError;await fs.writeFile(`${output}/report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx'});console.log(JSON.stringify({pass:report.pass,summary:report.summary,cleanup:report.cleanup.chrome?.termination}));process.exitCode=report.pass?0:1;
}
