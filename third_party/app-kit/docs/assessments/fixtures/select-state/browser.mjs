import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {trackOwnedChild,terminateOwned} from '../bar-resolution-2026-09-22/browser-owned_process.mjs';
import {candidateExpected,evaluateState} from './oracle.mjs';

const [packet,variant]=process.argv.slice(2);
assert(packet?.startsWith('/')&&['baseline','candidate'].includes(variant),'usage: node browser.mjs ABSOLUTE_PACKET baseline|candidate');
const output=`${packet}/browser-${variant}`;
const helper=fileURLToPath(new URL('../bar-resolution-2026-09-22/browser-owned_process.mjs',import.meta.url));
const paths={
  driver:fileURLToPath(import.meta.url),helper,node:process.execPath,
  playwright:'/Users/geoff/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs',
  chrome:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  fixture:`${packet}/${variant}.json`,collector:`${packet}/${variant}-collector`,
  collectorSource:`${packet}/collector.go`,sourceArchive:`${packet}/${variant}.tar`,oracle:fileURLToPath(new URL('./oracle.mjs',import.meta.url)),
};
const lanes=[{id:'wide-light',width:1440,height:1000,theme:'light'},{id:'narrow-dark',width:390,height:844,theme:'dark'}];
const sha=b=>createHash('sha256').update(b).digest('hex');
const bound=async(p,ms,name)=>{let timer;try{return await Promise.race([p,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(`${name} timeout`)),ms);})]);}finally{clearTimeout(timer);}};
const snapshot=async()=>{const out={};for(const [name,path] of Object.entries(paths)){const bytes=await fs.readFile(path);out[name]={path,size:bytes.length,sha256:sha(bytes)};}return out;};
const savedBody='<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Schedule received</title></head><body><main><h1>Schedule received</h1><p>Received cadence: weekly.</p><p>No setting was saved.</p></main></body></html>';
const report={schema:'appkit-select-state-browser-v3',variant,startedAt:new Date().toISOString(),observations:[],checks:[],baselineFailures:[],errors:[],unexpectedRequests:[],cleanup:{},timedOut:false,limits:['Synthetic native HTML form only; no persistence, consumer adoption, model, network, or physical-input claim.','Option changes use Playwright selectOption, not keyboard choice: the separately retained native End trial failed. Tab, reset, validation and submit use native controls.']};
const check=(ok,name,detail={})=>{const item={ok:Boolean(ok),name,detail};report.checks.push(item);if(!ok)report.errors.push(item);};
const observeOracle=(actual,id,phase,name)=>{
  const result=evaluateState(variant,id,phase,actual);
  if(!result.ok)check(false,`${name} differs from precise ${variant} oracle`,{actual,expected:result.expected});
  else if(result.baselineDefect)report.baselineFailures.push({name,actual,historical:result.expected,candidate:result.candidate});
  else check(true,`${name} exact state`,{actual,expected:result.expected});
};
const state=page=>page.locator('#cadence').evaluate(select=>{
  const form=select.form,submitted=new FormData(form).get('cadence');
  return {value:select.value,submitted:typeof submitted==='string'?submitted:null,valid:form.checkValidity(),prompt:select.selectedOptions[0]?.label??''};
});
const activeIs=locator=>locator.evaluate(node=>node===document.activeElement);
const exactResponse=async(response,body,name)=>{
  assert(response,`${name} response missing`);
  await bound(response.finished(),5000,`${name} completion`);
  const bytes=Buffer.from(await bound(response.body(),5000,`${name} body`)),expected=Buffer.from(body);
  check(response.status()===200&&sha(bytes)===sha(expected),`${name} exact response`,{status:response.status(),bytes:bytes.length,sha256:sha(bytes)});
};

