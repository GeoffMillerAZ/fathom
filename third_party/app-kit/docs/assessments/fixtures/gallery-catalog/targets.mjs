// Local, synthetic catalog task/target probe. Existing approved Chrome only.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {spawn} from 'node:child_process';

const [parent,binary]=process.argv.slice(2);
assert.ok(path.isAbsolute(parent)&&path.isAbsolute(binary));
const sha=b=>createHash('sha256').update(b).digest('hex'),write=(p,b)=>fs.writeFile(p,b,{flag:'wx',mode:0o600});
const root=await fs.mkdtemp(path.join(await fs.realpath(parent),'catalog-targets-'));await fs.chmod(root,0o700);
const source=fileURLToPath(import.meta.url),sourceBytes=await fs.readFile(source),binaryBytes=await fs.readFile(binary);
await write(path.join(root,'targets.mjs'),sourceBytes);
const pw=process.env.APPKIT_PLAYWRIGHT_MODULE;assert.ok(pw&&path.isAbsolute(pw));
const pwSHA=sha(await fs.readFile(pw));assert.equal(pwSHA,'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
const {chromium}=await import(pathToFileURL(pw));
const report={kind:'gallery-catalog-targets-v2',verification:'not-certified',root,sourceSHA:sha(sourceBytes),binary:{path:binary,sha256:sha(binaryBytes)},playwrightEntrySHA:pwSHA,startedAt:new Date().toISOString(),checks:[],pages:[],failures:[],browserProcess:null,galleryProcess:null,limits:['Catalog main-content controls only: local Chrome geometry/native-key tasks, not physical touch, all browsers, full accessibility or visual certification.','The probe owns launch and binds binary, origin and served CSS; build/source closure is separate operator evidence.']};
const check=(name,passed,detail)=>{report.checks.push({name,passed,detail});if(!passed)report.failures.push({name,detail})};
async function bounded(promise,ms,label){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' timed out')),ms)})])}finally{clearTimeout(timer)}}
let origin,server,browser,proc,browserExit,gallery,galleryClosed,stdout='',stderr='';
const galleryArgs=['-addr','127.0.0.1:0','-design-assets',''];
const timeout=setTimeout(()=>{report.failures.push({name:'watchdog',detail:'90 seconds'});server?.kill().catch(()=>{});gallery?.kill('SIGKILL')},90000);
async function tabTo(page,selector){for(let i=0;i<100;i++){if(await page.evaluate(s=>document.activeElement?.matches(s)===true,selector))return;await page.keyboard.press('Tab')}throw Error('native Tab did not reach '+selector)}
try{
  gallery=spawn(binary,galleryArgs,{cwd:root,stdio:['ignore','pipe','pipe']});
  galleryClosed=new Promise(resolve=>gallery.once('close',resolve));
  gallery.stdout.on('data',b=>{stdout+=b.toString()});
  const ready=new Promise((resolve,reject)=>{gallery.once('error',reject);gallery.once('exit',()=>reject(Error('Gallery exited before readiness')));gallery.stderr.on('data',b=>{stderr+=b.toString();const m=stderr.match(/app-kit Gallery → (http:\/\/127\.0\.0\.1:\d+)/);if(m)resolve(m[1])})});
  origin=await bounded(ready,15000,'Gallery readiness');report.origin=origin;
  const readiness=await fetch(origin+'/catalog',{signal:AbortSignal.timeout(6000)});check('owned Gallery ready',readiness.status===200,{status:readiness.status});await readiness.arrayBuffer();
  server=await chromium.launchServer({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',args:['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1']});proc=server.process();browserExit=new Promise(resolve=>proc.once('exit',resolve));browser=await chromium.connect(server.wsEndpoint());
  report.browserVersion=browser.version();assert.equal(report.browserVersion,'152.0.7977.84');
  for(const width of [390,1440])for(const scheme of ['light','dark']){
    const context=await browser.newContext({viewport:{width,height:1000},colorScheme:scheme,reducedMotion:'reduce',javaScriptEnabled:false,serviceWorkers:'block'});
    await context.route('**/*',route=>{const u=new URL(route.request().url());if(u.origin===origin&&route.request().method()==='GET'&&(u.pathname.startsWith('/catalog')||u.pathname==='/static/app.css'||u.pathname==='/api/catalog'))return route.continue();check('unexpected request',false,route.request().url());return route.abort()});
    const page=await context.newPage();page.setDefaultTimeout(6000);
    page.on('pageerror',e=>check('page error',false,String(e)));
    page.on('requestfailed',r=>check('request failed',false,{url:r.url(),error:r.failure()}));
    let styles=[];
    page.on('response',r=>{if(r.url()===origin+'/static/app.css')styles.push(r);if(r.request().resourceType()==='document'){const u=new URL(r.url()),expected=u.pathname==='/catalog'&&u.searchParams.get('cursor')==='bad'?400:200;check('navigation HTTP status',r.status()===expected,{url:r.url(),status:r.status(),expected})}else if(!r.ok())check('subresource HTTP failure',false,{url:r.url(),status:r.status()})});
    async function verifyStyle(target){
      assert.equal(styles.length,1,'exact Gallery stylesheet response required');
      check('stylesheet HTTP success',styles[0].status()===200&&/^text\/css\b/.test(styles[0].headers()['content-type']||''),{target,status:styles[0].status()});
      const css=await styles[0].body(),cssSHA=sha(css);
      if(!report.css){report.css={file:'app.css',sha256:cssSHA,bytes:css.length};await write(path.join(root,'app.css'),css)}
      check('served CSS unchanged',cssSHA===report.css.sha256,{target,cssSHA});
    }
    for(const [route,status] of [['/catalog',200],['/catalog?q=navigation&profile=go-web',200],['/catalog?q=definitelynotpresent',200],['/catalog?cursor=bad',400],['/catalog/shell.layout?q=navigation&profile=go-web',200]]){
      styles=[];
      const target=origin+route+(route.includes('?')?'&':'?')+'theme='+scheme;
      const response=await page.goto(target,{waitUntil:'load'});check('expected HTTP status',response.status()===status,{target,actual:response.status(),expected:status});
      await verifyStyle(target);
      const measurements=await page.evaluate(()=>{
        const main=document.querySelector('main');
        // Inline prose links retain normal text flow; these are standalone actions.
        const nodes=[...main.querySelectorAll('form input:not([type=hidden]),form select,form button,form a,article h3 a,summary,section > a,section > div > a,main > a,main > p > a,main > details > a')];
        return {title:document.title,scrollWidth:document.documentElement.scrollWidth,viewport:innerWidth,theme:document.documentElement.getAttribute('data-theme'),targets:nodes.filter(e=>e.checkVisibility()).map(e=>{const r=e.getBoundingClientRect();return {tag:e.tagName,text:(e.textContent||e.getAttribute('aria-label')||e.id).trim().slice(0,160),href:e.getAttribute('href'),id:e.id,targetWidth:r.width,targetHeight:r.height}})};
      });
      check('no outer horizontal overflow',measurements.scrollWidth<=measurements.viewport,{width,scheme,route,...measurements});
      check('target inventory not empty',measurements.targets.length>0,{width,scheme,route});
      for(const t of measurements.targets)check('standalone target at least 44px',t.targetWidth>=44&&t.targetHeight>=44,{viewportWidth:width,scheme,route,...t});
      const image=`${width}-${scheme}-${report.pages.length}.png`,png=await page.screenshot({fullPage:true,animations:'disabled'});await write(path.join(root,image),png);
      let disclosed=null;
      if(await page.locator('main details a[href="/api/catalog"]').count()){
        check('closed disclosure excludes hidden action',!measurements.targets.some(t=>t.href==='/api/catalog')&&!await page.locator('main details a[href="/api/catalog"]').evaluate(e=>e.checkVisibility()),{width,scheme,route});
        await tabTo(page,'main details summary');await page.keyboard.press('Enter');
        disclosed=await page.locator('main details a[href="/api/catalog"]').evaluate(e=>{const r=e.getBoundingClientRect();return {visible:e.checkVisibility(),targetWidth:r.width,targetHeight:r.height}});
        check('opened disclosure action at least 44px',disclosed.visible&&disclosed.targetWidth>=44&&disclosed.targetHeight>=44,{viewportWidth:width,scheme,route,...disclosed});
      }
      report.pages.push({width,scheme,route,status,htmlSHA:sha(await response.body()),measurements,disclosed,image,imageSHA:sha(png),imageBytes:png.length});
    }
    styles=[];await page.goto(origin+'/catalog?theme='+scheme,{waitUntil:'load'});await verifyStyle(page.url());
    await tabTo(page,'#catalog-query');await page.keyboard.type('navigation');
    await page.keyboard.press('Tab');check('native next filter focus',await page.evaluate(()=>document.activeElement?.id==='catalog-profile'),{width,scheme});
    await tabTo(page,'form button');styles=[];const navigation=page.waitForURL(u=>u.searchParams.get('q')==='navigation');await page.keyboard.press('Enter');await navigation;await verifyStyle(page.url());
    check('native GET preserves theme/query',new URL(page.url()).searchParams.get('theme')===scheme&&new URL(page.url()).searchParams.get('q')==='navigation',{width,scheme,url:page.url()});
    const link='article h3 a[href^="/catalog/shell.layout"]';
    await tabTo(page,link);check('catalog result focus visible',await page.evaluate(()=>{const s=getComputedStyle(document.activeElement);return s.outlineStyle!=='none'&&parseFloat(s.outlineWidth)>=2}),{width,scheme});
    styles=[];const detail=page.waitForURL(u=>u.pathname==='/catalog/shell.layout');await page.keyboard.press('Enter');await detail;await verifyStyle(page.url());
    await tabTo(page,'main > a[href^="/catalog?"]');styles=[];const back=page.waitForURL(u=>u.pathname==='/catalog');await page.keyboard.press('Enter');await back;await verifyStyle(page.url());
    check('native detail return preserves selection',new URL(page.url()).searchParams.get('q')==='navigation'&&new URL(page.url()).searchParams.get('theme')===scheme,{width,scheme,url:page.url()});
    await tabTo(page,'main details summary');await page.keyboard.press('Enter');check('native disclosure opens',await page.evaluate(()=>document.querySelector('main details')?.open===true),{width,scheme});
    await context.close();
  }
  assert.equal(sha(await fs.readFile(source)),report.sourceSHA,'probe source drift');assert.equal(sha(await fs.readFile(binary)),report.binary.sha256,'binary drift');
}catch(e){report.failures.push({name:'execution',detail:String(e.stack||e)})}
finally{
  if(server){try{await bounded(server.close(),7000,'Chrome close')}catch(e){report.failures.push({name:'browser cleanup',detail:String(e)});await bounded(server.kill(),3000,'Chrome kill').catch(e=>report.failures.push({name:'browser kill',detail:String(e)}))}}
  if(proc){try{await bounded(browserExit,3000,'Chrome exit')}catch(e){report.failures.push({name:'browser exit wait',detail:String(e)})}report.browserProcess={pid:proc.pid,exitCode:proc.exitCode,signalCode:proc.signalCode};if(proc.exitCode!==0||proc.signalCode!==null)report.failures.push({name:'browser exit',detail:report.browserProcess})}
  if(gallery){gallery.kill('SIGTERM');try{await bounded(galleryClosed,8000,'Gallery close')}catch(e){report.failures.push({name:'Gallery cleanup',detail:String(e)});gallery.kill('SIGKILL');await bounded(galleryClosed,3000,'Gallery kill/close').catch(e=>report.failures.push({name:'Gallery kill',detail:String(e)}))}report.galleryProcess={pid:gallery.pid,args:galleryArgs,exitCode:gallery.exitCode,signalCode:gallery.signalCode,stdoutSHA:sha(stdout),stderrSHA:sha(stderr)};if(gallery.exitCode!==0||gallery.signalCode!==null)report.failures.push({name:'Gallery exit',detail:report.galleryProcess})}
  clearTimeout(timeout);
  await write(path.join(root,'gallery.stdout'),stdout);await write(path.join(root,'gallery.stderr'),stderr);
  report.finishedAt=new Date().toISOString();report.passed=report.failures.length===0;
  await write(path.join(root,'receipt.json'),JSON.stringify(report,null,2)+'\n');
}
console.log(JSON.stringify({root,passed:report.passed,checks:report.checks.length,failures:report.failures.length,execution:report.failures.filter(f=>f.name==='execution'),browser:report.browserProcess},null,2));process.exitCode=report.passed?0:1;
