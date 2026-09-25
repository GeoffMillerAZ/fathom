// Freeze a new exclusive synthetic control packet before any tests or browser run.
// node prepare-memory-control.mjs NEW_EMPTY_PRIVATE_ROOT [--startup-only|--component-backgrounds-off|--shutdown-only]
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import {createReadStream} from 'node:fs';
import {fileURLToPath} from 'node:url';
import {execFileSync} from 'node:child_process';
import {memoryControlFlags, memoryControlModes} from './memory-control-profile.mjs';

const [output, option] = process.argv.slice(2);
assert.ok(process.argv.length === 3 || (process.argv.length === 4 &&
  ['--startup-only', '--component-backgrounds-off', '--shutdown-only'].includes(option)));
const startupProfile = option === '--startup-only' ? 'no-background-components-v1' :
  option === '--component-backgrounds-off' ? 'no-background-components-controls-v1' :
  option === '--shutdown-only' ? 'no-background-components-shutdown-v1' : 'baseline';
assert.ok(path.isAbsolute(output));
assert.deepEqual(await fs.readdir(output), [], 'staging root must be empty');
const root = fileURLToPath(new URL('../../../', import.meta.url));
const hash = async file => {
  const h = crypto.createHash('sha256');
  for await (const chunk of createReadStream(file)) h.update(chunk);
  return h.digest('hex');
};
const paths = [
  ...['memory-control', 'memory-control-fixture', 'memory-control-fixture.test', 'prepare-memory-control',
    'memory-control-profile', 'memory-control-profile.test',
    'startup-window', 'startup-window.test',
    'owned_context', 'owned_context.test',
    'owned_process', 'owned_process.test', 'cdp_guard', 'lifecycle', 'lifecycle-check', 'memory', 'process_rss']
    .map(name => `apps/styleguide/browser/${name}.mjs`),
  'docs/assessments/fixtures/styleguide-performance/memory-v2-metrics.mjs',
  'docs/assessments/fixtures/styleguide-performance/memory-check.mjs',
  'docs/assessments/fixtures/styleguide-performance/memory-v2-contract.md',
  'docs/assessments/2026-09-22-theme-memory-controls.md',
];
const sources = [];
for (const relative of paths) {
  const source = path.join(root, relative), destination = path.join(output, 'src', relative);
  await fs.mkdir(path.dirname(destination), {recursive: true, mode: 0o700});
  await fs.copyFile(source, destination, 1);
  const sha256 = await hash(source);
  assert.equal(await hash(destination), sha256, relative + ' changed during copy');
  sources.push({relative, path: destination, source, sha256});
}
const chromeRoot = '/Applications/Google Chrome.app/Contents';
const chromeVersion = execFileSync('/usr/libexec/PlistBuddy', ['-c', 'Print :CFBundleShortVersionString', chromeRoot + '/Info.plist'], {encoding: 'utf8'}).trim();
assert.match(chromeVersion, /^\d+\.\d+\.\d+\.\d+$/);
const selected = {
  node: process.execPath, chrome: chromeRoot + '/MacOS/Google Chrome',
  chrome_framework: chromeRoot + `/Frameworks/Google Chrome Framework.framework/Versions/${chromeVersion}/Google Chrome Framework`,
  chrome_info: chromeRoot + '/Info.plist',
};
const tools = {};
for (const [name, file] of Object.entries(selected)) tools[name] = {path: await fs.realpath(file), sha256: await hash(file)};
const manifest = {schema: 'appkit-memory-control-inputs-v1', created_at: new Date().toISOString(),
  source_head: execFileSync('git', ['rev-parse', 'HEAD'], {cwd: root, encoding: 'utf8'}).trim(),
  owner: path.join(output, 'src/apps/styleguide/browser/memory-control.mjs'), sources, tools,
  node_version: process.version, chrome_version: chromeVersion,
  startup_profile: startupProfile, chrome_flags: memoryControlFlags(startupProfile),
  allowed_modes: memoryControlModes(startupProfile),
  scope: option === '--startup-only' ? 'One startup-only target inventory and cleanup; no fixture, app or memory workload.' :
    option === '--shutdown-only' ? 'One synthetic twenty-action context-disposal/shutdown diagnostic; no memory sampling or acceptance.' :
    'One positive then one retention synthetic instrument control; stop on failed qualification.',
  exclusions: ['No Playwright dependency', 'Not a full Chrome bundle archive', 'Not the Gallery workload']};
await fs.writeFile(path.join(output, 'manifest.json'), JSON.stringify(manifest, null, 2), {flag: 'wx'});
console.log(JSON.stringify({output, files: sources.length, node: process.version, chrome: chromeVersion}));
