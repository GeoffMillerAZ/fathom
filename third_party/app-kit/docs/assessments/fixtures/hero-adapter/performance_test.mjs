import assert from 'node:assert/strict';
import test from 'node:test';
import {evaluateCanary, evaluateMatrix, evaluateSample} from './performance-oracle.mjs';

const contract = {
  schema:1, id:'p01-enhanced-local-performance-v1', widths:[390,1440], schemes:['light','dark'], repetitions:2, phases:['cold','warm'],
  viewportHeight:1000,
  network:{latencyMs:80,downloadBytesPerSecond:1250000,uploadBytesPerSecond:625000},
  cpuSlowdown:4,
  budgets:{
    cold:{bodyBytes:524288,requests:7,fcpMs:1000,lcpMs:1800,usableMs:2000,heapBytes:16777216},
    warm:{bodyBytes:16384,requests:7,fcpMs:600,lcpMs:1000,usableMs:1200,heapBytes:16777216},
  },
};

function sample(overrides={}) {
  const width=overrides.width??390, phase=overrides.phase??'cold';
  const links=width===1440?['/announcement','/start','/learn']:['/start','/learn'];
  return {
    repeat:1,width,scheme:'light',phase,
    bodyBytes:phase==='cold'?500000:12000,requests:7,fcpMs:500,lcpMs:800,usableMs:900,
    heapBytes:8_000_000,assetFailures:[],unexpectedRequests:[],scripts:0,fontReady:true,
    imagesDecoded:3,visibleImages:2,visibleLinks:links,keyboardLinks:[...links],cacheHits:phase==='warm'?1:0,
    navigationType:'navigate',documentCache:false,serverDocumentRequests:1,
    ...overrides,
  };
}

function matrix() {
  const samples=[];
  for(let repeat=1;repeat<=2;repeat++) for(const width of contract.widths) for(const scheme of contract.schemes) for(const phase of contract.phases) {
    samples.push(sample({repeat,width,scheme,phase}));
  }
  return samples;
}

const codes = failures => failures.map(item=>item.code);

test('baseline sample and exact matrix pass',()=>{
  assert.deepEqual(evaluateSample(contract,sample()),[]);
  assert.equal(matrix().length,16);
  assert.deepEqual(evaluateMatrix(contract,matrix()),[]);
});

test('payload and independent timing budgets fail without averaging',()=>{
  assert.ok(codes(evaluateSample(contract,sample({bodyBytes:524289}))).includes('budget.bodyBytes'));
  const delayed=evaluateSample(contract,sample({fcpMs:1001,lcpMs:1801,usableMs:2001}));
  assert.ok(codes(delayed).includes('budget.fcpMs'));
  assert.ok(codes(delayed).includes('budget.lcpMs'));
  assert.ok(codes(delayed).includes('budget.usableMs'));
  const samples=matrix(); samples[0].fcpMs=1001;
  assert.ok(codes(evaluateMatrix(contract,samples)).includes('matrix.sample.0.budget.fcpMs'));
});

test('assets, request counts and unexpected network cannot pass',()=>{
  const failures=evaluateSample(contract,sample({assetFailures:['photo'],unexpectedRequests:['https://example.test'],requests:6,imagesDecoded:2,visibleImages:1}));
  for(const code of ['assets.failed','requests.unexpected','count.requests','count.imagesDecoded','count.visibleImages']) assert.ok(codes(failures).includes(code),code);
});

test('metrics and booleans require their exact types and ranges',()=>{
  const malformed=evaluateSample(contract,sample({bodyBytes:NaN,fcpMs:0,lcpMs:-1,usableMs:Infinity,heapBytes:0,requests:7.5,cacheHits:1.5,fontReady:'true',scripts:'0'}));
  for(const code of ['sample.bodyBytes','sample.fcpMs','sample.lcpMs','sample.usableMs','sample.heapBytes','sample.requests.integer','sample.cacheHits.integer','state.fontReady','count.scripts']) assert.ok(codes(malformed).includes(code),code);
  const missing=sample(); delete missing.bodyBytes; delete missing.fcpMs;
  assert.ok(codes(evaluateSample(contract,missing)).includes('sample.bodyBytes'));
  assert.ok(codes(evaluateSample(contract,missing)).includes('sample.fcpMs'));
  assert.doesNotThrow(()=>evaluateSample(contract,null));
  assert.ok(codes(evaluateSample(contract,null)).includes('sample.type'));
});

test('byte counts are integral and cache hits cannot exceed requests',()=>{
  const failures=evaluateSample(contract,sample({bodyBytes:100.5,heapBytes:1000.5,cacheHits:8}));
  for(const code of ['sample.bodyBytes.integer','sample.heapBytes.integer','sample.cacheHits.range']) assert.ok(codes(failures).includes(code),code);
});

