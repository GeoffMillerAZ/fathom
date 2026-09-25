// Prospective theme-guide-browser-v1 acquisition. This probe uses only an
// explicitly frozen local server, source, browser and approved Playwright tree.
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {createReadStream, existsSync, readFileSync, realpathSync, statSync, writeFileSync} from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {performance} from 'node:perf_hooks';
import {configuration} from '../../gallery/browser/config.mjs';

const schema='theme-guide-browser-acquisition-v1';
const inputSchema='theme-guide-browser-inputs-v1';
const playwrightEntrySHA='a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f';
const playwrightPackageSHA='8bc2ddebe8d9b051b9ecb87d62c6c63877e59d08617f32c3215681ca8bb70998';
const exactMembers=['candidate.css','candidate.cue','diff.json','original.cue','receipt.json'];
const specimens=[['dashboard','Dashboard'],['components','Components'],['forms','Forms']];
const schemes=['light','dark'];
const viewports=[{name:'wide',width:1440,height:1000,form:'wide'},{name:'narrow',width:390,height:844,form:'narrow'}];
const samples=5;
const acquisitionStart=performance.now();
const now=()=>performance.now()-acquisitionStart;
const hash=bytes=>createHash('sha256').update(bytes).digest('hex');
const fileHash=filename=>new Promise((resolve,reject)=>{const h=createHash('sha256'),stream=createReadStream(filename);stream.on('data',chunk=>h.update(chunk));stream.on('error',reject);stream.on('end',()=>resolve(h.digest('hex')));});
const required=name=>{const value=process.env[name];if(!value)throw new Error(`Set ${name}; absent acquisition identity is not a skip.`);return value;};
const exactPath=(name,value)=>{if(!path.isAbsolute(value))throw new Error(`${name} must be an absolute path.`);const resolved=realpathSync(value);if(!statSync(resolved).isFile())throw new Error(`${name} must select a regular file.`);return resolved;};
const outputPath=value=>{if(!path.isAbsolute(value))throw new Error('APPKIT_THEME_PERF_OUTPUT_DIR must be absolute.');const resolved=realpathSync(value);if(!statSync(resolved).isDirectory())throw new Error('APPKIT_THEME_PERF_OUTPUT_DIR must be an existing directory.');return resolved;};