await fs.mkdir(output,{recursive:false,mode:0o700});
let server,browser,owned,runError;
const watchdog=setTimeout(()=>{report.timedOut=true;try{owned?.child.kill('SIGTERM');}catch{}},120000);
try{
  report.inputHashesBefore=await snapshot();
  assert.equal(paths.driver,fileURLToPath(import.meta.url));assert.equal(paths.helper,helper);
  assert.equal(process.execPath,'/opt/homebrew/Cellar/node/26.3.0/bin/node');assert.equal(process.version,'v26.3.0');
  const pwPackage=JSON.parse(await fs.readFile(new URL('package.json',pathToFileURL(paths.playwright)),'utf8'));
  assert.equal(pwPackage.version,'1.62.1');report.versions={node:process.version,playwright:pwPackage.version};
  const fixture=JSON.parse(await fs.readFile(paths.fixture,'utf8'));
  assert(Array.isArray(fixture.cases)&&fixture.cases.length===6&&typeof fixture.css==='string'&&fixture.css.length>0);
  assert.deepEqual(fixture.cases.map(x=>x.id),['missing-required','missing-custom','missing-optional','empty-duplicate','known','native-default']);
  for(const specimen of fixture.cases)assert.deepEqual(specimen.expected,candidateExpected(specimen.id));
  const {chromium}=await import(pathToFileURL(paths.playwright).href);
  server=await bound(chromium.launchServer({executablePath:paths.chrome,headless:true,chromiumSandbox:true,args:['--no-proxy-server','--disable-background-networking'],timeout:15000}),15000,'Chrome launch');
  owned=trackOwnedChild(server.process(),{name:'chrome'});
  browser=await bound(chromium.connect(server.wsEndpoint(),{timeout:10000}),10000,'Chrome connect');
  assert.equal(browser.version(),'153.0.8010.53');report.versions.chrome=browser.version();
  for(const lane of lanes)for(const specimen of fixture.cases){
    const id=`${variant}-${lane.id}-${specimen.id}`,requests=[],responses=[],trace=[];let context;
    try{
      context=await bound(browser.newContext({viewport:{width:lane.width,height:lane.height},colorScheme:lane.theme,javaScriptEnabled:false,serviceWorkers:'block'}),5000,`${id} context`);
      context.setDefaultTimeout(6000);context.setDefaultNavigationTimeout(10000);
      const fixtureURL=`http://127.0.0.1/select/${specimen.id}?theme=${lane.theme}`;
      const cssURL='http://127.0.0.1/static/app.css',savedURL='http://127.0.0.1/saved?cadence=weekly';
      await context.route('**/*',async route=>{
        const request=route.request(),url=request.url(),allowed=request.method()==='GET'&&[fixtureURL,cssURL,savedURL].includes(url);
        requests.push({url,method:request.method(),allowed});
        if(!allowed){report.unexpectedRequests.push({id,url,method:request.method()});return bound(route.abort('blockedbyclient'),3000,`${id} abort`);}
        const body=url===cssURL?fixture.css:url===savedURL?savedBody:(lane.theme==='light'?specimen.htmlLight:specimen.htmlDark);
        const contentType=url===cssURL?'text/css; charset=utf-8':'text/html; charset=utf-8';
        return bound(route.fulfill({status:200,contentType,headers:{'Cache-Control':'no-store'},body}),3000,`${id} fulfill`);
      });
      const page=await context.newPage();page.on('response',r=>responses.push({url:r.url(),status:r.status()}));
      const [cssResponse,documentResponse]=await Promise.all([bound(page.waitForResponse(r=>r.url()===cssURL),10000,`${id} CSS`),bound(page.goto(fixtureURL,{waitUntil:'load'}),10000,`${id} load`)]);
      const html=lane.theme==='light'?specimen.htmlLight:specimen.htmlDark;
      await exactResponse(documentResponse,html,`${id} document`);await exactResponse(cssResponse,fixture.css,`${id} CSS`);
      check(await page.locator('script').count()===0,`${id} no scripts`);
      check(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),`${id} no horizontal overflow`);
      const expected=specimen.expected;
      const initial=await state(page);trace.push({step:'initial',state:initial});observeOracle(initial,specimen.id,'initial',`${id} initial`);
      if(specimen.id==='missing-required')await bound(page.screenshot({path:`${output}/${id}-initial.png`,fullPage:true}),5000,`${id} screenshot`);
      await page.keyboard.press('Tab');check(await activeIs(page.locator('#cadence')),`${id} native Tab focuses select`);
      if(variant==='candidate'&&!expected.valid){
        const before=page.url();await page.getByRole('button',{name:'Save',exact:true}).click();
        check(page.url()===before&&await activeIs(page.locator('#cadence')),`${id} invalid Save stays and focuses select`,{before,after:page.url()});
      }
      await page.locator('#cadence').selectOption('weekly');
      check(await page.locator('#cadence').inputValue()==='weekly',`${id} controlled option change selects weekly`);
      await page.getByRole('button',{name:'Reset',exact:true}).click();
      const reset=await state(page);trace.push({step:'reset',state:reset});observeOracle(reset,specimen.id,'reset',`${id} reset`);
      await page.keyboard.press('Shift+Tab');await page.keyboard.press('Shift+Tab');
      check(await activeIs(page.locator('#cadence')),`${id} native return from Reset focuses select`);
      await page.locator('#cadence').selectOption('weekly');
      const weekly=await state(page);trace.push({step:'weekly',state:weekly});observeOracle(weekly,specimen.id,'weekly',`${id} recovered weekly state`);
      await page.keyboard.press('Tab');const save=page.getByRole('button',{name:'Save',exact:true});check(await activeIs(save),`${id} Tab reaches Save`);
      const navigation=page.waitForNavigation({waitUntil:'load'});await page.keyboard.press('Enter');const saved=await bound(navigation,10000,`${id} submit`);
      await exactResponse(saved,savedBody,`${id} saved`);check(page.url()===savedURL,`${id} native GET URL/value`,{url:page.url(),value:weekly.value});
      const storage=await page.evaluate(()=>({scripts:document.scripts.length,overflow:document.documentElement.scrollWidth>innerWidth+1,local:localStorage.length,session:sessionStorage.length,text:document.body.innerText}));
      check(storage.scripts===0&&!storage.overflow&&storage.local===0&&storage.session===0&&storage.text.includes('Received cadence: weekly.')&&storage.text.includes('No setting was saved.'),`${id} scriptless non-persistent receipt`,storage);
      check((await context.cookies()).length===0,`${id} no persistence cookies`);
      report.observations.push({id,case:specimen.id,lane:lane.id,expected,trace,requests,responses});
    }finally{if(context)await bound(context.close(),5000,`${id} context close`).catch(error=>report.errors.push({name:`${id} context close`,error:String(error)}));}
  }
}catch(error){runError=error;report.errors.push({name:'run error',error:String(error),stack:error?.stack});}
finally{
  try{if(browser)await bound(browser.close(),5000,'browser close');}catch(error){report.cleanup.browserCloseError=String(error);}
  try{if(server)await bound(server.close(),5000,'browser server close');}catch(error){report.cleanup.serverCloseError=String(error);}
  if(owned)try{report.cleanup.chrome={termination:await terminateOwned(owned,{graceMs:3000,killMs:2000,drainMs:2000}),lifecycle:owned.state};}catch(error){report.cleanup.chromeError=String(error);}
  clearTimeout(watchdog);
  try{report.inputHashesAfter=await snapshot();check(JSON.stringify(report.inputHashesBefore)===JSON.stringify(report.inputHashesAfter),'all frozen inputs unchanged');}catch(error){report.errors.push({name:'input hash recheck',error:String(error)});}
  const reproduced=variant==='baseline'&&report.baselineFailures.length===16,candidateClean=variant==='candidate'&&report.baselineFailures.length===0;
  const cleanupOK=report.cleanup.chrome?.termination?.ok&&!report.cleanup.browserCloseError&&!report.cleanup.serverCloseError&&!report.cleanup.chromeError;
  report.endedAt=new Date().toISOString();report.runError=runError?String(runError):null;report.summary={checks:report.checks.length,errors:report.errors.length,baselineFailures:report.baselineFailures.length,observations:report.observations.length,unexpectedRequests:report.unexpectedRequests.length};
  report.pass=!runError&&!report.timedOut&&report.errors.length===0&&report.unexpectedRequests.length===0&&report.observations.length===12&&cleanupOK&&(reproduced||candidateClean);
  report.taskAcceptance=variant==='candidate'&&report.pass;report.baselineMeaning=variant==='baseline'?'Exit 0 means exactly 16 expected failures were reproduced; it is not task acceptance.':null;
  await fs.writeFile(`${output}/report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx'});console.log(JSON.stringify({pass:report.pass,taskAcceptance:report.taskAcceptance,summary:report.summary}));process.exitCode=report.pass?0:1;
}
