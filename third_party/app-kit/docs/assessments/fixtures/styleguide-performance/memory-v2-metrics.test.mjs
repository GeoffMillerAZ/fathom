import test from 'node:test';
import assert from 'node:assert/strict';
import {evaluateMemoryV2Metrics} from './memory-v2-metrics.mjs';

const MIB = 1024 * 1024;
const origin = 'http://127.0.0.1:18766';
const start = {browser:'Wed Sep 17 00:00:00 2026', renderer:'Wed Sep 17 00:00:01 2026', server:'Wed Sep 17 00:00:02 2026'};
const owners = {browser:{pid:100,start_identity:start.browser},server:{pid:200,start_identity:start.server},origin};
const raw = rows => rows.map(r => `${r.pid} ${r.ppid} ${r.rss_kib} ${r.start_identity} ${r.command}`).join('\n')+'\n';
function rows(renderer=1024) { return [
  {pid:100,ppid:1,rss_kib:1000,start_identity:start.browser,command:'/Chrome'},
  {pid:101,ppid:100,rss_kib:renderer,start_identity:start.renderer,command:'/Chrome --type=renderer'},
  {pid:200,ppid:1,rss_kib:512,start_identity:start.server,command:'/server'},
]; }
function endpoint(prefix, started, idleStarted, idleCompleted, completed, sampleIndex, used=1024) {
  const page=prefix+'p', iframe=prefix+'i', pageSession=prefix+'ps', iframeSession=prefix+'is', isolate=prefix+'iso';
  return {started_ms:started,completed_ms:completed,sample_index:sampleIndex,
    idle:{started_ms:idleStarted,completed_ms:idleCompleted,network_before:10,network_after:10,targets_before:[page,iframe],targets_after:[page,iframe]},
    targets:[
      {target_id:page,session_id:pageSession,type:'page',url:origin+'/theme-lab',isolate_id:isolate},
      {target_id:iframe,session_id:iframeSession,type:'iframe',url:origin+'/theme-lab/preview',isolate_id:isolate},
    ],
    isolates:[{id:isolate,session_ids:[pageSession,iframeSession],used_bytes:used,total_bytes:used+1024,
      raw_heap:{usedSize:used,totalSize:used+1024,embedderHeapUsedSize:2048,backingStorageSize:4096}}],
    gc:[{isolate_id:isolate,session_id:pageSession,started_ms:started,completed_ms:idleStarted,ok:true}],
    dom:[{session_id:pageSession,documents:1,nodes:10,jsEventListeners:0},{session_id:iframeSession,documents:1,nodes:10,jsEventListeners:0}],
  };
}
function fixture() {
  const samples=[];
  for(let started=10;started<=12110;started+=100) {
    const list=rows();samples.push({started_ms:started,completed_ms:started+5,process_info:[{id:100,type:'browser',cpuTime:1},{id:101,type:'renderer',cpuTime:1}],raw_ps:raw(list),rows:list,errors:[]});
  }
  const lane={contract_id:'appkit-theme-guide-memory-v2',id:'synthetic',errors:[],samples,cycles:[],work_started_ms:5100,work_completed_ms:7100};
  for(let i=1;i<=20;i++)lane.cycles.push({index:i,started_ms:5000+i*100,completed_ms:5100+i*100,ok:true});
  lane.baseline=endpoint('b',0,10,5010,5015,50);
  lane.final=endpoint('f',7100,7110,12110,12115,121,2048);
  return lane;
}
const evaluate = lane => evaluateMemoryV2Metrics(lane,owners);
function addUtility(sample, command='/Chrome --type=utility --utility-sub-type=unzip.mojom.Unzipper') {
  sample.rows.splice(-1,0,{pid:102,ppid:100,rss_kib:256,start_identity:'Wed Sep 17 00:00:03 2026',command});
  sample.process_info.push({id:102,type:'unzip.mojom.Unzipper',cpuTime:1});
  sample.raw_ps=raw(sample.rows);
}
function reject(name, mutate, error) {
  test(name,()=>{const lane=fixture();mutate(lane);const result=evaluate(lane);
    assert.equal(result.metrics_valid,false,JSON.stringify(result));assert.equal(result.budgets_passed,null);
    assert(result.errors.some(e=>e.includes(error)),JSON.stringify(result.errors));
    assert.equal(result.heap.usedSize.growth_bytes,null);
  });
}

