// Opt-in native history probe. Never dispatches pageshow or edits application DOM.
import fs from 'node:fs';
import {bootstrap} from './config.mjs';

const {base, variant, root, chromium, executablePath} = await bootstrap();
const report = {base, variant, startedAt:new Date().toISOString(), passes:[], failures:[], lanes:[], blocked:[], pageErrors:[], limits:[
  'Synthetic local sessions only. CDP distinguishes actual BFCache restoration from network history; no cache headers are relaxed.',
  'Playwright native navigation and fill/select actions, not complete keyboard, screen-reader, native-host or cross-browser certification.',
]};
const check = (ok, name, detail) => (ok ? report.passes : report.failures).push({name, detail});
const save = page => page.locator('[data-shell-save]');
const version = page => page.locator('form[data-shell-edit] [name=version]').inputValue();
const routePath = kind => `/shell/${kind}`;
const equal = (a,b) => JSON.stringify(a)===JSON.stringify(b);
async function reveal(page, locator) {
  await locator.scrollIntoViewIfNeeded();
  // Native scrolling is needed when automatic scroll-into-view leaves a control
  // behind fixed mobile navigation. Never force clicks through an overlay.
  for(let attempt=0;attempt<4;attempt++) {
    const position=await locator.evaluate(n=>{
      const r=n.getBoundingClientRect(),nav=document.querySelector('[aria-label="Quick navigation"]'),nr=nav?.getBoundingClientRect();
      const bottom=nr && nr.height && nr.top<innerHeight ? nr.top : innerHeight;
      const hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);
      return {clear:!!hit && (hit===n || n.contains(hit)),top:r.top,bottom:r.bottom,limit:bottom};
    });
    if(position.clear && position.top>=0 && position.bottom<=position.limit) return;
    const delta=position.bottom>position.limit ? position.bottom-position.limit+32 : position.top-32;
    await page.mouse.move(100,200); await page.mouse.wheel(0,delta);
    // Poll from the test process: requestAnimationFrame polling inside a script-
    // disabled page does not advance, even though native scrolling succeeded.
    await new Promise(resolve=>setTimeout(resolve,50));
  }
  throw new Error('Native scrolling could not expose the control');
}
const sample = (kind, phase) => kind==='profile' ? {submitted:'History Avery',unsent:'Unsent history draft',newer:'Newer history value',reentered:'Reentered history draft'}[phase] : {
  submitted:{timezone:'UTC',start_page:'activity',digest:false}, unsent:{timezone:'Europe/London',start_page:'home',digest:true},
  newer:{timezone:'America/Phoenix',start_page:'activity',digest:false}, reentered:{timezone:'Europe/London',start_page:'home',digest:true},
}[phase];
async function value(page,kind) {
  return kind==='profile' ? page.locator('#shell-display_name').inputValue() : {timezone:await page.locator('#shell-timezone').inputValue(),start_page:await page.locator('#shell-start_page').inputValue(),digest:await page.locator('#shell-digest').isChecked()};
}
async function edit(page,kind,next) {
  if(kind==='profile') return page.locator('#shell-display_name').fill(next);
  await page.locator('#shell-timezone').selectOption(next.timezone);
  await page.locator('#shell-start_page').selectOption(next.start_page);
  const digest=page.locator('#shell-digest'); await reveal(page,digest); await digest.setChecked(next.digest);
}
async function state(page, kind) {
  return {value:await value(page,kind), version:await version(page), status:await page.locator('[data-shell-draft-status]').innerText(), disabled:await save(page).isDisabled(), busy:await page.locator('form[data-shell-edit]').getAttribute('aria-busy'), navigationType:await page.evaluate(()=>performance.getEntriesByType('navigation')[0]?.type)};
}
async function submit(page, lane, expected) {
  await reveal(page,save(page));
  const [response] = await Promise.all([page.waitForResponse(r=>r.request().method()==='POST' && new URL(r.url()).pathname==='/shell/action'), save(page).click()]);
  await page.waitForLoadState('load');
  lane.postStatuses.push(response.status());
  check(response.status()===expected,`${lane.name}: explicit submission ${expected}`,response.status());
  if (response.status()!==expected) throw new Error(`expected ${expected}, got ${response.status()}`);
}
async function travel(page, cdp, lane, direction, kind) {
  const start=lane.navigation.length, reads=lane.requests.filter(r=>r.method==='GET' && r.path===routePath(kind)).length;
  // A real BFCache restore has no new load event. Wait for the browser's main-frame
  // navigation event instead of Playwright's network-load history heuristic.
  const history=await cdp.send('Page.getNavigationHistory');
  const entry=history.entries[history.currentIndex+(direction==='back'?-1:1)];
  if(!entry) throw new Error(`no ${direction} history entry`);
  let timer, listener;
  const navigated=new Promise((resolve,reject)=>{
    listener=event=>{if(!event.frame.parentId) resolve(event);};
    cdp.on('Page.frameNavigated',listener);
    timer=setTimeout(()=>reject(new Error('history main-frame navigation timed out')),5000);
  });
  try {await Promise.all([navigated,cdp.send('Page.navigateToHistoryEntry',{entryId:entry.id})]);}
  finally {clearTimeout(timer);cdp.off('Page.frameNavigated',listener);}
  await page.locator('form[data-shell-edit]').waitFor();
  const frames=lane.navigation.slice(start);
  if(!frames.some(f=>f.type==='BackForwardCacheRestore')) await page.waitForLoadState('load');
  const current=await state(page,kind);
  const restores=frames.filter(f=>f.type==='BackForwardCacheRestore');
  const result={direction,frames,restored:restores.length===1, reads:lane.requests.filter(r=>r.method==='GET' && r.path===routePath(kind)).length-reads,...current};
  lane.history.push(result);
  return result;
}

