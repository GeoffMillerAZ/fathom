// Bounded local-browser review of the native theme-lab loop. This probe writes
// screenshots, downloads and its receipt only to one fresh temporary directory.
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {mkdtempSync, readFileSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {configuration} from '../../gallery/browser/config.mjs';

if (!process.env.APPKIT_THEME_BROWSER_ORIGIN) throw new Error('Select APPKIT_THEME_BROWSER_ORIGIN; absent evidence is not a skip.');
const config=configuration({...process.env,APPKIT_SHELL_BROWSER_ORIGIN:process.env.APPKIT_THEME_BROWSER_ORIGIN});
const hash=bytes=>createHash('sha256').update(bytes).digest('hex');
const moduleBytes=readFileSync(new URL(config.moduleURL));
const packageBytes=readFileSync(new URL('./package.json',config.moduleURL));
if(hash(moduleBytes)!=='a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f'||hash(packageBytes)!=='8bc2ddebe8d9b051b9ecb87d62c6c63877e59d08617f32c3215681ca8bb70998')throw new Error('Selected approved Playwright identity differs.');
const {chromium}=await import(config.moduleURL);
const sourcePath=new URL('../../../cue/tokens/geoff-synthwave/geoff-synthwave.cue',import.meta.url);
const sourceBefore=readFileSync(sourcePath);
const root=mkdtempSync(join(tmpdir(),'appkit-theme-review-loop-'));
console.log('Theme review-loop evidence: '+root);

const report={origin:config.base,variant:config.variant,probe_sha256:hash(readFileSync(new URL(import.meta.url))),source:{path:sourcePath.pathname,before_sha256:hash(sourceBefore),bytes:sourceBefore.length},started:new Date().toISOString(),checks:[],requests:[],responses:[],blocked:[],errors:[],console:[],screenshots:[],workers:[],downloads:[],expected_http_failures:[],limits:[
  'One local native theme-lab review loop; not consumer adoption, saved preferences, visual certification or authenticated approval.',
  'Computed-style evaluation is read-only diagnostic evidence; no script is injected into the page and CSP is not bypassed.',
  'Only the three app-owned static specimens and selected semantic token uses are sampled; this is not exhaustive accessibility, zoom, device or performance coverage.',
  'Playwright serviceWorkers:block is omitted because its init script faults in opaque frames; fresh-context, CSP, route, WebSocket and worker-event gates remain fail-closed.',
  'Non-download response bodies have a ten-second deadline; download bodies use the actual saved artifact because Chromium does not expose response.body() for download navigation.',
]};
const check=(name,pass,detail)=>report.checks.push({name,pass:!!pass,detail});
const pending=new Set();
const bounded=async promise=>{let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error('Response body deadline exceeded')),10000);})]);}finally{clearTimeout(timer);}};
const drain=async()=>{while(pending.size)await Promise.allSettled([...pending]);};
let browser,context,page;

async function shot(name){const path=join(root,name+'.png');await page.screenshot({path,fullPage:false});report.screenshots.push({name,path,sha256:hash(readFileSync(path))});}
async function openDetail(name){const detail=page.locator('details').filter({has:page.locator('summary',{hasText:name})});if(await detail.getAttribute('open')===null)await detail.locator('summary').click();}
async function post(buttonName,expectedStatus){
  const responsePromise=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/theme-lab');
  await page.getByRole('button',{name:buttonName,exact:true}).focus();
  const [response]=await Promise.all([responsePromise,page.keyboard.press('Enter')]);
  if(expectedStatus>=400)report.expected_http_failures.push({status:expectedStatus,url:response.url(),observed_at:Date.now()});
  await page.waitForLoadState('networkidle');
  return response;
}
async function download(name){
  const responsePromise=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/theme-lab');
  await page.getByRole('button',{name:'Export acknowledged candidate ZIP',exact:true}).focus();
  const [artifact,response]=await Promise.all([page.waitForEvent('download'),responsePromise,page.keyboard.press('Enter')]);
  const path=join(root,name+'.zip');await artifact.saveAs(path);const bytes=readFileSync(path);
  report.downloads.push({name,path,suggested_filename:artifact.suggestedFilename(),bytes:bytes.length,sha256:hash(bytes),response_status:response.status(),response_headers:response.headers()});
  return {path,bytes,response};
}
async function specimenState(id,scheme,title){
  await page.locator('[name="specimen"]').selectOption(id);
  await page.locator('[name="scheme"]').selectOption(scheme);
  const response=await post('Preview candidate',200);
  const frame=page.frameLocator('iframe');await frame.locator('h1').waitFor();
  const state=await frame.locator('body').evaluate(body=>({font:getComputedStyle(body).fontFamily,theme:document.documentElement.dataset.theme,title:document.querySelector('h1')?.textContent?.trim(),scripts:document.scripts.length}));
  check(`native preview ${id} ${scheme}`,response.status()===200&&state.title===title&&state.theme===scheme,{status:response.status(),state});
  check(`computed candidate font ${id} ${scheme}`,state.font.includes('ui-serif')&&state.font.includes('Georgia'),state.font);
  check(`no authored script ${id} ${scheme}`,state.scripts===0,state.scripts);
  await shot(`${id}-${scheme}`);
}

