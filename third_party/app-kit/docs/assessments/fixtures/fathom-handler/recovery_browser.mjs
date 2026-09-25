// Separate opt-in recovery journey for the actual Fathom handlers.
// The runner must execute a frozen copy and pass a frozen contract.
import {createHash} from 'node:crypto';
import {readFile,writeFile,readdir,realpath} from 'node:fs/promises';
import {dirname,isAbsolute,join} from 'node:path';
import {pathToFileURL} from 'node:url';
import {configuration} from '../../../../apps/gallery/browser/config.mjs';

const sha256=bytes=>createHash('sha256').update(bytes).digest('hex');
const pause=ms=>new Promise(resolve=>setTimeout(resolve,ms));

export function assessRecovery(contract,facts){
  const failures=[];
  if(typeof facts.before?.unreadable!=='boolean'||typeof facts.before?.falseZero!=='boolean')failures.push('before.missing-fact');
  if(typeof facts.immediate?.unreadable!=='boolean'||typeof facts.immediate?.exactData!=='boolean')failures.push('immediate.missing-fact');
  if(typeof facts.after?.unreadable!=='boolean'||typeof facts.after?.falseZero!=='boolean'||typeof facts.after?.exactData!=='boolean')failures.push('after.missing-fact');
  if(facts.before?.unreadable!==true)failures.push('before.unreadable');
  if(facts.before?.falseZero!==false)failures.push('before.false-zero');
  if(facts.immediate?.unreadable!==true||facts.immediate?.exactData===true)failures.push('immediate.premature-recovery');
  if(facts.immediate?.cacheAtUnixMilli!==facts.warm?.cacheAtUnixMilli)failures.push('immediate.cache-replaced');
  if(facts.after?.unreadable!==false)failures.push('after.stale-failure');
  if(facts.after?.falseZero!==false)failures.push('after.false-zero');
  if(facts.after?.exactData!==true)failures.push('after.missing-data');
  const elapsed=facts.after?.recoveredAtUnixMilli-facts.warm?.cacheAtUnixMilli;
  if(!Number.isFinite(elapsed)||elapsed<contract.cacheTTLMillis||elapsed>contract.recoveryWaitMillis)failures.push('after.expiry-window');
  return failures;
}

export function validateContract(contract){
  if(contract?.version!==1)throw Error('Unsupported recovery contract');
  if(contract.cacheTTLMillis!==30000||contract.freshCacheMaxMillis!==3000||contract.refreshWaitMillis!==35000||contract.recoveryWaitMillis!==40000||contract.pollMillis!==500)throw Error('Recovery timing contract changed');
  if(!contract.service||contract.expected?.callsTotal!==3||contract.expected?.callsAgent!==2||contract.expected?.configuration!=='not inspectable')throw Error('Exact service and recovered values required');
  if(contract.control?.statusPath!=='/__appkit_recovery/status'||contract.control?.transitionPath!=='/__appkit_recovery/transition'||contract.control?.tokenHeader!=='X-Appkit-Recovery'||contract.control?.requestBody!=='recover\n'||contract.control?.contentType!=='text/plain')throw Error('Owned test-control contract changed');
  if(!Array.isArray(contract.lanes)||contract.lanes.length!==4)throw Error('Four exact lanes required');
  const lanes=new Set(contract.lanes.map(lane=>`${lane.theme}/${lane.width}/${lane.height}`));
  if(lanes.size!==4||!['light/390/1000','dark/390/1000','light/1440/1000','dark/1440/1000'].every(lane=>lanes.has(lane)))throw Error('Recovery lanes changed');
  return contract;
}

async function tabTo(page,target,label){
  if(await target.count()!==1)throw Error(`${label}: target must be unique`);
  for(let step=0;step<64;step++){
    if(await target.evaluate(element=>element===document.activeElement))return;
    await page.keyboard.press('Tab');
  }
  throw Error(`${label}: target not reachable by Tab`);
}

async function focusFact(target){
  return target.evaluate(element=>{const rect=element.getBoundingClientRect(),style=getComputedStyle(element);return{active:element===document.activeElement,width:rect.width,height:rect.height,outlineWidth:parseFloat(style.outlineWidth),outlineStyle:style.outlineStyle,outlineColor:style.outlineColor,visible:rect.width>0&&rect.height>0&&rect.top<innerHeight&&rect.bottom>0};});
}

