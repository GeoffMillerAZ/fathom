// Opt-in P01 experiment, not a general font or delivery API. No installs/fetches.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {createHash} from 'node:crypto';
import {fileURLToPath,pathToFileURL} from 'node:url';

const args=process.argv.slice(2),[probeArg,referenceArg,python,hbArg,parentArg]=args;
assert.ok(args.length===5&&args.every(p=>path.isAbsolute(p)),
  'usage: node font-proof.mjs ABS_PROBE ABS_REFERENCE_RECEIPT ABS_PRIVATE_PYTHON ABS_HB_SHAPE ABS_PRIVATE_OUTPUT_PARENT');
const sha=b=>createHash('sha256').update(b).digest('hex');
const read=async p=>{const s=await fs.lstat(p);assert.ok(s.isFile()&&!s.isSymbolicLink()&&s.size<64*1024*1024,'bounded real file: '+p);return fs.readFile(p)};
const write=(p,b)=>fs.writeFile(p,b,{flag:'wx',mode:0o600});
const sourceDir=path.dirname(fileURLToPath(import.meta.url));
const parent=await fs.realpath(parentArg),probe=await fs.realpath(probeArg),referencePath=await fs.realpath(referenceArg);
assert.ok((await fs.stat(parent)).isDirectory());
const root=await fs.mkdtemp(path.join(parent,'font-proof-'));await fs.chmod(root,0o700);
const runner=path.join(root,'runner');await fs.mkdir(runner,{mode:0o700});
const report={schema:1,verification:'experimental-not-certified',startedAt:new Date().toISOString(),root,args,node:process.version,sources:[],inputs:[],tools:{},commands:[],shaping:[],failures:[]};
console.log('Private font evidence: '+root);
async function run(name,executable,argv,timeoutMs=15000){
  const child=spawn(executable,argv,{cwd:root,env:{...process.env,PYTHONDONTWRITEBYTECODE:'1'},stdio:['ignore','pipe','pipe']});
  const item={name,executable,argv,pid:child.pid,startedAt:new Date().toISOString(),timeoutMs};
  let stdout='',stderr='',forced=false,timer,killTimer;
  const stop=()=>{if(forced)return;forced=true;child.kill('SIGTERM');killTimer=setTimeout(()=>child.kill('SIGKILL'),25000)};
  timer=setTimeout(stop,timeoutMs);
  child.stdout.on('data',b=>{stdout+=b;if(stdout.length>4*1024*1024)stop()});
  child.stderr.on('data',b=>{stderr+=b;if(stderr.length>1024*1024)stop()});
  await new Promise(resolve=>{child.once('error',e=>item.error=String(e));child.once('close',(code,signal)=>{item.exitCode=code;item.signal=signal;resolve()})});
  clearTimeout(timer);clearTimeout(killTimer);Object.assign(item,{forced,finishedAt:new Date().toISOString(),stdoutSHA:sha(stdout),stderrSHA:sha(stderr)});
  await write(path.join(root,name+'.stdout.log'),stdout);await write(path.join(root,name+'.stderr.log'),stderr);report.commands.push(item);
  assert.ok(!item.error&&!forced&&!item.signal,'abnormal execution: '+name);
  return {...item,stdout,stderr};
}
async function bind(name,source,expected){
  const bytes=await read(source);if(expected)assert.equal(sha(bytes),expected,'input identity: '+name);
  const file=path.join('inputs',name);await fs.mkdir(path.dirname(path.join(root,file)),{recursive:true,mode:0o700});await write(path.join(root,file),bytes);
  const item={name,file,source,sha256:sha(bytes),bytes:bytes.length};report.inputs.push(item);return {bytes,item};
}
try{
  for(const name of ['font-proof.mjs','font-subset.py','font-delivery.mjs','font-delivery_test.mjs','font-painted.mjs','font-painted_test.mjs','font-browser.mjs','performance-contract.json','performance-oracle.mjs','performance_test.mjs']){
    const source=path.join(sourceDir,name),bytes=await read(source);await write(path.join(runner,name),bytes);report.sources.push({name,source,sha256:sha(bytes),bytes:bytes.length});
  }
  await write(path.join(root,'snapshot.json'),JSON.stringify({schema:1,sources:report.sources},null,2)+'\n');
  const test=await run('node-tests',process.execPath,['--test',path.join(runner,'font-delivery_test.mjs'),path.join(runner,'font-painted_test.mjs'),path.join(runner,'performance_test.mjs')]);assert.equal(test.exitCode,0,test.stderr);
  const contract=JSON.parse(await read(path.join(runner,'performance-contract.json')));
  const reference=JSON.parse((await bind('reference-receipt.json',referencePath,contract.pins.referenceReceipt)).bytes);
  await bind('original.html',path.join(probe,'original.html'),contract.pins.source);
  const artifact=JSON.parse((await bind('artifact.json',path.join(probe,'delivery/artifact.json'),contract.pins.artifact)).bytes);
  const fragment=(await bind('enhanced.html',path.join(probe,'enhanced.html'),contract.pins.enhanced)).bytes;
  const css=await bind('source.css',path.join(path.dirname(referencePath),'source.css'),reference.inputs.tailwind.css_sha256);
  const companion=await bind('hero.css',path.join(probe,'delivery/hero.css'),artifact.css_sha256);
  const resources=[];
  const add=(url,type,item)=>resources.push({path:url,type,file:item.file,sha256:item.sha256,bytes:item.bytes});
  add('/source.css','text/css',css.item);add('/hero.css','text/css',companion.item);
  const assets=JSON.parse((await bind('assets.json',path.join(path.dirname(referencePath),'assets.json'))).bytes);
  assert.equal(assets.assets.length,3);
  const urls={'hero.jpg':artifact.source_assets.image_url,'logo-light.svg':artifact.source_assets.logo_light_url,'logo-dark.svg':artifact.source_assets.logo_dark_url};
  for(const a of assets.assets){assert.equal(a.url,urls[a.name]);const pin=reference.assets.find(p=>p.name===a.name);assert.equal(a.sha256,pin.sha256);const bound=await bind('assets/'+a.name,path.join(path.dirname(referencePath),'assets',a.name),a.sha256);assert.equal(bound.bytes.length,a.bytes);add('/assets/'+a.name,a.mime,bound.item)}
  const original=await bind('assets/inter.woff2',path.join(path.dirname(referencePath),'assets/inter.woff2'),reference.inputs.font.sha256);add('/assets/inter.woff2','font/woff2',original.item);
  const hb=await fs.realpath(hbArg),realPython=await fs.realpath(python);
  for(const [label,file] of [['hbShape',hb],['python',realPython]]){const bytes=await read(file);report.tools[label]={path:file,sha256:sha(bytes),bytes:bytes.length}}
  const version=await run('harfbuzz-version',hb,['--version']);assert.equal(version.exitCode,0);assert.match(version.stdout,/^hb-shape \(HarfBuzz\) 14\.3\.0\n/);
  for(const name of ['build-a','build-b']){
    const r=await run(name,python,['-B',path.join(runner,'font-subset.py'),path.join(root,original.item.file),path.join(root,name)],30000);assert.equal(r.exitCode,0,r.stderr);
  }
  const a=JSON.parse(await read(path.join(root,'build-a/font-receipt.json'))),b=JSON.parse(await read(path.join(root,'build-b/font-receipt.json')));assert.deepEqual(a,b,'two generation manifests');
  for(const [name,identity] of Object.entries(a.outputs))for(const build of ['build-a','build-b']){const bytes=await read(path.join(root,build,name));assert.equal(sha(bytes),identity.sha256);assert.equal(bytes.length,identity.bytes)}
  const {makeFontCSS,subsetRanges}=await import(pathToFileURL(path.join(runner,'font-delivery.mjs')));assert.deepEqual(a.ranges,subsetRanges);
  report.generation={identical:true,manifestSHA:sha(await read(path.join(root,'build-a/font-receipt.json'))),manifest:a};
  await write(path.join(root,'invalid-font.woff2'),Buffer.from('not the retained original'));
  const bad=await run('reject-source',python,['-B',path.join(runner,'font-subset.py'),path.join(root,'invalid-font.woff2'),path.join(root,'must-not-exist')]);assert.equal(bad.exitCode,1);assert.match(bad.stderr,/retained original font identity mismatch/);
  await assert.rejects(fs.stat(path.join(root,'must-not-exist')),{code:'ENOENT'});
  const existing=await run('reject-existing',python,['-B',path.join(runner,'font-subset.py'),path.join(root,original.item.file),path.join(root,'build-a')]);assert.equal(existing.exitCode,1);assert.match(existing.stderr,/FileExistsError/);
  const texts=[
    'Anim aute id magna aliqua ad ad non deserunt sunt. Read more → Data to enrich your business Qui irure qui lorem cupidatat commodo. Elit sunt amet fugiat veniam occaecat. Get started Learn more →',
    'AVATAR To WA','office affine fi fl ffi ffl','café | e\u0301 | naïve | n\u0303 | Å | A\u030A',
    'İ | i\u0307 | ı','ß | Straße','“quote”—€↔',
  ];
  const selected=new Set(a.selectedCodepoints);for(const text of texts)for(const c of text)assert.ok(selected.has(c.codePointAt(0)),'shaping case outside admitted subset');
  let index=0;
  for(const weight of [100,400,500,600,900])for(const opticalSize of [14,32])for(const text of texts){
    const pair=[];
    for(const name of ['original','subset']){
      const r=await run('shape-'+(++index)+'-'+name,hb,[path.join(root,'build-a',name+'.ttf'),text,'--output-format=json',`--variations=wght=${weight},opsz=${opticalSize}`]);assert.equal(r.exitCode,0,r.stderr);
      const value=JSON.parse(r.stdout);assert.ok(value.length&&value.every(g=>typeof g.g==='string'&&g.g!=='.notdef'),'actual named glyphs, not missing glyphs');pair.push(value);
    }
    assert.deepEqual(pair[0],pair[1],'shaping mismatch: '+JSON.stringify({weight,opticalSize,text}));report.shaping.push({weight,opticalSize,text,glyphs:pair[0].length,sha256:sha(JSON.stringify(pair[0])),equal:true});
  }
  console.log('Two identical builds; '+report.shaping.length+' HarfBuzz shaping pairs pass.');
  const candidate=Buffer.from(makeFontCSS(css.bytes.toString()));await write(path.join(root,'inputs/candidate.css'),candidate);add('/candidate.css','text/css',{file:'inputs/candidate.css',sha256:sha(candidate),bytes:candidate.length});
  add('/assets/inter-subset.woff2','font/woff2',{file:'build-a/inter-subset.woff2',...a.outputs['inter-subset.woff2']});
  const document=Buffer.from('<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Private hero enhanced</title><link rel="icon" href="data:,"><link rel="stylesheet" href="/source.css"><link rel="stylesheet" href="/hero.css"></head><body>'+fragment.toString()+'</body></html>');
  await write(path.join(root,'inputs/document.html'),document);add('/enhanced','text/html',{file:'inputs/document.html',sha256:sha(document),bytes:document.length});
  const prepared={schema:1,resources,font:{originalSHA:original.item.sha256,subsetSHA:a.outputs['inter-subset.woff2'].sha256},expectedBrowserVersion:contract.pins.browserVersion,playwrightEntrySHA:contract.pins.playwrightEntry};
  await write(path.join(root,'prepared.json'),JSON.stringify(prepared,null,2)+'\n');report.preparedSHA=sha(await read(path.join(root,'prepared.json')));
  const browser=await run('font-browser',process.execPath,[path.join(runner,'font-browser.mjs'),root],150000);report.browserExit=browser.exitCode;
  const browserReport=await read(path.join(root,'font-browser-receipt.json'));report.browserReceiptSHA=sha(browserReport);report.browserResult=JSON.parse(browserReport);
  for(const item of report.sources){assert.equal(sha(await read(item.source)),item.sha256,'source drift '+item.name);assert.equal(sha(await read(path.join(runner,item.name))),item.sha256,'snapshot drift '+item.name)}
  for(const item of report.inputs){assert.equal(sha(await read(item.source)),item.sha256,'input drift '+item.name);assert.equal(sha(await read(path.join(root,item.file))),item.sha256,'copied input drift '+item.name)}
  for(const item of Object.values(report.tools))assert.equal(sha(await read(item.path)),item.sha256,'tool drift');
  for(const [name,identity] of Object.entries(a.outputs))assert.equal(sha(await read(path.join(root,'build-a',name))),identity.sha256,'output drift');
  report.preparationPassed=true;report.browserPassed=browser.exitCode===0;
}catch(error){report.failures.push(String(error.stack||error))}
report.finishedAt=new Date().toISOString();report.passed=report.preparationPassed===true&&report.browserPassed===true&&report.failures.length===0;
await write(path.join(root,'font-proof-receipt.json'),JSON.stringify(report,null,2)+'\n');
console.log(JSON.stringify({root,passed:report.passed,preparationPassed:report.preparationPassed,shapingPairs:report.shaping.length,browserExit:report.browserExit,failures:report.failures},null,2));process.exitCode=report.passed?0:1;
