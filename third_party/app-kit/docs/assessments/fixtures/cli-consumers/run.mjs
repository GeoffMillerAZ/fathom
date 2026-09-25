// Own one private CLI probe: execute retained harness/binaries, never mutable inputs.
// This does not authorize running a consumer. Use synthetic inputs until approved.
import fs from 'node:fs/promises';
import {constants} from 'node:fs';
import {createHash} from 'node:crypto';
import {spawn} from 'node:child_process';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const here=path.dirname(fileURLToPath(import.meta.url));
const sha=body=>createHash('sha256').update(body).digest('hex');
const LIMIT=128*1024*1024;

export async function readRegular(file, executable=false, limit=LIMIT){
  const handle=await fs.open(file,constants.O_RDONLY|constants.O_NOFOLLOW);
  try{
    const info=await handle.stat();
    if(!info.isFile()||info.size<1||info.size>limit||(executable&&(info.mode&0o111)===0))throw Error('Invalid bounded regular input: '+file);
    const chunks=[];let size=0;
    for(;;){
      const buffer=Buffer.alloc(Math.min(65536,limit-size+1));
      const {bytesRead}=await handle.read(buffer,0,buffer.length,null);
      if(!bytesRead)break;
      size+=bytesRead;if(size>limit)throw Error('Input exceeds byte limit: '+file);
      chunks.push(buffer.subarray(0,bytesRead));
    }
    if(size!==info.size)throw Error('Input size changed during retention: '+file);
    return Buffer.concat(chunks,size);
  }finally{await handle.close();}
}

export async function prepareTrial(bosun,surveyor,output){
  for(const value of [bosun,surveyor,output])if(!path.isAbsolute(value)||path.normalize(value)!==value)throw Error('Require absolute normalized input/output paths');
  const stat=await fs.lstat(output);
  if(!stat.isDirectory()||stat.isSymbolicLink()||(stat.mode&0o077)!==0||stat.uid!==process.getuid()||await fs.realpath(output)!==output||(await fs.readdir(output)).length)throw Error('Output must be an empty canonical private owned directory');
  // Validate/read before creating artifacts. Never reopen mutable input for copy.
  const inputs=[];
  for(const [name,source,executable,limit] of [['probe.mjs',path.join(here,'probe.mjs'),false,1024*1024],['bosun',bosun,true,LIMIT],['surveyor',surveyor,true,LIMIT]]){
    const body=await readRegular(source,executable,limit);
    inputs.push({name,source,executable,body,sha256:sha(body),bytes:body.length});
  }
  const inputRoot=path.join(output,'inputs'),results=path.join(output,'results');
  await fs.mkdir(inputRoot,{mode:0o700});await fs.mkdir(results,{mode:0o700});
  for(const input of inputs){
    input.executed=path.join(inputRoot,input.name);
    await fs.writeFile(input.executed,input.body,{flag:'wx',mode:input.executable?0o500:0o400});
    delete input.body;
  }
  const ownBytes=await readRegular(fileURLToPath(import.meta.url),false,1024*1024);
  await fs.writeFile(path.join(output,'run.mjs'),ownBytes,{flag:'wx',mode:0o400});
  const plan={schema:1,kind:'owned-cli-consumer-probe',scope:'exact retained probe/executable bytes, not consumer build/release provenance',output,inputs,observedRunnerSource:{sha256:sha(ownBytes),bytes:ownBytes.length,limit:'Snapshot of source observed during staging, not independent identity proof for the already-running bootstrap.'},command:{executable:process.execPath,args:[path.join(inputRoot,'probe.mjs'),path.join(inputRoot,'bosun'),path.join(inputRoot,'surveyor'),results],cwd:output},bounds:{timeoutMS:70000,outputBytesPerStream:65536},results};
  await fs.writeFile(path.join(output,'inputs.json'),JSON.stringify(plan,null,2)+'\n',{flag:'wx',mode:0o600});
  return plan;
}

export async function verifyInputs(plan){
  const checks=[];
  for(const input of plan.inputs){
    try{
      const body=await readRegular(input.executed,input.executable);
      checks.push({name:input.name,passed:body.length===input.bytes&&sha(body)===input.sha256,sha256:sha(body),bytes:body.length});
    }catch(error){checks.push({name:input.name,passed:false,error:String(error)});}
  }
  return checks;
}

export function validateProbeReport(result,plan){
  const canaries=['sandbox-denies-synthetic-read','sandbox-denies-synthetic-write','sandbox-denies-owned-loopback-network'];
  const cases={help:['--help'],version:['--version'],'unknown-flag':['--appkit-probe-unknown-flag'],'unknown-command':['appkit-probe-unknown-command']};
  const consumers=['bosun','surveyor'];
  if(result?.schema_version!==1||result.kind!=='app-kit-cli-consumer-runtime-probe'||!['passed','failed'].includes(result.status)||typeof result.canaries_passed!=='boolean'||!Array.isArray(result.checks)||!Array.isArray(result.runs))throw Error('Invalid probe receipt schema');
  const runIDs=result.canaries_passed?consumers.flatMap(name=>Object.keys(cases).map(id=>name+'-'+id)):[];
  const checkIDs=[...canaries,...(result.canaries_passed?consumers.flatMap(name=>[...Object.keys(cases).map(id=>name+'-'+id),name+'-cwd-remains-empty']):[])];
  const exactIDs=(rows,ids)=>rows.length===ids.length&&new Set(rows.map(row=>row.id)).size===ids.length&&rows.every(row=>ids.includes(row.id)&&typeof row.passed==='boolean');
  if(!exactIDs(result.checks,checkIDs)||!exactIDs(result.runs,runIDs))throw Error('Incomplete or repeated probe checks');
  if(canaries.every(id=>result.checks.find(c=>c.id===id).passed)!==result.canaries_passed)throw Error('Canary outcome differs');
  if((result.status==='passed')!==(result.canaries_passed&&result.checks.every(c=>c.passed)))throw Error('Probe status contradicts checks');
  for(const name of consumers){
    const retained=plan.inputs.find(input=>input.name===name);
    if(result.binaries?.[name]?.path!==retained.executed||result.binaries[name].sha256!==retained.sha256)throw Error(name+' probe identity differs');
    for(const [id,args] of Object.entries(cases)){
      if(!result.canaries_passed)continue;
      const run=result.runs.find(r=>r.id===name+'-'+id);
      if(run.consumer!==name||run.binary_sha256!==retained.sha256||JSON.stringify(run.args)!==JSON.stringify(args)||run.passed!==result.checks.find(c=>c.id===run.id).passed)throw Error('Probe run identity or result differs');
    }
  }
}

