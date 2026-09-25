// Exercise a real synthetic HTTP 503 and retry. Each lane owns one isolated
// compiled fixture process; no user backend, persistence, or live data is used.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {spawn} from 'node:child_process';
import {fileURLToPath,pathToFileURL} from 'node:url';

const [binary, packet, parent]=process.argv.slice(2);
const assert=(ok,message)=>{if(!ok)throw Error(message);};
for(const [file,directory]of [[binary,false],[packet,true],[parent,true]]){
  assert(file&&path.isAbsolute(file)&&path.normalize(file)===file,'Explicit clean paths required');
  const info=fs.lstatSync(file);assert(!info.isSymbolicLink()&&(directory?info.isDirectory():info.isFile()),'Regular binary and real private directories required');
}
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
const binaryHash=hash(fs.readFileSync(binary));
const pins=fs.readFileSync(path.join(path.dirname(binary),'SHA256SUMS'),'utf8').trim().split('\n').map(line=>line.split('  '));
assert(pins.filter(([digest,name])=>name===path.basename(binary)&&digest===binaryHash).length===1,'Recorded fixture binary identity required');
const referenceHash=hash(fs.readFileSync(path.join(packet,'reference.json')));
assert(referenceHash==='94d3ab4c145e33bf29af750918429833d833fff3f9ac8820e1314c7f2c67f53d','Selected private source packet required');
const modulePath=process.env.APPKIT_PLAYWRIGHT_MODULE;
assert(modulePath&&path.isAbsolute(modulePath),'Select approved isolated Playwright');
const pkg=JSON.parse(fs.readFileSync(path.join(path.dirname(modulePath),'package.json')));
assert(pkg.name==='playwright'&&pkg.version==='1.63.0','Pinned Playwright1.63.0 required');
const out=fs.mkdtempSync(path.join(parent,'p10-recovery-browser-'));fs.chmodSync(out,0o700);
const put=(name,body)=>{const b=Buffer.from(body);fs.writeFileSync(path.join(out,name),b,{flag:'wx',mode:0o600});return {name,sha256:hash(b),bytes:b.length};};
const plan={schema_version:1,fixture:'p10-explicit-first-validation-refusal',runner_sha256:hash(fs.readFileSync(fileURLToPath(import.meta.url))),binary_sha256:binaryHash,reference_sha256:referenceHash,
  chrome:'152.0.7977.84',playwright:'1.63.0',matrix:[390,1440].flatMap(width=>['light','dark'].flatMap(theme=>[false,true].map(enhanced=>({width,theme,enhanced})))),
  observation:'fonts.ready then fixed50ms Node barrier; no autofocus polling or latency claim',
  checks:['new fixture explicitly selects refusal mode','invalid422 does not consume backend refusal','valid503 retains all ten fields and textarea-effective counter','503 message focused, visible and identifies retry','Tab follows source order from status to Reset then Validate','draft editable without requests','native Enter retries successfully','reset restores independent initial snapshot','fixture exits cleanly'],
  exclusions:['network/proxy outage','real backend or saved data','screen reader/physical input certification','actual zoom','complete fidelity','performance budgets','consumer adoption'],verification:'not-verified'};
const planFile=put('plan.json',JSON.stringify(plan));console.log(JSON.stringify({out,plan_sha256:planFile.sha256,status:'recovery tasks starting'}));
const report={checks:[],errors:[],forbidden:[],requests:[],frames:[],processes:[]};
const check=(name,ok,actual)=>{report.checks.push({name,ok,actual});assert(ok,name);};
const delay=ms=>new Promise(resolve=>setTimeout(resolve,ms));
let browser,activeFixture;
const deadline=setTimeout(()=>{report.errors.push('120s collector deadline exceeded');process.exitCode=1;void browser?.close().catch(e=>report.errors.push(e.message));activeFixture?.child.kill('SIGTERM');},120_000);

