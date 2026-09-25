// Independent ordinary-lane checker. Never certifies the four unmeasured lanes.
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';

const hash = value => createHash('sha256').update(value).digest('hex');
const isHash = value => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const titles = {dashboard:'Dashboard', components:'Components', forms:'Forms'};
const views = {wide:[1440,1000], narrow:[390,844]};
const lanes = ['ordinary-fresh','ordinary-repeat'], schemes = ['light','dark'];
const ceilings = {navigation:1000, preview:500, export:500, reset:1000};
const members = ['candidate.css','candidate.cue','diff.json','original.cue','receipt.json'];
const identityKeys = ['probe','config','contract','source','server','playwright_entry','playwright_package','browser_executable','node'];
const expectedClosure = phase => phase === 'export' ? ['POST /theme-lab'] : [
  `${phase === 'preview' ? 'POST' : 'GET'} /theme-lab`, 'GET /theme-lab/style.css','GET /theme-lab/preview','GET /static/app.css',
].sort();
const normalize = value => String(value ?? '').trim().toLowerCase().replace(/\s+/g,' ');

export function checkReceipt(receipt, {snapshot, readArtifact} = {}) {
  const errors = [], maxima = {navigation:0,preview:0,export:0,reset:0,view_bytes:0,zip_bytes:0};
  const check = (condition, label) => {if (!condition) errors.push(label);};
  const number = (value, limit, label, integer=false) => check(typeof value==='number' && Number.isFinite(value) && value>=0 && value<=limit && (!integer||Number.isSafeInteger(value)),label);
  const array = (value,label) => {check(Array.isArray(value),label);return Array.isArray(value)?value:[];};
  const digest = (value,label) => check(isHash(value),label);
  const read = (path,bytes,sha,label) => {if(readArtifact)try{const body=readArtifact(path);check(body.length===bytes&&hash(body)===sha,label);}catch(error){check(false,`${label}: ${error}`);}};
  if(!receipt||typeof receipt!=='object')return{ok:false,complete:false,errors:['missing receipt']};
  check(receipt.schema==='theme-guide-browser-acquisition-v1'&&receipt.contract==='theme-guide-browser-v1','schema/contract');
  check(snapshot?.schema==='theme-guide-browser-inputs-v1','independently supplied frozen snapshot required');
  const ids=receipt.identities??{};
  for(const key of [...identityKeys,'snapshot','server_build_receipt','go']){
    const item=ids[key]??{}, expected=snapshot?.files?.[key]??snapshot?.[key];
    check(typeof item.path==='string'&&item.path.startsWith('/'),`identity ${key}: path`);
    number(item.bytes,Number.MAX_SAFE_INTEGER,`identity ${key}: bytes`,true);digest(item.sha256,`identity ${key}: SHA`);
    read(item.path,item.bytes,item.sha256,`identity ${key}: actual file differs/unreadable`);
    if(identityKeys.includes(key)||key==='server_build_receipt')check(!!expected&&item.path===expected.path&&item.sha256===expected.sha256,`identity ${key}: differs from frozen input`);
    check(receipt.identity_after?.[key]?.matches===true&&receipt.identity_after[key].sha256===item.sha256,`identity ${key}: final read missing/different`);
  }
  const server=`local-gallery-binary:sha256:${ids.server?.sha256}`,source=receipt.source??{};
  check(typeof ids.node_version==='string'&&/^v\d+\./.test(ids.node_version)&&ids.node_version===snapshot?.node?.version,'Node runtime version missing/different');
  check(typeof ids.go?.version==='string'&&ids.go.version.startsWith('go version '),'Go build version missing');
  if(readArtifact)try{const build=JSON.parse(readArtifact(ids.server_build_receipt.path));check(build.passed===true&&build.server?.path===ids.server.path&&build.server.sha256===ids.server.sha256&&build.go?.path===ids.go.path&&build.go.sha256===ids.go.sha256&&build.go.version===ids.go.version,'actual build receipt server/Go binding differs');}catch(error){check(false,'actual build receipt unreadable: '+error);}
  check(ids.expected_server_identity===server,'server identity not actual selected executable');
  check(typeof ids.browser_version==='string'&&/^\d+\./.test(ids.browser_version),'browser version missing');
  for(const key of ['platform','release','arch'])check(typeof ids.os?.[key]==='string'&&!!ids.os[key],`OS ${key} missing`);
  check(typeof ids.hardware?.cpu_model==='string'&&!!ids.hardware.cpu_model,'CPU missing');
  check(source.before_sha256===ids.source?.sha256&&source.after_sha256===source.before_sha256&&source.unchanged===true,'source changed/unverified');
  check(source.before_bytes===ids.source?.bytes&&source.after_bytes===source.before_bytes,'source bytes changed');
  check(receipt.cleanup?.browser_closed===true&&receipt.cleanup?.contexts_created===60&&receipt.cleanup?.contexts_closed===60,'cleanup incomplete');
  check(array(receipt.errors,'missing acquisition errors').length===0,'acquisition errors');
  try{const u=new URL(receipt.origin);check(u.origin===receipt.origin&&u.protocol==='http:'&&['127.0.0.1','[::1]'].includes(u.hostname)&&!!u.port,'origin not exact loopback');}catch{check(false,'bad origin');}
  const tasks=array(receipt.tasks,'missing tasks'),keys=new Set();
  for(const [index,t]of tasks.entries()){
    const p=`task ${index}`;
    if(!t||typeof t!=='object'){check(false,p+' missing');continue;}
    const key=[t.lane,t.sample,t.viewport?.name,t.specimen,t.scheme].join('|'),view=views[t.viewport?.name];
    check(!keys.has(key),p+' duplicate matrix identity');keys.add(key);
    check(lanes.includes(t.lane)&&Number.isInteger(t.sample)&&t.sample>=1&&t.sample<=5&&!!view&&Object.hasOwn(titles,t.specimen)&&schemes.includes(t.scheme),p+' invalid matrix identity');
    check(view&&t.viewport.width===view[0]&&t.viewport.height===view[1]&&t.viewport.form===t.viewport.name,p+' wrong viewport');
    check(t.cache_disabled===true,p+' routed run claims cache');
    for(const [metric,limit]of Object.entries(ceilings)){const v=t.timings_ms?.[metric];number(v,limit,p+' '+metric+' ms');if(Number.isFinite(v))maxima[metric]=Math.max(maxima[metric],v);}
    for(const field of ['errors','blocked','request_failures','console','workers'])check(array(t[field],p+' missing '+field).length===0,p+' '+field+' present');
    const post=t.postconditions??{},initial=post.initial??{},preview=post.preview??{},exp=post.export??{},reset=post.reset??{},baseline=initial.baseline_style??{},transport=post.transport??{};
    check(initial.status===200&&initial.guide_heading_visible===true&&initial.frame_heading==='Dashboard'&&initial.scheme==='light'&&initial.digest_blank===true&&initial.export_disabled===true&&initial.server_identity===server&&initial.source_sha256===source.before_sha256,p+' initial state');
    check(initial.source_label===ids.source?.path&&initial.source_label_matches===true&&exp.receipt_source_label===ids.source?.path&&exp.receipt_source_label_matches===true,p+' source label identity');
    check(normalize(baseline.font)==='inter, system-ui, sans-serif'&&normalize(baseline.radius_md)==='0.375rem'&&normalize(baseline.accent)==='#f92aad'&&baseline.scripts===0,p+' baseline computed values');
    check(preview.status===200&&preview.frame_heading===titles[t.specimen]&&preview.scheme===t.scheme&&isHash(preview.digest)&&preview.digest_sha256===true&&preview.export_enabled===true&&preview.font_system_serif===true&&preview.radius_md_075rem===true&&preview.no_authored_script===true&&normalize(preview.accent)===(t.scheme==='light'?'#ff7edb':'#f92aad'),p+' preview state');
    check(reset.status===200&&reset.frame_heading==='Dashboard'&&reset.scheme==='light'&&reset.digest_blank===true&&reset.export_disabled===true&&reset.edits_blank===true&&reset.baseline_style_restored===true&&JSON.stringify(reset.style)===JSON.stringify(baseline),p+' reset state');
    check(transport.same_origin_posts===true&&transport.opaque_iframe===true&&transport.iframe_sandbox===''&&transport.no_store_owned_responses===true,p+' transport policy');
    number(exp.bytes,1<<20,p+' ZIP bytes',true);digest(exp.sha256,p+' ZIP SHA');digest(exp.receipt_sha256,p+' receipt SHA');if(Number.isFinite(exp.bytes))maxima.zip_bytes=Math.max(maxima.zip_bytes,exp.bytes);
    check(exp.status===200&&exp.header_sha256===exp.sha256&&exp.header_sha256_match===true&&JSON.stringify(exp.members)===JSON.stringify(members)&&exp.exact_members===true&&exp.original_matches_source===true&&exp.receipt_sha256===preview.digest&&exp.receipt_digest_matches_review===true&&exp.receipt_kit_identity===server&&exp.receipt_kit_identity_matches===true&&exp.receipt_schema===1&&exp.receipt_scope==='experimental-app-owned-styleguide'&&exp.receipt_exportable===true&&exp.all_receipt_file_pins===true&&exp.diff_valid_json===true,p+' exported source/candidate identity');
    read(exp.path,exp.bytes,exp.sha256,p+' saved ZIP differs/unreadable');
    const pins=array(exp.file_pins,p+' missing export pins');
    check(JSON.stringify(pins.map(pin=>pin?.name).sort())===JSON.stringify(members.filter(name=>name!=='receipt.json')),p+' export pin set');
    for(const pin of pins){digest(pin.actual?.sha256,p+' pin SHA');number(pin.actual?.size,1<<20,p+' pin bytes',true);check(pin.expected?.sha256===pin.actual?.sha256&&pin.expected?.size===pin.actual?.size,p+' member pin differs');}
    check(pins.find(pin=>pin.name==='original.cue')?.actual?.sha256===source.before_sha256,p+' original pin differs');
    for(const [name,field]of [['candidate.cue','candidate_sha256'],['diff.json','diff_sha256']])check(pins.find(pin=>pin.name===name)?.actual?.sha256===exp[field],p+' '+name+' hash differs');
    const requests=array(t.requests,p+' missing requests'),responses=array(t.responses,p+' missing responses'),requestMap=new Map(),seen=new Set();
    for(const req of requests){check(Number.isSafeInteger(req?.sequence)&&!requestMap.has(req.sequence),p+' request sequence missing/duplicate');requestMap.set(req?.sequence,req);try{const u=new URL(req.url);check(u.origin===receipt.origin&&u.pathname===req.path,p+' request outside origin/path');}catch{check(false,p+' bad request URL');}if(req.method==='POST')check(req.headers?.origin===receipt.origin,p+' native POST Origin');}
    for(const res of responses){
      const req=requestMap.get(res?.sequence);check(!!req&&!seen.has(res?.sequence)&&res.url===req.url&&res.method===req.method&&res.phase===req.phase,p+' orphan/duplicate response');seen.add(res?.sequence);
      check(res.status===200&&res.ok===true&&res.from_service_worker===false&&!res.read_error,p+' failed/unreadable response');
      number(res.response_body_bytes,res.phase==='export'?1<<20:256<<10,p+' response bytes',true);digest(res.response_body_sha256,p+' body SHA');
      check(res.body_evidence===(res.phase==='export'?'saved-download':'retained-response-body'),p+' body evidence missing');
      check(!res.sizes_error&&!!res.sizes,p+' transfer observations missing/failed');
      for(const name of ['requestBodySize','requestHeadersSize','responseBodySize','responseHeadersSize'])number(res.sizes?.[name],Number.MAX_SAFE_INTEGER,p+' transfer '+name,true);
      check(res.sizes?.requestBodySize===req?.body_bytes&&res.sizes?.responseBodySize>0&&res.sizes?.requestHeadersSize>0&&res.sizes?.responseHeadersSize>0,p+' transfer sizes invalid/unaccounted');
      read(res.response_body_path,res.response_body_bytes,res.response_body_sha256,p+' saved response differs/unreadable');
      if(res.headers?.['content-length']!==undefined)check(res.headers['content-length']===String(res.response_body_bytes),p+' body length differs');
      for(const metric of ['started_ms','response_ms','finished_ms'])number(res[metric],Number.MAX_SAFE_INTEGER,p+' response '+metric);
      check(res.started_ms<=res.response_ms&&res.response_ms<=res.finished_ms,p+' response time order');
      if(res.phase==='export')check(res.response_body_bytes===exp.bytes&&res.response_body_sha256===exp.sha256,p+' download response differs');
      if(res.path==='/theme-lab'&&res.phase!=='export')check(res.headers?.['cache-control']==='no-store'&&res.headers?.['referrer-policy']==='same-origin'&&res.headers?.['content-security-policy']?.includes("script-src 'none'"),p+' editor policy');
      if(res.path==='/theme-lab/preview')check(res.headers?.['cache-control']==='no-store'&&res.headers?.['content-security-policy']?.includes('sandbox')&&res.headers?.['content-security-policy']?.includes("script-src 'none'"),p+' preview policy');
    }
    check(seen.size===requestMap.size,p+' request without response');
    const aborts=array(t.expected_download_aborts,p+' missing download abort records');
    check(aborts.length<=1,p+' duplicate download abort');
    for(const abort of aborts)check(abort.phase==='export'&&abort.method==='POST'&&abort.url===receipt.origin+'/theme-lab'&&abort.failure?.errorText==='net::ERR_ABORTED'&&responses.some(res=>res.sequence===abort.sequence&&res.phase==='export'&&res.response_body_sha256===exp.sha256),p+' unassociated download abort');
    for(const phase of Object.keys(ceilings)){
      const keyOf=x=>`${x.method} ${x.path}`,rs=responses.filter(x=>x.phase===phase),qs=requests.filter(x=>x.phase===phase);
      check(JSON.stringify(qs.map(keyOf).sort())===JSON.stringify(expectedClosure(phase))&&JSON.stringify(rs.map(keyOf).sort())===JSON.stringify(expectedClosure(phase)),p+' '+phase+' incomplete/duplicate closure');
      const bytes=rs.reduce((sum,r)=>sum+r.response_body_bytes,0);number(bytes,phase==='export'?1<<20:256<<10,p+' '+phase+' decoded closure',true);if(phase!=='export'&&Number.isFinite(bytes))maxima.view_bytes=Math.max(maxima.view_bytes,bytes);
      const accounting=t.closures?.[phase]?.accounting;
      check(accounting?.complete===true&&accounting.responses===rs.length&&accounting.successful_responses===rs.length&&accounting.decoded_body_bytes===bytes&&accounting.transfer_observation_complete===true&&accounting.transfer_body_bytes===rs.reduce((sum,r)=>sum+(r.sizes?.responseBodySize??NaN),0),p+' '+phase+' transfer/decoded accounting differs');
    }
    check(requests.length===13&&responses.length===13,p+' unexpected phase traffic');
  }
  check(tasks.length===120,'matrix: all 120 ordinary tasks required even in scoped mode');
  for(const lane of lanes)for(let sample=1;sample<=5;sample++)for(const view of Object.keys(views))for(const specimen of Object.keys(titles))for(const scheme of schemes)check(keys.has([lane,sample,view,specimen,scheme].join('|')),'matrix missing '+[lane,sample,view,specimen,scheme].join('|'));
  return{ok:errors.length===0,complete:false,scope:'ordinary-first-and-repeat-only',tasks:tasks.length,maxima,errors,incomplete_dimensions:['degraded','cache','memory','cancellation']};
}

