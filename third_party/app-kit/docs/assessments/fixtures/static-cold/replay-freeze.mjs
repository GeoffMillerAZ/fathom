import fs from 'node:fs';
import {createHash} from 'node:crypto';
const root='/Volumes/m2ext/appkit-evidence/appkit-static-replay.ud7EOtjr';
const old='/Volumes/m2ext/appkit-evidence/appkit-static-cold.7qApnn7w';
const hash=body=>createHash('sha256').update(body).digest('hex');
const oldBytes=fs.readFileSync(`${old}/frozen/pre-execution-v5-sha256.txt`);
if(hash(oldBytes)!=='0d98d8afc6c480ba735d57f30517476a6bee74ac1f22ad5febbb75dc394bf7c1')throw Error('old manifest changed');
const records=oldBytes.toString().trim().split('\n').map(line=>{
  const m=/^([0-9a-f]{64})  (\/.+)$/.exec(line);
  if(!m)throw Error('malformed prior manifest');
  const st=fs.lstatSync(m[2]);
  if(!st.isFile()||st.isSymbolicLink()||hash(fs.readFileSync(m[2]))!==m[1])throw Error(`prior frozen input drift: ${m[2]}`);
  return {sha256:m[1],path:m[2]};
});
for(const file of ['contract.md','freeze-inputs-v2.mjs','frozen/cold-runner-v2.mjs','frozen/inventory.mjs','frozen/inventory_test.mjs','frozen/policies/cold-1.sb','frozen/policies/cold-2.sb']){
  const p=`${root}/${file}`,st=fs.lstatSync(p);
  if(!st.isFile()||st.isSymbolicLink())throw Error(`unsafe new input: ${p}`);
  records.push({sha256:hash(fs.readFileSync(p)),path:p});
}
const node='/opt/homebrew/Cellar/node/26.3.0/bin/node';
if(hash(fs.readFileSync(node))!=='56694c81b093cc8da273fa017cf91765b3653e5f64f16727976ffaa87b2b6b31')throw Error('Node changed');
records.push({sha256:hash(fs.readFileSync(node)),path:node});
records.sort((a,b)=>a.path<b.path?-1:a.path>b.path?1:0);
const paths=new Set();for(const r of records){if(paths.has(r.path))throw Error('duplicate manifest path');paths.add(r.path);}
const body=records.map(r=>`${r.sha256}  ${r.path}\n`).join('');
fs.writeFileSync(`${root}/frozen/pre-execution-v2-sha256.txt`,body,{flag:'wx',mode:0o600});
console.log(JSON.stringify({files:records.length,sha256:hash(body)}));
