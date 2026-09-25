// Thin launcher: retain the import graph before Node executes the benchmark.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';

const args=process.argv.slice(2),[probe,reference,parentArg,mode='none',candidateReceipt,candidateSHA]=args;
const candidate=mode==='--font-candidate',canary=candidate?'none':mode;
assert.ok([probe,reference,parentArg].every(p=>p&&path.isAbsolute(p))&&
  (candidate?args.length===6&&path.isAbsolute(candidateReceipt)&&/^[a-f0-9]{64}$/.test(candidateSHA):args.length>=3&&args.length<=4&&['none','missing-font','delayed-css','oversize-image'].includes(canary)),
  'usage: node performance.mjs ABS_PRIVATE_PROBE ABS_BROWSER_RECEIPT ABS_PRIVATE_OUTPUT_PARENT [missing-font|delayed-css|oversize-image | --font-candidate ABS_FONT_PROOF_RECEIPT EXPECTED_SHA256]');
const parent=await fs.realpath(parentArg);assert.ok((await fs.stat(parent)).isDirectory());
const root=await fs.mkdtemp(path.join(parent,'hero-performance-'));await fs.chmod(root,0o700);
const destination=path.join(root,'runner');await fs.mkdir(destination,{mode:0o700});
console.log('Private performance evidence: '+root);
const dir=path.dirname(fileURLToPath(import.meta.url));
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
const write=(file,bytes)=>fs.writeFile(file,bytes,{flag:'wx',mode:0o600});
const snapshot={schema:1,createdAt:new Date().toISOString(),node:process.version,nodeExecutable:process.execPath,args,inputs:[]};
for(const name of ['performance.mjs','performance-run.mjs','performance-oracle.mjs','performance_test.mjs','performance-contract.json','font-delivery.mjs','font-delivery_test.mjs','font-painted.mjs','font-painted_test.mjs']){
  const source=path.join(dir,name),stat=await fs.lstat(source);
  assert.ok(stat.isFile()&&!stat.isSymbolicLink()&&stat.size<1024*1024,'bounded real runner input: '+name);
  const bytes=await fs.readFile(source);await write(path.join(destination,name),bytes);
  snapshot.inputs.push({name,source,bytes:bytes.length,sha256:sha(bytes)});
}
await write(path.join(root,'snapshot.json'),JSON.stringify(snapshot,null,2)+'\n');
const report={schema:1,root,snapshotSHA:sha(await fs.readFile(path.join(root,'snapshot.json'))),commands:[],failures:[]};
async function run(name,argv,timeoutMs){
  const startedAt=new Date().toISOString();
  const child=spawn(process.execPath,argv,{cwd:destination,env:{...process.env,APPKIT_HERO_PERF_RUN_ROOT:root},stdio:['ignore','pipe','pipe']});
  const record={name,argv,startedAt,pid:child.pid,timeoutMs};let stdout='',stderr='',forced=false,killTimer;
  const abort=()=>{if(forced)return;forced=true;child.kill('SIGTERM');killTimer=setTimeout(()=>child.kill('SIGKILL'),25000)};
  const timer=setTimeout(abort,timeoutMs);
  child.stdout.on('data',b=>{stdout+=b;if(stdout.length>1024*1024)abort()});
  child.stderr.on('data',b=>{stderr+=b;if(stderr.length>1024*1024)abort()});
  await new Promise(resolve=>{child.once('error',error=>{record.error=String(error)});child.once('close',(code,signal)=>{record.exitCode=code;record.signal=signal;resolve()})});
  clearTimeout(timer);clearTimeout(killTimer);
  Object.assign(record,{finishedAt:new Date().toISOString(),forced,stdoutSHA:sha(stdout),stderrSHA:sha(stderr)});
  await write(path.join(root,name+'.stdout.log'),stdout);await write(path.join(root,name+'.stderr.log'),stderr);report.commands.push(record);
  // A forced stop is never a cleanup pass; descendants need separate inspection.
  if(forced||record.error||record.signal)report.failures.push(name+': abnormal stop; owned descendants not verified');
  console.log(name+': exit='+record.exitCode+(forced?' forced':''));
  return record.exitCode===0&&!forced&&!record.error&&!record.signal;
}
try{
  if(await run('oracle-tests',['--test',path.join(destination,'performance_test.mjs'),path.join(destination,'font-delivery_test.mjs'),path.join(destination,'font-painted_test.mjs')],15000)){
    await run('benchmark',[path.join(destination,'performance-run.mjs'),...args],150000);
  }else report.failures.push('oracle tests failed; browser execution refused');
  for(const input of snapshot.inputs){
    assert.equal(sha(await fs.readFile(path.join(destination,input.name))),input.sha256,'snapshot drift: '+input.name);
    assert.equal(sha(await fs.readFile(input.source)),input.sha256,'source drift: '+input.name);
  }
}catch(error){report.failures.push(String(error.stack||error))}
report.finishedAt=new Date().toISOString();
report.passed=report.failures.length===0&&report.commands.length===2&&report.commands.every(c=>c.exitCode===0&&!c.forced&&!c.signal);
await write(path.join(root,'launcher-receipt.json'),JSON.stringify(report,null,2)+'\n');
console.log(JSON.stringify({root,passed:report.passed,failures:report.failures}));process.exitCode=report.passed?0:1;
