// Replay the frozen actual-200% P09 repair suite with the explicit runtime derivative.
// Measurement functions, tasks, thresholds and captured states remain unchanged.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';

const [baseArg,repairArg,repairSHA,runtimeArg,runtimeSHA]=process.argv.slice(2);
assert(process.argv.length===7&&[baseArg,repairArg,runtimeArg].every(path.isAbsolute)&&[repairSHA,runtimeSHA].every(v=>/^[a-f0-9]{64}$/.test(v??'')),
  'usage: node runtime-zoom.mjs BASE_PROBE REPAIR_PROBE REPAIR_COMPILE_SHA256 RUNTIME_ROOT RUNTIME_RECEIPT_SHA256');
const sha=body=>createHash('sha256').update(body).digest('hex'),kit=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../../..');
const frozenURL=new URL('./repair.mjs',import.meta.url),frozen=await fs.readFile(frozenURL,'utf8');
assert.equal(sha(frozen),'6db51ff8d5f291cfa78f15731a4cc9ce6c47be0eaa2f7d25ace994ed43b900f2');
const zoom=await fs.readFile(new URL('./zoom.mjs',import.meta.url)),zoomContract=await fs.readFile(new URL('./zoom-contract.mjs',import.meta.url));
const runtimeRoot=await fs.realpath(runtimeArg),temp=await fs.realpath(os.tmpdir());
assert(runtimeRoot.startsWith(temp+path.sep),'runtime proof must remain in command-scoped temporary storage');
const repairRoot=await fs.realpath(repairArg);
const runnerDir=await fs.mkdtemp(path.join(runtimeRoot,'zoom-runner-'));await fs.chmod(runnerDir,0o700);
const runnerFile=path.join(runnerDir,'run.mjs');
await fs.writeFile(path.join(runnerDir,'zoom.mjs'),zoom,{mode:0o600,flag:'wx'});
await fs.writeFile(path.join(runnerDir,'zoom-contract.mjs'),zoomContract,{mode:0o600,flag:'wx'});

let code=frozen;
function replaceOnce(before,after){assert.equal(code.split(before).length-1,1,'frozen input selector drift');code=code.replace(before,after);}
replaceOnce("assert(repairProbe.startsWith(await fs.realpath(os.tmpdir())+path.sep));","assert.equal(repairProbe,"+JSON.stringify(repairRoot)+",'repair input root differs');");
replaceOnce("path.join(repairProbe, 'browser-')","path.join("+JSON.stringify(runtimeRoot)+", 'runtime-zoom-')");
replaceOnce("kind:'private-p09-navbar-repair-browser'","kind:'private-p09-navbar-runtime-zoom'");
replaceOnce("  const kit=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../../..');","  const kit="+JSON.stringify(kit)+";");
const originalDeclaration="  const runtime=await input(path.join(assetsRoot,'elements.js'),artifact.runtime.esm_sha256);";
replaceOnce(originalDeclaration,`
  const originalRuntime=await input(path.join(assetsRoot,'elements.js'),artifact.runtime.esm_sha256);
  const runtimeReceipt=JSON.parse(await input(path.join(${JSON.stringify(runtimeRoot)},'runtime-receipt.json'),${JSON.stringify(runtimeSHA)}));
  assert.equal(runtimeReceipt.kind,'private-p09-navbar-runtime-replay');
  const runtimeArtifact=JSON.parse(await input(path.join(${JSON.stringify(runtimeRoot)},'runtime-artifact.json'),runtimeReceipt.artifact_sha256));
  assert.equal(runtimeArtifact.schema_version,1);assert.equal(runtimeArtifact.adapter,'p09-navbar-runtime-focus-v1');
  assert.equal(runtimeArtifact.repair_sha256,repairCompile.artifact_sha256);assert.equal(runtimeReceipt.repair_sha256,repairCompile.artifact_sha256);
  assert.equal(runtimeArtifact.original_runtime_sha256,sha(originalRuntime));assert.equal(runtimeReceipt.original_runtime_sha256,sha(originalRuntime));
  assert.equal(runtimeArtifact.wrapper_attribute,'data-appkit-navbar-repair');assert.equal(runtimeArtifact.verification,'not-verified');
  const runtimeRule=await input(path.join(kit,'internal/designadapt/navbar_runtime.go')),runtimeSchema=await input(path.join(kit,'internal/designadapt/navbar_runtime.cue'));
  assert.equal(runtimeArtifact.rule_sha256,sha(Buffer.concat([runtimeRule,Buffer.from([0]),runtimeSchema])));
  const before=runtimeRule.toString().match(/const navbarFocusBefore = \x60([^\x60]+)\x60/)[1],after=runtimeRule.toString().match(/const navbarFocusAfter = \x60([^\x60]+)\x60/)[1];
  assert.equal(originalRuntime.toString().split(before).length-1,1);
  const runtime=await input(path.join(${JSON.stringify(runtimeRoot)},'elements-navbar.js'),runtimeArtifact.runtime_sha256);
  assert.equal(runtime.toString(),runtimeArtifact.runtime);assert.equal(runtime.toString(),originalRuntime.toString().replace(before,after));
  assert.equal(runtimeReceipt.runtime_sha256,sha(runtime));assert.notEqual(sha(runtime),sha(originalRuntime));
  report.runtime_derivative={receipt_sha256:${JSON.stringify(runtimeSHA)},artifact_sha256:runtimeReceipt.artifact_sha256,original_sha256:sha(originalRuntime),served_sha256:sha(runtime),repair_sha256:repairCompile.artifact_sha256};
  report.frozen_harness_sha256=${JSON.stringify(sha(frozen))};
  report.derivation_harness_sha256=sha(await input(new URL(${JSON.stringify(import.meta.url)})));
  report.executed_harness_sha256=sha(await input(${JSON.stringify(runnerFile)}));
`);
await fs.writeFile(runnerFile,code,{mode:0o600,flag:'wx'});

// The derivative substitutions end before the task loop. This binds the actual
// 200% keyboard/focus/capture work to the frozen suite rather than restating it.
const taskStart="  for(const scheme of ['light','dark'])for(const width of [1440,780]){";
assert.equal(frozen.slice(frozen.indexOf(taskStart)),code.slice(code.indexOf(taskStart)),'frozen 200% task block changed');
process.argv=process.argv.slice(0,5);
await import(pathToFileURL(runnerFile).href);
