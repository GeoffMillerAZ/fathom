// Private synthetic browser proof for textarea transport/count agreement (P10).
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {fileURLToPath, pathToFileURL} from 'node:url';

const assert=(ok,message)=>{if(!ok)throw Error(message);};
const hash=bytes=>crypto.createHash('sha256').update(bytes).digest('hex');
const [originArg,binaryArg,parentArg]=process.argv.slice(2);
assert(originArg&&binaryArg&&parentArg,'Usage: newline-browser.mjs TARGET_ORIGIN TARGET_BINARY PRIVATE_PARENT');
const origin=new URL(originArg);
assert(origin.protocol==='http:'&&origin.hostname==='127.0.0.1'&&origin.port&&origin.origin===originArg,'Explicit loopback origin required');
assert(path.isAbsolute(binaryArg)&&path.isAbsolute(parentArg),'Absolute binary/private parent required');
const binary=fs.realpathSync(binaryArg),parent=fs.realpathSync(parentArg);
assert(parent===fs.realpathSync('data/design-assets'),'Selected private design-assets parent required');
assert(binary.startsWith(parent+path.sep)&&fs.statSync(binary).isFile(),'Private target binary required');
const modulePath=process.env.APPKIT_PLAYWRIGHT_MODULE;
assert(modulePath&&path.isAbsolute(modulePath),'Explicit isolated Playwright required');
const pkg=JSON.parse(fs.readFileSync(path.join(path.dirname(modulePath),'package.json')));
assert(pkg.name==='playwright'&&pkg.version==='1.63.0','Pinned Playwright required');
const out=fs.mkdtempSync(path.join(parent,'p10-newline-browser-'));fs.chmodSync(out,0o700);
const put=(name,bytes)=>{const b=Buffer.from(bytes);fs.writeFileSync(path.join(out,name),b,{flag:'wx',mode:0o600});return {name,sha256:hash(b),bytes:b.length};};
const cases=[
  {name:'240 with LF',value:'😀'.repeat(238)+'\nz',status:200},
  {name:'241 with LF',value:'😀'.repeat(239)+'\nz',status:422},
  {name:'leading and trailing LF',value:'\nfirst\nlast\n',status:200},
  {name:'three LF at limit',value:'e\u0301'.repeat(118)+'\n\n\nz',status:200},
];
const plan={schema_version:1,fixture:'p10-textarea-api-and-native-transport',runner_sha256:hash(fs.readFileSync(fileURLToPath(import.meta.url))),binary_sha256:hash(fs.readFileSync(binary)),chrome:'152.0.7977.84',playwright:pkg.version,
  matrix:[390,1440].flatMap(width=>['light','dark'].flatMap(theme=>[false,true].map(enhanced=>({width,theme,enhanced})))),cases:cases.map(c=>({...c,code_points:[...c.value].length})),
  contract:'Native URL-encoded POST uses CRLF. Count and limit use textarea API code points; raw submitted data is retained. No delay/focus repair.',
  exclusions:['source-native comparison','physical input','performance','certification'],verification:'not-verified'};
const planFile=put('plan.json',JSON.stringify(plan));console.log(JSON.stringify({out,plan_sha256:planFile.sha256}));
const report={plan_sha256:planFile.sha256,checks:[],errors:[],forbidden:[],requests:[],frames:[]};
const check=(name,ok,actual)=>report.checks.push({name,ok,actual});
let browser;
const deadline=setTimeout(()=>{report.errors.push('120s deadline exceeded');process.exitCode=1;void browser?.close().catch(()=>{});},120000);
try{
  const {chromium}=await import(pathToFileURL(modulePath).href);
  browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
  assert(browser.version()===plan.chrome,'Pinned Chrome required');
  for(const lane of plan.matrix){
    const label=`${lane.width}-${lane.theme}-${lane.enhanced?'enhanced':'native'}`;
    const context=await browser.newContext({viewport:{width:lane.width,height:1200},javaScriptEnabled:lane.enhanced,colorScheme:lane.theme,reducedMotion:'reduce',serviceWorkers:'block'});
    try{
      await context.route('**/*',route=>{const r=route.request();if(new URL(r.url()).origin===origin.origin&&['GET','POST','HEAD'].includes(r.method()))return route.continue();report.forbidden.push(r.url());return route.abort();});
      await context.routeWebSocket('**/*',ws=>{report.forbidden.push(ws.url());ws.close();});
      const page=await context.newPage();page.setDefaultTimeout(10000);page.setDefaultNavigationTimeout(10000);
      page.on('pageerror',e=>report.errors.push(e.message));
      page.on('request',r=>{if(r.method()==='POST')report.requests.push({lane:label,path:new URL(r.url()).pathname,body:r.postData()});});
      const route=`/${lane.theme}/${lane.enhanced?'enhanced/':''}settings`;
      for(const c of cases){
        assert((await page.goto(origin.origin+route)).status()===200,'Initial GET failed');
        if(lane.enhanced)await page.locator('form[data-p10-enhanced]').waitFor();
        await page.locator('[name=bio]').fill(c.value);
        const before=await page.locator('[name=bio]').inputValue();
        const [response]=await Promise.all([page.waitForNavigation({waitUntil:'load'}),page.getByRole('button',{name:'Validate changes',exact:true}).click()]);
        await page.evaluate(async()=>{await document.fonts.ready;});
        if(lane.enhanced)await page.locator('form[data-p10-enhanced]').waitFor();
        const state=await page.locator('form[data-p10-settings]').evaluate(form=>{
          const bio=form.querySelector('[name=bio]');const count=form.querySelector('#p10-settings-bio-count');
          return {value:bio.value,codePoints:[...bio.value].length,count:count?.textContent,invalid:bio.getAttribute('aria-invalid'),errors:[...form.querySelectorAll('[data-p10-error]:not([hidden])')].map(e=>e.getAttribute('data-p10-error'))};
        });
        const sent=new URLSearchParams(report.requests.at(-1)?.body).get('bio');
        check(label+': '+c.name+' transport',sent===c.value.replaceAll('\n','\r\n'),{sent,before});
        check(label+': '+c.name+' status',response.status()===c.status,{expected:c.status,actual:response.status()});
        check(label+': '+c.name+' retained',state.value===c.value&&before===c.value,state);
        check(label+': '+c.name+' counter',state.count===`${[...c.value].length} / 240 Unicode code points`,state);
        check(label+': '+c.name+' validation',state.invalid===(c.status===422?'true':'false')&&state.errors.includes('bio')===(c.status===422),state);
        if(c.name==='240 with LF')report.frames.push({lane:label,...put(`boundary-${label}.png`,await page.screenshot({fullPage:true}))});
      }
    }finally{await context.close();}
  }
}catch(e){report.errors.push(e.stack||e.message);}
finally{
  clearTimeout(deadline);if(browser)try{await browser.close();}catch(e){report.errors.push(e.message);}
  const failed=report.checks.filter(c=>!c.ok);
  report.status=failed.length||report.errors.length||report.forbidden.length?'failed':'scoped-newline-tasks-passed';
  if(report.status==='failed')process.exitCode=1;
  const result=put('report.json',JSON.stringify(report));console.log(JSON.stringify({out,status:report.status,checks:report.checks.length,failures:failed.length,errors:report.errors,report_sha256:result.sha256}));
}
