// Opt-in hardened derivative audit. Original source audit remains unchanged.
// No source mutation, install, baseline acceptance,
// real account, consumer write or conversion certification occurs here.
import { createHash } from 'node:crypto';
import { readFile, writeFile, mkdtemp, realpath, stat } from 'node:fs/promises';
import { dirname, isAbsolute, join } from 'node:path';
import { tmpdir } from 'node:os';
import { pathToFileURL } from 'node:url';

import { verifySource, buildHash } from './reference.mjs';
import { runEditorTasks, editorCheckNames } from './editor-tasks.mjs';
const sourceProof = verifySource(process.env.APPKIT_MATDASH_HARDENED_ROOT);
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const base = process.env.APPKIT_MATDASH_HARDENED_ORIGIN;
const modulePath = process.env.APPKIT_PLAYWRIGHT_MODULE;
if (!base || !modulePath || !isAbsolute(modulePath)) throw new Error('Select explicit APPKIT_MATDASH_HARDENED_ORIGIN and absolute existing APPKIT_PLAYWRIGHT_MODULE; nothing is installed.');
const origin = new URL(base);
if (origin.origin !== base || origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || !origin.port) throw new Error('Select an exact HTTP 127.0.0.1 origin with explicit non-default port, no extra URL data.');
const metadataBytes = await readFile(new URL('./build-assets.json', import.meta.url));
if (hash(metadataBytes) !== buildHash) throw new Error('Pinned asset metadata changed.');
const metadata = JSON.parse(metadataBytes), assets = new Map(metadata.assets.map(a=>['/'+a.path,a]));
const routes = new Set(['/dashboards/dashboard2','/apps/ecommerce/addproduct']);
const root = await mkdtemp(join(tmpdir(),'appkit-matdash-hardened-browser-'));
console.log(`Private MatDash browser evidence: ${root}`);
const report = {source:sourceProof,editorTasks:[],stateCaptures:[],origin:base,started:new Date().toISOString(),metadata_sha256:hash(metadataBytes),
  probe_sha256:hash(await readFile(new URL(import.meta.url))),task_probe_sha256:hash(await readFile(new URL('./editor-tasks.mjs',import.meta.url))),checks:[],views:[],errors:[],console:[],blocked:[],responses:[],limits:[
    'Pinned privately hardened source-native reference, not untouched original or converted target.',
    'Synthetic supplied data; no save backend, real account or app-owned financial semantics.',
    'Observed Chrome/macOS views and tasks, not complete accessibility, genuine browser zoom, IME, clipboard, offline reconciliation or latency certification.',
    'Response hashes cover observed requests only, not every possible route or asset request.',
    'Screenshots do not disable animations; recorded chart geometry and Customizer transition settling are bounded observations, not universal animation completion.',
    'Playwright entry/package hashes identify selected local tooling, not the full transitive dependency closure.']};
const check=(name,pass,detail)=>report.checks.push({name,pass:!!pass,detail});
const playwrightEntry=await realpath(modulePath),playwrightPackage=join(dirname(playwrightEntry),'package.json');
for(const path of [playwrightEntry,playwrightPackage]){const info=await stat(path);if(!info.isFile()||info.size>1024*1024)throw new Error('Selected Playwright identity input is nonregular or exceeds 1 MiB.');}
const playwrightEntryBytes=await readFile(playwrightEntry),playwrightPackageBytes=await readFile(playwrightPackage),playwrightMetadata=JSON.parse(playwrightPackageBytes);
if(playwrightMetadata.name!=='playwright'||typeof playwrightMetadata.version!=='string')throw new Error('Selected module must belong to the existing Playwright package.');
report.playwright={requested_module:modulePath,entrypoint:playwrightEntry,entry_bytes:playwrightEntryBytes.length,entry_sha256:hash(playwrightEntryBytes),package_json:playwrightPackage,package_json_sha256:hash(playwrightPackageBytes),package:playwrightMetadata.name,version:playwrightMetadata.version};
if(report.playwright.version!=='1.63.0'||report.playwright.entry_sha256!=='a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f'||report.playwright.package_json_sha256!=='8bc2ddebe8d9b051b9ecb87d62c6c63877e59d08617f32c3215681ca8bb70998')throw new Error('Approved isolated Playwright identity differs.');
const { chromium } = await import(pathToFileURL(modulePath).href);
let browser, currentPage, phase='launch';
const pending = new Set();
const expectedViews=['p11','p12'].flatMap(id=>['1440-light','1440-dark','390-dark','390-light'].map(view=>`${id}-${view}`));
const drainResponses=async()=>{while(pending.size)await Promise.allSettled([...pending]);};
async function responseBody(response) {
  let timer;
  try{return await Promise.race([response.body(),new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error(`Response body exceeded 10 seconds: ${response.url()}`)),10000);})]);}
  finally{clearTimeout(timer);}
}

