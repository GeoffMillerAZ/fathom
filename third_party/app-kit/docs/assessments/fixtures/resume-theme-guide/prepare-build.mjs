// Select the actual Go/embedded/CSS build closure, not the consumer's workspace.
// Input is `go list -mod=vendor -deps -json ./cmd` from the named consumer root.
import { readFile, writeFile, mkdir, lstat, readdir, realpath } from 'node:fs/promises';
import { resolve, relative, dirname, join, sep } from 'node:path';
import { createHash } from 'node:crypto';

const [rootArg, graphPath, outputArg] = process.argv.slice(2);
if (!rootArg || !graphPath || !outputArg) throw Error('usage: node prepare-build.mjs ROOT GO_LIST_JSON NEW_OUTPUT');
const root=await realpath(rootArg), output=resolve(outputArg);
if(output===root || output.startsWith(root+sep)) throw Error('output must be outside consumer root');
const graph=await readFile(graphPath,'utf8');
const packages=[];
let start=-1,depth=0,quoted=false,escaped=false;
for(let i=0;i<graph.length;i++) {
  const c=graph[i];
  if(start<0) { if(c==='{' && (i===0||graph[i-1]==='\n')) {start=i;depth=1;} continue; }
  if(quoted) { if(escaped) escaped=false; else if(c==='\\') escaped=true; else if(c==='"') quoted=false; }
  else if(c==='"') quoted=true;
  else if(c==='{') depth++;
  else if(c==='}' && --depth===0) {packages.push(JSON.parse(graph.slice(start,i+1)));start=-1;}
}
if(start!==-1 || !packages.some(p=>p.Name==='main'&&p.Dir===join(root,'cmd'))) throw Error('incomplete or wrong compile graph');
const selected=new Set(['go.mod','go.sum','vendor/modules.txt','cmd/serve/web/input.css','cmd/serve/web/build-css.sh']);
const admit=path=>{
  const rel=relative(root,resolve(path));
  if(!(rel.startsWith('cmd/')||rel.startsWith('vendor/')) || rel.includes('..'+sep)) throw Error('outside synthetic code/assets boundary: '+rel);
  selected.add(rel);
};
for(const p of packages.filter(p=>!p.Standard)) {
  if(p.Error || p.Incomplete || (p.CgoFiles||[]).length) throw Error('incomplete or CGO consumer package: '+p.ImportPath);
  for(const key of ['GoFiles','SFiles','HFiles','SysoFiles','EmbedFiles']) for(const f of p[key]||[]) admit(join(p.Dir,f));
}
async function collect(path) {
  const st=await lstat(path);
  if(st.isFile()) return admit(path);
  if(!st.isDirectory()) throw Error('special/symlink CSS input: '+path);
  for(const e of await readdir(path)) await collect(join(path,e));
}
const input=await readFile(join(root,'cmd/serve/web/input.css'),'utf8');
for(const m of input.matchAll(/^@(?:source|import) "([^"]+)";/gm)) {
  if(m[1]==='tailwindcss') continue;
  await collect(resolve(root,'cmd/serve/web',m[1]));
}
// Retain license/notice files from the selected vendored package ancestry.
for(const path of [...selected].filter(p=>p.startsWith('vendor/'))) {
  let dir=dirname(path);
  while(dir!=='vendor'&&dir!=='.') {
    for(const name of ['LICENSE','LICENSE.txt','LICENSE.md','COPYING','NOTICE']) {
      const file=join(dir,name);
      try { if((await lstat(join(root,file))).isFile()) selected.add(file); }
      catch(e) {if(e.code!=='ENOENT') throw e;}
    }
    dir=dirname(dir);
  }
}
const sha=b=>createHash('sha256').update(b).digest('hex');
const files=[];
for(const file of [...selected].sort()) {
  const path=join(root,file);
  if(await realpath(path)!==path || !(await lstat(path)).isFile()) throw Error('symlink/special source: '+file);
  const body=await readFile(path);
  if(body.length>32*1024*1024) throw Error('oversized source: '+file);
  files.push({path:file,sha256:sha(body),bytes:body.length,body});
}
await mkdir(output); // no overwrite
for(const file of files) {
  const target=join(output,file.path);
  await mkdir(dirname(target),{recursive:true});
  await writeFile(target,file.body,{flag:'wx',mode:0o600});
  if(sha(await readFile(join(root,file.path)))!==file.sha256) throw Error('source changed during copy: '+file.path);
}
const receipt={schema:1,kind:'selected-working-tree-build-closure',target:'darwin/arm64',files:files.map(({body,...metadata})=>metadata)};
await writeFile(join(output,'build-closure.json'),JSON.stringify(receipt,null,2)+'\n',{flag:'wx'});
process.stdout.write(JSON.stringify({files:files.length,bytes:files.reduce((n,f)=>n+f.bytes,0),manifestSHA256:sha(await readFile(join(output,'build-closure.json')))})+'\n');
