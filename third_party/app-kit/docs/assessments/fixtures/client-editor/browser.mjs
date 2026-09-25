// Synthetic P12 ownership/tasks in fresh Chrome contexts. No signed-in profile or external network.
import fs from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { createHash } from 'node:crypto';

const [base, buildDir, zoomArg] = process.argv.slice(2);
const origin = new URL(base);
if (origin.origin !== base || origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || !origin.port || ![4,5].includes(process.argv.length) || (zoomArg !== undefined && zoomArg !== '--zoom=200')) throw new Error('Usage: browser.mjs EXACT_LOOPBACK_ORIGIN ABSOLUTE_BUILD_DIR [--zoom=200]');
const actualZoom = zoomArg === '--zoom=200';
const build = await fs.realpath(buildDir);
const sha = body => createHash('sha256').update(body).digest('hex');
const manifestBody = await fs.readFile(path.join(build, 'dist-a/manifest.json'));
const manifest = JSON.parse(manifestBody);
const assets = new Map(manifest.files.map(f => [`/${f.path}`, f]));
const withMorph = assets.has('/assets/morph-client.js');
const root = await fs.mkdtemp(path.join(build, 'browser-'));
const here = path.dirname(fileURLToPath(import.meta.url));
const bindingPath = process.env.APPKIT_P12_TRIAL && path.join(process.env.APPKIT_P12_TRIAL, 'startup.json');
if (!bindingPath) throw new Error('Use run-browser.mjs to bind the exact owned host');
async function readBounded(file){
  const info=await fs.lstat(file);
  if(!info.isFile()||info.isSymbolicLink()||info.size<1||info.size>2*1024*1024)throw new Error('Invalid bounded evidence input: '+file);
  const body=await fs.readFile(file);if(body.length>2*1024*1024)throw new Error('Evidence input grew beyond limit');return body;
}
const envelopeBody=await readBounded(path.join(process.env.APPKIT_P12_TRIAL,'harness-inputs.json'));
const envelope=JSON.parse(envelopeBody),inputBindings=[];
if(envelope.schema!==1||!Array.isArray(envelope.files))throw new Error('Missing input identity contract');
async function retain(body,name,expected){
  if(!expected||sha(body)!==expected.sha256||(expected.bytes!==undefined&&body.length!==expected.bytes))throw new Error('Retained input identity mismatch: '+name);
  await fs.writeFile(path.join(root,name),body,{flag:'wx',mode:0o600});inputBindings.push({path:name,sha256:sha(body),bytes:body.length});
}
for(const [from,name] of [[fileURLToPath(import.meta.url),'browser.mjs'],[process.env.APPKIT_P12_EDITOR_TASKS??path.resolve(here,'../matdash-hardened/editor-tasks.mjs'),'editor-tasks.mjs'],[process.env.APPKIT_P12_SCALE_TASKS??path.join(here,'scale-tasks.mjs'),'scale-tasks.mjs'],[process.env.APPKIT_P12_MORPH_TASKS??path.join(here,'morph-tasks.mjs'),'morph-tasks.mjs']]){
  await retain(await readBounded(from),name,envelope.files.find(f=>f.path===name));
}
for(const [variable,name] of [['APPKIT_P12_ZOOM_CONTRACT','zoom-contract.mjs'],['APPKIT_P12_ZOOM_CAPTURE','zoom-capture.mjs']]){
  if(!process.env[variable])throw new Error('Missing retained zoom helper: '+name);
  await retain(await readBounded(process.env[variable]),name,envelope.files.find(f=>f.path===name));
}
await retain(manifestBody,'manifest.json',envelope.files.find(f=>f.path==='manifest.json'));
const startupBody=await readBounded(bindingPath);
await retain(startupBody,'startup.json',{sha256:envelope.startupSHA256});
await fs.writeFile(path.join(root,'harness-inputs.json'),envelopeBody,{flag:'wx',mode:0o600});
const hostBinding = JSON.parse(startupBody);
if (hostBinding.origin !== base || hostBinding.manifestSHA256 !== sha(manifestBody)) throw new Error('Live host binding differs');
const playwrightPath = '/private/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
const playwrightPackage = path.join(path.dirname(playwrightPath), 'package.json');
const toolIdentity = { entrySHA: sha(await fs.readFile(playwrightPath)), packageSHA: sha(await fs.readFile(playwrightPackage)) };
if (toolIdentity.entrySHA !== 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f' || toolIdentity.packageSHA !== '8bc2ddebe8d9b051b9ecb87d62c6c63877e59d08617f32c3215681ca8bb70998') throw new Error('Existing isolated Playwright identity changed');
const config = { base, build, root, zoom:actualZoom?200:100, manifestSHA: sha(manifestBody), hostBinding, playwright: toolIdentity, chrome: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', views: (actualZoom?[1440]:[1440,390]).flatMap(width=>['light','dark'].map(colorScheme=>({width,height:1000,colorScheme}))), latency: '30 Playwright keyboard inputs; input listener to second requestAnimationFrame, a paint-latency proxy, not physical paint or IME proof', device: { platform: os.platform(), arch: os.arch(), release: os.release(), cpu: os.cpus()[0]?.model, memoryBytes: os.totalmem() } };
config.inputBindings={envelopeSHA256:sha(envelopeBody),files:inputBindings};
await fs.writeFile(path.join(root, 'config.json'), JSON.stringify(config,null,2)+'\n', {flag:'wx'});
console.log(JSON.stringify({root,phase:'harness-retained'}));
const { runEditorTasks } = await import(pathToFileURL(path.join(root,'editor-tasks.mjs')).href);
const { runScaleTasks } = await import(pathToFileURL(path.join(root,'scale-tasks.mjs')).href);
const { runMorphTasks } = await import(pathToFileURL(path.join(root,'morph-tasks.mjs')).href);
const { zoomCaptureClip, prepareContext } = await import(pathToFileURL(path.join(root,'zoom-contract.mjs')).href);
const { verifyCapturePNG } = await import(pathToFileURL(path.join(root,'zoom-capture.mjs')).href);
const { chromium } = await import(pathToFileURL(playwrightPath).href);
const report = {config,checks:[],captures:[],latency:[],scale:[],zoom:[],requests:[],responses:[],blocked:[],errors:[],console:[],limits:['Private client-owned specimen, not a Go conversion, public API or certified target.','Reused isolated dependencies; no clean dependency installation.','Automated Chrome/macOS keyboard and pointer, not human acceptance, IME, clipboard, AT or native-device proof.',actualZoom?'Actual Chrome 200% zoom in isolated profiles; no-script notice checked separately at 100%.':'No actual browser-zoom evidence in this ordinary four-view run.','Input timing is a two-animation-frame proxy; scoped long-task/GC diagnostics are not full comparative performance or leak certification.']};
const check=(name,pass,detail)=>{report.checks.push({name,pass:!!pass,detail});};
report.morph=[];report.expectedFaults=[];
let browser, phase='launch';
const ownedContexts=new Set();
function ownContext(context){ownedContexts.add(context);context.once('close',()=>ownedContexts.delete(context));return context;}
async function closeContexts(){
  const outcomes=await Promise.allSettled([...ownedContexts].map(context=>context.close()));
  for(const outcome of outcomes)if(outcome.status==='rejected')report.errors.push({phase,error:'Context cleanup failed: '+String(outcome.reason)});
}
let terminateRequested=false;
process.once('SIGTERM',()=>{
  terminateRequested=true;
  report.errors.push({phase,error:'Owned browser trial interrupted by SIGTERM'});
  void closeContexts();
  if(browser) void browser.close().catch(error=>report.errors.push({phase,error:String(error)}));
});
const pending=new Set();
async function drain(){while(pending.size) await Promise.allSettled([...pending]);}
async function instrument(context,page,tag){
  await context.route('**/*',async route=>{
    const request=route.request(),url=new URL(request.url());
    if(url.origin!==base||!['/','/context',...(withMorph?['/morph-fragment']:[]),...assets.keys()].includes(url.pathname)||url.search||request.method()!=='GET'){
      report.blocked.push({phase:tag,url:request.url(),method:request.method()});await route.abort('blockedbyclient');return;
    }
    await route.continue();
  });
  page.on('pageerror',error=>report.errors.push({phase:tag,error:String(error)}));
  page.on('console',message=>{if(message.type()==='error'){
    const text=message.text(),url=message.location().url;
    report.console.push({phase:tag,text,url,expected:false});
  }});
  page.on('request',request=>report.requests.push({phase:tag,url:request.url(),method:request.method()}));
  page.on('response',response=>{
    const promise=(async()=>{
      const url=new URL(response.url()),asset=assets.get(url.pathname);
      if(asset){const bytes=await response.body();report.responses.push({phase:tag,path:url.pathname,status:response.status(),sha256:sha(bytes),bytes:bytes.length});check(`${tag}: exact asset ${url.pathname}`,response.status()===200&&sha(bytes)===asset.sha256&&bytes.length===asset.bytes);}
    })().catch(error=>report.errors.push({phase:tag,error:String(error)}));
    pending.add(promise);promise.finally(()=>pending.delete(promise));
  });
}
async function capture(page,name){
  await page.locator('.editor-container').first().scrollIntoViewIfNeeded();
  const geometry=await page.locator('.editor-container').first().evaluate(e=>{const r=e.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,scrollWidth:e.scrollWidth,clientWidth:e.clientWidth,viewport:{width:innerWidth,height:innerHeight},page:{width:document.documentElement.scrollWidth,height:document.documentElement.scrollHeight}};});
  const target=path.join(root,`${name}.png`);
  let bytes, zoom;
  if(actualZoom){
    const cdp=await page.context().newCDPSession(page);
    try{
      const metrics=await cdp.send('Page.getLayoutMetrics'),clip=zoomCaptureClip(metrics);
      const shot=await cdp.send('Page.captureScreenshot',{format:'png',fromSurface:true,captureBeyondViewport:true,clip});
      bytes=Buffer.from(shot.data,'base64');zoom={metrics,clip,png:verifyCapturePNG(bytes,{clip})};
      await fs.writeFile(target,bytes,{flag:'wx',mode:0o600});
    }finally{await cdp.detach();}
  }else{await page.screenshot({path:target,fullPage:true,animations:'disabled'});bytes=await fs.readFile(target);}
  report.captures.push({name,path:target,sha256:sha(bytes),width:bytes.readUInt32BE(16),height:bytes.readUInt32BE(20),geometry,...(zoom?{zoom}:{})});
}
async function checkZoom(page,tag,point){
  const geometry=await page.evaluate(()=>({outerWidth,innerWidth,innerHeight,devicePixelRatio,scale:visualViewport.scale,scrollWidth:document.documentElement.scrollWidth}));
  const pass=geometry.outerWidth===1440&&geometry.innerWidth===720&&geometry.innerHeight===500&&geometry.devicePixelRatio===2&&geometry.scale===1&&geometry.scrollWidth===720;
  report.zoom.push({phase:tag,point,geometry,pass});check(`${tag}: actual 200% zoom ${point}`,pass,geometry);
  if(!pass)throw new Error('Actual browser zoom geometry differs');
}
async function createContext(view){
  const options={viewport:{width:view.width,height:view.height},colorScheme:view.colorScheme,reducedMotion:'reduce',serviceWorkers:'block'};
  if(!actualZoom)return ownContext(await browser.newContext(options));
  const profile=await fs.mkdtemp(path.join(root,'chrome-profile-'));await fs.chmod(profile,0o700);
  const context=ownContext(await chromium.launchPersistentContext(profile,{...options,executablePath:config.chrome,headless:true,args:['--no-first-run','--no-default-browser-check']}));
  return prepareContext(context,async()=>{
    if(terminateRequested)throw new Error('Trial interrupted during persistent browser launch');
    const settings=context.pages()[0];settings.setDefaultTimeout(8000);
    await settings.goto('chrome://settings/appearance');await settings.locator('#zoomLevel').selectOption('2');
    if(await settings.locator('#zoomLevel').inputValue()!=='2')throw new Error('Chrome refused actual 200% zoom');
    await settings.goto('about:blank');
    report.zoom.push({phase,profile,setting:'2',chromeVersion:context.browser().version()});return context;
  });
}
try{
  browser=await chromium.launch({executablePath:config.chrome,headless:true,args:['--no-first-run','--no-default-browser-check']});
  if(terminateRequested) throw new Error('Trial interrupted during browser launch');
  report.chromeVersion=browser.version();
  for(const view of config.views){
    phase=`${view.width}-${view.colorScheme}`;
    const tag=phase;
    const context=await createContext(view);
    const page=await context.newPage();page.setDefaultTimeout(8000);
    await instrument(context,page,tag);
    await page.goto(base,{waitUntil:'networkidle'});
    await page.waitForFunction(()=>window.P12Specimen?.primary?.ready);
    await page.evaluate(()=>document.fonts.ready);await drain();
    if(actualZoom)await checkZoom(page,tag,'before tasks');
    check(`${tag}: named local icons render`,await page.locator('.toolbar svg').count()===9);
    check(`${tag}: source theme selected`,await page.evaluate(expected=>document.documentElement.dataset.colorTheme==='BLUE_THEME'&&document.documentElement.classList.contains('dark')===expected,view.colorScheme==='dark'));
    const firstFonts=report.responses.filter(r=>r.phase===tag&&r.path.endsWith('.woff2'));
    const fontBytes=[...new Map(firstFonts.map(f=>[f.path,f.bytes])).values()].reduce((a,b)=>a+b,0);
    check(`${tag}: first-view font budget`,fontBytes>0&&fontBytes<=350*1024,{fontBytes,paths:firstFonts.map(f=>f.path)});
    await capture(page,`${tag}-initial`);
    await runEditorTasks(page,base,tag,check,async state=>capture(page,`${tag}-${state}`));
    await page.waitForFunction(()=>window.P12Specimen?.primary?.ready);

    const editor=page.getByRole('textbox',{name:'Product description',exact:true});
    await editor.click();await page.keyboard.press('Meta+a');await page.keyboard.type('Draft with stable history');
    await page.keyboard.press('Meta+a');await page.getByRole('button',{name:'Bold',exact:true}).click();
    await page.evaluate(()=>{
      const selection=getSelection();window.p12Before={node:document.querySelector('#editor-slot .tiptap'),active:document.activeElement,anchor:selection.anchorNode,anchorOffset:selection.anchorOffset,focus:selection.focusNode,focusOffset:selection.focusOffset,snapshot:JSON.stringify(window.P12Specimen.primary.snapshot())};
      window.P12Specimen.primary.update({version:1,instanceId:'primary',label:'Updated product description',hint:'Context updated. Your local draft is unchanged.'});
    });
    const preserved=await page.evaluate(()=>{
      const before=window.p12Before,selection=getSelection();return {node:before.node===document.querySelector('#editor-slot .tiptap'),focus:before.active===document.activeElement,selection:before.anchor===selection.anchorNode&&before.anchorOffset===selection.anchorOffset&&before.focus===selection.focusNode&&before.focusOffset===selection.focusOffset,draft:before.snapshot===JSON.stringify(window.P12Specimen.primary.snapshot()),label:before.node.getAttribute('aria-label')==='Updated product description',hint:document.getElementById(before.node.getAttribute('aria-describedby')).textContent==='Context updated. Your local draft is unchanged.'};
    });
    check(`${tag}: context preserves node focus selection draft and revision`,Object.values(preserved).every(Boolean),preserved);
    await page.keyboard.press('Meta+z');const undone=await page.locator('#editor-slot .tiptap strong').count()===0;
    await page.keyboard.press('Meta+Shift+z');
    check(`${tag}: context preserves undo and redo`,undone&&await page.locator('#editor-slot .tiptap strong').count()===1);
    const invalid=await page.evaluate(()=>{
      const api=window.P12Specimen,slot=document.getElementById('editor-slot'),second=document.getElementById('secondary-slot');
      const before=JSON.stringify(api.primary.snapshot()),node=slot.querySelector('.tiptap'),html=slot.innerHTML;
      const init={version:1,instanceId:'primary',label:'Other',hint:'',initialText:'Other'};
      const attempts=[()=>api.primary.update({...init}),()=>api.primary.update({version:1,instanceId:'wrong',label:'Other',hint:''}),()=>api.primary.update({version:2,instanceId:'primary',label:'Other',hint:''}),()=>api.primary.update({version:1,instanceId:'primary',label:'',hint:''}),()=>api.primary.update({version:1,instanceId:'primary',label:'x',hint:'\ud800'}),()=>api.mount(slot,{...init,instanceId:'different'}),()=>api.mount(second,init),()=>api.mount(document.createElement('div'),{...init,instanceId:'detached'})];
      return attempts.map(attempt=>{let refused=false;try{attempt();}catch{refused=true;}return refused&&JSON.stringify(api.primary.snapshot())===before&&slot.querySelector('.tiptap')===node&&slot.innerHTML===html&&second.childNodes.length===0;});
    });
    check(`${tag}: invalid updates and duplicate/disconnected mounts refuse atomically`,invalid.length===8&&invalid.every(Boolean),invalid);
    const detached=await page.evaluate(()=>{
      const api=window.P12Specimen,before=JSON.stringify(api.primary.snapshot()),copy=api.primary.snapshot();copy.document.content[0].content[0].text='Changed detached snapshot';return JSON.stringify(api.primary.snapshot())===before;
    });check(`${tag}: snapshots do not expose mutable editor state`,detached);
    await page.evaluate(()=>{
      window.p12SecondaryChanges=0;
      window.p12Secondary=window.P12Specimen.mount(document.getElementById('secondary-slot'),{version:1,instanceId:'secondary',label:'Secondary description',hint:'Local second draft',initialText:'<strong>Plain text</strong>\nSecond line'},()=>window.p12SecondaryChanges++);
    });
    const second=page.getByRole('textbox',{name:'Secondary description',exact:true});
    check(`${tag}: initial text is not interpreted as HTML`,await second.locator('strong').count()===0&&await second.locator('p').count()===2&&(await second.textContent()).includes('<strong>Plain text</strong>'));
    check(`${tag}: two instances have unique accessible IDs`,await page.evaluate(()=>{const ids=[...document.querySelectorAll('[id]')].map(e=>e.id);const boxes=[...document.querySelectorAll('[role=textbox]')];return new Set(ids).size===ids.length&&boxes.length===2&&new Set(boxes.map(e=>e.getAttribute('aria-describedby'))).size===2;}));
    const beforePrimary=await page.evaluate(()=>JSON.stringify(window.P12Specimen.primary.snapshot()));
    await second.click();await page.keyboard.press('Meta+a');await page.keyboard.type('Independent second draft');
    check(`${tag}: instance edits stay isolated`,beforePrimary===await page.evaluate(()=>JSON.stringify(window.P12Specimen.primary.snapshot()))&&await page.evaluate(()=>window.p12SecondaryChanges>0));
    await page.evaluate(()=>window.p12Secondary.dispose());

    await page.getByRole('button',{name:'Update context',exact:true}).click();
    await page.waitForFunction(()=>document.getElementById('host-state').textContent==='Context updated. Local draft unchanged.');
    check(`${tag}: actual Go context roundtrip preserves draft`,beforePrimary===await page.evaluate(()=>JSON.stringify(window.P12Specimen.primary.snapshot())));
    const updated=page.getByRole('textbox',{name:'Updated product description',exact:true});
    await updated.click();await page.keyboard.press('Meta+ArrowRight');
    const requestCount=report.requests.length;
    await page.evaluate(()=>{window.p12StatusMutations=0;window.p12StatusObserver=new MutationObserver(records=>window.p12StatusMutations+=records.length);window.p12StatusObserver.observe(document.getElementById('host-state'),{childList:true,characterData:true,subtree:true});});
    await context.setOffline(true);await page.keyboard.type(' local offline edit');
    check(`${tag}: editing works disconnected without requests`,report.requests.length===requestCount&&(await updated.textContent()).includes('local offline edit'));
    check(`${tag}: typing does not repeatedly announce revisions`,await page.evaluate(()=>{window.p12StatusObserver.disconnect();return window.p12StatusMutations===1&&document.getElementById('host-state').textContent==='Local draft changed. Nothing is saved.';}));
    await context.setOffline(false);

    await page.evaluate(()=>{
      window.p12Samples=[];window.p12SampleInput=()=>{const start=performance.now();requestAnimationFrame(()=>requestAnimationFrame(()=>window.p12Samples.push(performance.now()-start)));};document.querySelector('#editor-slot .tiptap').addEventListener('input',window.p12SampleInput);
    });
    for(let i=0;i<30;i++){await page.keyboard.type('x');await page.waitForFunction(n=>window.p12Samples.length>=n,i+1);}
    const samples=await page.evaluate(()=>{document.querySelector('#editor-slot .tiptap').removeEventListener('input',window.p12SampleInput);return window.p12Samples;});
    const sorted=[...samples].sort((a,b)=>a-b),p95=sorted[Math.ceil(sorted.length*.95)-1];
    report.latency.push({phase:tag,samples,p95,documentCharacters:await updated.evaluate(e=>e.textContent.length)});
    check(`${tag}: 30 input two-frame latency samples stay within screening budget`,samples.length===30&&p95<=100,{p95,method:config.latency});
    const dispose=await page.evaluate(()=>{
      const api=window.P12Specimen,slot=document.getElementById('editor-slot'),oldNode=slot.querySelector('.tiptap');window.p12Stale=api.primary;window.p12ChangesAtDispose=api.changes;api.primary.dispose();api.primary.dispose();
      let updateRefused=false,snapshotRefused=false;try{api.primary.update({version:1,instanceId:'primary',label:'x',hint:''});}catch{updateRefused=true;}try{api.primary.snapshot();}catch{snapshotRefused=true;}
      oldNode.dispatchEvent(new InputEvent('input',{bubbles:true,inputType:'insertText',data:'late'}));document.dispatchEvent(new Event('selectionchange'));
      return {empty:slot.childNodes.length===0,disposed:api.primary.disposed,notReady:!api.primary.ready,updateRefused,snapshotRefused,focusReleased:document.activeElement!==oldNode};
    });
    await page.keyboard.type('after disposal');
    await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
    check(`${tag}: focused disposal is idempotent and stale operations refuse`,Object.values(dispose).every(Boolean),dispose);
    check(`${tag}: disposed editor emits no later callbacks`,await page.evaluate(()=>window.P12Specimen.changes===window.p12ChangesAtDispose));
    await page.getByRole('button',{name:'Mount editor',exact:true}).click();await page.waitForFunction(()=>window.P12Specimen.primary.ready);
    check(`${tag}: same element remount starts explicit fresh document`,await page.getByRole('textbox',{name:'Product description',exact:true}).textContent()==='Start typing...'&&await page.evaluate(()=>window.P12Specimen.primary!==window.p12Stale&&window.P12Specimen.primary.snapshot().revision===0));
    await capture(page,`${tag}-remounted`);
    report.scale.push(await runScaleTasks(page,context,tag,check,async(state,editor)=>{
      await editor.scrollIntoViewIfNeeded();
      const geometry=await editor.evaluate(e=>{const r=e.getBoundingClientRect();return {editor:{x:r.x,y:r.y,width:r.width,height:r.height,scrollTop:e.scrollTop,scrollHeight:e.scrollHeight},viewport:{width:innerWidth,height:innerHeight},document:{width:document.documentElement.scrollWidth,height:document.documentElement.scrollHeight},scope:'viewport capture of large editor task, not full long preview'};});
      const target=path.join(root,`${tag}-${state}.png`);let bytes,zoom;
      if(actualZoom){
        await checkZoom(page,tag,'large-document capture');
        const cdp=await context.newCDPSession(page);
        try{
          const metrics=await cdp.send('Page.getLayoutMetrics');
          const shot=await cdp.send('Page.captureScreenshot',{format:'png',fromSurface:true,captureBeyondViewport:false});
          bytes=Buffer.from(shot.data,'base64');
          zoom={metrics,scope:'viewport only, not full document',png:verifyCapturePNG(bytes,{clip:{width:1440,height:1000}})};
          await fs.writeFile(target,bytes,{flag:'wx',mode:0o600});
        }finally{await cdp.detach();}
      }else{await page.screenshot({path:target,fullPage:false,animations:'disabled'});bytes=await fs.readFile(target);}
      report.captures.push({name:`${tag}-${state}`,path:target,sha256:sha(bytes),width:bytes.readUInt32BE(16),height:bytes.readUInt32BE(20),geometry,...(zoom?{zoom}:{})});
    }));
    if(withMorph)report.morph.push(await runMorphTasks(page,base,tag,check,state=>capture(page,`${tag}-${state}`),report.requests,report.expectedFaults));
    if(actualZoom)await checkZoom(page,tag,'after tasks');
    await drain();await context.close();
  }
  phase='no-script';
  const context=ownContext(await browser.newContext({javaScriptEnabled:false,viewport:{width:390,height:844}}));
  const page=await context.newPage();await instrument(context,page,'no-script');await page.goto(base,{waitUntil:'networkidle'});
  check('no-script: honest limitation and no editable surface',await page.locator('noscript').isVisible()&&(await page.locator('noscript').textContent()).includes('requires JavaScript')&&await page.getByRole('textbox').count()===0);
  await drain();await context.close();
}catch(error){report.errors.push({phase,error:String(error),stack:error.stack});}
finally{
  await drain();await closeContexts();if(browser)await browser.close();
  // Classify after delivery, so late console messages cannot race route cleanup.
  // One exact resource diagnostic is allowed per retained injected 503 response.
  const allowances=report.expectedFaults.map(f=>({...f,remaining:f.fulfilled??0}));
  for(const entry of report.console){
    const fault=allowances.find(f=>f.remaining>0&&f.phase===entry.phase&&f.url===entry.url&&f.status===503&&entry.text==='Failed to load resource: the server responded with a status of 503 (Service Unavailable)');
    if(fault){entry.expected=true;fault.remaining--;}
  }
  check('no unexpected network destination',report.blocked.length===0,report.blocked);
  check('no unexpected runtime or browser-console errors',report.errors.length===0&&report.console.every(c=>c.expected),{errors:report.errors,console:report.console});
  check('all selected source task and lifecycle views completed',config.views.every(v=>report.checks.some(c=>c.pass&&c.name===`${v.width}-${v.colorScheme}: same element remount starts explicit fresh document`)));
  check('all selected representative-document probes completed',config.views.every(v=>report.scale.some(s=>s.phase===`${v.width}-${v.colorScheme}`&&s.checks.every(c=>c.pass))));
  if(withMorph)check('all selected pinned-runtime morph probes completed',config.views.every(v=>report.morph.some(s=>s.phase===`${v.width}-${v.colorScheme}`&&s.checks.every(c=>c.pass))));
  if(actualZoom)check('both actual-zoom views completed without changing zoom',config.views.every(v=>['before tasks','large-document capture','after tasks'].every(point=>report.zoom.some(z=>z.phase===`${v.width}-${v.colorScheme}`&&z.point===point&&z.pass))));
  const assetChecks=report.checks.filter(c=>c.name.includes(': exact asset '));
  const taskChecks=report.checks.filter(c=>!c.name.includes(': exact asset '));
  report.summary={passed:report.checks.filter(c=>c.pass).length,failed:report.checks.filter(c=>!c.pass).length,taskAssertions:{passed:taskChecks.filter(c=>c.pass).length,failed:taskChecks.filter(c=>!c.pass).length},assetObservations:{passed:assetChecks.filter(c=>c.pass).length,failed:assetChecks.filter(c=>!c.pass).length,uniquePaths:new Set(report.responses.map(r=>r.path)).size},captures:report.captures.length};
  await fs.writeFile(path.join(root,'receipt.json'),JSON.stringify(report,null,2)+'\n',{flag:'wx'});
  console.log(JSON.stringify({root,summary:report.summary,failures:report.checks.filter(c=>!c.pass),errors:report.errors}));
  if(report.summary.failed)process.exitCode=1;
}