async function settleCustomizerClosed(page) {
  const deadline=Date.now()+5000;let previous,stableSince=0,last;
  while(Date.now()<deadline){
    last=await page.evaluate(()=>{
      // Flowbite keeps the closed dialog mounted, with aria-modal still true.
      // Its off-position class, geometry, absent backdrop and finished CSS
      // transitions establish closure; a visibility/role query alone does not.
      const drawers=[...document.querySelectorAll('[data-testid="flowbite-drawer"]')].filter(d=>[...d.querySelectorAll('h5')].some(h=>h.textContent.trim()==='Settings'));
      if(drawers.length!==1)return {closed:false,count:drawers.length};
      const d=drawers[0],r=d.getBoundingClientRect(),animations=d.getAnimations({subtree:true}).map(a=>({state:a.playState,pending:a.pending}));
      const backdrop=!!d.nextElementSibling?.matches('.fixed.inset-0.z-30');
      return {closed:d.classList.contains('translate-x-full')&&!d.classList.contains('transform-none')&&r.width>0&&r.left>=innerWidth&&!backdrop&&animations.every(a=>!a.pending&&a.state!=='running'),
        count:1,viewport:innerWidth,bounds:{left:r.left,right:r.right,width:r.width,height:r.height},transform:getComputedStyle(d).transform,backdrop,animations};
    });
    const fingerprint=JSON.stringify(last);
    if(!last.closed||fingerprint!==previous){stableSince=Date.now();previous=fingerprint;}
    else if(Date.now()-stableSince>=200)return {...last,stableMilliseconds:Date.now()-stableSince};
    await page.waitForTimeout(100);
  }
  throw new Error(`Customizer closure/transition prerequisite not met within 5 seconds: ${JSON.stringify(last)}.`);
}

async function screenshot(page,file,fullPage) {
  const drawer=await settleCustomizerClosed(page);
  await page.screenshot({path:file,fullPage});
  const bytes=await readFile(file);
  if(!bytes.subarray(0,8).equals(Buffer.from([137,80,78,71,13,10,26,10])))throw new Error('Screenshot was not PNG.');
  return {path:file,sha256:hash(bytes),width:bytes.readUInt32BE(16),height:bytes.readUInt32BE(20),drawer};
}

