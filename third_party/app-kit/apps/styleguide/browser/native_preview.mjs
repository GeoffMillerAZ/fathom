// Explicit local-browser canary. No source rewrite, preference save, install,
// consumer write or real account. Normal assertions also run against baselines.
import { readFileSync, writeFileSync, mkdtempSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { configuration } from '../../gallery/browser/config.mjs';

if (!process.env.APPKIT_THEME_BROWSER_ORIGIN) throw new Error('Select APPKIT_THEME_BROWSER_ORIGIN; absent evidence is not a skip.');
const config=configuration({...process.env,APPKIT_SHELL_BROWSER_ORIGIN:process.env.APPKIT_THEME_BROWSER_ORIGIN});
const scriptMode=process.env.APPKIT_THEME_BROWSER_SCRIPT_MODE??'enabled';
if(!['enabled','disabled'].includes(scriptMode))throw new Error('Script mode must be enabled or disabled.');
const hash=bytes=>createHash('sha256').update(bytes).digest('hex');
const moduleBytes=readFileSync(new URL(config.moduleURL)),packageBytes=readFileSync(new URL('./package.json',config.moduleURL));
if(hash(moduleBytes)!=='a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f'||hash(packageBytes)!=='8bc2ddebe8d9b051b9ecb87d62c6c63877e59d08617f32c3215681ca8bb70998')throw new Error('Selected approved Playwright identity differs.');
const {chromium}=await import(config.moduleURL);
const root=mkdtempSync(join(tmpdir(),'appkit-theme-native-browser-'));
console.log('Theme-native browser evidence: '+root);
const report={origin:config.base,variant:config.variant,probe_sha256:hash(readFileSync(new URL(import.meta.url))),started:new Date().toISOString(),checks:[],requests:[],responses:[],blocked:[],errors:[],console:[],screenshots:[],workers:[],limits:[
  'One explicit native form task, not the complete theme-edit/export/consumer workflow.',
  'Browser script evaluation reads diagnostics; no page script is injected or CSP bypassed.',
  'Synthetic local draft only; source/preferences/files are not applied.',
  'Playwright worker-block init script is omitted: it throws in opaque frames. Fresh context, document script CSP, request allowlist and worker-event assertions remain enforced.',
]};
report.page_javascript=scriptMode;
const check=(name,pass,detail)=>report.checks.push({name,pass:!!pass,detail});
let browser,page,context;
const pending=new Set();
const drain=async()=>{while(pending.size)await Promise.allSettled([...pending]);};
async function bounded(promise){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error('Response body deadline exceeded')),10000);})]);}finally{clearTimeout(timer);}}
async function shot(name){const path=join(root,name+'.png');await page.screenshot({path,fullPage:false});report.screenshots.push({name,path,sha256:hash(readFileSync(path))});}
try{
  browser=await chromium.launch({executablePath:config.executablePath,headless:true,args:['--disable-background-networking','--no-proxy-server']});
  report.browser=browser.version();
  context=await browser.newContext({viewport:{width:1440,height:1000},reducedMotion:'reduce',acceptDownloads:false,javaScriptEnabled:scriptMode==='enabled'});
  context.on('serviceworker',worker=>report.workers.push(worker.url()));
  check('fresh context has no service worker',context.serviceWorkers().length===0);
  context.setDefaultTimeout(5000);context.setDefaultNavigationTimeout(15000);
  await context.route('**/*',route=>{
    const r=route.request(),u=new URL(r.url());
    const allowed=u.origin===config.base&&((r.method()==='GET'&&['/theme-lab','/theme-lab/style.css','/theme-lab/preview','/static/app.css'].includes(u.pathname))||(r.method()==='POST'&&u.pathname==='/theme-lab'));
    if(!allowed){report.blocked.push({method:r.method(),url:r.url()});return route.abort('blockedbyclient');}
    if(r.method()==='POST')report.requests.push({method:r.method(),path:u.pathname,origin:r.headers().origin,referer:r.headers().referer});
    return route.continue();
  });
  await context.routeWebSocket('**/*',ws=>{report.blocked.push({method:'WS',url:ws.url()});ws.close();});
  page=await context.newPage();
  page.on('pageerror',e=>report.errors.push(String(e)));
  page.on('console',m=>{if(['warning','error'].includes(m.type()))report.console.push({type:m.type(),text:m.text()});});
  page.on('response',response=>{
    const task=(async()=>{const body=await bounded(response.body());report.responses.push({url:response.url(),status:response.status(),sha256:hash(body),bytes:body.length,headers:response.headers()});})();
    pending.add(task);task.catch(e=>report.errors.push(String(e))).finally(()=>pending.delete(task));
  });
  const initial=await page.goto(config.base+'/theme-lab',{waitUntil:'networkidle'});
  check('initial theme guide loads',initial.status()===200,initial.status());
  check('native form policy preserves same-origin identity',initial.headers()['referrer-policy']==='same-origin',initial.headers()['referrer-policy']);
  check('preview retains opaque sandbox',await page.locator('iframe').getAttribute('sandbox')==='');
  await shot('initial-1440');
  await page.locator('details').filter({has:page.locator('summary',{hasText:'Corner radii'})}).locator('summary').click();
  await page.getByLabel('radius.md',{exact:true}).fill('0.75rem');
  const button=page.getByRole('button',{name:'Preview candidate',exact:true});await button.focus();
  const [response]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/theme-lab'),page.keyboard.press('Enter')]);
  await page.waitForLoadState('networkidle');
  check('native keyboard preview accepted',response.status()===200,response.status());
  check('actual native POST carries exact same-origin header',report.requests.length===1&&report.requests[0].origin===config.base,report.requests);
  if(response.status()===200){
    check('preview retains authored radius',await page.getByLabel('radius.md',{exact:true}).inputValue()==='0.75rem');
    check('preview acknowledges current candidate',/^[0-9a-f]{64}$/.test(await page.locator('[name="review_digest"]').inputValue()));
  }
  await shot('after-native-preview');
}catch(error){report.failure=String(error);}
finally{
  await drain();
  const previews=report.responses.filter(r=>new URL(r.url).pathname==='/theme-lab/preview');
  check('successful preview responses preserve privacy and sandbox',previews.length>=2&&previews.every(r=>r.status===200&&r.headers['referrer-policy']==='no-referrer'&&r.headers['content-security-policy']?.includes('; sandbox')&&["script-src 'none'","form-action 'none'","font-src 'none'","img-src 'none'"].every(p=>r.headers['content-security-policy'].includes(p))),previews.map(r=>({url:r.url,status:r.status,headers:r.headers})));
  check('document CSP refuses page scripts and workers',report.responses.filter(r=>r.headers['content-type']?.startsWith('text/html')).every(r=>r.headers['content-security-policy']?.includes("script-src 'none'")&&r.headers['content-security-policy']?.includes("default-src 'none'")));
  check('no service worker appeared',report.workers.length===0&&context?.serviceWorkers().length===0,report.workers);
  if(browser)await browser.close();await drain();
  check('no out-of-scope request',report.blocked.length===0,report.blocked);
  check('no browser execution error',report.errors.length===0,report.errors);
  check('no console warning or error',report.console.length===0,report.console);
  const expected=['fresh context has no service worker','initial theme guide loads','native form policy preserves same-origin identity','preview retains opaque sandbox','native keyboard preview accepted','actual native POST carries exact same-origin header','preview retains authored radius','preview acknowledges current candidate','successful preview responses preserve privacy and sandbox','document CSP refuses page scripts and workers','no service worker appeared','no out-of-scope request','no browser execution error','no console warning or error'];
  check('exact assertion set completed',JSON.stringify(report.checks.map(c=>c.name).sort())===JSON.stringify(expected.sort()));
  report.ok=!report.failure&&report.checks.every(c=>c.pass);
  writeFileSync(join(root,'receipt.json'),JSON.stringify(report,null,2)+'\n',{flag:'wx'});
  console.log(JSON.stringify({root,ok:report.ok,checks:report.checks.length,failed:report.checks.filter(c=>!c.pass),failure:report.failure}));
  if(!report.ok)process.exitCode=1;
}
