// Synthetic actual-process check. Arguments: baseline binary, candidate binary,
// fresh private evidence directory. Never use a real consumer root.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';

const [baseline,candidate,output]=process.argv.slice(2);
assert(baseline&&candidate&&output&&path.isAbsolute(output));
fs.mkdirSync(output,{mode:0o700});
const hash=body=>crypto.createHash('sha256').update(body).digest('hex');
const write=(name,body)=>fs.writeFileSync(path.join(output,name),body,{flag:'wx',mode:0o600});
write('probe.mjs',fs.readFileSync(new URL(import.meta.url)));
write('inputs.json',JSON.stringify({baseline:{path:baseline,sha256:hash(fs.readFileSync(baseline))},candidate:{path:candidate,sha256:hash(fs.readFileSync(candidate))},node:process.version},null,2));
const root=path.join(output,'synthetic-consumer');fs.mkdirSync(root);
fs.writeFileSync(path.join(root,'theme.css'),'/* synthetic app-owned bytes */\n');
const originalManifest='not JSON; draft must neither read nor overwrite this\n';
fs.writeFileSync(path.join(root,'appkit.adoption.json'),originalManifest);
const calls=[];
function run(binary,args,code=0) {
  const r=spawnSync(binary,['adoption',...args],{cwd:root,env:{...process.env,NO_COLOR:'1'},encoding:'utf8',timeout:10000,maxBuffer:2<<20});
  const record={binary,args,expected:code,status:r.status,signal:r.signal,error:r.error?String(r.error):null,stdout:r.stdout,stderr:r.stderr};
  calls.push(record);write(`call-${calls.length}.json`,JSON.stringify(record,null,2));
  assert.equal(r.status,code,r.stderr);assert(!r.error);assert.equal(r.signal,null);
  return r;
}
const result={started:new Date().toISOString()};
try {
  const observed=JSON.parse(run(candidate,['inspect','--root',root,'--json'],1).stdout);
  const args=['draft','--root',root,'--consumer','synthetic-draft','--profile','portable','--contract','app-kit.md',
    '--layer','L1','--layer','L0','--capability','foundation.tokens','--artifact','theme.css','--expected-catalog-revision',observed.catalog_revision,'--agents'];
  assert.equal(run(baseline,args,2).stdout,'');
  const draft=run(candidate,args).stdout;
  assert.equal(run(candidate,args).stdout,draft);
  const selection=JSON.parse(draft);
  assert.equal(selection.schema_version,2);assert(!('catalog_revision' in selection));
  assert.equal(selection.artifacts[0].sha256,hash(fs.readFileSync(path.join(root,'theme.css'))));
  assert.equal(fs.readFileSync(path.join(root,'appkit.adoption.json'),'utf8'),originalManifest);
  assert.deepEqual(fs.readdirSync(root).sort(),['appkit.adoption.json','theme.css']);
  // Saving is a separate explicit fixture action, never performed by appkit.
  fs.writeFileSync(path.join(root,'appkit.adoption.json'),draft);
  assert.equal(JSON.parse(run(candidate,['check','--root',root,'--json'],1).stdout).outcome,'failed');
  fs.writeFileSync(path.join(root,'app-kit.md'),'# Synthetic task\nReview selected bytes and return reusable findings.\n');
  const router=run(candidate,['router','--contract','app-kit.md']).stdout;
  for(const file of ['CLAUDE.md','AGENTS.md'])fs.writeFileSync(path.join(root,file),'# Synthetic app\n\n'+router);
  for(const mode of ['inspect','pins','check']) {
    const before=run(baseline,[mode,'--root',root,'--json']).stdout;
    const after=run(candidate,[mode,'--root',root,'--json']).stdout;
    assert.equal(after,before,`${mode}: existing report bytes changed`);
  }
  const stable=fs.readFileSync(path.join(root,'appkit.adoption.json'),'utf8');
  fs.appendFileSync(path.join(root,'theme.css'),'/* intentional drift */\n');
  run(candidate,['check','--root',root,'--json'],1);
  fs.unlinkSync(path.join(root,'theme.css')); // Only this synthetic fixture file.
  run(candidate,['check','--root',root,'--json'],1);
  assert.equal(run(candidate,args,2).stdout,'');
  assert.equal(fs.readFileSync(path.join(root,'appkit.adoption.json'),'utf8'),stable);
  fs.writeFileSync(path.join(root,'theme.css'),'/* synthetic app-owned bytes */\n');
  run(candidate,['check','--root',root,'--json']);
  for(const invalid of [args.concat('--capability','not.a.capability'),args.concat('--expected-catalog-revision','0'.repeat(64)),args.concat('--artifact','../outside'),args.concat('--layer','L1')]) {
    assert.equal(run(candidate,invalid,2).stdout,'');
  }
  const moduleOnly=['draft','--root',root,'--consumer','module-only','--profile','cli','--contract','app-kit.md','--module-file','go.mod','--module-version','v0.1.0'];
  const moduleDraft=JSON.parse(run(candidate,moduleOnly).stdout);
  assert.deepEqual(moduleDraft.capabilities,[]);assert.deepEqual(moduleDraft.capability_revisions,[]);
  assert.deepEqual(moduleDraft.artifacts,[]);assert.equal(moduleDraft.module.version,'v0.1.0');
  assert(!fs.existsSync(path.join(root,'go.mod')));
  result.pass=true;
} catch(error) {result.pass=false;result.error=String(error);}
result.calls=calls.length;result.finished=new Date().toISOString();write('result.json',JSON.stringify(result,null,2));
console.log(JSON.stringify(result));process.exitCode=result.pass?0:1;
