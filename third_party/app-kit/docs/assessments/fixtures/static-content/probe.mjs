#!/usr/bin/env node
import { createHash } from 'node:crypto';
import { spawn } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';
import net from 'node:net';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const sha256 = b => createHash('sha256').update(b).digest('hex');

const finite = value => Number.isFinite(value) && value >= 0;
export function assessAllowedRequest(origin, rawURL, method = 'GET') {
  const failures = [];
  let url;
  try { url = new URL(rawURL); } catch { return ['request.invalid-url']; }
  if (method !== 'GET') failures.push('request.method');
  if (url.origin !== origin) failures.push('request.origin');
  const paths = new Set(['/content', '/content/choosing-a-profile', '/content/not-a-page', '/static/app.css']);
  if (!paths.has(url.pathname)) failures.push('request.path');
  if (url.pathname === '/static/app.css' && url.search) failures.push('css.query');
  const allowed = new Set(['theme', 'state']);
  for (const key of url.searchParams.keys()) if (!allowed.has(key)) failures.push(`query.${key}`);
  for (const key of allowed) {
    const values = url.searchParams.getAll(key);
    if (values.length > 1) failures.push(`query.${key}.duplicate`);
  }
  const theme = url.searchParams.get('theme');
  if (theme !== null && !['light', 'dark'].includes(theme)) failures.push('query.theme.value');
  const state = url.searchParams.get('state');
  if (state !== null && !['ready', 'empty', 'unavailable'].includes(state)) failures.push('query.state.value');
  if (url.pathname !== '/content' && state !== null) failures.push('query.state.path');
  return failures;
}

export function assessObservation(contract, observation, expectedCSS) {
  const failures = [];
  if (observation.status !== observation.expectedStatus) failures.push('status');
  if (observation.taskPassed !== true) failures.push('task');
  if (observation.scriptNodes !== 0 || observation.scriptAttempts !== 0) failures.push('script-free');
  if (!Array.isArray(observation.deniedRequests) || observation.deniedRequests.length) failures.push('network');
  if (!Array.isArray(observation.resources) || !observation.resources.length) failures.push('resources');
  const resources = observation.resources ?? [];
  for (const resource of resources) {
    let url;
    try { url = new URL(resource.url); } catch { failures.push('resource.url'); continue; }
    if (url.origin !== observation.origin || resource.method !== 'GET') failures.push('resource.request');
    if (!['document', 'stylesheet'].includes(resource.kind)) failures.push('resource.kind');
    if (resource.fromCache !== false) failures.push('resource.cache');
    if (resource.terminal !== true) failures.push('resource.terminal');
    if (resource.bodyAvailable !== true || !Number.isInteger(resource.decodedBytes) || resource.decodedBytes < 0) failures.push('resource.body');
  }
  const css = resources.filter(r => { try { return new URL(r.url).pathname === '/static/app.css'; } catch { return false; } });
  if (css.length !== 1 || css[0]?.status !== 200 || css[0]?.kind !== 'stylesheet' || css[0]?.sha256 !== expectedCSS.sha256 || css[0]?.decodedBytes !== expectedCSS.bytes) failures.push('css.identity');
  const documents = resources.filter(r => r.kind === 'document');
  if (documents.length !== 1 || documents[0]?.url !== observation.url || documents[0]?.status !== observation.expectedStatus) failures.push('document.identity');
  if (documents[0]?.sha256 !== observation.html?.sha256 || documents[0]?.decodedBytes !== observation.html?.bytes) failures.push('document.artifact');
  const total = resources.reduce((n, r) => n + (Number.isInteger(r.decodedBytes) ? r.decodedBytes : 0), 0);
  if (total > contract.budgets.decoded_bytes) failures.push('budget.bytes');
  const t = observation.timings ?? {};
  for (const [key, ceiling] of [['responseEnd', contract.budgets.response_end_ms], ['domContentLoaded', contract.budgets.dom_content_loaded_ms], ['loadEnd', contract.budgets.load_end_ms]]) {
    if (!finite(t[key]) || t[key] <= 0) failures.push(`timing.${key}`);
    else if (t[key] > ceiling) failures.push(`budget.${key}`);
  }
  if (finite(t.responseEnd) && finite(t.domContentLoaded) && finite(t.loadEnd) && !(t.responseEnd <= t.domContentLoaded && t.domContentLoaded <= t.loadEnd)) failures.push('timing.order');
  return [...new Set(failures)];
}