for (const cache of ['disabled','enabled']) {
  const browser = await chromium.launch({headless:true, executablePath, ignoreDefaultArgs:cache==='enabled'?['--disable-back-forward-cache','--disable-prompt-on-repost']:['--disable-prompt-on-repost']});
  report.browser=browser.version();
  try {
    for (const kind of ['profile','settings']) for(const scripts of [true,false]) for(const width of [390,1440]) for(const theme of ['light','dark']) {
      const lane={name:`${cache}-${kind}-${scripts?'scripts':'no-script'}-${width}-${theme}`,cache,kind,scripts,width,theme,postStatuses:[],requests:[],navigation:[],notRestored:[],history:[],dialogs:[],console:[]}; report.lanes.push(lane);
      if(process.env.APPKIT_HISTORY_LANE && lane.name!==process.env.APPKIT_HISTORY_LANE) {report.lanes.pop();continue;}
      const context=await browser.newContext({javaScriptEnabled:scripts,viewport:{width,height:1000},colorScheme:theme,serviceWorkers:'block'});
      context.setDefaultTimeout(5000);
      await context.route('**/*',route=>{
        const req=route.request(),url=new URL(req.url());
        const allowed=url.origin===base && !url.search && ((req.method()==='GET' && ['/shell/profile','/shell/settings','/shell/help','/static/app.css','/static/shell-demo.js'].includes(url.pathname)) || (req.method()==='POST' && url.pathname==='/shell/action'));
        if(!allowed) {report.blocked.push({method:req.method(),url:req.url()}); return route.abort();}
        return route.continue();
      });
      await context.routeWebSocket('**/*',ws=>{report.blocked.push({websocket:ws.url()});ws.close();});
      context.on('request',req=>{const url=new URL(req.url()); if(url.origin===base) lane.requests.push({method:req.method(),path:url.pathname});});
      const page=await context.newPage(),cdp=await context.newCDPSession(page);
      await cdp.send('Page.enable');
      cdp.on('Page.frameNavigated',event=>{if(!event.frame.parentId) lane.navigation.push({type:event.type,path:new URL(event.frame.url).pathname});});
      cdp.on('Page.backForwardCacheNotUsed',event=>lane.notRestored.push(event.notRestoredExplanations.map(e=>({type:e.type,reason:e.reason}))));
      let dialogDecision='dismiss';
      page.on('dialog',async dialog=>{lane.dialogs.push({type:dialog.type(),decision:dialogDecision});await dialog[dialogDecision]();});
      page.on('pageerror',error=>report.pageErrors.push(String(error)));
      page.on('console',message=>{if(message.type()==='error') lane.console.push({text:message.text(),url:message.location().url});});
      try {
        const response=await page.goto(base+routePath(kind));
        check(response.status()===200 && response.headers()['cache-control']==='no-store',`${lane.name}: unchanged native no-store page`);
        lane.environment=await page.evaluate(()=>({width:innerWidth,height:innerHeight,dark:matchMedia('(prefers-color-scheme: dark)').matches,enhancement:!!window.AppKitShellDemo}));
        check(lane.environment.width===width && lane.environment.height===1000 && lane.environment.dark===(theme==='dark') && lane.environment.enhancement===scripts,`${lane.name}: actual viewport, system scheme and enhancement state`,lane.environment);
        lane.original=await state(page,kind);
        const submitted=sample(kind,'submitted');
        await edit(page,kind,submitted);
        await submit(page,lane,303);
        lane.saved=await state(page,kind);
        check(equal(lane.saved.value,submitted) && BigInt(lane.saved.version)===BigInt(lane.original.version)+1n && (!scripts || lane.saved.disabled),`${lane.name}: save advances exactly one version`,lane.saved);
        const back=await travel(page,cdp,lane,'back',kind);
        check(cache==='enabled' || !back.restored,`${lane.name}: disabled cache cannot restore`,back);
        check(back.busy===null,`${lane.name}: Back clears submission busy`,back);
        if(back.restored) {
          check(back.reads===0 && back.version===lane.original.version && equal(back.value,submitted),`${lane.name}: cached draft and original version retained`,back);
          if(scripts) check(/may already have saved|outcome.*unknown/i.test(back.status),`${lane.name}: returned submission outcome is explicitly uncertain`,back.status);
        } else {
          check(back.reads===1 && back.version===lane.saved.version && equal(back.value,submitted) && (!scripts || back.disabled),`${lane.name}: network history reloads current saved state`,back);
        }
        check(lane.requests.filter(r=>r.method==='POST').length===1,`${lane.name}: Back does not repost`);
        dialogDecision='accept';
        const forward=await travel(page,cdp,lane,'forward',kind);
        check(equal(forward.value,submitted) && forward.version===lane.saved.version && (!scripts || forward.disabled) && forward.busy===null,`${lane.name}: Forward shows clean saved snapshot`,forward);
        check(lane.requests.filter(r=>r.method==='POST').length===1,`${lane.name}: Forward does not repost`);

        // Native departure prompt: first dismiss, then accept. Never synthesize the event.
        const unsent=sample(kind,'unsent');
        await edit(page,kind,unsent); dialogDecision='dismiss';
        const before=lane.dialogs.length;
        if(scripts) {
          await page.goto(base+'/shell/help').catch(error=>{if(!String(error).includes('ERR_ABORTED')) throw error;});
          check(lane.dialogs.length===before+1 && lane.dialogs.at(-1).type==='beforeunload' && page.url()===base+routePath(kind),`${lane.name}: dismiss actual dirty departure keeps document`,lane.dialogs);
          check(equal(await value(page,kind),unsent) && await version(page)===lane.saved.version,`${lane.name}: dismissed departure keeps unsent draft/version`);
        }
        dialogDecision='accept'; await page.goto(base+'/shell/help');
        check(page.url()===base+'/shell/help',`${lane.name}: accepted departure reaches help`);
        const other=await context.newPage();
        await other.goto(base+routePath(kind));
        const newer=sample(kind,'newer');
        await edit(other,kind,newer); await submit(other,lane,303);
        lane.otherSaved=await state(other,kind); await other.close();
        const draftBack=await travel(page,cdp,lane,'back',kind);
        check(draftBack.restored===(cache==='enabled'),`${lane.name}: dirty history mode observed`,draftBack);
        // Native fallback may discard an abandoned draft, but never combine that
        // draft with a newer version. BFCache must retain the matching old pair.
        const safePair=draftBack.restored ? equal(draftBack.value,unsent) && draftBack.version===lane.saved.version : equal(draftBack.value,newer) && draftBack.version===lane.otherSaved.version;
        check(safePair && draftBack.reads===(draftBack.restored?0:1),`${lane.name}: history fields and optimistic version belong to the same snapshot`,draftBack);
        check(lane.requests.filter(r=>r.method==='POST').length===2,`${lane.name}: dirty departure/history never saves`);
        if(!safePair) throw new Error('History mixed an old draft with new authority, or lost the expected native snapshot');
        if(draftBack.restored) {
          await submit(page,lane,409);
          const summary=await page.locator('[aria-labelledby=shell-conflict-title]').innerText();
          check(kind==='profile'?summary.includes(newer):summary.includes(newer.timezone) && summary.includes(newer.start_page) && summary.includes(newer.digest?'included':'excluded'),`${lane.name}: explicit retry shows all newer saved values`);
          check(equal(await value(page,kind),unsent) && await version(page)===lane.otherSaved.version,`${lane.name}: conflict keeps draft and supplies reviewed version`);
          check(await save(page).innerText()==='Apply reviewed draft',`${lane.name}: conflict needs explicit reviewed action`);
        } else {
          check(!scripts || await save(page).isDisabled(),`${lane.name}: network reload is clean`);
          await edit(page,kind,sample(kind,'reentered'));
        }
        await page.screenshot({path:`${root}/${lane.name}.png`,fullPage:true});
        await submit(page,lane,303);
        lane.final=await state(page,kind);
        check(equal(lane.final.value,draftBack.restored?unsent:sample(kind,'reentered')) && BigInt(lane.final.version)===BigInt(lane.otherSaved.version)+1n,`${lane.name}: explicit final save advances exactly one version`,lane.final);
        check(equal(lane.postStatuses,draftBack.restored?[303,303,409,303]:[303,303,303]),`${lane.name}: exact POST outcomes`,lane.postStatuses);
        check(lane.requests.filter(r=>r.method==='POST').length===lane.postStatuses.length,`${lane.name}: no extra POST including final save`);
        check(equal(lane.dialogs,scripts?[{type:'beforeunload',decision:'dismiss'},{type:'beforeunload',decision:'accept'}]:[]),`${lane.name}: exact native departure dialog decisions`,lane.dialogs);
        const observer=await context.newPage(); await observer.goto(base+routePath(kind));
        check(equal(await value(observer,kind),lane.final.value) && await version(observer)===lane.final.version,`${lane.name}: independent GET confirms final saved snapshot`);
        await observer.close();
        lane.finished=true;
      } catch(error) {
        check(false,`${lane.name}: execution`,String(error.stack||error));
        await page.screenshot({path:`${root}/${lane.name}-failed.png`}).catch(()=>{});
        lane.geometry=await page.evaluate(()=>{
          const n=document.querySelector('[data-shell-save]'),items=[];
          for(let a=n;a;a=a.parentElement) {
            const r=a.getBoundingClientRect(),s=getComputedStyle(a);
            items.push({tag:a.tagName,id:a.id,rect:{x:r.x,y:r.y,width:r.width,height:r.height},scrollTop:a.scrollTop,scrollHeight:a.scrollHeight,clientHeight:a.clientHeight,overflow:s.overflowY});
          }
          return items;
        }).catch(()=>[]);
      }
      finally {await context.close();}
    }
  } finally {await browser.close();}
}
check(report.blocked.length===0,'no outside requests or WebSockets',report.blocked);
check(report.pageErrors.length===0,'no page errors',report.pageErrors);
check(report.lanes.length===(process.env.APPKIT_HISTORY_LANE?1:32),'requested lane count executed',report.lanes.length);
check(report.lanes.every(l=>l.console.length===l.postStatuses.filter(s=>s===409).length && l.console.every(e=>e.url===base+'/shell/action' && /^Failed to load resource: the server responded with a status of 409 \(/.test(e.text))),'only exact expected conflict console errors',report.lanes.map(l=>({lane:l.name,console:l.console})));
report.finishedAt=new Date().toISOString();
const output=`${root}/shell-history-${variant}-report.json`;
fs.writeFileSync(output,JSON.stringify(report,null,2)+'\n');
console.log(JSON.stringify({passes:report.passes.length,failures:report.failures,report:output},null,2));
process.exitCode=report.failures.length?1:0;