test('visible and keyboard links are exact and width-sensitive',()=>{
  assert.ok(codes(evaluateSample(contract,sample({visibleLinks:['/start'],keyboardLinks:['/learn']}))).includes('links.visible'));
  assert.ok(codes(evaluateSample(contract,sample({visibleLinks:['/start'],keyboardLinks:['/learn']}))).includes('links.keyboard'));
  assert.deepEqual(evaluateSample(contract,sample({width:1440})),[]);
});

test('warm samples require a cache hit and warm budgets',()=>{
  const failures=evaluateSample(contract,sample({phase:'warm',cacheHits:0,bodyBytes:16385,fcpMs:601,lcpMs:1001,usableMs:1201}));
  assert.ok(codes(failures).includes('state.warmCache'));
  for(const code of ['budget.bodyBytes','budget.fcpMs','budget.lcpMs','budget.usableMs']) assert.ok(codes(failures).includes(code),code);
});

test('every sample proves a fresh document navigation and cold cache',()=>{
  const failures=evaluateSample(contract,sample({navigationType:'back_forward',documentCache:true,serverDocumentRequests:0,cacheHits:1}));
  for(const code of ['navigation.type','navigation.documentCache','navigation.serverDocumentRequests','state.coldCache']) assert.ok(codes(failures).includes(code),code);
});

test('malformed or unknown coordinates never throw',()=>{
  for(const changed of [{repeat:0},{repeat:3},{width:768},{scheme:'system'},{phase:'hot'}]) {
    assert.doesNotThrow(()=>evaluateSample(contract,sample(changed)));
    assert.notEqual(evaluateSample(contract,sample(changed)).length,0);
  }
  const missingCoordinate=sample(); delete missingCoordinate.width;
  assert.ok(codes(evaluateSample(contract,missingCoordinate)).includes('sample.width'));
  assert.ok(codes(evaluateMatrix(contract,[null])).includes('matrix.sample.0.sample.type'));
});

test('matrix rejects empty, duplicate, missing and unknown coordinates',()=>{
  assert.ok(codes(evaluateMatrix(contract,[])).includes('matrix.coordinate.missing'));
  const missing=matrix(); missing.pop();
  assert.ok(codes(evaluateMatrix(contract,missing)).includes('matrix.coordinate.missing'));
  const duplicate=matrix(); duplicate[15]={...duplicate[0]};
  assert.ok(codes(evaluateMatrix(contract,duplicate)).includes('matrix.coordinate.duplicate'));
  assert.ok(codes(evaluateMatrix(contract,duplicate)).includes('matrix.coordinate.missing'));
  const unknown=matrix(); unknown[0]={...unknown[0],width:768};
  assert.ok(codes(evaluateMatrix(contract,unknown)).includes('matrix.coordinate.unknown'));
});

test('contract and array shape errors are reported rather than thrown',()=>{
  assert.doesNotThrow(()=>evaluateMatrix(null,[]));
  assert.ok(codes(evaluateMatrix(null,[])).includes('contract.type'));
  assert.ok(codes(evaluateMatrix({...contract,schema:2},matrix())).includes('contract.schema'));
  const weakened=structuredClone(contract); weakened.budgets.cold.bodyBytes++;
  assert.ok(codes(evaluateMatrix(weakened,matrix())).includes('contract.budgets.cold.bodyBytes'));
  for(const [changed,code] of [
    [{...contract,id:'other'},'contract.id'],
    [{...contract,viewportHeight:900},'contract.viewportHeight'],
    [{...contract,cpuSlowdown:1},'contract.cpuSlowdown'],
    [{...contract,network:{...contract.network,latencyMs:0}},'contract.network'],
    [{...contract,network:{...contract.network,downloadBytesPerSecond:9_999_999}},'contract.network'],
    [{...contract,network:{...contract.network,uploadBytesPerSecond:9_999_999}},'contract.network'],
  ]) assert.ok(codes(evaluateMatrix(changed,matrix())).includes(code),code);
  const malformed=sample({assetFailures:null,unexpectedRequests:'none',visibleLinks:null,keyboardLinks:false});
  for(const code of ['sample.assetFailures','sample.unexpectedRequests','sample.visibleLinks','sample.keyboardLinks']) assert.ok(codes(evaluateSample(contract,malformed)).includes(code),code);
});