export function assessCleanup(cleanup) {
  const failures = [];
  for (const name of ['browser', 'gallery']) {
    const p = cleanup?.[name];
    if (!p || p.exitCode !== 0 || p.signalCode !== null) failures.push(`cleanup.${name}`);
  }
  if (cleanup?.portRefused !== true) failures.push('cleanup.port');
  return failures;
}

export function isConnectionRefused(error) {
  return error?.code === 'ECONNREFUSED';
}

export function assessPreSubmit(fact, expectedState, expectedURL) {
  const failures = [];
  if (fact?.selected !== expectedState) failures.push('pre-submit.value');
  if (fact?.activeMatchesSubmit !== true) failures.push('pre-submit.focus');
  if (fact?.url !== expectedURL || fact?.navigationObserved !== false) failures.push('pre-submit.navigation');
  return failures;
}

export function validateContract(c) {
  if (c?.schema_version !== 'static-content-local-v1' || c.javascript !== false || c.cache !== 'disabled' || c.network !== 'unthrottled-owned-loopback') throw new Error('unsupported contract');
  if (JSON.stringify(c.assets) !== JSON.stringify(['/static/app.css'])) throw new Error('contract assets changed');
  if (JSON.stringify(c.paths) !== JSON.stringify(['/content', '/content/choosing-a-profile', '/content/not-a-page'])) throw new Error('contract paths changed');
  if (!Array.isArray(c.lanes) || c.lanes.length !== 4) throw new Error('contract lanes changed');
  const expectedBudgets = {decoded_bytes:131072,response_end_ms:100,dom_content_loaded_ms:250,load_end_ms:500};
  if (JSON.stringify(c.budgets) !== JSON.stringify(expectedBudgets)) throw new Error('contract budgets changed');
  const seen = new Set();
  for (const lane of c.lanes) {
    if (![390, 1440].includes(lane.width) || lane.height !== 1000 || !['light', 'dark'].includes(lane.theme)) throw new Error('invalid lane');
    seen.add(`${lane.width}:${lane.theme}`);
  }
  if (seen.size !== 4) throw new Error('contract lanes are not unique');
  return c;
}

async function main() {
  const [, , evidenceParent, binary, css, contractPath] = process.argv;
  if (![evidenceParent, binary, css, contractPath].every(Boolean)) throw new Error('usage: probe.mjs EVIDENCE_PARENT BINARY CSS CONTRACT_JSON');
  for (const value of [evidenceParent,binary,css,contractPath]) if (!path.isAbsolute(value)) throw new Error('all paths must be absolute');
  const parent=await fs.realpath(evidenceParent), source=fileURLToPath(import.meta.url);
  const inputs=await Promise.all([source,binary,css,contractPath].map(p=>fs.readFile(p)));
  validateContract(JSON.parse(inputs[3]));
  const root=await fs.mkdtemp(path.join(parent,'static-content-')); await fs.chmod(root,0o700);
  const names=['probe.mjs','gallery','app.css','contract.json'];
  for(let i=0;i<names.length;i++) await fs.writeFile(path.join(root,names[i]),inputs[i],{flag:'wx',mode:names[i]==='gallery'?0o700:0o600});
  const child=spawn(process.execPath,[path.join(root,'probe.mjs'),'--execute',root],{stdio:'inherit',env:process.env});
  process.exitCode=await new Promise((resolve,reject)=>{child.once('error',reject);child.once('exit',c=>resolve(c??1));});
}