const config=configuration({...process.env,APPKIT_SHELL_BROWSER_ORIGIN:required('APPKIT_THEME_BROWSER_ORIGIN')});
const outputDir=outputPath(required('APPKIT_THEME_PERF_OUTPUT_DIR'));
const receiptPath=path.join(outputDir,'theme-guide-browser-v1-receipt.json');
if(existsSync(receiptPath))throw new Error(`Refusing to replace ${receiptPath}`);
const retainBody=bytes=>{const sha256=hash(bytes),filename=path.join(outputDir,`body-${sha256}.bin`);if(existsSync(filename)){const retained=readFileSync(filename);if(!retained.equals(bytes))throw new Error(`Retained body identity collision at ${filename}`);}else writeFileSync(filename,bytes,{flag:'wx',mode:0o600});return {path:filename,sha256,bytes:bytes.length};};
const selected={
  probe:fileURLToPath(import.meta.url),
  config:fileURLToPath(new URL('../../gallery/browser/config.mjs',import.meta.url)),
  contract:fileURLToPath(new URL('../../../docs/assessments/fixtures/styleguide-performance/browser-contract.md',import.meta.url)),
  source:exactPath('APPKIT_THEME_SOURCE',required('APPKIT_THEME_SOURCE')),
  server:exactPath('APPKIT_THEME_SERVER_BINARY',required('APPKIT_THEME_SERVER_BINARY')),
  playwright_entry:exactPath('APPKIT_PLAYWRIGHT_MODULE',fileURLToPath(config.moduleURL)),
  playwright_package:exactPath('Playwright package.json',fileURLToPath(new URL('./package.json',config.moduleURL))),
  browser_executable:exactPath('APPKIT_BROWSER_EXECUTABLE',config.executablePath),
  node:exactPath('Node executable',process.execPath),
};
selected.probe=realpathSync(selected.probe);selected.config=realpathSync(selected.config);selected.contract=realpathSync(selected.contract);
const snapshotPath=exactPath('APPKIT_THEME_PERF_SNAPSHOT',required('APPKIT_THEME_PERF_SNAPSHOT'));
const snapshotBytes=readFileSync(snapshotPath);
const snapshot=JSON.parse(snapshotBytes);
if(snapshot.schema!==inputSchema||!snapshot.files||!snapshot.server_build_receipt)throw new Error(`Snapshot must use ${inputSchema} and declare files plus server_build_receipt.`);
const identities={};
for(const [name,filename] of Object.entries(selected)){
  const bytes=statSync(filename).size,sha256=await fileHash(filename),frozen=snapshot.files[name];
  if(!frozen||exactPath(`snapshot.files.${name}.path`,frozen.path)!==filename||frozen.sha256!==sha256)throw new Error(`Frozen ${name} identity differs from the selected file.`);
  identities[name]={path:filename,bytes,sha256};
}
const buildReceiptPath=exactPath('snapshot.server_build_receipt.path',snapshot.server_build_receipt.path);
const buildReceiptSHA=await fileHash(buildReceiptPath);
if(snapshot.server_build_receipt.sha256!==buildReceiptSHA)throw new Error('Frozen server build receipt identity differs.');
identities.server_build_receipt={path:buildReceiptPath,bytes:statSync(buildReceiptPath).size,sha256:buildReceiptSHA};
const buildReceipt=JSON.parse(readFileSync(buildReceiptPath));
if(!buildReceipt.go||typeof buildReceipt.go.version!=='string'||typeof buildReceipt.go.path!=='string'||!/^[0-9a-f]{64}$/.test(buildReceipt.go.sha256??''))throw new Error('Frozen server build receipt must declare go {path, version, sha256}.');
const goPath=exactPath('server_build_receipt.go.path',buildReceipt.go.path),goSHA=await fileHash(goPath);
if(goSHA!==buildReceipt.go.sha256)throw new Error('Frozen server build receipt Go identity differs from its exact tool path.');
identities.go={path:goPath,bytes:statSync(goPath).size,sha256:goSHA,version:buildReceipt.go.version};
identities.snapshot={path:snapshotPath,bytes:snapshotBytes.length,sha256:hash(snapshotBytes)};
if(identities.playwright_entry.sha256!==playwrightEntrySHA||identities.playwright_package.sha256!==playwrightPackageSHA)throw new Error('Selected approved Playwright identity differs.');
const sourceBefore=readFileSync(selected.source);
const expectedServerIdentity=`local-gallery-binary:sha256:${identities.server.sha256}`;
const {chromium}=await import(config.moduleURL);

const report={
  schema,contract:'theme-guide-browser-v1',started:new Date().toISOString(),origin:config.base,variant:config.variant,
  identities:{...identities,expected_server_identity:expectedServerIdentity,node_version:process.version,os:{platform:os.platform(),release:os.release(),arch:os.arch()},hardware:{cpu_model:os.cpus()[0]?.model??'',logical_cpus:os.cpus().length,total_memory_bytes:os.totalmem()}},
  acquisition:{samples,viewports:viewports.map(({name,width,height})=>({name,width,height})),specimens:specimens.map(([id,title])=>({id,title})),schemes,ordinary_expected_tasks:samples*viewports.length*specimens.length*schemes.length*2,degraded:{status:'incomplete',reason:'Omitted: a configured CDP command alone cannot prove that 100ms latency and both throughput limits covered the top document, iframe, assets and native upload.'},cache:{status:'disabled-by-routing',warm_cache_claim:false}},
  tasks:[],cleanup:{contexts_created:0,contexts_closed:0,browser_closed:false,server_owned:false},
  source:{path:selected.source,before_sha256:hash(sourceBefore),before_bytes:sourceBefore.length},
  limits:['Synthetic Gallery task, not consumer adoption, visual/accessibility/font certification or a population p95.','Same-context repeat is not a warm-cache lane; Playwright routing disables HTTP cache.','Memory and cancellation are separate incomplete contract lanes.','Decoded body sizes and Playwright transfer-size observations are reported separately.'],
  errors:[],
};

