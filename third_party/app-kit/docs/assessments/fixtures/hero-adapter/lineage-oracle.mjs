// Pure P01 v2 pair oracle. It derives v1-shaped observations from a raw
// cache-lineage graph; it performs no browser, filesystem or network work.
import {checkCacheLineage} from './cache-lineage.mjs';
import {evaluateSample} from './performance-oracle.mjs';

const digest=value=>typeof value==='string'&&/^[0-9a-f]{64}$/.test(value);
const plain=value=>value!==null&&typeof value==='object'&&!Array.isArray(value);
const safe=value=>Number.isSafeInteger(value)&&value>=0;
const same=(left,right)=>{try{return JSON.stringify(left)===JSON.stringify(right)}catch{return false}};
const staticPolicy='private,max-age=3600,immutable';
const pathTypes=profile=>new Map([
  ['/enhanced','Document'],['/source.css','Stylesheet'],['/hero.css','Stylesheet'],
  ['/assets/logo-light.svg','Image'],['/assets/logo-dark.svg','Image'],['/assets/hero.jpg','Image'],
  [profile==='original-control'?'/assets/inter.woff2':'/assets/inter-subset.woff2','Font'],
]);
const forbiddenCredentials=['cookie','authorization','proxy-authorization','range','if-none-match','if-modified-since'];

function pathOf(origin,url) {
  try { const parsed=new URL(url); return parsed.origin===origin&&!parsed.search&&!parsed.hash?parsed.pathname:null; } catch { return null; }
}
function resourcesFor(expected,failures,profile='subset') {
  const exactPaths=pathTypes(profile);
  if(!['subset','original-control'].includes(profile)) failures.push({code:'expected.profile',detail:'unknown lineage resource profile'});
  const out=new Map(), resources=expected?.resources;
  if(!Array.isArray(resources)||resources.length!==7) {
    failures.push({code:'expected.resources',detail:'exactly seven frozen resources are required'});return out;
  }
  for(const item of resources) {
    if(!plain(item)||!exactPaths.has(item.path)||out.has(item.path)||item.type!==exactPaths.get(item.path)||
      !digest(item.sha256)||!safe(item.bytes)||item.bytes===0||typeof item.mime!=='string'||!item.mime) {
      failures.push({code:'expected.resource',detail:'resource identity, type or bounds are invalid'});continue;
    }
    out.set(item.path,item);
  }
  if(out.size!==exactPaths.size) failures.push({code:'expected.resourceSet',detail:'resource paths and types must equal the P01 manifest'});
  const fontPath=pathOf(expected?.origin,expected?.font?.url);
  const font=fontPath&&out.get(fontPath);
  if(!font||font.type!=='Font'||font.sha256!==expected?.font?.sha256||font.bytes!==expected?.font?.bytes||font.mime!==expected?.font?.mime) {
    failures.push({code:'expected.fontResource',detail:'font pin must match the frozen Font resource'});
  }
  return out;
}
function headerFailure(request,resource) {
  const headers=request?.responseHeaders;
  if(!plain(headers)) return 'response headers are missing';
  const policy=resource.path==='/enhanced'?'no-store':staticPolicy;
  const wanted={
    'content-type':resource.mime,'content-length':String(resource.bytes),
    'cache-control':policy,etag:`"sha256-${resource.sha256}"`,
  };
  for(const [key,value] of Object.entries(wanted)) if(headers[key]!==value) return `${key} differs from the frozen representation`;
  if(Object.hasOwn(headers,'vary')||Object.hasOwn(headers,'content-encoding')) return 'negotiated or encoded responses are refused';
  return null;
}

