// Launch only the exact prepared synthetic A/B artifacts; never a consumer app.
import fs from 'node:fs';
import path from 'node:path';
import net from 'node:net';
import {createHash} from 'node:crypto';
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';

const [pairRootArg,playwright,chrome,outputArg]=process.argv.slice(2);
if(process.argv.length!==6)throw Error('usage: node run-pair.mjs <prepared-pair-root> <playwright/index.mjs> <Chrome-executable> <fresh-private-output>');
const pairRoot=fs.realpathSync(pairRootArg),output=path.resolve(outputArg);
if(fs.existsSync(output)||!path.isAbsolute(outputArg))throw Error('output must be a fresh absolute path');
fs.mkdirSync(output,{mode:0o700});
const hash=b=>createHash('sha256').update(b).digest('hex');
const put=(rel,b)=>{const file=path.join(output,rel);fs.mkdirSync(path.dirname(file),{recursive:true,mode:0o700});fs.writeFileSync(file,b,{flag:'wx',mode:0o600});return file;};
const receiptBytes=fs.readFileSync(path.join(pairRoot,'receipt.json')),pair=JSON.parse(receiptBytes);
if(pair.kind!=='analytics-usability-reconstructed-composition-preparation'||pair.variants?.length!==2)throw Error('unrecognized preparation receipt');
put('frozen/preparation-receipt.json',receiptBytes);
const ownFile=fileURLToPath(import.meta.url),probeSource=path.join(path.dirname(ownFile),'verify-pair.mjs');
put('frozen/run-pair.mjs',fs.readFileSync(ownFile));
const probe=put('frozen/verify-pair.mjs',fs.readFileSync(probeSource));
const env={PATH:'/usr/bin:/bin',LC_ALL:'C',TMPDIR:path.join(output,'tmp'),APPKIT_DESIGN_ASSETS:''};
fs.mkdirSync(env.TMPDIR,{mode:0o700});
const variantConfig=pair.variants.map(v=>{
 if(!['A','B'].includes(v.tag))throw Error('unexpected variant');
 const binary=path.join(pairRoot,v.tag,'gallery'),css=path.join(pairRoot,v.tag,'source','apps/gallery/ui/app.css');
 if(hash(fs.readFileSync(binary))!==v.gallerySHA256||hash(fs.readFileSync(css))!==v.css.sha256)throw Error('prepared artifact drift '+v.tag);
 return {label:v.tag,binary:{path:binary,sha256:v.gallerySHA256},css:{path:css,sha256:v.css.sha256}};
});
if(new Set(variantConfig.map(v=>v.label)).size!==2)throw Error('duplicate variants');
put('config.json',JSON.stringify({pairRoot,preparationReceiptSHA256:hash(receiptBytes),env,variantConfig,playwright,chrome,probeSHA256:hash(fs.readFileSync(probe)),node:process.execPath,limits:['Synthetic loopback Gallery only; actual browser PID/tool identity is owned by the frozen probe.','Server lifecycle and automated semantic readiness are not human task measurements.']},null,2)+'\n');
const report={passed:false,startedAt:new Date().toISOString(),servers:[],probe:null,errors:[]};
const writeReport=()=>fs.writeFileSync(path.join(output,'run-report.json'),JSON.stringify(report,null,2)+'\n',{mode:0o600});
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const states=[];
function start(v){
 const child=spawn(v.binary.path,['-addr','127.0.0.1:0','-design-assets',''],{cwd:output,env,detached:true,stdio:['ignore','pipe','pipe']});
 const state={v,child,stdout:Buffer.alloc(0),stderr:Buffer.alloc(0),error:null,overflow:false};
 for(const key of ['stdout','stderr'])child[key].on('data',b=>{state[key]=Buffer.concat([state[key],b]);if(state[key].length>(2<<20)){state.overflow=true;try{process.kill(-child.pid,'SIGKILL');}catch{}}});
 child.on('error',e=>{state.error=String(e);});states.push(state);return state;
}
async function origin(state){
 for(let i=0;i<300;i++){const m=(state.stdout.toString()+state.stderr.toString()).match(/http:\/\/127\.0\.0\.1:\d+/);if(m)return m[0];if(state.error||state.child.exitCode!==null||state.child.signalCode!==null)break;await sleep(50);}
 throw Error('server startup failed '+state.v.label+': '+state.error);
}
async function refused(origin){const u=new URL(origin);return new Promise(resolve=>{const s=net.connect({host:u.hostname,port:Number(u.port)});let done=false;const finish=x=>{if(done)return;done=true;clearTimeout(timer);s.destroy();resolve(x);};const timer=setTimeout(()=>finish(false),2000);s.once('connect',()=>finish(false));s.once('error',e=>finish(e.code==='ECONNREFUSED'));});}
async function stop(state){
 const c=state.child;if(c.exitCode===null&&c.signalCode===null&&!state.error)c.kill('SIGTERM');
 for(let i=0;i<200&&c.exitCode===null&&c.signalCode===null&&!state.error;i++)await sleep(50);
 const timeout=c.exitCode===null&&c.signalCode===null&&!state.error;
 if(timeout){try{process.kill(-c.pid,'SIGKILL');}catch{}}
 const portRefused=state.origin?await refused(state.origin):null;
 put('servers/'+state.v.label+'.stdout.txt',state.stdout);put('servers/'+state.v.label+'.stderr.txt',state.stderr);
 return {label:state.v.label,pid:c.pid,origin:state.origin,exit:c.exitCode,signal:c.signalCode,timeout,portRefused,error:state.error,overflow:state.overflow,binaryUnchanged:hash(fs.readFileSync(state.v.binary.path))===state.v.binary.sha256};
}
try{
 const configs=[];
 for(const v of variantConfig){const state=start(v);state.origin=await origin(state);configs.push(put(v.label+'.json',JSON.stringify({schema:'analytics-usability-artifact-v1',...v,origin:state.origin,expectedPlaywrightVersion:'1.62.1'},null,2)+'\n'));}
 const browserOut=path.join(output,'browser');fs.mkdirSync(browserOut,{mode:0o700});
 const args=[probe,...configs,playwright,chrome,browserOut];
 put('probe-command.json',JSON.stringify({executable:process.execPath,args,env},null,2)+'\n');
 const child=spawn(process.execPath,args,{cwd:output,env,detached:true,stdio:['ignore','pipe','pipe']});
 let stdout=Buffer.alloc(0),stderr=Buffer.alloc(0),timedOut=false,overflow=false;
 const kill=()=>{try{process.kill(-child.pid,'SIGKILL');}catch{}};
 const timer=setTimeout(()=>{timedOut=true;kill();},600000);
 child.stdout.on('data',b=>{stdout=Buffer.concat([stdout,b]);if(stdout.length>(4<<20)){overflow=true;kill();}});
 child.stderr.on('data',b=>{stderr=Buffer.concat([stderr,b]);if(stderr.length>(4<<20)){overflow=true;kill();}});
 const end=await new Promise(resolve=>{child.once('error',e=>resolve({error:String(e)}));child.once('close',(code,signal)=>resolve({code,signal}));});clearTimeout(timer);
 put('probe.stdout.txt',stdout);put('probe.stderr.txt',stderr);
 report.probe={...end,timedOut,overflow};
 try{report.probe.summary=JSON.parse(stdout.toString().trim().split('\n').at(-1));}catch{}
 if(end.code!==0||end.signal||end.error||timedOut||overflow||report.probe.summary?.passed!==true)throw Error('pair browser probe failed');
}catch(e){report.errors.push(String(e.stack||e));}
finally{
 for(const state of states){try{report.servers.push(await stop(state));}catch(e){report.errors.push(String(e));}}
 report.passed=report.errors.length===0&&report.servers.length===2&&report.servers.every(s=>s.exit===0&&!s.signal&&!s.timeout&&!s.error&&!s.overflow&&s.portRefused&&s.binaryUnchanged);
 report.finishedAt=new Date().toISOString();writeReport();
 console.log(JSON.stringify({output,passed:report.passed,errors:report.errors,probe:report.probe,servers:report.servers,reportSHA256:hash(fs.readFileSync(path.join(output,'run-report.json')))}));
 if(!report.passed)process.exitCode=1;
}