async function settleCharts(page) {
  const deadline=Date.now()+12000;let previous,stableSince=0,lastCounts;
  while(Date.now()<deadline){
    const state=await page.evaluate(()=>{
      const charts=[...document.querySelectorAll('.apexcharts-canvas svg.apexcharts-svg')];
      const rect=n=>{const r=n.getBoundingClientRect();return [r.x,r.y,r.width,r.height];};
      const effects=charts.flatMap(c=>[c,...c.querySelectorAll('*')]).map(n=>n.instance?.fx).filter(Boolean);
      // Subscriptions also uses rounded-bars (14 bars). Locate the forecast by
      // its authored heading, independently of the expected 18-bar assertion.
      const forecastHeadings=[...document.querySelectorAll('h5.card-title')].filter(h=>h.textContent.trim()==='Revenue Forecast');
      const forecast=forecastHeadings.length===1?forecastHeadings[0].closest('.card'):null;
      const forecastCharts=forecast?.querySelectorAll('.apexcharts-canvas svg.apexcharts-svg').length??0;
      const bars=[...forecast?.querySelectorAll('.apexcharts-series .apexcharts-bar-area')??[]];
      return {ready:charts.length===7 && charts.every(c=>c.instance&&c.querySelectorAll('.apexcharts-series path').length>0&&[...c.querySelectorAll('.apexcharts-series path')].every(p=>p.getAttribute('d')?.trim()&&!/NaN|Infinity/.test(p.getAttribute('d')))&&rect(c).every(Number.isFinite)&&rect(c)[2]>0&&rect(c)[3]>0)
        &&effects.every(f=>!f.active&&f.situation===null&&f.situations.length===0)
        &&forecastCharts===1&&bars.length===18&&bars.every(b=>rect(b).every(Number.isFinite)&&rect(b)[2]>0&&rect(b)[3]>0),
        counts:{charts:charts.length,observableRoots:charts.filter(c=>c.instance).length,effects:effects.length,forecastHeadings:forecastHeadings.length,forecastCharts,bars:bars.length,nonzeroBars:bars.filter(b=>rect(b).every(Number.isFinite)&&rect(b)[2]>0&&rect(b)[3]>0).length},
        geometry:charts.map(c=>[rect(c),...Array.from(c.querySelectorAll('.apexcharts-series path'),p=>[p.getAttribute('d'),p.getAttribute('transform'),rect(p)])])};
    });
    lastCounts=state.counts;
    // Synchronous theme rerenders need not allocate FX. Observable SVG.js roots,
    // nonempty geometry and 18 nonzero forecast bars prevent a vacuous readiness pass.
    const fingerprint=JSON.stringify(state.geometry);
    if(!state.ready||fingerprint!==previous){stableSince=Date.now();previous=fingerprint;}
    else if(Date.now()-stableSince>=600){check(`${phase}: seven chart geometries stable for 600ms with 18 forecast bars`,true,{...state.counts,stableMilliseconds:Date.now()-stableSince});return;}
    await page.waitForTimeout(100);
  }
  throw new Error(`Seven-chart/18-bar stable-geometry prerequisite not met within 12 seconds: ${JSON.stringify(lastCounts)}.`);
}

async function editorTask(page) {
  if(!phase.startsWith('p12'))return;
  const taskPhase=phase;
  const facts=await runEditorTasks(page,base,taskPhase,check,async state=>{
    await page.locator('.editor-container').scrollIntoViewIfNeeded();
    const result=await screenshot(page,join(root,`${taskPhase}-${state}.png`),false);
    report.stateCaptures.push({name:`${taskPhase}-${state}`,...result});
  });
  report.editorTasks.push({name:taskPhase,facts});
}

