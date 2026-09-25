// Synthetic CLIs only. Never point these tests at an installed/fleet binary.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import {createHash} from 'node:crypto';
import {prepareTrial,executeTrial,verifyInputs,readRegular,validateProbeReport} from './run.mjs';

async function fixture(code=2){
  const root=await fs.realpath(await fs.mkdtemp(path.join(os.tmpdir(),'appkit-cli-owned-test-')));
  const binary=path.join(root,'synthetic-cli'),output=path.join(root,'trial');
  await fs.writeFile(binary,`#!/bin/sh
case "$1" in
  --help) printf 'Synthetic CLI help only\\n' ;;
  --version) printf 'synthetic v0.1.0-dev\\n' ;;
  --appkit-probe-unknown-flag|appkit-probe-unknown-command) printf 'Synthetic invocation refused\\n' >&2; exit ${code} ;;
  *) exit 99 ;;
esac
`,{flag:'wx',mode:0o700});
  await fs.mkdir(output,{mode:0o700});
  return {root,binary,output};
}

test('input staging rejects unsafe/unbounded sources before creating trial contents',async()=>{
  const f=await fixture();
  await assert.rejects(readRegular(f.binary,false,3),/bounded regular/);
  const link=path.join(f.root,'link');await fs.symlink(f.binary,link);
  await assert.rejects(prepareTrial(link,f.binary,f.output));
  assert.deepEqual(await fs.readdir(f.output),[]);
  await fs.chmod(f.output,0o755);
  await assert.rejects(prepareTrial(f.binary,f.binary,f.output),/private owned/);
  assert.deepEqual(await fs.readdir(f.output),[]);
});

test('retained tampering refuses execution and retains failure evidence',async()=>{
  const f=await fixture(),plan=await prepareTrial(f.binary,f.binary,f.output);
  const input=plan.inputs.find(i=>i.name==='bosun');
  await fs.chmod(input.executed,0o700);await fs.writeFile(input.executed,'#!/bin/sh\nexit 99\n');
  const checks=await verifyInputs(plan);assert.equal(checks.find(c=>c.name==='bosun').passed,false);
  const result=await executeTrial(plan);
  assert.equal(result.passed,false);assert.equal(result.started,false);assert.equal(result.pid,undefined);
  assert.deepEqual(await fs.readdir(plan.results),[]);
  assert.equal(JSON.parse(await fs.readFile(path.join(f.output,'run-receipt.json'))).started,false);
});

test('actual sandbox probe runs retained synthetic bytes even if originals change',{skip:process.platform!=='darwin'},async t=>{
  const f=await fixture(),plan=await prepareTrial(f.binary,f.binary,f.output);
  await fs.writeFile(f.binary,'#!/bin/sh\nprintf "mutable original must never run" >&2\nexit 99\n');
  const result=await executeTrial(plan);
  assert.equal(result.passed,true,JSON.stringify(result));
  assert.equal(result.probe.checks,13);assert.deepEqual(result.probe.failed,[]);
  assert.ok(result.before.every(c=>c.passed)&&result.after.every(c=>c.passed));
  for(const stream of ['stdout','stderr']){
    const bytes=await fs.readFile(path.join(f.output,'probe.'+stream));
    assert.equal(result.logs[stream].capturedBytes,bytes.length);
    assert.equal(result.logs[stream].sha256,createHash('sha256').update(bytes).digest('hex'));
    assert.equal(result.logs[stream].limited,false);
  }
  const receipt=JSON.parse(await fs.readFile(path.join(plan.results,'checks/report.json')));
  assert.equal(receipt.runs.length,8);assert.ok(receipt.runs.every(r=>r.passed));
  assert.ok(receipt.runs.filter(r=>r.id.includes('unknown')).every(r=>r.dimensions.reporting.passed&&r.dimensions.exit_policy.passed));
  for(const alter of [r=>r.schema_version=2,r=>r.kind='different',r=>r.canaries_passed=false,r=>r.checks=[],r=>r.checks.pop(),r=>r.checks[1]=r.checks[0],r=>r.runs=[],r=>r.runs[0].binary_sha256='wrong',r=>r.runs[0].args=['operational-command'],r=>r.status='failed',r=>r.runs[0].passed=false]){
    const changed=structuredClone(receipt);alter(changed);assert.throws(()=>validateProbeReport(changed,plan));
  }
  t.diagnostic('Retained synthetic-only trial: '+f.output);
});

test('useful synthetic diagnostics do not turn usage exit1 into acceptance',{skip:process.platform!=='darwin'},async t=>{
  const f=await fixture(1),plan=await prepareTrial(f.binary,f.binary,f.output),result=await executeTrial(plan);
  assert.equal(result.passed,false);assert.equal(result.exit.code,1);
  assert.equal(result.probe.failed.length,4);assert.deepEqual(result.errors,[]);
  const receipt=JSON.parse(await fs.readFile(path.join(plan.results,'checks/report.json')));
  const usage=receipt.runs.filter(r=>r.id.includes('unknown'));
  assert.equal(usage.length,4);assert.ok(usage.every(r=>!r.passed&&r.dimensions.reporting.passed&&!r.dimensions.exit_policy.passed));
  t.diagnostic('Retained intentional usage2 failure: '+f.output);
});

test('owned process-group timeout retains failure instead of hanging',{skip:process.platform!=='darwin'},async t=>{
  const f=await fixture(),plan=await prepareTrial(f.binary,f.binary,f.output);
  // Deliberately shorter than interpreter startup; no consumer or leaf executes.
  plan.bounds.timeoutMS=1;
  const result=await executeTrial(plan);
  assert.equal(result.passed,false);assert.equal(result.timedOut,true);
  assert.equal(result.exit.signal,'SIGKILL');
  assert.throws(()=>process.kill(result.pid,0),{code:'ESRCH'});
  t.diagnostic('Retained timeout negative: '+f.output);
});

test('probe output is capped and receipt-bound on overflow',{skip:process.platform!=='darwin'},async t=>{
  const f=await fixture(),plan=await prepareTrial(f.binary,f.binary,f.output);
  plan.bounds.outputBytesPerStream=1;
  const result=await executeTrial(plan);
  assert.equal(result.passed,false);assert.equal(result.logs.stdout.limited,true);
  assert.equal(result.logs.stdout.capturedBytes,1);assert.ok(result.logs.stdout.observedBytes>1);
  assert.equal(result.errors.filter(e=>e==='stdout limit exceeded').length,1);
  assert.equal((await fs.readFile(path.join(f.output,'probe.stdout'))).length,1);
  t.diagnostic('Retained output-bound negative: '+f.output);
});
