import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {dirname,join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {createHash} from 'node:crypto';
import {assessRecovery,validateContract} from './recovery_browser.mjs';

const frozenRoot=process.env.APPKIT_FATHOM_FROZEN_ROOT;
const fixtureRelative='docs/assessments/fixtures/fathom-handler';
if(!frozenRoot||dirname(fileURLToPath(import.meta.url))!==join(frozenRoot,'source-snapshot',fixtureRelative))throw Error('Recovery tests require the retained runner; unfrozen execution refused');
const preparation=JSON.parse(await readFile(join(frozenRoot,'run-config.json'),'utf8'));
for(const file of ['recovery_run_test.mjs','recovery_browser.mjs','recovery_contract.json']){
  const bytes=await readFile(new URL('./'+file,import.meta.url));
  if(createHash('sha256').update(bytes).digest('hex')!==preparation.snapshots[join(fixtureRelative,file)])throw Error('Frozen test input identity mismatch: '+file);
}

const contract={version:1,service:'synthetic-long-service',expected:{callsTotal:3,callsAgent:2,configuration:'not inspectable'},cacheTTLMillis:30000,freshCacheMaxMillis:3000,refreshWaitMillis:35000,recoveryWaitMillis:40000,pollMillis:500,control:{statusPath:'/__appkit_recovery/status',transitionPath:'/__appkit_recovery/transition',tokenHeader:'X-Appkit-Recovery',requestBody:'recover\n',contentType:'text/plain'},lanes:[{width:390,height:1000,theme:'light'},{width:390,height:1000,theme:'dark'},{width:1440,height:1000,theme:'light'},{width:1440,height:1000,theme:'dark'}]};
const facts={before:{unreadable:true,falseZero:false},warm:{cacheAtUnixMilli:1000},immediate:{unreadable:true,exactData:false,cacheAtUnixMilli:1000},after:{unreadable:false,falseZero:false,exactData:true,recoveredAtUnixMilli:32000}};

test('contract pins the real TTL and exact recovery data',()=>assert.deepEqual(validateContract(structuredClone(contract)),contract));
test('owned frozen contract matches the browser schema',async()=>assert.equal(validateContract(JSON.parse(await readFile(new URL('./recovery_contract.json',import.meta.url),'utf8'))).version,1));
test('complete natural recovery passes',()=>assert.deepEqual(assessRecovery(contract,facts),[]));
test('false zero fails',()=>assert.ok(assessRecovery(contract,{...facts,before:{unreadable:true,falseZero:true}}).includes('before.false-zero')));
test('premature recovery fails',()=>assert.ok(assessRecovery(contract,{...facts,immediate:{...facts.immediate,exactData:true}}).includes('immediate.premature-recovery')));
test('cache replacement fails',()=>assert.ok(assessRecovery(contract,{...facts,immediate:{...facts.immediate,cacheAtUnixMilli:2000}}).includes('immediate.cache-replaced')));
test('stale failure fails',()=>assert.ok(assessRecovery(contract,{...facts,after:{...facts.after,unreadable:true}}).includes('after.stale-failure')));
test('missing expected data fails',()=>assert.ok(assessRecovery(contract,{...facts,after:{...facts.after,exactData:false}}).includes('after.missing-data')));
test('missing state facts fail closed',()=>{assert.ok(assessRecovery(contract,{...facts,before:{unreadable:true}}).includes('before.missing-fact'));assert.ok(assessRecovery(contract,{...facts,immediate:{unreadable:true,cacheAtUnixMilli:1000}}).includes('immediate.missing-fact'));assert.ok(assessRecovery(contract,{...facts,after:{unreadable:false,exactData:true,recoveredAtUnixMilli:32000}}).includes('after.missing-fact'));});
test('early and late recovery fail',()=>{assert.ok(assessRecovery(contract,{...facts,after:{...facts.after,recoveredAtUnixMilli:30000}}).includes('after.expiry-window'));assert.ok(assessRecovery(contract,{...facts,after:{...facts.after,recoveredAtUnixMilli:42001}}).includes('after.expiry-window'));});
