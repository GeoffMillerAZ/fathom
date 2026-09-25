// Synthetic assessment control for Chrome Target attachment behavior. The caller owns
// the fresh Chrome/profile; this producer never launches it, navigates, or fetches data.
import {createHash} from 'node:crypto';
import {existsSync,readFileSync,readdirSync,realpathSync,statSync,writeFileSync} from 'node:fs';
import path from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {performance} from 'node:perf_hooks';
import {setTimeout as pause} from 'node:timers/promises';

const inputSchema='appkit-target-control-inputs-v1';
const playwrightEntrySHA='a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f';
const playwrightPackageSHA='8bc2ddebe8d9b051b9ecb87d62c6c63877e59d08617f32c3215681ca8bb70998';
const requiredNames=['probe','guard','node','browser_executable','playwright_entry','playwright_package'];
const required=name=>{const value=process.env[name];if(!value)throw Error(`Set ${name}; absent control evidence is not a skip.`);return value;};
const exactFile=(name,value)=>{if(typeof value!=='string'||!path.isAbsolute(value))throw Error(`${name} must be an absolute path`);const resolved=realpathSync(value);if(!statSync(resolved).isFile())throw Error(`${name} must select a file`);return resolved;};
const exactDir=(name,value)=>{if(!path.isAbsolute(value))throw Error(`${name} must be an absolute path`);const resolved=realpathSync(value);if(!statSync(resolved).isDirectory())throw Error(`${name} must select a directory`);return resolved;};
const hash=bytes=>createHash('sha256').update(bytes).digest('hex');
const fileIdentity=filename=>({path:filename,bytes:statSync(filename).size,sha256:hash(readFileSync(filename))});
const bounded=async(label,promise,ms=10000)=>{let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(`${label} exceeded ${ms}ms`)),ms);})]);}finally{clearTimeout(timer);}};
const exactWS=raw=>{const url=new URL(raw);if(url.protocol!=='ws:'||!['127.0.0.1','[::1]'].includes(url.hostname)||!url.port||url.username||url.password||url.search||url.hash||!/^\/devtools\/browser\/[^/]+$/.test(url.pathname)||raw!==url.href)throw Error('APPKIT_TARGET_CONTROL_WS must be an exact loopback browser endpoint');return url.href;};
const errorText=error=>error instanceof Error?`${error.name}: ${error.message}`:String(error);

const outputDir=exactDir('APPKIT_TARGET_CONTROL_OUTPUT_DIR',required('APPKIT_TARGET_CONTROL_OUTPUT_DIR'));
const receiptPath=path.join(outputDir,'target-control-v1-receipt.json');
if(existsSync(receiptPath))throw Error(`Refusing to replace ${receiptPath}`);
const variant=process.env.APPKIT_TARGET_CONTROL_VARIANT;
const report={
  schema:'appkit-target-control-v1',variant,identities:{},final_identities:{},protocol_events:[],observations:[500,1000,1500].map(delay_ms=>({delay_ms,status:'missing',reason:'not reached'})),
  guard_events:[],guard:null,endpoint:null,contexts:[],requests:[],downloads:[],page_errors:[],unexpected_pages:[],errors:[],
  cleanup:{producer_disconnected:false,context_closed:false,guard_disconnected:false},diagnostic_completed:false,cause_proven:false,complete:false,
  limits:['Synthetic empty/blank control only; no Gallery, server, navigation, download, or real data.','An unknown target remains paused and is never exempted or resumed.','A completed diagnostic does not prove the cause of prior unknown-target failures or certify full app quality.','Current prior evidence includes a later unknown target outside the guide context; guard failure alone does not prove browser UI.'],
};
const record=(scope,error)=>report.errors.push({at:new Date().toISOString(),scope,error:errorText(error)});
let browser,guard,ownContext,ownPage,selected={},snapshotPath;
const seenPages=new WeakSet(),started=performance.now();
const mark=label=>({label,at:new Date().toISOString(),monotonic_ms:performance.now()-started});
const watchPage=page=>{if(seenPages.has(page))return;seenPages.add(page);page.on('download',download=>report.downloads.push({at:new Date().toISOString(),url:page.url(),suggested_filename:download.suggestedFilename()}));page.on('pageerror',error=>report.page_errors.push({at:new Date().toISOString(),url:page.url(),error:errorText(error)}));};
const inventory=label=>{
  if(!browser)return;
  const item={label,at:new Date().toISOString(),contexts:[]};
  for(const context of browser.contexts()){
    const contextItem={owned:context===ownContext,pages:[]};
    for(const page of context.pages()){
      watchPage(page);const pageItem={owned:context===ownContext&&page===ownPage,url:page.url()};contextItem.pages.push(pageItem);
      if(!pageItem.owned||pageItem.url!=='about:blank')report.unexpected_pages.push({label,...pageItem});
    }
    item.contexts.push(contextItem);
  }
  report.contexts.push(item);
};

