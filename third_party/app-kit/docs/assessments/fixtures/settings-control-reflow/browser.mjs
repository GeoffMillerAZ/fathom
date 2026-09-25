import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {trackOwnedChild,terminateOwned} from '../bar-resolution-2026-09-22/browser-owned_process.mjs';

const [manifestPath,output]=process.argv.slice(2);
if(![manifestPath,output].every(x=>x?.startsWith('/')))throw Error('two absolute arguments required');
const manifest=JSON.parse(await fs.readFile(manifestPath,'utf8')),I=manifest.inputs;
const required=['baselineLight','baselineDark','candidateLight','candidateDark','cssPath','pwPath','chromePath','node','frameworkPath','driver','helper','fixture','contract'];
const themes=['light','dark'],lanes=[{name:'narrow',width:390,height:844},{name:'wide',width:1440,height:1000}];
const expected={
  fieldLabel:'FIELD_LABEL_'+'L'.repeat(128),fieldHelp:'FIELD_HELP_'+'H'.repeat(128),
  fieldError:'FIELD_ERROR_'+'E'.repeat(128),checkbox:'CHECKBOX_LABEL_'+'C'.repeat(128),
  radioLegend:'RADIO_LEGEND_'+'G'.repeat(128),radioOne:'RADIO_OPTION_ONE_'+'O'.repeat(128),
  radioTwo:'RADIO_OPTION_TWO_'+'P'.repeat(128),radioHelp:'RADIO_HELP_'+'R'.repeat(128),
};
const targets=[
  ['fieldLabel','[data-case="field-help"] label'],['fieldHelp','[data-case="field-help"] p'],
  ['fieldError','[data-case="field-error"] p'],['checkbox','[data-case="checkbox"] label'],
  ['radioLegend','[data-case="radio"] legend'],['radioOne','[data-case="radio"] label:nth-of-type(1)'],
  ['radioTwo','[data-case="radio"] label:nth-of-type(2)'],['radioHelp','[data-case="radio"] fieldset > p'],
];
const sha=b=>createHash('sha256').update(b).digest('hex');
const report={startedAt:new Date().toISOString(),identities:{},checks:[],cells:[],cleanup:{},unexpectedRequests:[],errors:[],timedOut:false};
const check=(ok,name,detail={})=>{const x={ok,name,detail};report.checks.push(x);if(!ok)report.errors.push(x);};
const bound=async(p,ms,name)=>{let timer;try{return await Promise.race([p,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(`${name} timeout`)),ms);})]);}finally{clearTimeout(timer);}};
async function identity(input){assert(input?.path?.startsWith('/')&&/^[0-9a-f]{64}$/.test(input.sha256));const b=await fs.readFile(input.path);assert.equal(sha(b),input.sha256,`${input.path} differs`);return {path:input.path,size:b.length,sha256:sha(b)};}
async function textGeometry(page,key,selector){return page.locator(selector).evaluate((el,{text,key})=>{
  const walker=document.createTreeWalker(el,NodeFilter.SHOW_TEXT);let node,match;
  while((node=walker.nextNode()))if(node.data.trim()===text){match=node;break;}
  if(!match)return {key,text,found:false,actual:el.textContent.trim()};
  const range=document.createRange();range.selectNodeContents(match);
  const section=el.closest('section'),column=section?.children[1],box=r=>({x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom});
  const bounds=box(range.getBoundingClientRect()),fragments=[...range.getClientRects()].map(box),columnBounds=box(column.getBoundingClientRect()),viewport={left:0,right:innerWidth};
  const contained=r=>r.x>=columnBounds.x-1&&r.right<=columnBounds.right+1&&r.x>=-1&&r.right<=innerWidth+1;
  return {key,text,found:true,bounds,fragments,columnBounds,viewport,contained:fragments.length>0&&fragments.every(contained),overflow:fragments.some(r=>!contained(r))};
},{text:expected[key],key});}
async function focusByTab(page,selector,max=24){for(let i=0;i<max;i++){await page.keyboard.press('Tab');if(await page.locator(selector).evaluate(e=>document.activeElement===e))return i+1;}return 0;}
async function focusState(locator){return locator.evaluate(e=>{const s=getComputedStyle(e),b=e.getBoundingClientRect(),column=e.closest('section')?.children[1]?.getBoundingClientRect();return {id:e.id,type:e.type,focused:document.activeElement===e,outlineWidth:s.outlineWidth,outlineStyle:s.outlineStyle,outlineColor:s.outlineColor,boxShadow:s.boxShadow,geometry:{x:b.x,y:b.y,width:b.width,height:b.height,right:b.right,bottom:b.bottom},column:{x:column.x,width:column.width,right:column.right},relativeX:b.x-column.x,rightGap:column.right-b.right,withinColumn:b.x>=column.x-1&&b.right<=column.right+1};});}

