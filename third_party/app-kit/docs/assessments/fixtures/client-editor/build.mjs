// P12 private build rehearsal. Uses an existing isolated dependency tree; never installs.
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { createHash } from 'node:crypto';
import { gzipSync } from 'node:zlib';
import { spawnSync } from 'node:child_process';

const [parentArg, candidateArg, option] = process.argv.slice(2);
if (!parentArg || !candidateArg || process.argv.length > 5 || (option && option !== '--datastar')) throw new Error('Usage: node build.mjs ABSOLUTE_PARENT ABSOLUTE_PRIVATE_CANDIDATE [--datastar]');
const withDatastar = option === '--datastar';
const parent = await fs.realpath(parentArg), candidate = await fs.realpath(candidateArg);
for (const value of [parent, candidate]) {
  if ((await fs.stat(value)).mode & 0o077) throw new Error('Expected private input directories');
}
const here = path.dirname(fileURLToPath(import.meta.url));
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const pins = {
  'src/views/forms/from-tiptap/TiptapEdit.tsx': 'd50b606844d6841b876f166ac381f1c447eedd50996b18a74b54844dc01e6298',
  'src/views/forms/from-tiptap/Tiptap.css': 'c125744e0a50b06fa68a81572d49f881f6aa9d4dc51173fc46ef91eef1ddf240',
  'src/reference-assets/iconsets.json': '34d0868c0f66f51179525e8cfeb2d8a7a5fb8a0c5b97a16b0735518c8a8218e0',
};
for (const [name, expected] of Object.entries(pins)) {
  if (sha(await fs.readFile(path.join(parent, name))) !== expected) throw new Error(`Parent pin mismatch: ${name}`);
}
if (sha(await fs.readFile(path.join(candidate, 'source/Tiptap.css'))) !== pins['src/views/forms/from-tiptap/Tiptap.css']) throw new Error('Editor styles must remain identical');
const run = await fs.mkdtemp(path.join(candidate, 'build-'));
const source = path.join(run, 'source');
await fs.mkdir(source);
const copy = async (from, to) => {
  await fs.mkdir(path.dirname(to), { recursive: true });
  await fs.copyFile(from, to, 1);
};
for (const name of ['TiptapEdit.tsx', 'Tiptap.css']) await copy(path.join(candidate, 'source', name), path.join(source, name));
for (const name of ['island.tsx', 'bootstrap.ts', 'protocol.ts', 'protocol_test.mjs', 'host.css', 'build.mjs']) await copy(path.join(here, name), path.join(source, name));
for (const name of ['main.go', 'main_test.go']) await copy(path.join(here, 'host', name), path.join(run, 'host', name));
const morphAssets = [];
let runtimeIdentity;
if (withDatastar) {
  const runtimeRoot = path.resolve(here, '../../../../apps/gallery/assets/datastar/v1.0.3');
  const runtime = await fs.readFile(path.join(runtimeRoot, 'datastar.js'));
  if (runtime.length !== 33538 || sha(runtime) !== '5d6b7794a50a83d82da962aec5e382f5ae83ac7afbc751f903f7a9c6bd433c65') throw new Error('Datastar runtime pin changed');
  const license = await fs.readFile(path.join(runtimeRoot,'LICENSE.md'));
  if (sha(license)!=='17da0e216d3c6a819526f5b72a6a44d84d34bcd9beb4191ac6af89c1508ef08f') throw new Error('Datastar license pin changed');
  runtimeIdentity={bundle:{bytes:runtime.length,sha256:sha(runtime)},license:{bytes:license.length,sha256:sha(license)}};
  for (const [from, name] of [[path.join(runtimeRoot,'datastar.js'),'datastar-v1.0.3.js'],[path.join(runtimeRoot,'LICENSE.md'),'datastar-LICENSE.txt'],[path.join(here,'morph-client.js'),'morph-client.js']]) {
    await fs.mkdir(path.join(run,'morph-inputs'),{recursive:true});
    await fs.writeFile(path.join(run,'morph-inputs',name),name==='datastar-v1.0.3.js'?runtime:name==='datastar-LICENSE.txt'?license:await fs.readFile(from),{flag:'wx'});morphAssets.push(name);
  }
  await copy(path.join(runtimeRoot,'README.md'),path.join(run,'morph-inputs/README.md'));
}
await copy(path.resolve(here, '../../../design/client-editor-island.md'), path.join(run, 'contract.md'));
for (const name of Object.keys(pins)) await copy(path.join(parent, name), path.join(run, 'parent', path.basename(name)));