try{
  browser=await chromium.launch({executablePath:config.executablePath,headless:true,args:['--disable-background-networking','--no-proxy-server']});
  report.browser=browser.version();
  context=await browser.newContext({viewport:{width:1440,height:1000},reducedMotion:'reduce',acceptDownloads:true});
  context.on('serviceworker',worker=>report.workers.push(worker.url()));
  check('fresh context has no service worker',context.serviceWorkers().length===0);
  context.setDefaultTimeout(7000);context.setDefaultNavigationTimeout(15000);
  await context.route('**/*',route=>{
    const request=route.request(),url=new URL(request.url());
    const allowed=url.origin===config.base&&((request.method()==='GET'&&['/theme-lab','/theme-lab/style.css','/theme-lab/preview','/static/app.css'].includes(url.pathname))||(request.method()==='POST'&&url.pathname==='/theme-lab'));
    if(!allowed){report.blocked.push({method:request.method(),url:request.url()});return route.abort('blockedbyclient');}
    report.requests.push({method:request.method(),path:url.pathname,origin:request.headers().origin,referer:request.headers().referer});return route.continue();
  });
  await context.routeWebSocket('**/*',ws=>{report.blocked.push({method:'WS',url:ws.url()});ws.close();});
  page=await context.newPage();
  page.on('pageerror',error=>report.errors.push(String(error)));
  page.on('console',message=>{if(['warning','error'].includes(message.type())){const location=message.location();report.console.push({type:message.type(),text:message.text(),url:location.url,line:location.lineNumber,column:location.columnNumber,observed_at:Date.now()});}});
  page.on('response',response=>{
    const headers=response.headers();
    if(headers['content-disposition']?.startsWith('attachment;')){report.responses.push({url:response.url(),status:response.status(),sha256:headers['x-content-sha256'],bytes:Number(headers['content-length']),headers,body_evidence:'saved download artifact'});return;}
    const task=(async()=>{const body=await bounded(response.body());report.responses.push({url:response.url(),status:response.status(),sha256:hash(body),bytes:body.length,headers});})();
    pending.add(task);task.catch(error=>report.errors.push(String(error))).finally(()=>pending.delete(task));
  });

  const initial=await page.goto(config.base+'/theme-lab',{waitUntil:'networkidle'});
  check('initial guide loads',initial.status()===200,initial.status());
  check('native policy and opaque sandbox',initial.headers()['referrer-policy']==='same-origin'&&await page.locator('iframe').getAttribute('sandbox')==='',{referrer:initial.headers()['referrer-policy'],sandbox:await page.locator('iframe').getAttribute('sandbox')});
  check('initial export disabled',await page.getByRole('button',{name:'Export acknowledged candidate ZIP',exact:true}).isDisabled());
  await shot('initial');

  await openDetail('Typography');await page.locator('[name="fonts.sans"]').selectOption('system-serif');
  await openDetail('Corner radii');await page.locator('[name="radius.md"]').fill('.75rem');
  await openDetail('Color roles');
  await page.locator('[name="colors.accent.light"]').fill('#ff7edb');
  const authored=await post('Preview candidate',200);
  check('native authored preview accepted',authored.status()===200&&/^[0-9a-f]{64}$/.test(await page.locator('[name="review_digest"]').inputValue()),authored.status());
  check('authored values retained',await page.locator('[name="fonts.sans"]').inputValue()==='system-serif'&&await page.locator('[name="radius.md"]').inputValue()==='.75rem'&&await page.locator('[name="colors.accent.light"]').inputValue()==='#ff7edb');

  for(const [id,title] of [['dashboard','Dashboard'],['components','Components'],['forms','Forms']])for(const scheme of ['light','dark'])await specimenState(id,scheme,title);
  await page.locator('[name="specimen"]').selectOption('components');await page.locator('[name="scheme"]').selectOption('light');await post('Preview candidate',200);
  const rendered=await page.frameLocator('iframe').locator('body').evaluate(body=>({radius:getComputedStyle(document.querySelector('.rounded-md')).borderRadius,accent:getComputedStyle(document.querySelector('.text-accent')).color,font:getComputedStyle(body).fontFamily}));
  check('rendered radius and accent use candidate tokens',rendered.radius==='12px'&&rendered.accent==='rgb(255, 126, 219)',rendered);

  await page.locator('select[name="viewport"]').selectOption('narrow');await post('Preview candidate',200);
  const iframeBox=await page.locator('iframe').boundingBox();
  check('narrow native iframe selected',await page.locator('select[name="viewport"]').inputValue()==='narrow'&&iframeBox&&iframeBox.width<=390,{value:await page.locator('select[name="viewport"]').inputValue(),box:iframeBox});await shot('narrow');

  const sourceView=await post('Show source',200);const sourceComputed=await page.frameLocator('iframe').locator('body').evaluate(body=>({font:getComputedStyle(body).fontFamily,accent:getComputedStyle(document.querySelector('.text-accent')).color}));
  check('show source uses baseline rendering',sourceView.status()===200&&!sourceComputed.font.includes('ui-serif')&&sourceComputed.accent==='rgb(249, 42, 173)',sourceComputed);
  const restored=await post('Preview candidate',200);check('candidate restored after source view',restored.status()===200&&/^[0-9a-f]{64}$/.test(await page.locator('[name="review_digest"]').inputValue()));

  const first=await download('candidate-first'),second=await download('candidate-second');
  check('two native downloads are byte identical',first.response.status()===200&&second.response.status()===200&&first.bytes.equals(second.bytes),report.downloads);
  check('download headers hash saved bytes',[first,second].every(item=>item.response.headers()['x-content-sha256']===hash(item.bytes)),report.downloads);
  const members=execFileSync('/usr/bin/unzip',['-Z1',first.path],{encoding:'utf8',maxBuffer:1<<20}).trim().split('\n').sort();
  const expectedMembers=['candidate.css','candidate.cue','diff.json','original.cue','receipt.json'];
  check('download has exact five members',JSON.stringify(members)===JSON.stringify(expectedMembers),members);
  const bundle=Object.fromEntries(expectedMembers.map(name=>[name,execFileSync('/usr/bin/unzip',['-p',first.path,name],{maxBuffer:1<<20})]));
  const receipt=JSON.parse(bundle['receipt.json']);const digest=hash(bundle['receipt.json']);
  const filePins=Object.entries(receipt.files??{}).every(([name,pin])=>bundle[name]&&pin.sha256===hash(bundle[name])&&pin.size===bundle[name].length);
  check('download preserves exact original source',bundle['original.cue'].equals(sourceBefore),{expected:hash(sourceBefore),actual:hash(bundle['original.cue'])});
  check('receipt binds exact exported members',receipt.schema_version===1&&receipt.scope==='experimental-app-owned-styleguide'&&receipt.exportable===true&&Object.keys(receipt.files??{}).length===4&&filePins&&digest===await page.locator('[name="review_digest"]').inputValue(),{receipt_sha256:digest,file_count:Object.keys(receipt.files??{}).length});

  await openDetail('Corner radii');await page.locator('[name="radius.md"]').fill('1rem');const stale=await post('Export acknowledged candidate ZIP',409);
  check('stale native export rejected',stale.status()===409&&await page.locator('[name="radius.md"]').inputValue()==='1rem'&&await page.locator('[name="review_digest"]').inputValue()==='',stale.status());
  await openDetail('Corner radii');await page.locator('[name="radius.md"]').fill('-1px');const invalid=await post('Preview candidate',422);
  check('invalid radius preserves edit and clears acknowledgement',invalid.status()===422&&await page.locator('[name="radius.md"]').inputValue()==='-1px'&&await page.locator('[name="review_digest"]').inputValue()==='',invalid.status());
  await openDetail('Corner radii');await page.locator('[name="radius.md"]').fill('.75rem');const corrected=await post('Preview candidate',200);
  check('invalid radius correction succeeds',corrected.status()===200&&/^[0-9a-f]{64}$/.test(await page.locator('[name="review_digest"]').inputValue()),corrected.status());

  await openDetail('Color roles');await page.locator('[name="colors.ink.light"]').fill('#241b2f');const contrast=await post('Preview candidate',200);
  check('contrast failure disables export',contrast.status()===200&&await page.getByRole('button',{name:'Export acknowledged candidate ZIP',exact:true}).isDisabled()&&await page.getByText('Needs review',{exact:true}).count()>0,contrast.status());
  const reset=await Promise.all([page.waitForResponse(r=>r.request().method()==='GET'&&new URL(r.url()).pathname==='/theme-lab'),page.getByRole('link',{name:'Reset to source',exact:true}).click()]);await page.waitForLoadState('networkidle');
  check('reset returns untouched source form',reset[0].status()===200&&await page.locator('[name="colors.ink.light"]').inputValue()===''&&await page.locator('[name="review_digest"]').inputValue()==='',reset[0].status());await shot('reset');
}catch(error){report.failure=String(error);}
finally{
  await drain();
  const html=report.responses.filter(response=>response.headers['content-type']?.startsWith('text/html'));
  check('document CSP refuses scripts and workers',html.length>0&&html.every(response=>response.headers['content-security-policy']?.includes("script-src 'none'")&&response.headers['content-security-policy']?.includes("default-src 'none'")),html.map(response=>({url:response.url,status:response.status,csp:response.headers['content-security-policy']})));
  const previews=html.filter(response=>response.status===200&&new URL(response.url).pathname==='/theme-lab/preview');
  const iframeSandbox=page&&!page.isClosed()?await page.locator('iframe').getAttribute('sandbox').catch(()=>null):null;
  check('successful preview response is hardened',previews.length>0&&iframeSandbox===''&&previews.every(response=>{const csp=response.headers['content-security-policy']??'';return response.headers['referrer-policy']==='no-referrer'&&csp.includes('sandbox')&&csp.includes("script-src 'none'")&&csp.includes("form-action 'none'")&&csp.includes("font-src 'none'")&&csp.includes("img-src 'none'");}),{count:previews.length,iframe_sandbox:iframeSandbox,policies:[...new Set(previews.map(response=>response.headers['content-security-policy']))]});
  const posts=report.requests.filter(request=>request.method==='POST'&&request.path==='/theme-lab');
  check('every native POST has exact Origin',posts.length>0&&posts.every(request=>request.origin===config.base),posts);
  check('no service worker appeared',report.workers.length===0&&context?.serviceWorkers().length===0,report.workers);
  if(browser)await browser.close();await drain();
  const expectedConsoleText=new Map([[409,'Failed to load resource: the server responded with a status of 409 (Conflict)'],[422,'Failed to load resource: the server responded with a status of 422 (Unprocessable Entity)']]);
  const expectedStatuses=report.expected_http_failures.map(failure=>failure.status).sort((a,b)=>a-b);
  const consoleAssociated=JSON.stringify(expectedStatuses)===JSON.stringify([409,422])&&report.console.length===report.expected_http_failures.length&&report.expected_http_failures.every(failure=>report.console.filter(message=>message.type==='error'&&message.text===expectedConsoleText.get(failure.status)&&message.url===failure.url&&Math.abs(message.observed_at-failure.observed_at)<10000).length===1);
  check('expected HTTP console errors are associated one to one',consoleAssociated,{console:report.console,failures:report.expected_http_failures});
  check('no out-of-scope request',report.blocked.length===0,report.blocked);
  check('no browser execution error',report.errors.length===0,report.errors);
  const sourceAfter=readFileSync(sourcePath);report.source.after_sha256=hash(sourceAfter);
  check('source remains byte unchanged',sourceAfter.equals(sourceBefore),report.source);
  const expected=[
    'fresh context has no service worker','initial guide loads','native policy and opaque sandbox','initial export disabled','native authored preview accepted','authored values retained',
    ...[['dashboard','Dashboard'],['components','Components'],['forms','Forms']].flatMap(([id])=>['light','dark'].flatMap(scheme=>[`native preview ${id} ${scheme}`,`computed candidate font ${id} ${scheme}`,`no authored script ${id} ${scheme}`])),
    'rendered radius and accent use candidate tokens','narrow native iframe selected','show source uses baseline rendering','candidate restored after source view','two native downloads are byte identical','download headers hash saved bytes','download has exact five members','download preserves exact original source','receipt binds exact exported members','stale native export rejected','invalid radius preserves edit and clears acknowledgement','invalid radius correction succeeds','contrast failure disables export','reset returns untouched source form','document CSP refuses scripts and workers','successful preview response is hardened','every native POST has exact Origin','no service worker appeared','expected HTTP console errors are associated one to one','no out-of-scope request','no browser execution error','source remains byte unchanged',
  ];
  check('exact assertion set completed',JSON.stringify(report.checks.map(item=>item.name).sort())===JSON.stringify(expected.sort()));
  report.finished=new Date().toISOString();report.ok=!report.failure&&report.checks.every(item=>item.pass);
  writeFileSync(join(root,'receipt.json'),JSON.stringify(report,null,2)+'\n',{flag:'wx'});
  console.log(JSON.stringify({root,ok:report.ok,checks:report.checks.length,failed:report.checks.filter(item=>!item.pass),failure:report.failure}));
  if(!report.ok)process.exitCode=1;
}