export async function executeTrial(plan){
  const report={...plan,startedAt:new Date().toISOString(),errors:[]};
  const stdout=[],stderr=[];const sizes={stdout:0,stderr:0},limited={stdout:false,stderr:false};
  report.before=await verifyInputs(plan);
  if(!report.before.every(c=>c.passed)){
    report.passed=false;report.started=false;report.errors.push('Retained inputs differ before execution');
    await fs.writeFile(path.join(plan.output,'run-receipt.json'),JSON.stringify(report,null,2)+'\n',{flag:'wx',mode:0o600});
    return report;
  }
  // Separate process group contains only this probe and its fixed child checks.
  const child=spawn(plan.command.executable,plan.command.args,{cwd:plan.command.cwd,env:{PATH:'/usr/bin:/bin:/usr/sbin:/sbin'},stdio:['ignore','pipe','pipe'],detached:true});
  report.started=true;report.pid=child.pid;
  let closed=false;
  // A leader may exit while its owned descendants still hold the pipes open.
  const stop=signal=>{if(child.pid&&!closed){try{process.kill(-child.pid,signal);}catch(error){if(error.code!=='ESRCH')report.errors.push('Owned group stop failed: '+String(error));}}};
  const completion=new Promise(resolve=>{
    child.once('error',error=>report.errors.push(String(error)));
    child.once('close',(code,signal)=>{closed=true;resolve({code,signal});});
  });
  let timedOut=false,interrupted=false;
  const interrupt=()=>{interrupted=true;stop('SIGKILL');};process.once('SIGTERM',interrupt);
  for(const [stream,chunks] of [['stdout',stdout],['stderr',stderr]])child[stream].on('data',chunk=>{
    const available=Math.max(0,plan.bounds.outputBytesPerStream-sizes[stream]);sizes[stream]+=chunk.length;
    if(available>0)chunks.push(Buffer.from(chunk.subarray(0,available)));
    if(sizes[stream]>plan.bounds.outputBytesPerStream&&!limited[stream]){limited[stream]=true;report.errors.push(stream+' limit exceeded');stop('SIGKILL');}
  });
  const timer=setTimeout(()=>{timedOut=true;stop('SIGKILL');},plan.bounds.timeoutMS);
  report.exit=await completion;clearTimeout(timer);process.removeListener('SIGTERM',interrupt);
  report.timedOut=timedOut;report.interrupted=interrupted;
  report.logs={};
  for(const [name,chunks] of [['stdout',stdout],['stderr',stderr]]){
    const bytes=Buffer.concat(chunks);
    report.logs[name]={observedBytes:sizes[name],capturedBytes:bytes.length,sha256:sha(bytes),limited:limited[name]};
    await fs.writeFile(path.join(plan.output,'probe.'+name),bytes,{flag:'wx',mode:0o600});
  }
  report.after=await verifyInputs(plan);
  try{
    const body=await readRegular(path.join(plan.results,'checks/report.json'),false,2*1024*1024);
    const result=JSON.parse(body);validateProbeReport(result,plan);
    report.probe={status:result.status,sha256:sha(body),bytes:body.length,checks:result.checks.length,failed:result.checks.filter(c=>!c.passed).map(c=>c.id)};
  }catch(error){report.errors.push('Probe receipt unavailable: '+String(error));}
  report.passed=report.exit.code===0&&report.exit.signal===null&&!timedOut&&!interrupted&&report.errors.length===0&&report.after.every(c=>c.passed)&&report.probe?.status==='passed';
  report.finishedAt=new Date().toISOString();
  await fs.writeFile(path.join(plan.output,'run-receipt.json'),JSON.stringify(report,null,2)+'\n',{flag:'wx',mode:0o600});
  return report;
}

if(process.argv[1]&&path.resolve(process.argv[1])===fileURLToPath(import.meta.url)){
  try{
    const args=process.argv.slice(2);if(args.length!==3)throw Error('Usage: node run.mjs ABSOLUTE_BOSUN ABSOLUTE_SURVEYOR EMPTY_PRIVATE_OUTPUT');
    const report=await executeTrial(await prepareTrial(...args));
    console.log(JSON.stringify({passed:report.passed,output:report.output,exit:report.exit,failures:report.probe?.failed,errors:report.errors}));
    if(!report.passed)process.exitCode=1;
  }catch(error){console.error(String(error));process.exitCode=1;}
}