await fs.mkdir(output,{recursive:false,mode:0o700});
let browserServer,browser,browserOwned,runError;const normal={};
const watchdog=setTimeout(()=>{report.timedOut=true;try{browserOwned?.child.kill('SIGTERM');}catch{}},120000);
try{
  assert.deepEqual(Object.keys(I).sort(),required.slice().sort());
  for(const key of required)report.identities[key]=await identity(I[key]);
  assert.equal(fileURLToPath(import.meta.url),I.driver.path);assert.equal(fileURLToPath(new URL('../bar-resolution-2026-09-22/browser-owned_process.mjs',import.meta.url)),I.helper.path);assert.equal(process.execPath,I.node.path);assert.equal(process.version,manifest.node_version);
  const pkg=JSON.parse(await fs.readFile(new URL('package.json',pathToFileURL(I.pwPath.path)),'utf8'));assert.equal(pkg.version,manifest.pw_version);report.playwrightVersion=pkg.version;
  const {chromium}=await import(pathToFileURL(I.pwPath.path).href);
  browserServer=await chromium.launchServer({executablePath:I.chromePath.path,headless:true,args:['--no-proxy-server','--disable-background-networking']});
  browserOwned=trackOwnedChild(browserServer.process(),{name:'chrome'});browser=await chromium.connect(browserServer.wsEndpoint());assert.equal(browser.version(),manifest.chrome_version);report.chromeVersion=browser.version();
  const css=await fs.readFile(I.cssPath.path);
  for(const variant of ['baseline','candidate'])for(const lane of lanes)for(const theme of themes){
    const id=`${variant}-${lane.name}-${theme}`,input=I[variant+theme[0].toUpperCase()+theme.slice(1)],body=await fs.readFile(input.path);let context;
    try{
      context=await browser.newContext({viewport:{width:lane.width,height:lane.height},colorScheme:theme,javaScriptEnabled:false,serviceWorkers:'block'});context.setDefaultTimeout(8000);
      const requests=[];await context.route('**/*',route=>{const r=route.request(),u=new URL(r.url()),doc=u.origin==='http://127.0.0.1'&&u.pathname==='/'&&u.search===`?cell=${id}`,style=u.origin==='http://127.0.0.1'&&u.pathname==='/app.css'&&!u.search,allowed=r.method()==='GET'&&(doc||style);requests.push({url:r.url(),method:r.method(),allowed});if(!allowed){report.unexpectedRequests.push({id,url:r.url(),method:r.method()});return route.abort('blockedbyclient');}return route.fulfill({status:200,headers:{'content-type':style?'text/css; charset=utf-8':'text/html; charset=utf-8','content-security-policy':"default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",'x-content-type-options':'nosniff','cache-control':'no-store'},body:style?css:body});});
      const page=await context.newPage(),cssReady=page.waitForResponse(r=>new URL(r.url()).pathname==='/app.css');const doc=await page.goto(`http://127.0.0.1/?cell=${id}`,{waitUntil:'load'}),cssResponse=await cssReady;await Promise.all([doc.finished(),cssResponse.finished(),page.evaluate(()=>document.fonts.ready)]);const docBytes=Buffer.from(await doc.body()),cssBytes=Buffer.from(await cssResponse.body());
      check(doc.status()===200&&cssResponse.status()===200&&docBytes.length<=65536&&sha(docBytes)===input.sha256&&sha(cssBytes)===I.cssPath.sha256,`${id} completed exact document and CSS`,{document:doc.status(),css:cssResponse.status(),documentBytes:docBytes.length});
      check(await page.locator('script').count()===0&&await page.locator('html').getAttribute('data-theme')===theme,`${id} scriptless exact theme`);
      check(requests.length===2&&requests.every(x=>x.allowed),`${id} exact virtual routes`,requests);
      const geometry=[];for(const [key,selector]of targets)geometry.push(await textGeometry(page,key,selector));
      check(geometry.every(x=>x.found&&x.text===expected[x.key]),`${id} exact public strings`,geometry);
      if(variant==='baseline')check(geometry.every(x=>x.overflow),`${id} positive public-string overflow`,geometry);
      else check(geometry.every(x=>x.contained)&&await page.evaluate(()=>document.body.scrollWidth<=innerWidth+1),`${id} public strings contained by control column and viewport`,geometry);
      const normalInput=page.locator('#normal-control'),normalState=await focusState(normalInput);normal[`${variant}-${lane.name}-${theme}`]=normalState;
      const first=page.locator('#field-help-control'),checkbox=page.locator('#long-checkbox'),radioOne=page.locator('#long_radio-0'),radioTwo=page.locator('#long_radio-1');
      const unfocused={first:await focusState(first),checkbox:await focusState(checkbox),radio:await focusState(radioTwo)};
      const firstTabs=await focusByTab(page,'#field-help-control'),selectionBefore=await first.evaluate(e=>({value:e.value,start:e.selectionStart,end:e.selectionEnd}));await page.keyboard.press('ArrowRight');const selectionAfter=await first.evaluate(e=>({value:e.value,start:e.selectionStart,end:e.selectionEnd}));check(selectionBefore.value==='Help value'&&selectionAfter.value==='Help value'&&selectionAfter.start===selectionAfter.value.length&&selectionAfter.end===selectionAfter.value.length,`${id} native caret collapse before append`,{selectionBefore,selectionAfter});await page.keyboard.type(' typed');const firstFocus=await focusState(first);
      const checkboxTabs=await focusByTab(page,'#long-checkbox');await page.keyboard.press('Space');const checkboxFocus=await focusState(checkbox);
      const radioTabs=await focusByTab(page,'#long_radio-0');await page.keyboard.press('ArrowRight');const radioFocus=await focusState(radioTwo);
      const associations=await page.evaluate(()=>({method:document.querySelector('form').method,action:new URL(document.querySelector('form').action).pathname,helpFor:document.querySelector('[data-case="field-help"] label').htmlFor,helpDescribed:document.querySelector('#field-help-control').getAttribute('aria-describedby'),helpID:document.querySelector('[data-case="field-help"] p').id,errorFor:document.querySelector('[data-case="field-error"] label').htmlFor,errorDescribed:document.querySelector('#field-error-control').getAttribute('aria-describedby'),errorID:document.querySelector('[data-case="field-error"] p').id,checkboxNested:document.querySelector('[data-case="checkbox"] label').contains(document.querySelector('#long-checkbox')),radioLegend:document.querySelector('[data-case="radio"] fieldset').querySelector(':scope > legend')?.textContent.trim(),radioLabels:[...document.querySelectorAll('[data-case="radio"] label')].map(x=>x.control?.id)}));
      check(associations.method==='post'&&associations.action==='/not-implemented'&&associations.helpFor==='field-help-control'&&associations.helpDescribed===associations.helpID&&associations.helpID==='field-help-control-help'&&associations.errorFor==='field-error-control'&&associations.errorDescribed===associations.errorID&&associations.errorID==='field-error-control-error'&&associations.checkboxNested&&associations.radioLegend===expected.radioLegend&&JSON.stringify(associations.radioLabels)===JSON.stringify(['long_radio-0','long_radio-1']),`${id} exact form, label and ARIA associations`,associations);
      const formData=await page.locator('form').evaluate(f=>Object.fromEntries(new FormData(f)));
      const nativeGeometry=await page.locator('input,select,textarea').evaluateAll(nodes=>nodes.map(e=>{const b=e.getBoundingClientRect(),column=e.closest('section')?.children[1]?.getBoundingClientRect();return {id:e.id,type:e.type,x:b.x,y:b.y,width:b.width,height:b.height,right:b.right,bottom:b.bottom,column:{x:column.x,right:column.right},withinColumn:b.x>=column.x-1&&b.right<=column.right+1};}));
      const focused=[firstFocus,checkboxFocus,radioFocus],before=[unfocused.first,unfocused.checkbox,unfocused.radio],transparent=c=>c==='transparent'||/rgba\([^)]*,\s*0(?:\.0+)?\)$/.test(c),indicator=(x,i)=>{const old=before[i],outline=parseFloat(x.outlineWidth)>0&&x.outlineStyle!=='none'&&!transparent(x.outlineColor)&&(x.outlineWidth!==old.outlineWidth||x.outlineStyle!==old.outlineStyle||x.outlineColor!==old.outlineColor),shadow=x.boxShadow!=='none'&&x.boxShadow!==old.boxShadow;return outline||shadow;};check(firstTabs>0&&checkboxTabs>0&&radioTabs>0&&focused.every((x,i)=>x.focused&&indicator(x,i))&&nativeGeometry.every(x=>x.width>=12&&x.withinColumn),`${id} native input geometry and focus indicators`,{firstTabs,checkboxTabs,radioTabs,unfocused,focused,nativeGeometry});
      check(formData['field-help-control']==='Help value typed'&&formData.long_checkbox==='true'&&formData.long_radio==='second'&&formData['field-error-control']==='Error value'&&formData['normal-control']==='Normal Value',`${id} native keys and FormData`,formData);
      if(lane.name==='narrow'||variant==='candidate')await page.screenshot({path:`${output}/${id}.png`,fullPage:true});
      report.cells.push({id,documentSha256:sha(docBytes),cssSha256:sha(cssBytes),requests,geometry,normalGeometry:normalState,focus:{unfocused,first:firstFocus,checkbox:checkboxFocus,radio:radioFocus},nativeGeometry,associations,formData});
    }finally{if(context)await bound(context.close(),5000,`${id} context close`);}
  }
  for(const lane of lanes)for(const theme of themes){const a=normal[`baseline-${lane.name}-${theme}`],b=normal[`candidate-${lane.name}-${theme}`],same=['width','height'].every(k=>Math.abs(a.geometry[k]-b.geometry[k])<=1)&&Math.abs(a.relativeX-b.relativeX)<=1&&Math.abs(a.rightGap-b.rightGap)<=1;check(same&&a.withinColumn&&b.withinColumn,`${lane.name}-${theme} normal control geometry and column layout unchanged`,{baseline:a,candidate:b});}
}catch(error){runError=error;report.errors.push({name:'run error',detail:{message:String(error),stack:error?.stack}});}
finally{
  try{if(browser)await bound(browser.close(),5000,'browser close');}catch(e){report.cleanup.browserCloseError=String(e);}try{if(browserServer)await bound(browserServer.close(),5000,'browser server close');}catch(e){report.cleanup.browserServerCloseError=String(e);}if(browserOwned)report.cleanup.chrome={termination:await terminateOwned(browserOwned,{graceMs:3000,killMs:2000,drainMs:2000}),lifecycle:browserOwned.state};
  clearTimeout(watchdog);report.endedAt=new Date().toISOString();report.runError=runError?String(runError):null;const clean=report.cleanup.chrome?.termination?.ok&&!report.cleanup.browserCloseError&&!report.cleanup.browserServerCloseError;report.summary={checks:report.checks.length,failures:report.errors.length,cells:report.cells.length,unexpectedRequests:report.unexpectedRequests.length};report.pass=!runError&&!report.timedOut&&report.cells.length===8&&report.errors.length===0&&report.unexpectedRequests.length===0&&clean;await fs.writeFile(`${output}/report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx'});
}
process.exitCode=report.pass?0:1;
