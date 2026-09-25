import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import test from 'node:test';
import {auditReport,candidateExpected,evaluateState} from './oracle.mjs';

const cases=['missing-required','missing-custom','missing-optional','empty-duplicate','known','native-default'];
const lanes=['wide-light','narrow-dark'];
const makeReport=variant=>({variant,observations:lanes.flatMap(lane=>cases.map(id=>({lane,case:id,expected:candidateExpected(id),trace:['initial','reset','weekly'].map(step=>({step,state:evaluateState(variant,id,step,{}).expected}))})))});

test('accepts exact historical baseline and exact candidate states',()=>{
  assert.deepEqual(auditReport(makeReport('baseline')),{variant:'baseline',cells:12,states:36});
  assert.deepEqual(auditReport(makeReport('candidate')),{variant:'candidate',cells:12,states:36});
});
test('rejects an arbitrary baseline mismatch rather than counting any difference',()=>{
  const report=makeReport('baseline');report.observations[0].trace[0].state.prompt='arbitrary';
  assert.throws(()=>auditReport(report),/state mismatch/);
});
test('rejects candidate regression and preserves known/native-default controls',()=>{
  const report=makeReport('candidate');report.observations[0].trace[1].state.valid=true;
  assert.throws(()=>auditReport(report),/state mismatch/);
  for(const id of ['known','native-default'])for(const phase of ['initial','reset'])assert(evaluateState('baseline',id,phase,candidateExpected(id)).ok);
});

const supplied=[...(process.env.SELECT_STATE_REPORTS?.split(path.delimiter)??[]),...process.argv.slice(2).filter(x=>x.startsWith('/'))].filter(Boolean);
for(const reportPath of supplied)test(`audits retained report ${reportPath}`,async()=>{
  assert(path.isAbsolute(reportPath),'retained report path must be absolute');
  const report=JSON.parse(await fs.readFile(reportPath,'utf8'));assert.equal(auditReport(report).cells,12);
  const mutated=structuredClone(report);mutated.observations[0].trace[0].state.prompt+=' mutation';
  assert.throws(()=>auditReport(mutated),/state mismatch/);
});
