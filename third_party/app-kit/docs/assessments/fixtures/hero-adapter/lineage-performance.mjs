// Separate prospective v2 launcher. V1 sources and failed receipts stay untouched.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';

const args=process.argv.slice(2),[candidate,original,parentArg,mode='matrix']=args;
assert.ok(args.length>=3&&args.length<=4&&[candidate,original,parentArg].every(p=>p&&path.isAbsolute(p))&&['matrix','cache-clear','representation-policy','original-control'].includes(mode),
  'usage: node lineage-performance.mjs ABS_RETAINED_CANDIDATE_RECEIPT ABS_ORIGINAL_CONTROL_RECEIPT ABS_PRIVATE_PARENT [matrix|cache-clear|representation-policy|original-control]');
const parent=await fs.realpath(parentArg);assert.ok((await fs.stat(parent)).isDirectory());
const root=await fs.mkdtemp(path.join(parent,'hero-lineage-'));await fs.chmod(root,0o700);
const runner=path.join(root,'runner');await fs.mkdir(runner,{mode:0o700});
const sourceDir=path.dirname(fileURLToPath(import.meta.url));
const sha=b=>createHash('sha256').update(b).digest('hex');
const write=(p,b)=>fs.writeFile(p,b,{flag:'wx',mode:0o600});
const snapshot={schema:1,args,node:process.version,nodeExecutable:process.execPath,createdAt:new Date().toISOString(),inputs:[]};
console.log('Private prospective v2 evidence: '+root);
for(const name of ['lineage-performance.mjs','lineage-run.mjs','lineage-oracle.mjs','lineage-oracle_test.mjs','cache-lineage.mjs','cache-lineage_test.mjs','cache-lineage-contract.json','performance-contract.json','performance-oracle.mjs','performance_test.mjs','font-painted.mjs','font-painted_test.mjs']){
  const source=path.join(sourceDir,name),stat=await fs.lstat(source);
  assert.ok(stat.isFile()&&!stat.isSymbolicLink()&&stat.size<1024*1024,'bounded real runner source: '+name);
  const bytes=await fs.readFile(source);await write(path.join(runner,name),bytes);
  snapshot.inputs.push({name,source,bytes:bytes.length,sha256:sha(bytes)});
}
await write(path.join(root,'snapshot.json'),JSON.stringify(snapshot,null,2)+'\n');
const report={schema:1,root,snapshotSHA:sha(await fs.readFile(path.join(root,'snapshot.json'))),commands:[],failures:[]};
async function run(name,argv,timeoutMs){
  const child=spawn(process.execPath,argv,{cwd:runner,env:{...process.env,APPKIT_LINEAGE_RUN_ROOT:root},stdio:['ignore','pipe','pipe']});
  const record={name,argv,pid:child.pid,timeoutMs,startedAt:new Date().toISOString()};let stdout='',stderr='',forced=false,killTimer;
  const abort=()=>{if(forced)return;forced=true;child.kill('SIGTERM');killTimer=setTimeout(()=>child.kill('SIGKILL'),25000)};
  const timer=setTimeout(abort,timeoutMs);
  child.stdout.on('data',b=>{stdout+=b;if(stdout.length>1024*1024)abort()});
  child.stderr.on('data',b=>{stderr+=b;if(stderr.length>1024*1024)abort()});
  await new Promise(resolve=>{child.once('error',e=>{record.error=String(e)});child.once('close',(code,signal)=>{record.exitCode=code;record.signal=signal;resolve()})});
  clearTimeout(timer);clearTimeout(killTimer);
  Object.assign(record,{finishedAt:new Date().toISOString(),forced,stdoutSHA:sha(stdout),stderrSHA:sha(stderr)});
  await write(path.join(root,name+'.stdout.log'),stdout);await write(path.join(root,name+'.stderr.log'),stderr);report.commands.push(record);
  if(forced||record.error||record.signal)report.failures.push(name+': abnormal stop; owned descendants not verified');
  console.log(name+': exit='+record.exitCode+(forced?' forced':''));
  return record.exitCode===0&&!forced&&!record.error&&!record.signal;
}
try{
  if(await run('oracle-tests',['--test',...['cache-lineage_test.mjs','lineage-oracle_test.mjs','performance_test.mjs','font-painted_test.mjs'].map(n=>path.join(runner,n))],15000))
    await run('collection',[path.join(runner,'lineage-run.mjs'),...args],150000);
  else report.failures.push('refusal tests failed; browser execution refused');
  for(const input of snapshot.inputs){
    assert.equal(sha(await fs.readFile(path.join(runner,input.name))),input.sha256,'executed snapshot drift: '+input.name);
    assert.equal(sha(await fs.readFile(input.source)),input.sha256,'maintained source drift: '+input.name);
  }
}catch(e){report.failures.push(String(e.stack||e))}
report.finishedAt=new Date().toISOString();report.passed=report.failures.length===0&&report.commands.length===2&&report.commands.every(c=>c.exitCode===0&&!c.forced&&!c.signal&&!c.error);
await write(path.join(root,'launcher-receipt.json'),JSON.stringify(report,null,2)+'\n');
console.log(JSON.stringify({root,passed:report.passed,failures:report.failures}));process.exitCode=report.passed?0:1;