export function deriveLineageSample(v1Contract,expected,pair,phaseName,profile='subset') {
  const assetFailures=[], manifestFailures=[], resources=resourcesFor(expected,manifestFailures,profile);
  for(const failure of manifestFailures) assetFailures.push(failure);
  const phase=pair?.[phaseName], requestInput=Array.isArray(phase?.requests)?phase.requests:[];
  const serverInput=Array.isArray(phase?.serverRequests)?phase.serverRequests:[];
  if(requestInput.length!==7)assetFailures.push({code:'request.count',detail:'exactly seven bounded requests are required'});
  if(serverInput.length>7)assetFailures.push({code:'server.count',detail:'server records exceed the seven-resource bound'});
  const requests=requestInput.slice(0,7),server=serverInput.slice(0,7);
  const byPath=new Map(), serverByPath=new Map(); let bodyBytes=0,cacheHits=0;
  for(const request of requests) {
    const path=pathOf(expected?.origin,request?.url),resource=path&&resources.get(path);
    if(!resource||byPath.has(path)) { assetFailures.push({code:'request.path',path});continue; }
    byPath.set(path,request);
    if(request.type!==resource.type) assetFailures.push({code:'request.type',path});
    if(request.status!==200||request.redirect!==false||!safe(request.finishSequence)) assetFailures.push({code:'request.completion',path});
    const cached=safe(request.cacheSequence);
    const shouldCache=phaseName==='warm'&&path!=='/enhanced';
    if(cached!==shouldCache) assetFailures.push({code:'request.cache',path});
    if(request.fromPrefetchCache!==false||request.fromServiceWorker!==false) assetFailures.push({code:'request.cacheSource',path});
    const headerError=headerFailure(request,resource);if(headerError)assetFailures.push({code:'response.headers',path,detail:headerError});
    if(request.body?.available===true) {
      if(request.body.bytes!==resource.bytes||request.body.sha256!==resource.sha256) assetFailures.push({code:'response.body',path});
      if(!cached&&safe(request.body.bytes)) bodyBytes+=request.body.bytes;
    } else if(!(phaseName==='warm'&&resource.type==='Font')) assetFailures.push({code:'response.bodyMissing',path});
    if(cached)cacheHits++;
  }
  for(const record of server) {
    const path=pathOf(expected?.origin,record?.url),resource=path&&resources.get(path);
    if(!resource||serverByPath.has(path)||record.method!=='GET'||record.status!==200||record.bytes!==resource?.bytes||record.sha256!==resource?.sha256) {
      assetFailures.push({code:'server.identity',path});continue;
    }
    const browser=byPath.get(path),serverHeaderError=headerFailure({responseHeaders:record.responseHeaders},resource);
    if(serverHeaderError||!sameHeaders(record.responseHeaders,browser?.responseHeaders))assetFailures.push({code:'server.responseHeaders',path});
    if(hasCredentials(record.requestHeaders)||hasCredentials(browser?.requestHeaders))assetFailures.push({code:'server.requestCredentials',path});
    serverByPath.set(path,record);
  }
  for(const [path] of resources) {
    if(!byPath.has(path))assetFailures.push({code:'request.missing',path});
    if((phaseName==='cold'||path==='/enhanced')&&!serverByPath.has(path))assetFailures.push({code:'server.missing',path});
    if(phaseName==='warm'&&path!=='/enhanced'&&serverByPath.has(path))assetFailures.push({code:'server.warmStatic',path});
  }
  const metrics=plain(phase?.metrics)?phase.metrics:{};
  const documentRequest=byPath.get('/enhanced');
  const unexpected=Array.isArray(phase?.unexpectedRequests)&&phase.unexpectedRequests.length<=64?[...phase.unexpectedRequests]:['malformed or oversized unexpectedRequests'];
  const sample={
    repeat:pair?.repeat,width:pair?.width,scheme:pair?.scheme,phase:phaseName,
    bodyBytes,requests:requestInput.length,cacheHits,navigationType:phase?.navigationType,
    documentCache:documentRequest?safe(documentRequest.cacheSequence):null,
    serverDocumentRequests:server.filter(record=>pathOf(expected?.origin,record?.url)==='/enhanced').length,
    scripts:metrics.scripts,fontReady:metrics.fontReady,imagesDecoded:metrics.imagesDecoded,
    visibleImages:metrics.visibleImages,visibleLinks:metrics.visibleLinks,keyboardLinks:metrics.keyboardLinks,
    usableMs:metrics.usableMs,fcpMs:metrics.fcpMs,lcpMs:metrics.lcpMs,heapBytes:metrics.heapBytes,
    assetFailures,unexpectedRequests:unexpected,
  };
  sample.failures=evaluateSample(v1Contract,sample);
  return sample;
}

function sameHeaders(left,right) {
  if(!plain(left)||!plain(right))return false;
  const keys=Object.keys(left).sort();return same(keys,Object.keys(right).sort())&&keys.every(key=>left[key]===right[key]);
}
function hasCredentials(value) {
  return !plain(value)||forbiddenCredentials.some(key=>Object.hasOwn(value,key));
}

export function evaluateLineagePair(v1Contract,v2Contract,expected,pair) {
  const failures=[];
  const add=(code,detail)=>failures.push({code,detail});
  if(!plain(v2Contract)||v2Contract.schema!==2||v2Contract.id!=='p01-enhanced-cache-lineage-performance-v2'||
    v2Contract.cacheProof?.identityMethod!=='isolated-cache-lineage'||v2Contract.cacheProof?.directWarmBodyObserved!==false) add('contract.v2','wrong v2 contract identity');
  for(const key of ['widths','schemes','repetitions','phases','viewportHeight','network','cpuSlowdown','budgets']) {
    if(!same(v2Contract?.[key],v1Contract?.[key]))add(`contract.${key}`,'v2 must preserve the v1 workload and budgets');
  }
  const widths=Array.isArray(v2Contract?.widths)?v2Contract.widths:[],schemes=Array.isArray(v2Contract?.schemes)?v2Contract.schemes:[];
  if(!safe(pair?.repeat)||pair.repeat<1||pair.repeat>v2Contract?.repetitions||!widths.includes(pair?.width)||!schemes.includes(pair?.scheme)) add('pair.coordinate','pair coordinate is outside v2');
  const expectedFailures=[];resourcesFor(expected,expectedFailures);failures.push(...expectedFailures);
  const lineage=checkCacheLineage(expected,pair);
  for(const failure of lineage.failures) add(`lineage.${failure.code}`,failure.detail);
  const samples=['cold','warm'].map(phase=>deriveLineageSample(v1Contract,expected,pair,phase));
  for(const sample of samples)for(const failure of sample.failures)add(`${sample.phase}.${failure.code}`,failure.detail);
  for(const phaseName of ['cold','warm']) {
    const render=pair?.[phaseName]?.render;
    if(!plain(render)||render.pngWidth!==pair?.width||render.pngHeight!==v2Contract?.viewportHeight) add(`render.${phaseName}.dimensions`,'PNG dimensions must equal the pair viewport');
  }
  return {passed:failures.length===0,failures,lineage,samples};
}
