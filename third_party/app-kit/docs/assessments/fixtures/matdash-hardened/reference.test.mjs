import assert from 'node:assert/strict';
import { constants, copyFileSync, cpSync, lstatSync, mkdtempSync, readFileSync, rmSync, symlinkSync, unlinkSync, writeFileSync } from 'node:fs';
import { join, isAbsolute } from 'node:path';
import { tmpdir } from 'node:os';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import { sourceRoots, sourceSnapshot, sha256 } from './source-closure.mjs';
import { verifySource, startHardenedReference, buildHash } from './reference.mjs';

const project = process.env.APPKIT_MATDASH_HARDENED_ROOT;
const base = process.env.APPKIT_MATDASH_BASE_ROOT;
assert.ok(project && base && isAbsolute(project) && isAbsolute(base), 'Select absolute APPKIT_MATDASH_HARDENED_ROOT and APPKIT_MATDASH_BASE_ROOT; absent private evidence fails, not skips.');
const raw = readFileSync(new URL('./build-assets.json', import.meta.url));
assert.equal(sha256(raw), buildHash);
const metadata = JSON.parse(raw), manifest = JSON.parse(readFileSync(new URL('./source-manifest.json', import.meta.url)));
const ownedRoot = t => { const root=mkdtempSync(join(tmpdir(),'appkit-matdash-hardened-canary-'));t.after(()=>rmSync(root,{recursive:true,force:true}));return root; };

test('full selected source identity preserves original and only two editor files differ', () => {
  assert.equal(verifySource(base,true).files,1269);
  assert.equal(verifySource(project).files,1269);
  assert.equal(manifest.changes.length,2);
  assert.deepEqual(manifest.changes.map(f=>f.path),['src/views/forms/from-tiptap/Tiptap.css','src/views/forms/from-tiptap/TiptapEdit.tsx']);
  const original=sourceSnapshot(base),candidate=sourceSnapshot(project),old=new Map(original.files.map(f=>[f.path,f]));
  assert.deepEqual(candidate.files.map(f=>f.path),original.files.map(f=>f.path));
  assert.deepEqual(candidate.files.filter(f=>old.get(f.path).sha256!==f.sha256).map(f=>({path:f.path,original_sha256:old.get(f.path).sha256,candidate_sha256:f.sha256})),manifest.changes);
  for(const pin of metadata.source_pins)assert.deepEqual(pin,candidate.files.find(f=>f.path===pin.path));
  for(const path of ['package.json','package-lock.json'])assert.deepEqual(readFileSync(join(project,path)),readFileSync(join(base,path)));
  assert.throws(()=>verifySource(base),/closure differs/);
  assert.throws(()=>verifySource(project,true),/closure differs/);
});

test('every pinned build asset and all declared routes served; no source or writes exposed', {timeout:45000}, async t => {
  const reference=await startHardenedReference(project);t.after(()=>reference.close());
  const request=(path,options={})=>fetch(reference.origin+path,{...options,signal:AbortSignal.timeout(5000)});
  assert.equal(metadata.assets.length,383);assert.equal(metadata.routes.length,133);
  let total=0;
  for(const asset of metadata.assets){
    const response=await request('/'+asset.path);const bytes=Buffer.from(await response.arrayBuffer());
    assert.equal(response.status,200);assert.equal(bytes.length,asset.bytes);assert.equal(sha256(bytes),asset.sha256,asset.path);
    assert.equal(response.headers.get('cache-control'),'no-store');assert.equal(response.headers.get('x-appkit-reference'),'matdash-static-read-only');
    total+=bytes.length;
  }
  assert.equal(total,metadata.total_bytes);
  const index=metadata.assets.find(f=>f.path==='index.html');
  for(const route of metadata.routes){const response=await request(route.replace(/:[A-Za-z_][A-Za-z0-9_]*/g,'synthetic-123'));assert.equal(response.status,200);assert.equal(sha256(Buffer.from(await response.arrayBuffer())),index.sha256);}
  for(const path of ['/src/App.tsx','/package.json','/not-a-reference-route'])assert.equal((await request(path)).status,404);
  assert.equal((await request('/',{method:'POST'})).status,405);
  assert.equal((await request('/',{headers:{Origin:'https://example.invalid'}})).status,403);
  const head=await request('/apps/ecommerce/addproduct',{method:'HEAD'});assert.equal(head.status,200);assert.equal((await head.arrayBuffer()).byteLength,0);
  await reference.close();await assert.rejects(request('/'));
});

test('selected source and dist tampering fail before listener startup; restored copy passes', {timeout:45000},async t=>{
  verifySource(project);
  const root=ownedRoot(t);
  for(const relative of [...sourceRoots,'dist'])cpSync(join(project,relative),join(root,relative),{recursive:lstatSync(join(project,relative)).isDirectory(),mode:constants.COPYFILE_FICLONE});
  const pristine=await startHardenedReference(root);await pristine.close();
  const nonPin=sourceSnapshot(project).files.find(f=>f.path.startsWith('src/')&&!metadata.source_pins.some(p=>p.path===f.path));
  assert.ok(nonPin);
  for(const [name,path] of [['editor','src/views/forms/from-tiptap/TiptapEdit.tsx'],['non-pin source',nonPin.path],['build','dist/index.html']]){
    await t.test(name,async()=>{
      const target=join(root,path);writeFileSync(target,'synthetic tamper\n');
      try{await assert.rejects(startHardenedReference(root));}finally{copyFileSync(join(project,path),target);}
    });
  }
  await t.test('unexpected source',async()=>{const path=join(root,'src/unexpected-synthetic.txt');writeFileSync(path,'synthetic');try{await assert.rejects(startHardenedReference(root));}finally{unlinkSync(path);}});
  await t.test('missing source',async()=>{const path='src/App.tsx';unlinkSync(join(root,path));try{await assert.rejects(startHardenedReference(root));}finally{copyFileSync(join(project,path),join(root,path));}});
  await t.test('source symlink',async()=>{const path='src/App.tsx';unlinkSync(join(root,path));symlinkSync(join(project,path),join(root,path));try{await assert.rejects(startHardenedReference(root));}finally{unlinkSync(join(root,path));copyFileSync(join(project,path),join(root,path));}});
  await t.test('root symlink',async()=>{const parent=ownedRoot(t),link=join(parent,'linked-root');symlinkSync(project,link);await assert.rejects(startHardenedReference(link));});
  const restored=await startHardenedReference(root);await restored.close();
});

test('CLI usage and missing explicit private evidence fail loudly',()=>{
  const script=fileURLToPath(new URL('./reference.mjs',import.meta.url));
  const options={encoding:'utf8',timeout:5000,env:{}};
  for(const args of [[],['--verify'],['--unknown',project],['--serve',project,'extra']])assert.equal(spawnSync(process.execPath,[script,...args],options).status,2);
  const invalid=spawnSync(process.execPath,[script,'--verify','relative'],options);assert.equal(invalid.status,1);assert.match(invalid.stderr,/reference refused/);
  const missing=spawnSync(process.execPath,['--test',fileURLToPath(import.meta.url)],options);assert.equal(missing.status,1);assert.match(missing.stdout+missing.stderr,/Select absolute APPKIT_MATDASH_HARDENED_ROOT/);
});