try{
  if(!['empty','blank'].includes(variant))throw Error('APPKIT_TARGET_CONTROL_VARIANT must be empty or blank');
  if((statSync(outputDir).mode&0o077)!==0)throw Error('APPKIT_TARGET_CONTROL_OUTPUT_DIR must be private (no group/other permissions)');
  const entries=readdirSync(outputDir);if(entries.length)throw Error(`APPKIT_TARGET_CONTROL_OUTPUT_DIR must be fresh and empty; found ${entries.length} entr${entries.length===1?'y':'ies'}`);
  const wsURL=exactWS(required('APPKIT_TARGET_CONTROL_WS'));
  report.endpoint=wsURL;
  snapshotPath=exactFile('APPKIT_TARGET_CONTROL_SNAPSHOT',required('APPKIT_TARGET_CONTROL_SNAPSHOT'));
  const snapshot=JSON.parse(readFileSync(snapshotPath,'utf8'));
  if(snapshot.schema!==inputSchema||snapshot.variant!==variant||!snapshot.files||typeof snapshot.node?.version!=='string')throw Error(`Snapshot must use ${inputSchema} with matching variant, files and node.version`);
  const actual={probe:fileURLToPath(import.meta.url),guard:fileURLToPath(new URL('./cdp_guard.mjs',import.meta.url)),node:process.execPath};
  for(const name of requiredNames){
    const frozen=snapshot.files[name];if(!frozen||typeof frozen.sha256!=='string')throw Error(`Snapshot lacks files.${name} path/sha256`);
    const filename=exactFile(`snapshot.files.${name}.path`,frozen.path);
    if(actual[name]&&filename!==realpathSync(actual[name]))throw Error(`Frozen ${name} path differs from the selected runtime`);
    const identity=fileIdentity(filename);if(identity.sha256!==frozen.sha256)throw Error(`Frozen ${name} digest differs`);
    selected[name]=filename;report.identities[name]=identity;
  }
  report.identities.snapshot=fileIdentity(snapshotPath);
  if(snapshot.node.version!==process.version)throw Error('Frozen Node version differs from the selected runtime');
  report.identities.node.version=process.version;
  if(report.identities.playwright_entry.sha256!==playwrightEntrySHA||report.identities.playwright_package.sha256!==playwrightPackageSHA)throw Error('Selected Playwright files differ from the approved pins');

  // All selected identities are confirmed before either local module import or Chrome connection.
  const [{connectGuard},{chromium}]=await bounded('module imports',Promise.all([import(pathToFileURL(selected.guard)),import(pathToFileURL(selected.playwright_entry))]));
  browser=await bounded('Playwright CDP connection',chromium.connectOverCDP(wsURL,{timeout:5000}));
  report.identities.browser_version=browser.version();inventory('after-connect');
  guard=await bounded('guard connection',connectGuard(wsURL,{allowedOrigin:'http://127.0.0.1:1',events:report.guard_events,protocolEvents:report.protocol_events,shaping:false}));
  let guardReady=true;
  try{await bounded('guard ready',guard.ready);}catch(error){guardReady=false;record('guard-ready',error);}
  report.guard=guard.coverage;

  let setupAt;
  if(variant==='empty')setupAt=performance.now();
  else if(guardReady){
    ownContext=await bounded('blank context creation',browser.newContext({viewport:{width:1440,height:1000},reducedMotion:'reduce',acceptDownloads:true}));
    ownContext.on('request',request=>report.requests.push({at:new Date().toISOString(),method:request.method(),url:request.url(),resource_type:request.resourceType()}));
    ownContext.on('page',watchPage);
    ownPage=await bounded('blank page creation',ownContext.newPage());watchPage(ownPage);
    if(ownPage.url()!=='about:blank')throw Error(`Blank control opened unexpected URL ${ownPage.url()}`);
    setupAt=performance.now();inventory('after-blank-setup');
  }
  if(setupAt!==undefined){
    for(let index=0;index<report.observations.length;index++){
      const observation=report.observations[index],wait=Math.max(0,observation.delay_ms-(performance.now()-setupAt));
      if(wait)await pause(wait);
      observation.before=mark(`enumeration-${index+1}-before`);observation.guard_failed_before=guard.coverage.failed;
      try{const result=await bounded(`Target.getTargets observation ${index+1}`,guard.send('Target.getTargets'));observation.target_infos=result.targetInfos??[];observation.status='recorded';observation.reason=null;}
      catch(error){observation.status='failed';observation.reason=errorText(error);record(`observation-${index+1}`,error);}
      observation.after=mark(`enumeration-${index+1}-after`);observation.guard_failed_after=guard.coverage.failed;inventory(`observation-${index+1}`);
      if(observation.status!=='recorded'||guard.coverage.failed){for(const missing of report.observations.slice(index+1))missing.reason=guard.coverage.failed?'guard failed closed after prior observation':'prior observation failed';break;}
    }
  }else for(const observation of report.observations)observation.reason='variant setup was not reached after guard readiness failure';
}catch(error){record('acquisition',error);}
finally{
  if(browser)inventory('before-cleanup');
  if(ownContext)await bounded('owned context close',ownContext.close()).then(()=>{report.cleanup.context_closed=true;}).catch(error=>record('context-close',error));
  else report.cleanup.context_closed=variant==='empty';
  if(browser)await bounded('Playwright disconnect',browser.close()).then(()=>{report.cleanup.producer_disconnected=true;}).catch(error=>record('producer-disconnect',error));
  if(guard){report.guard=guard.coverage;await bounded('guard disconnect',guard.close()).then(()=>{report.cleanup.guard_disconnected=true;}).catch(error=>record('guard-disconnect',error));}
  for(const [name,filename] of Object.entries({...selected,...(snapshotPath?{snapshot:snapshotPath}:{})}))try{const identity=fileIdentity(filename),initial=report.identities[name];report.final_identities[name]={...identity,matches_initial:Boolean(initial)&&identity.sha256===initial.sha256};if(!report.final_identities[name].matches_initial)record(`final-identity-${name}`,Error('identity differs or was not confirmed initially'));}catch(error){report.final_identities[name]={path:filename,matches_initial:false,read_error:errorText(error)};record(`final-identity-${name}`,error);}
  const observationsComplete=report.observations.every(item=>item.status==='recorded');
  report.diagnostic_completed=report.errors.length===0&&observationsComplete&&report.guard?.failed===false&&report.requests.length===0&&report.downloads.length===0&&report.page_errors.length===0&&report.unexpected_pages.length===0&&Object.values(report.cleanup).every(Boolean)&&Object.values(report.final_identities).every(item=>item.matches_initial);
  report.cause_proven=false;report.complete=false;
  writeFileSync(receiptPath,JSON.stringify(report,null,2)+'\n',{flag:'wx',mode:0o600});
  process.stdout.write(JSON.stringify({receipt:receiptPath,variant,diagnostic_completed:report.diagnostic_completed,cause_proven:false,complete:false,errors:report.errors.length})+'\n');
  if(!report.diagnostic_completed)process.exitCode=1;
}
