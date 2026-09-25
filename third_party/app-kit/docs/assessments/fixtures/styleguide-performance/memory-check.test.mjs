import test from 'node:test';
import assert from 'node:assert/strict';
import {checkMemoryLane} from './memory-check.mjs';

const starts={browser:'Wed Sep 17 00:00:00 2026',renderer:'Wed Sep 17 00:00:01 2026',server:'Wed Sep 17 00:00:02 2026'};
const owners={browser:{pid:100,start_identity:starts.browser},server:{pid:200,start_identity:starts.server}};
const raw=rows=>rows.map(r=>`${r.pid} ${r.ppid} ${r.rss_kib} ${r.start_identity} ${r.command}`).join('\n')+'\n';
function rows(rss=1024){return[{pid:100,ppid:1,rss_kib:1000,start_identity:starts.browser,command:'/Chrome'},{pid:101,ppid:100,rss_kib:rss,start_identity:starts.renderer,command:'/Chrome --type=renderer'},{pid:200,ppid:1,rss_kib:512,start_identity:starts.server,command:'/server'}];}
function endpoint(prefix,start,idleStart,idleEnd,completed,sampleIndex,used=1024){return{started_ms:start,completed_ms:completed,idle:{started_ms:idleStart,completed_ms:idleEnd,network_before:10,network_after:10,targets_before:[prefix+'p',prefix+'i'],targets_after:[prefix+'p',prefix+'i']},targets:[{target_id:prefix+'p',session_id:prefix+'ps',type:'page',url:'http://127.0.0.1:18766/theme-lab',isolate_id:prefix+'iso'},{target_id:prefix+'i',session_id:prefix+'is',type:'iframe',url:'http://127.0.0.1:18766/theme-lab/preview',isolate_id:prefix+'iso'}],isolates:[{id:prefix+'iso',session_ids:[prefix+'ps',prefix+'is'],used_bytes:used,total_bytes:used+1024,raw_heap:{usedSize:used,totalSize:used+1024}}],gc:[{isolate_id:prefix+'iso',session_id:prefix+'ps',started_ms:start,completed_ms:idleStart,ok:true}],dom:[{session_id:prefix+'ps',documents:1,nodes:10,jsEventListeners:0},{session_id:prefix+'is',documents:1,nodes:10,jsEventListeners:0}],sample_index:sampleIndex};}
function fixture(){
  const samples=[];for(let started=10;started<=12110;started+=100){const rs=rows();samples.push({started_ms:started,completed_ms:started+5,process_info:[{id:100,type:'browser',cpuTime:1},{id:101,type:'renderer',cpuTime:1}],raw_ps:raw(rs),rows:rs,errors:[]});}
  const lane={id:'memory-v1',errors:[],cycles:[],work_started_ms:5100,work_completed_ms:7100,samples};for(let i=1;i<=20;i++)lane.cycles.push({index:i,started_ms:5000+i*100,completed_ms:5100+i*100,ok:true});lane.baseline=endpoint('b',0,10,5010,5015,50,1024);lane.final=endpoint('f',7100,7110,12110,12115,121,2048);return lane;
}
const check=lane=>checkMemoryLane(lane,{...owners,origin:'http://127.0.0.1:18766'});
const reject=(name,mutate,want)=>test('rejects '+name,()=>{const lane=fixture();mutate(lane);const out=check(lane);assert.equal(out.ok,false);assert(out.errors.some(x=>x.includes(want)),JSON.stringify(out.errors));});

test('accepts exact 20-cycle owned-renderer memory lane',()=>{const out=check(fixture());assert.equal(out.ok,true,JSON.stringify(out.errors));assert.equal(out.complete,false);assert.equal(out.max_gap_ms,100);assert.equal(out.heap_growth_bytes,1024);});
reject('19 cycles',lane=>lane.cycles.pop(),'exactly 20 cycles');
reject('21 cycles',lane=>lane.cycles.push({index:21,started_ms:7100,completed_ms:7100,ok:true}),'exactly 20 cycles');
reject('failed cycle',lane=>lane.cycles[4].ok=false,'cycle 5 failed');
reject('lane errors',lane=>lane.errors.push('sampler failed'),'lane errors');
reject('sampling gap',lane=>lane.samples.splice(60,2),'sample gap');
reject('missing SystemInfo PID row',lane=>{lane.samples[0].rows.splice(1,1);lane.samples[0].raw_ps=raw(lane.samples[0].rows);},'SystemInfo PID missing ps row');
reject('reused PID',lane=>{lane.samples[1].rows[1].start_identity='Thu Sep 18 00:00:01 2026';lane.samples[1].raw_ps=raw(lane.samples[1].rows);},'reused PID');
reject('nonowned renderer',lane=>{lane.samples[0].rows[1].ppid=999;lane.samples[0].raw_ps=raw(lane.samples[0].rows);},'nonowned process');
reject('misclassified renderer',lane=>{lane.samples[0].rows[1].command='/Chrome';lane.samples[0].raw_ps=raw(lane.samples[0].rows);},'renderer command classification');
reject('missing renderer',lane=>lane.samples[0].process_info[1].type='utility','missing renderer');
reject('duplicate isolate',lane=>lane.final.isolates.push(structuredClone(lane.final.isolates[0])),'duplicate/invalid isolate');
reject('unbound raw heap',lane=>lane.final.isolates[0].raw_heap.usedSize++,'raw heap binding');
reject('bad GC',lane=>lane.final.gc[0].ok=false,'GC result/order');
reject('short idle',lane=>lane.final.idle.completed_ms=12000,'five second idle');
reject('idle network',lane=>lane.final.idle.network_after++,'idle network changed');
reject('extra frame',lane=>{lane.final.targets.push({...lane.final.targets[1],target_id:'fi2',session_id:'fis2'});lane.final.idle.targets_before.push('fi2');lane.final.idle.targets_after.push('fi2');},'guide topology');
reject('prefix-confusable target URL',lane=>lane.final.targets.find(x=>x.type==='page').url='http://127.0.0.1:18766/theme-lab-evil','target origin/path/type');
reject('invalid metric',lane=>lane.samples[0].rows[1].rss_kib=NaN,'invalid ps row');
reject('null row',lane=>{lane.samples[0].rows[1]=null;},'invalid ps row');
reject('renderer RSS budget',lane=>{lane.samples[60].rows=rows(262145);lane.samples[60].raw_ps=raw(lane.samples[60].rows);},'renderer RSS budget');
reject('heap growth budget',lane=>{lane.final.isolates[0].used_bytes=5*1024*1024+1024;lane.final.isolates[0].total_bytes=lane.final.isolates[0].used_bytes+1;},'retained heap growth budget');
reject('RSS growth budget',lane=>{const i=lane.final.sample_index;lane.samples[i].rows=rows(33793);lane.samples[i].raw_ps=raw(lane.samples[i].rows);},'retained renderer RSS growth budget');
reject('raw ps mismatch',lane=>lane.samples[0].raw_ps+='bad\n','unparseable raw_ps');
reject('missing browser owner',lane=>lane.samples[0].process_info=lane.samples[0].process_info.filter(x=>x.type!=='browser'),'browser SystemInfo identity');
reject('missing server row',lane=>{lane.samples[0].rows.pop();lane.samples[0].raw_ps=raw(lane.samples[0].rows);},'server ps identity');
