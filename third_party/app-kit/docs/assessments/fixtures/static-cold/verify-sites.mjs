// Independent static-output check. No browser, server, dependency install or mutation of sites.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {createHash} from 'node:crypto';
import {pathToFileURL} from 'node:url';

const hash = b => createHash('sha256').update(b).digest('hex');
const [left, right, reportPath, contractPath, historicalPath, probePath] = process.argv.slice(2);
assert.equal(process.argv.length,8,'six arguments required');
assert([left,right,reportPath,contractPath,historicalPath,probePath].every(p=>p&&path.isAbsolute(p)), 'six absolute paths required');
assert.equal(hash(await fs.readFile(probePath)),'21285bcb533c284ff620ce189dc370d4a2a2efee9479d2cb09a08ee2fafeefb2');
const {validateContract, validateManifest, assessLinks}=await import(pathToFileURL(probePath));
assert.notEqual(await fs.realpath(left), await fs.realpath(right), 'site roots must differ');
const contractBytes=await fs.readFile(contractPath);
assert.equal(hash(contractBytes),'e2a3969b3f5514cffe4c5859702bd5b6556d7d2a09fef551b03e00579b54bd9f');
const contract=validateContract(JSON.parse(contractBytes));
const historicBytes=await fs.readFile(historicalPath);
assert.equal(hash(historicBytes),'d210b335715fdeeafc44d4b3c70bdd5c919548cf41b69a43d995c77e54c69057');
const historic=JSON.parse(historicBytes);
const expected=[...contract.files].sort();
const checks=[];
async function readSite(root) {
  const names=(await fs.readdir(root)).sort();assert.deepEqual(names,expected,'exact eleven-file closure');
  const files={}, bodies={};
  for(const name of names) {
    const file=path.join(root,name), stat=await fs.lstat(file);
    assert(stat.isFile()&&!stat.isSymbolicLink()&&stat.size>0&&stat.size<=131072,`bounded regular output: ${name}`);
    const body=await fs.readFile(file);assert.equal(body.length,stat.size);
    files[name]={kind:'file',bytes:body.length,sha256:hash(body)};bodies[name]=body;
  }
  const manifest=JSON.parse(bodies['manifest.json']);
  assert.deepEqual(validateManifest(contract,manifest,files),[], 'independent manifest validation');
  assert.deepEqual(manifest.files.map(f=>f.path),expected.filter(n=>n!=='manifest.json'),'sorted manifest');
  const css=bodies['app.css'].toString();
  for(const token of ['--ak-color-ink','--ak-color-focus','--ak-color-accent','.min-h-11','focus-visible','data-theme=dark'])assert(css.includes(token),`required selected CSS: ${token}`);
  assert(!/@font-face|@import|url\(\s*["']?(?:https?:|\/\/)/i.test(css),'no unclosed imported/font/remote CSS dependency');
  for(const name of names.filter(n=>n.endsWith('.html'))) {
    const html=bodies[name].toString();
    assert(!/<(?:script|form|base)\b|\son[a-z]+\s*=/i.test(html),`no inert/executable controls: ${name}`);
    assert.equal((html.match(/<main\b/g)||[]).length,1,`one main: ${name}`);
    assert(html.includes('id="static-reference-main"')&&html.includes('tabindex="-1"'),`focusable main: ${name}`);
    const csp=/<meta\s+http-equiv="Content-Security-Policy"\s+content="([^"]+)"/i.exec(html)?.[1];
    assert.equal(csp,"default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'none'",`exact script-denying CSP: ${name}`);
    const links=[...html.matchAll(/<a\b[^>]*href="([^"]*)"/gi)].map(m=>m[1]);
    assert.deepEqual(assessLinks(contract,name,links),[],`independent destination grammar: ${name}`);
    if(name.startsWith('index')) {
      assert(html.includes(contract.oracles.index_heading)&&html.includes(contract.oracles.faq_question),`reading/FAQ task: ${name}`);
      assert.equal((html.match(/<details\b/g)||[]).length,4,`outline plus three native FAQs: ${name}`);
      assert.deepEqual([...html.matchAll(/<details\s+id="([^"]+)"/g)].map(match=>match[1]).sort(),
        ['profile','quality','source'],`exact native FAQ identities: ${name}`);
    } else if(name.startsWith('profile')) assert(html.includes(contract.oracles.profile_heading),`article task: ${name}`);
    else assert(html.includes(contract.oracles.problem_heading)&&html.includes('/reading/index'),`prefix-bound recovery: ${name}`);
  }
  // Seeded manifest failures use detached facts, never alter retained files.
  const wrongHash=structuredClone(manifest);wrongHash.files[0].sha256='0'.repeat(64);
  assert(validateManifest(contract,wrongHash,files).length,'hash poison must fail');
  assert(validateManifest(contract,manifest,{...files,'unexpected.html':files['index.html']}).length,'extra output must fail');
  const wrongBytes=structuredClone(manifest);wrongBytes.files[0].bytes++;
  assert(validateManifest(contract,wrongBytes,files).length,'size poison must fail');
  checks.push({root,manifestPass:true,semanticChecksPass:true,poisonControls:3});
  return files;
}
const a=await readSite(left),b=await readSite(right);assert.deepEqual(a,b,'two cold sites must be byte-identical');
const historical=expected.map(name=>({path:name,current:a[name].sha256,historical:historic.artifacts[`site/${name}`]?.sha256,
  equal:a[name].sha256===historic.artifacts[`site/${name}`]?.sha256}));
const report={schema_version:'static-cold-output-check-v1',pass:true,checks,files:a,historical,
  claim:'exact twin output/manifest/content checks; historical identity comparison, not a fresh browser or host result'};
await fs.writeFile(reportPath,JSON.stringify(report,null,2)+'\n',{flag:'wx',mode:0o600});
console.log(JSON.stringify({pass:true,files:expected.length,historicalMatches:historical.filter(f=>f.equal).length}));