const parentAssetsBody = await fs.readFile(path.resolve(here, '../matdash-hardened/build-assets.json'));
if (sha(parentAssetsBody) !== 'efa4bba0c8fb5da36eded564a717a034c47ac2ca22796a5797ef14be0f20cf54') throw new Error('Parent asset manifest identity changed');
await fs.writeFile(path.join(run, 'parent/build-assets.json'), parentAssetsBody, {flag:'wx'});
const selectedAssets = JSON.parse(parentAssetsBody).assets.filter(asset =>
  ['assets/index-B5wpYHXD.css', 'assets/dash-prd-2-C01gIwt2.jpg'].includes(asset.path) ||
  (asset.path.startsWith('__appkit_reference_assets/') && /\.(css|woff2)$/.test(asset.path)));
if (selectedAssets.length !== 14) throw new Error('Unexpected source asset selection');
for (const asset of selectedAssets) {
  const bytes = await fs.readFile(path.join(parent, 'dist', asset.path));
  if (sha(bytes) !== asset.sha256 || bytes.length !== asset.bytes) throw new Error(`Parent asset pin mismatch: ${asset.path}`);
  const target = path.join(run, 'static-inputs', asset.path);
  await fs.mkdir(path.dirname(target), {recursive:true});
  await fs.writeFile(target, bytes, {flag:'wx'});
}