async function capture(page) {
  await page.evaluate(()=>document.fonts.ready);
  const drawer=await settleCustomizerClosed(page);
  check(`${phase}: Customizer closed and transition settled`,true,drawer);
  if(phase.startsWith('p11'))await settleCharts(page);
  const view=await page.evaluate(()=>{
    const shown=e=>{const r=e.getBoundingClientRect(),s=getComputedStyle(e);return r.width>0&&r.height>0&&r.right>0&&r.left<innerWidth&&s.visibility!=='hidden'&&s.display!=='none';};
    const bounds=e=>{const r=e.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom};};
    return {width:innerWidth,scrollWidth:document.documentElement.scrollWidth,theme:document.documentElement.className,
      font:getComputedStyle(document.body).fontFamily,loadedFonts:[...document.fonts].filter(f=>f.status==='loaded').map(f=>({family:f.family,weight:f.weight})),
      brokenImages:[...document.images].filter(i=>shown(i)&&(!i.complete||i.naturalWidth===0)).map(i=>i.getAttribute('src')),
      headings:[...document.querySelectorAll('h1,h2,h3,h4,h5,h6')].filter(shown).map(e=>e.textContent.trim()),
      unnamedButtons:[...document.querySelectorAll('button')].filter(shown).filter(e=>!e.textContent.trim()&&!e.getAttribute('aria-label')&&!e.getAttribute('aria-labelledby')&&!e.title).map(e=>({html:e.outerHTML.slice(0,200),x:e.getBoundingClientRect().x,y:e.getBoundingClientRect().y})),
      chartCount:document.querySelectorAll('.apexcharts-canvas').length,
      toolbars:[...document.querySelectorAll('.toolbar')].map(t=>({bounds:bounds(t),scrollWidth:t.scrollWidth,clientWidth:t.clientWidth,display:getComputedStyle(t).display,flexWrap:getComputedStyle(t).flexWrap,gap:getComputedStyle(t).gap,
        buttons:[...t.querySelectorAll('button')].map((b,index)=>({index,name:b.getAttribute('aria-label')||b.textContent.trim(),bounds:bounds(b)}))})),
      resourceBytes:performance.getEntriesByType('resource').reduce((t,r)=>({transfer:t.transfer+r.transferSize,encoded:t.encoded+r.encodedBodySize,decoded:t.decoded+r.decodedBodySize}),{transfer:0,encoded:0,decoded:0})};
  });
  check(`${phase}: no page-width overflow`,view.scrollWidth<=view.width,view);
  check(`${phase}: declared Manrope font loaded`,view.loadedFonts.some(f=>f.family==='Manrope'));
  check(`${phase}: no broken visible image`,view.brokenImages.length===0,view.brokenImages);
  const full=await screenshot(page,join(root,`${phase}.png`),true);
  const viewport=view.width===390?await screenshot(page,join(root,`${phase}-viewport.png`),false):null;
  if(viewport)check(`${phase}: viewport-only capture is 390 by 1000`,viewport.width===390&&viewport.height===1000,{width:viewport.width,height:viewport.height});
  report.views.push({name:phase,...view,drawer,screenshot:full.path,screenshot_sha256:full.sha256,screenshot_width:full.width,screenshot_height:full.height,
    ...(viewport?{viewport_screenshot:viewport.path,viewport_screenshot_sha256:viewport.sha256,viewport_screenshot_width:viewport.width,viewport_screenshot_height:viewport.height,viewport_drawer:viewport.drawer}:{})});
}

