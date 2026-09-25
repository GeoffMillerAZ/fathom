// Local native-form tasks only. Fresh profiles; no scripts in the tested page.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {fileURLToPath,pathToFileURL} from 'node:url';

const [base,parent]=process.argv.slice(2);
const assert=(ok,message)=>{if(!ok)throw Error(message);};
const origin=new URL(base);
assert(origin.origin===base&&origin.protocol==='http:'&&origin.hostname==='127.0.0.1'&&origin.port,'Explicit loopback origin required');
assert(path.isAbsolute(parent)&&fs.lstatSync(parent).isDirectory()&&!fs.lstatSync(parent).isSymbolicLink(),'Explicit existing private output parent required');
const modulePath=process.env.APPKIT_PLAYWRIGHT_MODULE;
assert(modulePath&&path.isAbsolute(modulePath),'Select approved isolated Playwright');
const pkg=JSON.parse(fs.readFileSync(path.join(path.dirname(modulePath),'package.json')));
assert(pkg.name==='playwright'&&pkg.version==='1.63.0','Pinned Playwright 1.63.0 required');
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
const out=fs.mkdtempSync(path.join(parent,'p10-native-browser-'));fs.chmodSync(out,0o700);
const put=(name,body)=>{const b=Buffer.from(body);fs.writeFileSync(path.join(out,name),b,{flag:'wx',mode:0o600});return {name,sha256:hash(b),bytes:b.length};};
const plan={schema_version:1,fixture:'complete-p10-native-http',runner_sha256:hash(fs.readFileSync(fileURLToPath(import.meta.url))),chrome:'152.0.7977.84',playwright:'1.63.0',
  matrix:[390,1440].flatMap(width=>['light','dark'].map(theme=>({width,theme}))),javascript:false,minimum_control_height:44,observation:'font readiness plus fixed 50ms external wait before every snapshot; never polls or moves focus; not a latency claim',
  checks:['complete five-section form','native named controls','all canonical region groups available','no overflow','44px controls/checkbox label','valid submit','Enter validates and never resets','retained 241-codepoint draft','bio-first focus','region membership refusal','correction','native reset/focus'],
  exclusions:['enhanced local feedback','physical keyboard/pointer certification','actual browser zoom','payload/performance budgets','backend refusal UI','complete visual fidelity','real consumer'],verification:'not-verified'};