// Keep only the nine source-used icons; their exact bodies remain private generated data.
const collections = JSON.parse(await fs.readFile(path.join(run, 'parent/iconsets.json')));
const tabler = collections.find(collection => collection.prefix === 'tabler');
const names = ['bold', 'italic', 'underline', 'list', 'list-numbers', 'link', 'image-in-picture', 'u-turn-left', 'u-turn-right'];
const icons = Object.fromEntries(names.map(name => {
  if (!tabler?.icons[name]) throw new Error(`Missing pinned icon ${name}`);
  return [name, tabler.icons[name]];
}));
const selected = { prefix: 'tabler', width: tabler.width, height: tabler.height, icons };
await fs.writeFile(path.join(source, 'preload-icons.ts'), `import { addCollection, iconLoaded } from '@iconify/react/dist/iconify.js';\naddCollection(${JSON.stringify(selected)});\nfor (const name of ${JSON.stringify(names)}) if (!iconLoaded('tabler:' + name)) throw new Error('Missing local icon: ' + name);\n`, { flag: 'wx' });
await fs.symlink(path.join(parent, 'node_modules'), path.join(run, 'node_modules'));
const tsconfig = { compilerOptions: { target: 'ES2022', lib: ['ES2022', 'DOM'], module: 'ESNext', moduleResolution: 'Bundler', jsx: 'react-jsx', strict: true, noEmit: true, skipLibCheck: true, esModuleInterop: true, allowImportingTsExtensions: true }, include: ['source/*.ts', 'source/*.tsx'] };
await fs.writeFile(path.join(run, 'tsconfig.json'), JSON.stringify(tsconfig, null, 2) + '\n', { flag: 'wx' });
async function inventory(dir, prefix = '') {
  const result = [];
  for (const entry of (await fs.readdir(path.join(dir, prefix), { withFileTypes: true })).sort((a,b) => a.name.localeCompare(b.name, 'en'))) {
    const name = prefix ? `${prefix}/${entry.name}` : entry.name;
    if (entry.isDirectory()) result.push(...await inventory(dir, name));
    else if (entry.isFile()) { const bytes = await fs.readFile(path.join(dir, name)); result.push({ path: name, sha256: sha(bytes), bytes: bytes.length }); }
    else if (name !== 'node_modules') throw new Error(`Unexpected non-file ${name}`);
  }
  return result;
}
const sourceFiles = await inventory(run);
await fs.writeFile(path.join(run, 'inputs.json'), JSON.stringify({ parent, pins, dependencyMode: 'existing private node_modules symlink; not clean-install proof', installedLockSHA: sha(await fs.readFile(path.join(parent, 'node_modules/.package-lock.json'))), files: sourceFiles }, null, 2) + '\n', { flag: 'wx' });
console.log(JSON.stringify({ run, phase: 'inputs-retained' }));
for (const [label, args] of [
  ['protocol', ['--test', path.join(source, 'protocol_test.mjs')]],
  ['typecheck', [path.join(parent, 'node_modules/typescript/bin/tsc'), '-p', path.join(run, 'tsconfig.json')]],
  ...(withDatastar ? [['morph-syntax',['--check',path.join(run,'morph-inputs/morph-client.js')]]] : []),
]) {
  const result = spawnSync(process.execPath, args, { encoding: 'utf8' });
  await fs.writeFile(path.join(run, `${label}.log`), (result.stdout ?? '') + (result.stderr ?? ''), { flag: 'wx' });
  if (result.status !== 0) throw new Error(`${label} failed: ${run}/${label}.log\n${result.stdout}${result.stderr}`);
}
const { build } = await import(pathToFileURL(path.join(parent, 'node_modules/vite/dist/node/index.js')).href);
const config = { configFile: false, root: run, publicDir: false, logLevel: 'warn', esbuild: { jsx: 'automatic' }, build: { minify: 'esbuild', target: 'es2022', emptyOutDir: false, sourcemap: false, commonjsOptions: { strictRequires: true }, rollupOptions: { input: path.join(source, 'bootstrap.ts'), output: { entryFileNames: 'assets/editor.js', chunkFileNames: 'assets/[name]-[hash].js', assetFileNames: 'assets/editor.[ext]' } } } };
const buildConfig = { ...config, build: { ...config.build } };
await fs.writeFile(path.join(run, 'build-config.json'), JSON.stringify(buildConfig, null, 2) + '\n', { flag: 'wx' });
let previous;
const outputs = [];
for (const name of ['dist-a', 'dist-b']) {
  const outDir = path.join(run, name);
  await fs.mkdir(outDir, { mode: 0o700 });
  await build({ ...config, build: { ...config.build, outDir } });
  await copy(path.join(source, 'host.css'), path.join(outDir, 'assets/host.css'));
  for (const name of morphAssets) await copy(path.join(run,'morph-inputs',name),path.join(outDir,'assets',name));
  for (const asset of selectedAssets) {
    const target = asset.path === 'assets/index-B5wpYHXD.css' ? 'assets/base.css' : asset.path;
    await copy(path.join(run, 'static-inputs', asset.path), path.join(outDir, target));
  }
  const files = await inventory(outDir);
  const manifest = { schema: 1, entry: 'assets/editor.js', styles: ['__appkit_reference_assets/jakarta.css', '__appkit_reference_assets/manrope.css', 'assets/base.css', 'assets/editor.css', 'assets/host.css'], files };
  const bytes = JSON.stringify(manifest, null, 2) + '\n';
  await fs.writeFile(path.join(outDir, 'manifest.json'), bytes, { flag: 'wx' });
  if (previous && previous !== bytes) throw new Error('Repeat build output differs');
  previous = bytes;
  outputs.push({ name, manifestSHA: sha(bytes), files: files.length, totalBytes: files.reduce((sum,f)=>sum+f.bytes,0) });
}
const files = JSON.parse(previous).files;
const sumGzip = async ext => (await Promise.all(files.filter(f=>f.path.endsWith(ext)).map(async f=>gzipSync(await fs.readFile(path.join(run,'dist-a',f.path))).length))).reduce((a,b)=>a+b,0);
const budgets = { jsGzipBytes: await sumGzip('.js'), cssGzipBytes: await sumGzip('.css'), allFontBytes: files.filter(f=>f.path.endsWith('.woff2')).reduce((sum,f)=>sum+f.bytes,0), firstViewFonts: 'browser measurement required' };
const receipt = { schema: 1, run, node: process.version, datastarMorph:withDatastar, runtimeIdentity, outputs, repeatIdentical: true, budgets, gates: { protocol: 'pass', typecheck: 'pass', jsBudget: budgets.jsGzipBytes <= 500*1024, cssBudget: budgets.cssGzipBytes <= 100*1024, browser: 'not-run' } };
await fs.writeFile(path.join(run, 'build-receipt.json'), JSON.stringify(receipt, null, 2) + '\n', { flag: 'wx' });
console.log(JSON.stringify(receipt));
if (!receipt.gates.jsBudget || !receipt.gates.cssBudget) process.exitCode = 1;
