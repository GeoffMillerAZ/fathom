// Browser task audit of the editor, not accessibility certification of specimens.
import { readFileSync, writeFileSync, mkdtempSync, chmodSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { configuration } from '../../gallery/browser/config.mjs';
import { prepareContext, zoomCaptureClip } from '../../../docs/assessments/fixtures/gallery-analytics/zoom-contract.mjs';

if(process.argv.length>3||(process.argv[2]&&process.argv[2]!=='--zoom=200'))throw new Error('Only the optional --zoom=200 lane is supported.');
const zoom=process.argv[2]==='--zoom=200';
if (!process.env.APPKIT_THEME_BROWSER_ORIGIN) throw new Error('Select a local theme-lab origin.');
const config = configuration({...process.env, APPKIT_SHELL_BROWSER_ORIGIN:process.env.APPKIT_THEME_BROWSER_ORIGIN});
const hash = b => createHash('sha256').update(b).digest('hex');
if (hash(readFileSync(new URL(config.moduleURL))) !== 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f' || hash(readFileSync(new URL('./package.json', config.moduleURL))) !== '8bc2ddebe8d9b051b9ecb87d62c6c63877e59d08617f32c3215681ca8bb70998') throw new Error('Approved harness identity differs.');
const { chromium } = await import(config.moduleURL);
const root = mkdtempSync(join(tmpdir(), 'appkit-theme-usability-'));
console.log('Theme usability evidence: '+root);
const report = { origin:config.base, variant:config.variant, probe_sha256:hash(readFileSync(new URL(import.meta.url))), zoom_contract_sha256:hash(readFileSync(new URL('../../../docs/assessments/fixtures/gallery-analytics/zoom-contract.mjs',import.meta.url))), checks:[], errors:[], console:[], blocked:[], workers:[], responses:[], screenshots:[], profiles:[], limits:[
  zoom?'The unchanged default lane plus an explicit Chrome Appearance 200% lane; 720 CSS pixels alone is never accepted as zoom.':'1440, 390, 320 and 720 CSS-pixel editor viewports; 720 is reflow geometry, not actual browser zoom.',
  'Real Tab/Enter section navigation and error recovery; not exhaustive keyboard traversal or a screen-reader audit.',
  'Page evaluation only reads geometry/focus; no DOM mutation, CSP bypass, persistence or consumer write.',
]};
const check = (name,pass,detail) => report.checks.push({name,pass:!!pass,detail});
const pending = new Set();
let browser, context, page, zoomContext;
async function shot(name) { const path=join(root,name+'.png');await page.screenshot({path,fullPage:false});report.screenshots.push({name,path,sha256:hash(readFileSync(path))}); }
async function drain() { while(pending.size) await Promise.allSettled([...pending]); }
async function bodyWithin(response) { let timer;try{return await Promise.race([response.body(),new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error('Body deadline')),10000);})]);}finally{clearTimeout(timer);} }
async function tabTo(locator,limit=12) { for(let i=0;i<limit;i++){await page.keyboard.press('Tab');if(await locator.evaluate(el=>el===document.activeElement))return i+1;}return false; }
async function nativeTabTo(locator,limit=12) { for(let i=0;i<limit;i++){await page.keyboard.press('Tab');if(await locator.and(page.locator(':focus')).count()===1)return i+1;}return false; }
try {
  browser=await chromium.launch({executablePath:config.executablePath,headless:true,args:['--disable-background-networking','--no-proxy-server']});
  report.browser=browser.version();
  context=await browser.newContext({viewport:{width:1440,height:1000},reducedMotion:'reduce',acceptDownloads:false});
  context.setDefaultTimeout(5000);context.setDefaultNavigationTimeout(15000);
  context.on('serviceworker',w=>report.workers.push(w.url()));
  await context.route('**/*',route=>{const r=route.request(),u=new URL(r.url());const allowed=u.origin===config.base&&((r.method()==='GET'&&['/theme-lab','/theme-lab/style.css','/theme-lab/preview','/static/app.css'].includes(u.pathname))||(r.method()==='POST'&&u.pathname==='/theme-lab'));if(!allowed){report.blocked.push({method:r.method(),url:r.url()});return route.abort('blockedbyclient');}return route.continue();});
  await context.routeWebSocket('**/*',ws=>{report.blocked.push({method:'WS',url:ws.url()});ws.close();});
  page=await context.newPage();
  page.on('pageerror',e=>report.errors.push(String(e)));
  page.on('console',m=>{if(['warning','error'].includes(m.type()))report.console.push({type:m.type(),text:m.text(),url:m.location().url});});
  page.on('response',r=>{const task=(async()=>{const bytes=await bodyWithin(r);report.responses.push({url:r.url(),status:r.status(),headers:r.headers(),bytes:bytes.length,sha256:hash(bytes)});})();pending.add(task);task.catch(e=>report.errors.push(String(e))).finally(()=>pending.delete(task));});
  for(const width of [1440,390,320,720]) {
    await page.setViewportSize({width,height:1000});await page.goto(config.base+'/theme-lab',{waitUntil:'networkidle'});
    const layout=await page.evaluate(()=>({width:innerWidth,scroll:document.documentElement.scrollWidth,reviewTop:document.querySelector('.sg-controls').getBoundingClientRect().top,targets:[...document.querySelectorAll('.sg button,.sg input:not([type="hidden"]),.sg select,.sg-editor summary')].filter(el=>el.checkVisibility()).map(el=>({name:el.getAttribute('name')||el.textContent.trim(),height:el.getBoundingClientRect().height,width:el.getBoundingClientRect().width})),clipped:[...document.querySelectorAll('.sg a,.sg button,.sg input:not([type="hidden"]),.sg select,.sg-editor summary')].filter(el=>el.checkVisibility()).map(el=>({name:el.getAttribute('name')||el.textContent.trim(),left:el.getBoundingClientRect().left,right:el.getBoundingClientRect().right})).filter(r=>r.left<0||r.right>innerWidth+1)}));
    check(width+' editor has no horizontal overflow',layout.scroll<=width+1&&layout.clipped.length===0,layout);
    check(width+' review controls discoverable without long scroll',layout.reviewTop<900,layout.reviewTop);
    check(width+' primary edit targets at least 44px',layout.targets.every(t=>t.height>=44&&t.width>=44),layout.targets.filter(t=>t.height<44||t.width<44));
    await shot('editor-'+width);
  }
  await page.setViewportSize({width:390,height:1000});await page.goto(config.base+'/theme-lab',{waitUntil:'networkidle'});
  const review=page.getByRole('link',{name:'Review preview',exact:true});
  check('review section has native shortcut',await review.count()===1);
  if(await review.count()) {
    check('review shortcut reachable by Tab',await tabTo(review));await page.keyboard.press('Enter');
    check('native shortcut focuses review region',await page.locator('#sg-review').evaluate(el=>document.activeElement===el&&el.getBoundingClientRect().top>=-1&&el.getBoundingClientRect().top<innerHeight));
    await page.keyboard.press('Tab');
    check('next Tab reaches specimen selection',await page.locator('[name="specimen"]').evaluate(el=>el===document.activeElement));
  }
  const radius=page.locator('details').filter({has:page.locator('summary',{hasText:'Corner radii'})});
  report.radius_before=await radius.evaluate(el=>({open:el.open,markup:el.outerHTML.slice(0,300)}));
  // Split scrolling from the normal click: the combined Playwright action can
  // put pointer-down in the iframe before its auto-scroll settles after #sg-review.
  if(!await radius.evaluate(el=>el.open)) {
    await radius.locator('summary').scrollIntoViewIfNeeded();
    await radius.locator('summary').click();
  }
  report.radius_after=await radius.evaluate(el=>({open:el.open,markup:el.outerHTML.slice(0,300)}));
  check('corner radii opens for repair',await radius.evaluate(el=>el.open));
  await page.getByLabel('radius.md',{exact:true}).fill('-1px');
  const [invalid]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'),page.getByRole('button',{name:'Preview candidate',exact:true}).click()]);await page.waitForLoadState('networkidle');
  check('invalid radius returns repairable 422',invalid.status()===422,invalid.status());
  check('error summary receives native focus',await page.getByRole('alert').evaluate(el=>el===document.activeElement&&el.getAttribute('aria-labelledby')==='sg-error-title'));
  check('invalid radius is retained and visible',await page.getByLabel('radius.md',{exact:true}).inputValue()==='-1px'&&await page.getByLabel('radius.md',{exact:true}).isVisible());
  check('failed candidate has no acknowledgement',await page.locator('[name="review_digest"]').inputValue()===''&&await page.getByRole('button',{name:'Export acknowledged candidate ZIP',exact:true}).isDisabled());
  await shot('invalid-radius-390');
  const repair=page.getByRole('link',{name:'Fix radius.md',exact:true});
  check('error provides native repair link',await repair.count()===1);
  if(await repair.count()){await page.keyboard.press('Tab');check('error Tab reaches repair link',await repair.evaluate(el=>el===document.activeElement));await page.keyboard.press('Enter');check('repair shortcut focuses invalid field',await page.getByLabel('radius.md',{exact:true}).evaluate(el=>el===document.activeElement&&el.getAttribute('aria-invalid')==='true'&&document.getElementById(el.getAttribute('aria-describedby'))?.textContent.length>0));}

  if(zoom){
  const profile=mkdtempSync(join(root,'chrome-profile-'));chmodSync(profile,0o700);
  report.profiles.push({path:profile.slice(root.length+1),mode:'0700',disposition:'retained synthetic-only profile'});
  const launchedZoomContext=await chromium.launchPersistentContext(profile,{executablePath:config.executablePath,headless:true,args:['--disable-background-networking','--no-proxy-server','--no-first-run','--no-default-browser-check'],viewport:{width:1440,height:1000},reducedMotion:'reduce',acceptDownloads:false});
  zoomContext=await prepareContext(launchedZoomContext,async()=>{
    const settings=launchedZoomContext.pages()[0];settings.setDefaultTimeout(6000);
    await settings.goto('chrome://settings/appearance');
    await settings.locator('#zoomLevel').selectOption('2');
    check('zoom Chrome Appearance setting selects 200%',await settings.locator('#zoomLevel').inputValue()==='2');
    await settings.goto('about:blank');
    launchedZoomContext.setDefaultTimeout(5000);launchedZoomContext.setDefaultNavigationTimeout(15000);
    launchedZoomContext.on('serviceworker',w=>report.workers.push(w.url()));
    await launchedZoomContext.route('**/*',route=>{const r=route.request(),u=new URL(r.url());const allowed=u.origin===config.base&&((r.method()==='GET'&&['/theme-lab','/theme-lab/style.css','/theme-lab/preview','/static/app.css'].includes(u.pathname))||(r.method()==='POST'&&u.pathname==='/theme-lab'));if(!allowed){report.blocked.push({method:r.method(),url:r.url()});return route.abort('blockedbyclient');}return route.continue();});
    await launchedZoomContext.routeWebSocket('**/*',ws=>{report.blocked.push({method:'WS',url:ws.url()});ws.close();});
    page=settings;
    page.on('pageerror',e=>report.errors.push(String(e)));
    page.on('console',m=>{if(['warning','error'].includes(m.type()))report.console.push({type:m.type(),text:m.text(),url:m.location().url});});
    page.on('response',r=>{const task=(async()=>{const bytes=await bodyWithin(r);report.responses.push({url:r.url(),status:r.status(),headers:r.headers(),bytes:bytes.length,sha256:hash(bytes)});})();pending.add(task);task.catch(e=>report.errors.push(String(e))).finally(()=>pending.delete(task));});
    return launchedZoomContext;
  });
  await page.goto(config.base+'/theme-lab',{waitUntil:'networkidle'});
  const cdp=await zoomContext.newCDPSession(page);
  try {
    const metrics=await cdp.send('Page.getLayoutMetrics'),clip=zoomCaptureClip(metrics);
    report.zoom_geometry={contentSize:metrics.contentSize,cssContentSize:metrics.cssContentSize,clip};
    check('zoom actual 200% geometry',metrics.contentSize.width===1440&&metrics.cssContentSize.width===720&&clip.width===1440,report.zoom_geometry);
  } finally { await cdp.detach(); }
  const exposedZoom=await page.evaluate(()=>({dpr:devicePixelRatio,inner:innerWidth,outer:outerWidth,viewport:document.documentElement.clientWidth,visualScale:visualViewport.scale}));
  report.zoom_geometry.exposed=exposedZoom;
  check('zoom browser reports actual 200%',exposedZoom.dpr===2&&exposedZoom.inner===720&&exposedZoom.outer===1440&&exposedZoom.viewport===720&&exposedZoom.visualScale===1,exposedZoom);
  report.zoom_geometry.stages=[];
  const inspectZoomStage=async name=>{
    const facts=await page.evaluate(()=>{const all=[...document.querySelectorAll('.sg a,.sg button,.sg input:not([type="hidden"]),.sg select,.sg-editor summary')].filter(el=>el.checkVisibility()),primary=new Set(document.querySelectorAll('.sg button,.sg input:not([type="hidden"]),.sg select,.sg-editor summary'));return {viewport:innerWidth,scroll:document.documentElement.scrollWidth,count:all.length,controls:all.slice(0,129).map(el=>{const r=el.getBoundingClientRect();return {name:(el.getAttribute('name')||el.textContent.trim()).slice(0,80),left:r.left,right:r.right,width:r.width,height:r.height,primary:primary.has(el)};})};});
    report.zoom_geometry.stages.push({name,...facts});
    check(`zoom ${name} layout has no overflow`,facts.count>0&&facts.count<=128&&facts.controls.length===facts.count&&facts.scroll<=facts.viewport+1,facts);
    check(`zoom ${name} controls fit and primary targets reach 44px`,facts.controls.every(c=>c.left>=-1&&c.right<=facts.viewport+1&&(!c.primary||(c.width>=44&&c.height>=44))),facts.controls.filter(c=>c.left< -1||c.right>facts.viewport+1||(c.primary&&(c.width<44||c.height<44))));
  };
  const zoomRadius=page.locator('details').filter({has:page.locator('summary',{hasText:'Corner radii'})});
  const zoomSummary=zoomRadius.locator('summary');
  check('zoom keyboard reaches radius section',await nativeTabTo(zoomSummary,40));await page.keyboard.press('Enter');
  const zoomInput=page.getByLabel('radius.md',{exact:true});
  check('zoom keyboard reaches radius.md',await nativeTabTo(zoomInput,12));await page.keyboard.type('-1px');
  const zoomPreview=page.getByRole('button',{name:'Preview candidate',exact:true});
  check('zoom keyboard reaches preview',await nativeTabTo(zoomPreview,20));
  const [zoomInvalid]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/theme-lab'),page.waitForNavigation({waitUntil:'networkidle'}),page.keyboard.press('Enter')]);
  check('zoom invalid radius returns 422',zoomInvalid.status()===422,zoomInvalid.status());
  const zoomRepair=page.getByRole('link',{name:'Fix radius.md',exact:true});
  check('zoom error focuses summary',await page.locator(':focus').getAttribute('role')==='alert');
  await inspectZoomStage('invalid');await shot('zoom-invalid-radius-200');
  check('zoom keyboard reaches repair link',await nativeTabTo(zoomRepair,1));await page.keyboard.press('Enter');
  check('zoom repair link focuses invalid field',await page.locator(':focus').getAttribute('name')==='radius.md'&&await zoomInput.getAttribute('aria-invalid')==='true');
  await page.keyboard.press('Meta+A');await page.keyboard.type('1.5rem');
  check('zoom keyboard reaches corrected preview',await nativeTabTo(zoomPreview,20));
  const [zoomCorrected]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/theme-lab'),page.waitForNavigation({waitUntil:'networkidle'}),page.keyboard.press('Enter')]);
  check('zoom corrected candidate returns 200',zoomCorrected.status()===200,zoomCorrected.status());
  check('zoom corrected candidate acknowledged',await page.getByRole('heading',{name:'Candidate',exact:true}).count()===1&&(await page.locator('[name="review_digest"]').inputValue()).length===64&&await zoomInput.inputValue()==='1.5rem');
  await inspectZoomStage('candidate');await shot('zoom-candidate-200');
  const showSource=page.getByRole('button',{name:'Show source',exact:true});
  check('zoom keyboard reaches Show source',await nativeTabTo(showSource,40));
  const [zoomSource]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/theme-lab'),page.waitForNavigation({waitUntil:'networkidle'}),page.keyboard.press('Enter')]);
  check('zoom source view returns 200',zoomSource.status()===200,zoomSource.status());
  const sourceState={heading:await page.getByRole('heading',{name:'Source baseline',exact:true}).count(),digest:await page.locator('[name="review_digest"]').inputValue(),radius:await zoomInput.inputValue()};
  check('zoom source view preserves corrected candidate',sourceState.heading===1&&sourceState.digest.length===64&&sourceState.radius==='1.5rem',sourceState);
  await inspectZoomStage('source');
  const reset=page.getByRole('link',{name:'Reset to source',exact:true});
  check('zoom keyboard reaches source reset',await nativeTabTo(reset,4));await Promise.all([page.waitForNavigation({waitUntil:'networkidle'}),page.keyboard.press('Enter')]);
  check('zoom reset clears candidate state',await page.getByRole('heading',{name:'Source baseline',exact:true}).count()===1&&await page.locator('[name="review_digest"]').inputValue()===''&&await page.getByLabel('radius.md',{exact:true}).inputValue()==='');
  await inspectZoomStage('reset');
  await shot('zoom-reset-200');
  }
} catch(error) { report.failure=String(error);if(page)await shot('failure'); }
finally {
  await drain();
  // Attempt both owned closures even if one fails, then account for late events
  // before evaluating the error gate or writing a successful receipt.
  const closures=await Promise.allSettled([zoomContext?.close(),browser?.close()]);
  for(const result of closures)if(result.status==='rejected')report.errors.push('Browser cleanup: '+String(result.reason));
  await drain();
  check('document CSP blocks scripts and workers',report.responses.filter(r=>r.headers['content-type']?.startsWith('text/html')).every(r=>r.headers['content-security-policy']?.includes("script-src 'none'")&&r.headers['content-security-policy']?.includes("default-src 'none'")));
  check('no workers',report.workers.length===0&&context?.serviceWorkers().length===0&&(!zoomContext||zoomContext.serviceWorkers().length===0),report.workers);
  check('no out-of-scope requests',report.blocked.length===0,report.blocked);
  check('no page errors',report.errors.length===0,report.errors);
  const expectedConsole=report.console.filter(m=>m.type==='error'&&m.text==='Failed to load resource: the server responded with a status of 422 (Unprocessable Entity)'&&report.responses.some(r=>r.status===422&&r.url===m.url));
  check(zoom?'only the expected validation network console errors':'only the expected validation network console error',report.console.length===expectedConsole.length&&expectedConsole.length===(zoom?2:1),report.console);
  const expected=[...[1440,390,320,720].flatMap(w=>[w+' editor has no horizontal overflow',w+' review controls discoverable without long scroll',w+' primary edit targets at least 44px']),'review section has native shortcut','review shortcut reachable by Tab','native shortcut focuses review region','next Tab reaches specimen selection','corner radii opens for repair','invalid radius returns repairable 422','error summary receives native focus','invalid radius is retained and visible','failed candidate has no acknowledgement','error provides native repair link','error Tab reaches repair link','repair shortcut focuses invalid field',...(zoom?['zoom Chrome Appearance setting selects 200%','zoom actual 200% geometry','zoom browser reports actual 200%','zoom keyboard reaches radius section','zoom keyboard reaches radius.md','zoom keyboard reaches preview','zoom invalid radius returns 422',...['invalid','candidate','source','reset'].flatMap(s=>[`zoom ${s} layout has no overflow`,`zoom ${s} controls fit and primary targets reach 44px`]),'zoom error focuses summary','zoom keyboard reaches repair link','zoom repair link focuses invalid field','zoom keyboard reaches corrected preview','zoom corrected candidate returns 200','zoom corrected candidate acknowledged','zoom keyboard reaches Show source','zoom source view returns 200','zoom source view preserves corrected candidate','zoom keyboard reaches source reset','zoom reset clears candidate state']:[]),'document CSP blocks scripts and workers','no workers','no out-of-scope requests','no page errors',zoom?'only the expected validation network console errors':'only the expected validation network console error'];
  check('exact assertion set completed',JSON.stringify(report.checks.map(c=>c.name).sort())===JSON.stringify(expected.sort()));
  report.ok=!report.failure&&report.checks.every(c=>c.pass);writeFileSync(join(root,'receipt.json'),JSON.stringify(report,null,2)+'\n',{flag:'wx'});
  console.log(JSON.stringify({root,ok:report.ok,checks:report.checks.length,failed:report.checks.filter(c=>!c.pass).map(c=>({name:c.name,detail:c.detail})),failure:report.failure,radius_before:report.radius_before,radius_after:report.radius_after}));if(!report.ok)process.exitCode=1;
}