async function execute(root) {
  const contract=validateContract(JSON.parse(await fs.readFile(path.join(root,'contract.json'),'utf8')));
  const cssBytes=await fs.readFile(path.join(root,'app.css')); const expectedCSS={bytes:cssBytes.length,sha256:sha256(cssBytes)};
  const pw=process.env.APPKIT_PLAYWRIGHT_MODULE;
  if(!pw||!path.isAbsolute(pw)) throw Error('APPKIT_PLAYWRIGHT_MODULE must be absolute');
  const pwBytes=await fs.readFile(pw); if(sha256(pwBytes)!=='a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f') throw Error('unapproved Playwright entry');
  const {chromium}=await import(pathToFileURL(pw));
  const runConfig={schema_version:'static-content-run-v1',contract,chrome:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',playwrightEntry:{path:pw,sha256:sha256(pwBytes)},serverArgs:['-addr','127.0.0.1:0','-design-assets','']};
  const configBytes=Buffer.from(JSON.stringify(runConfig,null,2)+'\n');await fs.writeFile(path.join(root,'run-config.json'),configBytes,{flag:'wx'});
  const report={schema_version:'static-content-browser-v1',root,started:new Date().toISOString(),inputs:{harness:sha256(await fs.readFile(fileURLToPath(import.meta.url))),binary:sha256(await fs.readFile(path.join(root,'gallery'))),css:expectedCSS,contract:sha256(await fs.readFile(path.join(root,'contract.json')))},observations:[],requests:[],failures:[],captures:[]};
  report.inputs.config=sha256(configBytes);
  let gallery,browserServer,browser,origin,port; let stderr='',stdout='';
  const fail=(name,detail={})=>report.failures.push({name,detail});
  try {
    gallery=spawn(path.join(root,'gallery'),['-addr','127.0.0.1:0','-design-assets',''],{cwd:root,stdio:['ignore','pipe','pipe']});
    gallery.stdout.on('data',b=>{stdout+=b;});
    origin=await new Promise((resolve,reject)=>{const timer=setTimeout(()=>reject(Error('gallery readiness timeout')),15000);gallery.once('error',reject);gallery.stderr.on('data',b=>{stderr+=b;const m=stderr.match(/app-kit Gallery → (http:\/\/127\.0\.0\.1:(\d+))/);if(m){clearTimeout(timer);port=Number(m[2]);resolve(m[1]);}});});
    browserServer=await chromium.launchServer({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',args:['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1','--disable-features=BackForwardCache']});
    browser=await chromium.connect(browserServer.wsEndpoint());
    report.chrome={path:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',version:browser.version(),playwrightEntry:{path:pw,sha256:sha256(pwBytes)}};
    for(const lane of contract.lanes){
      const context=await browser.newContext({viewport:{width:lane.width,height:lane.height},colorScheme:lane.theme,reducedMotion:'reduce',javaScriptEnabled:false,serviceWorkers:'block'});
      const page=await context.newPage(); page.setDefaultTimeout(7000); const cdp=await context.newCDPSession(page);await cdp.send('Network.enable');await cdp.send('Network.setCacheDisabled',{cacheDisabled:true});
      let active=null;
      const cdpURLs=new Map();cdp.on('Network.requestWillBeSent',e=>cdpURLs.set(e.requestId,e.request.url));cdp.on('Network.requestServedFromCache',e=>{const url=cdpURLs.get(e.requestId);if(url)active?.cachedURLs.add(url);});cdp.on('Network.responseReceived',e=>{if(e.response.fromDiskCache||e.response.fromPrefetchCache||e.response.fromServiceWorker)active?.cachedURLs.add(e.response.url);});
      await context.route('**/*',route=>{const q=route.request(),denied=assessAllowedRequest(origin,q.url(),q.method());report.requests.push({lane,method:q.method(),url:q.url(),admitted:!denied.length});if(denied.length){const fact={url:q.url(),method:q.method(),denied};active?.deniedRequests.push(fact);fail('forbidden-request',fact);return route.abort();}return route.continue();});
      page.on('request',q=>{if(active)active.pending.set(q,{url:q.url(),method:q.method(),kind:q.resourceType()==='document'?'document':q.resourceType()==='stylesheet'?'stylesheet':q.resourceType(),fromCache:false,terminal:false,bodyAvailable:false});});
      page.on('response',response=>{if(!active)return;const current=active,r=current.pending.get(response.request());if(!r)return;current.settlements.push((async()=>{r.status=response.status();r.fromCache=response.fromServiceWorker()||current.cachedURLs.has(response.url());try{const body=await response.body();r.decodedBytes=body.length;r.sha256=sha256(body);r.bodyAvailable=true;}catch(e){r.bodyError=String(e);}r.terminal=true;})());});
      page.on('requestfailed',q=>{const r=active?.pending.get(q);if(r){r.failure=q.failure();r.terminal=true;}});
      async function visit(target,stage,status,task,action){
        active={deniedRequests:[],pending:new Map(),settlements:[],cachedURLs:new Set()};
        let response;
        if(action){
          const pending=page.waitForNavigation({waitUntil:'load'}).then(response=>({response}),error=>({error}));
          try{await action();}catch(error){await pending;throw error;}
          const navigation=await pending;if(navigation.error)throw navigation.error;response=navigation.response;
        }
        else response=await page.goto(origin+target,{waitUntil:'load'});
        await page.waitForLoadState('networkidle'); await Promise.all(active.settlements);await new Promise(resolve=>setTimeout(resolve,0));for(const resource of active.pending.values())if(active.cachedURLs.has(resource.url))resource.fromCache=true;
        const html=await response.body(),htmlFile=`${lane.width}-${lane.theme}-${stage}.html`;
        await fs.writeFile(path.join(root,htmlFile),html,{flag:'wx'});
        const timing=await page.evaluate(()=>{const n=performance.getEntriesByType('navigation')[0];return {responseEnd:n?.responseEnd,domContentLoaded:n?.domContentLoadedEventEnd,loadEnd:n?.loadEventEnd};});
        const facts=await task();
        const obs={lane,stage,origin,url:response?.url(),status:response?.status(),expectedStatus:status,html:{file:htmlFile,bytes:html.length,sha256:sha256(html)},preSubmit:active.preSubmit,taskPassed:facts.passed,taskFacts:facts,scriptNodes:await page.locator('script').count(),scriptAttempts:[...active.pending.values()].filter(r=>r.kind==='script').length,deniedRequests:active.deniedRequests,resources:[...active.pending.values()],timings:timing};
        obs.failures=assessObservation(contract,obs,expectedCSS);if(obs.failures.length)fail(stage,{lane,failures:obs.failures});report.observations.push(obs);
        const png=await page.screenshot({fullPage:true,animations:'disabled'}),name=`${lane.width}-${lane.theme}-${stage}.png`;await fs.writeFile(path.join(root,name),png,{flag:'wx'});report.captures.push({name,sha256:sha256(png)});active=null;
      }
      async function tabTo(selector){for(let i=0;i<120;i++){if(await page.evaluate(s=>document.activeElement?.matches(s),selector))return;await page.keyboard.press('Tab');}throw Error(`Tab did not reach ${selector}`);}
      async function focusFact(){return page.evaluate(()=>{const e=document.activeElement,s=getComputedStyle(e),r=e.getBoundingClientRect(),color=s.outlineColor;return {tag:e.tagName,id:e.id,outlineStyle:s.outlineStyle,outlineWidth:parseFloat(s.outlineWidth),outlineColor:color,rect:{x:r.x,y:r.y,width:r.width,height:r.height},viewport:{width:innerWidth,height:innerHeight},painted:s.outlineStyle!=='none'&&parseFloat(s.outlineWidth)>=2&&color!=='transparent'&&color!=='rgba(0, 0, 0, 0)'&&r.width>0&&r.height>0&&r.bottom>0&&r.top<innerHeight};});}
      async function paintedFocus(){return (await focusFact()).painted;}
      const theme=`theme=${lane.theme}`;
      const basic=async(expectedTheme=lane.theme)=>page.evaluate(({theme})=>{const root=document.documentElement,controls=[...document.querySelectorAll('main a,main button,main select,main summary')].filter(e=>e.checkVisibility());return {actualTheme:root.dataset.theme,expectedTheme:theme,passed:root.dataset.theme===theme&&document.querySelectorAll('main').length===1&&document.documentElement.scrollWidth<=innerWidth&&controls.every(e=>{const r=e.getBoundingClientRect();return r.width>=44&&r.height>=44;})};},{theme:expectedTheme});
      await visit(`/content?${theme}`,'ready',200,async()=>{const b=await basic();const hrefs=await page.locator('main a').evaluateAll(es=>es.map(e=>e.getAttribute('href')));return {...b,passed:b.passed&&await page.locator('#content-title').isVisible()&&await page.locator('#content-state').inputValue()==='ready'&&hrefs.includes(`/content/choosing-a-profile?${theme}`)&&hrefs.includes(`/catalog?${theme}`)&&hrefs.includes(`/quality?${theme}`)};});
      await tabTo('a[href="#start-with-task"]');const tocLinkFocus=await focusFact();if(!tocLinkFocus.painted)fail('toc-link-painted-focus',{lane,tocLinkFocus});let png=await page.screenshot({animations:'disabled'}),focusName=`${lane.width}-${lane.theme}-toc-link-focus.png`;await fs.writeFile(path.join(root,focusName),png,{flag:'wx'});report.captures.push({lane,stage:'toc-link-focus',name:focusName,sha256:sha256(png)});await page.keyboard.press('Enter');const tocTargetFocus=await focusFact();if(await page.evaluate(()=>location.hash)!=='#start-with-task'||!await page.locator('#start-with-task').evaluate(e=>e===document.activeElement)||!tocTargetFocus.painted)fail('toc-target-focus',{lane,tocTargetFocus});png=await page.screenshot({animations:'disabled'});focusName=`${lane.width}-${lane.theme}-toc-target-focus.png`;await fs.writeFile(path.join(root,focusName),png,{flag:'wx'});report.captures.push({lane,stage:'toc-target-focus',name:focusName,sha256:sha256(png)});
      await tabTo('#profile summary');if(!await paintedFocus())fail('faq-painted-focus',{lane});await page.keyboard.press('Enter');await page.locator('#source summary').click();if(!await page.locator('#profile').evaluate(e=>e.open)||!await page.locator('#source').evaluate(e=>e.open))fail('independent-faq',{lane});
      await visit(null,'article',200,async()=>{const b=await basic();return {...b,heading:await page.locator('#content-profile-title').textContent(),passed:b.passed&&await page.locator('#content-profile-title').textContent()==='Choosing a profile'};},async()=>{await tabTo('#profile a');if(!await paintedFocus())fail('article-link-painted-focus',{lane});await page.keyboard.press('Enter');});
      await visit(null,'native-back',200,async()=>{const b=await basic();return {...b,passed:b.passed&&await page.locator('#content-title').isVisible()};},async()=>{await page.goBack({waitUntil:'commit'});});
      async function chooseState(state,status,label,title,reason){await visit(null,state,status,async()=>{const b=await basic(),text=await page.locator('main').innerText(),resetHref=await page.getByRole('link',{name:'Return to the ready guide'}).getAttribute('href'),url=new URL(page.url());return {...b,title,reason,resetHref,query:{theme:url.searchParams.get('theme'),state:url.searchParams.get('state')},passed:b.passed&&url.searchParams.get('theme')===lane.theme&&url.searchParams.get('state')===state&&resetHref===`/content?${theme}`&&await page.locator('#content-state').inputValue()===state&&!await page.locator('#content-faq').count()&&text.includes(title)&&text.includes(reason)&&!text.includes('Questions before reuse')};},async()=>{
        await tabTo('#content-state');if(!await paintedFocus())fail('state-select-painted-focus',{lane,state});
        const startingURL=page.url();let navigationObserved=false;const listener=frame=>{if(frame===page.mainFrame())navigationObserved=true;};page.on('framenavigated',listener);
        for(const character of label){await page.keyboard.type(character);await page.waitForTimeout(20);if(page.url()!==startingURL||navigationObserved)break;}
        await page.keyboard.press('Tab');
        const selected=await page.locator('#content-state').inputValue(),activeMatchesSubmit=await page.evaluate(()=>document.activeElement?.matches('form[aria-label="Static content specimen state"] button')===true);
        active.preSubmit={state,label,selected,activeMatchesSubmit,url:page.url(),navigationObserved};page.off('framenavigated',listener);
        const preSubmitFailures=assessPreSubmit(active.preSubmit,state,startingURL);
        if(preSubmitFailures.length){const bytes=await page.screenshot({animations:'disabled'}),name=`${lane.width}-${lane.theme}-${state}-pre-submit-failure.png`;await fs.writeFile(path.join(root,name),bytes,{flag:'wx'});const capture={lane,stage:`${state}-pre-submit-failure`,name,sha256:sha256(bytes)};report.captures.push(capture);fail('state-pre-submit',{lane,...active.preSubmit,failures:preSubmitFailures,capture});throw Error(`native state pre-submit failed: ${preSubmitFailures.join(', ')}`);}
        if(!await paintedFocus())fail('state-submit-painted-focus',{lane,state});await page.keyboard.press('Enter');
      });}
      await chooseState('empty',200,'Synthetic empty state','No guide entries are available','synthetic empty specimen');
      const opposite=lane.theme==='light'?'dark':'light';
      await visit(null,'empty-theme-opposite',200,async()=>{const b=await basic(opposite),text=await page.locator('main').innerText();return {...b,passed:b.passed&&text.includes('synthetic empty specimen')&&await page.locator('#content-state').inputValue()==='empty'};},async()=>{await tabTo(`header a[href="/content?state=empty&theme=${opposite}"]`);await page.keyboard.press('Enter');});
      await visit(null,'empty-theme-return',200,async()=>{const b=await basic();return {...b,passed:b.passed&&await page.locator('#content-state').inputValue()==='empty'};},async()=>{await tabTo(`header a[href="/content?state=empty&theme=${lane.theme}"]`);await page.keyboard.press('Enter');});
      await visit(null,'empty-reset',200,async()=>{const b=await basic();return {...b,passed:b.passed&&await page.locator('#content-title').isVisible()&&await page.locator('#content-state').inputValue()==='ready'};},async()=>{await tabTo('main a[href="/content?theme='+lane.theme+'"]');if(!await paintedFocus())fail('reset-painted-focus',{lane});await page.keyboard.press('Enter');});
      await chooseState('unavailable',503,'Synthetic unavailable state','The guide is temporarily unavailable','synthetic unavailable specimen');
      await visit(null,'unavailable-theme-opposite',503,async()=>{const b=await basic(opposite),text=await page.locator('main').innerText();return {...b,passed:b.passed&&text.includes('synthetic unavailable specimen')&&await page.locator('#content-state').inputValue()==='unavailable'};},async()=>{await tabTo(`header a[href="/content?state=unavailable&theme=${opposite}"]`);await page.keyboard.press('Enter');});
      await visit(null,'unavailable-theme-return',503,async()=>{const b=await basic();return {...b,passed:b.passed&&await page.locator('#content-state').inputValue()==='unavailable'};},async()=>{await tabTo(`header a[href="/content?state=unavailable&theme=${lane.theme}"]`);await page.keyboard.press('Enter');});
      await visit(null,'unavailable-reset',200,async()=>{const b=await basic();return {...b,passed:b.passed&&await page.locator('#content-title').isVisible()};},async()=>{await tabTo(`main a[href="/content?theme=${lane.theme}"]`);await page.keyboard.press('Enter');});
      await visit(`/content/not-a-page?${theme}`,'unknown',404,async()=>{const b=await basic();return {...b,passed:b.passed&&await page.locator('#content-problem-title').isVisible()&&!await page.locator('#content-title').count()};});
      await visit(null,'recovered',200,async()=>{const b=await basic();return {...b,passed:b.passed&&await page.locator('#content-title').isVisible()};},async()=>{await tabTo('main a[href="/content?theme='+lane.theme+'"]');if(!await paintedFocus())fail('unknown-recovery-painted-focus',{lane});await page.keyboard.press('Enter');});
      await context.close();
    }
  } catch(e){fail('runtime',{error:String(e),stack:e.stack});}
  finally {
    const bounded=(promise,label)=>Promise.race([promise,new Promise((_,reject)=>setTimeout(()=>reject(Error(`${label} timeout`)),8000))]);
    const waitExit=async(child,label)=>{if(!child)return; if(child.exitCode!==null||child.signalCode!==null)return;await Promise.race([new Promise(resolve=>child.once('close',resolve)),new Promise((_,reject)=>setTimeout(()=>reject(Error(`${label} cleanup timeout`)),8000))]);};
    try{if(browser)await bounded(browser.close(),'browser close');if(browserServer)await bounded(browserServer.close(),'browser server close');await waitExit(browserServer?.process(),'browser');}catch(e){fail('cleanup.browser',{error:String(e)});try{browserServer?.process()?.kill('SIGKILL');}catch{}}
    try{if(gallery&&gallery.exitCode===null&&gallery.signalCode===null)gallery.kill('SIGTERM');await waitExit(gallery,'gallery');}catch(e){fail('cleanup.gallery',{error:String(e)});try{gallery?.kill('SIGKILL');}catch{}}
    let refused=false,refusalError=null;
    if(Number.isInteger(port))await new Promise(resolve=>{const s=net.connect({host:'127.0.0.1',port});const timer=setTimeout(()=>{refusalError={code:'TIMEOUT',message:'port refusal check timeout'};s.destroy();resolve();},3000);s.once('error',error=>{clearTimeout(timer);refusalError={code:error.code,message:error.message};refused=isConnectionRefused(error);resolve();});s.once('connect',()=>{clearTimeout(timer);s.destroy();resolve();});});
    else fail('cleanup.port-missing');
    await Promise.all([fs.writeFile(path.join(root,'gallery.stdout'),stdout,{flag:'wx'}),fs.writeFile(path.join(root,'gallery.stderr'),stderr,{flag:'wx'})]);
    report.logs={stdout:{file:'gallery.stdout',bytes:Buffer.byteLength(stdout),sha256:sha256(Buffer.from(stdout))},stderr:{file:'gallery.stderr',bytes:Buffer.byteLength(stderr),sha256:sha256(Buffer.from(stderr))}};
    report.cleanup={browser:{exitCode:browserServer?.process()?.exitCode,signalCode:browserServer?.process()?.signalCode??null},gallery:{exitCode:gallery?.exitCode,signalCode:gallery?.signalCode??null},portRefused:refused,refusalError};report.failures.push(...assessCleanup(report.cleanup).map(name=>({name})));report.finished=new Date().toISOString();await fs.writeFile(path.join(root,'receipt.json'),JSON.stringify(report,null,2),{flag:'wx'});
  }
  if(report.failures.length)process.exitCode=1;
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) (process.argv[2]==='--execute'?execute(process.argv[3]):main()).catch(error => { console.error(error.stack||error.message); process.exitCode = 1; });