async function assertFocused(target,label){
  const fact=await focusFact(target);
  if(!fact.active||!fact.visible||fact.width<44||fact.height<44||fact.outlineWidth<2||['none','hidden'].includes(fact.outlineStyle)||fact.outlineColor==='transparent')throw Error(`${label}: focus evidence failed ${JSON.stringify(fact)}`);
  return fact;
}

async function assertLongLabel(target,label){
  const fact=await target.evaluate(element=>{const rect=element.getBoundingClientRect(),main=document.querySelector('main').getBoundingClientRect();return{text:element.textContent.trim(),rect:{left:rect.left,right:rect.right,top:rect.top,bottom:rect.bottom,width:rect.width,height:rect.height},main:{left:main.left,right:main.right},contained:rect.left>=main.left-1&&rect.right<=main.right+1,noOverflow:document.documentElement.scrollWidth<=innerWidth};});
  if(!fact.contained||!fact.noOverflow||fact.rect.width<=0||fact.rect.height<=0)throw Error(`${label}: long label clipping ${JSON.stringify(fact)}`);
  return fact;
}

async function assertServiceHeadings(target,label){
  if(await target.count()!==2)throw Error(`${label}: expected section heading and detail-card heading`);
  return [await assertLongLabel(target.nth(0),`${label} section`),await assertLongLabel(target.nth(1),`${label} card`)];
}

async function openNav(page,width,label){
  if(width>=1024)return page.getByRole('navigation',{name:'Primary',exact:true});
  const menu=page.locator('summary').filter({hasText:/^Menu$/});await tabTo(page,menu,`${label} menu`);await assertFocused(menu,`${label} menu`);
  if(await menu.evaluate(element=>element.parentElement.open))throw Error(`${label}: mobile Menu began open`);
  await page.keyboard.press('Enter');
  if(!await menu.evaluate(element=>element.parentElement.open))throw Error(`${label}: mobile Menu did not open`);
  return page.getByRole('navigation',{name:'Primary (mobile)',exact:true});
}

async function follow(page,target,destination,label){
  await tabTo(page,target,label);await assertFocused(target,label);
  await Promise.all([page.waitForURL(new URL(destination,page.url()).href,{waitUntil:'load'}),page.keyboard.press('Enter')]);
  const activePath=destination.startsWith('/observatory')?'/observatory':'/';
  const facts=await documentFacts(page,activePath);
  if(facts.main!==1||facts.headings!==1||facts.scripts!==0||facts.overflow||facts.current.length!==2||facts.current.some(value=>value!==activePath))throw Error(`${label}: destination structure/current-route failed ${JSON.stringify({...facts,body:undefined})}`);
}

async function assertSkip(page,label){
  await page.keyboard.press('Tab');
  const skip=page.locator('a[href="#observatory-main"]');
  if(await skip.count()!==1||!await skip.evaluate(element=>element===document.activeElement))throw Error(`${label}: first Tab was not skip link`);
  await page.keyboard.press('Enter');
  if(!await page.locator('#observatory-main').evaluate(element=>element===document.activeElement))throw Error(`${label}: skip target did not receive focus`);
}

async function documentFacts(page,activePath){
  return page.evaluate(activePath=>({main:document.querySelectorAll('main#observatory-main[tabindex="-1"]').length,headings:document.querySelectorAll('h1').length,scripts:document.scripts.length,current:[...document.querySelectorAll('nav a[aria-current="page"]')].map(element=>element.getAttribute('href')),overflow:document.documentElement.scrollWidth>innerWidth,background:getComputedStyle(document.body).backgroundColor,body:document.body.innerText}),activePath);
}

function stateFacts(contract,document){
  const body=document.body;
  const unreadable=body.includes('call log unreadable:')&&body.includes('traffic figures are not measured, never zero');
  const falseZero=body.includes('0 calls')||body.includes('0 agent');
  const exactData=body.includes(`${contract.expected.callsTotal} calls (${contract.expected.callsAgent} agent)`)&&!unreadable;
  return{unreadable,falseZero,exactData};
}

async function control(origin,path,token,options={}){
  const headers={Origin:origin,'X-Appkit-Recovery':token,...options.headers};
  const response=await fetch(new URL(path,origin),{redirect:'error',...options,headers});
  const text=await response.text();let body;try{body=JSON.parse(text);}catch{throw Error(`Control ${path} returned non-JSON`);}
  if(!response.ok)throw Error(`Control ${path} returned ${response.status}: ${text}`);
  return{status:response.status,body};
}