const expectedClosure={
  navigation:[['GET','/theme-lab'],['GET','/theme-lab/style.css'],['GET','/theme-lab/preview'],['GET','/static/app.css']],
  preview:[['POST','/theme-lab'],['GET','/theme-lab/style.css'],['GET','/theme-lab/preview'],['GET','/static/app.css']],
  export:[['POST','/theme-lab']],
  reset:[['GET','/theme-lab'],['GET','/theme-lab/style.css'],['GET','/theme-lab/preview'],['GET','/static/app.css']],
};
const responseKey=item=>`${item.method} ${item.path}`;
const closureFor=(task,phase)=>{
  const want=expectedClosure[phase].map(([method,p])=>`${method} ${p}`).sort();
  const requests=task.requests.filter(item=>item.phase===phase).map(responseKey).sort();
  const responses=task.responses.filter(item=>item.phase===phase).map(responseKey).sort();
  return {expected:want,requests,responses,exact:JSON.stringify(want)===JSON.stringify(requests)&&JSON.stringify(want)===JSON.stringify(responses)};
};
const accountingFor=(task,phase)=>{
  const responses=task.responses.filter(item=>item.phase===phase),successful=responses.filter(item=>item.ok&&!item.read_error&&Number.isFinite(item.response_body_bytes));
  const transferKnown=responses.every(item=>item.sizes&&Number.isFinite(item.sizes.responseBodySize));
  return {responses:responses.length,successful_responses:successful.length,complete:responses.length>0&&successful.length===responses.length,decoded_body_bytes:successful.reduce((sum,item)=>sum+item.response_body_bytes,0),transfer_body_bytes:transferKnown?responses.reduce((sum,item)=>sum+item.sizes.responseBodySize,0):null,transfer_observation_complete:transferKnown};
};
const newTask=(lane,sample,viewport,specimen,scheme)=>({lane,sample,viewport:{name:viewport.name,width:viewport.width,height:viewport.height,form:viewport.form},specimen,scheme,cache_disabled:true,timings_ms:{},postconditions:{},requests:[],responses:[],request_failures:[],expected_download_aborts:[],blocked:[],errors:[],console:[],workers:[],closures:{},ok:false});
const normal=value=>String(value??'').trim().replace(/\s+/g,' ').toLowerCase();
const validDigest=value=>/^[0-9a-f]{64}$/.test(value);
const timed=start=>Number((performance.now()-start).toFixed(3));
const bounded=async promise=>{let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error('Response body deadline exceeded')),10000);})]);}finally{clearTimeout(timer);}};

