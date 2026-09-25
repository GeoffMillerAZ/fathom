// Replay the frozen acceptance suite with an explicitly supplied runtime derivative.
// Only input selection/provenance and output naming differ. No assertion is replaced.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';

const args=process.argv.slice(2),[,,,,,runtimeArg,runtimeSHA]=args;
assert(args.length===7&&path.isAbsolute(runtimeArg)&&/^[a-f0-9]{64}$/.test(runtimeSHA??''),
  'usage: node runtime-acceptance.mjs BASE_PROBE REPAIR_PROBE REPAIR_COMPILE_SHA256 SUPPLEMENTAL_ROOT SUPPLEMENTAL_RECEIPT_SHA256 RUNTIME_ROOT RUNTIME_RECEIPT_SHA256');
const sha=body=>createHash('sha256').update(body).digest('hex');
const frozenURL=new URL('./acceptance.mjs',import.meta.url),frozen=await fs.readFile(frozenURL,'utf8');
assert.equal(sha(frozen),'5517f72b46b986954ee6001d36388c583b38ff260a03ead9404f51f7f95ee590');
const runtimeRoot=await fs.realpath(runtimeArg),temp=await fs.realpath(os.tmpdir());
assert(runtimeRoot.startsWith(temp+path.sep),'runtime proof must remain in command-scoped temporary storage');
const runnerDir=await fs.mkdtemp(path.join(runtimeRoot,'acceptance-runner-'));await fs.chmod(runnerDir,0o700);
const runnerFile=path.join(runnerDir,'run.mjs');
let code=frozen;
function replaceOnce(before,after){assert.equal(code.split(before).length-1,1,'frozen input selector drift');code=code.replace(before,after);}
replaceOnce("path.join(supplemental,'acceptance-')","path.join(supplemental,'runtime-acceptance-')");
replaceOnce("kind:'private-p09-navbar-repair-acceptance'","kind:'private-p09-navbar-runtime-acceptance'");
const originalDeclaration="  const runtime=await input(path.join(assetsRoot,'elements.js'),artifact.base.runtime.esm_sha256),font=await input(path.join(assetsRoot,'inter.woff2'),prior.assets['inter.woff2'].sha256),repairCSS=await input(path.join(repairProbe,'navbar_repair.css'),artifact.css_sha256);";
replaceOnce(originalDeclaration,`
  const originalRuntime=await input(path.join(assetsRoot,'elements.js'),artifact.base.runtime.esm_sha256);
  const runtimeReceipt=JSON.parse(await input(path.join(${JSON.stringify(runtimeRoot)},'runtime-receipt.json'),${JSON.stringify(runtimeSHA)}));
  assert.equal(runtimeReceipt.kind,'private-p09-navbar-runtime-replay');
  const runtimeArtifact=JSON.parse(await input(path.join(${JSON.stringify(runtimeRoot)},'runtime-artifact.json'),runtimeReceipt.artifact_sha256));
  assert.equal(runtimeArtifact.schema_version,1);assert.equal(runtimeArtifact.adapter,'p09-navbar-runtime-focus-v1');
  assert.equal(runtimeArtifact.repair_sha256,compile.artifact_sha256);assert.equal(runtimeReceipt.repair_sha256,compile.artifact_sha256);
  assert.equal(runtimeArtifact.original_runtime_sha256,sha(originalRuntime));assert.equal(runtimeReceipt.original_runtime_sha256,sha(originalRuntime));
  assert.equal(runtimeArtifact.wrapper_attribute,'data-appkit-navbar-repair');assert.equal(runtimeArtifact.verification,'not-verified');
  const rule=await input(path.join(kit,'internal/designadapt/navbar_runtime.go')),schema=await input(path.join(kit,'internal/designadapt/navbar_runtime.cue'));
  assert.equal(runtimeArtifact.rule_sha256,sha(Buffer.concat([rule,Buffer.from([0]),schema])));
  const before=rule.toString().match(/const navbarFocusBefore = \x60([^\x60]+)\x60/)[1],after=rule.toString().match(/const navbarFocusAfter = \x60([^\x60]+)\x60/)[1];
  assert.equal(originalRuntime.toString().split(before).length-1,1);
  const runtime=await input(path.join(${JSON.stringify(runtimeRoot)},'elements-navbar.js'),runtimeArtifact.runtime_sha256);
  assert.equal(runtime.toString(),runtimeArtifact.runtime);assert.equal(runtime.toString(),originalRuntime.toString().replace(before,after));
  assert.equal(runtimeReceipt.runtime_sha256,sha(runtime));assert.notEqual(sha(runtime),sha(originalRuntime));
  report.runtime_derivative={receipt_sha256:${JSON.stringify(runtimeSHA)},artifact_sha256:runtimeReceipt.artifact_sha256,original_sha256:sha(originalRuntime),served_sha256:sha(runtime),repair_sha256:compile.artifact_sha256};
  report.derivation_harness_sha256=sha(await input(new URL(${JSON.stringify(import.meta.url)})));
  report.executed_harness_sha256=sha(await input(${JSON.stringify(runnerFile)}));
  const font=await input(path.join(assetsRoot,'inter.woff2'),prior.assets['inter.woff2'].sha256),repairCSS=await input(path.join(repairProbe,'navbar_repair.css'),artifact.css_sha256);
`);
// Preserve the frozen suite's relative fixtures and identity. Record the executed
// derivative separately above, so it can never masquerade as the original harness.
assert.equal((code.match(/import\.meta\.url/g)||[]).length,3);
code=code.replaceAll('import.meta.url',JSON.stringify(frozenURL.href));
await fs.writeFile(runnerFile,code,{mode:0o600,flag:'wx'});
process.argv=process.argv.slice(0,7);
await import(pathToFileURL(runnerFile).href);
