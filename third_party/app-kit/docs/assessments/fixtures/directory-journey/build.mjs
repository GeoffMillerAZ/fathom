// Explicit private P04 integration. No dependency installation, original edits or publication.
import fs from 'node:fs/promises';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {spawn} from 'node:child_process';
import {createHash} from 'node:crypto';
import assert from 'node:assert/strict';

const [kitArg,parentArg]=process.argv.slice(2);
assert.ok(path.isAbsolute(kitArg)&&path.isAbsolute(parentArg));
const kit=await fs.realpath(kitArg),parent=await fs.realpath(parentArg);
const root=await fs.mkdtemp(path.join(parent,'directory-journey-'));await fs.chmod(root,0o700);
const fixtures=path.join(kit,'docs/assessments/fixtures/directory-journey');
assert.equal(await fs.realpath(path.dirname(fileURLToPath(import.meta.url))),fixtures);
const sha=b=>createHash('sha256').update(b).digest('hex');
const write=(p,b)=>fs.writeFile(p,b,{flag:'wx',mode:0o600});
const report={kind:'p04-directory-journey-build-v1',verification:'not-verified',root,startedAt:new Date().toISOString(),inputs:{},commands:[],failures:[]};
// Preserve the real home solely for the existing read-only Tailwind cache lookup.
// Do not repurpose it as scratch; all new output lives under root.
const env={PATH:process.env.PATH,HOME:process.env.HOME,TMPDIR:root,GOCACHE:process.env.GOCACHE,GOMODCACHE:process.env.GOMODCACHE,
  GOENV:'off',GOWORK:'off',GOTOOLCHAIN:'local',GOPROXY:'off',GOSUMDB:'off',GONOSUMDB:'',GOPRIVATE:'',GOFLAGS:'-mod=readonly',CGO_ENABLED:'0'};