async function startFixture(){
  const child=spawn(binary,[packet,'--refuse-first-validation'],{stdio:['ignore','pipe','pipe']});
  const record={pid:child.pid,mode:'refuse-first-validation',binary_sha256:binaryHash,exit:null,stderr:''};report.processes.push(record);
  const ended=new Promise(resolve=>child.once('exit',(code,signal)=>{record.exit={code,signal};resolve(record.exit);}));
  const state={child,ended,record};activeFixture=state;
  child.stderr.setEncoding('utf8');child.stderr.on('data',chunk=>{record.stderr=(record.stderr+chunk).slice(0,8192);});
  const ready=new Promise((resolve,reject)=>{
    let buffered='';child.stdout.setEncoding('utf8');
    child.once('error',reject);child.once('exit',()=>reject(Error('fixture exited before readiness')));
    child.stdout.on('data',chunk=>{buffered+=chunk;if(buffered.length>4096){reject(Error('fixture startup exceeded bound'));return;}const end=buffered.indexOf('\n');if(end<0)return;try{resolve(JSON.parse(buffered.slice(0,end)));}catch(e){reject(e);}});
  });
  let timer;
  try{
    const readyInfo=await Promise.race([ready,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('fixture readiness timeout')),10_000);})]);
    const origin=new URL(readyInfo.url);
    assert(origin.protocol==='http:'&&origin.hostname==='127.0.0.1'&&origin.port&&origin.origin===readyInfo.url&&readyInfo.pid===child.pid&&readyInfo.refuse_first_validation===true,'Explicit owned refusal fixture required');
    return {...state,base:origin.origin};
  }finally{clearTimeout(timer);}
}

async function stopFixture(state){
  if(!state)return;
  if(state.child.exitCode===null&&state.child.signalCode===null)state.child.kill('SIGTERM');
  let timer;
  try{await Promise.race([state.ended,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('fixture shutdown timeout')),5000);})]);}
  catch(e){state.child.kill('SIGKILL');throw e;}
  finally{clearTimeout(timer);if(activeFixture?.child===state.child)activeFixture=null;}
  check('owned fixture clean exit',state.record.exit?.code===0&&state.record.stderr==='',state.record);
}

async function snapshot(page){
  await page.evaluate(async()=>{await document.fonts.ready;});await delay(50);
  return page.locator('form[data-p10-settings]').evaluate(form=>{
    const fields=[...form.querySelectorAll('input:not([type=hidden]),textarea,select')];const status=form.querySelector('[role=status]');const box=status.getBoundingClientRect();
    return {values:Object.fromEntries(fields.map(c=>[c.name,c.type==='checkbox'?c.checked:c.value])),fields:fields.length,enabled:fields.every(c=>!c.disabled),status:status.textContent,active:document.activeElement?.id,activeValue:document.activeElement?.value,activeName:document.activeElement?.name,
      bioCount:form.querySelector('#p10-settings-bio-count')?.textContent,bioCodePoints:[...form.querySelector('[name=bio]').value].length,
      documentHasFocus:document.hasFocus(),visibilityState:document.visibilityState,autofocusIds:[...document.querySelectorAll('[autofocus]')].map(n=>n.id),
      visibleStatus:box.height>0&&box.top>=0&&box.bottom<=innerHeight,errors:[...form.querySelectorAll('[data-p10-error]:not([hidden])')].map(e=>e.getAttribute('data-p10-error')),enhanced:form.hasAttribute('data-p10-enhanced'),overflow:document.documentElement.scrollWidth>innerWidth};
  });
}
const same=(a,b)=>Object.keys(a).length===Object.keys(b).length&&Object.entries(a).every(([key,value])=>b[key]===value);

