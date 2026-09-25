import assert from 'node:assert/strict';
import fs from 'node:fs';
import {createHash} from 'node:crypto';
const root='/Volumes/m2ext/appkit-evidence/appkit-static-replay.ud7EOtjr';
const hash=b=>createHash('sha256').update(b).digest('hex');
const runs=['cold-1','cold-2'].map(name=>{
  const receiptPath=`${root}/${name}/run-receipt.json`;
  const bytes=fs.readFileSync(receiptPath),r=JSON.parse(bytes);
  assert.equal(r.passed,true);assert.equal(r.tests.skips,0);assert.equal(r.tests.failures,0);assert.equal(r.tests.passed.length,7);
  assert.deepEqual(r.initial_caches,{module_cache_entries:0,build_cache_entries:0});
  assert.equal(r.commands.length,9);
  for(const c of r.commands){assert.equal(c.timed_out,false);assert.equal(c.output_overflow,false);assert.equal(c.group.absent,true);assert.equal(c.group.cleanup_required,false);assert.equal(c.lifecycle.ok,true);}
  for(const [name,facts] of Object.entries(r.artifacts)){
    const body=fs.readFileSync(`${root}/${r.run}/${name}`);assert.equal(body.length,facts.bytes);assert.equal(hash(body),facts.sha256);
  }
  return {receiptPath,receiptSHA256:hash(bytes),r};
});
assert.deepEqual(runs[0].r.candidate,runs[1].r.candidate);
assert.deepEqual(runs[0].r.artifacts,runs[1].r.artifacts);
assert.deepEqual(runs[0].r.site,runs[1].r.site);
const report={schema_version:'static-cold-pair-v1',pass:true,runs:runs.map(({receiptPath,receiptSHA256})=>({receiptPath,receiptSHA256})),artifacts:runs[0].r.artifacts,site:runs[0].r.site,
  claim:'two filesystem-constrained local Go1.26.3 darwin/arm64 CGO-disabled cache-cold builds; not independent-host, browser or deployed-consumer proof'};
fs.writeFileSync(`${root}/pair-report.json`,JSON.stringify(report,null,2)+'\n',{flag:'wx',mode:0o600});
console.log(JSON.stringify({pass:true,binariesMatch:true,cssMatch:true,siteFiles:report.site.length}));