async function main(){
  const[origin,output,modulePath,contractPath,token]=process.argv.slice(2);
  if(!origin||!output||!modulePath||!contractPath||![output,modulePath,contractPath].every(isAbsolute)||!/^[0-9a-f]{64}$/.test(token))throw Error('Expected origin, absolute output/Playwright/contract paths, and a 64-hex token');
  const root=await realpath(output),reportRoot=await realpath(process.env.APPKIT_FATHOM_HANDLER_REPORT_ROOT||'');
  if(dirname(root)!==reportRoot||(await readdir(root)).length)throw Error('Report must be a fresh empty child of the explicit report root');
  const contractBytes=await readFile(contractPath),contract=validateContract(JSON.parse(contractBytes));
  const moduleBytes=await readFile(modulePath);
  const browserConfig=configuration({APPKIT_SHELL_BROWSER_ORIGIN:origin,APPKIT_PLAYWRIGHT_MODULE:modulePath});
  const{chromium}=await import(browserConfig.moduleURL);
  const contractSHA256=sha256(contractBytes);
  const report={schema_version:'fathom-shell-recovery-browser-v1',contractSHA256,origin,started_at:new Date().toISOString(),inputs:{browser:{sha256:sha256(await readFile(new URL(import.meta.url)))},contract:{sha256:contractSHA256},playwright:{sha256:sha256(moduleBytes)}},requests:[],blocked:[],pageErrors:[],consoleErrors:[],cases:[],controls:[],failures:[],screenshots:[],captures:[],limits:['Synthetic same-server fixture recovery through actual handlers and cache policy.','All-time counts only; fixed timestamps do not prove freshness or rolling-window counts.','Not live-daemon, mutation, complete accessibility, performance, native-host or production evidence.']};
  let browser;const lanes=[];
  try{
    browser=await chromium.launch({executablePath:browserConfig.executablePath,headless:true,args:['--no-proxy-server','--disable-background-networking']});report.browser=browser.version();
    const serviceHref='/observatory/service?id='+contract.service;
    const browserRoutes=new Set(['/','/observatory','/assets/app.css',serviceHref]);
    for(const laneContract of contract.lanes){const{theme,width,height}=laneContract;
      const tag=`${theme}/${width}`,context=await browser.newContext({viewport:{width,height},colorScheme:theme,reducedMotion:'reduce',deviceScaleFactor:1,javaScriptEnabled:false,serviceWorkers:'block',acceptDownloads:false});
      await context.route('**/*',route=>{const request=route.request(),url=new URL(request.url()),key=url.pathname+url.search,allowed=request.method()==='GET'&&url.origin===origin&&browserRoutes.has(key);report.requests.push({tag,method:request.method(),path:key,allowed});if(!allowed)report.blocked.push({tag,method:request.method(),url:request.url()});return allowed?route.continue():route.abort();});
      await context.routeWebSocket('**/*',socket=>{report.blocked.push({tag,method:'WebSocket',url:socket.url()});socket.close();});
      const page=await context.newPage();page.setDefaultTimeout(5000);page.on('pageerror',error=>report.pageErrors.push({tag,error:error.message}));page.on('console',message=>{if(message.type()==='error')report.consoleErrors.push({tag,error:message.text()});});page.on('dialog',async dialog=>{report.pageErrors.push({tag,error:`unexpected ${dialog.type()} dialog`});await dialog.dismiss();});const record={tag,theme,width,height,before:{},immediate:null,after:null};const lane={tag,theme,width,height,context,page,record,before:record.before};lanes.push(lane);report.cases.push(record);
      let response=await page.goto(origin+'/',{waitUntil:'load'});if(response.status()!==200)throw Error(`${tag}: landing status ${response.status()}`);
      await page.keyboard.press('Tab');const skip=page.locator('a[href="#observatory-main"]');if(!await skip.evaluate(element=>element===document.activeElement))throw Error(`${tag}: first Tab was not skip link`);await page.keyboard.press('Enter');if(!await page.locator('#observatory-main').evaluate(element=>element===document.activeElement))throw Error(`${tag}: skip target did not receive focus`);
      await page.goto(origin+'/',{waitUntil:'load'});const navigation=await openNav(page,width,`${tag} home`);await follow(page,navigation.locator('a[href="/observatory"]'),'/observatory',`${tag} fleet`);
      const fleet=await documentFacts(page,'/observatory');if(fleet.main!==1||fleet.headings!==1||fleet.scripts!==0||fleet.overflow||fleet.current.length!==2||fleet.current.some(value=>value!=='/observatory'))throw Error(`${tag}: fleet structure failed`);
      Object.assign(lane.before,stateFacts(contract,fleet),{background:fleet.background});if(!lane.before.unreadable||lane.before.falseZero)throw Error(`${tag}: initial unreadable truth failed`);
      const heading=page.getByRole('heading',{level:2,name:contract.service,exact:true}),section=heading.locator('xpath=ancestor::section[1]'),link=section.locator(`a[href="${serviceHref}"]`);record.before.longLabel=await assertLongLabel(heading,`${tag} long service`);const beforeFleet=`recovery-${theme}-${width}-before-fleet.png`;await page.screenshot({path:join(root,beforeFleet),fullPage:true});report.captures.push({file:beforeFleet,sha256:sha256(await readFile(join(root,beforeFleet))),phase:'before-fleet',theme,width});await follow(page,link,serviceHref,`${tag} long service detail`);const service=await documentFacts(page,'/observatory'),serviceHeading=page.getByRole('heading',{level:2,name:contract.service,exact:true});record.before.serviceHeading=await assertServiceHeadings(serviceHeading,`${tag} service heading`);if(!service.body.includes(contract.service))throw Error(`${tag}: long service label absent`);const beforeService=`recovery-${theme}-${width}-before-service.png`;await page.screenshot({path:join(root,beforeService),fullPage:true});report.captures.push({file:beforeService,sha256:sha256(await readFile(join(root,beforeService))),phase:'before-service',theme,width});
      const serviceNav=await openNav(page,width,`${tag} service`);await follow(page,serviceNav.locator('a[href="/observatory"]'),'/observatory',`${tag} return fleet`);
    }
    const warmDeadline=Date.now()+contract.refreshWaitMillis;let warm;
    while(Date.now()<warmDeadline){await lanes[0].page.reload({waitUntil:'load'});const status=await control(origin,contract.control.statusPath,token);report.controls.push({kind:'status',at:Date.now(),...status});warm=status.body;if(Number.isFinite(warm.cacheAgeMillis)&&warm.cacheAgeMillis>=0&&warm.cacheAgeMillis<contract.freshCacheMaxMillis)break;const remaining=contract.cacheTTLMillis-warm.cacheAgeMillis;if(Number.isFinite(remaining)&&remaining>0)await pause(Math.min(remaining+50,Math.max(0,warmDeadline-Date.now())));else await pause(contract.pollMillis);}
    if(!warm||warm.cacheAgeMillis>=contract.freshCacheMaxMillis||warm.transitioned)throw Error('Could not observe a fresh unreadable cache before transition');
    const transition=await control(origin,contract.control.transitionPath,token,{method:'POST',headers:{'content-type':contract.control.contentType},body:contract.control.requestBody});report.controls.push({kind:'transition',at:Date.now(),...transition});if(!transition.body.transitioned||transition.body.cacheAtUnixMilli!==warm.cacheAtUnixMilli)throw Error('Transition did not preserve the warm cache identity');
    for(const lane of lanes){await lane.page.reload({waitUntil:'load'});const document=await documentFacts(lane.page,'/observatory');lane.immediate={...stateFacts(contract,document),...(await control(origin,contract.control.statusPath,token)).body};lane.record.immediate=lane.immediate;const file=`recovery-${lane.theme}-${lane.width}-immediate.png`;await lane.page.screenshot({path:join(root,file),fullPage:true});report.captures.push({file,sha256:sha256(await readFile(join(root,file))),phase:'immediate',theme:lane.theme,width:lane.width});if(!lane.immediate.transitioned||!lane.immediate.unreadable||lane.immediate.exactData||lane.immediate.cacheAtUnixMilli!==warm.cacheAtUnixMilli)throw Error(`${lane.tag}: immediate stale-cache assertion failed`);}
    let recovered=false,recoveredAt;await pause(Math.max(0,warm.cacheAtUnixMilli+contract.cacheTTLMillis-Date.now()));
    const recoveryDeadline=warm.cacheAtUnixMilli+contract.recoveryWaitMillis;
    while(Date.now()<=recoveryDeadline){recovered=true;for(const lane of lanes){await lane.page.reload({waitUntil:'load'});const document=await documentFacts(lane.page,'/observatory'),facts=stateFacts(contract,document);if(facts.unreadable||!facts.exactData){recovered=false;break;}}if(recovered){recoveredAt=Date.now();break;}await pause(contract.pollMillis);}
    if(!recovered)throw Error('Recovered data did not appear by the natural-expiry deadline');const recoveredStatus=(await control(origin,contract.control.statusPath,token)).body;report.controls.push({kind:'recovered-status',at:Date.now(),body:recoveredStatus});if(!recoveredStatus.transitioned||recoveredStatus.cacheAtUnixMilli<=warm.cacheAtUnixMilli)throw Error('Recovered cache identity did not advance after natural expiry');
    for(const lane of lanes){const document=await documentFacts(lane.page,'/observatory');lane.after={...stateFacts(contract,document),recoveredAtUnixMilli:recoveredAt};lane.record.after=lane.after;const failures=assessRecovery(contract,{before:lane.before,warm,immediate:lane.immediate,after:lane.after});if(failures.length)throw Error(`${lane.tag}: ${failures.join(', ')}`);const heading=lane.page.getByRole('heading',{level:2,name:contract.service,exact:true}),section=heading.locator('xpath=ancestor::section[1]'),service=section.locator(`a[href="${serviceHref}"]`);lane.record.after.longLabel=await assertLongLabel(heading,`${lane.tag} recovered long service`);await follow(lane.page,service,serviceHref,`${lane.tag} recovered long service detail`);const serviceDocument=await documentFacts(lane.page,'/observatory'),serviceHeading=lane.page.getByRole('heading',{level:2,name:contract.service,exact:true}),agentShare=lane.page.locator('#service-agent-share');lane.record.after.serviceHeading=await assertServiceHeadings(serviceHeading,`${lane.tag} recovered service heading`);if(!serviceDocument.body.includes(contract.service)||serviceDocument.body.includes('call log unreadable:')||!serviceDocument.body.includes('3 calls (2 agent · 2/3)')||!serviceDocument.body.includes(contract.expected.configuration)||await agentShare.count()!==1||!(await agentShare.innerText()).includes('2 of 3 recorded calls; current local call-log snapshot'))throw Error(`${lane.tag}: recovered service truth failed`);const afterService=`recovery-${lane.theme}-${lane.width}-after-service.png`;await lane.page.screenshot({path:join(root,afterService),fullPage:true});report.captures.push({file:afterService,sha256:sha256(await readFile(join(root,afterService))),phase:'after-service',theme:lane.theme,width:lane.width});const serviceNav=await openNav(lane.page,lane.width,`${lane.tag} recovered service`);await follow(lane.page,serviceNav.locator('a[href="/"]'),'/',`${lane.tag} recovered home`);await follow(lane.page,(await openNav(lane.page,lane.width,`${lane.tag} recovered home nav`)).locator('a[href="/observatory"]'),'/observatory',`${lane.tag} recovered fleet return`);await assertSkip(lane.page,`${lane.tag} recovered skip`);const file=`recovery-${lane.theme}-${lane.width}-after-fleet.png`;await lane.page.screenshot({path:join(root,file),fullPage:true});const shot={file,sha256:sha256(await readFile(join(root,file)))};report.screenshots.push(shot);report.captures.push({...shot,phase:'after-fleet',theme:lane.theme,width:lane.width});}
    for(const width of[390,1440]){const light=lanes.find(lane=>lane.width===width&&lane.theme==='light')?.before.background,dark=lanes.find(lane=>lane.width===width&&lane.theme==='dark')?.before.background;if(!light||!dark||light===dark)throw Error(`Actual theme colors did not differ at ${width}`);}
    if(report.blocked.length||report.pageErrors.length||report.consoleErrors.length)throw Error('Unexpected traffic or browser errors were recorded');
  }catch(error){report.failures.push({error:String(error),stack:error.stack});process.exitCode=1;for(const lane of lanes)try{const file=`failure-${lane.theme}-${lane.width}.png`;await lane.page.screenshot({path:join(root,file),fullPage:true});report.captures.push({file,sha256:sha256(await readFile(join(root,file))),phase:'failure',theme:lane.theme,width:lane.width});}catch(captureError){report.failures.push({capture:lane.tag,error:String(captureError)});}}
  finally{for(const lane of lanes)try{await lane.context.close();}catch(error){report.failures.push({cleanup:lane.tag,error:String(error)});}if(browser)await browser.close();report.finished_at=new Date().toISOString();await writeFile(join(root,'browser-report.json'),JSON.stringify(report,null,2)+'\n',{flag:'wx',mode:0o600});console.log(JSON.stringify({passed:report.failures.length===0,cases:report.cases.length,checks:report.cases.length*2,failures:report.failures}));}
}

if(import.meta.url===pathToFileURL(process.argv[1]??'').href)main().catch(error=>{console.error(error.stack||error);process.exitCode=1;});
