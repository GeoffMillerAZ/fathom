#!/usr/bin/env node
// Private A/B source preparation only. It deliberately does not launch Gallery.
import { createHash } from 'node:crypto';
import { promises as fs } from 'node:fs';
import { basename, dirname, isAbsolute, join, relative, resolve } from 'node:path';
import { spawn } from 'node:child_process';

const maxLog = 4 << 20;
const commandTimeoutMS = 5 * 60 * 1000;
const usage = 'usage: node prepare.mjs /absolute/config.json /fresh/private/output';

const sha = async file => createHash('sha256').update(await fs.readFile(file)).digest('hex');
const write = async (file, value) => { await fs.writeFile(file, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600 }); };
async function realDir(value, name) {
  if (typeof value !== 'string' || !isAbsolute(value)) throw new Error(`${name} must be an absolute path`);
  const stat = await fs.lstat(value);
  if (stat.isSymbolicLink() || !stat.isDirectory()) throw new Error(`${name} must be a real directory`);
  return await fs.realpath(value);
}
async function regular(value, name, executable = false) {
  if (typeof value !== 'string' || !isAbsolute(value)) throw new Error(`${name} must be an absolute path`);
  let path = resolve(value); let stat = await fs.lstat(path);
  if (stat.isSymbolicLink()) { path = await fs.realpath(path); stat = await fs.lstat(path); }
  if (!stat.isFile() || (executable && !(stat.mode & 0o111))) throw new Error(`${name} must resolve to a ${executable ? 'regular executable' : 'regular file'}`);
  return path;
}
async function command(name, argv, options, receipt) {
  const stdout = [], stderr = []; let size = 0; let overflow = false;
  const record = { name, argv, cwd: options.cwd, env: options.env, timeoutMS: commandTimeoutMS, state: 'planned' };
  receipt.push(record); await write(options.commandsPath, receipt);
  const child = spawn(argv[0], argv.slice(1), { cwd: options.cwd, env: options.env, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
  let timedOut = false, spawnError = null, groupKill = null;
  const terminate = reason => { try { process.kill(-child.pid, 'SIGKILL'); groupKill = reason; } catch (error) { if (error.code !== 'ESRCH') groupKill = `${reason}:${error.code || error.message}`; } };
  const take = (into, data) => { size += data.length; if (size <= maxLog) into.push(data); else { overflow = true; terminate('output-overflow'); } };
  child.stdout.on('data', data => take(stdout, data)); child.stderr.on('data', data => take(stderr, data));
  const timer = setTimeout(() => { timedOut = true; terminate('timeout'); }, commandTimeoutMS);
  const result = await new Promise(done => { let complete = false; const finish = result => { if (!complete) { complete = true; clearTimeout(timer); done(result); } }; child.once('error', error => { spawnError = error.message; finish({ code: null, signal: null }); }); child.once('close', (code, signal) => finish({ code, signal })); });
  const prefix = join(options.logs, name);
  await fs.writeFile(`${prefix}.stdout.txt`, Buffer.concat(stdout), { mode: 0o600 });
  await fs.writeFile(`${prefix}.stderr.txt`, Buffer.concat(stderr), { mode: 0o600 });
  Object.assign(record, { state: 'completed', code: result.code, signal: result.signal, timedOut, spawnError, groupKill, overflow, stdoutSHA256: await sha(`${prefix}.stdout.txt`), stderrSHA256: await sha(`${prefix}.stderr.txt`) });
  await write(options.commandsPath, receipt);
  if (result.code !== 0 || result.signal || timedOut || spawnError || overflow) throw new Error(`${name} failed${timedOut ? ' (timeout)' : overflow ? ' (output limit)' : ''}`);
}
async function inventory(root) {
  const records = [];
  async function walk(dir) {
    for (const entry of await fs.readdir(dir, { withFileTypes: true })) {
      const file = join(dir, entry.name); const rel = relative(root, file);
      if (entry.isSymbolicLink()) throw new Error(`symlink refused in prepared source: ${rel}`);
      if (entry.isDirectory()) await walk(file);
      else if (entry.isFile()) { const stat = await fs.stat(file); records.push({ path: rel, bytes: stat.size, sha256: await sha(file) }); }
      else throw new Error(`special file refused in prepared source: ${rel}`);
    }
  }
  await walk(root); records.sort((a, b) => a.path.localeCompare(b.path)); return records;
}
const inventorySHA = async records => createHash('sha256').update(JSON.stringify(records)).digest('hex');
function sameInventory(actual, expected, allowed = new Set(), required = new Set()) {
  const a = new Map(actual.map(file => [file.path, file])); const b = new Map(expected.map(file => [file.path, file])); const changed = [];
  for (const path of new Set([...a.keys(), ...b.keys()])) if (JSON.stringify(a.get(path)) !== JSON.stringify(b.get(path))) changed.push(path);
  if (changed.some(path => !allowed.has(path)) || [...required].some(path => !changed.includes(path))) throw new Error(`unexpected source delta: ${changed.join(', ')}`);
  return changed.sort();
}
function toolEnv(c) {
  return { PATH: process.env.PATH || '', CGO_ENABLED: '0', GOWORK: 'off', GOENV: 'off', GOVCS: '*:off', GOTOOLCHAIN: 'local', GOFLAGS: '', GOPROXY: `file://${c.publicModuleProxy}`, GOPRIVATE: '', GONOPROXY: '', GOSUMDB: 'off', GONOSUMDB: '*', GOCACHE: c.goCache, GOMODCACHE: c.goModCache, TMPDIR: c.tmpDir, GOTMPDIR: c.tmpDir };
}
async function main() {
  const [configPath, output] = process.argv.slice(2);
  if (!configPath || !output || process.argv.length !== 4) throw new Error(usage);
  const configFile = await regular(configPath, 'config');
  const c = JSON.parse(await fs.readFile(configFile, 'utf8'));
  if (c.schema !== 1 || c.mode !== 'full-closure') throw new Error('config must declare schema:1 and mode:"full-closure"');
  for (const key of ['appkitRoot', 'goCache', 'goModCache', 'publicModuleProxy', 'tmpDir']) c[key] = await realDir(c[key], key);
  for (const key of ['go', 'tailwind', 'unzip', 'candidatePreparer', 'priorTemplate', 'candidatePreparerReceipt']) c[key] = await regular(c[key], key, key === 'go' || key === 'tailwind' || key === 'unzip' || key === 'candidatePreparer');
  if (typeof c.tailwindSHA256 !== 'string' || await sha(c.tailwind) !== c.tailwindSHA256) throw new Error('tailwind SHA-256 mismatch');
  if (typeof c.priorTemplateSHA256 !== 'string' || await sha(c.priorTemplate) !== c.priorTemplateSHA256) throw new Error('prior template SHA-256 mismatch');
  if (typeof c.candidatePreparerSHA256 !== 'string' || await sha(c.candidatePreparer) !== c.candidatePreparerSHA256) throw new Error('candidate preparer SHA-256 mismatch');
  if (typeof c.candidatePreparerReceiptSHA256 !== 'string' || await sha(c.candidatePreparerReceipt) !== c.candidatePreparerReceiptSHA256) throw new Error('candidate preparer receipt SHA-256 mismatch');
  if (!isAbsolute(output)) throw new Error('output must be an absolute path');
  const parent = await realDir(dirname(output), 'output parent');
  const out = join(parent, basename(output));
  if (!relative(c.appkitRoot, out).startsWith('..')) throw new Error('output must be outside appkitRoot');
  try { await fs.lstat(out); throw new Error('output already exists'); } catch (err) { if (err.code !== 'ENOENT') throw err; }
  await fs.mkdir(out, { mode: 0o700 });
  const logs = join(out, 'logs'); await fs.mkdir(logs, { mode: 0o700 });
  const frozen = join(out, 'frozen-inputs'); await fs.mkdir(frozen, { mode: 0o700 });
  const freeze = async (name, source, executable = false) => { const target = join(frozen, name); await fs.copyFile(source, target); await fs.chmod(target, executable ? 0o700 : 0o600); if (await sha(target) !== await sha(source)) throw new Error(`freeze mismatch: ${name}`); return target; };
  const frozenConfig = await freeze('config.json', configFile);
  const frozenRunner = await freeze('prepare.mjs', resolve(process.argv[1]));
  const frozenTemplate = await freeze('prior-analytics_templates.go', c.priorTemplate);
  const frozenPreparer = await freeze('candidate-preparer', c.candidatePreparer, true);
  const frozenPreparerReceipt = await freeze('candidate-preparer-receipt.json', c.candidatePreparerReceipt);
  const inputs = { config: { path: frozenConfig, sha256: await sha(frozenConfig) }, preparer: { path: frozenRunner, sha256: await sha(frozenRunner) }, priorTemplate: { path: frozenTemplate, sha256: await sha(frozenTemplate) }, candidatePreparer: { path: frozenPreparer, sha256: await sha(frozenPreparer) }, candidatePreparerReceipt: { path: frozenPreparerReceipt, sha256: await sha(frozenPreparerReceipt) }, tools: {} };
  for (const key of ['go', 'tailwind', 'unzip']) inputs.tools[key] = { path: c[key], sha256: await sha(c[key]) };
  await write(join(out, 'frozen-inputs.json'), inputs);
  const runs = [], env = toolEnv(c);
  const candidate = join(out, 'current-candidate');
  await command('snapshot-current-source', [frozenPreparer, '-root', c.appkitRoot, '-out', candidate], { cwd: c.appkitRoot, env, logs, commandsPath: join(out, 'commands.json') }, runs);
  const candidateReceipt = JSON.parse(await fs.readFile(join(candidate, 'receipt.json'), 'utf8'));
  const zip = join(candidate, 'proxy', 'github.com', 'geoffmilleraz', 'app-kit', '@v', `${candidateReceipt.version}.zip`);
  if (await sha(zip) !== candidateReceipt.zip_sha256) throw new Error('candidate ZIP hash mismatch');
  const extraction = join(out, 'extracted'); await fs.mkdir(extraction, { mode: 0o700 });
  await command('extract-current-source', [c.unzip, '-q', zip, '-d', extraction], { cwd: out, env, logs, commandsPath: join(out, 'commands.json') }, runs);
  const moduleRoot = join(extraction, 'github.com', 'geoffmilleraz', `app-kit@${candidateReceipt.version}`);
  await realDir(moduleRoot, 'extracted module root');
  const candidateFiles = candidateReceipt.files.map(file => ({ path: file.path, bytes: file.bytes, sha256: file.sha256 }));
  const extracted = await inventory(moduleRoot);
  sameInventory(extracted, candidateFiles);
  const variants = [];
  for (const [tag, prior] of [['A', true], ['B', false]]) {
    const root = join(out, tag, 'source'); await fs.mkdir(dirname(root), { mode: 0o700, recursive: true }); await fs.cp(moduleRoot, root, { recursive: true, dereference: false });
    if (prior) await fs.copyFile(frozenTemplate, join(root, 'apps/gallery/analytics_templates.go'));
    const template = join(root, 'apps/gallery/analytics_templates.go');
    const expected = prior ? c.priorTemplateSHA256 : candidateReceipt.files.find(file => file.path === 'apps/gallery/analytics_templates.go')?.sha256;
    if (!expected || await sha(template) !== expected) throw new Error(`${tag} analytics template identity mismatch`);
    const beforeBuild = await inventory(root);
    const required = prior ? new Set(['apps/gallery/analytics_templates.go']) : new Set();
    const allowedBefore = prior ? new Set(['apps/gallery/analytics_templates.go']) : new Set();
    sameInventory(beforeBuild, candidateFiles, allowedBefore, required);
    const variantLogs = join(out, tag, 'logs'); await fs.mkdir(variantLogs, { mode: 0o700 });
    await command(`${tag}-build-css`, [c.tailwind, '-i', 'apps/gallery/ui/input.css', '-o', 'apps/gallery/ui/app.css', '--minify'], { cwd: root, env, logs: variantLogs, commandsPath: join(out, 'commands.json') }, runs);
    const afterCSS = await inventory(root); sameInventory(afterCSS, beforeBuild, new Set(['apps/gallery/ui/app.css']));
    await command(`${tag}-build-gallery`, [c.go, 'build', '-trimpath', '-buildvcs=false', '-mod=readonly', '-o', join(out, tag, 'gallery'), './apps/gallery/cmd/gallery'], { cwd: root, env, logs: variantLogs, commandsPath: join(out, 'commands.json') }, runs);
    const afterGallery = await inventory(root); sameInventory(afterGallery, afterCSS);
    const finalDelta = sameInventory(afterGallery, candidateFiles, prior ? new Set(['apps/gallery/analytics_templates.go', 'apps/gallery/ui/app.css']) : new Set(['apps/gallery/ui/app.css']), required);
    variants.push({ tag, label: prior ? 'reconstructed-prior-analytics-composition' : 'current-analytics-composition', inventories: { beforeBuildSHA256: await inventorySHA(beforeBuild), afterCSSSHA256: await inventorySHA(afterCSS), afterGallerySHA256: await inventorySHA(afterGallery), finalDelta }, source: afterGallery, css: { path: 'apps/gallery/ui/app.css', sha256: await sha(join(root, 'apps/gallery/ui/app.css')) }, gallerySHA256: await sha(join(out, tag, 'gallery')) });
  }
  await write(join(out, 'receipt.json'), { schema: 1, kind: 'analytics-usability-reconstructed-composition-preparation', output: out, candidate: { receipt: join(candidate, 'receipt.json'), version: candidateReceipt.version, treeSHA256: candidateReceipt.tree_sha256, zipSHA256: candidateReceipt.zip_sha256 }, variants, commands: runs, limitations: ['A substitutes only the retained prior analytics template into a current frozen runtime/data/shared-shell source closure.', 'This is not a historical runtime replay, usability result, release candidate, or consumer evidence.', 'Gallery was built but not launched by this preparer.'] });
}
main().catch(error => { console.error(`analytics-usability prepare: ${error.message}`); process.exitCode = 1; });