try{
  const {chromium}=await import(pathToFileURL(modulePath).href);
  browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
  check('pinned Chrome',browser.version()===plan.chrome,browser.version());
  for(const lane of plan.matrix){
    let fixture,context;
    const label=`${lane.width}-${lane.theme}-${lane.enhanced?'enhanced':'native'}`;
    try{
      fixture=await startFixture();
      context=await browser.newContext({viewport:{width:lane.width,height:1200},colorScheme:lane.theme,reducedMotion:'reduce',serviceWorkers:'block',javaScriptEnabled:lane.enhanced});
      await context.route('**/*',route=>{const req=route.request(),u=new URL(req.url());if(u.origin===fixture.base&&['GET','POST','HEAD'].includes(req.method()))return route.continue();report.forbidden.push(u.href);return route.abort();});
      const page=await context.newPage();page.setDefaultTimeout(10_000);page.setDefaultNavigationTimeout(10_000);
      page.on('pageerror',e=>report.errors.push(e.message));page.on('request',r=>report.requests.push({lane:label,path:new URL(r.url()).pathname,method:r.method()}));
      const route=`/${lane.theme}/${lane.enhanced?'enhanced/':''}settings`;
      const get=await page.goto(fixture.base+route);
      check(label+': initial GET',get.status()===200,get.status());
      const initial=await snapshot(page);
      check(label+': ten editable fields and selected mode',initial.fields===10&&initial.enabled&&initial.enhanced===lane.enhanced,initial);
      const post=async(status)=>{const [response]=await Promise.all([page.waitForNavigation({waitUntil:'load'}),page.getByRole('button',{name:'Validate changes',exact:true}).click()]);check(label+': native POST '+status,response.status()===status,response.status());};
      await page.locator('[name=bio]').fill('😀'.repeat(241));await post(422);
      let state=await snapshot(page);
      check(label+': invalid bio keeps focus and full draft',state.values.bio==='😀'.repeat(241)&&state.active==='p10-settings-bio'&&state.errors.includes('bio'),state);
      for(const [name,value]of Object.entries({name:'Recovery "quoted" <draft>',bio:'Retry 😀e\u0301\nAll fields retained',email:'retry@example.test',address:'2 Synthetic Way',city:'Sample city',postal_code:'54321'}))await page.locator(`[name="${name}"]`).fill(value);
      await page.locator('[name=email_is_public]').setChecked(!initial.values.email_is_public);
      await page.locator('select[name="country[code]"]').selectOption('MX');await page.locator('select[name=region]').selectOption('Jalisco');
      const currency=await page.locator('select[name=currency]').evaluate((select,current)=>[...select.options].find(option=>option.value&&option.value!==current).value,initial.values.currency);await page.locator('select[name=currency]').selectOption(currency);
      const expected=(await snapshot(page)).values;
      await post(503);state=await snapshot(page);
      check(label+': 503 retains all ten editable values',same(state.values,expected)&&state.enabled&&state.fields===10&&state.errors.length===0,state);
      check(label+': 503 counter agrees with textarea value',state.bioCount===`${state.bioCodePoints} / 240 Unicode code points`,state);
      check(label+': 503 focuses visible retry message',state.active==='p10-settings-status'&&state.visibleStatus&&/unavailable.*draft retained.*nothing was saved.*Try Validate changes again/.test(state.status)&&!state.overflow,state);
      report.frames.push({...lane,state:'503',screenshot:put(`503-${label}.png`,await page.locator('form[data-p10-settings]').screenshot())});
      await page.keyboard.press('Tab');state=await snapshot(page);
      check(label+': Tab reaches Reset after status',state.activeName==='_action'&&state.activeValue==='reset',state);
      await page.keyboard.press('Tab');state=await snapshot(page);
      check(label+': second Tab reaches Validate after Reset',state.activeName==='_action'&&state.activeValue==='validate',state);
      const beforeEdit=report.requests.length;
      await page.locator('[name=address]').fill('Corrected after refusal');expected.address='Corrected after refusal';state=await snapshot(page);
      check(label+': correction is local and editable',state.values.address===expected.address&&report.requests.length===beforeEdit&&(!lane.enhanced||state.status===''),state);
      const [retried]=await Promise.all([page.waitForNavigation({waitUntil:'load'}),page.locator('[name=address]').press('Enter')]);state=await snapshot(page);
      check(label+': native Enter retry succeeds without loss',retried.status()===200&&same(state.values,expected)&&state.errors.length===0&&state.status==='validated; nothing was saved',state);
      const reset=page.getByRole('button',{name:'Reset',exact:true});
      if(lane.enhanced){
        await reset.focus();await page.evaluate(()=>{window.__recoveryReset=document.activeElement;});const before=report.requests.length;await reset.click();state=await snapshot(page);
        check(label+': enhanced Reset has no request and retains node',report.requests.length===before&&await page.evaluate(()=>document.activeElement===window.__recoveryReset),state);
      }else{
        const [response]=await Promise.all([page.waitForNavigation({waitUntil:'load'}),reset.click()]);state=await snapshot(page);
        check(label+': native Reset response and focus',response.status()===200&&state.activeName==='_action'&&state.activeValue==='reset',state);
      }
      check(label+': Reset restores independent initial values',same(state.values,initial.values)&&state.errors.length===0&&state.status==='',state);
    }finally{
      try{if(context)await context.close();}finally{await stopFixture(fixture||activeFixture);}
    }
  }
  check('no exceptions or forbidden requests',report.errors.length===0&&report.forbidden.length===0);
}catch(e){report.errors.push(e.stack||e.message);process.exitCode=1;}
finally{
  clearTimeout(deadline);
  if(browser)try{await browser.close();}catch(e){report.errors.push(e.message);process.exitCode=1;}
  if(activeFixture)try{await stopFixture(activeFixture);}catch(e){report.errors.push(e.message);process.exitCode=1;}
  report.status=report.errors.length||report.forbidden.length?'failed':'scoped-recovery-tasks-passed';
  const result=put('report.json',JSON.stringify(report));console.log(JSON.stringify({out,status:report.status,checks:report.checks.length,errors:report.errors,report_sha256:result.sha256}));
}
