import test from 'node:test';
import assert from 'node:assert/strict';
import {setImmediate as nextTurn} from 'node:timers/promises';
import {waitForPhase} from './phase_wait.mjs';

const response=()=>({method:'GET',path:'/asset.css',finished_ms:1,response_body_bytes:10,read_error:null});
const fixture=()=>({requests:[{method:'GET',path:'/asset.css'}],responses:[],pending:0});
const run=(state,options={})=>waitForPhase({inspect:()=>state,expected:['GET /asset.css'],...options});

test('does not finish before a delayed response has appeared',async()=>{
  const state=fixture();let done=false;
  const waiting=run(state).then(()=>{done=true;});await nextTurn();assert.equal(done,false);
  state.responses.push(response());await waiting;assert.equal(done,true);
});
test('waits for the body and pending acquisition even after headers arrive',async()=>{
  const state=fixture();state.responses.push({...response(),finished_ms:null,response_body_bytes:null});state.pending=1;let done=false;
  const waiting=run(state).then(()=>{done=true;});await nextTurn();assert.equal(done,false);
  Object.assign(state.responses[0],response());await nextTurn();assert.equal(done,false);
  state.pending=0;await waiting;assert.equal(done,true);
});
test('missing response has a failure deadline',async()=>{await assert.rejects(run(fixture(),{timeoutMs:15}),/deadline exceeded/);});
test('missing body has a failure deadline',async()=>{const s=fixture();s.responses.push({...response(),finished_ms:null});await assert.rejects(run(s,{timeoutMs:15}),/deadline exceeded/);});
test('read failure cannot become completion',async()=>{const s=fixture();s.responses.push({...response(),read_error:'synthetic unreadable body'});await assert.rejects(run(s),/acquisition failed/);});
test('transfer failure cannot become completion',async()=>{const s=fixture();s.responses.push({...response(),sizes_error:'synthetic missing sizes'});await assert.rejects(run(s),/acquisition failed/);});
test('unexpected duplicate traffic fails',async()=>{const s=fixture();s.requests.push({...s.requests[0]});await assert.rejects(run(s),/Unexpected phase traffic/);});
test('wrong response closure fails',async()=>{const s=fixture();s.responses.push({...response(),path:'/other.css'});await assert.rejects(run(s),/closure differs/);});
test('nonfinite completion cannot pass',async()=>{const s=fixture();s.responses.push({...response(),finished_ms:NaN});await assert.rejects(run(s,{timeoutMs:15}),/deadline exceeded/);});
test('empty body cannot stand in for acquisition',async()=>{const s=fixture();s.responses.push({...response(),response_body_bytes:0});await assert.rejects(run(s,{timeoutMs:15}),/deadline exceeded/);});