assert.ok(path.isAbsolute(env.GOCACHE)&&path.isAbsolute(env.GOMODCACHE));
async function run(command,args,cwd){
  const child=spawn(command,args,{cwd,env,stdio:['ignore','pipe','pipe']});let out='',err='';
  const timer=setTimeout(()=>child.kill('SIGKILL'),120000);
  child.stdout.on('data',b=>{out+=b.toString()});child.stderr.on('data',b=>{err+=b.toString()});
  const result=await new Promise((resolve,reject)=>{child.once('error',reject);child.once('close',(code,signal)=>resolve({code,signal}))}).finally(()=>clearTimeout(timer));
  const index=report.commands.length;await write(path.join(root,`command-${index}.stdout`),out);await write(path.join(root,`command-${index}.stderr`),err);
  report.commands.push({command,args,cwd,...result,stdoutSHA:sha(out),stderrSHA:sha(err)});
  assert.equal(result.code,0,`command ${index} failed; retained logs`);assert.equal(result.signal,null);
  return out;
}
async function snapshot(relative,local){
  const b=await fs.readFile(path.join(kit,relative));report.inputs[relative]={sha256:sha(b),bytes:b.length,snapshot:local};
  await write(path.join(root,local),b);return b;
}
try{
  await fs.mkdir(path.join(root,'sources'),{mode:0o700});
  for(const name of ['build.mjs','README.md','host.go.txt','journey.go','journey_test.go'])await snapshot(`docs/assessments/fixtures/directory-journey/${name}`,`sources/${name}`);
  await snapshot('go.mod','sources/kit.mod');await snapshot('go.sum','sources/kit.sum');
  await snapshot('docs/assessments/fixtures/table-adapter/prepare.go','sources/prepare.go');
  const archive='data/design-assets/archives/b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8.zip';
  const archiveBefore=sha(await fs.readFile(path.join(kit,archive)));
  assert.equal(archiveBefore,path.basename(archive,'.zip'));
  const catalogBefore=sha(await fs.readFile(path.join(kit,'data/design-assets/catalog.json')));
  report.original={archive,sha256:archiveBefore,catalogSHA:catalogBefore};
  const cli=path.join(root,'appkit'),preparer=path.join(root,'prepare');
  await run('go',['build','-o',cli,'./cmd/appkit'],kit);
  await run('go',['build','-o',preparer,'./docs/assessments/fixtures/table-adapter'],kit);
  report.tools={};
  for(const [name,file]of Object.entries({appkit:cli,prepare:preparer}))report.tools[name]={path:file,sha256:sha(await fs.readFile(file))};
  report.goVersion=(await run('go',['version'],kit)).trim();
  const delivery=path.join(root,'delivery');
  await run(cli,['data','assets-adapt-directory','-root',path.join(kit,'data/design-assets'),'-enhanced','-out',delivery],kit);
  const reference=JSON.parse(await run(preparer,['-kit-root',kit],kit));
  assert.ok(reference.path.startsWith(root+path.sep));
  const manifest=JSON.parse(await fs.readFile(path.join(reference.path,'reference.json')));
  assert.equal(manifest.case_id,'P04-table');assert.equal(manifest.source.sha256,'5d00e2755dcf060cceafa4c2dba4f15452719e54f5cd40646ab3155c89dc8288');
  const source=await fs.readFile(path.join(reference.path,'source-light.html'));
  assert.equal(sha(source),manifest.documents['source-light.html'].sha256);
  const text=source.toString(),start=text.indexOf('<style>'),end=text.indexOf('</style>');
  assert.ok(start>0&&end>start&&text.indexOf('<style>',start+1)<0);
  const css=text.slice(start+7,end)+'\n'+await fs.readFile(path.join(delivery,'directory_enhanced.css'),'utf8');
  const module=path.join(root,'consumer');await fs.mkdir(module,{mode:0o700});
  await fs.mkdir(path.join(module,'converted'),{mode:0o700});await fs.mkdir(path.join(module,'journey'),{mode:0o700});
  report.delivery={};
  for(const name of (await fs.readdir(delivery)).sort()){
    const b=await fs.readFile(path.join(delivery,name));report.delivery[name]={sha256:sha(b),bytes:b.length};
    await write(path.join(module,'converted',name),b);
  }
  const mod=await snapshot('docs/assessments/fixtures/pricing-adapter/consumer.mod.txt','sources/consumer.mod.txt');
  const sum=await snapshot('docs/assessments/fixtures/pricing-adapter/consumer.sum.txt','sources/consumer.sum.txt');
  const moduleText=mod.toString().replace(/^module \S+/m,'module example.invalid/appkit-directory-journey');
  assert.ok(!moduleText.includes('replace ')&&!moduleText.includes('github.com/geoffmilleraz/app-kit'));
  await write(path.join(module,'go.mod'),moduleText);await write(path.join(module,'go.sum'),sum);
  await write(path.join(module,'app.css'),css);await write(path.join(module,'main.go'),await fs.readFile(path.join(root,'sources/host.go.txt')));
  for(const name of ['journey.go','journey_test.go'])await write(path.join(module,'journey',name),await fs.readFile(path.join(root,'sources',name)));
  await run('go',['tool','templ','generate'],module);await run('go',['test','./...','-count=1'],module);
  await run('go',['build','-trimpath','-o',path.join(root,'directory-journey') ,'.'],module);
  await run('go',['build','-trimpath','-o',path.join(root,'directory-journey-repeat'),'.'],module);
  const binary=await fs.readFile(path.join(root,'directory-journey'));
  assert.equal(sha(binary),sha(await fs.readFile(path.join(root,'directory-journey-repeat'))));
  report.binary={path:path.join(root,'directory-journey'),sha256:sha(binary),bytes:binary.length};report.css={sha256:sha(css),bytes:Buffer.byteLength(css)};
  const dataset=await run(report.binary.path,['--dataset'],module);await write(path.join(root,'dataset.json'),dataset);report.datasetSHA=sha(dataset);
  const frozen=JSON.parse(dataset);assert.equal(frozen.Revision,sha(JSON.stringify(frozen.Rows)),'canonical dataset revision');
  await run(cli,['data','assets-adapt-directory','-root',path.join(kit,'data/design-assets'),'-enhanced','-check',delivery],kit);
  assert.equal(archiveBefore,sha(await fs.readFile(path.join(kit,archive))));assert.equal(catalogBefore,sha(await fs.readFile(path.join(kit,'data/design-assets/catalog.json'))));
  for(const [name,input]of Object.entries(report.inputs))assert.equal(sha(await fs.readFile(path.join(kit,name))),input.sha256,'source drift: '+name);
}catch(e){report.failures.push(String(e.stack||e));process.exitCode=1}
report.finishedAt=new Date().toISOString();report.passed=report.failures.length===0;
await write(path.join(root,'build-receipt.json'),JSON.stringify(report,null,2)+'\n');
console.log(JSON.stringify({root,passed:report.passed,failures:report.failures},null,2));