try {
  browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true,timeout:15000,args:['--disable-background-networking','--no-proxy-server']});
  report.browser=browser.version();
  for(const [id,route] of [['p11','/dashboards/dashboard2'],['p12','/apps/ecommerce/addproduct']]){
    const context=await browser.newContext({viewport:{width:1440,height:1000},deviceScaleFactor:1,reducedMotion:'reduce',colorScheme:'light',serviceWorkers:'block',acceptDownloads:false});
    context.setDefaultTimeout(5000);context.setDefaultNavigationTimeout(15000);
    await context.route('**/*',r=>{
      const req=r.request(),u=new URL(req.url());
      if(u.origin!==base || !['GET','HEAD'].includes(req.method()) || (!assets.has(u.pathname)&&!routes.has(u.pathname))){report.blocked.push({phase,url:req.url(),method:req.method()});return r.abort('blockedbyclient');}
      return r.continue();
    });
    await context.routeWebSocket('**/*',ws=>{report.blocked.push({phase,url:ws.url(),method:'WS'});ws.close();});
    const page=await context.newPage();currentPage=page;
    page.on('pageerror',e=>report.errors.push({phase,error:String(e)}));
    page.on('console',m=>{if(['warning','error'].includes(m.type())){const text=m.text();report.console.push({phase,type:m.type(),text,
      classification:m.type()==='warning'&&text==="[tiptap warn]: Duplicate extension names found: ['codeBlock']. This can lead to issues."?'known-unresolved-duplicate-codeBlock':'unclassified'});}});
    page.on('response',response=>{
      const at=phase;
      const task=(async()=>{const u=new URL(response.url()),pin=assets.get(u.pathname)||(routes.has(u.pathname)?assets.get('/index.html'):null);
        const bytes=await responseBody(response);report.responses.push({phase:at,url:response.url(),status:response.status(),bytes:bytes.length,sha256:hash(bytes),matchesPin:!!pin&&hash(bytes)===pin.sha256});})();
      pending.add(task);task.catch(e=>report.errors.push({phase:at,error:String(e)})).finally(()=>pending.delete(task));
    });
    phase=`${id}-1440-light`;
    await page.goto(base+route,{waitUntil:'networkidle'});await capture(page);await editorTask(page);
    for(const [width,theme] of [[1440,'dark'],[390,'dark'],[390,'light']]){
      phase=`${id}-${width}-${theme}`;await page.setViewportSize({width,height:1000});
      await page.locator('button').filter({has:page.locator('svg.tabler-icon-settings')}).click();
      await page.getByRole('button',{name:theme==='dark'?'Dark':'Light',exact:true}).click();
      await page.keyboard.press('Escape');
      await page.waitForFunction(t=>document.documentElement.classList.contains(t),theme);
      await capture(page);await editorTask(page);
    }
    await drainResponses();await context.close();await drainResponses();
  }
} catch(error){report.failure=String(error);report.failurePhase=phase;process.exitCode=1;
  if(currentPage&&!currentPage.isClosed())report.diagnostic=await currentPage.evaluate(()=>({theme:document.documentElement.className,charts:[...document.querySelectorAll('.apexcharts-canvas svg.apexcharts-svg')].map(c=>({id:c.id,effects:[c,...c.querySelectorAll('*')].filter(n=>n.instance?.fx).map(n=>({tag:n.tagName,active:n.instance.fx.active,situation:n.instance.fx.situation!==null,queued:n.instance.fx.situations.length}))}))})).catch(e=>({error:String(e)}));
}
finally{
  // Body promises can enqueue completion work during teardown. Drain the live
  // set, and record close failures instead of letting them suppress the receipt.
  await drainResponses();
  try{if(browser)await browser.close();}catch(error){report.errors.push({phase:'shutdown',error:String(error)});}
  await drainResponses();
  check('all observed asset and route bytes match pinned build',report.responses.length>0&&report.responses.every(r=>r.status===200&&r.matchesPin),report.responses.length);
  check('no outside or write request observed',report.blocked.length===0,report.blocked);
  check('no page execution error',report.errors.length===0,report.errors);
  check('no runtime console warnings or errors, including known unresolved warnings',report.console.length===0,report.console);
  check('all eight planned source-reference views captured',JSON.stringify(report.views.map(view=>view.name))===JSON.stringify(expectedViews),report.views.map(view=>view.name));
  check('initial P11 view contains all seven chart roots',report.views.find(view=>view.name==='p11-1440-light')?.chartCount===7);
  check('all four editor task sequences and eight state captures completed',report.editorTasks.length===4&&report.stateCaptures.length===8);
  const expectedChecks=expectedViews.flatMap(view=>[
    `${view}: Customizer closed and transition settled`,`${view}: no page-width overflow`,`${view}: declared Manrope font loaded`,`${view}: no broken visible image`,
    ...(view.startsWith('p11')?[`${view}: seven chart geometries stable for 600ms with 18 forecast bars`]:[]),
    ...(view.includes('-390-')?[`${view}: viewport-only capture is 390 by 1000`]:[]),
  ]).concat([
    ...expectedViews.filter(view=>view.startsWith('p12')).flatMap(view=>editorCheckNames.map(name=>`${view}: ${name}`)),
    'all observed asset and route bytes match pinned build','no outside or write request observed','no page execution error','no runtime console warnings or errors, including known unresolved warnings',
    'all eight planned source-reference views captured','initial P11 view contains all seven chart roots','all four editor task sequences and eight state captures completed',
  ]);
  const actualChecks=report.checks.map(c=>c.name);
  check('exact expected behavior check-name set completed',JSON.stringify([...actualChecks].sort())===JSON.stringify([...expectedChecks].sort()),{
    expected:expectedChecks.length,actual:actualChecks.length,missing:expectedChecks.filter(name=>!actualChecks.includes(name)),unexpected:actualChecks.filter(name=>!expectedChecks.includes(name)),duplicates:actualChecks.filter((name,index)=>actualChecks.indexOf(name)!==index)});
  report.ok=!report.failure&&report.checks.every(c=>c.pass);if(!report.ok)process.exitCode=1;
  await writeFile(join(root,'receipt.json'),JSON.stringify(report,null,2)+'\n');
  console.log(JSON.stringify({root,ok:report.ok,checks:report.checks.length,failed:report.checks.filter(c=>!c.pass),failure:report.failure,errors:report.errors,console:report.console}));
}