let browser;
try{
  browser=await chromium.launch({executablePath:selected.browser_executable,headless:true,downloadsPath:outputDir,args:['--disable-background-networking','--no-proxy-server']});
  report.identities.browser_version=browser.version();
  for(let sample=1;sample<=samples;sample++)for(const viewport of viewports)for(const [specimen,title] of specimens)for(const scheme of schemes){
    let context,page,currentTask=null,currentPhase='setup',sequence=0;
    const pending=new Set(),requestMeta=new WeakMap(),responseMeta=new WeakMap();
    const assign=entry=>{pending.add(entry);entry.promise.finally(()=>pending.delete(entry));};
    const drain=async(task,phase)=>{for(;;){const chosen=[...pending].filter(item=>item.task===task&&item.phase===phase).map(item=>item.promise);if(!chosen.length)return;await Promise.allSettled(chosen);}};
    try{
      context=await browser.newContext({viewport:{width:viewport.width,height:viewport.height},reducedMotion:'reduce',acceptDownloads:true});report.cleanup.contexts_created++;
      context.setDefaultTimeout(7000);context.setDefaultNavigationTimeout(15000);
      context.on('serviceworker',worker=>{if(currentTask)currentTask.workers.push({url:worker.url(),observed_ms:now()});});
      await context.route('**/*',route=>{
        const request=route.request(),url=new URL(request.url());
        const allowed=url.origin===config.base&&((request.method()==='GET'&&['/theme-lab','/theme-lab/style.css','/theme-lab/preview','/static/app.css'].includes(url.pathname))||(request.method()==='POST'&&url.pathname==='/theme-lab'));
        if(!allowed){currentTask?.blocked.push({phase:currentPhase,method:request.method(),url:request.url(),observed_ms:now()});return route.abort('blockedbyclient');}
        return route.continue();
      });
      await context.routeWebSocket('**/*',ws=>{currentTask?.blocked.push({phase:currentPhase,method:'WS',url:ws.url(),observed_ms:now()});ws.close();});
      context.on('request',request=>{
        if(!currentTask)return;const url=new URL(request.url()),body=request.postDataBuffer();
        const meta={task:currentTask,phase:currentPhase,sequence:++sequence,started_ms:now()};requestMeta.set(request,meta);
        currentTask.requests.push({sequence:meta.sequence,phase:meta.phase,method:request.method(),url:request.url(),path:url.pathname,headers:request.headers(),body_bytes:body?.length??0,body_sha256:body?hash(body):null,resource_type:request.resourceType(),timing:request.timing(),started_ms:meta.started_ms});
      });
      context.on('requestfailed',request=>{const meta=requestMeta.get(request);if(!meta)return;const record={sequence:meta.sequence,phase:meta.phase,method:request.method(),url:request.url(),failure:request.failure(),observed_ms:now()};if(meta.phase==='export'&&request.method()==='POST'&&new URL(request.url()).pathname==='/theme-lab'&&request.failure()?.errorText?.includes('ERR_ABORTED'))meta.task.expected_download_aborts.push(record);else meta.task.request_failures.push(record);});
      context.on('response',response=>{
        const request=response.request(),meta=requestMeta.get(request);if(!meta)return;
        const headers=response.headers(),record={sequence:meta.sequence,phase:meta.phase,method:request.method(),url:response.url(),path:new URL(response.url()).pathname,status:response.status(),ok:response.ok(),headers,from_service_worker:response.fromServiceWorker(),request_body_bytes:request.postDataBuffer()?.length??0,request_body_sha256:request.postDataBuffer()?hash(request.postDataBuffer()):null,response_body_bytes:null,response_body_sha256:null,sizes:null,body_evidence:null,read_error:null,started_ms:meta.started_ms,response_ms:now(),finished_ms:null};
        meta.task.responses.push(record);responseMeta.set(response,record);
        const wrapped={task:meta.task,phase:meta.phase,promise:(async()=>{try{if(headers['content-disposition']?.startsWith('attachment;')){record.body_evidence='saved-download';try{record.sizes=await bounded(request.sizes());}catch(error){record.sizes_error=String(error);}record.timing=request.timing();}else{const finishError=await bounded(response.finished());if(finishError)throw finishError;record.sizes=await bounded(request.sizes());record.timing=request.timing();const body=await bounded(response.body()),retained=retainBody(body);record.response_body_bytes=body.length;record.response_body_sha256=retained.sha256;record.response_body_path=retained.path;record.body_evidence='retained-response-body';}}catch(error){record.read_error=String(error);meta.task.errors.push(`response ${record.method} ${record.path}: ${error}`);}finally{record.finished_ms=now();}})()};assign(wrapped);
      });
      page=await context.newPage();
      page.on('pageerror',error=>currentTask?.errors.push(`pageerror: ${error}`));
      page.on('console',message=>{if(currentTask&&['warning','error'].includes(message.type())){const location=message.location();currentTask.console.push({type:message.type(),text:message.text(),url:location.url,line:location.lineNumber,column:location.columnNumber,observed_ms:now()});}});

      const frameState=async()=>{
        const frame=page.frameLocator('iframe');await frame.locator('h1').waitFor({state:'visible'});
        return frame.locator('body').evaluate(body=>{const root=getComputedStyle(document.documentElement),computed=getComputedStyle(body);return {heading:document.querySelector('h1')?.textContent?.trim()??'',scheme:document.documentElement.dataset.theme??'',font:computed.fontFamily,radius_md:root.getPropertyValue('--ak-radius-md').trim(),accent:root.getPropertyValue('--ak-color-accent').trim(),scripts:document.scripts.length};});
      };
      const waitFrameState=async predicate=>{const deadline=performance.now()+7000;let state,lastError;while(performance.now()<deadline){try{state=await frameState();if(predicate(state))return state;}catch(error){lastError=error;}await new Promise(resolve=>setTimeout(resolve,25));}throw new Error(`Useful iframe state deadline exceeded: ${lastError??JSON.stringify(state)}`);};
      const waitPageState=async predicate=>{const deadline=performance.now()+7000;let state,lastError;while(performance.now()<deadline){try{state=await predicate();if(state)return state;}catch(error){lastError=error;}await new Promise(resolve=>setTimeout(resolve,25));}throw new Error(`Useful guide state deadline exceeded: ${lastError??'postcondition false'}`);};
      const openDetail=async name=>{const detail=page.locator('details').filter({has:page.locator('summary',{hasText:name})});if(await detail.getAttribute('open')===null)await detail.locator('summary').click();};
      const serverIdentity=async()=>page.locator('.sg-identity dl').evaluate(dl=>{const terms=[...dl.querySelectorAll('dt')],values=[...dl.querySelectorAll('dd')];return Object.fromEntries(terms.map((term,index)=>[term.textContent.trim(),values[index]?.textContent.trim()??'']));});
      const usefulInitial=async()=>{await page.getByRole('heading',{name:'Gallery theme lab',level:1}).waitFor({state:'visible'});const frame=await frameState(),identity=await serverIdentity(),digest=await page.locator('[name="review_digest"]').inputValue(),exportDisabled=await page.getByRole('button',{name:'Export acknowledged candidate ZIP',exact:true}).isDisabled(),sourceSHA=hash(sourceBefore);return {guide_heading_visible:true,frame_heading:frame.heading,scheme:frame.scheme,digest_blank:digest==='',export_disabled:exportDisabled,baseline_style:frame,source_label:identity['Source label'],source_label_matches:identity['Source label']===selected.source,source_sha256:identity['Original source SHA-256'],source_sha256_matches:identity['Original source SHA-256']===sourceSHA,server_identity:identity['Selected kit identity'],server_identity_matches:identity['Selected kit identity']===expectedServerIdentity,pass:frame.heading==='Dashboard'&&frame.scheme==='light'&&digest===''&&exportDisabled&&frame.scripts===0&&identity['Source label']===selected.source&&identity['Original source SHA-256']===sourceSHA&&identity['Selected kit identity']===expectedServerIdentity};};
      const saveDownload=async(task,response,artifact,digest)=>{
        const filename=`${task.lane}-s${String(sample).padStart(2,'0')}-${viewport.name}-${specimen}-${scheme}.zip`,saved=path.join(outputDir,filename);if(existsSync(saved))throw new Error(`Refusing to replace ${saved}`);await artifact.saveAs(saved);const bytes=readFileSync(saved),headers=response.headers();
        const members=execFileSync('/usr/bin/unzip',['-Z1',saved],{encoding:'utf8',maxBuffer:1<<20}).trim().split('\n').sort();
        const bundle=Object.fromEntries(exactMembers.map(name=>[name,execFileSync('/usr/bin/unzip',['-p',saved,name],{maxBuffer:2<<20})]));
        const receipt=JSON.parse(bundle['receipt.json']),receiptSHA=hash(bundle['receipt.json']);
        const pins=Object.entries(receipt.files??{}),allPins=pins.length===4&&pins.every(([name,pin])=>bundle[name]&&pin.sha256===hash(bundle[name])&&pin.size===bundle[name].length);
        const record=responseMeta.get(response);if(record){record.response_body_bytes=bytes.length;record.response_body_sha256=hash(bytes);record.response_body_path=saved;record.body_evidence='saved-download';}
        return {path:saved,filename,suggested_filename:artifact.suggestedFilename(),bytes:bytes.length,sha256:hash(bytes),status:response.status(),header_sha256:headers['x-content-sha256']??'',header_sha256_match:headers['x-content-sha256']===hash(bytes),exact_members:JSON.stringify(members)===JSON.stringify(exactMembers),members,original_matches_source:bundle['original.cue'].equals(sourceBefore),receipt_sha256:receiptSHA,receipt_digest_matches_review:receiptSHA===digest,receipt_source_label:receipt.source_label,receipt_source_label_matches:receipt.source_label===selected.source,receipt_kit_identity:receipt.kit_revision,receipt_kit_identity_matches:receipt.kit_revision===expectedServerIdentity,receipt_schema:receipt.schema_version,receipt_scope:receipt.scope,receipt_exportable:receipt.exportable,all_receipt_file_pins:allPins,file_pins:pins.map(([name,pin])=>({name,expected:pin,actual:{sha256:bundle[name]?hash(bundle[name]):null,size:bundle[name]?.length??null}})),candidate_sha256:hash(bundle['candidate.cue']),diff_sha256:hash(bundle['diff.json']),diff_valid_json:(()=>{try{JSON.parse(bundle['diff.json']);return true;}catch{return false;}})()};
      };
      const runTask=async task=>{
        currentTask=task;
        currentPhase='navigation';let started=performance.now();
        const initialResponse=await page.goto(config.base+'/theme-lab',{waitUntil:'domcontentloaded'});task.postconditions.initial=await usefulInitial();task.postconditions.initial.status=initialResponse?.status()??null;task.postconditions.initial.pass=task.postconditions.initial.pass&&initialResponse?.status()===200;task.timings_ms.navigation=timed(started);await drain(task,'navigation');task.closures.navigation=closureFor(task,'navigation');task.closures.navigation.accounting=accountingFor(task,'navigation');
        await page.locator('[name="specimen"]').selectOption(specimen);await page.locator('[name="scheme"]').selectOption(scheme);await page.locator('select[name="viewport"]').selectOption(viewport.form);
        await openDetail('Typography');await page.locator('[name="fonts.sans"]').selectOption('system-serif');await openDetail('Corner radii');await page.locator('[name="radius.md"]').fill('.75rem');await openDetail('Color roles');await page.locator('[name="colors.accent.light"]').fill('#ff7edb');
        const sourceAccent=await page.locator(`[name="colors.accent.${scheme}"]`).getAttribute('placeholder');
        currentPhase='preview';const previewWait=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/theme-lab'),previewNavigation=page.waitForEvent('framenavigated',frame=>frame===page.mainFrame());await page.getByRole('button',{name:'Preview candidate',exact:true}).focus();started=performance.now();const [previewResponse]=await Promise.all([previewWait,previewNavigation,page.keyboard.press('Enter')]);
        const digest=await waitPageState(async()=>{const value=await page.locator('[name="review_digest"]').inputValue();return validDigest(value)?value:false;}),previewFrame=await waitFrameState(state=>state.heading===title&&state.scheme===scheme&&normal(state.font).includes('ui-serif')&&normal(state.font).includes('georgia')&&['.75rem','0.75rem'].includes(normal(state.radius_md))),exportEnabled=!(await page.getByRole('button',{name:'Export acknowledged candidate ZIP',exact:true}).isDisabled());
        const accentPass=scheme==='light'?normal(previewFrame.accent)==='#ff7edb':normal(previewFrame.accent)===normal(sourceAccent);
        task.postconditions.preview={status:previewResponse.status(),frame_heading:previewFrame.heading,scheme:previewFrame.scheme,digest,digest_sha256:validDigest(digest),export_enabled:exportEnabled,font_system_serif:normal(previewFrame.font).includes('ui-serif')&&normal(previewFrame.font).includes('georgia'),radius_md_075rem:['.75rem','0.75rem'].includes(normal(previewFrame.radius_md)),accent:previewFrame.accent,expected_accent:scheme==='light'?'#ff7edb':sourceAccent,accent_matches:accentPass,no_authored_script:previewFrame.scripts===0,pass:previewResponse.status()===200&&previewFrame.heading===title&&previewFrame.scheme===scheme&&validDigest(digest)&&exportEnabled&&normal(previewFrame.font).includes('ui-serif')&&normal(previewFrame.font).includes('georgia')&&['.75rem','0.75rem'].includes(normal(previewFrame.radius_md))&&accentPass&&previewFrame.scripts===0};
        task.timings_ms.preview=timed(started);await drain(task,'preview');task.closures.preview=closureFor(task,'preview');task.closures.preview.accounting=accountingFor(task,'preview');
        currentPhase='export';const exportWait=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/theme-lab');await page.getByRole('button',{name:'Export acknowledged candidate ZIP',exact:true}).focus();started=performance.now();const [artifact,exportResponse]=await Promise.all([page.waitForEvent('download'),exportWait,page.keyboard.press('Enter')]);const download=await saveDownload(task,exportResponse,artifact,digest);task.timings_ms.export=timed(started);task.postconditions.export={...download,pass:download.status===200&&download.header_sha256_match&&download.exact_members&&download.original_matches_source&&download.receipt_digest_matches_review&&download.receipt_source_label_matches&&download.receipt_kit_identity_matches&&download.receipt_schema===1&&download.receipt_scope==='experimental-app-owned-styleguide'&&download.receipt_exportable===true&&download.all_receipt_file_pins&&download.diff_valid_json};await drain(task,'export');task.closures.export=closureFor(task,'export');task.closures.export.accounting=accountingFor(task,'export');
        currentPhase='reset';started=performance.now();const [resetResponse]=await Promise.all([page.waitForResponse(r=>r.request().method()==='GET'&&new URL(r.url()).pathname==='/theme-lab'),page.waitForEvent('framenavigated',frame=>frame===page.mainFrame()),page.getByRole('link',{name:'Reset to source',exact:true}).click()]);const resetFrame=await waitFrameState(state=>JSON.stringify(state)===JSON.stringify(task.postconditions.initial.baseline_style));await waitPageState(async()=>{const values=await Promise.all(['review_digest','fonts.sans','radius.md','colors.accent.light'].map(name=>page.locator(`[name="${name}"]`).inputValue()));return values.every(value=>value==='')&&await page.getByRole('button',{name:'Export acknowledged candidate ZIP',exact:true}).isDisabled();});const resetDigest=await page.locator('[name="review_digest"]').inputValue(),resetDisabled=await page.getByRole('button',{name:'Export acknowledged candidate ZIP',exact:true}).isDisabled(),edits=await Promise.all(['fonts.sans','radius.md','colors.accent.light'].map(name=>page.locator(`[name="${name}"]`).inputValue()));task.timings_ms.reset=timed(started);task.postconditions.reset={status:resetResponse.status(),frame_heading:resetFrame.heading,scheme:resetFrame.scheme,digest_blank:resetDigest==='',export_disabled:resetDisabled,edits_blank:edits.every(value=>value===''),baseline_style_restored:JSON.stringify(resetFrame)===JSON.stringify(task.postconditions.initial.baseline_style),style:resetFrame,pass:resetResponse.status()===200&&resetFrame.heading==='Dashboard'&&resetFrame.scheme==='light'&&resetDigest===''&&resetDisabled&&edits.every(value=>value==='')&&JSON.stringify(resetFrame)===JSON.stringify(task.postconditions.initial.baseline_style)};await drain(task,'reset');task.closures.reset=closureFor(task,'reset');task.closures.reset.accounting=accountingFor(task,'reset');
        const posts=task.requests.filter(item=>item.method==='POST'&&item.path==='/theme-lab'),iframeSandbox=await page.locator('iframe').getAttribute('sandbox'),ownedDocuments=task.responses.filter(item=>item.path.startsWith('/theme-lab')),noStore=ownedDocuments.length>0&&ownedDocuments.every(item=>item.headers['cache-control']==='no-store');
        task.postconditions.transport={same_origin_posts:posts.length===2&&posts.every(item=>item.headers.origin===config.base),post_origins:posts.map(item=>item.headers.origin??null),iframe_sandbox:iframeSandbox,opaque_iframe:iframeSandbox==='',no_store_owned_responses:noStore,service_workers:task.workers.length,pass:posts.length===2&&posts.every(item=>item.headers.origin===config.base)&&iframeSandbox===''&&noStore&&task.workers.length===0};
        task.ok=Object.values(task.postconditions).every(item=>item.pass)&&Object.values(task.closures).every(item=>item.exact&&item.accounting.complete)&&task.responses.every(item=>item.ok&&!item.read_error)&&task.blocked.length===0&&task.request_failures.length===0&&task.errors.length===0&&task.console.length===0&&task.workers.length===0;
      };
      const fresh=newTask('ordinary-fresh',sample,viewport,specimen,scheme);report.tasks.push(fresh);try{await runTask(fresh);}catch(error){fresh.errors.push(String(error));fresh.ok=false;}
      const repeat=newTask('ordinary-repeat',sample,viewport,specimen,scheme);report.tasks.push(repeat);try{await runTask(repeat);}catch(error){repeat.errors.push(String(error));repeat.ok=false;}
    }catch(error){
      report.errors.push(`sample ${sample} ${viewport.name} ${specimen} ${scheme}: ${error}`);
      for(const lane of ['ordinary-fresh','ordinary-repeat'])if(!report.tasks.some(item=>item.lane===lane&&item.sample===sample&&item.viewport.name===viewport.name&&item.specimen===specimen&&item.scheme===scheme)){const missing=newTask(lane,sample,viewport,specimen,scheme);missing.errors.push(`not executed: ${error}`);report.tasks.push(missing);}
    }finally{
      currentTask=null;await Promise.allSettled([...pending].map(item=>item.promise));if(context){await context.close().catch(error=>report.errors.push(`context close: ${error}`));report.cleanup.contexts_closed++;}
    }
  }
}catch(error){report.errors.push(`acquisition: ${error}`);}
finally{
  if(browser){await browser.close().then(()=>{report.cleanup.browser_closed=true;}).catch(error=>report.errors.push(`browser close: ${error}`));}
  let sourceIdentityAfter;try{const sourceAfter=readFileSync(selected.source),afterSHA=hash(sourceAfter);report.source.after_sha256=afterSHA;report.source.after_bytes=sourceAfter.length;report.source.unchanged=sourceAfter.equals(sourceBefore);sourceIdentityAfter={sha256:afterSHA,matches:afterSHA===identities.source.sha256};}catch(error){report.source.after_read_error=String(error);report.source.unchanged=false;sourceIdentityAfter={error:String(error),matches:false};report.errors.push(`final source read: ${error}`);}
  report.identity_after={source:sourceIdentityAfter};for(const [name,identity] of Object.entries(identities)){if(!identity.path||name==='source')continue;try{const actual=await fileHash(identity.path);report.identity_after[name]={sha256:actual,matches:actual===identity.sha256};}catch(error){report.identity_after[name]={error:String(error),matches:false};report.errors.push(`final ${name} identity read: ${error}`);}}
  report.finished=new Date().toISOString();report.duration_ms=Number((performance.now()-acquisitionStart).toFixed(3));
  report.summary={tasks:report.tasks.length,passed:report.tasks.filter(task=>task.ok).length,failed:report.tasks.filter(task=>!task.ok).length,ordinary_complete:report.tasks.length===report.acquisition.ordinary_expected_tasks&&report.tasks.every(task=>task.ok),degraded_complete:false,memory_complete:false,cancellation_complete:false};
  report.ok=report.summary.ordinary_complete&&report.errors.length===0&&report.source.unchanged&&report.cleanup.browser_closed&&report.cleanup.contexts_created===report.cleanup.contexts_closed&&Object.values(report.identity_after).every(item=>item.matches);
  writeFileSync(receiptPath,JSON.stringify(report,null,2)+'\n',{flag:'wx',mode:0o600});
  console.log(JSON.stringify({receipt:receiptPath,ok:report.ok,summary:report.summary,errors:report.errors}));
  if(!report.ok)process.exitCode=1;
}