test('valid v2 lane reports independent domains, deduplicated shared isolate, and separate RSS',()=>{
  const result=evaluate(fixture());
  assert.equal(result.metrics_valid,true,JSON.stringify(result.errors));
  assert.equal(result.budgets_passed,true);
  assert.deepEqual(result.heap.usedSize,{baseline_bytes:1024,final_bytes:2048,growth_bytes:1024});
  assert.equal(result.heap.embedderHeapUsedSize.growth_bytes,0);
  assert.equal(result.heap.backingStorageSize.growth_bytes,0);
  assert.equal(result.dom.baseline.length,2);
  assert.equal(result.browser_cost.renderer_rss_bytes.max,1024*1024);
  assert.equal(result.browser_cost.nonrenderer_rss_bytes.max,1000*1024);
  assert.equal(result.browser_cost.total_rss_bytes.max,2024*1024);
  assert.equal(result.browser_cost.total_rss_bytes.growth,0);
  assert.equal(result.browser_cost.server_rss_bytes.max,512*1024);
  assert.equal(result.browser_cost.max_observed_gap_ms,100);
  assert.equal(result.memory_complete,false);assert.equal(result.full_contract_complete,false);
  assert.equal(result.attribution,'not-verified');
});

for(const domain of ['usedSize','embedderHeapUsedSize','backingStorageSize']) {
  for(const extra of [0,1]) test(`${domain} growth exactly 4MiB ${extra?'plus one fails':'passes'}`,()=>{
    const lane=fixture(),base=lane.baseline.isolates[0].raw_heap[domain];
    lane.final.isolates[0].raw_heap[domain]=base+4*MIB+extra;
    if(domain==='usedSize') {lane.final.isolates[0].used_bytes=base+4*MIB+extra;lane.final.isolates[0].total_bytes=lane.final.isolates[0].used_bytes+1024;lane.final.isolates[0].raw_heap.totalSize=lane.final.isolates[0].total_bytes;}
    const result=evaluate(lane);assert.equal(result.metrics_valid,true,JSON.stringify(result.errors));
    assert.equal(result.heap[domain].growth_bytes,4*MIB+extra);
    assert.equal(result.budgets_passed,!extra);
  });
}
test('total Chrome RSS above v1 renderer limit is reported but not a v2 heap-budget failure',()=>{
  const lane=fixture();for(const sample of lane.samples){sample.rows=rows(300*1024);sample.raw_ps=raw(sample.rows);}
  const result=evaluate(lane);assert.equal(result.metrics_valid,true,JSON.stringify(result.errors));assert.equal(result.budgets_passed,true);
  assert(result.browser_cost.total_rss_bytes.max>256*MIB);
});
reject('wrong contract id refuses v1 reuse',lane=>{lane.contract_id='appkit-theme-guide-memory-v1';},'contract_id');
reject('missing raw backing field is not zero',lane=>{delete lane.final.isolates[0].raw_heap.backingStorageSize;},'raw backingStorageSize');
reject('unsafe integer field is not rounded',lane=>{lane.final.isolates[0].raw_heap.embedderHeapUsedSize=Number.MAX_SAFE_INTEGER+1;},'raw embedderHeapUsedSize');
reject('duplicate isolate is not double counted',lane=>{lane.final.isolates.push(structuredClone(lane.final.isolates[0]));},'unique isolates');
reject('shared session assigned to second isolate is refused',lane=>{lane.final.isolates.push({...structuredClone(lane.final.isolates[0]),id:'other'});},'isolate session coverage');
reject('target must bind a raw isolate/session',lane=>{lane.final.targets[1].isolate_id='missing';},'target/isolate binding');
test('native target/session replacement across endpoints is allowed',()=>{
  const lane=fixture();const result=evaluate(lane);assert.equal(result.metrics_valid,true,JSON.stringify(result.errors));
  assert.notEqual(lane.baseline.targets[0].target_id,lane.final.targets[0].target_id);
});
test('fractional monotonic clock timestamps remain valid',()=>{
  const lane=fixture();
  for(const sample of lane.samples){sample.started_ms+=.25;sample.completed_ms+=.25;}
  lane.work_started_ms+=.25;lane.work_completed_ms+=.25;
  for(const cycle of lane.cycles){cycle.started_ms+=.25;cycle.completed_ms+=.25;}
  for(const endpoint of [lane.baseline,lane.final]){
    endpoint.started_ms+=.25;endpoint.completed_ms+=.25;
    endpoint.idle.started_ms+=.25;endpoint.idle.completed_ms+=.25;
    for(const gc of endpoint.gc){gc.started_ms+=.25;gc.completed_ms+=.25;}
  }
  const result=evaluate(lane);assert.equal(result.metrics_valid,true,JSON.stringify(result.errors));
});
test('exact guide preview path permits native selector query',()=>{
  const lane=fixture();
  for(const endpoint of [lane.baseline,lane.final])endpoint.targets[1].url=origin+'/theme-lab/preview?scheme=light&specimen=dashboard';
  const result=evaluate(lane);assert.equal(result.metrics_valid,true,JSON.stringify(result.errors));
});
test('IPv6 exact HTTP loopback origin is allowed',()=>{
  const lane=fixture(),ipv6='http://[::1]:18766';
  for(const endpoint of [lane.baseline,lane.final])for(const target of endpoint.targets)target.url=target.url.replace(origin,ipv6);
  const result=evaluateMemoryV2Metrics(lane,{...owners,origin:ipv6});
  assert.equal(result.metrics_valid,true,JSON.stringify(result.errors));
});
test('nonlocal and inexact origins are refused',()=>{
  for(const value of ['https://example.test:18766','http://example.test:18766','http://127.0.0.1:80','http://127.0.0.1:18766/path','http://127.0.0.1:18766?x=1']){
    const result=evaluateMemoryV2Metrics(fixture(),{...owners,origin:value});
    assert.equal(result.metrics_valid,false,value);
    assert(result.errors.includes('exact origin'),value);
  }
});
reject('wrong page URL',lane=>{lane.final.targets[0].url=origin+'/theme-lab-evil';},'target origin/path/type');
reject('wrong iframe origin',lane=>{lane.final.targets[1].url='https://example.test/theme-lab/preview';},'target origin/path/type');
reject('failed cycle',lane=>{lane.cycles[7].ok=false;},'cycle 8 failed');
reject('overlapping cycles',lane=>{lane.cycles[7].started_ms=lane.cycles[6].started_ms;},'cycle 8 non-overlapping order');
reject('missing sample',lane=>{lane.samples.splice(60,3);},'inter-completion gap');
reject('slow acquisition',lane=>{lane.samples[30].completed_ms=lane.samples[30].started_ms+201;},'acquisition gap');
reject('invalid raw ps',lane=>{lane.samples[0].raw_ps+='bad\n';},'unparseable raw_ps');
reject('raw ps and rows must agree',lane=>{lane.samples[0].rows[1].rss_kib++;},'raw_ps/rows mismatch');
reject('reused PID',lane=>{lane.samples[1].rows[1].start_identity='Thu Sep 18 00:00:01 2026';lane.samples[1].raw_ps=raw(lane.samples[1].rows);},'reused PID');
reject('nonowned renderer',lane=>{lane.samples[0].rows[1].ppid=999;lane.samples[0].raw_ps=raw(lane.samples[0].rows);},'nonowned process');
reject('misclassified renderer',lane=>{lane.samples[0].rows[1].command='/Chrome';lane.samples[0].raw_ps=raw(lane.samples[0].rows);},'renderer classification');
for(const [kind,mutate] of [
  ['renderer',sample=>{sample.rows[1].command='(Google Chrome He)';}],
  ['utility',sample=>addUtility(sample,'(Google Chrome He)')],
  ['browser',sample=>{sample.rows[0].command='(Google Chrome He)';}],
  ['server',sample=>{sample.rows.at(-1).command='(Google Chrome He)';}],
]) reject(`parenthesized ${kind} command is unavailable`,lane=>{mutate(lane.samples[0]);lane.samples[0].raw_ps=raw(lane.samples[0].rows);},'process command unavailable');
reject('owned child without an explicit process type is unavailable',lane=>{
  addUtility(lane.samples[0],'/Chrome Helper');
},'process type unavailable');
reject('owned child with multiple process types is unavailable',lane=>{
  addUtility(lane.samples[0],'/Chrome Helper --type=utility --type=gpu-process');
},'process type unavailable');
test('full utility process type remains independently classifiable',()=>{
  const lane=fixture();addUtility(lane.samples[0]);
  const result=evaluate(lane);assert.equal(result.metrics_valid,true,JSON.stringify(result.errors));assert.equal(result.budgets_passed,true);
});
test('full renderer executable name remains independently classifiable',()=>{
  const lane=fixture();lane.samples[0].rows[1].command='/Google Chrome Helper (Renderer) --type=renderer';lane.samples[0].raw_ps=raw(lane.samples[0].rows);
  const result=evaluate(lane);assert.equal(result.metrics_valid,true,JSON.stringify(result.errors));assert.equal(result.budgets_passed,true);
});
reject('renderer text inside another argument is not the process type',lane=>{
  lane.samples[0].rows[1].command='/Chrome --type=utility --value=--type=renderer';
  lane.samples[0].raw_ps=raw(lane.samples[0].rows);
},'renderer classification');
reject('renderer-prefixed process type is not an exact renderer',lane=>{
  lane.samples[0].rows[1].command='/Chrome --type=renderer-other';
  lane.samples[0].raw_ps=raw(lane.samples[0].rows);
},'renderer classification');
test('explicit utility type wins over unrelated renderer text',()=>{
  const lane=fixture();addUtility(lane.samples[0],'/Chrome --type=utility --value=--type=renderer');
  const result=evaluate(lane);assert.equal(result.metrics_valid,true,JSON.stringify(result.errors));
  assert.equal(result.budgets_passed,true);
});
reject('child cannot masquerade as browser',lane=>{lane.samples[0].process_info[1].type='browser';},'duplicate browser classification');
reject('missing SystemInfo PID',lane=>{lane.samples[0].process_info.pop();},'unknown ps process');
reject('missing server row',lane=>{lane.samples[0].rows.pop();lane.samples[0].raw_ps=raw(lane.samples[0].rows);},'server ps identity');
reject('bad GC',lane=>{lane.final.gc[0].ok=false;},'GC result/order');
reject('duplicate GC on shared isolate',lane=>{lane.final.gc.push(structuredClone(lane.final.gc[0]));},'GC each dedup isolate');
reject('short idle',lane=>{lane.final.idle.completed_ms=12000;},'five second idle');
reject('network activity in idle',lane=>{lane.final.idle.network_after++;},'idle network stable');
reject('target replacement within idle is not stable',lane=>{lane.final.idle.targets_after[0]='replacement';},'idle targets stable');
reject('null target is a refusal not an exception',lane=>{lane.final.targets[1]=null;},'guide topology');
reject('null GC is a refusal not an exception',lane=>{lane.final.gc[0]=null;},'GC result/order');
reject('nonarray isolate sessions are refused',lane=>{lane.final.isolates[0].session_ids={0:'fps'};},'isolate session coverage');
reject('future endpoint sample is refused',lane=>{lane.final.completed_ms=12114;},'endpoint sample timing');
reject('missing per-session DOM',lane=>{lane.final.dom.pop();},'DOM sessions');
reject('invalid DOM count',lane=>{lane.final.dom[0].nodes=NaN;},'DOM nodes');