const planFile=put('plan.json',JSON.stringify(plan));
console.log(JSON.stringify({out,plan_sha256:planFile.sha256,status:'native tasks starting'}));
const report={checks:[],errors:[],forbidden:[],frames:[],responses:[]};
const check=(name,ok,actual)=>{report.checks.push({name,ok,actual});assert(ok,name);};
const {chromium}=await import(pathToFileURL(modulePath).href);
let browser;
try{
  browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
  check('pinned Chrome',browser.version()===plan.chrome,browser.version());
  for(const lane of plan.matrix){
    const label=`${lane.width}-${lane.theme}`;
    const context=await browser.newContext({viewport:{width:lane.width,height:1200},colorScheme:lane.theme,reducedMotion:'reduce',serviceWorkers:'block',javaScriptEnabled:false});
    try{
      await context.route('**/*',route=>{const req=route.request(),u=new URL(req.url());if(u.origin===base&&['GET','POST','HEAD'].includes(req.method()))return route.continue();report.forbidden.push(u.href);return route.abort();});
      const page=await context.newPage();page.on('pageerror',e=>report.errors.push(e.message));
      page.on('response',r=>report.responses.push({url:new URL(r.url()).pathname,status:r.status(),method:r.request().method()}));
      const response=await page.goto(base+`/${lane.theme}/settings`);
      check(label+': initial response',response.status()===200,response.status());
      await page.evaluate(async()=>{await document.fonts.ready;});
      const snapshot=async()=>{await page.evaluate(async()=>{await document.fonts.ready;});await new Promise(resolve=>setTimeout(resolve,50));return page.locator('form[data-p10-settings]').evaluate(f=>({headings:[...f.querySelectorAll('h1,h2,h3')].map(e=>e.textContent),
        values:Object.fromEntries(new FormData(f)),fields:[...f.querySelectorAll('input:not([type=hidden]),textarea,select')].map(e=>({name:e.name,type:e.type,id:e.id,invalid:e.getAttribute('aria-invalid'),height:e.getBoundingClientRect().height})),
        buttons:[...f.querySelectorAll('button')].map(e=>({text:e.textContent,height:e.getBoundingClientRect().height})),
        regionGroups:f.querySelectorAll('select[name=region] optgroup').length,active:document.activeElement?.id||'',activeName:document.activeElement?.name||'',activeValue:document.activeElement?.value||'',
        labelHeight:f.querySelector('[data-p10-checkbox-label]').getBoundingClientRect().height,scroll:document.documentElement.scrollWidth,width:innerWidth,
        errors:[...f.querySelectorAll('[data-p10-error]:not([hidden])')].map(e=>e.getAttribute('data-p10-error')),status:f.querySelector('[role=status]')?.textContent||''}));};
      const initial=await snapshot();
      check(label+': complete sections',initial.headings.join('|')==='Settings|Organization Name|Organization Bio|Organization Email|Address|Currency');
      check(label+': ten native fields',initial.fields.length===10&&new Set(initial.fields.map(e=>e.name)).size===10,initial.fields);
      check(label+': complete native region groups',initial.regionGroups===3,initial.regionGroups);
      check(label+': no overflow',initial.scroll<=initial.width,initial);
      check(label+': 44px controls',initial.fields.filter(e=>e.type!=='checkbox').every(e=>e.height>=43.99)&&initial.buttons.every(e=>e.height>=43.99)&&initial.labelHeight>=43.99,initial);
      const shot=put(`default-${label}.png`,await page.locator('form[data-p10-settings]').screenshot());report.frames.push({...lane,state:'default',screenshot:shot});
      const post=async(button,status)=>{const [r]=await Promise.all([page.waitForNavigation({waitUntil:'load'}),page.getByRole('button',{name:button,exact:true}).click()]);check(label+': '+button+' HTTP '+status,r?.status()===status,r?.status());};
      await post('Validate changes',200);
      let state=await snapshot();check(label+': valid submit no persistence claim',/validat/i.test(state.status)&&/nothing.*sav/i.test(state.status)&&state.errors.length===0,state.status);
      await page.locator('input[name=name]').fill('Keyboard-enter example');
      const [enterResponse]=await Promise.all([page.waitForNavigation({waitUntil:'load'}),page.locator('input[name=name]').press('Enter')]);state=await snapshot();
      check(label+': Enter validates without discarding draft',enterResponse?.status()===200&&new URL(page.url()).pathname===`/${lane.theme}/settings`&&state.values.name==='Keyboard-enter example'&&/validat/i.test(state.status),state);
      const draft='😀'.repeat(241);
      await page.locator('[name=bio]').fill(draft);
      await page.locator('select[name="country[code]"]').selectOption('MX');
      await post('Validate changes',422);
      state=await snapshot();
      check(label+': over-limit draft retained',state.values.bio===draft&&state.values['country[code]']==='MX'&&state.values.region===initial.values.region);
      check(label+': bio first invalid focus',state.active.endsWith('-bio')&&state.errors.includes('bio')&&state.errors.includes('region')&&state.status==='',state);
      const errorShot=put(`errors-${label}.png`,await page.locator('form[data-p10-settings]').screenshot());report.frames.push({...lane,state:'errors',screenshot:errorShot});
      await page.locator('[name=bio]').fill('Corrected local example');
      await post('Validate changes',422);state=await snapshot();
      check(label+': region correction focus',state.active.endsWith('-region')&&!state.errors.includes('bio')&&state.errors.includes('region'),state);
      await page.locator('select[name=region]').selectOption('Jalisco');
      await post('Validate changes',200);state=await snapshot();
      check(label+': corrected values retained',state.values.bio==='Corrected local example'&&state.values.region==='Jalisco'&&state.values['country[code]']==='MX'&&state.errors.length===0,state);
      await post('Reset',200);state=await snapshot();
      check(label+': full native reset',JSON.stringify(state.values)===JSON.stringify(initial.values)&&state.errors.length===0&&state.status==='',state);
      check(label+': native reset restores focus',state.activeName==='_action'&&state.activeValue==='reset',state);
    }finally{await context.close();}
  }
  check('no page errors or external requests',report.errors.length===0&&report.forbidden.length===0);
}catch(e){report.errors.push(e.stack||e.message);process.exitCode=1;}
finally{
  if(browser)try{await browser.close();}catch(e){report.errors.push(e.message);process.exitCode=1;}
  report.status=report.errors.length||report.forbidden.length?'failed':'scoped-native-tasks-passed';
  const result=put('report.json',JSON.stringify(report));
  console.log(JSON.stringify({out,status:report.status,checks:report.checks.length,errors:report.errors,report_sha256:result.sha256}));
}