if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
  try{const raw=readFileSync(process.argv[2]),snapshotRaw=readFileSync(process.argv[3]),receipt=JSON.parse(raw),snapshot=JSON.parse(snapshotRaw),result=checkReceipt(receipt,{snapshot,readArtifact:readFileSync});
    if(hash(snapshotRaw)!==receipt.identities?.snapshot?.sha256)result.errors.push('snapshot raw identity differs');
    for(const [i,t]of (receipt.tasks??[]).entries())try{
      const exp=t.postconditions.export,read=name=>execFileSync('/usr/bin/unzip',['-p',exp.path,name],{maxBuffer:2<<20});
      const names=execFileSync('/usr/bin/unzip',['-Z1',exp.path],{encoding:'utf8',maxBuffer:1<<20}).trim().split('\n').sort();
      if(JSON.stringify(names)!==JSON.stringify(members))throw Error('archive member set differs');
      const bundle=Object.fromEntries(members.map(name=>[name,read(name)])),raw=bundle['receipt.json'],proof=JSON.parse(raw);
      if(hash(raw)!==t.postconditions.preview.digest||proof.kit_revision!==receipt.identities.expected_server_identity||proof.source_label!==receipt.identities.source.path||hash(bundle['original.cue'])!==receipt.source.before_sha256)throw Error('actual receipt/source identity differs');
      for(const name of members.filter(n=>n!=='receipt.json'))if(proof.files?.[name]?.sha256!==hash(bundle[name])||proof.files[name].size!==bundle[name].length)throw Error('actual member pin differs: '+name);
    }catch(error){result.errors.push(`task ${i}: actual ZIP verification: ${error}`);}
    if(!process.argv.includes('--scoped'))result.errors.push('complete browser gate unavailable: four separate lanes incomplete');
    result.ok=result.errors.length===0;console.log(JSON.stringify(result));process.exitCode=result.ok?0:1;
  }catch(error){console.log(JSON.stringify({ok:false,complete:false,errors:[String(error)]}));process.exitCode=1;}
}