const hashes={expected:'a'.repeat(64),changed:'b'.repeat(64)};
const resourcePaths=['/enhanced','/source.css','/hero.css','/assets/logo-light.svg','/assets/logo-dark.svg','/assets/hero.jpg','/assets/inter.woff2'];
function canarySample(name) {
  const requestGraph=resourcePaths.map(path=>({path,status:200,finished:true,cache:false,bodyBytes:100,sha256:hashes.expected,expectedBodyBytes:100,expectedSHA256:hashes.expected,durationMs:100,serverDelayMs:0}));
  const next=sample({requestGraph});
  const target={
    'missing-font':'/assets/inter.woff2',
    'delayed-css':'/source.css',
    'oversize-image':'/assets/hero.jpg',
  }[name];
  const entry=requestGraph.find(item=>item.path===target);
  if(name==='missing-font') {entry.status=404;entry.bodyBytes=20;entry.sha256=hashes.changed;next.fontReady=false;next.assetFailures=[{path:target,status:404}]}
  if(name==='delayed-css') {entry.durationMs=2500;entry.serverDelayMs=2500;next.fcpMs=1001;next.usableMs=2001}
  if(name==='oversize-image') {entry.bodyBytes=614500;entry.sha256=hashes.changed;next.bodyBytes=524289;next.assetFailures=[{path:target,status:200}]}
  return next;
}

test('three canaries require their exact injected evidence',()=>{
  for(const name of ['missing-font','delayed-css','oversize-image']) {
    const result=evaluateCanary(contract,name,canarySample(name));
    assert.equal(result.detected,true,`${name}: ${JSON.stringify(result.failures)}`);
  }
});

test('unrelated failures and wrong targets never count as canary detection',()=>{
  const unrelated=sample({bodyBytes:524289,requestGraph:canarySample('delayed-css').requestGraph});
  assert.equal(evaluateCanary(contract,'oversize-image',unrelated).detected,false);
  const wrong=canarySample('missing-font');
  const font=wrong.requestGraph.find(item=>item.path==='/assets/inter.woff2');Object.assign(font,{status:200,bodyBytes:100,sha256:hashes.expected});
  const photo=wrong.requestGraph.find(item=>item.path==='/assets/hero.jpg');Object.assign(photo,{status:404,bodyBytes:20,sha256:hashes.changed});wrong.assetFailures=[{path:'/assets/hero.jpg'}];
  assert.equal(evaluateCanary(contract,'missing-font',wrong).detected,false);
});

test('canaries fail closed on partial metrics, navigation, sizes and timing',()=>{
  const partial=canarySample('missing-font');delete partial.heapBytes;
  assert.equal(evaluateCanary(contract,'missing-font',partial).detected,false);
  const navigation=canarySample('oversize-image');navigation.navigationType='back_forward';
  assert.equal(evaluateCanary(contract,'oversize-image',navigation).detected,false);
  const size=canarySample('oversize-image');size.requestGraph.find(item=>item.path==='/assets/hero.jpg').bodyBytes--;
  assert.equal(evaluateCanary(contract,'oversize-image',size).detected,false);
  const timing=canarySample('delayed-css');timing.requestGraph.find(item=>item.path==='/source.css').durationMs=2499;
  assert.equal(evaluateCanary(contract,'delayed-css',timing).detected,false);
  assert.equal(evaluateCanary(contract,'unknown',canarySample('missing-font')).detected,false);
  assert.equal(evaluateCanary(contract,'missing-font',null).detected,false);
});

test('canaries reject unrelated structural request failures',()=>{
  const changed=canarySample('oversize-image');
  const css=changed.requestGraph.find(item=>item.path==='/hero.css');css.finished=false;changed.assetFailures.push({path:'/hero.css'});
  assert.equal(evaluateCanary(contract,'oversize-image',changed).detected,false);
});

test('Chrome aborted-font 404 requires exact failure identity and failed readiness',()=>{
  const changed=canarySample('missing-font');
  const entry=changed.requestGraph.find(item=>item.path==='/assets/inter.woff2');
  Object.assign(entry,{finished:false,error:'net::ERR_ABORTED',bodyBytes:0,sha256:'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'});
  assert.equal(evaluateCanary(contract,'missing-font',changed).detected,true);
  for(const patch of [{status:200},{path:'/assets/hero.jpg'},{error:'net::ERR_FAILED'},{error:'unknown'},{durationMs:undefined},{bodyBytes:1},{sha256:hashes.changed}]){
    const bad=structuredClone(changed);Object.assign(bad.requestGraph.at(-1),patch);
    assert.equal(evaluateCanary(contract,'missing-font',bad).detected,false,JSON.stringify(patch));
  }
  for(const fontReady of [true,undefined,'false'])assert.equal(evaluateCanary(contract,'missing-font',{...changed,fontReady}).detected,false);
  const noFailure={...changed,assetFailures:[]};assert.equal(evaluateCanary(contract,'missing-font',noFailure).detected,false);
});
