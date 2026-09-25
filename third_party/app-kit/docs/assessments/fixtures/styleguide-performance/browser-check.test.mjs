import test from 'node:test';
import assert from 'node:assert/strict';
import {checkReceipt} from './browser-check.mjs';

const pin={sha256:'a'.repeat(64),size:10},server='local-gallery-binary:sha256:'+pin.sha256;
function fixture(){
  const ids=Object.fromEntries(['probe','config','contract','source','server','playwright_entry','playwright_package','browser_executable','node','go','server_build_receipt','snapshot'].map(name=>[name,{path:'/test/'+name,bytes:10,sha256:pin.sha256}]));
  const snapshot={schema:'theme-guide-browser-inputs-v1',files:structuredClone(ids),server_build_receipt:structuredClone(ids.server_build_receipt)};
  const r={schema:'theme-guide-browser-acquisition-v1',contract:'theme-guide-browser-v1',origin:'http://127.0.0.1:18766',identities:{...ids,browser_version:'152.0.0',os:{platform:'darwin',release:'25.6',arch:'arm64'},hardware:{cpu_model:'fixture'},expected_server_identity:server},identity_after:Object.fromEntries(Object.keys(ids).map(key=>[key,{sha256:pin.sha256,matches:true}])),source:{before_sha256:pin.sha256,after_sha256:pin.sha256,before_bytes:10,after_bytes:10,unchanged:true},cleanup:{browser_closed:true,contexts_created:60,contexts_closed:60},errors:[],tasks:[]};
  for(const lane of ['ordinary-fresh','ordinary-repeat'])for(let sample=1;sample<=5;sample++)for(const [name,width,height]of [['wide',1440,1000],['narrow',390,844]])for(const [specimen,title]of [['dashboard','Dashboard'],['components','Components'],['forms','Forms']])for(const scheme of ['light','dark']){
    const style={heading:'Dashboard',scheme:'light',font:'Inter, system-ui, sans-serif',radius_md:'0.375rem',accent:'#f92aad',scripts:0},members=['candidate.css','candidate.cue','diff.json','original.cue','receipt.json'];
    const t={lane,sample,viewport:{name,width,height,form:name},specimen,scheme,cache_disabled:true,timings_ms:{navigation:10,preview:10,export:10,reset:10},errors:[],blocked:[],request_failures:[],expected_download_aborts:[],console:[],workers:[],requests:[],responses:[],postconditions:{
      initial:{status:200,guide_heading_visible:true,frame_heading:'Dashboard',scheme:'light',digest_blank:true,export_disabled:true,server_identity:server,source_sha256:pin.sha256,baseline_style:style},
      preview:{status:200,frame_heading:title,scheme,digest:pin.sha256,digest_sha256:true,export_enabled:true,font_system_serif:true,radius_md_075rem:true,no_authored_script:true,accent:scheme==='light'?'#ff7edb':'#f92aad'},
      reset:{status:200,frame_heading:'Dashboard',scheme:'light',digest_blank:true,export_disabled:true,edits_blank:true,baseline_style_restored:true,style:structuredClone(style)},
      transport:{same_origin_posts:true,opaque_iframe:true,iframe_sandbox:'',no_store_owned_responses:true},
      export:{path:'/test/export.zip',bytes:10,sha256:pin.sha256,receipt_sha256:pin.sha256,status:200,header_sha256:pin.sha256,header_sha256_match:true,members,exact_members:true,original_matches_source:true,receipt_digest_matches_review:true,receipt_kit_identity:server,receipt_kit_identity_matches:true,receipt_schema:1,receipt_scope:'experimental-app-owned-styleguide',receipt_exportable:true,all_receipt_file_pins:true,diff_valid_json:true,candidate_sha256:pin.sha256,diff_sha256:pin.sha256,file_pins:members.filter(n=>n!=='receipt.json').map(name=>({name,expected:structuredClone(pin),actual:structuredClone(pin)}))}}};
    let sequence=0;
    for(const phase of ['navigation','preview','export','reset'])for(const [method,path]of phase==='export'?[['POST','/theme-lab']]:[[phase==='preview'?'POST':'GET','/theme-lab'],['GET','/theme-lab/style.css'],['GET','/theme-lab/preview'],['GET','/static/app.css']]){
      const req={sequence:++sequence,phase,method,path,url:r.origin+path,headers:{origin:r.origin}};t.requests.push(req);
      t.responses.push({...req,status:200,ok:true,from_service_worker:false,read_error:null,response_body_bytes:10,response_body_sha256:pin.sha256,response_body_path:'/test/body.bin',body_evidence:phase==='export'?'saved-download':'retained-response-body',started_ms:1,response_ms:2,finished_ms:3,headers:{'content-length':'10','cache-control':'no-store','referrer-policy':'same-origin','content-security-policy':"default-src 'none'; script-src 'none'; sandbox"}});
    }
    r.tasks.push(t);
  }
  r.identities.node_version='v26.3.0';r.identities.go.version='go version go1.26.3 darwin/arm64';snapshot.node={version:'v26.3.0'};
  for(const task of r.tasks){
    Object.assign(task.postconditions.initial,{source_label:ids.source.path,source_label_matches:true});
    Object.assign(task.postconditions.export,{receipt_source_label:ids.source.path,receipt_source_label_matches:true});
    for(const request of task.requests)request.body_bytes=0;
    for(const response of task.responses)response.sizes={requestBodySize:0,requestHeadersSize:10,responseBodySize:10,responseHeadersSize:10};
    task.closures=Object.fromEntries(['navigation','preview','export','reset'].map(phase=>{const count=phase==='export'?1:4;return[phase,{accounting:{complete:true,responses:count,successful_responses:count,decoded_body_bytes:10*count,transfer_observation_complete:true,transfer_body_bytes:10*count}}];}));
  }
  return{r,snapshot};
}
test('all 120 ordinary tasks pass scoped check, never full certification',()=>{const {r,snapshot}=fixture(),out=checkReceipt(r,{snapshot});assert.equal(out.ok,true,JSON.stringify(out.errors));assert.equal(out.complete,false);assert.equal(out.tasks,120);});
const cases=[
  ['missing sample',r=>r.tasks.pop(),'matrix'],['duplicate sample',r=>r.tasks[1]=structuredClone(r.tasks[0]),'duplicate matrix'],
  ['NaN timing',r=>r.tasks[0].timings_ms.preview=NaN,'preview ms'],['infinite timing',r=>r.tasks[0].timings_ms.preview=Infinity,'preview ms'],['negative timing',r=>r.tasks[0].timings_ms.preview=-1,'preview ms'],['excess timing',r=>r.tasks[0].timings_ms.preview=501,'preview ms'],
  ['wrong binary',r=>r.identities.server.sha256='b'.repeat(64),'differs from frozen'],['changed source',r=>r.source.after_sha256='b'.repeat(64),'source changed'],['missing final read',r=>delete r.identity_after.source,'final read'],
  ['wrong heading',r=>r.tasks[0].postconditions.preview.frame_heading='Wrong','preview state'],['wrong baseline',r=>r.tasks[0].postconditions.initial.baseline_style.accent='#000','baseline computed'],['wrong reset',r=>r.tasks[0].postconditions.reset.edits_blank=false,'reset state'],
  ['forged export',r=>r.tasks[0].postconditions.export.receipt_kit_identity='other','exported source'],['wrong file pin',r=>r.tasks[0].postconditions.export.file_pins[0].actual.sha256='b'.repeat(64),'member pin'],
  ['orphan response',r=>r.tasks[0].responses[0].sequence=999,'orphan/duplicate'],['absent response',r=>r.tasks[0].responses.pop(),'request without response'],['failed response',r=>r.tasks[0].responses[0].status=500,'failed/unreadable'],
  ['failed request',r=>r.tasks[0].request_failures.push({failure:'aborted'}),'request_failures present'],['blocked traffic',r=>r.tasks[0].blocked.push({url:'http://127.0.0.1:9999'}),'blocked present'],['unknown phase',r=>r.tasks[0].responses[0].phase='later','incomplete/duplicate closure'],
  ['extra request',r=>r.tasks[0].requests.push({...r.tasks[0].requests[0],sequence:100}),'incomplete/duplicate closure'],['foreign origin',r=>r.tasks[0].requests[0].url='http://127.0.0.1:9999/theme-lab','outside origin'],['missing Origin',r=>delete r.tasks[0].requests[4].headers.origin,'native POST'],
  ['infinite body',r=>r.tasks[0].responses[0].response_body_bytes=Infinity,'response bytes'],['excess closure',r=>{for(const x of r.tasks[0].responses.filter(x=>x.phase==='navigation'))x.response_body_bytes=70000;},'decoded closure'],['oversized ZIP',r=>r.tasks[0].postconditions.export.bytes=1048577,'ZIP bytes'],
  ['missing cleanup',r=>r.cleanup.browser_closed=false,'cleanup'],['console failure',r=>r.tasks[0].console.push({text:'fault'}),'console present'],['worker',r=>r.tasks[0].workers.push('worker'),'workers present'],['unmatched abort',r=>r.tasks[0].expected_download_aborts.push({sequence:500,phase:'export',method:'POST',url:r.origin+'/theme-lab',failure:{errorText:'net::ERR_ABORTED'}}),'unassociated download abort'],
  ['wrong source label',r=>r.tasks[0].postconditions.initial.source_label='/different','source label identity'],['missing Node version',r=>delete r.identities.node_version,'Node runtime version'],['missing Go version',r=>delete r.identities.go.version,'Go build version'],['missing transfer',r=>delete r.tasks[0].responses[0].sizes,'transfer observations'],['invalid transfer',r=>r.tasks[0].responses[0].sizes.responseBodySize=NaN,'transfer responseBodySize'],['false transfer summary',r=>r.tasks[0].closures.preview.accounting.transfer_body_bytes=0,'transfer/decoded accounting'],
];
for(const [name,mutate,expected]of cases)test('reject '+name,()=>{const {r,snapshot}=fixture();mutate(r);const out=checkReceipt(r,{snapshot});assert.equal(out.ok,false);assert(out.errors.some(error=>error.includes(expected)),JSON.stringify(out.errors));});
test('frozen manifest mandatory',()=>assert.equal(checkReceipt(fixture().r).ok,false));
test('fixtures share no mutable identities',()=>{const a=fixture(),b=fixture();delete a.r.identities.source.sha256;assert.equal(checkReceipt(b.r,{snapshot:b.snapshot}).ok,true);});
test('artifact reader failure cannot pass',()=>{const {r,snapshot}=fixture(),out=checkReceipt(r,{snapshot,readArtifact:()=>{throw Error('read failure');}});assert.equal(out.ok,false);assert(out.errors.some(x=>x.includes('unreadable')));});
